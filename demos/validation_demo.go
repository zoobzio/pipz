package main

import (
	"fmt"
	"math"
	"strings"

	"pipz"
)

// Order types for validation demo
type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Total      float64
}

type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// Validation functions
func validateOrderID(o Order) (Order, error) {
	if o.ID == "" {
		return Order{}, fmt.Errorf("order ID required")
	}
	if !strings.HasPrefix(o.ID, "ORD-") {
		return Order{}, fmt.Errorf("order ID must start with ORD-")
	}
	return o, nil
}

func validateItems(o Order) (Order, error) {
	if len(o.Items) == 0 {
		return Order{}, fmt.Errorf("order must have at least one item")
	}
	for i, item := range o.Items {
		if item.Quantity <= 0 {
			return Order{}, fmt.Errorf("item %d: quantity must be positive", i)
		}
		if item.Price < 0 {
			return Order{}, fmt.Errorf("item %d: price cannot be negative", i)
		}
	}
	return o, nil
}

func validateTotal(o Order) (Order, error) {
	calculated := 0.0
	for _, item := range o.Items {
		calculated += float64(item.Quantity) * item.Price
	}
	if math.Abs(calculated-o.Total) > 0.01 {
		return Order{}, fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
	}
	return o, nil
}

func runValidationDemo() {
	section("DATA VALIDATION PIPELINE")
	
	info("Use Case: E-commerce Order Validation")
	info("• Validate order ID format (must start with 'ORD-')")
	info("• Ensure all items have valid quantities and prices")
	info("• Verify order total matches sum of items")

	// Create validation pipeline
	validator := pipz.NewContract[Order]()
	validator.Register(
		pipz.Apply(validateOrderID),
		pipz.Apply(validateItems),
		pipz.Apply(validateTotal),
	)

	code("go", `// Create validation pipeline
validator := pipz.NewContract[Order]()
validator.Register(
    pipz.Apply(validateOrderID),
    pipz.Apply(validateItems),
    pipz.Apply(validateTotal),
)`)

	// Test Case 1: Valid Order
	fmt.Println("\n📝 Test Case 1: Valid Order")
	validOrder := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-001",
		Items: []OrderItem{
			{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
			{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
		},
		Total: 109.97, // (2 * 29.99) + 49.99
	}
	
	result, err := validator.Process(validOrder)
	if err != nil {
		showError(fmt.Sprintf("Validation failed: %v", err))
	} else {
		success(fmt.Sprintf("Order %s validated successfully", result.ID))
	}

	// Test Case 2: Invalid Order ID
	fmt.Println("\n📝 Test Case 2: Invalid Order ID")
	invalidIDOrder := Order{
		ID:         "12345", // Missing ORD- prefix
		CustomerID: "CUST-002",
		Items: []OrderItem{
			{ProductID: "PROD-C", Quantity: 1, Price: 99.99},
		},
		Total: 99.99,
	}
	
	_, err = validator.Process(invalidIDOrder)
	if err != nil {
		showError(fmt.Sprintf("Validation failed as expected: %v", err))
	}

	// Test Case 3: Total Mismatch
	fmt.Println("\n📝 Test Case 3: Total Mismatch")
	mismatchOrder := Order{
		ID:         "ORD-12346",
		CustomerID: "CUST-003",
		Items: []OrderItem{
			{ProductID: "PROD-D", Quantity: 3, Price: 19.99},
		},
		Total: 50.00, // Wrong! Should be 59.97
	}
	
	_, err = validator.Process(mismatchOrder)
	if err != nil {
		showError(fmt.Sprintf("Validation failed as expected: %v", err))
	}

	// Demonstrate early exit
	fmt.Println("\n🔍 Pipeline Behavior:")
	info("• Pipelines exit on first error (fail-fast)")
	info("• Returns zero value on error (Go convention)")
	info("• Each validator is independent and reusable")
}