package models

import (
	"encoding/json"
	"testing"
	"time"
)

// Mock time function for testing
func setMockTime(t time.Time) func() {
	original := nowFunc
	nowFunc = func() time.Time { return t }
	return func() { nowFunc = original }
}

// TestNewDeadLetterMessage tests creation of a new DLQ message.
func TestNewDeadLetterMessage(t *testing.T) {
	fixedTime := time.Date(2025, 1, 3, 12, 0, 0, 0, time.UTC)
	cleanup := setMockTime(fixedTime)
	defer cleanup()

	msg := NewDeadLetterMessage(
		"orders",
		2,
		100,
		`{"order_id": "test-123"}`,
		FailureReasonValidation,
		"Order validation failed",
	)

	if msg == nil {
		t.Fatal("NewDeadLetterMessage returned nil")
	}

	// Check required fields
	if msg.ID == "" {
		t.Error("ID should not be empty")
	}
	if msg.OriginalTopic != "orders" {
		t.Errorf("Expected OriginalTopic='orders', got '%s'", msg.OriginalTopic)
	}
	if msg.OriginalPartition != 2 {
		t.Errorf("Expected OriginalPartition=2, got %d", msg.OriginalPartition)
	}
	if msg.OriginalOffset != 100 {
		t.Errorf("Expected OriginalOffset=100, got %d", msg.OriginalOffset)
	}
	if msg.OriginalPayload != `{"order_id": "test-123"}` {
		t.Errorf("OriginalPayload mismatch")
	}
	if msg.FailureReason != FailureReasonValidation {
		t.Errorf("Expected FailureReason=%s, got %s", FailureReasonValidation, msg.FailureReason)
	}
	if msg.ErrorMessage != "Order validation failed" {
		t.Errorf("ErrorMessage mismatch")
	}
	if msg.RetryCount != 0 {
		t.Errorf("Expected RetryCount=0, got %d", msg.RetryCount)
	}
	if msg.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

// TestDeadLetterMessage_IncrementRetry tests retry counter increment.
func TestDeadLetterMessage_IncrementRetry(t *testing.T) {
	msg := NewDeadLetterMessage(
		"orders", 0, 0, "{}", FailureReasonProcessing, "error",
	)

	initialCount := msg.RetryCount

	msg.IncrementRetry()

	if msg.RetryCount != initialCount+1 {
		t.Errorf("Expected RetryCount=%d, got %d", initialCount+1, msg.RetryCount)
	}
	if msg.LastFailure == "" {
		t.Error("LastFailure should be updated")
	}

	msg.IncrementRetry()
	msg.IncrementRetry()

	if msg.RetryCount != 3 {
		t.Errorf("Expected RetryCount=3, got %d", msg.RetryCount)
	}
}

// TestDeadLetterMessage_ShouldRetry tests retry eligibility check.
func TestDeadLetterMessage_ShouldRetry(t *testing.T) {
	msg := NewDeadLetterMessage(
		"orders", 0, 0, "{}", FailureReasonProcessing, "error",
	)

	tests := []struct {
		retryCount int
		maxRetries int
		expected   bool
	}{
		{0, 3, true},
		{1, 3, true},
		{2, 3, true},
		{3, 3, false},
		{4, 3, false},
		{0, 0, false},
		{0, 1, true},
	}

	for _, tt := range tests {
		msg.RetryCount = tt.retryCount
		result := msg.ShouldRetry(tt.maxRetries)
		if result != tt.expected {
			t.Errorf("RetryCount=%d, MaxRetries=%d: expected %v, got %v",
				tt.retryCount, tt.maxRetries, tt.expected, result)
		}
	}
}

// TestDeadLetterMessage_MarshalJSON tests JSON serialization.
func TestDeadLetterMessage_MarshalJSON(t *testing.T) {
	msg := NewDeadLetterMessage(
		"orders",
		1,
		50,
		`{"order_id": "order-456"}`,
		FailureReasonDeserialization,
		"invalid JSON structure",
	)
	msg.ConsumerGroup = "test-group"
	msg.ProcessingHost = "worker-01"
	msg.Metadata["custom_key"] = "custom_value"

	data, err := msg.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Produced invalid JSON: %v", err)
	}

	// Check key fields
	if parsed["original_topic"] != "orders" {
		t.Errorf("Expected original_topic='orders', got '%v'", parsed["original_topic"])
	}
	if parsed["failure_reason"] != string(FailureReasonDeserialization) {
		t.Errorf("Expected failure_reason=%s, got '%v'", FailureReasonDeserialization, parsed["failure_reason"])
	}
	if parsed["consumer_group"] != "test-group" {
		t.Errorf("Expected consumer_group='test-group', got '%v'", parsed["consumer_group"])
	}
}

