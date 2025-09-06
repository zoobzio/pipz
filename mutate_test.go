package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMutate(t *testing.T) {
	t.Run("Mutate When Condition True", func(t *testing.T) {
		// Uppercase only long strings
		upperLong := Mutate("upper_long",
			func(_ context.Context, s string) string {
				return strings.ToUpper(s)
			},
			func(_ context.Context, s string) bool {
				return len(s) > 5
			},
		)

		// Long string gets transformed
		result, err := upperLong.Process(context.Background(), "hello world")
		if err != nil {
			t.Fatalf("mutate should not return error: %v", err)
		}
		if result != "HELLO WORLD" {
			t.Errorf("expected HELLO WORLD, got %s", result)
		}
	})

	t.Run("Mutate When Condition False", func(t *testing.T) {
		// Uppercase only long strings
		upperLong := Mutate("upper_long",
			func(_ context.Context, s string) string {
				return strings.ToUpper(s)
			},
			func(_ context.Context, s string) bool {
				return len(s) > 5
			},
		)

		// Short string passes through unchanged
		result, err := upperLong.Process(context.Background(), "hi")
		if err != nil {
			t.Fatalf("mutate should not return error: %v", err)
		}
		if result != "hi" {
			t.Errorf("expected unchanged value, got %s", result)
		}
	})

	t.Run("Mutate Never Returns Error", func(t *testing.T) {
		// Even if we do something that might panic, no error
		divider := Mutate("divide_even",
			func(_ context.Context, n int) int {
				return 100 / n // Could panic if n is 0
			},
			func(_ context.Context, n int) bool {
				return n%2 == 0 && n != 0 // Protect against divide by zero
			},
		)

		// Safe case
		result, err := divider.Process(context.Background(), 10)
		if err != nil {
			t.Fatalf("mutate should not return error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// Would panic but condition is false
		result, err = divider.Process(context.Background(), 0)
		if err != nil {
			t.Fatalf("mutate should not return error: %v", err)
		}
		if result != 0 {
			t.Errorf("expected unchanged value, got %d", result)
		}
	})

	t.Run("Mutate Complex Condition", func(t *testing.T) {
		type User struct {
			Name     string
			Age      int
			Premium  bool
			Discount float64
		}

		// Apply discount for premium users over 65
		applyDiscount := Mutate("senior_discount",
			func(_ context.Context, u User) User {
				u.Discount = 0.2 // 20% discount
				return u
			},
			func(_ context.Context, u User) bool {
				return u.Premium && u.Age >= 65
			},
		)

		tests := []struct {
			name     string
			user     User
			expected float64
		}{
			{
				name:     "Premium Senior",
				user:     User{Name: "Alice", Age: 70, Premium: true},
				expected: 0.2,
			},
			{
				name:     "Non-Premium Senior",
				user:     User{Name: "Bob", Age: 70, Premium: false},
				expected: 0.0,
			},
			{
				name:     "Premium Non-Senior",
				user:     User{Name: "Charlie", Age: 30, Premium: true},
				expected: 0.0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := applyDiscount.Process(context.Background(), tt.user)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.Discount != tt.expected {
					t.Errorf("expected discount %f, got %f", tt.expected, result.Discount)
				}
			})
		}
	})

	t.Run("Mutate panic recovery in condition", func(t *testing.T) {
		// Panic in condition function
		panicCondition := Mutate("panic_condition",
			func(_ context.Context, s string) string { return s + "_transformed" },
			func(_ context.Context, _ string) bool { panic("condition panic") },
		)

		result, err := panicCondition.Process(context.Background(), "test")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.InputData != "test" {
			t.Errorf("expected input data 'test', got %q", pipzErr.InputData)
		}

		// Check that panic message is properly wrapped
		var panicErr *panicError
		if !errors.As(pipzErr.Err, &panicErr) {
			t.Fatal("expected panicError")
		}

		expectedMsg := "panic occurred: condition panic"
		if panicErr.sanitized != expectedMsg {
			t.Errorf("expected %q, got %q", expectedMsg, panicErr.sanitized)
		}
	})

	t.Run("Mutate panic recovery in transformer", func(t *testing.T) {
		// Panic in transformer function
		panicTransformer := Mutate("panic_transformer",
			func(_ context.Context, _ string) string { panic("transformer panic") },
			func(_ context.Context, _ string) bool { return true },
		)

		result, err := panicTransformer.Process(context.Background(), "test")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.InputData != "test" {
			t.Errorf("expected input data 'test', got %q", pipzErr.InputData)
		}

		// Check that panic message is properly wrapped
		var panicErr *panicError
		if !errors.As(pipzErr.Err, &panicErr) {
			t.Fatal("expected panicError")
		}

		expectedMsg := "panic occurred: transformer panic"
		if panicErr.sanitized != expectedMsg {
			t.Errorf("expected %q, got %q", expectedMsg, panicErr.sanitized)
		}
	})
}
