package pipz

import (
	"context"
	"testing"
)

func BenchmarkFilter_ConditionFalse(b *testing.B) {
	filter := NewFilter("bench-filter",
		func(_ context.Context, data int) bool { return data < 0 },
		Transform("double", func(_ context.Context, data int) int { return data * 2 }))

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = filter.Process(ctx, 5) //nolint:errcheck // benchmark ignores errors
	}
}

func BenchmarkFilter_ConditionTrue(b *testing.B) {
	filter := NewFilter("bench-filter",
		func(_ context.Context, data int) bool { return data > 0 },
		Transform("double", func(_ context.Context, data int) int { return data * 2 }))

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = filter.Process(ctx, 5) //nolint:errcheck // benchmark ignores errors
	}
}

func BenchmarkFilter_ComplexCondition(b *testing.B) {
	filter := NewFilter("complex-filter",
		func(_ context.Context, data int) bool {
			return data%2 == 0 && data > 10 && data < 1000
		},
		Transform("complex", func(_ context.Context, data int) int {
			return data*3 + 7
		}))

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = filter.Process(ctx, 20) //nolint:errcheck // benchmark ignores errors
	}
}
