package pipz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/clockz"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("Normal Operation in Closed State", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			calls++
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, time.Second)

		// Should allow normal operation
		for i := 0; i < 5; i++ {
			result, err := breaker.Process(context.Background(), i)
			if err != nil {
				t.Fatalf("unexpected error on request %d: %v", i, err)
			}
			if result != i*2 {
				t.Errorf("expected %d, got %d", i*2, result)
			}
		}

		if calls != 5 {
			t.Errorf("expected 5 calls, got %d", calls)
		}
		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state, got %s", state)
		}
	})

	t.Run("Opens After Failure Threshold", func(t *testing.T) {
		failureCount := 0
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			failureCount++
			return 0, errors.New("service error")
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, time.Second)

		// First 3 failures should pass through
		for i := 0; i < 3; i++ {
			_, err := breaker.Process(context.Background(), i)
			if err == nil {
				t.Fatal("expected error")
			}
		}

		if failureCount != 3 {
			t.Errorf("expected 3 calls before opening, got %d", failureCount)
		}
		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state after threshold, got %s", state)
		}

		// Next call should fail fast without calling processor
		_, err := breaker.Process(context.Background(), 99)
		if err == nil {
			t.Fatal("expected circuit open error")
		}
		if !strings.Contains(err.Error(), "circuit breaker is open") {
			t.Errorf("unexpected error: %v", err)
		}
		if failureCount != 3 {
			t.Errorf("processor should not be called when open, but got %d calls", failureCount)
		}
	})

	t.Run("Resets to Half-Open After Timeout", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		failureCount := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			failureCount++
			if failureCount <= 3 {
				return 0, errors.New("service error")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, 5*time.Second)
		breaker.WithClock(clock)

		// Trigger opening
		for i := 0; i < 3; i++ {
			_, err := breaker.Process(context.Background(), i)
			if err == nil {
				t.Fatal("expected error")
			}
		}

		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state, got %s", state)
		}

		// Advance time past reset timeout
		clock.Advance(6 * time.Second)

		// Should transition to half-open
		if state := breaker.GetState(); state != "half-open" {
			t.Errorf("expected half-open state after timeout, got %s", state)
		}

		// Next request should be allowed
		result, err := breaker.Process(context.Background(), 42)
		if err != nil {
			t.Fatalf("unexpected error in half-open: %v", err)
		}
		if result != 84 {
			t.Errorf("expected 84, got %d", result)
		}

		// Should be closed after success
		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after half-open success, got %s", state)
		}
	})

	t.Run("Half-Open Returns to Open on Failure", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		callCount := 0
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			callCount++
			return 0, errors.New("still broken")
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, 5*time.Second)
		breaker.WithClock(clock)

		// Trigger opening
		for i := 0; i < 3; i++ {
			breaker.Process(context.Background(), i)
		}

		// Advance to half-open
		clock.Advance(6 * time.Second)

		// Failure in half-open should reopen immediately
		_, err := breaker.Process(context.Background(), 99)
		if err == nil {
			t.Fatal("expected error")
		}

		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state after half-open failure, got %s", state)
		}

		// Should need to wait again
		clock.Advance(3 * time.Second) // Not enough time
		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected still open, got %s", state)
		}

		clock.Advance(3 * time.Second) // Now past timeout
		if state := breaker.GetState(); state != "half-open" {
			t.Errorf("expected half-open after second timeout, got %s", state)
		}
	})

	t.Run("Success Resets Failure Count", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			if n == 999 {
				return 0, errors.New("error")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, time.Second)

		// Two failures
		breaker.Process(context.Background(), 999)
		breaker.Process(context.Background(), 999)

		// Success should reset counter
		result, err := breaker.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 2 {
			t.Errorf("expected 2, got %d", result)
		}

		// Two more failures shouldn't open (count reset)
		breaker.Process(context.Background(), 999)
		breaker.Process(context.Background(), 999)

		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after reset, got %s", state)
		}

		// One more failure should open
		breaker.Process(context.Background(), 999)

		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state after 3 consecutive failures, got %s", state)
		}
	})

	t.Run("Multiple Success Threshold in Half-Open", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		callCount := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			callCount++
			if callCount <= 3 {
				return 0, errors.New("failure")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, 5*time.Second)
		breaker.SetSuccessThreshold(2) // Need 2 successes to close
		breaker.WithClock(clock)

		// Open the breaker
		for i := 0; i < 3; i++ {
			breaker.Process(context.Background(), i)
		}

		// Move to half-open
		clock.Advance(6 * time.Second)

		// First success
		result, err := breaker.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 2 {
			t.Errorf("expected 2, got %d", result)
		}

		// Should still be half-open
		if state := breaker.GetState(); state != "half-open" {
			t.Errorf("expected still half-open after 1 success, got %s", state)
		}

		// Second success should close
		result, err = breaker.Process(context.Background(), 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 4 {
			t.Errorf("expected 4, got %d", result)
		}

		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed after 2 successes, got %s", state)
		}
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		failAfter := atomic.Int32{}
		failAfter.Store(100) // Allow first 100 calls

		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			count := failAfter.Add(-1)
			if count < 0 {
				return 0, errors.New("failure")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 10, time.Second)

		var wg sync.WaitGroup
		errors := make(chan error, 1000)

		// Launch concurrent requests
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_, err := breaker.Process(context.Background(), id*100+j)
					if err != nil && !strings.Contains(err.Error(), "circuit breaker is open") &&
						!strings.Contains(err.Error(), "failure") {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for unexpected errors
		for err := range errors {
			t.Errorf("unexpected error: %v", err)
		}

		// Circuit should be open after failures
		state := breaker.GetState()
		if state != "open" && state != "closed" {
			t.Errorf("expected open or closed state, got %s", state)
		}
	})

	t.Run("Manual Reset", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("failure")
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 1, time.Hour)

		// Open the breaker
		breaker.Process(context.Background(), 1)

		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state, got %s", state)
		}

		// Manual reset
		breaker.Reset()

		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after reset, got %s", state)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		breaker := NewCircuitBreaker("test", processor, 5, 10*time.Second)

		// Test getters with defaults
		if threshold := breaker.GetFailureThreshold(); threshold != 5 {
			t.Errorf("expected failure threshold 5, got %d", threshold)
		}
		if threshold := breaker.GetSuccessThreshold(); threshold != 1 {
			t.Errorf("expected success threshold 1, got %d", threshold)
		}
		if timeout := breaker.GetResetTimeout(); timeout != 10*time.Second {
			t.Errorf("expected reset timeout 10s, got %v", timeout)
		}

		// Test setters with chaining
		result := breaker.SetFailureThreshold(10).SetSuccessThreshold(3).SetResetTimeout(30 * time.Second)
		if result != breaker {
			t.Error("setters should return the same instance for chaining")
		}

		// Verify changes
		if threshold := breaker.GetFailureThreshold(); threshold != 10 {
			t.Errorf("expected failure threshold 10, got %d", threshold)
		}
		if threshold := breaker.GetSuccessThreshold(); threshold != 3 {
			t.Errorf("expected success threshold 3, got %d", threshold)
		}
		if timeout := breaker.GetResetTimeout(); timeout != 30*time.Second {
			t.Errorf("expected reset timeout 30s, got %v", timeout)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		breaker := NewCircuitBreaker("my-breaker", processor, 3, time.Second)

		if name := breaker.Name(); name != "my-breaker" {
			t.Errorf("expected name 'my-breaker', got %q", name)
		}
	})

	t.Run("Panic Recovery", func(t *testing.T) {
		panicProcessor := Apply("panic", func(_ context.Context, _ int) (int, error) {
			panic("processor panic")
		})

		breaker := NewCircuitBreaker("panic-breaker", panicProcessor, 3, time.Second)
		data := 42

		result, err := breaker.Process(context.Background(), data)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "panic-breaker" {
			t.Errorf("expected path to start with 'panic-breaker', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	t.Run("Generation Protection in Half-Open", func(t *testing.T) {
		// This test ensures that concurrent requests in half-open state
		// are handled correctly with the generation counter
		clock := clockz.NewFakeClock()
		processingDelay := make(chan struct{})
		completions := atomic.Int32{}

		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			// First request blocks, others fail
			if completions.Add(1) == 1 {
				<-processingDelay
				return n * 2, nil
			}
			return 0, errors.New("failure")
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 3, 5*time.Second)
		breaker.WithClock(clock)

		// Open the breaker
		for i := 0; i < 3; i++ {
			completions.Store(10) // Skip blocking for setup
			breaker.Process(context.Background(), i)
		}
		completions.Store(0)

		// Move to half-open
		clock.Advance(6 * time.Second)

		// Launch multiple concurrent requests
		var wg sync.WaitGroup
		results := make(chan error, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				_, err := breaker.Process(context.Background(), id)
				results <- err
			}(i)
		}

		// Let requests start
		time.Sleep(10 * time.Millisecond)

		// Unblock the first request
		close(processingDelay)

		wg.Wait()
		close(results)

		// Count outcomes
		successCount := 0
		failCount := 0
		for err := range results {
			if err == nil {
				successCount++
			} else {
				failCount++
			}
		}

		// At least one should succeed (the blocked one)
		if successCount < 1 {
			t.Errorf("expected at least 1 success, got %d", successCount)
		}
	})

	t.Run("Pipeline Integration", func(t *testing.T) {
		failUntil := 3
		callCount := 0
		processor := Apply("flaky", func(_ context.Context, n int) (int, error) {
			callCount++
			if callCount <= failUntil {
				return 0, errors.New("temporary failure")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("breaker", processor, 5, time.Second)
		retry := NewRetry("retry", breaker, 10)

		// Should retry through the circuit breaker
		result, err := retry.Process(context.Background(), 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 84 {
			t.Errorf("expected 84, got %d", result)
		}

		// Verify breaker is still closed (failures were spread across retries)
		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state, got %s", state)
		}
	})
}

func TestCircuitBreakerClose(t *testing.T) {
	t.Run("Closes Child Processor", func(t *testing.T) {
		p := newTrackingProcessor[int]("p")

		cb := NewCircuitBreaker("test", p, 3, time.Second)
		err := cb.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})

	t.Run("Propagates Close Error", func(t *testing.T) {
		p := newTrackingProcessor[int]("p").WithCloseError(errors.New("close error"))

		cb := NewCircuitBreaker("test", p, 3, time.Second)
		err := cb.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[int]("p")
		cb := NewCircuitBreaker("test", p, 3, time.Second)

		_ = cb.Close()
		_ = cb.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})
}

func BenchmarkCircuitBreaker(b *testing.B) {
	b.Run("Closed State", func(b *testing.B) {
		processor := Transform("test", func(_ context.Context, n int) int { return n * 2 })
		breaker := NewCircuitBreaker("bench-breaker", processor, 100, time.Second)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := breaker.Process(ctx, i)
			_ = result
			_ = err
		}
	})

	b.Run("Open State", func(b *testing.B) {
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("failure")
		})
		breaker := NewCircuitBreaker("bench-breaker", processor, 1, time.Hour)
		ctx := context.Background()

		// Open the circuit
		breaker.Process(ctx, 0)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := breaker.Process(ctx, i)
			_ = result
			_ = err
		}
	})

	b.Run("Mixed State with Clock", func(b *testing.B) {
		clock := clockz.NewFakeClock()
		shouldFail := atomic.Bool{}
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			if shouldFail.Load() {
				return 0, errors.New("failure")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("bench-breaker", processor, 3, 50*time.Millisecond)
		breaker.WithClock(clock)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Alternate between success and failure patterns
			shouldFail.Store(i%10 < 3)
			_, err := breaker.Process(ctx, i)
			_ = err // Intentionally ignore error in benchmark

			// Occasionally advance clock instead of sleeping
			if i%100 == 0 {
				clock.Advance(60 * time.Millisecond)
			}
		}
	})
}

