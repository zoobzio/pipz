package main

import (
	"testing"

	"pipz"
)

func TestValidateOrderID(t *testing.T) {
	tests := []struct {
		name    string
		order   Order
		wantErr bool
	}{
		{
			name: "valid order ID",
			order: Order{
				ID: "ORD-12345",
			},
			wantErr: false,
		},
		{
			name: "empty order ID",
			order: Order{
				ID: "",
			},
			wantErr: true,
		},
		{
			name: "missing prefix",
			order: Order{
				ID: "12345",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateOrderID(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOrderID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateItems(t *testing.T) {
	tests := []struct {
		name    string
		order   Order
		wantErr bool
	}{
		{
			name: "valid items",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
				},
			},
			wantErr: false,
		},
		{
			name: "no items",
			order: Order{
				Items: []OrderItem{},
			},
			wantErr: true,
		},
		{
			name: "negative quantity",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: -1, Price: 29.99},
				},
			},
			wantErr: true,
		},
		{
			name: "zero quantity",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 0, Price: 29.99},
				},
			},
			wantErr: true,
		},
		{
			name: "negative price",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 1, Price: -10.00},
				},
			},
			wantErr: true,
		},
		{
			name: "missing product ID",
			order: Order{
				Items: []OrderItem{
					{ProductID: "", Quantity: 1, Price: 29.99},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateItems(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateItems() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTotal(t *testing.T) {
	tests := []struct {
		name    string
		order   Order
		wantErr bool
	}{
		{
			name: "correct total",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
				},
				Total: 109.97,
			},
			wantErr: false,
		},
		{
			name: "incorrect total",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 3, Price: 19.99},
				},
				Total: 50.00,
			},
			wantErr: true,
		},
		{
			name: "total within epsilon",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 1, Price: 99.99},
				},
				Total: 99.995, // Within 0.01 epsilon
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateTotal(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTotal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationPipeline(t *testing.T) {
	pipeline := CreateValidationPipeline()

	tests := []struct {
		name    string
		order   Order
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid order",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-001",
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
				},
				Total: 109.97,
			},
			wantErr: false,
		},
		{
			name: "invalid ID stops pipeline",
			order: Order{
				ID:         "12345",
				CustomerID: "CUST-001",
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
				},
				Total: 59.98,
			},
			wantErr: true,
			errMsg:  "order ID must start with ORD-",
		},
		{
			name: "invalid items stops pipeline",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-001",
				Items:      []OrderItem{},
				Total:      0,
			},
			wantErr: true,
			errMsg:  "order must have at least one item",
		},
		{
			name: "invalid total fails validation",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-001",
				Items: []OrderItem{
					{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
				},
				Total: 100.00,
			},
			wantErr: true,
			errMsg:  "total mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Process() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err == nil && result.ID != tt.order.ID {
				t.Errorf("Process() returned different order ID: got %v, want %v", result.ID, tt.order.ID)
			}
		})
	}
}

func TestValidationPipelineChaining(t *testing.T) {
	// Create two separate validation pipelines
	idValidator := pipz.NewContract[Order]()
	idValidator.Register(pipz.Apply(ValidateOrderID))

	itemValidator := pipz.NewContract[Order]()
	itemValidator.Register(
		pipz.Apply(ValidateItems),
		pipz.Apply(ValidateTotal),
	)

	// Chain them together
	chain := pipz.NewChain[Order]()
	chain.Add(idValidator, itemValidator)

	// Test the chained validators
	validOrder := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-001",
		Items: []OrderItem{
			{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
		},
		Total: 59.98,
	}

	result, err := chain.Process(validOrder)
	if err != nil {
		t.Errorf("Chain.Process() unexpected error: %v", err)
	}
	if result.ID != validOrder.ID {
		t.Errorf("Chain.Process() returned different order ID: got %v, want %v", result.ID, validOrder.ID)
	}

	// Test with invalid order ID (should fail early)
	invalidOrder := Order{
		ID:         "12345",
		CustomerID: "CUST-001",
		Items: []OrderItem{
			{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
		},
		Total: 59.98,
	}

	_, err = chain.Process(invalidOrder)
	if err == nil {
		t.Error("Chain.Process() expected error for invalid order ID")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && findSubstring(s, substr) != -1)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}