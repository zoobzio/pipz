package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zoobzio/pipz"
)

func TestValidateOrderID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		order   Order
		wantErr string
	}{
		{
			name:    "valid order ID",
			order:   Order{ID: "ORD-12345"},
			wantErr: "",
		},
		{
			name:    "empty order ID",
			order:   Order{ID: ""},
			wantErr: "order ID required",
		},
		{
			name:    "wrong prefix",
			order:   Order{ID: "ORDER-12345"},
			wantErr: "order ID must start with ORD-",
		},
		{
			name:    "too short",
			order:   Order{ID: "ORD-1"},
			wantErr: "order ID too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateOrderID(ctx, tt.order)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestValidateItems(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		order   Order
		wantErr string
	}{
		{
			name: "valid items",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
				},
			},
			wantErr: "",
		},
		{
			name:    "no items",
			order:   Order{Items: []OrderItem{}},
			wantErr: "order must have at least one item",
		},
		{
			name: "missing product ID",
			order: Order{
				Items: []OrderItem{
					{ProductID: "", Quantity: 1, Price: 10.00},
				},
			},
			wantErr: "item 0: product ID required",
		},
		{
			name: "zero quantity",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 0, Price: 10.00},
				},
			},
			wantErr: "item 0: quantity must be positive",
		},
		{
			name: "negative price",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 1, Price: -10.00},
				},
			},
			wantErr: "item 0: price cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateItems(ctx, tt.order)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestValidateTotal(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		order   Order
		wantErr bool
	}{
		{
			name: "correct total",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
				},
				Total: 109.97,
			},
			wantErr: false,
		},
		{
			name: "incorrect total",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
				},
				Total: 100.00, // Wrong!
			},
			wantErr: true,
		},
		{
			name: "floating point tolerance",
			order: Order{
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 3, Price: 33.33},
				},
				Total: 99.99, // 3 * 33.33 = 99.99
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateTotal(ctx, tt.order)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidationPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateValidationPipeline()

	tests := []struct {
		name    string
		order   Order
		wantErr string
	}{
		{
			name: "valid order",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
				},
				Total: 109.97,
			},
			wantErr: "",
		},
		{
			name: "fails at first validator",
			order: Order{
				ID:         "", // Invalid
				CustomerID: "CUST-789",
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 1, Price: 10.00},
				},
				Total: 10.00,
			},
			wantErr: "order ID required",
		},
		{
			name: "fails at customer validation",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "789", // Invalid
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 1, Price: 10.00},
				},
				Total: 10.00,
			},
			wantErr: "customer ID must start with CUST-",
		},
		{
			name: "duplicate products",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 1, Price: 10.00},
					{ProductID: "PROD-001", Quantity: 2, Price: 10.00}, // Duplicate!
				},
				Total: 30.00,
			},
			wantErr: "duplicate product PROD-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pipeline.Process(ctx, tt.order)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestPipelineErrorContext(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateValidationPipeline()

	// Create an order that will fail at the total validation step
	order := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-789",
		Items: []OrderItem{
			{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
		},
		Total: 100.00, // Wrong total
	}

	_, err := pipeline.Process(ctx, order)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check that we get a PipelineError with context
	var pipelineErr *pipz.PipelineError[Order]
	if !errors.As(err, &pipelineErr) {
		t.Fatalf("expected PipelineError, got %T", err)
	}

	// Verify error context
	if pipelineErr.ProcessorName != "validate_total" {
		t.Errorf("expected processor name 'validate_total', got %q", pipelineErr.ProcessorName)
	}

	if pipelineErr.StageIndex != 3 { // 4th processor (0-indexed)
		t.Errorf("expected stage index 3, got %d", pipelineErr.StageIndex)
	}
}

func TestStrictValidationPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateStrictValidationPipeline()

	tests := []struct {
		name    string
		order   Order
		wantErr string
	}{
		{
			name: "too many items",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items:      make([]OrderItem, 101), // 101 items
				Total:      100.00,
			},
			wantErr: "order exceeds maximum item limit",
		},
		{
			name: "total too low",
			order: Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items: []OrderItem{
					{ProductID: "PROD-001", Quantity: 1, Price: 0.50},
				},
				Total: 0.50,
			},
			wantErr: "order total must be at least $1.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize items properly for the test
			if tt.name == "too many items" {
				for i := range tt.order.Items {
					tt.order.Items[i] = OrderItem{
						ProductID: fmt.Sprintf("PROD-%03d", i),
						Quantity:  1,
						Price:     1.00,
					}
				}
				tt.order.Total = float64(len(tt.order.Items))
			}

			_, err := pipeline.Process(ctx, tt.order)
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
