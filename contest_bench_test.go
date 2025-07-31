package pipz

import (
	"context"
	"testing"
	"time"
)

func BenchmarkContest(b *testing.B) {
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
	ctx := context.Background()
	data := TestData{Value: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := contest.Process(ctx, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContestNoWinner(b *testing.B) {
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
	ctx := context.Background()
	data := TestData{Value: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = contest.Process(ctx, data) //nolint:errcheck // benchmarking no-winner scenario performance
	}
}

func BenchmarkContestComplexCondition(b *testing.B) {
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
	ctx := context.Background()
	data := TestData{Value: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := contest.Process(ctx, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContestVsRace(b *testing.B) {
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
		ctx := context.Background()
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
		ctx := context.Background()
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := race.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
