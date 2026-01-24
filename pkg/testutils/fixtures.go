/*
Package testutils provides test fixtures for Kafka testing.

This package includes:
- Sample order data
- Mock message generators
- Test data factories
*/
package testutils

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"kafka-demo/pkg/models"
)

// OrderFixture generates test orders.
type OrderFixture struct {
	sequence int
}

// NewOrderFixture creates a new order fixture generator.
func NewOrderFixture() *OrderFixture {
	return &OrderFixture{sequence: 0}
}

// GenerateOrder creates a valid test order.
func (f *OrderFixture) GenerateOrder() *models.Order {
	f.sequence++
	return f.GenerateOrderWithSequence(f.sequence)
}

// GenerateOrderWithSequence creates a test order with a specific sequence number.
func (f *OrderFixture) GenerateOrderWithSequence(seq int) *models.Order {
	orderID := uuid.New().String()
	customerID := uuid.New().String()
	itemID := uuid.New().String()
	now := time.Now().UTC()

	items := []models.OrderItem{
		{
			ItemID:     itemID,
			ItemName:   "Test Product",
			Quantity:   2,
			UnitPrice:  29.99,
			TotalPrice: 59.98,
		},
	}

	subTotal := 59.98
	tax := 12.00
	shippingFee := 5.99
	total := subTotal + tax + shippingFee

	return &models.Order{
		OrderID:  orderID,
		Sequence: seq,
		Status:   "pending",
		Items:    items,
		SubTotal: subTotal,
		Tax:      tax,
		ShippingFee: shippingFee,
		Total:       total,
		Currency:    "EUR",
		PaymentMethod:   "credit_card",
		ShippingAddress: "123 Test Street, Paris, France 75001",
		Metadata: models.OrderMetadata{
			Timestamp:     now.Format(time.RFC3339),
			Version:       "1.0",
			EventType:     "order.created",
			Source:        "test-fixture",
			CorrelationID: uuid.New().String(),
		},
		CustomerInfo: models.CustomerInfo{
			CustomerID:   customerID,
			Name:         "Test Customer",
			Email:        "test@example.com",
			Phone:        "+33612345678",
			Address:      "123 Test Street, Paris, France 75001",
			LoyaltyLevel: "gold",
		},
		InventoryStatus: []models.InventoryStatus{
			{
				ItemID:       itemID,
				ItemName:     "Test Product",
				AvailableQty: 100,
				ReservedQty:  2,
				UnitPrice:    29.99,
				InStock:      true,
				Warehouse:    "PARIS-01",
			},
		},
	}
}

// GenerateInvalidOrder creates an invalid order for error testing.
func (f *OrderFixture) GenerateInvalidOrder() *models.Order {
	order := f.GenerateOrder()
	order.OrderID = ""  // Invalid: empty order ID
	return order
}

// GenerateMalformedJSON returns malformed JSON for deserialization testing.
func (f *OrderFixture) GenerateMalformedJSON() []byte {
	return []byte(`{"order_id": "test", "sequence": "not-a-number"}`)
}

// GenerateOrderJSON returns a valid order as JSON bytes.
func (f *OrderFixture) GenerateOrderJSON() ([]byte, error) {
	order := f.GenerateOrder()
	return json.Marshal(order)
}

// GenerateOrders creates multiple test orders.
func (f *OrderFixture) GenerateOrders(count int) []*models.Order {
	orders := make([]*models.Order, count)
	for i := 0; i < count; i++ {
		orders[i] = f.GenerateOrder()
	}
	return orders
}

// DLQFixture generates test DLQ messages.
type DLQFixture struct{}

// NewDLQFixture creates a new DLQ fixture generator.
func NewDLQFixture() *DLQFixture {
	return &DLQFixture{}
}

// GenerateDLQMessage creates a test DLQ message.
func (f *DLQFixture) GenerateDLQMessage(reason models.FailureReason, errorMsg string) *models.DeadLetterMessage {
	return models.NewDeadLetterMessage(
		"orders",
		0,
		12345,
		`{"test": "payload"}`,
		reason,
		errorMsg,
	)
}

// GenerateDeserializationError creates a DLQ message for deserialization failure.
func (f *DLQFixture) GenerateDeserializationError() *models.DeadLetterMessage {
	return f.GenerateDLQMessage(
		models.FailureReasonDeserialization,
		"json: cannot unmarshal string into Go struct field",
	)
}

// GenerateValidationError creates a DLQ message for validation failure.
func (f *DLQFixture) GenerateValidationError() *models.DeadLetterMessage {
	return f.GenerateDLQMessage(
		models.FailureReasonValidation,
		"order_id est requis",
	)
}

// GenerateTimeoutError creates a DLQ message for timeout failure.
func (f *DLQFixture) GenerateTimeoutError() *models.DeadLetterMessage {
	return f.GenerateDLQMessage(
		models.FailureReasonTimeout,
		"context deadline exceeded",
	)
}

// CustomerFixture generates test customers.
type CustomerFixture struct{}

// NewCustomerFixture creates a new customer fixture generator.
func NewCustomerFixture() *CustomerFixture {
	return &CustomerFixture{}
}

// GenerateCustomer creates a valid test customer.
func (f *CustomerFixture) GenerateCustomer() models.CustomerInfo {
	return models.CustomerInfo{
		CustomerID:   uuid.New().String(),
		Name:         "Test Customer",
		Email:        "customer@example.com",
		Phone:        "+33612345678",
		Address:      "456 Customer Street, Lyon, France 69001",
		LoyaltyLevel: "silver",
	}
}

// GenerateInvalidCustomer creates an invalid customer for error testing.
func (f *CustomerFixture) GenerateInvalidCustomer() models.CustomerInfo {
	return models.CustomerInfo{
		CustomerID:   "",  // Invalid: empty customer ID
		Name:         "",  // Invalid: empty name
		Email:        "not-an-email",  // Invalid: bad email format
	}
}

// MessageFixture generates test Kafka messages.
type MessageFixture struct {
	orderFixture *OrderFixture
}

// NewMessageFixture creates a new message fixture generator.
func NewMessageFixture() *MessageFixture {
	return &MessageFixture{
		orderFixture: NewOrderFixture(),
	}
}

// TestMessage represents a test Kafka message.
type TestMessage struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Timestamp time.Time
	Headers   map[string]string
}

// GenerateOrderMessage creates a test message containing an order.
func (f *MessageFixture) GenerateOrderMessage() (*TestMessage, error) {
	order := f.orderFixture.GenerateOrder()
	value, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}

	return &TestMessage{
		Topic:     "orders",
		Partition: 0,
		Offset:    1,
		Key:       []byte(order.OrderID),
		Value:     value,
		Timestamp: time.Now(),
		Headers: map[string]string{
			"content-type":   "application/json",
			"correlation-id": order.Metadata.CorrelationID,
		},
	}, nil
}

// GenerateMessages creates multiple test messages.
func (f *MessageFixture) GenerateMessages(count int) ([]*TestMessage, error) {
	messages := make([]*TestMessage, count)
	for i := 0; i < count; i++ {
		msg, err := f.GenerateOrderMessage()
		if err != nil {
			return nil, err
		}
		msg.Offset = int64(i + 1)
		messages[i] = msg
	}
	return messages, nil
}
