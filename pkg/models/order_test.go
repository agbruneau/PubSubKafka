package models

import (
	"testing"
)

// TestOrderItemValidate tests the OrderItem.Validate method with table-driven tests.
func TestOrderItemValidate(t *testing.T) {
	tests := []struct {
		name    string
		item    OrderItem
		wantErr bool
		errType error
	}{
		{
			name: "Valid item",
			item: OrderItem{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   2,
				UnitPrice:  3.50,
				TotalPrice: 7.00,
			},
			wantErr: false,
		},
		{
			name: "Empty item ID",
			item: OrderItem{
				ItemID:     "",
				ItemName:   "Espresso",
				Quantity:   2,
				UnitPrice:  3.50,
				TotalPrice: 7.00,
			},
			wantErr: true,
			errType: ErrEmptyItemID,
		},
		{
			name: "Empty item name",
			item: OrderItem{
				ItemID:     "item-001",
				ItemName:   "",
				Quantity:   2,
				UnitPrice:  3.50,
				TotalPrice: 7.00,
			},
			wantErr: true,
			errType: ErrEmptyItemName,
		},
		{
			name: "Zero quantity",
			item: OrderItem{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   0,
				UnitPrice:  3.50,
				TotalPrice: 0.00,
			},
			wantErr: true,
			errType: ErrInvalidQuantity,
		},
		{
			name: "Negative quantity",
			item: OrderItem{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   -1,
				UnitPrice:  3.50,
				TotalPrice: -3.50,
			},
			wantErr: true,
			errType: ErrInvalidQuantity,
		},
		{
			name: "Negative price",
			item: OrderItem{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   2,
				UnitPrice:  -3.50,
				TotalPrice: -7.00,
			},
			wantErr: true,
			errType: ErrInvalidPrice,
		},
		{
			name: "Incorrect total",
			item: OrderItem{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   2,
				UnitPrice:  3.50,
				TotalPrice: 10.00, // Should be 7.00
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("OrderItem.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCustomerInfoValidate tests the CustomerInfo.Validate method.
func TestCustomerInfoValidate(t *testing.T) {
	tests := []struct {
		name     string
		customer CustomerInfo
		wantErr  bool
	}{
		{
			name: "Valid customer",
			customer: CustomerInfo{
				CustomerID:   "cust-001",
				Name:         "John Doe",
				Email:        "john@example.com",
				Phone:        "+33 6 00 00 00 00",
				Address:      "123 Main St",
				LoyaltyLevel: "gold",
			},
			wantErr: false,
		},
		{
			name: "Valid customer without email",
			customer: CustomerInfo{
				CustomerID: "cust-001",
				Name:       "John Doe",
			},
			wantErr: false,
		},
		{
			name: "Empty customer ID",
			customer: CustomerInfo{
				CustomerID: "",
				Name:       "John Doe",
			},
			wantErr: true,
		},
		{
			name: "Empty name",
			customer: CustomerInfo{
				CustomerID: "cust-001",
				Name:       "",
			},
			wantErr: true,
		},
		{
			name: "Invalid email format",
			customer: CustomerInfo{
				CustomerID: "cust-001",
				Name:       "John Doe",
				Email:      "invalid-email",
			},
			wantErr: true,
		},
		{
			name: "Valid email with subdomain",
			customer: CustomerInfo{
				CustomerID: "cust-001",
				Name:       "John Doe",
				Email:      "john@mail.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.customer.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CustomerInfo.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestOrderValidate tests the Order.Validate method.
func TestOrderValidate(t *testing.T) {
	validItem := OrderItem{
		ItemID:     "item-001",
		ItemName:   "Espresso",
		Quantity:   2,
		UnitPrice:  3.50,
		TotalPrice: 7.00,
	}

	validCustomer := CustomerInfo{
		CustomerID: "cust-001",
		Name:       "John Doe",
		Email:      "john@example.com",
	}

	tests := []struct {
		name    string
		order   Order
		wantErr bool
	}{
		{
			name: "Valid order",
			order: Order{
				OrderID:      "order-123",
				Sequence:     1,
				Status:       "pending",
				Items:        []OrderItem{validItem},
				SubTotal:     7.00,
				Tax:          1.40,
				ShippingFee:  2.50,
				Total:        10.90,
				CustomerInfo: validCustomer,
			},
			wantErr: false,
		},
		{
			name: "Empty order ID",
			order: Order{
				OrderID:      "",
				Sequence:     1,
				Status:       "pending",
				Items:        []OrderItem{validItem},
				SubTotal:     7.00,
				Tax:          1.40,
				ShippingFee:  2.50,
				Total:        10.90,
				CustomerInfo: validCustomer,
			},
			wantErr: true,
		},
		{
			name: "Invalid sequence",
			order: Order{
				OrderID:      "order-123",
				Sequence:     0,
				Status:       "pending",
				Items:        []OrderItem{validItem},
				SubTotal:     7.00,
				Tax:          1.40,
				ShippingFee:  2.50,
				Total:        10.90,
				CustomerInfo: validCustomer,
			},
			wantErr: true,
		},
		{
			name: "Empty status",
			order: Order{
				OrderID:      "order-123",
				Sequence:     1,
				Status:       "",
				Items:        []OrderItem{validItem},
				SubTotal:     7.00,
				Tax:          1.40,
				ShippingFee:  2.50,
				Total:        10.90,
				CustomerInfo: validCustomer,
			},
			wantErr: true,
		},
		{
			name: "No items",
			order: Order{
				OrderID:      "order-123",
				Sequence:     1,
				Status:       "pending",
				Items:        []OrderItem{},
				SubTotal:     0,
				Tax:          0,
				ShippingFee:  0,
				Total:        0,
				CustomerInfo: validCustomer,
			},
			wantErr: true,
		},
		{
			name: "Invalid total",
			order: Order{
				OrderID:      "order-123",
				Sequence:     1,
				Status:       "pending",
				Items:        []OrderItem{validItem},
				SubTotal:     7.00,
				Tax:          1.40,
				ShippingFee:  2.50,
				Total:        100.00, // Wrong total
				CustomerInfo: validCustomer,
			},
			wantErr: true,
		},
		{
			name: "Invalid customer",
			order: Order{
				OrderID:     "order-123",
				Sequence:    1,
				Status:      "pending",
				Items:       []OrderItem{validItem},
				SubTotal:    7.00,
				Tax:         1.40,
				ShippingFee: 2.50,
				Total:       10.90,
				CustomerInfo: CustomerInfo{
					CustomerID: "",
					Name:       "John",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Order.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestOrderIsValid tests the Order.IsValid method.
func TestOrderIsValid(t *testing.T) {
	validOrder := Order{
		OrderID:  "order-123",
		Sequence: 1,
		Status:   "pending",
		Items: []OrderItem{
			{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   2,
				UnitPrice:  3.50,
				TotalPrice: 7.00,
			},
		},
		SubTotal:    7.00,
		Tax:         1.40,
		ShippingFee: 2.50,
		Total:       10.90,
		CustomerInfo: CustomerInfo{
			CustomerID: "cust-001",
			Name:       "John Doe",
		},
	}

	invalidOrder := Order{
		OrderID: "",
	}

	if !validOrder.IsValid() {
		t.Error("Expected valid order to return true from IsValid()")
	}

	if invalidOrder.IsValid() {
		t.Error("Expected invalid order to return false from IsValid()")
	}
}

// TestMultipleItemsSubtotal tests that subtotal is correctly validated with multiple items.
func TestMultipleItemsSubtotal(t *testing.T) {
	order := Order{
		OrderID:  "order-123",
		Sequence: 1,
		Status:   "pending",
		Items: []OrderItem{
			{
				ItemID:     "item-001",
				ItemName:   "Espresso",
				Quantity:   2,
				UnitPrice:  3.50,
				TotalPrice: 7.00,
			},
			{
				ItemID:     "item-002",
				ItemName:   "Cappuccino",
				Quantity:   1,
				UnitPrice:  4.00,
				TotalPrice: 4.00,
			},
		},
		SubTotal:    11.00, // 7.00 + 4.00
		Tax:         2.20,
		ShippingFee: 2.50,
		Total:       15.70, // 11.00 + 2.20 + 2.50
		CustomerInfo: CustomerInfo{
			CustomerID: "cust-001",
			Name:       "John Doe",
		},
	}

	if err := order.Validate(); err != nil {
		t.Errorf("Expected valid order with multiple items, got error: %v", err)
	}
}

