package examples

import (
	"fmt"
	"math"
	"strings"
)

// ValidatorKey is the contract key type for validation pipelines.
type ValidatorKey string

// Order represents an e-commerce order.
type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Total      float64
}

// OrderItem represents a single item in an order.
type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// ValidateOrderID ensures the order ID follows the required format.
// Orders must start with "ORD-" prefix.
func ValidateOrderID(o Order) (Order, error) {
	if o.ID == "" {
		return o, fmt.Errorf("order ID required")
	}
	if !strings.HasPrefix(o.ID, "ORD-") {
		return o, fmt.Errorf("order ID must start with ORD-")
	}
	return o, nil
}

// ValidateItems ensures all order items have valid quantities and prices.
func ValidateItems(o Order) (Order, error) {
	if len(o.Items) == 0 {
		return o, fmt.Errorf("order must have at least one item")
	}
	
	for i, item := range o.Items {
		if item.Quantity <= 0 {
			return o, fmt.Errorf("item %d: quantity must be positive", i)
		}
		if item.Price < 0 {
			return o, fmt.Errorf("item %d: price cannot be negative", i)
		}
		if item.ProductID == "" {
			return o, fmt.Errorf("item %d: product ID required", i)
		}
	}
	
	return o, nil
}

// ValidateTotal ensures the order total matches the sum of item prices.
// Uses a small epsilon for floating-point comparison.
func ValidateTotal(o Order) (Order, error) {
	calculated := 0.0
	for _, item := range o.Items {
		calculated += float64(item.Quantity) * item.Price
	}
	
	// Use epsilon for floating-point comparison
	if math.Abs(calculated-o.Total) > 0.01 {
		return o, fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
	}
	
	return o, nil
}