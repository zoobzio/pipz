package testing

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestMockProcessor(t *testing.T) {
	ctx := context.Background()

	t.Run("Returns Configured Value", func(t *testing.T) {
		mock := NewMockProcessor[string](t, "mock-test")
		mock.WithReturn("mocked", nil)

		result, err := mock.Process(ctx, "input")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "mocked" {
			t.Errorf("expected 'mocked', got %q", result)
		}
	})

	t.Run("Returns Configured Error", func(t *testing.T) {
		mock := NewMockProcessor[string](t, "mock-error")
		expectedErr := errors.New("test error")
		mock.WithReturn("", expectedErr)

		_, err := mock.Process(ctx, "input")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Tracks Call Count", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock-count")
		mock.WithReturn(42, nil)

		for i := 0; i < 5; i++ {
			_, _ = mock.Process(ctx, i)
		}

		if mock.CallCount() != 5 {
			t.Errorf("expected 5 calls, got %d", mock.CallCount())
		}
	})

	t.Run("Tracks Last Input", func(t *testing.T) {
		mock := NewMockProcessor[string](t, "mock-input")
		mock.WithReturn("out", nil)

		_, _ = mock.Process(ctx, "first")
		_, _ = mock.Process(ctx, "second")
		_, _ = mock.Process(ctx, "third")

		if mock.LastInput() != "third" {
			t.Errorf("expected last input 'third', got %q", mock.LastInput())
		}
	})

	t.Run("Applies Delay", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock-delay")
		mock.WithReturn(42, nil).WithDelay(50 * time.Millisecond)

		start := time.Now()
		_, _ = mock.Process(ctx, 1)
		elapsed := time.Since(start)

		if elapsed < 50*time.Millisecond {
			t.Errorf("expected delay of at least 50ms, got %v", elapsed)
		}
	})

	t.Run("Respects Context Cancellation During Delay", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock-cancel")
		mock.WithReturn(42, nil).WithDelay(1 * time.Second)

		ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		_, err := mock.Process(ctx, 1)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context deadline exceeded, got %v", err)
		}
	})

	t.Run("Panics When Configured", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock-panic")
		mock.WithPanic("test panic")

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			} else if r != "test panic" {
				t.Errorf("expected panic 'test panic', got %v", r)
			}
		}()

		_, _ = mock.Process(ctx, 1)
	})

	t.Run("Tracks Call History", func(t *testing.T) {
		mock := NewMockProcessor[string](t, "mock-history")
		mock.WithReturn("out", nil).WithHistorySize(3)

		_, _ = mock.Process(ctx, "a")
		_, _ = mock.Process(ctx, "b")
		_, _ = mock.Process(ctx, "c")
		_, _ = mock.Process(ctx, "d")

		history := mock.CallHistory()
		if len(history) != 3 {
			t.Errorf("expected 3 history entries, got %d", len(history))
		}
		if history[0].Input != "b" {
			t.Errorf("expected first history entry 'b', got %q", history[0].Input)
		}
	})

	t.Run("Reset Clears State", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock-reset")
		mock.WithReturn(42, nil)

		_, _ = mock.Process(ctx, 1)
		_, _ = mock.Process(ctx, 2)

		mock.Reset()

		if mock.CallCount() != 0 {
			t.Errorf("expected 0 calls after reset, got %d", mock.CallCount())
		}
		if len(mock.CallHistory()) != 0 {
			t.Errorf("expected empty history after reset, got %d entries", len(mock.CallHistory()))
		}
	})

	t.Run("Name Returns Configured Name", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "my-mock")
		if mock.Identity().Name() != "my-mock" {
			t.Errorf("expected name 'my-mock', got %q", mock.Identity().Name())
		}
	})

	t.Run("Close Returns Nil", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		if err := mock.Close(); err != nil {
			t.Errorf("expected nil from Close, got %v", err)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock-schema")

		schema := mock.Schema()

		if schema.Type != "mock" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "mock")
		}
		if schema.Identity.Name() != "mock-schema" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "mock-schema")
		}
	})
}

func TestMockProcessorAssertions(t *testing.T) {
	ctx := context.Background()

	t.Run("AssertProcessed", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		mock.WithReturn(0, nil)

		_, _ = mock.Process(ctx, 1)
		_, _ = mock.Process(ctx, 2)
		_, _ = mock.Process(ctx, 3)

		AssertProcessed(t, mock, 3)
	})

	t.Run("AssertNotProcessed", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		AssertNotProcessed(t, mock)
	})

	t.Run("AssertProcessedWith", func(t *testing.T) {
		mock := NewMockProcessor[string](t, "mock")
		mock.WithReturn("out", nil)

		_, _ = mock.Process(ctx, "expected-input")

		AssertProcessedWith(t, mock, "expected-input")
	})

	t.Run("AssertProcessedBetween", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		mock.WithReturn(0, nil)

		for i := 0; i < 5; i++ {
			_, _ = mock.Process(ctx, i)
		}

		AssertProcessedBetween(t, mock, 3, 7)
	})
}

