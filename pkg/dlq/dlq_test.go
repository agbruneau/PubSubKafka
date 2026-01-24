package dlq

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"kafka-demo/pkg/models"
)

// MockProducer is a test double for the Producer interface.
type MockProducer struct {
	mu           sync.Mutex
	messages     []ProducedMessage
	produceErr   error
	flushErr     error
	closed       bool
	produceDelay time.Duration
}

// ProducedMessage represents a message sent to the mock producer.
type ProducedMessage struct {
	Topic string
	Key   []byte
	Value []byte
}

// NewMockProducer creates a new MockProducer.
func NewMockProducer() *MockProducer {
	return &MockProducer{
		messages: make([]ProducedMessage, 0),
	}
}

func (m *MockProducer) Produce(topic string, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.produceDelay > 0 {
		time.Sleep(m.produceDelay)
	}

	if m.produceErr != nil {
		return m.produceErr
	}

	m.messages = append(m.messages, ProducedMessage{
		Topic: topic,
		Key:   key,
		Value: value,
	})
	return nil
}

func (m *MockProducer) Flush(timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushErr
}

func (m *MockProducer) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

func (m *MockProducer) Messages() []ProducedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ProducedMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *MockProducer) SetProduceError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.produceErr = err
}

func (m *MockProducer) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// TestNewHandler tests handler creation.
func TestNewHandler(t *testing.T) {
	config := DefaultConfig()
	producer := NewMockProducer()

	handler := NewHandler(config, producer)
	defer handler.Close()

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.IsClosed() {
		t.Error("New handler should not be closed")
	}

	if handler.config.MaxRetries != models.DLQMaxRetries {
		t.Errorf("Expected MaxRetries=%d, got %d", models.DLQMaxRetries, handler.config.MaxRetries)
	}
}

// TestDefaultConfig tests default configuration values.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Broker != models.DefaultKafkaBroker {
		t.Errorf("Expected Broker=%s, got %s", models.DefaultKafkaBroker, config.Broker)
	}
	if config.DLQTopic != models.DLQTopic {
		t.Errorf("Expected DLQTopic=%s, got %s", models.DLQTopic, config.DLQTopic)
	}
	if config.MaxRetries != models.DLQMaxRetries {
		t.Errorf("Expected MaxRetries=%d, got %d", models.DLQMaxRetries, config.MaxRetries)
	}
}

// TestProcessWithRetry_Success tests successful processing on first attempt.
func TestProcessWithRetry_Success(t *testing.T) {
	producer := NewMockProducer()
	handler := NewHandler(DefaultConfig(), producer)
	defer handler.Close()

	msg := &MessageContext{
		Topic:     "orders",
		Partition: 0,
		Offset:    100,
		Key:       []byte("key-1"),
		Value:     []byte(`{"order_id": "123"}`),
		Timestamp: time.Now(),
	}

	processFunc := func(ctx context.Context, m *MessageContext) error {
		return nil // Success
	}

	err := handler.ProcessWithRetry(context.Background(), msg, processFunc)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	metrics := handler.Metrics()
	if metrics.MessagesProcessed != 1 {
		t.Errorf("Expected 1 processed message, got %d", metrics.MessagesProcessed)
	}
	if metrics.MessagesSentToDLQ != 0 {
		t.Errorf("Expected 0 DLQ messages, got %d", metrics.MessagesSentToDLQ)
	}
}

