// Package validation demonstrates how to use pipz for complex data validation
// with composable validators and clear error messages.
package validation

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/zoobzio/pipz"
)

// Order represents a customer order with items and pricing
type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Total      float64
}

// OrderItem represents a single item in an order
type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// ValidateOrderID ensures the order has a valid ID format
func ValidateOrderID(_ context.Context, o Order) (Order, error) {
	if o.ID == "" {
		return Order{}, fmt.Errorf("order ID required")
	}
	if !strings.HasPrefix(o.ID, "ORD-") {
		return Order{}, fmt.Errorf("order ID must start with ORD-")
	}
	if len(o.ID) < 8 { // ORD-XXXX minimum
		return Order{}, fmt.Errorf("order ID too short")
	}
	return o, nil
}

// ValidateCustomerID ensures the order has a valid customer
func ValidateCustomerID(_ context.Context, o Order) (Order, error) {
	if o.CustomerID == "" {
		return Order{}, fmt.Errorf("customer ID required")
	}
	if !strings.HasPrefix(o.CustomerID, "CUST-") {
		return Order{}, fmt.Errorf("customer ID must start with CUST-")
	}
	return o, nil
}

// ValidateItems ensures all order items are valid
func ValidateItems(_ context.Context, o Order) (Order, error) {
	if len(o.Items) == 0 {
		return Order{}, fmt.Errorf("order must have at least one item")
	}

	for i, item := range o.Items {
		if item.ProductID == "" {
			return Order{}, fmt.Errorf("item %d: product ID required", i)
		}
		if item.Quantity <= 0 {
			return Order{}, fmt.Errorf("item %d: quantity must be positive", i)
		}
		if item.Price < 0 {
			return Order{}, fmt.Errorf("item %d: price cannot be negative", i)
		}
	}
	return o, nil
}

// ValidateTotal ensures the order total matches the sum of items
func ValidateTotal(_ context.Context, o Order) (Order, error) {
	calculated := 0.0
	for _, item := range o.Items {
		calculated += float64(item.Quantity) * item.Price
	}

	// Allow for small floating point differences
	if math.Abs(calculated-o.Total) > 0.01 {
		return Order{}, fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
	}
	return o, nil
}

// ValidateBusinessRules applies business-specific validation rules
func ValidateBusinessRules(_ context.Context, o Order) error {
	// Example business rules
	if o.Total > 10000 {
		// Large orders might need additional approval
		// This is where you'd check for approval status
		// For demo purposes, we'll just log it
	}

	// Check for duplicate items
	seen := make(map[string]bool)
	for _, item := range o.Items {
		if seen[item.ProductID] {
			return fmt.Errorf("duplicate product %s in order", item.ProductID)
		}
		seen[item.ProductID] = true
	}

	return nil
}

// CreateValidationPipeline creates a reusable order validation pipeline
func CreateValidationPipeline() *pipz.Pipeline[Order] {
	pipeline := pipz.NewPipeline[Order]()
	pipeline.Register(
		pipz.Apply("validate_order_id", ValidateOrderID),
		pipz.Apply("validate_customer_id", ValidateCustomerID),
		pipz.Apply("validate_items", ValidateItems),
		pipz.Apply("validate_total", ValidateTotal),
		pipz.Effect("business_rules", ValidateBusinessRules),
	)
	return pipeline
}

// CreateStrictValidationPipeline creates a validation pipeline with additional checks
func CreateStrictValidationPipeline() *pipz.Pipeline[Order] {
	pipeline := CreateValidationPipeline()

	// Add additional strict validations
	pipeline.PushTail(
		pipz.Effect("max_items", func(_ context.Context, o Order) error {
			if len(o.Items) > 100 {
				return fmt.Errorf("order exceeds maximum item limit (100)")
			}
			return nil
		}),
		pipz.Effect("min_total", func(_ context.Context, o Order) error {
			if o.Total < 1.00 {
				return fmt.Errorf("order total must be at least $1.00")
			}
			return nil
		}),
	)

	return pipeline
}

// Example shows how to use the validation pipeline
func Example() {
	// Create an order to validate
	order := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-789",
		Items: []OrderItem{
			{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
			{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
		},
		Total: 109.97, // 2*29.99 + 1*49.99
	}

	// Create and use the validation pipeline
	pipeline := CreateValidationPipeline()

	ctx := context.Background()
	validatedOrder, err := pipeline.Process(ctx, order)
	if err != nil {
		// In a real application, you'd handle the error appropriately
		fmt.Printf("Validation failed: %v\n", err)
		return
	}

	fmt.Printf("Order %s validated successfully\n", validatedOrder.ID)
}