func TestChaosProcessor(t *testing.T) {
	ctx := context.Background()

	t.Run("No Chaos Passes Through", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, s string) string {
			return s + "_processed"
		})

		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			FailureRate: 0.0,
			Seed:        12345,
		})

		result, err := chaos.Process(ctx, "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "test_processed" {
			t.Errorf("expected 'test_processed', got %q", result)
		}
	})

	t.Run("Tracks Statistics", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int {
			return n * 2
		})

		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			FailureRate: 0.0,
			Seed:        12345,
		})

		for i := 0; i < 10; i++ {
			_, _ = chaos.Process(ctx, i)
		}

		stats := chaos.Stats()
		if stats.TotalCalls != 10 {
			t.Errorf("expected 10 total calls, got %d", stats.TotalCalls)
		}
	})

	t.Run("Injects Failures At Configured Rate", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int {
			return n
		})

		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			FailureRate: 0.5,
			Seed:        42,
		})

		failures := 0
		for i := 0; i < 100; i++ {
			_, err := chaos.Process(ctx, i)
			if err != nil {
				failures++
			}
		}

		// With 50% failure rate, expect roughly 40-60 failures
		if failures < 30 || failures > 70 {
			t.Errorf("expected ~50 failures, got %d", failures)
		}
	})

	t.Run("Simulates Timeouts", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int {
			return n
		})

		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			TimeoutRate: 1.0, // Always timeout
			Seed:        12345,
		})

		_, err := chaos.Process(ctx, 1)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded, got %v", err)
		}
	})

	t.Run("Stats String Format", func(t *testing.T) {
		stats := ChaosStats{
			TotalCalls:   100,
			FailedCalls:  25,
			TimeoutCalls: 10,
			PanicCalls:   5,
		}

		s := stats.String()
		if s == "" {
			t.Error("expected non-empty string")
		}
	})

	t.Run("Stats Rate Calculations", func(t *testing.T) {
		stats := ChaosStats{
			TotalCalls:   100,
			FailedCalls:  25,
			TimeoutCalls: 10,
			PanicCalls:   5,
		}

		if stats.FailureRate() != 0.25 {
			t.Errorf("expected failure rate 0.25, got %f", stats.FailureRate())
		}
		if stats.TimeoutRate() != 0.10 {
			t.Errorf("expected timeout rate 0.10, got %f", stats.TimeoutRate())
		}
		if stats.PanicRate() != 0.05 {
			t.Errorf("expected panic rate 0.05, got %f", stats.PanicRate())
		}
	})

	t.Run("Stats Zero Calls", func(t *testing.T) {
		stats := ChaosStats{}
		if stats.FailureRate() != 0 {
			t.Errorf("expected 0 failure rate with no calls, got %f", stats.FailureRate())
		}
		if stats.TimeoutRate() != 0 {
			t.Errorf("expected 0 timeout rate with no calls, got %f", stats.TimeoutRate())
		}
		if stats.PanicRate() != 0 {
			t.Errorf("expected 0 panic rate with no calls, got %f", stats.PanicRate())
		}
	})

	t.Run("Name Returns Configured Name", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("my-chaos", base, ChaosConfig{})
		if chaos.Identity().Name() != "my-chaos" {
			t.Errorf("expected name 'my-chaos', got %q", chaos.Identity().Name())
		}
	})

	t.Run("Close Returns Nil", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("chaos", base, ChaosConfig{})
		if err := chaos.Close(); err != nil {
			t.Errorf("expected nil from Close, got %v", err)
		}
	})
}

func TestWaitForCalls(t *testing.T) {
	ctx := context.Background()

	t.Run("Returns True When Calls Reached", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		mock.WithReturn(0, nil)

		go func() {
			time.Sleep(10 * time.Millisecond)
			for i := 0; i < 3; i++ {
				_, _ = mock.Process(ctx, i)
			}
		}()

		if !WaitForCalls(mock, 3, 500*time.Millisecond) {
			t.Error("expected WaitForCalls to return true")
		}
	})

	t.Run("Returns False On Timeout", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")

		if WaitForCalls(mock, 5, 50*time.Millisecond) {
			t.Error("expected WaitForCalls to return false")
		}
	})
}

func TestParallelTest(t *testing.T) {
	t.Run("Runs All Goroutines", func(t *testing.T) {
		var counter int32

		ParallelTest(t, 10, func(_ int) {
			atomic.AddInt32(&counter, 1)
		})

		if counter != 10 {
			t.Errorf("expected 10 goroutines to run, got %d", counter)
		}
	})

	t.Run("Provides Unique IDs", func(t *testing.T) {
		seen := make(map[int]bool)
		var mu sync.Mutex

		ParallelTest(t, 5, func(id int) {
			mu.Lock()
			seen[id] = true
			mu.Unlock()
		})

		if len(seen) != 5 {
			t.Errorf("expected 5 unique IDs, got %d", len(seen))
		}
	})
}

