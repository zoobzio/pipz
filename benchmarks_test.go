package pipz

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Focused benchmarks for pipz - measuring what actually matters for performance

// BenchmarkCoreProcessors measures the fundamental processing primitives.
func BenchmarkCoreProcessors(b *testing.B) {
	ctx := context.Background()
	data := 42

	b.Run("Apply/Success", func(b *testing.B) {
		processor := Apply("benchmark", func(_ context.Context, _ int) (int, error) {
			return 84, nil // 42 * 2
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Apply/Error", func(b *testing.B) {
		processor := Apply("benchmark", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("benchmark error")
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Process(ctx, data) //nolint:errcheck // benchmarking error path performance
		}
	})

	b.Run("Transform", func(b *testing.B) {
		processor := Transform("benchmark", func(_ context.Context, n int) int {
			return n * 2
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Effect", func(b *testing.B) {
		processor := Effect("benchmark", func(_ context.Context, _ int) error {
			return nil
		})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkComposition measures how processors combine (most common patterns).
func BenchmarkComposition(b *testing.B) {
	ctx := context.Background()
	data := 42

	// Simple processors for composition
	double := Transform("double", func(_ context.Context, n int) int { return n * 2 })
	add10 := Transform("add10", func(_ context.Context, n int) int { return n + 10 })
	validate := Apply("validate", func(_ context.Context, n int) (int, error) {
		if n < 0 {
			return 0, errors.New("negative")
		}
		return n, nil
	})

	b.Run("Sequence/Short", func(b *testing.B) {
		seq := NewSequence[int]("short")
		seq.Register(double, add10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Sequence/Long", func(b *testing.B) {
		seq := NewSequence[int]("long")
		seq.Register(double, add10, validate, double, add10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Fallback/Primary", func(b *testing.B) {
		fallback := NewFallback[int]("fallback", double, add10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := fallback.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Fallback/Secondary", func(b *testing.B) {
		failing := Apply("fail", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("always fails")
		})
		fallback := NewFallback[int]("fallback", failing, double)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := fallback.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkErrorHandling measures error processing overhead.
func BenchmarkErrorHandling(b *testing.B) {
	ctx := context.Background()
	data := 42

	b.Run("Handle/NoError", func(b *testing.B) {
		processor := Apply("success", func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		handler := NewHandle[int]("handler", processor, Effect("noop", func(_ context.Context, _ *Error[int]) error {
			return nil
		}))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := handler.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Handle/WithError", func(b *testing.B) {
		processor := Apply("error", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("benchmark error")
		})
		handler := NewHandle[int]("handler", processor, Effect("noop", func(_ context.Context, _ *Error[int]) error {
			return nil
		}))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = handler.Process(ctx, data) //nolint:errcheck // Handle passes through errors after processing them
		}
	})

	b.Run("Retry/Success", func(b *testing.B) {
		processor := Apply("success", func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		retry := NewRetry[int]("retry", processor, 3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := retry.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Retry/Failure", func(b *testing.B) {
		processor := Apply("error", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("always fails")
		})
		retry := NewRetry[int]("retry", processor, 3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, data) //nolint:errcheck // benchmarking retry failure performance
		}
	})
}

// BenchmarkRealWorld measures patterns from actual usage.
func BenchmarkRealWorld(b *testing.B) {
	ctx := context.Background()

	// Simulate validation pipeline (common pattern)
	b.Run("ValidationPipeline", func(b *testing.B) {
		validate := Apply("validate", func(_ context.Context, s string) (string, error) {
			if s == "" {
				return "", errors.New("empty string")
			}
			return s, nil
		})
		normalize := Transform("normalize", func(_ context.Context, s string) string {
			return s + "_normalized"
		})
		enrich := Apply("enrich", func(_ context.Context, s string) (string, error) {
			return s + "_enriched", nil
		})

		pipeline := NewSequence[string]("validation")
		pipeline.Register(validate, normalize, enrich)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := pipeline.Process(ctx, "test_data")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Simulate retry with backoff (common resilience pattern)
	b.Run("RetryBackoff", func(b *testing.B) {
		attempts := 0
		flaky := Apply("flaky", func(_ context.Context, n int) (int, error) {
			attempts++
			if attempts%3 == 0 { // Succeed every 3rd attempt
				return n * 2, nil
			}
			return 0, errors.New("flaky error")
		})

		retry := NewBackoff[int]("retry", flaky, 3, time.Microsecond)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			attempts = 0 // Reset for each iteration
			_, err := retry.Process(ctx, 42)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Simulate timeout protection
	b.Run("TimeoutProtection", func(b *testing.B) {
		fast := Apply("fast", func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})

		timeout := NewTimeout[int]("timeout", fast, time.Second)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := timeout.Process(ctx, 42)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkFilter benchmarks filter operations.
func BenchmarkFilter(b *testing.B) {
	ctx := context.Background()

	b.Run("ConditionFalse", func(b *testing.B) {
		filter := NewFilter("bench-filter",
			func(_ context.Context, data int) bool { return data < 0 },
			Transform("double", func(_ context.Context, data int) int { return data * 2 }))

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = filter.Process(ctx, 5) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("ConditionTrue", func(b *testing.B) {
		filter := NewFilter("bench-filter",
			func(_ context.Context, data int) bool { return data > 0 },
			Transform("double", func(_ context.Context, data int) int { return data * 2 }))

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = filter.Process(ctx, 5) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("ComplexCondition", func(b *testing.B) {
		filter := NewFilter("complex-filter",
			func(_ context.Context, data int) bool {
				return data%2 == 0 && data > 10 && data < 1000
			},
			Transform("complex", func(_ context.Context, data int) int {
				return data*3 + 7
			}))

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = filter.Process(ctx, 20) //nolint:errcheck // benchmark ignores errors
		}
	})
}

// BenchmarkContest benchmarks contest operations.
func BenchmarkContest(b *testing.B) {
	ctx := context.Background()

	b.Run("FirstWinner", func(b *testing.B) {
		// Simple condition that checks if value is even
		condition := func(_ context.Context, _ TestData) bool {
			return true // Always return true for benchmarking
		}

		// Create processors with different values
		p1 := Transform("odd", func(_ context.Context, d TestData) TestData {
			d.Value = 101 // Odd - won't win
			return d
		})
		p2 := Transform("even-slow", func(_ context.Context, d TestData) TestData {
			time.Sleep(time.Microsecond) // Small delay
			d.Value = 100                // Even - could win but slower
			return d
		})
		p3 := Transform("even-fast", func(_ context.Context, d TestData) TestData {
			d.Value = 50 // Even - should win (fastest)
			return d
		})

		contest := NewContest("bench-contest", condition, p1, p2, p3)
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := contest.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NoWinner", func(b *testing.B) {
		// Condition that nothing will meet
		condition := func(_ context.Context, _ TestData) bool {
			return false // Always return false for benchmarking
		}

		p1 := Transform("p1", func(_ context.Context, d TestData) TestData {
			d.Value = 10
			return d
		})
		p2 := Transform("p2", func(_ context.Context, d TestData) TestData {
			d.Value = 20
			return d
		})
		p3 := Transform("p3", func(_ context.Context, d TestData) TestData {
			d.Value = 30
			return d
		})

		contest := NewContest("bench-no-winner", condition, p1, p2, p3)
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = contest.Process(ctx, data) //nolint:errcheck // benchmarking no-winner scenario performance
		}
	})

	b.Run("ComplexCondition", func(b *testing.B) {
		// More complex condition
		condition := func(_ context.Context, _ TestData) bool {
			return true // Simplified for benchmarking
		}

		processors := make([]Chainable[TestData], 10)
		for i := 0; i < 10; i++ {
			val := i * 10
			processors[i] = Transform("p", func(_ context.Context, d TestData) TestData {
				d.Value = val
				return d
			})
		}

		contest := NewContest("bench-complex", condition, processors...)
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := contest.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("VsRace", func(b *testing.B) {
		// Compare Contest with a simple "first wins" condition against Race
		b.Run("Contest", func(b *testing.B) {
			// Condition that always returns true (equivalent to Race behavior)
			condition := func(_ context.Context, _ TestData) bool {
				return true
			}

			p1 := Transform("p1", func(_ context.Context, d TestData) TestData {
				d.Value = 100
				return d
			})
			p2 := Transform("p2", func(_ context.Context, d TestData) TestData {
				time.Sleep(time.Microsecond)
				d.Value = 200
				return d
			})

			contest := NewContest("contest", condition, p1, p2)
			data := TestData{Value: 1}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := contest.Process(ctx, data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("Race", func(b *testing.B) {
			p1 := Transform("p1", func(_ context.Context, d TestData) TestData {
				d.Value = 100
				return d
			})
			p2 := Transform("p2", func(_ context.Context, d TestData) TestData {
				time.Sleep(time.Microsecond)
				d.Value = 200
				return d
			})

			race := NewRace("race", p1, p2)
			data := TestData{Value: 1}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := race.Process(ctx, data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