// TestProcessWithRetry_FailThenSuccess tests retry logic with eventual success.
func TestProcessWithRetry_FailThenSuccess(t *testing.T) {
	config := DefaultConfig()
	config.BaseDelay = 1 * time.Millisecond // Speed up test
	config.MaxRetries = 3

	producer := NewMockProducer()
	handler := NewHandler(config, producer)
	defer handler.Close()

	msg := &MessageContext{
		Topic:     "orders",
		Partition: 0,
		Offset:    100,
		Value:     []byte(`{"order_id": "123"}`),
		Timestamp: time.Now(),
	}

	attempts := 0
	processFunc := func(ctx context.Context, m *MessageContext) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient error")
		}
		return nil // Success on 3rd attempt
	}

	err := handler.ProcessWithRetry(context.Background(), msg, processFunc)
	if err != nil {
		t.Errorf("Expected no error after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	metrics := handler.Metrics()
	if metrics.TotalRetries != 2 {
		t.Errorf("Expected 2 retries, got %d", metrics.TotalRetries)
	}
}

// TestProcessWithRetry_MaxRetriesExceeded tests DLQ routing after max retries.
func TestProcessWithRetry_MaxRetriesExceeded(t *testing.T) {
	config := DefaultConfig()
	config.BaseDelay = 1 * time.Millisecond
	config.MaxRetries = 2

	producer := NewMockProducer()
	handler := NewHandler(config, producer)
	defer handler.Close()

	msg := &MessageContext{
		Topic:     "orders",
		Partition: 0,
		Offset:    100,
		Key:       []byte("order-456"),
		Value:     []byte(`{"order_id": "456"}`),
		Timestamp: time.Now(),
	}

	attempts := 0
	processFunc := func(ctx context.Context, m *MessageContext) error {
		attempts++
		return errors.New("persistent error")
	}

	err := handler.ProcessWithRetry(context.Background(), msg, processFunc)
	if err != nil {
		t.Errorf("Expected nil error (message sent to DLQ), got: %v", err)
	}

	// Should have tried MaxRetries + 1 times (initial + retries)
	expectedAttempts := config.MaxRetries + 1
	if attempts != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, attempts)
	}

	// Verify message was sent to DLQ
	messages := producer.Messages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 DLQ message, got %d", len(messages))
	}

	if messages[0].Topic != models.DLQTopic {
		t.Errorf("Expected topic %s, got %s", models.DLQTopic, messages[0].Topic)
	}

	// Verify DLQ message content
	var dlqMsg models.DeadLetterMessage
	if err := json.Unmarshal(messages[0].Value, &dlqMsg); err != nil {
		t.Fatalf("Failed to unmarshal DLQ message: %v", err)
	}

	if dlqMsg.OriginalTopic != "orders" {
		t.Errorf("Expected OriginalTopic=orders, got %s", dlqMsg.OriginalTopic)
	}
	if dlqMsg.OriginalOffset != 100 {
		t.Errorf("Expected OriginalOffset=100, got %d", dlqMsg.OriginalOffset)
	}
	if dlqMsg.RetryCount != expectedAttempts {
		t.Errorf("Expected RetryCount=%d, got %d", expectedAttempts, dlqMsg.RetryCount)
	}
	if dlqMsg.FailureReason != models.FailureReasonProcessing {
		t.Errorf("Expected FailureReason=%s, got %s", models.FailureReasonProcessing, dlqMsg.FailureReason)
	}
}

// TestProcessWithRetry_NilMessage tests error on nil message.
func TestProcessWithRetry_NilMessage(t *testing.T) {
	handler := NewHandler(DefaultConfig(), NewMockProducer())
	defer handler.Close()

	err := handler.ProcessWithRetry(context.Background(), nil, func(ctx context.Context, m *MessageContext) error {
		return nil
	})

	if !errors.Is(err, ErrNilMessage) {
		t.Errorf("Expected ErrNilMessage, got: %v", err)
	}
}

// TestProcessWithRetry_ClosedHandler tests error on closed handler.
func TestProcessWithRetry_ClosedHandler(t *testing.T) {
	handler := NewHandler(DefaultConfig(), NewMockProducer())
	handler.Close()

	msg := &MessageContext{
		Topic: "orders",
		Value: []byte(`{}`),
	}

	err := handler.ProcessWithRetry(context.Background(), msg, func(ctx context.Context, m *MessageContext) error {
		return nil
	})

	if !errors.Is(err, ErrHandlerClosed) {
		t.Errorf("Expected ErrHandlerClosed, got: %v", err)
	}
}

