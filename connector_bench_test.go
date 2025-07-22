package pipz_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// BenchmarkSequential measures the overhead of Sequential connector.
func BenchmarkSequential(b *testing.B) {
	ctx := context.Background()

	b.Run("Empty", func(b *testing.B) {
		seq := pipz.Sequential[int]()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = seq.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("SingleProcessor", func(b *testing.B) {
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		seq := pipz.Sequential(proc)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = seq.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("ThreeProcessors", func(b *testing.B) {
		proc1 := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 1, nil
		})
		proc2 := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		proc3 := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n - 5, nil
		})
		seq := pipz.Sequential(proc1, proc2, proc3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = seq.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("TenProcessors", func(b *testing.B) {
		procs := make([]pipz.Chainable[int], 10)
		for i := 0; i < 10; i++ {
			procs[i] = pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n + 1, nil
			})
		}
		seq := pipz.Sequential(procs...)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = seq.Process(ctx, 0) //nolint:errcheck
		}
	})
}

// BenchmarkSwitch measures the overhead of Switch connector.
func BenchmarkSwitch(b *testing.B) {
	ctx := context.Background()

	b.Run("TwoRoutes", func(b *testing.B) {
		condition := func(_ context.Context, n int) string {
			if n%2 == 0 {
				return "even"
			}
			return "odd"
		}
		routes := map[string]pipz.Chainable[int]{
			"even": pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n * 2, nil
			}),
			"odd": pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n + 1, nil
			}),
		}
		sw := pipz.Switch(condition, routes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = sw.Process(ctx, i) //nolint:errcheck
		}
	})

	b.Run("FiveRoutes", func(b *testing.B) {
		condition := func(_ context.Context, n int) string {
			return fmt.Sprintf("route_%d", n%5)
		}
		routes := make(map[string]pipz.Chainable[int], 5)
		for i := 0; i < 5; i++ {
			idx := i
			routes[fmt.Sprintf("route_%d", i)] = pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n * (idx + 1), nil
			})
		}
		sw := pipz.Switch(condition, routes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = sw.Process(ctx, i) //nolint:errcheck
		}
	})

	b.Run("WithDefault", func(b *testing.B) {
		condition := func(_ context.Context, n int) string {
			if n > 100 {
				return "large"
			}
			return "default"
		}
		routes := map[string]pipz.Chainable[int]{
			"large": pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n / 2, nil
			}),
			"default": pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n, nil
			}),
		}
		sw := pipz.Switch(condition, routes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = sw.Process(ctx, i) //nolint:errcheck
		}
	})
}

// BenchmarkFallback measures the overhead of Fallback connector.
func BenchmarkFallback(b *testing.B) {
	ctx := context.Background()

	b.Run("PrimarySuccess", func(b *testing.B) {
		primary := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		fallback := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 100, nil
		})
		fb := pipz.Fallback(primary, fallback)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fb.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("PrimaryFailure", func(b *testing.B) {
		primary := pipz.ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 100, nil
		})
		fb := pipz.Fallback(primary, fallback)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fb.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("ConditionalFailure", func(b *testing.B) {
		// Primary fails 50% of the time
		primary := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			if n%2 == 0 {
				return 0, errors.New("even number")
			}
			return n * 2, nil
		})
		fallback := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 100, nil
		})
		fb := pipz.Fallback(primary, fallback)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fb.Process(ctx, i) //nolint:errcheck
		}
	})
}

