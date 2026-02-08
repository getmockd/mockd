// Package recording provides storage for SOAP recordings.
package recording

import (
	"time"
)

// SOAPStore provides storage for SOAP recordings.
type SOAPStore struct {
	*RecordingStore[SOAPRecording]
}

// NewSOAPStore creates a new SOAP recording store.
func NewSOAPStore(maxSize int) *SOAPStore {
	return &SOAPStore{NewRecordingStore[SOAPRecording](maxSize, "soap_")}
}

// NewSOAPStoreWithDir creates a new SOAP store with persistent storage.
func NewSOAPStoreWithDir(maxSize int, dataDir string) (*SOAPStore, error) {
	s, err := NewRecordingStoreWithDir[SOAPRecording](maxSize, dataDir, "soap_")
	if err != nil {
		return nil, err
	}
	return &SOAPStore{s}, nil
}

// List returns recordings matching the filter.
func (s *SOAPStore) List(filter SOAPRecordingFilter) ([]*SOAPRecording, int) {
	return s.ListFiltered(func(r *SOAPRecording) bool {
		if filter.Endpoint != "" && r.Endpoint != filter.Endpoint {
			return false
		}
		if filter.Operation != "" && r.Operation != filter.Operation {
			return false
		}
		if filter.SOAPAction != "" && r.SOAPAction != filter.SOAPAction {
			return false
		}
		if filter.HasFault != nil {
			if *filter.HasFault != r.HasFault {
				return false
			}
		}
		return true
	}, filter.Offset, filter.Limit)
}

// ListByEndpoint returns recordings for a specific endpoint.
func (s *SOAPStore) ListByEndpoint(endpoint string) []*SOAPRecording {
	recordings, _ := s.List(SOAPRecordingFilter{Endpoint: endpoint})
	return recordings
}

// ListByOperation returns recordings for a specific operation.
func (s *SOAPStore) ListByOperation(operation string) []*SOAPRecording {
	recordings, _ := s.List(SOAPRecordingFilter{Operation: operation})
	return recordings
}

// SOAPRecordingStats contains statistics about SOAP recordings.
type SOAPRecordingStats struct {
	TotalRecordings int            `json:"totalRecordings"`
	ByEndpoint      map[string]int `json:"byEndpoint"`
	ByOperation     map[string]int `json:"byOperation"`
	FaultCount      int            `json:"faultCount"`
	OldestTimestamp *time.Time     `json:"oldestTimestamp,omitempty"`
	NewestTimestamp *time.Time     `json:"newestTimestamp,omitempty"`
}

// Stats returns statistics about the recordings.
func (s *SOAPStore) Stats() *SOAPRecordingStats {
	all := s.All()

	stats := &SOAPRecordingStats{
		TotalRecordings: len(all),
		ByEndpoint:      make(map[string]int),
		ByOperation:     make(map[string]int),
	}

	for _, r := range all {
		stats.ByEndpoint[r.Endpoint]++
		stats.ByOperation[r.Operation]++

		if r.HasFault {
			stats.FaultCount++
		}

		if stats.OldestTimestamp == nil || r.Timestamp.Before(*stats.OldestTimestamp) {
			t := r.Timestamp
			stats.OldestTimestamp = &t
		}
		if stats.NewestTimestamp == nil || r.Timestamp.After(*stats.NewestTimestamp) {
			t := r.Timestamp
			stats.NewestTimestamp = &t
		}
	}

	return stats
}
