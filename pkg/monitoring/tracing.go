/*
Package monitoring provides OpenTelemetry tracing for Kafka operations.

This package provides distributed tracing capabilities to track
message flow across services and identify performance bottlenecks.
*/
package monitoring

import (
	"context"
	"sync"
	"time"
)

// SpanKind defines the type of span.
type SpanKind string

const (
	SpanKindProducer SpanKind = "producer"
	SpanKindConsumer SpanKind = "consumer"
	SpanKindInternal SpanKind = "internal"
)

// SpanStatus represents the status of a span.
type SpanStatus string

const (
	SpanStatusOK    SpanStatus = "OK"
	SpanStatusError SpanStatus = "ERROR"
)

// SpanContext contains trace context information.
type SpanContext struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Sampled    bool
}

// SpanData represents the data collected for a span.
type SpanData struct {
	Name       string
	Kind       SpanKind
	StartTime  time.Time
	EndTime    time.Time
	Status     SpanStatus
	Context    SpanContext
	Attributes map[string]interface{}
	Events     []SpanEvent
	Error      error
}

// SpanEvent represents an event within a span.
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]interface{}
}

// Span represents an active tracing span.
type Span interface {
	// SetTag sets an attribute on the span.
	SetTag(key string, value interface{})
	
	// SetError records an error on the span.
	SetError(err error)
	
	// AddEvent adds an event to the span.
	AddEvent(name string, attrs map[string]interface{})
	
	// Finish completes the span.
	Finish()
	
	// Context returns the span context.
	Context() SpanContext
}

// Tracer provides span creation and management.
type Tracer interface {
	// StartSpan creates a new span.
	StartSpan(ctx context.Context, operationName string) (context.Context, Span)
	
	// StartSpanWithKind creates a new span with a specific kind.
	StartSpanWithKind(ctx context.Context, operationName string, kind SpanKind) (context.Context, Span)
	
	// Extract extracts span context from carrier.
	Extract(carrier map[string]string) SpanContext
	
	// Inject injects span context into carrier.
	Inject(ctx context.Context, carrier map[string]string)
}

// NoopTracer provides a no-operation tracer for testing.
type NoopTracer struct{}

// NoopSpan provides a no-operation span.
type NoopSpan struct {
	ctx SpanContext
}

// NewNoopTracer creates a new no-op tracer.
func NewNoopTracer() *NoopTracer {
	return &NoopTracer{}
}

// StartSpan implements Tracer.
func (t *NoopTracer) StartSpan(ctx context.Context, operationName string) (context.Context, Span) {
	return ctx, &NoopSpan{}
}

// StartSpanWithKind implements Tracer.
func (t *NoopTracer) StartSpanWithKind(ctx context.Context, operationName string, kind SpanKind) (context.Context, Span) {
	return ctx, &NoopSpan{}
}

// Extract implements Tracer.
func (t *NoopTracer) Extract(carrier map[string]string) SpanContext {
	return SpanContext{}
}

// Inject implements Tracer.
func (t *NoopTracer) Inject(ctx context.Context, carrier map[string]string) {}

// SetTag implements Span.
func (s *NoopSpan) SetTag(key string, value interface{}) {}

// SetError implements Span.
func (s *NoopSpan) SetError(err error) {}

// AddEvent implements Span.
func (s *NoopSpan) AddEvent(name string, attrs map[string]interface{}) {}

// Finish implements Span.
func (s *NoopSpan) Finish() {}

// Context implements Span.
func (s *NoopSpan) Context() SpanContext {
	return s.ctx
}

// InMemoryTracer stores spans in memory for testing/development.
type InMemoryTracer struct {
	mu       sync.RWMutex
	spans    []SpanData
	maxSpans int
}

// InMemorySpan is an in-memory span implementation.
type InMemorySpan struct {
	tracer     *InMemoryTracer
	data       SpanData
	mu         sync.Mutex
}

// NewInMemoryTracer creates a new in-memory tracer.
func NewInMemoryTracer(maxSpans int) *InMemoryTracer {
	return &InMemoryTracer{
		spans:    make([]SpanData, 0, maxSpans),
		maxSpans: maxSpans,
	}
}

// StartSpan implements Tracer.
func (t *InMemoryTracer) StartSpan(ctx context.Context, operationName string) (context.Context, Span) {
	return t.StartSpanWithKind(ctx, operationName, SpanKindInternal)
}

// StartSpanWithKind implements Tracer.
func (t *InMemoryTracer) StartSpanWithKind(ctx context.Context, operationName string, kind SpanKind) (context.Context, Span) {
	span := &InMemorySpan{
		tracer: t,
		data: SpanData{
			Name:       operationName,
			Kind:       kind,
			StartTime:  time.Now(),
			Status:     SpanStatusOK,
			Context:    t.generateContext(ctx),
			Attributes: make(map[string]interface{}),
			Events:     make([]SpanEvent, 0),
		},
	}
	return ctx, span
}

// generateContext creates a new span context.
func (t *InMemoryTracer) generateContext(ctx context.Context) SpanContext {
	return SpanContext{
		TraceID: generateID(),
		SpanID:  generateID(),
		Sampled: true,
	}
}

// generateID generates a random trace/span ID.
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// Extract implements Tracer.
func (t *InMemoryTracer) Extract(carrier map[string]string) SpanContext {
	return SpanContext{
		TraceID:  carrier["trace-id"],
		SpanID:   carrier["span-id"],
		ParentID: carrier["parent-id"],
		Sampled:  carrier["sampled"] == "1",
	}
}

// Inject implements Tracer.
func (t *InMemoryTracer) Inject(ctx context.Context, carrier map[string]string) {
	// In a real implementation, extract span from context and inject headers
}

// GetSpans returns all recorded spans.
func (t *InMemoryTracer) GetSpans() []SpanData {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]SpanData, len(t.spans))
	copy(result, t.spans)
	return result
}

// Clear removes all recorded spans.
func (t *InMemoryTracer) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spans = t.spans[:0]
}

// recordSpan adds a completed span to storage.
func (t *InMemoryTracer) recordSpan(data SpanData) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.spans) >= t.maxSpans {
		// Remove oldest span
		t.spans = t.spans[1:]
	}
	t.spans = append(t.spans, data)
}

// SetTag implements Span.
func (s *InMemorySpan) SetTag(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Attributes[key] = value
}

// SetError implements Span.
func (s *InMemorySpan) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Error = err
	s.data.Status = SpanStatusError
}

// AddEvent implements Span.
func (s *InMemorySpan) AddEvent(name string, attrs map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Events = append(s.data.Events, SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

// Finish implements Span.
func (s *InMemorySpan) Finish() {
	s.mu.Lock()
	s.data.EndTime = time.Now()
	data := s.data
	s.mu.Unlock()
	
	s.tracer.recordSpan(data)
}

// Context implements Span.
func (s *InMemorySpan) Context() SpanContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Context
}

// KafkaSpanAttributes returns standard Kafka span attributes.
func KafkaSpanAttributes(topic string, partition int32, offset int64) map[string]interface{} {
	return map[string]interface{}{
		"messaging.system":               "kafka",
		"messaging.destination":          topic,
		"messaging.kafka.partition":      partition,
		"messaging.kafka.offset":         offset,
		"messaging.operation":            "receive",
	}
}