// BenchmarkRetry measures the overhead of Retry connector.
func BenchmarkRetry(b *testing.B) {
	ctx := context.Background()

	b.Run("SuccessFirstAttempt", func(b *testing.B) {
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		retry := pipz.Retry(proc, 3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("SuccessSecondAttempt", func(b *testing.B) {
		attempt := 0
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempt++
			if attempt%2 == 1 {
				return 0, errors.New("temporary failure")
			}
			return n * 2, nil
		})
		retry := pipz.Retry(proc, 3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("AllAttemptsFail", func(b *testing.B) {
		proc := pipz.ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("permanent failure")
		})
		retry := pipz.Retry(proc, 3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})
}

// BenchmarkRetryWithBackoff measures the overhead of RetryWithBackoff connector.
func BenchmarkRetryWithBackoff(b *testing.B) {
	ctx := context.Background()

	b.Run("SuccessFirstAttempt", func(b *testing.B) {
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		retry := pipz.RetryWithBackoff(proc, 3, time.Microsecond)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("SuccessSecondAttempt", func(b *testing.B) {
		attempt := 0
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempt++
			if attempt%2 == 1 {
				return 0, errors.New("temporary failure")
			}
			return n * 2, nil
		})
		retry := pipz.RetryWithBackoff(proc, 3, time.Microsecond)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("MinimalBackoff", func(b *testing.B) {
		// Test with minimal backoff to measure overhead
		proc := pipz.ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("always fails")
		})
		retry := pipz.RetryWithBackoff(proc, 2, time.Nanosecond)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})
}

// BenchmarkTimeout measures the overhead of Timeout connector.
func BenchmarkTimeout(b *testing.B) {
	ctx := context.Background()

	b.Run("FastProcessor", func(b *testing.B) {
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		timeout := pipz.Timeout(proc, time.Second)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = timeout.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("WithSleep", func(b *testing.B) {
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(time.Microsecond)
			return n * 2, nil
		})
		timeout := pipz.Timeout(proc, time.Millisecond)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = timeout.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("TimeoutOverhead", func(b *testing.B) {
		// Measure the overhead of timeout mechanism
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			// Instant return to measure pure overhead
			return n, nil
		})
		timeout := pipz.Timeout(proc, time.Hour) // Large timeout
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = timeout.Process(ctx, 42) //nolint:errcheck
		}
	})
}

// BenchmarkConnectorComposition measures composed connectors.
func BenchmarkConnectorComposition(b *testing.B) {
	ctx := context.Background()

	b.Run("SequentialWithRetry", func(b *testing.B) {
		proc1 := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 1, nil
		})
		proc2 := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		seq := pipz.Sequential(proc1, proc2)
		retry := pipz.Retry(seq, 3)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = retry.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("TimeoutWithFallback", func(b *testing.B) {
		primary := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(time.Microsecond)
			return n * 2, nil
		})
		fallback := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 100, nil
		})
		primaryWithTimeout := pipz.Timeout(primary, time.Millisecond)
		fb := pipz.Fallback(primaryWithTimeout, fallback)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = fb.Process(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("SwitchWithSequential", func(b *testing.B) {
		// Create sequential processors for each route
		evenSeq := pipz.Sequential(
			pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n * 2, nil
			}),
			pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n + 10, nil
			}),
		)
		oddSeq := pipz.Sequential(
			pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n + 1, nil
			}),
			pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n * 3, nil
			}),
		)
		condition := func(_ context.Context, n int) string {
			if n%2 == 0 {
				return "even"
			}
			return "odd"
		}
		routes := map[string]pipz.Chainable[int]{
			"even": evenSeq,
			"odd":  oddSeq,
		}
		sw := pipz.Switch(condition, routes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = sw.Process(ctx, i) //nolint:errcheck
		}
	})

	b.Run("DeepNesting", func(b *testing.B) {
		// Create a deeply nested composition
		proc := pipz.ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 1, nil
		})
		chain := pipz.Chainable[int](proc)

		// Wrap in multiple layers
		chain = pipz.Timeout(chain, time.Second)
		chain = pipz.Retry(chain, 2)
		chain = pipz.Fallback(chain, proc)
		chain = pipz.Sequential(chain, proc)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = chain.Process(ctx, 42) //nolint:errcheck
		}
	})
}

// BenchmarkConnectorScaling measures how connectors scale with input size.
func BenchmarkConnectorScaling(b *testing.B) {
	ctx := context.Background()

	sizes := []int{1, 10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Sequential_Size_%d", size), func(b *testing.B) {
			procs := make([]pipz.Chainable[[]int], 3)
			for i := 0; i < 3; i++ {
				procs[i] = pipz.ProcessorFunc[[]int](func(_ context.Context, data []int) ([]int, error) {
					result := make([]int, len(data))
					for j, v := range data {
						result[j] = v + 1
					}
					return result, nil
				})
			}
			seq := pipz.Sequential(procs...)

			data := make([]int, size)
			for i := range data {
				data[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = seq.Process(ctx, data) //nolint:errcheck
			}
		})
	}
}

// benchData is a simple struct for concurrent benchmarks.
type benchData struct {
	ID     int
	Values []int
	Name   string
}

func (d benchData) Clone() benchData {
	newValues := make([]int, len(d.Values))
	copy(newValues, d.Values)
	return benchData{
		ID:     d.ID,
		Values: newValues,
		Name:   d.Name,
	}
}