// TestProcessWithRetry_ContextCancellation tests context cancellation during retries.
func TestProcessWithRetry_ContextCancellation(t *testing.T) {
	config := DefaultConfig()
	config.BaseDelay = 100 * time.Millisecond
	config.MaxRetries = 5

	producer := NewMockProducer()
	handler := NewHandler(config, producer)
	defer handler.Close()

	msg := &MessageContext{
		Topic:     "orders",
		Value:     []byte(`{"order_id": "123"}`),
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	processFunc := func(ctx context.Context, m *MessageContext) error {
		attempts++
		return errors.New("always fails")
	}

	err := handler.ProcessWithRetry(ctx, msg, processFunc)
	// Should send to DLQ due to timeout
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		// Message should be in DLQ
	}

	// Verify we didn't complete all retries
	if attempts > 2 {
		t.Errorf("Expected fewer attempts due to context cancellation, got %d", attempts)
	}
}

// TestSendToDLQ_Direct tests direct DLQ routing without retries.
func TestSendToDLQ_Direct(t *testing.T) {
	producer := NewMockProducer()
	handler := NewHandler(DefaultConfig(), producer)
	defer handler.Close()

	msg := &MessageContext{
		Topic:     "orders",
		Partition: 2,
		Offset:    500,
		Key:       []byte("order-789"),
		Value:     []byte(`{"order_id": "789", "invalid": true}`),
		Timestamp: time.Now(),
	}

	err := handler.SendToDLQ(msg, models.FailureReasonValidation, "Order validation failed")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	messages := producer.Messages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 DLQ message, got %d", len(messages))
	}

	var dlqMsg models.DeadLetterMessage
	if err := json.Unmarshal(messages[0].Value, &dlqMsg); err != nil {
		t.Fatalf("Failed to unmarshal DLQ message: %v", err)
	}

	if dlqMsg.FailureReason != models.FailureReasonValidation {
		t.Errorf("Expected FailureReason=%s, got %s", models.FailureReasonValidation, dlqMsg.FailureReason)
	}
	if dlqMsg.ErrorMessage != "Order validation failed" {
		t.Errorf("Expected ErrorMessage='Order validation failed', got '%s'", dlqMsg.ErrorMessage)
	}
}

// TestSendToDLQ_ProducerError tests handling of producer errors.
func TestSendToDLQ_ProducerError(t *testing.T) {
	producer := NewMockProducer()
	producer.SetProduceError(errors.New("kafka unavailable"))

	handler := NewHandler(DefaultConfig(), producer)
	defer handler.Close()

	msg := &MessageContext{
		Topic: "orders",
		Value: []byte(`{}`),
	}

	err := handler.SendToDLQ(msg, models.FailureReasonProcessing, "error")
	if err == nil {
		t.Error("Expected error when producer fails")
	}
}

