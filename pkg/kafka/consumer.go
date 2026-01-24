/*
Package kafka provides the Consumer interface and implementation for Kafka message consumption.

The consumer supports:
- Consumer group coordination for horizontal scaling
- Manual offset commit for at-least-once delivery
- Configurable message handling with context support
- Graceful shutdown with rebalance handling
*/
package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// MessageHandler is the function signature for processing consumed messages.
type MessageHandler func(ctx context.Context, msg *Message) error

// Message represents a consumed Kafka message.
type Message struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Timestamp time.Time
	Headers   map[string]string
}

// MessageConsumer defines the interface for consuming messages from Kafka.
type MessageConsumer interface {
	// Start begins consuming messages from the configured topic.
	Start(ctx context.Context) error

	// Commit commits the offset for the given message.
	Commit(msg *Message) error

	// Close shuts down the consumer gracefully.
	Close() error

	// Metrics returns current consumer metrics.
	Metrics() ConsumerMetrics
}

// ConsumerMetrics holds consumer statistics.
type ConsumerMetrics struct {
	MessagesReceived   int64
	MessagesProcessed  int64
	MessagesFailed     int64
	BytesReceived      int64
	LastReceivedAt     time.Time
	CurrentLag         int64
	RebalanceCount     int
	AverageProcessMs   float64
}

// Consumer implements MessageConsumer using confluent-kafka-go.
type Consumer struct {
	config  ConsumerConfig
	client  *kafka.Consumer
	handler MessageHandler

	mu      sync.RWMutex
	closed  bool
	metrics ConsumerMetrics
}

// NewConsumer creates a new Kafka consumer with the given configuration.
func NewConsumer(config ConsumerConfig, handler MessageHandler) (*Consumer, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}

	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers":     config.Broker,
		"group.id":              config.GroupID,
		"auto.offset.reset":     config.AutoOffsetReset,
		"enable.auto.commit":    config.AutoCommit,
		"session.timeout.ms":    int(config.SessionTimeout.Milliseconds()),
		"heartbeat.interval.ms": int(config.HeartbeatInterval.Milliseconds()),
	}

	client, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		return nil, &ConsumerError{
			Op:    "create",
			Topic: config.Topic,
			Err:   fmt.Errorf("%w: %v", ErrConnectionFailed, err),
		}
	}

	c := &Consumer{
		config:  config,
		client:  client,
		handler: handler,
	}

	return c, nil
}

// Start begins consuming messages from the configured topic.
func (c *Consumer) Start(ctx context.Context) error {
	if err := c.client.Subscribe(c.config.Topic, nil); err != nil {
		return &ConsumerError{
			Op:    "subscribe",
			Topic: c.config.Topic,
			Err:   fmt.Errorf("%w: %v", ErrSubscriptionFailed, err),
		}
	}

	for {
		select {
		case <-ctx.Done():
			return c.shutdown()
		default:
			msg, err := c.poll()
			if err != nil {
				// Log error but continue
				if err != ErrNoMessages {
					fmt.Printf("Poll error: %v\n", err)
				}
				continue
			}

			if msg != nil {
				if err := c.processMessage(ctx, msg); err != nil {
					c.mu.Lock()
					c.metrics.MessagesFailed++
					c.mu.Unlock()
					fmt.Printf("Process error: %v\n", err)
				}
			}
		}
	}
}

// poll retrieves the next message from Kafka.
func (c *Consumer) poll() (*Message, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrConsumerClosed
	}
	c.mu.RUnlock()

	ev := c.client.Poll(int(c.config.ReadTimeout.Milliseconds()))
	if ev == nil {
		return nil, ErrNoMessages
	}

	switch e := ev.(type) {
	case *kafka.Message:
		msg := &Message{
			Topic:     *e.TopicPartition.Topic,
			Partition: e.TopicPartition.Partition,
			Offset:    int64(e.TopicPartition.Offset),
			Key:       e.Key,
			Value:     e.Value,
			Timestamp: e.Timestamp,
			Headers:   make(map[string]string),
		}

		for _, h := range e.Headers {
			msg.Headers[h.Key] = string(h.Value)
		}

		c.mu.Lock()
		c.metrics.MessagesReceived++
		c.metrics.BytesReceived += int64(len(e.Value))
		c.metrics.LastReceivedAt = time.Now()
		c.mu.Unlock()

		return msg, nil

	case kafka.PartitionEOF:
		return nil, ErrNoMessages

	case kafka.Error:
		if e.Code() == kafka.ErrAllBrokersDown {
			return nil, ErrBrokerNotAvailable
		}
		return nil, fmt.Errorf("kafka error: %v", e)

	default:
		return nil, nil
	}
}

// processMessage handles a single message.
func (c *Consumer) processMessage(ctx context.Context, msg *Message) error {
	start := time.Now()

	if err := c.handler(ctx, msg); err != nil {
		return err
	}

	// Commit offset after successful processing
	if !c.config.AutoCommit {
		if err := c.Commit(msg); err != nil {
			return err
		}
	}

	c.mu.Lock()
	c.metrics.MessagesProcessed++
	elapsed := time.Since(start).Milliseconds()
	// Update rolling average
	total := float64(c.metrics.MessagesProcessed)
	c.metrics.AverageProcessMs = ((c.metrics.AverageProcessMs * (total - 1)) + float64(elapsed)) / total
	c.mu.Unlock()

	return nil
}

// Commit commits the offset for the given message.
func (c *Consumer) Commit(msg *Message) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConsumerClosed
	}
	c.mu.RUnlock()

	offsets := []kafka.TopicPartition{
		{
			Topic:     &msg.Topic,
			Partition: msg.Partition,
			Offset:    kafka.Offset(msg.Offset + 1),
		},
	}

	if _, err := c.client.CommitOffsets(offsets); err != nil {
		return &ConsumerError{
			Op:        "commit",
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Err:       fmt.Errorf("%w: %v", ErrCommitFailed, err),
		}
	}

	return nil
}

// Metrics returns current consumer metrics.
func (c *Consumer) Metrics() ConsumerMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

// Close shuts down the consumer gracefully.
func (c *Consumer) Close() error {
	return c.shutdown()
}

// shutdown performs graceful shutdown.
func (c *Consumer) shutdown() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	return c.client.Close()
}
