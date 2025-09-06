package benchmarks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	pipztesting "github.com/zoobzio/pipz/testing"
)

// BenchmarkProcessors measures the performance of individual processor types.
func BenchmarkProcessors(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	b.Run("Transform", func(b *testing.B) {
		processor := pipz.Transform("transform", func(_ context.Context, n ClonableInt) ClonableInt {
			return n * 2
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result // Prevent optimization
		}
	})

	b.Run("Apply_Success", func(b *testing.B) {
		processor := pipz.Apply("apply", func(_ context.Context, n ClonableInt) (ClonableInt, error) {
			return n * 2, nil
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Apply_Error", func(b *testing.B) {
		processor := pipz.Apply("apply-error", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			return 0, errors.New("test error")
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			_ = result
			_ = err // Expected error, don't fail benchmark
		}
	})

	b.Run("Effect", func(b *testing.B) {
		processor := pipz.Effect("effect", func(_ context.Context, _ ClonableInt) error {
			return nil
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Enrich", func(b *testing.B) {
		processor := pipz.Enrich("enrich", func(_ context.Context, n ClonableInt) (ClonableInt, error) {
			return n + 100, nil
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Mutate", func(b *testing.B) {
		type MutableData struct {
			Value int
		}

		processor := pipz.Mutate("mutate",
			func(_ context.Context, d MutableData) MutableData {
				d.Value *= 2
				return d
			},
			func(_ context.Context, d MutableData) bool { return d.Value > 0 },
		)
		mutableData := MutableData{Value: 42}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, mutableData)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkConnectors measures the performance of connector types.
func BenchmarkConnectors(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	// Create simple processors for composition
	double := pipz.Transform("double", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
	add10 := pipz.Transform("add10", func(_ context.Context, n ClonableInt) ClonableInt { return n + 10 })

	b.Run("Sequence_Short", func(b *testing.B) {
		seq := pipz.NewSequence("short", double, add10)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Sequence_Long", func(b *testing.B) {
		processors := make([]pipz.Chainable[ClonableInt], 10)
		for i := 0; i < 10; i++ {
			processors[i] = pipz.Transform("step", func(_ context.Context, n ClonableInt) ClonableInt { return n + 1 })
		}
		seq := pipz.NewSequence("long", processors...)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Concurrent_Two", func(b *testing.B) {
		concurrent := pipz.NewConcurrent("concurrent", double, add10)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := concurrent.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Race_Two", func(b *testing.B) {
		race := pipz.NewRace("race", double, add10)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := race.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Contest_First_Wins", func(b *testing.B) {
		contest := pipz.NewContest("contest",
			func(_ context.Context, n ClonableInt) bool { return n > 0 },
			double, add10,
		)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := contest.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Fallback_Primary_Success", func(b *testing.B) {
		fallback := pipz.NewFallback("fallback", double, add10)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := fallback.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Fallback_Primary_Fails", func(b *testing.B) {
		failing := pipz.Apply("fail", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			return 0, errors.New("always fails")
		})
		fallback := pipz.NewFallback("fallback", failing, double)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := fallback.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkStatefulConnectors measures performance of stateful connectors.
func BenchmarkStatefulConnectors(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	processor := pipz.Transform("processor", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })

	b.Run("CircuitBreaker_Closed", func(b *testing.B) {
		cb := pipz.NewCircuitBreaker("cb", processor, 5, time.Minute)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := cb.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("CircuitBreaker_Open", func(b *testing.B) {
		// Create a circuit breaker and force it open
		failingProcessor := pipz.Apply("fail", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			return 0, errors.New("forced failure")
		})
		cb := pipz.NewCircuitBreaker("cb", failingProcessor, 1, time.Minute)

		// Open the circuit
		_, _ = cb.Process(ctx, data) //nolint:errcheck // This will fail and open circuit - intentionally ignoring error

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := cb.Process(ctx, data)
			_ = result
			_ = err // Circuit is open, expect error
		}
	})

	b.Run("RateLimiter_Within_Limit", func(b *testing.B) {
		// Create rate-limited sequence
		limitedPipeline := pipz.NewSequence("rate-limited",
			pipz.NewRateLimiter[ClonableInt]("limiter", 1000000, 100), // Very high limit
			processor,
		)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := limitedPipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Timeout_Fast_Operation", func(b *testing.B) {
		timeout := pipz.NewTimeout("timeout", processor, time.Second)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := timeout.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Retry_Success_First_Try", func(b *testing.B) {
		retry := pipz.NewRetry("retry", processor, 3)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := retry.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Backoff_Success_First_Try", func(b *testing.B) {
		backoff := pipz.NewBackoff("backoff", processor, 3, time.Microsecond)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := backoff.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkErrorHandling measures error processing performance.
func BenchmarkErrorHandling(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	b.Run("Handle_No_Error", func(b *testing.B) {
		processor := pipz.Transform("success", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
		handler := pipz.NewHandle("handler", processor,
			pipz.Effect("log", func(_ context.Context, _ *pipz.Error[ClonableInt]) error {
				return nil
			}),
		)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := handler.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Handle_With_Error", func(b *testing.B) {
		processor := pipz.Apply("error", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			return 0, errors.New("test error")
		})
		handler := pipz.NewHandle("handler", processor,
			pipz.Effect("log", func(_ context.Context, _ *pipz.Error[ClonableInt]) error {
				return nil
			}),
		)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := handler.Process(ctx, data)
			_ = result
			_ = err // Handle processes errors but passes them through
		}
	})

	b.Run("Error_Creation", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := &pipz.Error[ClonableInt]{
				Timestamp: time.Now(),
				InputData: data,
				Err:       errors.New("test error"),
				Path:      []pipz.Name{"test-processor"},
			}
			_ = err
		}
	})

	b.Run("Error_Wrapping", func(b *testing.B) {
		originalErr := errors.New("original error")
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := &pipz.Error[ClonableInt]{
				Timestamp: time.Now(),
				InputData: data,
				Err:       originalErr,
				Path:      []pipz.Name{"wrapper"},
			}
			_ = err
		}
	})
}

// BenchmarkMemoryUsage specifically measures memory allocation patterns.
func BenchmarkMemoryUsage(b *testing.B) {
	ctx := context.Background()

	b.Run("Zero_Alloc_Transform", func(b *testing.B) {
		processor := pipz.Transform("transform", func(_ context.Context, n ClonableInt) ClonableInt {
			return n * 2
		})
		data := ClonableInt(42)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("String_Processing", func(b *testing.B) {
		processor := pipz.Transform("string", func(_ context.Context, s string) string {
			return s + "_processed"
		})
		data := "test"

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Struct_Cloning", func(b *testing.B) {
		type TestData struct {
			ID    int
			Name  string
			Items []int
		}

		// Implement Clone method
		cloneFunc := func(d TestData) TestData {
			items := make([]int, len(d.Items))
			copy(items, d.Items)
			return TestData{
				ID:    d.ID,
				Name:  d.Name,
				Items: items,
			}
		}

		processor := pipz.Transform("clone", func(_ context.Context, d TestData) TestData {
			return cloneFunc(d)
		})

		data := TestData{
			ID:    1,
			Name:  "test",
			Items: []int{1, 2, 3, 4, 5},
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Large_Pipeline_Memory", func(b *testing.B) {
		// Create a large pipeline to measure memory overhead
		processors := make([]pipz.Chainable[ClonableInt], 50)
		for i := 0; i < 50; i++ {
			processors[i] = pipz.Transform("step", func(_ context.Context, n ClonableInt) ClonableInt { return n + 1 })
		}
		seq := pipz.NewSequence("large", processors...)
		data := ClonableInt(42)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkConcurrentAccess measures performance under concurrent load.
func BenchmarkConcurrentAccess(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	b.Run("Single_Processor_Concurrent", func(b *testing.B) {
		processor := pipz.Transform("concurrent", func(_ context.Context, n ClonableInt) ClonableInt {
			return n * 2
		})

		b.ResetTimer()
		b.ReportAllocs()
		b.SetParallelism(4) // Use 4 goroutines

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result, err := processor.Process(ctx, data)
				if err != nil {
					b.Error(err)
					return
				}
				_ = result
			}
		})
	})

	b.Run("Circuit_Breaker_Concurrent", func(b *testing.B) {
		processor := pipz.Transform("base", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
		cb := pipz.NewCircuitBreaker("cb", processor, 1000, time.Minute)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetParallelism(8)

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result, err := cb.Process(ctx, data)
				if err != nil {
					b.Error(err)
					return
				}
				_ = result
			}
		})
	})

	b.Run("Rate_Limiter_Concurrent", func(b *testing.B) {
		processor := pipz.Transform("base", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
		limitedPipeline := pipz.NewSequence("rate-limited",
			pipz.NewRateLimiter[ClonableInt]("limiter", 1000000, 100),
			processor,
		)

		b.ResetTimer()
		b.ReportAllocs()
		b.SetParallelism(8)

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result, err := limitedPipeline.Process(ctx, data)
				if err != nil {
					b.Error(err)
					return
				}
				_ = result
			}
		})
	})
}

// BenchmarkTestingHelpers measures the performance impact of test helpers.
func BenchmarkTestingHelpers(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	b.Run("MockProcessor_Success", func(b *testing.B) {
		// Note: Using nil for testing.T in benchmark (normally not recommended)
		mock := pipztesting.NewMockProcessor[ClonableInt](nil, "mock")
		mock.WithReturn(ClonableInt(84), nil)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := mock.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("MockProcessor_With_History", func(b *testing.B) {
		mock := pipztesting.NewMockProcessor[ClonableInt](nil, "mock-history")
		mock.WithReturn(ClonableInt(84), nil).WithHistorySize(100)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := mock.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("ChaosProcessor_No_Chaos", func(b *testing.B) {
		baseProcessor := pipz.Transform("base", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
		chaos := pipztesting.NewChaosProcessor("chaos", baseProcessor, pipztesting.ChaosConfig{
			FailureRate: 0.0, // No failures for benchmark
			LatencyMin:  0,
			LatencyMax:  0,
			TimeoutRate: 0.0,
			PanicRate:   0.0,
			Seed:        12345,
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := chaos.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("ChaosProcessor_With_Chaos", func(b *testing.B) {
		baseProcessor := pipz.Transform("base", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
		chaos := pipztesting.NewChaosProcessor("chaos", baseProcessor, pipztesting.ChaosConfig{
			FailureRate: 0.1, // 10% failure rate
			LatencyMin:  0,   // No latency for benchmark speed
			LatencyMax:  0,
			TimeoutRate: 0.0,
			PanicRate:   0.0,
			Seed:        12345,
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := chaos.Process(ctx, data)
			_ = result
			_ = err // May fail due to chaos, that's expected
		}
	})
}

// BenchmarkPanicRecovery measures the performance impact of panic recovery mechanisms.
// This critical benchmark ensures panic safety doesn't compromise pipz performance goals.
func BenchmarkPanicRecovery(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	b.Run("Normal_Path_Defer_Overhead", func(b *testing.B) {
		// Measure the defer overhead in normal execution (no panic)
		processor := pipz.Transform("normal", func(_ context.Context, n ClonableInt) ClonableInt {
			return n * 2
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Panic_Recovery_Simple_Panic", func(b *testing.B) {
		// Measure panic recovery with simple panic message
		processor := pipz.Apply("panic-simple", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			panic("simple test panic")
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			_ = result
			_ = err // Expect panic error, don't fail benchmark
		}
	})

	b.Run("Panic_Recovery_Complex_Message", func(b *testing.B) {
		// Measure sanitization overhead with complex panic content
		complexMessage := "panic with addresses 0x123456789abcdef and paths /sensitive/path/data"
		processor := pipz.Apply("panic-complex", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			panic(complexMessage)
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			_ = result
			_ = err
		}
	})

	b.Run("Panic_Recovery_Nil_Panic", func(b *testing.B) {
		// Measure recovery from nil panic (edge case)
		processor := pipz.Apply("panic-nil", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			panic(nil)
		})
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, data)
			_ = result
			_ = err
		}
	})

	b.Run("Panic_vs_Regular_Error", func(b *testing.B) {
		// Compare panic recovery cost vs normal error handling
		errorProcessor := pipz.Apply("regular-error", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			return 0, errors.New("regular error")
		})
		panicProcessor := pipz.Apply("panic-error", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
			panic("equivalent panic")
		})

		b.Run("RegularError", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := errorProcessor.Process(ctx, data)
				_ = result
				_ = err
			}
		})

		b.Run("PanicError", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := panicProcessor.Process(ctx, data)
				_ = result
				_ = err
			}
		})
	})

	b.Run("Pipeline_Defer_Overhead", func(b *testing.B) {
		// Measure cumulative defer overhead in long pipelines
		processors := make([]pipz.Chainable[ClonableInt], 20)
		for i := 0; i < 20; i++ {
			processors[i] = pipz.Transform("step", func(_ context.Context, n ClonableInt) ClonableInt {
				return n + 1
			})
		}
		pipeline := pipz.NewSequence("long-pipeline", processors...)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Concurrent_Panic_Recovery", func(b *testing.B) {
		// Test panic recovery under concurrent access
		processor := pipz.Transform("concurrent-safe", func(_ context.Context, n ClonableInt) ClonableInt {
			// Occasionally panic to test recovery under load
			if n%1000 == 0 {
				panic("concurrent panic test")
			}
			return n * 2
		})

		b.ResetTimer()
		b.ReportAllocs()
		b.SetParallelism(4)

		b.RunParallel(func(pb *testing.PB) {
			testData := ClonableInt(42)
			for pb.Next() {
				result, err := processor.Process(ctx, testData)
				_ = result
				_ = err    // May panic occasionally, that's expected
				testData++ // Vary input to trigger occasional panics
			}
		})
	})

	b.Run("Memory_Pressure_With_Panics", func(b *testing.B) {
		// Test panic recovery performance under memory pressure
		// Allocate significant memory to stress test recovery allocation
		ballast := make([]byte, 1024*1024*50) // 50MB ballast
		defer func() { ballast = nil }()
		_ = ballast // Prevent unused variable warning

		processor := pipz.Apply("memory-pressure", func(_ context.Context, n ClonableInt) (ClonableInt, error) {
			if n%100 == 0 {
				panic("memory pressure panic test")
			}
			return n * 2, nil
		})

		b.ResetTimer()
		b.ReportAllocs()

		testData := ClonableInt(1)
		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, testData)
			_ = result
			_ = err
			testData++
		}
	})
}

// BenchmarkPanicRecovery_RealWorld measures panic recovery with realistic data and scenarios.
func BenchmarkPanicRecovery_RealWorld(b *testing.B) {
	ctx := context.Background()

	// Create realistic panic scenarios that might occur in production
	user := User{
		ID:    12345,
		Name:  "Test User",
		Email: "test@example.com",
		Age:   30,
	}

	b.Run("JSON_Parse_Panic", func(b *testing.B) {
		// Simulate JSON parsing panic with realistic data
		processor := pipz.Apply("json-parse", func(_ context.Context, u User) (User, error) {
			// Simulate a panic that might occur during JSON processing
			if u.Email == "test@example.com" {
				panic("json: cannot unmarshal number into Go struct field")
			}
			return u, nil
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, user)
			_ = result
			_ = err
		}
	})

	b.Run("Database_Connection_Panic", func(b *testing.B) {
		// Simulate database driver panic with realistic error
		processor := pipz.Apply("db-query", func(_ context.Context, _ User) (User, error) {
			panic("sql: database connection lost during query execution")
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, user)
			_ = result
			_ = err
		}
	})

	b.Run("Index_Out_Of_Bounds", func(b *testing.B) {
		// Simulate slice bounds panic - common in data processing
		processor := pipz.Apply("slice-access", func(_ context.Context, u User) (User, error) {
			slice := []int{1, 2, 3}
			_ = slice[10] // Panic: index out of range
			return u, nil
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, user)
			_ = result
			_ = err
		}
	})

	b.Run("Nil_Pointer_Dereference", func(b *testing.B) {
		// Simulate nil pointer panic - another common scenario
		processor := pipz.Apply("nil-deref", func(_ context.Context, u User) (User, error) {
			var ptr *string
			_ = *ptr // Panic: nil pointer dereference
			return u, nil
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := processor.Process(ctx, user)
			_ = result
			_ = err
		}
	})

	b.Run("Complex_Pipeline_With_Intermittent_Panics", func(b *testing.B) {
		// Realistic pipeline where some stages might panic
		pipeline := pipz.NewSequence("complex-pipeline",
			pipz.Transform("validate", func(_ context.Context, u User) User {
				if u.ID <= 0 {
					panic("invalid user ID")
				}
				return u
			}),
			pipz.Apply("enrich", func(_ context.Context, u User) (User, error) {
				// This succeeds normally
				u.Age++
				return u, nil
			}),
			pipz.Transform("format", func(_ context.Context, u User) User {
				if u.Email == "" {
					panic("empty email not allowed")
				}
				return u
			}),
		)

		// Use data that will trigger panic in first step
		panicUser := User{ID: 0, Name: "Invalid", Email: "test@example.com"}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, panicUser)
			_ = result
			_ = err // Expected to fail on validation
		}
	})
}

// BenchmarkPanicRecovery_Connectors measures panic recovery performance across different connector types.
func BenchmarkPanicRecovery_Connectors(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	// Create processors that may panic
	panicProcessor := pipz.Transform("panic", func(_ context.Context, n ClonableInt) ClonableInt {
		if n == 42 {
			panic("test panic in connector")
		}
		return n * 2
	})

	safeProcessor := pipz.Transform("safe", func(_ context.Context, n ClonableInt) ClonableInt {
		return n + 10
	})

	b.Run("Sequence_With_Panic", func(b *testing.B) {
		seq := pipz.NewSequence("seq", panicProcessor, safeProcessor)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := seq.Process(ctx, data)
			_ = result
			_ = err
		}
	})

	b.Run("Fallback_Primary_Panics", func(b *testing.B) {
		fallback := pipz.NewFallback("fallback", panicProcessor, safeProcessor)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := fallback.Process(ctx, data)
			if err != nil {
				b.Fatal("fallback should have succeeded")
			}
			_ = result
		}
	})

	b.Run("Retry_With_Panic", func(b *testing.B) {
		retry := pipz.NewRetry("retry", panicProcessor, 3)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := retry.Process(ctx, data)
			_ = result
			_ = err // Will fail after 3 attempts
		}
	})

	b.Run("CircuitBreaker_With_Panic", func(b *testing.B) {
		cb := pipz.NewCircuitBreaker("cb", panicProcessor, 5, time.Minute)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := cb.Process(ctx, data)
			_ = result
			_ = err // Will eventually open circuit
		}
	})

	b.Run("Handle_Panic_Processing", func(b *testing.B) {
		// Test panic recovery in error handling
		handler := pipz.NewHandle("handler", panicProcessor,
			pipz.Effect("log-panic", func(_ context.Context, e *pipz.Error[ClonableInt]) error {
				// Process the panic error
				_ = e.Error()
				return nil
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := handler.Process(ctx, data)
			_ = result
			_ = err
		}
	})
}
