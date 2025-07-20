package pipz_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zoobzio/pipz"
)

// BenchmarkPipeline_Baseline measures the overhead of an empty pipeline.
func BenchmarkPipeline_Baseline(b *testing.B) {
	ctx := context.Background()
	pipeline := pipz.NewPipeline[int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pipeline.Process(ctx, 42) //nolint:errcheck // benchmark ignores errors
	}
}

// BenchmarkPipeline_SingleProcessor measures a pipeline with one processor.
func BenchmarkPipeline_SingleProcessor(b *testing.B) {
	ctx := context.Background()
	pipeline := pipz.NewPipeline[int]()
	pipeline.Register(
		pipz.Apply("double", func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		}),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pipeline.Process(ctx, 42) //nolint:errcheck // benchmark ignores errors
	}
}

// BenchmarkPipeline_ProcessorTypes measures different processor types.
func BenchmarkPipeline_ProcessorTypes(b *testing.B) {
	ctx := context.Background()

	b.Run("Transform", func(b *testing.B) {
		pipeline := pipz.NewPipeline[int]()
		pipeline.Register(
			pipz.Apply("increment", func(_ context.Context, n int) (int, error) {
				return n + 1, nil
			}),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, 42) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("Effect", func(b *testing.B) {
		pipeline := pipz.NewPipeline[int]()
		pipeline.Register(
			pipz.Effect("positive", func(_ context.Context, n int) error {
				if n <= 0 {
					return fmt.Errorf("not positive")
				}
				return nil
			}),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, 42) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("Effect", func(b *testing.B) {
		var sum int64
		pipeline := pipz.NewPipeline[int]()
		pipeline.Register(
			pipz.Effect("sum", func(_ context.Context, n int) error {
				sum += int64(n)
				return nil
			}),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, 42) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("Mutate", func(b *testing.B) {
		pipeline := pipz.NewPipeline[int]()
		pipeline.Register(
			pipz.Mutate("double_if_even",
				func(_ context.Context, n int) int {
					return n * 2
				},
				func(_ context.Context, n int) bool {
					return n%2 == 0
				},
			),
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, 42) //nolint:errcheck // benchmark ignores errors
		}
	})
}

// BenchmarkPipeline_Multiple measures pipelines with varying processor counts.
func BenchmarkPipeline_Multiple(b *testing.B) {
	ctx := context.Background()

	processorCounts := []int{1, 5, 10, 20, 50}

	for _, count := range processorCounts {
		b.Run(fmt.Sprintf("Processors_%d", count), func(b *testing.B) {
			pipeline := pipz.NewPipeline[int]()

			// Add processors
			for i := 0; i < count; i++ {
				name := fmt.Sprintf("processor_%d", i)
				pipeline.Register(
					pipz.Apply(name, func(_ context.Context, n int) (int, error) {
						return n + 1, nil
					}),
				)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, 0) //nolint:errcheck // benchmark ignores errors
			}
		})
	}
}
