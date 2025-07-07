package main

import (
	"fmt"
	"log"
	"math"
	"strings"
	
	"pipz"
)

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

// CreateValidationPipeline creates a pipeline for order validation
func CreateValidationPipeline() *pipz.Contract[Order] {
	pipeline := pipz.NewContract[Order]()
	pipeline.Register(
		pipz.Apply(ValidateOrderID),
		pipz.Apply(ValidateItems),
		pipz.Apply(ValidateTotal),
	)
	return pipeline
}

func main() {
	// Create the validation pipeline
	validator := CreateValidationPipeline()
	
	// Valid order
	validOrder := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-001",
		Items: []OrderItem{
			{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
			{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
		},
		Total: 109.97, // (2 * 29.99) + 49.99
	}
	
	fmt.Println("Validating valid order...")
	result, err := validator.Process(validOrder)
	if err != nil {
		log.Printf("Validation failed: %v", err)
	} else {
		fmt.Printf("✓ Order %s validated successfully\n", result.ID)
	}
	
	// Invalid order - wrong total
	invalidOrder := Order{
		ID:         "ORD-12346",
		CustomerID: "CUST-002",
		Items: []OrderItem{
			{ProductID: "PROD-C", Quantity: 3, Price: 19.99},
		},
		Total: 50.00, // Wrong! Should be 59.97
	}
	
	fmt.Println("\nValidating invalid order...")
	_, err = validator.Process(invalidOrder)
	if err != nil {
		fmt.Printf("✗ Validation failed: %v\n", err)
	}
	
	// Invalid order - missing order ID prefix
	badIDOrder := Order{
		ID:         "12347", // Missing ORD- prefix
		CustomerID: "CUST-003",
		Items: []OrderItem{
			{ProductID: "PROD-D", Quantity: 1, Price: 99.99},
		},
		Total: 99.99,
	}
	
	fmt.Println("\nValidating order with bad ID...")
	_, err = validator.Process(badIDOrder)
	if err != nil {
		fmt.Printf("✗ Validation failed: %v\n", err)
	}
}