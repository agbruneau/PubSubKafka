/*
Package dlq implements the Dead Letter Queue pattern for Kafka message processing.

The Dead Letter Queue (DLQ) pattern provides a mechanism to handle messages that
cannot be processed successfully after multiple retry attempts. Instead of losing
these messages or blocking the processing pipeline, they are routed to a dedicated
"dead letter" topic for later analysis, debugging, and potential reprocessing.

Key Features:
  - Configurable retry attempts with exponential backoff
  - Automatic routing of failed messages to DLQ topic
  - Rich failure context for debugging and audit
  - Thread-safe operations with proper cleanup
  - Support for manual message reprocessing

Usage:

	handler := dlq.NewHandler(dlq.Config{
	    Broker:     "localhost:9092",
	    DLQTopic:   "orders-dlq",
	    MaxRetries: 3,
	})
	defer handler.Close()

	// Process with automatic DLQ routing on failure
	err := handler.ProcessWithRetry(ctx, message, processFunc)
*/
package dlq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"kafka-demo/pkg/models"
)

// Common errors
var (
	ErrHandlerClosed   = errors.New("dlq handler is closed")
	ErrNilMessage      = errors.New("message cannot be nil")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	ErrProducerNotReady   = errors.New("dlq producer not ready")
)

// Config holds the configuration for the DLQ handler.
type Config struct {
	// Broker is the Kafka broker address.
	Broker string
	// DLQTopic is the destination topic for failed messages.
	DLQTopic string
	// MaxRetries is the maximum number of processing attempts.
	MaxRetries int
	// BaseDelay is the initial delay for exponential backoff.
	BaseDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// FlushTimeout is the timeout for flushing the producer.
	FlushTimeout time.Duration
	// ConsumerGroup identifies the consumer group for tracking.
	ConsumerGroup string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Broker:        models.DefaultKafkaBroker,
		DLQTopic:      models.DLQTopic,
		MaxRetries:    models.DLQMaxRetries,
		BaseDelay:     models.DLQRetryBaseDelay,
		MaxDelay:      models.DLQRetryMaxDelay,
		FlushTimeout:  models.DLQFlushTimeout,
		ConsumerGroup: models.DefaultConsumerGroup,
	}
}

// MessageContext contains the original Kafka message metadata.
type MessageContext struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Timestamp time.Time
}

// ProcessFunc is the function signature for message processing.
// It should return nil on success, or an error on failure.
type ProcessFunc func(ctx context.Context, msg *MessageContext) error

// Producer is an interface for sending messages to Kafka.
// This allows for easy mocking in tests.
type Producer interface {
	// Produce sends a message to the specified topic.
	Produce(topic string, key, value []byte) error
	// Flush waits for all messages to be delivered.
	Flush(timeout time.Duration) error
	// Close closes the producer.
	Close()
}

// Handler manages the Dead Letter Queue operations.
type Handler struct {
	config   Config
	producer Producer
	mu       sync.RWMutex
	closed   bool
	hostname string

	// Metrics
	metrics *Metrics
}

// Metrics tracks DLQ handler statistics.
type Metrics struct {
	mu                sync.RWMutex
	MessagesProcessed int64
	MessagesFailed    int64
	MessagesSentToDLQ int64
	TotalRetries      int64
	LastError         string
	LastErrorTime     time.Time
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementProcessed increments the processed counter.
func (m *Metrics) IncrementProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesProcessed++
}

// IncrementFailed increments the failed counter and records the error.
func (m *Metrics) IncrementFailed(err string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesFailed++
	m.LastError = err
	m.LastErrorTime = time.Now()
}

// IncrementDLQ increments the DLQ sent counter.
func (m *Metrics) IncrementDLQ() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesSentToDLQ++
}

// IncrementRetries increments the total retries counter.
func (m *Metrics) IncrementRetries() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRetries++
}

// Snapshot returns a copy of the current metrics.
func (m *Metrics) Snapshot() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Metrics{
		MessagesProcessed: m.MessagesProcessed,
		MessagesFailed:    m.MessagesFailed,
		MessagesSentToDLQ: m.MessagesSentToDLQ,
		TotalRetries:      m.TotalRetries,
		LastError:         m.LastError,
		LastErrorTime:     m.LastErrorTime,
	}
}

// NewHandler creates a new DLQ Handler with the given configuration.
// The producer parameter can be nil for testing purposes.
func NewHandler(config Config, producer Producer) *Handler {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &Handler{
		config:   config,
		producer: producer,
		hostname: hostname,
		metrics:  NewMetrics(),
	}
}

// ProcessWithRetry attempts to process a message with automatic retries.
// If all retries fail, the message is sent to the DLQ.
func (h *Handler) ProcessWithRetry(
	ctx context.Context,
	msg *MessageContext,
	process ProcessFunc,
) error {
	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return ErrHandlerClosed
	}
	h.mu.RUnlock()

	if msg == nil {
		return ErrNilMessage
	}

	var lastErr error
	firstFailure := time.Now()

	for attempt := 0; attempt <= h.config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return h.sendToDLQ(msg, models.FailureReasonTimeout, ctx.Err().Error(), attempt, firstFailure)
		default:
		}

		// Attempt processing
		lastErr = process(ctx, msg)
		if lastErr == nil {
			h.metrics.IncrementProcessed()
			return nil
		}

		// Log retry attempt
		if attempt < h.config.MaxRetries {
			h.metrics.IncrementRetries()
			delay := h.calculateBackoff(attempt)
			
			select {
			case <-ctx.Done():
				return h.sendToDLQ(msg, models.FailureReasonTimeout, ctx.Err().Error(), attempt+1, firstFailure)
			case <-time.After(delay):
				// Continue to next retry
			}
		}
	}

	// All retries exhausted, send to DLQ
	reason := classifyError(lastErr)
	return h.sendToDLQ(msg, reason, lastErr.Error(), h.config.MaxRetries+1, firstFailure)
}

