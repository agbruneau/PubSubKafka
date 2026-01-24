/*
Package kafka provides centralized Kafka configuration management.

This file contains configuration structures and defaults for all Kafka
components (producers, consumers, admin clients).
*/
package kafka

import (
	"time"

	"kafka-demo/pkg/models"
)

// ProducerConfig holds configuration for the Kafka producer.
type ProducerConfig struct {
	// Broker is the Kafka broker address (e.g., "localhost:9092").
	Broker string

	// Topic is the target Kafka topic for messages.
	Topic string

	// Interval is the time between message productions.
	Interval time.Duration

	// FlushTimeout is the maximum time to wait for message delivery.
	FlushTimeout time.Duration

	// Acks defines the acknowledgment level ("all", "1", "0").
	Acks string

	// Retries is the number of retries for failed sends.
	Retries int

	// BatchSize is the maximum batch size in bytes.
	BatchSize int

	// LingerMs is the time to wait before sending a batch.
	LingerMs int

	// CompressionType specifies compression algorithm ("none", "gzip", "snappy", "lz4", "zstd").
	CompressionType string

	// EnableIdempotence enables exactly-once delivery semantics.
	EnableIdempotence bool
}

// DefaultProducerConfig returns a ProducerConfig with sensible defaults.
func DefaultProducerConfig() ProducerConfig {
	return ProducerConfig{
		Broker:            models.DefaultKafkaBroker,
		Topic:             models.DefaultTopic,
		Interval:          models.ProducerMessageInterval,
		FlushTimeout:      time.Duration(models.FlushTimeoutMs) * time.Millisecond,
		Acks:              "all",
		Retries:           3,
		BatchSize:         16384,
		LingerMs:          5,
		CompressionType:   "snappy",
		EnableIdempotence: true,
	}
}

// ConsumerConfig holds configuration for the Kafka consumer.
type ConsumerConfig struct {
	// Broker is the Kafka broker address.
	Broker string

	// Topic is the source Kafka topic to consume from.
	Topic string

	// GroupID is the consumer group identifier.
	GroupID string

	// ReadTimeout is the maximum time to wait for a message.
	ReadTimeout time.Duration

	// AutoCommit enables automatic offset commits.
	AutoCommit bool

	// AutoCommitInterval is the interval between automatic commits.
	AutoCommitInterval time.Duration

	// AutoOffsetReset defines behavior when no offset exists ("earliest", "latest").
	AutoOffsetReset string

	// MaxPollRecords is the maximum number of records per poll.
	MaxPollRecords int

	// SessionTimeout is the consumer session timeout.
	SessionTimeout time.Duration

	// HeartbeatInterval is the interval between heartbeats.
	HeartbeatInterval time.Duration
}

// DefaultConsumerConfig returns a ConsumerConfig with sensible defaults.
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Broker:             models.DefaultKafkaBroker,
		Topic:              models.DefaultTopic,
		GroupID:            models.DefaultConsumerGroup,
		ReadTimeout:        models.TrackerConsumerReadTimeout,
		AutoCommit:         false,
		AutoCommitInterval: 5 * time.Second,
		AutoOffsetReset:    "earliest",
		MaxPollRecords:     500,
		SessionTimeout:     30 * time.Second,
		HeartbeatInterval:  3 * time.Second,
	}
}

// AdminConfig holds configuration for the Kafka admin client.
type AdminConfig struct {
	// Broker is the Kafka broker address.
	Broker string

	// OperationTimeout is the timeout for admin operations.
	OperationTimeout time.Duration
}

// DefaultAdminConfig returns an AdminConfig with sensible defaults.
func DefaultAdminConfig() AdminConfig {
	return AdminConfig{
		Broker:           models.DefaultKafkaBroker,
		OperationTimeout: 30 * time.Second,
	}
}

// Validate checks if the ProducerConfig is valid.
func (c *ProducerConfig) Validate() error {
	if c.Broker == "" {
		return ErrInvalidBroker
	}
	if c.Topic == "" {
		return ErrInvalidTopic
	}
	return nil
}

// Validate checks if the ConsumerConfig is valid.
func (c *ConsumerConfig) Validate() error {
	if c.Broker == "" {
		return ErrInvalidBroker
	}
	if c.Topic == "" {
		return ErrInvalidTopic
	}
	if c.GroupID == "" {
		return ErrInvalidGroupID
	}
	return nil
}

// ToConfigMap converts ProducerConfig to a map for the Kafka client.
func (c *ProducerConfig) ToConfigMap() map[string]interface{} {
	return map[string]interface{}{
		"bootstrap.servers":   c.Broker,
		"acks":                c.Acks,
		"retries":             c.Retries,
		"batch.size":          c.BatchSize,
		"linger.ms":           c.LingerMs,
		"compression.type":    c.CompressionType,
		"enable.idempotence":  c.EnableIdempotence,
	}
}

// ToConfigMap converts ConsumerConfig to a map for the Kafka client.
func (c *ConsumerConfig) ToConfigMap() map[string]interface{} {
	return map[string]interface{}{
		"bootstrap.servers":        c.Broker,
		"group.id":                 c.GroupID,
		"auto.offset.reset":        c.AutoOffsetReset,
		"enable.auto.commit":       c.AutoCommit,
		"auto.commit.interval.ms":  int(c.AutoCommitInterval.Milliseconds()),
		"max.poll.records":         c.MaxPollRecords,
		"session.timeout.ms":       int(c.SessionTimeout.Milliseconds()),
		"heartbeat.interval.ms":    int(c.HeartbeatInterval.Milliseconds()),
	}
}
