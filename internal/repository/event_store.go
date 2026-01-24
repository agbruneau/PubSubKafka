/*
Package repository provides data access layer implementations.

This file implements the Event Store pattern for persisting and
querying events from the Kafka message flow.
*/
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Common errors
var (
	ErrEventNotFound     = errors.New("event not found")
	ErrDuplicateEvent    = errors.New("duplicate event ID")
	ErrStoreClosed       = errors.New("event store is closed")
	ErrInvalidEventID    = errors.New("invalid event ID")
)

// StoredEvent represents an event persisted in the store.
type StoredEvent struct {
	ID            string          `json:"id"`
	AggregateID   string          `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	EventType     string          `json:"event_type"`
	Version       int             `json:"version"`
	Timestamp     time.Time       `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      EventMetadata   `json:"metadata"`
}

// EventMetadata contains metadata about the event.
type EventMetadata struct {
	CorrelationID string            `json:"correlation_id"`
	CausationID   string            `json:"causation_id"`
	Source        string            `json:"source"`
	UserID        string            `json:"user_id,omitempty"`
	TraceID       string            `json:"trace_id,omitempty"`
	SpanID        string            `json:"span_id,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// EventQuery defines criteria for querying events.
type EventQuery struct {
	AggregateID   string
	AggregateType string
	EventTypes    []string
	FromVersion   int
	ToVersion     int
	FromTime      time.Time
	ToTime        time.Time
	Limit         int
	Offset        int
}

// EventStore defines the interface for event persistence.
type EventStore interface {
	// Append adds events to the store.
	Append(ctx context.Context, events ...StoredEvent) error

	// Get retrieves an event by ID.
	Get(ctx context.Context, eventID string) (*StoredEvent, error)

	// Query retrieves events matching the query criteria.
	Query(ctx context.Context, query EventQuery) ([]StoredEvent, error)

	// GetByAggregate retrieves all events for an aggregate.
	GetByAggregate(ctx context.Context, aggregateID string) ([]StoredEvent, error)

	// GetLatestVersion returns the latest version for an aggregate.
	GetLatestVersion(ctx context.Context, aggregateID string) (int, error)

	// Close closes the store.
	Close() error
}

// InMemoryEventStore provides an in-memory event store implementation.
// Suitable for testing and development.
type InMemoryEventStore struct {
	mu       sync.RWMutex
	events   map[string]StoredEvent           // eventID -> event
	byAggregate map[string][]string           // aggregateID -> []eventID
	closed   bool
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:      make(map[string]StoredEvent),
		byAggregate: make(map[string][]string),
	}
}

// Append adds events to the store.
func (s *InMemoryEventStore) Append(ctx context.Context, events ...StoredEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStoreClosed
	}

	for _, event := range events {
		if event.ID == "" {
			return ErrInvalidEventID
		}

		if _, exists := s.events[event.ID]; exists {
			return ErrDuplicateEvent
		}

		s.events[event.ID] = event
		s.byAggregate[event.AggregateID] = append(s.byAggregate[event.AggregateID], event.ID)
	}

	return nil
}

// Get retrieves an event by ID.
func (s *InMemoryEventStore) Get(ctx context.Context, eventID string) (*StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	event, exists := s.events[eventID]
	if !exists {
		return nil, ErrEventNotFound
	}

	return &event, nil
}

// Query retrieves events matching the query criteria.
func (s *InMemoryEventStore) Query(ctx context.Context, query EventQuery) ([]StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	var results []StoredEvent

	for _, event := range s.events {
		if s.matchesQuery(event, query) {
			results = append(results, event)
		}
	}

	// Apply offset and limit
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, nil
}

// matchesQuery checks if an event matches the query criteria.
func (s *InMemoryEventStore) matchesQuery(event StoredEvent, query EventQuery) bool {
	if query.AggregateID != "" && event.AggregateID != query.AggregateID {
		return false
	}

	if query.AggregateType != "" && event.AggregateType != query.AggregateType {
		return false
	}

	if len(query.EventTypes) > 0 {
		found := false
		for _, t := range query.EventTypes {
			if event.EventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if query.FromVersion > 0 && event.Version < query.FromVersion {
		return false
	}

	if query.ToVersion > 0 && event.Version > query.ToVersion {
		return false
	}

	if !query.FromTime.IsZero() && event.Timestamp.Before(query.FromTime) {
		return false
	}

	if !query.ToTime.IsZero() && event.Timestamp.After(query.ToTime) {
		return false
	}

	return true
}

// GetByAggregate retrieves all events for an aggregate.
func (s *InMemoryEventStore) GetByAggregate(ctx context.Context, aggregateID string) ([]StoredEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	eventIDs, exists := s.byAggregate[aggregateID]
	if !exists {
		return []StoredEvent{}, nil
	}

	results := make([]StoredEvent, 0, len(eventIDs))
	for _, id := range eventIDs {
		if event, ok := s.events[id]; ok {
			results = append(results, event)
		}
	}

	return results, nil
}

// GetLatestVersion returns the latest version for an aggregate.
func (s *InMemoryEventStore) GetLatestVersion(ctx context.Context, aggregateID string) (int, error) {
	events, err := s.GetByAggregate(ctx, aggregateID)
	if err != nil {
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil
	}

	maxVersion := 0
	for _, event := range events {
		if event.Version > maxVersion {
			maxVersion = event.Version
		}
	}

	return maxVersion, nil
}

// Close closes the store.
func (s *InMemoryEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.events = nil
	s.byAggregate = nil

	return nil
}

// Count returns the total number of events in the store.
func (s *InMemoryEventStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// Clear removes all events from the store.
func (s *InMemoryEventStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = make(map[string]StoredEvent)
	s.byAggregate = make(map[string][]string)
}
