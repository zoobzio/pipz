package benchmarks

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// BenchmarkFilter benchmarks filter operations.
func BenchmarkFilter(b *testing.B) {
	ctx := context.Background()

	b.Run("ConditionFalse", func(b *testing.B) {
		filter := pipz.NewFilter(pipz.NewIdentity("bench-filter", ""),
			func(_ context.Context, data ClonableInt) bool { return data < 0 },
			pipz.Transform(pipz.NewIdentity("double", ""), func(_ context.Context, data ClonableInt) ClonableInt { return data * 2 }))

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = filter.Process(ctx, 5) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("ConditionTrue", func(b *testing.B) {
		filter := pipz.NewFilter(pipz.NewIdentity("bench-filter", ""),
			func(_ context.Context, data ClonableInt) bool { return data > 0 },
			pipz.Transform(pipz.NewIdentity("double", ""), func(_ context.Context, data ClonableInt) ClonableInt { return data * 2 }))

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, _ = filter.Process(ctx, 5) //nolint:errcheck // benchmark ignores errors
		}
	})

	b.Run("ComplexCondition", func(b *testing.B) {
		filter := pipz.NewFilter(pipz.NewIdentity("complex-filter", ""),
			func(_ context.Context, data ClonableInt) bool {
				return data%2 == 0 && data > 10 && data < 1000
			},
			pipz.Transform(pipz.NewIdentity("complex", ""), func(_ context.Context, data ClonableInt) ClonableInt {
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
		condition := func(_ context.Context, _ User) bool {
			return true // Always return true for benchmarking
		}

		p1 := pipz.Transform(pipz.NewIdentity("odd", ""), func(_ context.Context, u User) User {
			u.Age = 101
			return u
		})
		p2 := pipz.Transform(pipz.NewIdentity("even-slow", ""), func(_ context.Context, u User) User {
			time.Sleep(time.Microsecond)
			u.Age = 100
			return u
		})
		p3 := pipz.Transform(pipz.NewIdentity("even-fast", ""), func(_ context.Context, u User) User {
			u.Age = 50
			return u
		})

		contest := pipz.NewContest(pipz.NewIdentity("bench-contest", ""), condition, p1, p2, p3)
		data := User{ID: 1, Name: "Test", Email: "test@example.com", Age: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := contest.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NoWinner", func(b *testing.B) {
		condition := func(_ context.Context, _ User) bool {
			return false // Always return false for benchmarking
		}

		p1 := pipz.Transform(pipz.NewIdentity("p1", ""), func(_ context.Context, u User) User {
			u.Age = 10
			return u
		})
		p2 := pipz.Transform(pipz.NewIdentity("p2", ""), func(_ context.Context, u User) User {
			u.Age = 20
			return u
		})
		p3 := pipz.Transform(pipz.NewIdentity("p3", ""), func(_ context.Context, u User) User {
			u.Age = 30
			return u
		})

		contest := pipz.NewContest(pipz.NewIdentity("bench-no-winner", ""), condition, p1, p2, p3)
		data := User{ID: 1, Name: "Test", Email: "test@example.com", Age: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = contest.Process(ctx, data) //nolint:errcheck // benchmarking no-winner scenario performance
		}
	})

	b.Run("ComplexCondition", func(b *testing.B) {
		condition := func(_ context.Context, _ User) bool {
			return true // Simplified for benchmarking
		}

		processors := make([]pipz.Chainable[User], 10)
		for i := 0; i < 10; i++ {
			val := i * 10
			processors[i] = pipz.Transform(pipz.NewIdentity("p", ""), func(_ context.Context, u User) User {
				u.Age = val
				return u
			})
		}

		contest := pipz.NewContest(pipz.NewIdentity("bench-complex", ""), condition, processors...)
		data := User{ID: 1, Name: "Test", Email: "test@example.com", Age: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := contest.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("VsRace", func(b *testing.B) {
		b.Run("Contest", func(b *testing.B) {
			condition := func(_ context.Context, _ User) bool {
				return true
			}

			p1 := pipz.Transform(pipz.NewIdentity("p1", ""), func(_ context.Context, u User) User {
				u.Age = 100
				return u
			})
			p2 := pipz.Transform(pipz.NewIdentity("p2", ""), func(_ context.Context, u User) User {
				time.Sleep(time.Microsecond)
				u.Age = 200
				return u
			})

			contest := pipz.NewContest(pipz.NewIdentity("contest", ""), condition, p1, p2)
			data := User{ID: 1, Name: "Test", Email: "test@example.com", Age: 1}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := contest.Process(ctx, data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("Race", func(b *testing.B) {
			p1 := pipz.Transform(pipz.NewIdentity("p1", ""), func(_ context.Context, u User) User {
				u.Age = 100
				return u
			})
			p2 := pipz.Transform(pipz.NewIdentity("p2", ""), func(_ context.Context, u User) User {
				time.Sleep(time.Microsecond)
				u.Age = 200
				return u
			})

			race := pipz.NewRace(pipz.NewIdentity("race", ""), p1, p2)
			data := User{ID: 1, Name: "Test", Email: "test@example.com", Age: 1}

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