func TestCircuitBreakerGuardClauses(t *testing.T) {
	processor := Transform("test", func(_ context.Context, n int) int { return n })

	t.Run("NewCircuitBreaker clamps failureThreshold", func(t *testing.T) {
		breaker := NewCircuitBreaker("test", processor, 0, time.Second)
		if threshold := breaker.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected threshold clamped to 1, got %d", threshold)
		}

		breaker2 := NewCircuitBreaker("test", processor, -5, time.Second)
		if threshold := breaker2.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected threshold clamped to 1, got %d", threshold)
		}
	})

	t.Run("SetFailureThreshold clamps to 1", func(t *testing.T) {
		breaker := NewCircuitBreaker("test", processor, 5, time.Second)
		breaker.SetFailureThreshold(0)
		if threshold := breaker.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected threshold clamped to 1, got %d", threshold)
		}

		breaker.SetFailureThreshold(-10)
		if threshold := breaker.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected threshold clamped to 1, got %d", threshold)
		}
	})

	t.Run("SetSuccessThreshold clamps to 1", func(t *testing.T) {
		breaker := NewCircuitBreaker("test", processor, 5, time.Second)
		breaker.SetSuccessThreshold(0)
		if threshold := breaker.GetSuccessThreshold(); threshold != 1 {
			t.Errorf("expected threshold clamped to 1, got %d", threshold)
		}

		breaker.SetSuccessThreshold(-10)
		if threshold := breaker.GetSuccessThreshold(); threshold != 1 {
			t.Errorf("expected threshold clamped to 1, got %d", threshold)
		}
	})
}
