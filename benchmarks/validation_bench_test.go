package benchmarks

import (
	"testing"

	"pipz"
	"pipz/examples"
)

func BenchmarkValidationPipeline(b *testing.B) {
	// Setup
	const benchKey examples.ValidatorKey = "bench"
	contract := pipz.GetContract[examples.Order](benchKey)
	
	contract.Register(
		pipz.Apply(examples.ValidateOrderID),
		pipz.Apply(examples.ValidateItems),
		pipz.Apply(examples.ValidateTotal),
	)

	order := examples.Order{
		ID:         "ORD-BENCH",
		CustomerID: "BENCH",
		Items: []examples.OrderItem{
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