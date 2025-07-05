package processors_test

import (
	"strings"
	"testing"

	"pipz"
	"pipz/demo/processors"
)

func TestValidationPipeline(t *testing.T) {
	// Register validation pipeline
	const testKey processors.ValidatorKey = "test"
	contract := pipz.GetContract[processors.ValidatorKey, processors.Order](testKey)
	
	err := contract.Register(
		processors.Adapt(processors.ValidateOrderID),
		processors.Adapt(processors.ValidateItems),
		processors.Adapt(processors.ValidateTotal),
	)
	if err != nil {
		t.Fatalf("Failed to register validation pipeline: %v", err)
	}

	t.Run("ValidOrder", func(t *testing.T) {
		order := processors.Order{
			ID:         "ORD-12345",
			CustomerID: "CUST-001",
			Items: []processors.OrderItem{
				{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
				{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
			},
			Total: 109.97, // 2*29.99 + 49.99
		}

		result, err := contract.Process(order)
		if err != nil {
			t.Fatalf("Valid order failed validation: %v", err)
		}

		if result.ID != order.ID {
			t.Errorf("Order ID changed during validation")
		}
	})

	t.Run("InvalidOrderID", func(t *testing.T) {
		order := processors.Order{
			ID: "12345", // Missing ORD- prefix
			Items: []processors.OrderItem{
				{ProductID: "P1", Quantity: 1, Price: 10.00},
			},
			Total: 10.00,
		}

		_, err := contract.Process(order)
		if err == nil {
			t.Fatal("Expected validation error for invalid order ID")
		}

		if !strings.Contains(err.Error(), "must start with ORD-") {
			t.Errorf("Wrong error message: %v", err)
		}
	})

	t.Run("EmptyOrder", func(t *testing.T) {
		order := processors.Order{
			ID:    "ORD-999",
			Items: []processors.OrderItem{}, // No items
			Total: 0,
		}

		_, err := contract.Process(order)
		if err == nil {
			t.Fatal("Expected validation error for empty order")
		}

		if !strings.Contains(err.Error(), "at least one item") {
			t.Errorf("Wrong error message: %v", err)
		}
	})

	t.Run("InvalidQuantity", func(t *testing.T) {
		order := processors.Order{
			ID: "ORD-123",
			Items: []processors.OrderItem{
				{ProductID: "P1", Quantity: 0, Price: 10.00}, // Zero quantity
			},
			Total: 0,
		}

		_, err := contract.Process(order)
		if err == nil {
			t.Fatal("Expected validation error for zero quantity")
		}

		if !strings.Contains(err.Error(), "quantity must be positive") {
			t.Errorf("Wrong error message: %v", err)
		}
	})

	t.Run("TotalMismatch", func(t *testing.T) {
		order := processors.Order{
			ID: "ORD-456",
			Items: []processors.OrderItem{
				{ProductID: "P1", Quantity: 2, Price: 25.00},
			},
			Total: 100.00, // Should be 50.00
		}

		_, err := contract.Process(order)
		if err == nil {
			t.Fatal("Expected validation error for total mismatch")
		}

		if !strings.Contains(err.Error(), "total mismatch") {
			t.Errorf("Wrong error message: %v", err)
		}
	})
}

func BenchmarkValidationPipeline(b *testing.B) {
	// Setup
	const benchKey processors.ValidatorKey = "bench"
	contract := pipz.GetContract[processors.ValidatorKey, processors.Order](benchKey)
	
	contract.Register(
		processors.Adapt(processors.ValidateOrderID),
		processors.Adapt(processors.ValidateItems),
		processors.Adapt(processors.ValidateTotal),
	)

	order := processors.Order{
		ID:         "ORD-BENCH",
		CustomerID: "BENCH",
		Items: []processors.OrderItem{
			{ProductID: "P1", Quantity: 5, Price: 19.99},
			{ProductID: "P2", Quantity: 3, Price: 29.99},
			{ProductID: "P3", Quantity: 1, Price: 99.99},
		},
		Total: 289.91,
	}

	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(order)
		if err != nil {
			b.Fatal(err)
		}
	}
}