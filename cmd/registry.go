package main

import (
	"context"
	"testing"
)

// Example defines the interface that all examples must implement
type Example interface {
	Name() string
	Description() string
	Demo(ctx context.Context) error
	Benchmark(b *testing.B) error
}

// getAllExamples returns all registered examples in a consistent order
func getAllExamples() []Example {
	return []Example{
		&ValidationExample{},
		&SecurityExample{},
		// &TransformExample{},
		// &MiddlewareExample{},
		// &PaymentExample{},
	}
}

// getExampleByName returns a specific example by name
func getExampleByName(name string) (Example, bool) {
	for _, ex := range getAllExamples() {
		if ex.Name() == name {
			return ex, true
		}
	}
	return nil, false
}