// TestClassifyError tests error classification logic.
func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected models.FailureReason
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: models.FailureReasonUnknown,
		},
		{
			name:     "Validation error - empty order ID",
			err:      models.ErrEmptyOrderID,
			expected: models.FailureReasonValidation,
		},
		{
			name:     "Validation error - invalid email",
			err:      models.ErrInvalidEmail,
			expected: models.FailureReasonValidation,
		},
		{
			name:     "JSON syntax error",
			err:      &json.SyntaxError{Offset: 10},
			expected: models.FailureReasonDeserialization,
		},
		{
			name:     "JSON unmarshal type error",
			err:      &json.UnmarshalTypeError{Value: "string", Field: "test"},
			expected: models.FailureReasonDeserialization,
		},
		{
			name:     "Error message contains 'json'",
			err:      errors.New("failed to json unmarshal"),
			expected: models.FailureReasonDeserialization,
		},
		{
			name:     "Error message contains 'timeout'",
			err:      errors.New("operation timeout"),
			expected: models.FailureReasonTimeout,
		},
		{
			name:     "Generic processing error",
			err:      errors.New("database connection failed"),
			expected: models.FailureReasonProcessing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestMetrics tests metrics tracking.
func TestMetrics(t *testing.T) {
	metrics := NewMetrics()

	metrics.IncrementProcessed()
	metrics.IncrementProcessed()
	metrics.IncrementFailed("error 1")
	metrics.IncrementDLQ()
	metrics.IncrementRetries()
	metrics.IncrementRetries()

	snapshot := metrics.Snapshot()

	if snapshot.MessagesProcessed != 2 {
		t.Errorf("Expected MessagesProcessed=2, got %d", snapshot.MessagesProcessed)
	}
	if snapshot.MessagesFailed != 1 {
		t.Errorf("Expected MessagesFailed=1, got %d", snapshot.MessagesFailed)
	}
	if snapshot.MessagesSentToDLQ != 1 {
		t.Errorf("Expected MessagesSentToDLQ=1, got %d", snapshot.MessagesSentToDLQ)
	}
	if snapshot.TotalRetries != 2 {
		t.Errorf("Expected TotalRetries=2, got %d", snapshot.TotalRetries)
	}
	if snapshot.LastError != "error 1" {
		t.Errorf("Expected LastError='error 1', got '%s'", snapshot.LastError)
	}
}

// TestHandler_Close tests proper cleanup on close.
func TestHandler_Close(t *testing.T) {
	producer := NewMockProducer()
	handler := NewHandler(DefaultConfig(), producer)

	// Close should succeed
	if err := handler.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !handler.IsClosed() {
		t.Error("Handler should be closed")
	}

	if !producer.IsClosed() {
		t.Error("Producer should be closed")
	}

	// Double close should be safe
	if err := handler.Close(); err != nil {
		t.Errorf("Double close failed: %v", err)
	}
}

// TestHandler_Flush tests flush operation.
func TestHandler_Flush(t *testing.T) {
	producer := NewMockProducer()
	handler := NewHandler(DefaultConfig(), producer)
	defer handler.Close()

	if err := handler.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

// TestCalculateBackoff tests exponential backoff calculation.
func TestCalculateBackoff(t *testing.T) {
	config := DefaultConfig()
	config.BaseDelay = 100 * time.Millisecond
	config.MaxDelay = 1 * time.Second

	handler := NewHandler(config, nil)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second}, // Capped at MaxDelay
		{5, 1 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		result := handler.calculateBackoff(tt.attempt)
		if result != tt.expected {
			t.Errorf("Attempt %d: expected %v, got %v", tt.attempt, tt.expected, result)
		}
	}
}

// TestConcurrentProcessing tests thread safety under concurrent access.
func TestConcurrentProcessing(t *testing.T) {
	config := DefaultConfig()
	config.BaseDelay = 1 * time.Millisecond
	config.MaxRetries = 1

	producer := NewMockProducer()
	handler := NewHandler(config, producer)
	defer handler.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := &MessageContext{
					Topic:     "orders",
					Partition: int32(id),
					Offset:    int64(j),
					Value:     []byte(`{"order_id": "test"}`),
					Timestamp: time.Now(),
				}

				// Alternate between success and failure
				processFunc := func(ctx context.Context, m *MessageContext) error {
					if j%2 == 0 {
						return nil
					}
					return errors.New("test error")
				}

				_ = handler.ProcessWithRetry(context.Background(), msg, processFunc)
			}
		}(i)
	}

	wg.Wait()

	metrics := handler.Metrics()
	totalMessages := int64(numGoroutines * messagesPerGoroutine)

	// All messages should be accounted for
	if metrics.MessagesProcessed+metrics.MessagesFailed != totalMessages {
		t.Errorf("Expected %d total messages, got %d processed + %d failed",
			totalMessages, metrics.MessagesProcessed, metrics.MessagesFailed)
	}
}
