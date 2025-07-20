package pipz_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/zoobzio/pipz"
)

func BenchmarkAdapt_Transform(b *testing.B) {
	ctx := context.Background()

	b.Run("SimpleTransform", func(b *testing.B) {
		processor := pipz.Transform("double", func(_ context.Context, value int) int {
			return value * 2
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("StringTransform", func(b *testing.B) {
		processor := pipz.Transform("upper", func(_ context.Context, value string) string {
			return fmt.Sprintf("UPPER_%s", value)
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, "test") //nolint:errcheck
		}
	})
}

func BenchmarkAdapt_Apply(b *testing.B) {
	ctx := context.Background()

	b.Run("SuccessfulApply", func(b *testing.B) {
		processor := pipz.Apply("increment", func(_ context.Context, value int) (int, error) {
			return value + 1, nil
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("ErrorApply", func(b *testing.B) {
		processor := pipz.Apply("fail", func(_ context.Context, value int) (int, error) {
			if value < 0 {
				return 0, errors.New("negative value")
			}
			return value, nil
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, -1) //nolint:errcheck
		}
	})
}

func BenchmarkAdapt_Effect(b *testing.B) {
	ctx := context.Background()

	b.Run("SuccessfulEffect", func(b *testing.B) {
		processor := pipz.Effect("positive", func(_ context.Context, value int) error {
			if value > 0 {
				return nil
			}
			return errors.New("must be positive")
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("FailingEffect", func(b *testing.B) {
		processor := pipz.Effect("positive", func(_ context.Context, value int) error {
			if value > 0 {
				return nil
			}
			return errors.New("must be positive")
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, -1) //nolint:errcheck
		}
	})
}

func BenchmarkAdapt_Mutate(b *testing.B) {
	ctx := context.Background()

	b.Run("ConditionTrue", func(b *testing.B) {
		processor := pipz.Mutate("double_if_even",
			func(_ context.Context, value int) int {
				return value * 2
			},
			func(_ context.Context, value int) bool {
				return value%2 == 0
			},
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck // Even number
		}
	})

	b.Run("ConditionFalse", func(b *testing.B) {
		processor := pipz.Mutate("double_if_even",
			func(_ context.Context, value int) int {
				return value * 2
			},
			func(_ context.Context, value int) bool {
				return value%2 == 0
			},
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 43) //nolint:errcheck // Odd number
		}
	})
}

func BenchmarkAdapt_EffectLogging(b *testing.B) {
	ctx := context.Background()

	b.Run("SuccessfulEffect", func(b *testing.B) {
		processor := pipz.Effect("log", func(_ context.Context, value int) error {
			// Simulate logging
			_ = fmt.Sprintf("Processing: %d", value)
			return nil
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("FailingEffect", func(b *testing.B) {
		processor := pipz.Effect("fail", func(_ context.Context, _ int) error {
			return errors.New("effect failed")
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})
}

func BenchmarkAdapt_Enrich(b *testing.B) {
	ctx := context.Background()

	b.Run("SuccessfulEnrich", func(b *testing.B) {
		processor := pipz.Enrich("add_metadata", func(_ context.Context, value int) (int, error) {
			// Simulate enrichment
			return value + 1000, nil
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})

	b.Run("FailingEnrich", func(b *testing.B) {
		processor := pipz.Enrich("fail_enrich", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("enrichment failed")
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, 42) //nolint:errcheck
		}
	})
}

func BenchmarkAdapt_ProcessorTypes(b *testing.B) {
	ctx := context.Background()

	types := []struct {
		name string
		proc pipz.Processor[int]
	}{
		{
			name: "Transform",
			proc: pipz.Transform("double", func(_ context.Context, value int) int {
				return value * 2
			}),
		},
		{
			name: "Apply",
			proc: pipz.Apply("increment", func(_ context.Context, value int) (int, error) {
				return value + 1, nil
			}),
		},
		{
			name: "Effect",
			proc: pipz.Effect("positive", func(_ context.Context, value int) error {
				if value > 0 {
					return nil
				}
				return errors.New("must be positive")
			}),
		},
		{
			name: "Mutate",
			proc: pipz.Mutate("double_if_even",
				func(_ context.Context, value int) int {
					return value * 2
				},
				func(_ context.Context, value int) bool {
					return value%2 == 0
				},
			),
		},
		{
			name: "Effect",
			proc: pipz.Effect("log", func(_ context.Context, _ int) error {
				return nil
			}),
		},
		{
			name: "Enrich",
			proc: pipz.Enrich("add_metadata", func(_ context.Context, value int) (int, error) {
				return value + 1000, nil
			}),
		},
	}

	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = tt.proc.Fn(ctx, 42) //nolint:errcheck
			}
		})
	}
}

func BenchmarkAdapt_ComplexOperations(b *testing.B) {
	ctx := context.Background()

	b.Run("ComplexTransform", func(b *testing.B) {
		processor := pipz.Transform("complex", func(_ context.Context, value []int) []int {
			result := make([]int, len(value))
			for i, v := range value {
				result[i] = v*v + 1
			}
			return result
		})

		data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, data) //nolint:errcheck
		}
	})

	b.Run("ComplexEffect", func(b *testing.B) {
		processor := pipz.Effect("all_positive", func(_ context.Context, value []int) error {
			for _, v := range value {
				if v <= 0 {
					return errors.New("all values must be positive")
				}
			}
			return nil
		})

		data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = processor.Fn(ctx, data) //nolint:errcheck
		}
	})
}