func TestMeasureLatency(t *testing.T) {
	t.Run("Measures Execution Time", func(t *testing.T) {
		latency := MeasureLatency(func() {
			time.Sleep(50 * time.Millisecond)
		})

		if latency < 50*time.Millisecond {
			t.Errorf("expected latency >= 50ms, got %v", latency)
		}
	})
}

func TestMeasureLatencyWithResult(t *testing.T) {
	t.Run("Returns Result And Duration", func(t *testing.T) {
		result, latency := MeasureLatencyWithResult(func() string {
			time.Sleep(50 * time.Millisecond)
			return "done"
		})

		if result != "done" {
			t.Errorf("expected result 'done', got %q", result)
		}
		if latency < 50*time.Millisecond {
			t.Errorf("expected latency >= 50ms, got %v", latency)
		}
	})
}

func TestMockProcessorHistoryEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("WithHistorySize Zero Disables History", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		mock.WithReturn(0, nil).WithHistorySize(0)

		_, _ = mock.Process(ctx, 1)
		_, _ = mock.Process(ctx, 2)

		history := mock.CallHistory()
		if history != nil {
			t.Errorf("expected nil history when disabled, got %v", history)
		}
	})

	t.Run("WithHistorySize Trims Existing History", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		mock.WithReturn(0, nil).WithHistorySize(10)

		// Add 5 entries
		for i := 0; i < 5; i++ {
			_, _ = mock.Process(ctx, i)
		}

		// Trim to 2
		mock.WithHistorySize(2)

		history := mock.CallHistory()
		if len(history) != 2 {
			t.Errorf("expected 2 history entries after trim, got %d", len(history))
		}
		// Should keep the last 2 (3 and 4)
		if history[0].Input != 3 {
			t.Errorf("expected first entry to be 3, got %d", history[0].Input)
		}
	})

	t.Run("CallHistory Returns Nil When Disabled", func(t *testing.T) {
		mock := NewMockProcessor[int](t, "mock")
		mock.WithHistorySize(0)

		history := mock.CallHistory()
		if history != nil {
			t.Errorf("expected nil, got %v", history)
		}
	})
}

func TestChaosProcessorEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("Panic Injection", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			PanicRate: 1.0, // Always panic
			Seed:      12345,
		})

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()

		_, _ = chaos.Process(ctx, 1)
	})

	t.Run("Latency Injection With Range", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			LatencyMin: 10 * time.Millisecond,
			LatencyMax: 20 * time.Millisecond,
			Seed:       12345,
		})

		start := time.Now()
		_, err := chaos.Process(ctx, 1)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if elapsed < 10*time.Millisecond {
			t.Errorf("expected at least 10ms latency, got %v", elapsed)
		}
	})

	t.Run("Latency Injection Min Only", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			LatencyMin: 10 * time.Millisecond,
			LatencyMax: 0, // No max, should use min
			Seed:       12345,
		})

		start := time.Now()
		_, _ = chaos.Process(ctx, 1)
		elapsed := time.Since(start)

		if elapsed < 10*time.Millisecond {
			t.Errorf("expected at least 10ms latency, got %v", elapsed)
		}
	})

	t.Run("Context Cancellation During Latency", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			LatencyMin: 1 * time.Second,
			LatencyMax: 2 * time.Second,
			Seed:       12345,
		})

		ctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel()

		_, err := chaos.Process(ctx, 1)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded, got %v", err)
		}
	})

	t.Run("Stats With Timeouts And Panics", func(t *testing.T) {
		stats := ChaosStats{
			TotalCalls:   100,
			TimeoutCalls: 20,
			PanicCalls:   5,
		}

		if stats.TimeoutRate() != 0.2 {
			t.Errorf("expected timeout rate 0.2, got %f", stats.TimeoutRate())
		}
		if stats.PanicRate() != 0.05 {
			t.Errorf("expected panic rate 0.05, got %f", stats.PanicRate())
		}
	})

	t.Run("Random Seed From Crypto", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		// Seed=0 triggers crypto/rand path
		chaos := NewChaosProcessor("chaos", base, ChaosConfig{
			Seed: 0,
		})

		// Just verify it works
		_, err := chaos.Process(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ChaosProcessor Schema", func(t *testing.T) {
		base := pipz.Transform(pipz.NewIdentity("base", ""), func(_ context.Context, n int) int { return n })
		chaos := NewChaosProcessor("chaos-schema", base, ChaosConfig{})

		schema := chaos.Schema()

		if schema.Type != "chaos" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "chaos")
		}
		if schema.Identity.Name() != "chaos-schema" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "chaos-schema")
		}
	})
}