// TestDeadLetterMessage_RoundTrip tests JSON marshal/unmarshal round trip.
func TestDeadLetterMessage_RoundTrip(t *testing.T) {
	original := NewDeadLetterMessage(
		"orders",
		5,
		999,
		`{"order_id": "round-trip-test"}`,
		FailureReasonTimeout,
		"processing timeout exceeded",
	)
	original.OriginalKey = "key-123"
	original.ConsumerGroup = "consumer-group-1"
	original.ProcessingHost = "host-abc"
	original.RetryCount = 3
	original.IsReprocessed = false
	original.Metadata["trace_id"] = "abc-123"

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var restored DeadLetterMessage
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify fields
	if restored.ID != original.ID {
		t.Errorf("ID mismatch: expected '%s', got '%s'", original.ID, restored.ID)
	}
	if restored.OriginalTopic != original.OriginalTopic {
		t.Errorf("OriginalTopic mismatch")
	}
	if restored.OriginalPartition != original.OriginalPartition {
		t.Errorf("OriginalPartition mismatch")
	}
	if restored.OriginalOffset != original.OriginalOffset {
		t.Errorf("OriginalOffset mismatch")
	}
	if restored.FailureReason != original.FailureReason {
		t.Errorf("FailureReason mismatch")
	}
	if restored.ErrorMessage != original.ErrorMessage {
		t.Errorf("ErrorMessage mismatch")
	}
	if restored.RetryCount != original.RetryCount {
		t.Errorf("RetryCount mismatch: expected %d, got %d", original.RetryCount, restored.RetryCount)
	}
	if restored.ConsumerGroup != original.ConsumerGroup {
		t.Errorf("ConsumerGroup mismatch")
	}
	if restored.Metadata["trace_id"] != original.Metadata["trace_id"] {
		t.Errorf("Metadata mismatch")
	}
}

// TestFailureReason_Constants tests failure reason constant values.
func TestFailureReason_Constants(t *testing.T) {
	reasons := []FailureReason{
		FailureReasonDeserialization,
		FailureReasonValidation,
		FailureReasonProcessing,
		FailureReasonTimeout,
		FailureReasonUnknown,
	}

	// Ensure all reasons are unique
	seen := make(map[FailureReason]bool)
	for _, r := range reasons {
		if seen[r] {
			t.Errorf("Duplicate FailureReason: %s", r)
		}
		seen[r] = true

		// Ensure they're not empty
		if r == "" {
			t.Error("FailureReason should not be empty")
		}
	}
}

// TestDeadLetterMessage_EmptyMetadata tests behavior with nil metadata.
func TestDeadLetterMessage_EmptyMetadata(t *testing.T) {
	msg := &DeadLetterMessage{
		ID:              "test-id",
		OriginalTopic:   "orders",
		FailureReason:   FailureReasonUnknown,
		Metadata:        nil, // Explicitly nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal with nil metadata failed: %v", err)
	}

	// Should serialize correctly
	if len(data) == 0 {
		t.Error("Marshaled data should not be empty")
	}
}
