package mqtt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroker_ListClientInfos_Empty(t *testing.T) {
	t.Parallel()
	broker, err := NewBroker(&MQTTConfig{Port: 0})
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	infos := broker.ListClientInfos()
	// The inline client may or may not appear, but we filter it out
	for _, info := range infos {
		assert.False(t, info.Closed, "active clients should not be closed")
	}
}

func TestBroker_GetClientInfo_NotFound(t *testing.T) {
	t.Parallel()
	broker, err := NewBroker(&MQTTConfig{Port: 0})
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	info := broker.GetClientInfo("nonexistent-client")
	assert.Nil(t, info)
}

func TestBroker_DisconnectClient_NotFound(t *testing.T) {
	t.Parallel()
	broker, err := NewBroker(&MQTTConfig{Port: 0})
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	err = broker.DisconnectClient("nonexistent-client")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBroker_DisconnectClient_BrokerNotRunning(t *testing.T) {
	t.Parallel()
	broker, err := NewBroker(&MQTTConfig{Port: 0})
	require.NoError(t, err)

	// Don't start the broker
	err = broker.DisconnectClient("some-client")
	assert.Error(t, err)
}

func TestBroker_GetConnectionStats_Empty(t *testing.T) {
	t.Parallel()
	broker, err := NewBroker(&MQTTConfig{Port: 0})
	require.NoError(t, err)

	require.NoError(t, broker.Start(context.Background()))
	defer broker.Stop(context.Background(), 5*time.Second)

	time.Sleep(100 * time.Millisecond)

	connected, totalSubs, subsByClient := broker.GetConnectionStats()
	assert.GreaterOrEqual(t, connected, 0)
	assert.Equal(t, 0, totalSubs)
	assert.NotNil(t, subsByClient)
}

func TestBroker_ListClientInfos_NilServer(t *testing.T) {
	t.Parallel()
	broker := &Broker{
		config:              &MQTTConfig{ID: "test"},
		clientSubscriptions: make(map[string][]string),
	}

	infos := broker.ListClientInfos()
	assert.Nil(t, infos)
}

func TestBroker_GetClientInfo_NilServer(t *testing.T) {
	t.Parallel()
	broker := &Broker{
		config:              &MQTTConfig{ID: "test"},
		clientSubscriptions: make(map[string][]string),
	}

	info := broker.GetClientInfo("any-client")
	assert.Nil(t, info)
}
