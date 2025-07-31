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
