package protocol

import (
	"context"
	"time"
)

// MessageHandler processes received messages.
// Used as a callback for subscription handlers.
type MessageHandler func(topic string, msg Message) error

// Subscription represents an active subscription.
// Returned by Subscriber.Subscribe() and used to manage the subscription lifecycle.
type Subscription interface {
	// ID returns the unique subscription identifier.
	ID() string

	// Topic returns the topic or pattern this subscription is for.
	Topic() string

	// Unsubscribe cancels the subscription.
	// After calling Unsubscribe, no more messages will be delivered to the handler.
	Unsubscribe() error
}

// SubscriptionInfo provides information about a subscription.
// This struct is returned by PubSub.ListSubscriptions() and used
// by the Admin API to display subscription details.
type SubscriptionInfo struct {
	// ID is the unique subscription identifier.
	ID string `json:"id"`

	// Topic is the topic or pattern this subscription is for.
	Topic string `json:"topic"`

	// ClientID is the client that owns this subscription.
	// May be empty for internal subscriptions.
	ClientID string `json:"clientId,omitempty"`

	// CreatedAt is when the subscription was created.
	CreatedAt time.Time `json:"createdAt"`
}

// Publisher can publish messages to topics/channels.
// Implement this interface for protocols that support message publishing.
//
// Example implementation:
//
//	func (h *MyHandler) Publish(ctx context.Context, topic string, msg protocol.Message) error {
//	    h.mu.RLock()
//	    subs := h.subscriptions[topic]
//	    h.mu.RUnlock()
//	    for _, sub := range subs {
//	        sub.handler(topic, msg)
//	    }
//	    return nil
//	}
type Publisher interface {
	// Publish sends a message to a topic.
	// The context can be used for cancellation and timeout.
	Publish(ctx context.Context, topic string, msg Message) error
}

// Subscriber can subscribe to topics/channels.
// Implement this interface for protocols that support subscriptions.
//
// Example implementation:
//
//	func (h *MyHandler) Subscribe(ctx context.Context, pattern string, handler protocol.MessageHandler) (protocol.Subscription, error) {
//	    sub := &subscription{
//	        id:      uuid.New().String(),
//	        topic:   pattern,
//	        handler: handler,
//	    }
//	    h.mu.Lock()
//	    h.subscriptions[pattern] = append(h.subscriptions[pattern], sub)
//	    h.mu.Unlock()
//	    return sub, nil
//	}
type Subscriber interface {
	// Subscribe registers a handler for messages on topics matching the pattern.
	// The pattern may include wildcards (protocol-specific).
	// Returns a Subscription that can be used to unsubscribe.
	Subscribe(ctx context.Context, pattern string, handler MessageHandler) (Subscription, error)
}

// PubSub combines publishing and subscribing capabilities.
// Implement this interface for full pub/sub protocols like MQTT, NATS, etc.
type PubSub interface {
	Publisher
	Subscriber

	// ListTopics returns all active topic names.
	// The definition of "active" is protocol-specific (e.g., has subscribers, has messages).
	ListTopics() []string

	// ListSubscriptions returns information about all active subscriptions.
	ListSubscriptions() []SubscriptionInfo
}
