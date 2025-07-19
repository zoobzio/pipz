package validation_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zoobzio/pipz/examples/validation"
)

func BenchmarkValidation_SingleValidator(b *testing.B) {
	ctx := context.Background()
	order := validation.Order{ID: "ORD-12345"}

	b.Run("ValidateOrderID", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = validation.ValidateOrderID(ctx, order)
		}
	})

	b.Run("ValidateItems", func(b *testing.B) {
		order := validation.Order{
			Items: []validation.OrderItem{
				{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
				{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
				{ProductID: "PROD-003", Quantity: 3, Price: 15.99},
			},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = validation.ValidateItems(ctx, order)
		}
	})
}

func BenchmarkValidation_Pipeline(b *testing.B) {
	ctx := context.Background()
	pipeline := validation.CreateValidationPipeline()

	b.Run("ValidOrder", func(b *testing.B) {
		order := validation.Order{
			ID:         "ORD-12345",
			CustomerID: "CUST-789",
			Items: []validation.OrderItem{
				{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
				{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
			},
			Total: 109.97,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, order)
		}
	})

	b.Run("InvalidOrder_FailsFast", func(b *testing.B) {
		// Order that fails at first validation
		order := validation.Order{ID: "INVALID"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, order)
		}
	})
}

func BenchmarkValidation_OrderSize(b *testing.B) {
	ctx := context.Background()
	pipeline := validation.CreateValidationPipeline()

	sizes := []int{1, 10, 50, 100}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Items_%d", size), func(b *testing.B) {
			// Create order with specified number of items
			items := make([]validation.OrderItem, size)
			total := 0.0
			for i := range items {
				items[i] = validation.OrderItem{
					ProductID: fmt.Sprintf("PROD-%03d", i),
					Quantity:  i + 1,
					Price:     float64(i) + 0.99,
				}
				total += float64(items[i].Quantity) * items[i].Price
			}

			order := validation.Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items:      items,
				Total:      total,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, order)
			}
		})
	}
}
