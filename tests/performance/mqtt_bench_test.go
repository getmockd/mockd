package performance

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mqttclient "github.com/eclipse/paho.mqtt.golang"
	"github.com/getmockd/mockd/pkg/mqtt"
)

func setupBenchMQTTBroker(b *testing.B) (*mqtt.Broker, int) {
	// Find free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	cfg := &mqtt.MQTTConfig{
		Port: port,
	}

	broker, err := mqtt.NewBroker(cfg)
	if err != nil {
		b.Fatalf("failed to create broker: %v", err)
	}

	if err := broker.Start(context.Background()); err != nil {
		b.Fatalf("failed to start broker: %v", err)
	}

	b.Cleanup(func() {
		broker.Stop(context.Background(), time.Second)
	})

	time.Sleep(50 * time.Millisecond)
	return broker, port
}

func createBenchMQTTClient(b *testing.B, port int, clientID string) mqttclient.Client {
	opts := mqttclient.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://localhost:%d", port))
	opts.SetClientID(clientID)
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(5 * time.Second)

	client := mqttclient.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		b.Fatalf("connect timeout")
	}
	if token.Error() != nil {
		b.Fatalf("connect error: %v", token.Error())
	}

	return client
}

// BenchmarkMQTT_PublishQoS0 measures QoS 0 (fire and forget) publish rate.
func BenchmarkMQTT_PublishQoS0(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)
	client := createBenchMQTTClient(b, port, "bench-pub-qos0")
	defer client.Disconnect(100)

	payload := []byte(`{"sensor":"temp","value":23.5}`)
	topic := "bench/qos0"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := client.Publish(topic, 0, false, payload)
		token.Wait()
	}
}

// BenchmarkMQTT_PublishQoS1 measures QoS 1 (at least once) publish rate.
func BenchmarkMQTT_PublishQoS1(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)
	client := createBenchMQTTClient(b, port, "bench-pub-qos1")
	defer client.Disconnect(100)

	payload := []byte(`{"sensor":"temp","value":23.5}`)
	topic := "bench/qos1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := client.Publish(topic, 1, false, payload)
		token.Wait()
	}
}

// BenchmarkMQTT_PublishQoS2 measures QoS 2 (exactly once) publish rate.
func BenchmarkMQTT_PublishQoS2(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)
	client := createBenchMQTTClient(b, port, "bench-pub-qos2")
	defer client.Disconnect(100)

	payload := []byte(`{"sensor":"temp","value":23.5}`)
	topic := "bench/qos2"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token := client.Publish(topic, 2, false, payload)
		token.Wait()
	}
}

// BenchmarkMQTT_PubSubLatency measures end-to-end message latency.
func BenchmarkMQTT_PubSubLatency(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)

	// Publisher
	pub := createBenchMQTTClient(b, port, "bench-pub")
	defer pub.Disconnect(100)

	// Subscriber
	sub := createBenchMQTTClient(b, port, "bench-sub")
	defer sub.Disconnect(100)

	topic := "bench/latency"
	payload := []byte(`{"test":"latency"}`)

	received := make(chan struct{}, 1)
	sub.Subscribe(topic, 1, func(c mqttclient.Client, m mqttclient.Message) {
		select {
		case received <- struct{}{}:
		default:
		}
	}).Wait()

	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pub.Publish(topic, 1, false, payload).Wait()
		<-received
	}
}

// BenchmarkMQTT_ConnectionEstablishment measures connection setup time.
func BenchmarkMQTT_ConnectionEstablishment(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := createBenchMQTTClient(b, port, fmt.Sprintf("bench-conn-%d", i))
		client.Disconnect(50)
	}
}

// BenchmarkMQTT_ConcurrentPublishers measures throughput with multiple publishers.
func BenchmarkMQTT_ConcurrentPublishers(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)

	numPublishers := 10
	msgsPerPublisher := b.N / numPublishers
	if msgsPerPublisher < 1 {
		msgsPerPublisher = 1
	}

	payload := []byte(`{"sensor":"temp","value":23.5}`)

	b.ResetTimer()

	var wg sync.WaitGroup
	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := createBenchMQTTClient(b, port, fmt.Sprintf("bench-pub-%d", id))
			defer client.Disconnect(100)

			topic := fmt.Sprintf("bench/concurrent/%d", id)
			for i := 0; i < msgsPerPublisher; i++ {
				client.Publish(topic, 0, false, payload).Wait()
			}
		}(p)
	}
	wg.Wait()
}

// BenchmarkMQTT_Fanout measures message fanout to multiple subscribers.
func BenchmarkMQTT_Fanout(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)

	numSubscribers := 10
	topic := "bench/fanout"
	payload := []byte(`{"broadcast":"message"}`)

	// Create subscribers
	var received int64
	var subs []mqttclient.Client
	for i := 0; i < numSubscribers; i++ {
		sub := createBenchMQTTClient(b, port, fmt.Sprintf("bench-sub-%d", i))
		sub.Subscribe(topic, 1, func(c mqttclient.Client, m mqttclient.Message) {
			atomic.AddInt64(&received, 1)
		}).Wait()
		subs = append(subs, sub)
	}
	defer func() {
		for _, s := range subs {
			s.Disconnect(100)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Publisher
	pub := createBenchMQTTClient(b, port, "bench-pub-fanout")
	defer pub.Disconnect(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		atomic.StoreInt64(&received, 0)
		pub.Publish(topic, 1, false, payload).Wait()

		// Wait for all subscribers to receive
		deadline := time.Now().Add(time.Second)
		for atomic.LoadInt64(&received) < int64(numSubscribers) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
	}
}

// BenchmarkMQTT_MessageSizes measures throughput with different payload sizes.
func BenchmarkMQTT_MessageSizes(b *testing.B) {
	_, port := setupBenchMQTTBroker(b)
	client := createBenchMQTTClient(b, port, "bench-sizes")
	defer client.Disconnect(100)

	sizes := map[string]int{
		"64B":  64,
		"1KB":  1024,
		"10KB": 10 * 1024,
		"64KB": 64 * 1024,
	}

	for name, size := range sizes {
		payload := make([]byte, size)
		topic := fmt.Sprintf("bench/size/%s", name)

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				client.Publish(topic, 0, false, payload).Wait()
			}
		})
	}
}
