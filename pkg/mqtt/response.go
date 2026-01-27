package mqtt

import (
	"encoding/json"
	"sync"
	"time"
)

// MockResponse represents a configured mock response for MQTT
type MockResponse struct {
	ID              string `json:"id"`
	TriggerPattern  string `json:"triggerPattern"`  // MQTT pattern with wildcards
	ResponseTopic   string `json:"responseTopic"`   // Template for response topic
	PayloadTemplate string `json:"payloadTemplate"` // Template with variables
	DelayMs         int    `json:"delayMs"`         // Delay before response
	Enabled         bool   `json:"enabled"`
}

// ResponseHandler handles mock responses for published messages
type ResponseHandler struct {
	broker    *Broker
	responses []*MockResponse
	sequences *SequenceStore
	mu        sync.RWMutex
}

// NewResponseHandler creates a new response handler
func NewResponseHandler(broker *Broker) *ResponseHandler {
	return &ResponseHandler{
		broker:    broker,
		responses: make([]*MockResponse, 0),
		sequences: NewSequenceStore(),
	}
}

// SetResponses updates the list of configured responses
func (h *ResponseHandler) SetResponses(responses []*MockResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.responses = responses
}

// AddResponse adds a new mock response
func (h *ResponseHandler) AddResponse(resp *MockResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.responses = append(h.responses, resp)
}

// RemoveResponse removes a mock response by ID
func (h *ResponseHandler) RemoveResponse(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, resp := range h.responses {
		if resp.ID == id {
			h.responses = append(h.responses[:i], h.responses[i+1:]...)
			return
		}
	}
}

// GetResponses returns all configured responses
func (h *ResponseHandler) GetResponses() []*MockResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]*MockResponse, len(h.responses))
	copy(result, h.responses)
	return result
}

// HandlePublish processes a published message and triggers any matching responses
func (h *ResponseHandler) HandlePublish(topic string, payload []byte, clientID string) {
	h.mu.RLock()
	responses := make([]*MockResponse, len(h.responses))
	copy(responses, h.responses)
	h.mu.RUnlock()

	for _, resp := range responses {
		if !resp.Enabled {
			continue
		}

		if matchTopic(resp.TriggerPattern, topic) {
			go h.executeResponse(resp, topic, payload, clientID)
		}
	}
}

// executeResponse sends a mock response after optional delay
func (h *ResponseHandler) executeResponse(resp *MockResponse, triggerTopic string, payload []byte, clientID string) {
	// Apply delay if configured
	if resp.DelayMs > 0 {
		time.Sleep(time.Duration(resp.DelayMs) * time.Millisecond)
	}

	// Extract wildcard values from the trigger topic
	wildcards := ExtractWildcardValues(resp.TriggerPattern, triggerTopic)

	// Parse payload as JSON if possible
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	// Build template context
	ctx := &TemplateContext{
		Topic:        triggerTopic,
		WildcardVals: wildcards,
		ClientID:     clientID,
		Payload:      payloadMap,
	}

	// Render response topic
	responseTopic := resp.ResponseTopic
	if responseTopic == "" {
		// Default to trigger topic + "/response"
		responseTopic = triggerTopic + "/response"
	} else {
		// Substitute wildcards in topic
		responseTopic = RenderTopicTemplate(responseTopic, wildcards)
	}

	// Render payload template (MQTT-specific variables first, then shared variables)
	template := NewTemplate(resp.PayloadTemplate, h.sequences)
	responsePayload := template.Render(ctx)
	responsePayload = processSharedTemplateVars(responsePayload)

	// Prevent infinite loop: mark this topic as an active mock response.
	// If the topic is already marked, a loop has been detected.
	if !h.broker.markMockResponseTopic(responseTopic) {
		h.broker.log.Warn("skipping mock response to prevent infinite loop",
			"responseTopic", responseTopic,
			"responseID", resp.ID)
		return
	}
	defer h.broker.unmarkMockResponseTopic(responseTopic)

	// Publish the response and log any errors
	if err := h.broker.Publish(responseTopic, []byte(responsePayload), 0, false); err != nil {
		h.broker.log.Error("failed to publish mock response",
			"topic", responseTopic,
			"responseID", resp.ID,
			"error", err)
	}

	// Notify test panel sessions about the mock response
	h.broker.notifyMockResponse(responseTopic, []byte(responsePayload), resp.ID)
}

// ResponseMatch represents a matched response with context
type ResponseMatch struct {
	Response  *MockResponse
	Wildcards []string
	Topic     string
}

// FindMatchingResponses finds all responses matching a topic
func (h *ResponseHandler) FindMatchingResponses(topic string) []*ResponseMatch {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var matches []*ResponseMatch
	for _, resp := range h.responses {
		if resp.Enabled && matchTopic(resp.TriggerPattern, topic) {
			wildcards := ExtractWildcardValues(resp.TriggerPattern, topic)
			matches = append(matches, &ResponseMatch{
				Response:  resp,
				Wildcards: wildcards,
				Topic:     topic,
			})
		}
	}
	return matches
}
