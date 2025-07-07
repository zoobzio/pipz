package benchmarks

import (
	"fmt"
	"math"
	"strings"
	"testing"
	
	"pipz"
)

// Order types (copied from examples for benchmarking)
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

func BenchmarkValidationPipeline(b *testing.B) {
	// Setup pipeline
	pipeline := pipz.NewContract[Order]()
	pipeline.Register(
		pipz.Apply(validateOrderID),
		pipz.Apply(validateItems),
		pipz.Apply(validateTotal),
	)

	order := Order{
		ID:         "ORD-BENCH",
		CustomerID: "BENCH",
		Items: []OrderItem{
			{ProductID: "P1", Quantity: 5, Price: 19.99},
			{ProductID: "P2", Quantity: 3, Price: 29.99},
			{ProductID: "P3", Quantity: 1, Price: 99.99},
		},
		Total: 289.91,
	}

	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Process(order)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidationPipelineParallel(b *testing.B) {
	// Setup pipeline
	pipeline := pipz.NewContract[Order]()
	pipeline.Register(
		pipz.Apply(validateOrderID),
		pipz.Apply(validateItems),
		pipz.Apply(validateTotal),
	)

	order := Order{
		ID:         "ORD-BENCH",
		CustomerID: "BENCH",
		Items: []OrderItem{
			{ProductID: "P1", Quantity: 5, Price: 19.99},
			{ProductID: "P2", Quantity: 3, Price: 29.99},
			{ProductID: "P3", Quantity: 1, Price: 99.99},
		},
		Total: 289.91,
	}

	b.ResetTimer()
	
	// Benchmark parallel execution
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := pipeline.Process(order)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark individual validators
func BenchmarkValidateOrderID(b *testing.B) {
	order := Order{
		ID:         "ORD-BENCH",
		CustomerID: "BENCH",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validateOrderID(order)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateItems(b *testing.B) {
	order := Order{
		Items: []OrderItem{
			{ProductID: "P1", Quantity: 5, Price: 19.99},
			{ProductID: "P2", Quantity: 3, Price: 29.99},
			{ProductID: "P3", Quantity: 1, Price: 99.99},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validateItems(order)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateTotal(b *testing.B) {
	order := Order{
		Items: []OrderItem{
			{ProductID: "P1", Quantity: 5, Price: 19.99},
			{ProductID: "P2", Quantity: 3, Price: 29.99},
			{ProductID: "P3", Quantity: 1, Price: 99.99},
		},
		Total: 289.91,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validateTotal(order)
		if err != nil {
			b.Fatal(err)
		}
	}
}