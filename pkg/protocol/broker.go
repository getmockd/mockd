package protocol

// MessageBroker is a handler that acts as a message broker (MQTT, AMQP, NATS).
// This combines the Handler, PubSub, and ConnectionManager interfaces to provide
// a complete message broker abstraction.
//
// Example implementation:
//
//	type MQTTBroker struct {
//	    // Embeds or delegates to implementations of:
//	    // - Handler (lifecycle)
//	    // - PubSub (topics and subscriptions)
//	    // - ConnectionManager (client connections)
//	}
type MessageBroker interface {
	Handler
	PubSub
	ConnectionManager
}

// QueueBroker supports queue-based messaging (AMQP, Kafka).
// This extends MessageBroker with queue operations for protocols
// that have explicit queue concepts.
//
// Example implementation:
//
//	func (b *AMQPBroker) CreateQueue(name string, durable bool) error {
//	    b.mu.Lock()
//	    defer b.mu.Unlock()
//	    if _, exists := b.queues[name]; exists {
//	        return protocol.ErrQueueExists
//	    }
//	    b.queues[name] = &queue{
//	        name:    name,
//	        durable: durable,
//	    }
//	    return nil
//	}
type QueueBroker interface {
	MessageBroker

	// ListQueues returns information about all queues.
	ListQueues() []QueueInfo

	// CreateQueue creates a new queue.
	// Returns ErrQueueExists if a queue with the name already exists.
	CreateQueue(name string, durable bool) error

	// DeleteQueue removes a queue.
	// Returns ErrQueueNotFound if the queue does not exist.
	DeleteQueue(name string) error

	// QueueDepth returns the number of messages in a queue.
	// Returns ErrQueueNotFound if the queue does not exist.
	QueueDepth(name string) (int64, error)
}

// QueueInfo provides information about a queue.
// Returned by QueueBroker.ListQueues() and used by the Admin API.
type QueueInfo struct {
	// Name is the queue name.
	Name string `json:"name"`

	// Messages is the current number of messages in the queue.
	Messages int64 `json:"messages"`

	// Consumers is the number of consumers currently consuming from the queue.
	Consumers int `json:"consumers"`

	// Durable indicates if the queue survives broker restart.
	Durable bool `json:"durable"`
}
