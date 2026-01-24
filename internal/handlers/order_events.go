/*
Package handlers provides business logic handlers for Kafka message processing.

This package contains the event handlers that process incoming messages
and implement the core business logic.
*/
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kafka-demo/pkg/dlq"
	"kafka-demo/pkg/kafka"
	"kafka-demo/pkg/models"
	"kafka-demo/pkg/monitoring"
)

// OrderEventHandler processes order-related Kafka events.
type OrderEventHandler struct {
	logger     *monitoring.Logger
	dlqHandler *dlq.Handler
	metrics    *OrderMetrics
}

// OrderMetrics tracks order processing statistics.
type OrderMetrics struct {
	OrdersReceived    int64
	OrdersProcessed   int64
	OrdersValidated   int64
	OrdersFailed      int64
	ValidationErrors  int64
	ProcessingTimeMs  float64
}

// NewOrderEventHandler creates a new order event handler.
func NewOrderEventHandler(logger *monitoring.Logger, dlqHandler *dlq.Handler) *OrderEventHandler {
	return &OrderEventHandler{
		logger:     logger,
		dlqHandler: dlqHandler,
		metrics:    &OrderMetrics{},
	}
}

// Handle processes a single order event message.
func (h *OrderEventHandler) Handle(ctx context.Context, msg *kafka.Message) error {
	start := time.Now()
	h.metrics.OrdersReceived++

	h.logger.Info("Processing order message",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	// Deserialize the order
	var order models.Order
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		h.metrics.OrdersFailed++
		h.logger.Error("Failed to deserialize order",
			"error", err,
			"offset", msg.Offset,
		)

		// Send to DLQ for deserialization errors
		return h.sendToDLQ(ctx, msg, models.FailureReasonDeserialization, err)
	}

	// Validate the order
	if err := order.Validate(); err != nil {
		h.metrics.ValidationErrors++
		h.logger.Error("Order validation failed",
			"order_id", order.OrderID,
			"error", err,
		)

		// Send to DLQ for validation errors (non-retryable)
		return h.sendToDLQ(ctx, msg, models.FailureReasonValidation, err)
	}

	h.metrics.OrdersValidated++

	// Process the order based on event type
	if err := h.processOrder(ctx, &order); err != nil {
		h.metrics.OrdersFailed++
		h.logger.Error("Order processing failed",
			"order_id", order.OrderID,
			"error", err,
		)

		// For processing errors, use the DLQ handler with retries
		msgCtx := &dlq.MessageContext{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
			Timestamp: msg.Timestamp,
		}

		return h.dlqHandler.ProcessWithRetry(ctx, msgCtx, func(ctx context.Context, m *dlq.MessageContext) error {
			return h.processOrder(ctx, &order)
		})
	}

	h.metrics.OrdersProcessed++
	
	duration := time.Since(start)
	h.metrics.ProcessingTimeMs = float64(duration.Milliseconds())

	h.logger.Info("Order processed successfully",
		"order_id", order.OrderID,
		"sequence", order.Sequence,
		"status", order.Status,
		"total", order.Total,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

// processOrder implements the core order processing logic.
func (h *OrderEventHandler) processOrder(ctx context.Context, order *models.Order) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Process based on event type
	eventType := order.Metadata.EventType
	switch eventType {
	case "order.created":
		return h.handleOrderCreated(ctx, order)
	case "order.updated":
		return h.handleOrderUpdated(ctx, order)
	case "order.cancelled":
		return h.handleOrderCancelled(ctx, order)
	case "order.shipped":
		return h.handleOrderShipped(ctx, order)
	default:
		// Unknown event type - process as generic order
		h.logger.Warn("Unknown event type, processing as generic order",
			"event_type", eventType,
			"order_id", order.OrderID,
		)
		return h.handleGenericOrder(ctx, order)
	}
}

// handleOrderCreated processes new order events.
func (h *OrderEventHandler) handleOrderCreated(ctx context.Context, order *models.Order) error {
	h.logger.Info("Handling order.created event",
		"order_id", order.OrderID,
		"customer_id", order.CustomerInfo.CustomerID,
		"items_count", len(order.Items),
		"total", order.Total,
	)

	// TODO: Implement business logic
	// - Save order to database
	// - Update inventory
	// - Send confirmation email
	// - Trigger payment processing

	return nil
}

// handleOrderUpdated processes order update events.
func (h *OrderEventHandler) handleOrderUpdated(ctx context.Context, order *models.Order) error {
	h.logger.Info("Handling order.updated event",
		"order_id", order.OrderID,
		"status", order.Status,
	)

	// TODO: Implement business logic
	// - Update order in database
	// - Notify relevant services

	return nil
}

// handleOrderCancelled processes order cancellation events.
func (h *OrderEventHandler) handleOrderCancelled(ctx context.Context, order *models.Order) error {
	h.logger.Info("Handling order.cancelled event",
		"order_id", order.OrderID,
	)

	// TODO: Implement business logic
	// - Update order status
	// - Release inventory
	// - Process refund

	return nil
}

// handleOrderShipped processes order shipped events.
func (h *OrderEventHandler) handleOrderShipped(ctx context.Context, order *models.Order) error {
	h.logger.Info("Handling order.shipped event",
		"order_id", order.OrderID,
		"shipping_address", order.ShippingAddress,
	)

	// TODO: Implement business logic
	// - Update order status
	// - Send shipping notification

	return nil
}

// handleGenericOrder processes unknown order events.
func (h *OrderEventHandler) handleGenericOrder(ctx context.Context, order *models.Order) error {
	h.logger.Info("Handling generic order event",
		"order_id", order.OrderID,
		"event_type", order.Metadata.EventType,
	)

	return nil
}

// sendToDLQ sends a failed message to the Dead Letter Queue.
func (h *OrderEventHandler) sendToDLQ(ctx context.Context, msg *kafka.Message, reason models.FailureReason, err error) error {
	if h.dlqHandler == nil {
		return fmt.Errorf("DLQ handler not configured: %w", err)
	}

	msgCtx := &dlq.MessageContext{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       msg.Key,
		Value:     msg.Value,
		Timestamp: msg.Timestamp,
	}

	return h.dlqHandler.SendToDLQ(msgCtx, reason, err.Error())
}

// Metrics returns the current handler metrics.
func (h *OrderEventHandler) Metrics() OrderMetrics {
	return *h.metrics
}
