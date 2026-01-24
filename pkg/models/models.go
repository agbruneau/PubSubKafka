/*
Package models defines the shared data structures for system observability.

These models are used for:
- **Structured Logging**: Standardized format for system health and audit trails.
- **Real-time Monitoring**: Decoupled metrics processing for the TUI.
- **Dead Letter Queue**: Failed message tracking and reprocessing support.
*/

package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// nowFunc is used for time injection in tests.
var nowFunc = time.Now

// generateUUID generates a new UUID string.
func generateUUID() string {
	return uuid.New().String()
}

// LogLevel defines the severity levels for structured logging.
type LogLevel string

const (
	LogLevelINFO  LogLevel = "INFO"
	LogLevelERROR LogLevel = "ERROR"
)

// LogEntry defines the structure of a system health log record.
// It implements the "Application Health Monitoring" pattern, following
// a structured JSON format optimized for ingestion and visualization
// by modern monitoring and alerting stacks.
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`          // Horodatage du log au format RFC3339.
	Level     LogLevel               `json:"level"`              // Niveau de sévérité (INFO, ERROR).
	Message   string                 `json:"message"`            // Message principal du log.
	Service   string                 `json:"service"`            // Nom du service émetteur.
	Error     string                 `json:"error,omitempty"`    // Message d'erreur, si applicable.
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Données contextuelles supplémentaires.
}

// EventEntry defines the record structure for the business event journal.
// It implements the "Audit Trail" pattern by capturing an immutable,
// high-fidelity copy of every Kafka message received, along with its metadata.
// This journal serves as the source of truth for compliance and debugging.
type EventEntry struct {
	Timestamp      string          `json:"timestamp"`            // Horodatage de la réception au format RFC3339.
	EventType      string          `json:"event_type"`           // Type d'événement (ex: "message.received").
	KafkaTopic     string          `json:"kafka_topic"`          // Topic Kafka d'origine.
	KafkaPartition int32           `json:"kafka_partition"`      // Partition Kafka d'origine.
	KafkaOffset    int64           `json:"kafka_offset"`         // Offset du message dans la partition.
	RawMessage     string          `json:"raw_message"`          // Contenu brut du message.
	MessageSize    int             `json:"message_size"`         // Taille du message en octets.
	Deserialized   bool            `json:"deserialized"`         // Indique si la désérialisation a réussi.
	Error          string          `json:"error,omitempty"`      // Erreur de désérialisation, si applicable.
	OrderFull      json.RawMessage `json:"order_full,omitempty"` // Contenu complet de la commande désérialisée.
}

// FailureReason categorizes why a message was sent to the Dead Letter Queue.
type FailureReason string

const (
	// FailureReasonDeserialization indicates JSON parsing/unmarshaling failure.
	FailureReasonDeserialization FailureReason = "DESERIALIZATION_ERROR"
	// FailureReasonValidation indicates business validation failure.
	FailureReasonValidation FailureReason = "VALIDATION_ERROR"
	// FailureReasonProcessing indicates transient processing failure after max retries.
	FailureReasonProcessing FailureReason = "PROCESSING_ERROR"
	// FailureReasonTimeout indicates processing timeout.
	FailureReasonTimeout FailureReason = "TIMEOUT_ERROR"
	// FailureReasonUnknown indicates an unknown/unexpected error.
	FailureReasonUnknown FailureReason = "UNKNOWN_ERROR"
)

// DeadLetterMessage represents a failed message that has been routed to the DLQ.
// It implements the "Dead Letter Queue" pattern by capturing the original message,
// failure context, and retry metadata to enable later analysis and potential reprocessing.
type DeadLetterMessage struct {
	// --- Message Identification ---
	ID        string `json:"id"`        // Unique identifier for this DLQ entry (UUID).
	Timestamp string `json:"timestamp"` // ISO 8601 timestamp when message was sent to DLQ.

	// --- Original Message Context ---
	OriginalTopic     string `json:"original_topic"`     // Source Kafka topic.
	OriginalPartition int32  `json:"original_partition"` // Source partition number.
	OriginalOffset    int64  `json:"original_offset"`    // Offset in source partition.
	OriginalKey       string `json:"original_key"`       // Original message key (if any).
	OriginalPayload   string `json:"original_payload"`   // Raw message payload.
	OriginalTimestamp string `json:"original_timestamp"` // Original message timestamp.

	// --- Failure Context ---
	FailureReason  FailureReason `json:"failure_reason"`  // Categorized failure type.
	ErrorMessage   string        `json:"error_message"`   // Detailed error description.
	ErrorStack     string        `json:"error_stack"`     // Stack trace (if available).
	ConsumerGroup  string        `json:"consumer_group"`  // Consumer group that failed.
	ProcessingHost string        `json:"processing_host"` // Hostname of the failing consumer.

	// --- Retry Metadata ---
	RetryCount    int    `json:"retry_count"`    // Number of processing attempts made.
	FirstFailure  string `json:"first_failure"`  // Timestamp of first failure attempt.
	LastFailure   string `json:"last_failure"`   // Timestamp of last failure attempt.
	NextRetryAt   string `json:"next_retry_at"`  // Scheduled time for next retry (if applicable).
	IsReprocessed bool   `json:"is_reprocessed"` // Whether this message has been reprocessed.

	// --- Additional Context ---
	Metadata map[string]string `json:"metadata,omitempty"` // Additional key-value metadata.
}

// NewDeadLetterMessage creates a new DeadLetterMessage with required fields initialized.
func NewDeadLetterMessage(
	originalTopic string,
	originalPartition int32,
	originalOffset int64,
	originalPayload string,
	reason FailureReason,
	errorMsg string,
) *DeadLetterMessage {
	now := nowFunc().Format("2006-01-02T15:04:05Z07:00")
	return &DeadLetterMessage{
		ID:                generateUUID(),
		Timestamp:         now,
		OriginalTopic:     originalTopic,
		OriginalPartition: originalPartition,
		OriginalOffset:    originalOffset,
		OriginalPayload:   originalPayload,
		FailureReason:     reason,
		ErrorMessage:      errorMsg,
		RetryCount:        0,
		FirstFailure:      now,
		LastFailure:       now,
		Metadata:          make(map[string]string),
	}
}

// IncrementRetry updates retry metadata for another attempt.
func (d *DeadLetterMessage) IncrementRetry() {
	d.RetryCount++
	d.LastFailure = nowFunc().Format("2006-01-02T15:04:05Z07:00")
}

// ShouldRetry returns true if retry count is below maximum.
func (d *DeadLetterMessage) ShouldRetry(maxRetries int) bool {
	return d.RetryCount < maxRetries
}

// MarshalJSON serializes the DeadLetterMessage to JSON bytes.
func (d *DeadLetterMessage) MarshalJSON() ([]byte, error) {
	type Alias DeadLetterMessage
	return json.Marshal((*Alias)(d))
}