/*
Package kafka provides the Producer interface and implementation for Kafka message publishing.

The producer supports:
- Asynchronous message delivery with delivery reports
- Configurable batching and compression
- Idempotent delivery for exactly-once semantics
- Graceful shutdown with message flushing
*/
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"

	"kafka-demo/pkg/models"
)

// MessageProducer defines the interface for producing messages to Kafka.
type MessageProducer interface {
	// Produce sends a message to the configured topic.
	Produce(ctx context.Context, key string, value interface{}) error

	// ProduceRaw sends raw bytes to the specified topic.
	ProduceRaw(topic string, key, value []byte) error

	// Flush waits for all pending messages to be delivered.
	Flush(timeout time.Duration) error

	// Close shuts down the producer gracefully.
	Close() error

	// Metrics returns current producer metrics.
	Metrics() ProducerMetrics
}

// ProducerMetrics holds producer statistics.
type ProducerMetrics struct {
	MessagesSent       int64
	MessagesDelivered  int64
	MessagesFailed     int64
	BytesSent          int64
	LastSentAt         time.Time
	AverageLatencyMs   float64
}

// Producer implements MessageProducer using confluent-kafka-go.
type Producer struct {
	config     ProducerConfig
	client     *kafka.Producer
	deliveryCh chan kafka.Event

	mu       sync.RWMutex
	closed   bool
	metrics  ProducerMetrics
	sequence int
}

// NewProducer creates a new Kafka producer with the given configuration.
func NewProducer(config ProducerConfig) (*Producer, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers":  config.Broker,
		"acks":               config.Acks,
		"retries":            config.Retries,
		"enable.idempotence": config.EnableIdempotence,
	}

	client, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		return nil, &ProducerError{
			Op:    "create",
			Topic: config.Topic,
			Err:   fmt.Errorf("%w: %v", ErrConnectionFailed, err),
		}
	}

	p := &Producer{
		config:     config,
		client:     client,
		deliveryCh: make(chan kafka.Event, models.ProducerDeliveryChannelSize),
	}

	// Start delivery report handler
	go p.handleDeliveryReports()

	return p, nil
}

// handleDeliveryReports processes delivery reports from Kafka.
func (p *Producer) handleDeliveryReports() {
	for e := range p.client.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			p.mu.Lock()
			if ev.TopicPartition.Error != nil {
				p.metrics.MessagesFailed++
			} else {
				p.metrics.MessagesDelivered++
			}
			p.mu.Unlock()
		}
	}
}

// Start begins the message production loop.
func (p *Producer) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return p.shutdown()
		case <-ticker.C:
			if err := p.produceOrder(); err != nil {
				// Log error but continue
				fmt.Printf("Error producing message: %v\n", err)
			}
		}
	}
}

// produceOrder generates and sends an order message.
func (p *Producer) produceOrder() error {
	p.mu.Lock()
	p.sequence++
	seq := p.sequence
	p.mu.Unlock()

	order := generateOrder(seq)
	return p.Produce(context.Background(), order.OrderID, order)
}

// Produce sends a message to the configured topic.
func (p *Producer) Produce(ctx context.Context, key string, value interface{}) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return ErrProducerClosed
	}
	p.mu.RUnlock()

	payload, err := json.Marshal(value)
	if err != nil {
		return &ProducerError{
			Op:    "serialize",
			Topic: p.config.Topic,
			Key:   key,
			Err:   fmt.Errorf("%w: %v", ErrSerializationFailed, err),
		}
	}

	return p.ProduceRaw(p.config.Topic, []byte(key), payload)
}

// ProduceRaw sends raw bytes to the specified topic.
func (p *Producer) ProduceRaw(topic string, key, value []byte) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return ErrProducerClosed
	}
	p.mu.RUnlock()

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   key,
		Value: value,
	}

	if err := p.client.Produce(msg, nil); err != nil {
		return &ProducerError{
			Op:    "produce",
			Topic: topic,
			Key:   string(key),
			Err:   fmt.Errorf("%w: %v", ErrDeliveryFailed, err),
		}
	}

	p.mu.Lock()
	p.metrics.MessagesSent++
	p.metrics.BytesSent += int64(len(value))
	p.metrics.LastSentAt = time.Now()
	p.mu.Unlock()

	return nil
}

// Flush waits for all pending messages to be delivered.
func (p *Producer) Flush(timeout time.Duration) error {
	remaining := p.client.Flush(int(timeout.Milliseconds()))
	if remaining > 0 {
		return fmt.Errorf("%w: %d messages remaining", ErrFlushTimeout, remaining)
	}
	return nil
}

// Metrics returns current producer metrics.
func (p *Producer) Metrics() ProducerMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metrics
}

// Close shuts down the producer gracefully.
func (p *Producer) Close() error {
	return p.shutdown()
}

// shutdown performs graceful shutdown.
func (p *Producer) shutdown() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Flush pending messages
	if err := p.Flush(p.config.FlushTimeout); err != nil {
		fmt.Printf("Warning: flush timeout during shutdown: %v\n", err)
	}

	p.client.Close()
	return nil
}

// generateOrder creates a sample order for testing.
func generateOrder(sequence int) *models.Order {
	orderID := uuid.New().String()
	customerID := uuid.New().String()
	now := time.Now().UTC()

	items := []models.OrderItem{
		{
			ItemID:     uuid.New().String(),
			ItemName:   "Sample Product",
			Quantity:   2,
			UnitPrice:  29.99,
			TotalPrice: 59.98,
		},
	}

	subTotal := items[0].TotalPrice
	tax := subTotal * models.ProducerDefaultTaxRate
	total := subTotal + tax + models.ProducerDefaultShippingFee

	return &models.Order{
		OrderID:  orderID,
		Sequence: sequence,
		Status:   "pending",
		Items:    items,
		SubTotal: subTotal,
		Tax:      tax,
		ShippingFee: models.ProducerDefaultShippingFee,
		Total:       total,
		Currency:    models.ProducerDefaultCurrency,
		PaymentMethod:   models.ProducerDefaultPayment,
		ShippingAddress: "123 Test Street, Paris, France",
		Metadata: models.OrderMetadata{
			Timestamp:     now.Format(time.RFC3339),
			Version:       "1.0",
			EventType:     "order.created",
			Source:        "producer-service",
			CorrelationID: uuid.New().String(),
		},
		CustomerInfo: models.CustomerInfo{
			CustomerID:   customerID,
			Name:         "Test Customer",
			Email:        "test@example.com",
			Phone:        "+33612345678",
			Address:      "123 Test Street, Paris, France",
			LoyaltyLevel: "gold",
		},
		InventoryStatus: []models.InventoryStatus{
			{
				ItemID:       items[0].ItemID,
				ItemName:     items[0].ItemName,
				AvailableQty: 100,
				ReservedQty:  2,
				UnitPrice:    items[0].UnitPrice,
				InStock:      true,
				Warehouse:    models.ProducerDefaultWarehouse,
			},
		},
	}
}