// sendToDLQ sends a failed message to the Dead Letter Queue.
func (h *Handler) sendToDLQ(
	msg *MessageContext,
	reason models.FailureReason,
	errorMsg string,
	attempts int,
	firstFailure time.Time,
) error {
	h.mu.RLock()
	if h.producer == nil {
		h.mu.RUnlock()
		h.metrics.IncrementFailed(errorMsg)
		return ErrProducerNotReady
	}
	h.mu.RUnlock()

	dlqMsg := models.NewDeadLetterMessage(
		msg.Topic,
		msg.Partition,
		msg.Offset,
		string(msg.Value),
		reason,
		errorMsg,
	)

	dlqMsg.OriginalKey = string(msg.Key)
	dlqMsg.OriginalTimestamp = msg.Timestamp.Format(time.RFC3339)
	dlqMsg.ConsumerGroup = h.config.ConsumerGroup
	dlqMsg.ProcessingHost = h.hostname
	dlqMsg.RetryCount = attempts
	dlqMsg.FirstFailure = firstFailure.Format(time.RFC3339)
	dlqMsg.LastFailure = time.Now().Format(time.RFC3339)

	payload, err := json.Marshal(dlqMsg)
	if err != nil {
		h.metrics.IncrementFailed(fmt.Sprintf("marshal error: %v", err))
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	if err := h.producer.Produce(h.config.DLQTopic, msg.Key, payload); err != nil {
		h.metrics.IncrementFailed(fmt.Sprintf("produce error: %v", err))
		return fmt.Errorf("failed to send to DLQ: %w", err)
	}

	h.metrics.IncrementDLQ()
	h.metrics.IncrementFailed(errorMsg)
	return nil
}

// SendToDLQ directly sends a message to the DLQ without retries.
// Useful for validation errors that should not be retried.
func (h *Handler) SendToDLQ(
	msg *MessageContext,
	reason models.FailureReason,
	errorMsg string,
) error {
	return h.sendToDLQ(msg, reason, errorMsg, 0, time.Now())
}

// calculateBackoff returns the delay for the given attempt using exponential backoff.
func (h *Handler) calculateBackoff(attempt int) time.Duration {
	delay := h.config.BaseDelay * (1 << uint(attempt))
	if delay > h.config.MaxDelay {
		delay = h.config.MaxDelay
	}
	return delay
}

// Metrics returns the current handler metrics.
func (h *Handler) Metrics() Metrics {
	return h.metrics.Snapshot()
}

// Flush flushes any pending messages in the producer.
func (h *Handler) Flush() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrHandlerClosed
	}
	if h.producer == nil {
		return nil
	}

	return h.producer.Flush(h.config.FlushTimeout)
}

// Close closes the handler and releases resources.
func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true
	if h.producer != nil {
		h.producer.Close()
	}

	return nil
}

// IsClosed returns true if the handler has been closed.
func (h *Handler) IsClosed() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.closed
}

// classifyError determines the FailureReason based on the error type.
func classifyError(err error) models.FailureReason {
	if err == nil {
		return models.FailureReasonUnknown
	}

	// Check for validation errors first
	if errors.Is(err, models.ErrEmptyOrderID) ||
		errors.Is(err, models.ErrInvalidSequence) ||
		errors.Is(err, models.ErrEmptyStatus) ||
		errors.Is(err, models.ErrNoItems) ||
		errors.Is(err, models.ErrInvalidTotal) ||
		errors.Is(err, models.ErrEmptyCustomerID) ||
		errors.Is(err, models.ErrEmptyCustomerName) ||
		errors.Is(err, models.ErrInvalidEmail) ||
		errors.Is(err, models.ErrEmptyItemID) ||
		errors.Is(err, models.ErrEmptyItemName) ||
		errors.Is(err, models.ErrInvalidQuantity) ||
		errors.Is(err, models.ErrInvalidPrice) ||
		errors.Is(err, models.ErrInvalidItemTotal) {
		return models.FailureReasonValidation
	}

	// Check for JSON/deserialization errors by type (before calling Error())
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return models.FailureReasonDeserialization
	}

	// Now safe to get error message for pattern matching
	errMsg := err.Error()

	// Check error message for common patterns
	switch {
	case contains(errMsg, "json", "unmarshal", "deserialize", "parse"):
		return models.FailureReasonDeserialization
	case contains(errMsg, "validation", "invalid", "required"):
		return models.FailureReasonValidation
	case contains(errMsg, "timeout", "deadline"):
		return models.FailureReasonTimeout
	default:
		return models.FailureReasonProcessing
	}
}

// contains checks if the message contains any of the keywords (case-insensitive).
func contains(msg string, keywords ...string) bool {
	lower := stringToLower(msg)
	for _, kw := range keywords {
		if stringContains(lower, stringToLower(kw)) {
			return true
		}
	}
	return false
}

// stringToLower converts a string to lowercase without importing strings.
func stringToLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// stringContains checks if s contains substr.
func stringContains(s, substr string) bool {
	return len(substr) <= len(s) && findSubstring(s, substr) >= 0
}

// findSubstring returns the index of the first occurrence of substr in s, or -1.
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
