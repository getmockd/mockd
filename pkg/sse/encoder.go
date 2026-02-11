package sse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Encoder handles SSE message formatting per W3C specification.
// See: https://html.spec.whatwg.org/multipage/server-sent-events.html
type Encoder struct{}

// NewEncoder creates a new SSE encoder.
func NewEncoder() *Encoder {
	return &Encoder{}
}

// FormatEvent formats an SSE event definition into wire format.
// Returns the formatted string and number of bytes.
func (e *Encoder) FormatEvent(event *SSEEventDef) (string, error) {
	if event == nil {
		return "", ErrInvalidConfig
	}

	var sb strings.Builder

	// Write comment line first if present
	if event.Comment != "" {
		comment := e.FormatComment(event.Comment)
		sb.WriteString(comment)
	}

	// Write event type if present
	if event.Type != "" {
		if strings.ContainsAny(event.Type, "\r\n") {
			return "", ErrInvalidEventID
		}
		sb.WriteString(fieldEvent)
		sb.WriteString(event.Type)
		sb.WriteByte('\n')
	}

	// Write event ID if present
	if event.ID != "" {
		if strings.ContainsAny(event.ID, "\r\n") {
			return "", ErrInvalidEventID
		}
		sb.WriteString(fieldID)
		sb.WriteString(event.ID)
		sb.WriteByte('\n')
	}

	// Write retry if present
	if event.Retry > 0 {
		sb.WriteString(fieldRetry)
		sb.WriteString(strconv.Itoa(event.Retry))
		sb.WriteByte('\n')
	}

	// Write data field (required)
	dataStr, err := e.formatData(event.Data)
	if err != nil {
		return "", err
	}

	// Check size limit
	if len(dataStr) > MaxEventDataSize {
		return "", ErrEventTooLarge
	}

	// Split multiline data into multiple data: fields
	lines := strings.Split(dataStr, "\n")
	for _, line := range lines {
		sb.WriteString(fieldData)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	// End with blank line to dispatch the event
	sb.WriteByte('\n')

	return sb.String(), nil
}

// FormatComment formats a comment line (keepalive, etc).
// Comments start with : and are ignored by EventSource clients.
func (e *Encoder) FormatComment(comment string) string {
	var sb strings.Builder

	// Handle multiline comments
	lines := strings.Split(comment, "\n")
	for _, line := range lines {
		sb.WriteString(fieldComment)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	return sb.String()
}

// FormatKeepalive returns a keepalive comment.
func (e *Encoder) FormatKeepalive() string {
	return ": keepalive\n\n"
}

// FormatRetry formats a retry interval message.
func (e *Encoder) FormatRetry(retryMs int) string {
	return fmt.Sprintf("%s%d\n\n", fieldRetry, retryMs)
}

// formatData converts event data to string format.
func (e *Encoder) formatData(data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	switch v := data.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		// JSON encode non-string data
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal event data: %w", err)
		}
		return string(jsonBytes), nil
	}
}

// FormatEventSimple creates a simple event with just data.
func (e *Encoder) FormatEventSimple(data string) string {
	return fieldData + data + "\n\n"
}

// FormatEventWithID creates an event with data and ID.
func (e *Encoder) FormatEventWithID(data, id string) string {
	return fieldID + id + "\n" + fieldData + data + "\n\n"
}

// FormatEventWithType creates an event with type and data.
func (e *Encoder) FormatEventWithType(eventType, data string) string {
	return fieldEvent + eventType + "\n" + fieldData + data + "\n\n"
}

// FormatEventFull creates an event with all fields.
func (e *Encoder) FormatEventFull(eventType, data, id string, retry int) string {
	var sb strings.Builder

	if eventType != "" {
		sb.WriteString(fieldEvent)
		sb.WriteString(eventType)
		sb.WriteByte('\n')
	}

	if id != "" {
		sb.WriteString(fieldID)
		sb.WriteString(id)
		sb.WriteByte('\n')
	}

	if retry > 0 {
		sb.WriteString(fieldRetry)
		sb.WriteString(strconv.Itoa(retry))
		sb.WriteByte('\n')
	}

	// Handle multiline data
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		sb.WriteString(fieldData)
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	return sb.String()
}