// benchLargeData is a larger struct to measure copy overhead.
type benchLargeData struct {
	ID      int
	Matrix  [][]int
	Strings []string
	Map     map[string]int
}

func (d benchLargeData) Clone() benchLargeData {
	newMatrix := make([][]int, len(d.Matrix))
	for i := range d.Matrix {
		newMatrix[i] = make([]int, len(d.Matrix[i]))
		copy(newMatrix[i], d.Matrix[i])
	}
	newStrings := make([]string, len(d.Strings))
	copy(newStrings, d.Strings)
	newMap := make(map[string]int, len(d.Map))
	for k, v := range d.Map {
		newMap[k] = v
	}
	return benchLargeData{
		ID:      d.ID,
		Matrix:  newMatrix,
		Strings: newStrings,
		Map:     newMap,
	}
}

// BenchmarkConcurrent measures the performance of Concurrent connector.
func BenchmarkConcurrent(b *testing.B) {
	ctx := context.Background()

	b.Run("Empty", func(b *testing.B) {
		concurrent := pipz.Concurrent[benchData]()
		data := benchData{ID: 1, Values: []int{1, 2, 3}, Name: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = concurrent.Process(ctx, data) //nolint:errcheck
		}
	})

	b.Run("ThreeEffects_Sequential", func(b *testing.B) {
		// Simulate I/O-bound operations
		effect1 := pipz.Effect("io1", func(_ context.Context, _ benchData) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})
		effect2 := pipz.Effect("io2", func(_ context.Context, _ benchData) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})
		effect3 := pipz.Effect("io3", func(_ context.Context, _ benchData) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})

		seq := pipz.Sequential(effect1, effect2, effect3)
		data := benchData{ID: 1, Values: []int{1, 2, 3}, Name: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = seq.Process(ctx, data) //nolint:errcheck
		}
	})

	b.Run("ThreeEffects_Concurrent", func(b *testing.B) {
		// Same I/O-bound operations but concurrent
		effect1 := pipz.Effect("io1", func(_ context.Context, _ benchData) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})
		effect2 := pipz.Effect("io2", func(_ context.Context, _ benchData) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})
		effect3 := pipz.Effect("io3", func(_ context.Context, _ benchData) error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})

		concurrent := pipz.Concurrent(effect1, effect2, effect3)
		data := benchData{ID: 1, Values: []int{1, 2, 3}, Name: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = concurrent.Process(ctx, data) //nolint:errcheck
		}
	})

	b.Run("DeepCopyOverhead_SmallStruct", func(b *testing.B) {
		// Measure just the copy overhead with fast processors
		proc := pipz.Effect("fast", func(_ context.Context, _ benchData) error {
			return nil
		})

		concurrent := pipz.Concurrent(proc, proc, proc)
		data := benchData{ID: 1, Values: []int{1, 2, 3}, Name: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = concurrent.Process(ctx, data) //nolint:errcheck
		}
	})

	b.Run("DeepCopyOverhead_LargeStruct", func(b *testing.B) {
		// Larger struct to measure copy overhead
		proc := pipz.Effect("fast", func(_ context.Context, _ benchLargeData) error {
			return nil
		})

		concurrent := pipz.Concurrent(proc, proc, proc)

		// Create large data
		matrix := make([][]int, 100)
		for i := range matrix {
			matrix[i] = make([]int, 100)
		}
		strings := make([]string, 1000)
		for i := range strings {
			strings[i] = fmt.Sprintf("string_%d", i)
		}
		mapping := make(map[string]int, 1000)
		for i := 0; i < 1000; i++ {
			mapping[fmt.Sprintf("key_%d", i)] = i
		}

		data := benchLargeData{
			ID:      1,
			Matrix:  matrix,
			Strings: strings,
			Map:     mapping,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = concurrent.Process(ctx, data) //nolint:errcheck
		}
	})

	b.Run("WithErrorHandler", func(b *testing.B) {
		// Measure error handling overhead
		failingProc := pipz.Effect("fail", func(_ context.Context, _ benchData) error {
			return errors.New("expected error")
		})

		errorHandler := pipz.Effect("handle", func(_ context.Context, _ error) error {
			return nil
		})

		wrapped := pipz.WithErrorHandler(failingProc, errorHandler)
		concurrent := pipz.Concurrent(wrapped, wrapped, wrapped)

		data := benchData{ID: 1, Values: []int{1, 2, 3}, Name: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = concurrent.Process(ctx, data) //nolint:errcheck
		}
	})
}
