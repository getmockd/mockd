// Package recording provides conversion from MQTT recordings to mock configurations.
package recording

import (
	"github.com/getmockd/mockd/pkg/mqtt"
)

// MQTTConvertOptions configures how MQTT recordings are converted to configs.
type MQTTConvertOptions struct {
	// Deduplicate uses first message per topic when true
	Deduplicate bool `json:"deduplicate,omitempty"`

	// IncludeQoS preserves QoS level in the config
	IncludeQoS bool `json:"includeQoS,omitempty"`

	// IncludeRetain preserves retain flag in the config
	IncludeRetain bool `json:"includeRetain,omitempty"`
}

// DefaultMQTTConvertOptions returns default conversion options.
func DefaultMQTTConvertOptions() MQTTConvertOptions {
	return MQTTConvertOptions{
		Deduplicate:   true,
		IncludeQoS:    true,
		IncludeRetain: true,
	}
}

// ToTopicConfig converts a single MQTT recording to a TopicConfig.
func ToTopicConfig(rec *MQTTRecording, opts MQTTConvertOptions) *mqtt.TopicConfig {
	if rec == nil {
		return nil
	}

	cfg := &mqtt.TopicConfig{
		Topic: rec.Topic,
		Messages: []mqtt.MessageConfig{
			{
				Payload: rec.PayloadString(),
			},
		},
	}

	if opts.IncludeQoS {
		cfg.QoS = rec.QoS
	}

	if opts.IncludeRetain {
		cfg.Retain = rec.Retain
	}

	return cfg
}

// ToMQTTConfig converts recordings to a complete MQTTConfig.
func ToMQTTConfig(recordings []*MQTTRecording, opts MQTTConvertOptions) *mqtt.MQTTConfig {
	if len(recordings) == 0 {
		return nil
	}

	// Group recordings by topic
	groups := make(map[string][]*MQTTRecording)
	order := make([]string, 0)

	for _, rec := range recordings {
		if _, exists := groups[rec.Topic]; !exists {
			order = append(order, rec.Topic)
		}
		groups[rec.Topic] = append(groups[rec.Topic], rec)
	}

	// Build topic configs
	topics := make([]mqtt.TopicConfig, 0, len(order))

	for _, topic := range order {
		recs := groups[topic]

		if opts.Deduplicate {
			// Use first recording for each topic
			topicCfg := ToTopicConfig(recs[0], opts)
			if topicCfg != nil {
				topics = append(topics, *topicCfg)
			}
		} else if len(recs) > 0 {
			// Use all recordings - combine messages into single topic config
			cfg := &mqtt.TopicConfig{
				Topic:    topic,
				Messages: make([]mqtt.MessageConfig, 0, len(recs)),
			}

			// Use QoS and Retain from first recording
			if opts.IncludeQoS {
				cfg.QoS = recs[0].QoS
			}
			if opts.IncludeRetain {
				cfg.Retain = recs[0].Retain
			}

			// Add all messages
			for _, rec := range recs {
				cfg.Messages = append(cfg.Messages, mqtt.MessageConfig{
					Payload: rec.PayloadString(),
				})
			}

			topics = append(topics, *cfg)
		}
	}

	return &mqtt.MQTTConfig{
		Topics:  topics,
		Enabled: true,
	}
}

// MQTTConvertResult contains the result of converting MQTT recordings.
type MQTTConvertResult struct {
	Config       *mqtt.MQTTConfig `json:"config"`
	TopicCount   int              `json:"topicCount"`
	MessageCount int              `json:"messageCount"`
	Total        int              `json:"total"`
	Warnings     []string         `json:"warnings,omitempty"`
}

// ConvertMQTTRecordings converts a set of recordings to an MQTTConfig with stats.
func ConvertMQTTRecordings(recordings []*MQTTRecording, opts MQTTConvertOptions) *MQTTConvertResult {
	result := &MQTTConvertResult{
		Total:    len(recordings),
		Warnings: make([]string, 0),
	}

	if len(recordings) == 0 {
		return result
	}

	result.Config = ToMQTTConfig(recordings, opts)

	// Count topics and messages
	if result.Config != nil {
		result.TopicCount = len(result.Config.Topics)
		for _, topic := range result.Config.Topics {
			result.MessageCount += len(topic.Messages)
		}
	}

	// Add warnings for empty payloads
	for _, rec := range recordings {
		if len(rec.Payload) == 0 {
			result.Warnings = append(result.Warnings,
				"Recording "+rec.ID+" on topic "+rec.Topic+" has empty payload")
		}
	}

	return result
}

// MergeMQTTConfigs merges recordings into an existing MQTTConfig.
func MergeMQTTConfigs(base *mqtt.MQTTConfig, recordings []*MQTTRecording, opts MQTTConvertOptions) *mqtt.MQTTConfig {
	if base == nil {
		return ToMQTTConfig(recordings, opts)
	}

	newConfig := ToMQTTConfig(recordings, opts)
	if newConfig == nil {
		return base
	}

	// Build a map of existing topics for fast lookup
	existingTopics := make(map[string]int) // topic -> index in base.Topics
	for i, topic := range base.Topics {
		existingTopics[topic.Topic] = i
	}

	// Merge new topics into base
	for _, newTopic := range newConfig.Topics {
		if idx, exists := existingTopics[newTopic.Topic]; exists {
			// Merge messages into existing topic
			base.Topics[idx].Messages = append(base.Topics[idx].Messages, newTopic.Messages...)
		} else {
			// Add new topic
			base.Topics = append(base.Topics, newTopic)
		}
	}

	return base
}
