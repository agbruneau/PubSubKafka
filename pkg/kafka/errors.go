/*
Package kafka provides custom error types for Kafka operations.

This file defines specific error types to enable precise error handling
and classification across the Kafka producer and consumer components.
*/
package kafka

import (
	"errors"
	"fmt"
)

// Configuration errors
var (
	// ErrInvalidBroker indicates an invalid or empty broker address.
	ErrInvalidBroker = errors.New("invalid broker address: broker cannot be empty")

	// ErrInvalidTopic indicates an invalid or empty topic name.
	ErrInvalidTopic = errors.New("invalid topic name: topic cannot be empty")

	// ErrInvalidGroupID indicates an invalid or empty consumer group ID.
	ErrInvalidGroupID = errors.New("invalid group ID: group ID cannot be empty")
)

// Connection errors
var (
	// ErrConnectionFailed indicates a connection failure to the Kafka broker.
	ErrConnectionFailed = errors.New("failed to connect to Kafka broker")

	// ErrBrokerNotAvailable indicates the broker is not responding.
	ErrBrokerNotAvailable = errors.New("broker not available")

	// ErrConnectionTimeout indicates a connection timeout.
	ErrConnectionTimeout = errors.New("connection timeout")
)

// Producer errors
var (
	// ErrProducerClosed indicates the producer has been closed.
	ErrProducerClosed = errors.New("producer is closed")

	// ErrMessageTooLarge indicates the message exceeds the maximum size.
	ErrMessageTooLarge = errors.New("message exceeds maximum size limit")

	// ErrSerializationFailed indicates message serialization failure.
	ErrSerializationFailed = errors.New("failed to serialize message")

	// ErrDeliveryFailed indicates message delivery failure.
	ErrDeliveryFailed = errors.New("message delivery failed")

	// ErrFlushTimeout indicates a flush operation timed out.
	ErrFlushTimeout = errors.New("flush operation timed out")
)

// Consumer errors
var (
	// ErrConsumerClosed indicates the consumer has been closed.
	ErrConsumerClosed = errors.New("consumer is closed")

	// ErrSubscriptionFailed indicates topic subscription failure.
	ErrSubscriptionFailed = errors.New("failed to subscribe to topic")

	// ErrDeserializationFailed indicates message deserialization failure.
	ErrDeserializationFailed = errors.New("failed to deserialize message")

	// ErrCommitFailed indicates offset commit failure.
	ErrCommitFailed = errors.New("failed to commit offset")

	// ErrRebalanceInProgress indicates a consumer group rebalance is in progress.
	ErrRebalanceInProgress = errors.New("consumer group rebalance in progress")

	// ErrNoMessages indicates no messages available within timeout.
	ErrNoMessages = errors.New("no messages available")
)

// Topic errors
var (
	// ErrTopicNotFound indicates the topic does not exist.
	ErrTopicNotFound = errors.New("topic not found")

	// ErrTopicAlreadyExists indicates the topic already exists.
	ErrTopicAlreadyExists = errors.New("topic already exists")

	// ErrInvalidPartitionCount indicates an invalid partition count.
	ErrInvalidPartitionCount = errors.New("invalid partition count: must be positive")

	// ErrInvalidReplicationFactor indicates an invalid replication factor.
	ErrInvalidReplicationFactor = errors.New("invalid replication factor: must be positive")
)

// ProducerError wraps producer-related errors with additional context.
type ProducerError struct {
	Op      string // Operation that failed
	Topic   string // Topic involved
	Key     string // Message key if applicable
	Err     error  // Underlying error
}

func (e *ProducerError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("producer %s on topic %s (key=%s): %v", e.Op, e.Topic, e.Key, e.Err)
	}
	return fmt.Sprintf("producer %s on topic %s: %v", e.Op, e.Topic, e.Err)
}

func (e *ProducerError) Unwrap() error {
	return e.Err
}

// ConsumerError wraps consumer-related errors with additional context.
type ConsumerError struct {
	Op        string // Operation that failed
	Topic     string // Topic involved
	Partition int32  // Partition number
	Offset    int64  // Offset if applicable
	Err       error  // Underlying error
}

func (e *ConsumerError) Error() string {
	if e.Partition >= 0 {
		return fmt.Sprintf("consumer %s on topic %s (partition=%d, offset=%d): %v",
			e.Op, e.Topic, e.Partition, e.Offset, e.Err)
	}
	return fmt.Sprintf("consumer %s on topic %s: %v", e.Op, e.Topic, e.Err)
}

func (e *ConsumerError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is transient and can be retried.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// List of retryable errors
	retryableErrors := []error{
		ErrBrokerNotAvailable,
		ErrConnectionTimeout,
		ErrRebalanceInProgress,
		ErrFlushTimeout,
	}

	for _, re := range retryableErrors {
		if errors.Is(err, re) {
			return true
		}
	}

	return false
}

// IsFatal returns true if the error is fatal and should stop processing.
func IsFatal(err error) bool {
	if err == nil {
		return false
	}

	// List of fatal errors
	fatalErrors := []error{
		ErrInvalidBroker,
		ErrInvalidTopic,
		ErrInvalidGroupID,
		ErrProducerClosed,
		ErrConsumerClosed,
	}

	for _, fe := range fatalErrors {
		if errors.Is(err, fe) {
			return true
		}
	}

	return false
}
