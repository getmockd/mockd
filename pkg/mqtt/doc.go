// Package mqtt provides an MQTT broker mock for IoT testing.
//
// The mqtt package implements a fully functional MQTT broker that can be used
// to mock IoT device communication patterns. It supports authentication, ACL rules,
// TLS encryption, and automated message simulation.
//
// # Basic Usage
//
// Create and start a broker:
//
//	config := &mqtt.MQTTConfig{
//	    Port:    1883,
//	    Enabled: true,
//	}
//	broker, err := mqtt.NewBroker(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := broker.Start(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//	defer broker.Stop(context.Background(), 5*time.Second)
//
// # Authentication
//
// Enable authentication with username/password and ACL rules:
//
//	config := &mqtt.MQTTConfig{
//	    Port:    1883,
//	    Enabled: true,
//	    Auth: &mqtt.MQTTAuthConfig{
//	        Enabled: true,
//	        Users: []mqtt.MQTTUser{
//	            {
//	                Username: "device1",
//	                Password: "secret",
//	                ACL: []mqtt.ACLRule{
//	                    {Topic: "sensors/#", Access: "readwrite"},
//	                },
//	            },
//	        },
//	    },
//	}
//
// # Simulating IoT Devices
//
// Configure topics to automatically publish messages:
//
//	config := &mqtt.MQTTConfig{
//	    Port:    1883,
//	    Enabled: true,
//	    Topics: []mqtt.TopicConfig{
//	        {
//	            Topic: "sensors/temperature",
//	            Messages: []mqtt.MessageConfig{
//	                {
//	                    Payload:  `{"temp": 22.5, "unit": "C"}`,
//	                    Interval: "5s",
//	                    Repeat:   true,
//	                },
//	            },
//	        },
//	    },
//	}
//
// # TLS Support
//
// Enable TLS encryption:
//
//	config := &mqtt.MQTTConfig{
//	    Port:    8883,
//	    Enabled: true,
//	    TLS: &mqtt.MQTTTLSConfig{
//	        Enabled:  true,
//	        CertFile: "/path/to/cert.pem",
//	        KeyFile:  "/path/to/key.pem",
//	    },
//	}
//
// # Message Forwarding
//
// Configure topics to forward messages to other topics:
//
//	topics := []mqtt.TopicConfig{
//	    {
//	        Topic: "sensors/raw",
//	        OnPublish: &mqtt.PublishHandler{
//	            Forward: "sensors/processed",
//	        },
//	    },
//	}
package mqtt
