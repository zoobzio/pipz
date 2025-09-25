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
	"github.com/zoobzio/tracez"
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

	t.Run("Observability", func(t *testing.T) {
		t.Run("Metrics and Spans - State Transitions", func(t *testing.T) {
			clock := clockz.NewFakeClock()
			failCount := 0
			processor := Apply("test", func(_ context.Context, n int) (int, error) {
				failCount++
				if failCount <= 3 {
					return 0, errors.New("service error")
				}
				return n * 2, nil
			})

			breaker := NewCircuitBreaker("test-breaker", processor, 3, 5*time.Second)
			breaker.WithClock(clock)
			defer breaker.Close()

			// Verify observability components are initialized
			if breaker.Metrics() == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if breaker.Tracer() == nil {
				t.Error("expected tracer to be initialized")
			}

			// Capture spans
			var spans []tracez.Span
			var spanMu sync.Mutex
			breaker.Tracer().OnSpanComplete(func(span tracez.Span) {
				spanMu.Lock()
				spans = append(spans, span)
				spanMu.Unlock()
			})

			// Initial state should be closed (0)
			currentState := breaker.Metrics().Gauge(CircuitBreakerCurrentState).Value()
			if currentState != 0 {
				t.Errorf("expected initial state gauge 0 (closed), got %f", currentState)
			}

			// Trigger 3 failures to open the circuit
			for i := 0; i < 3; i++ {
				_, err := breaker.Process(context.Background(), i)
				if err == nil {
					t.Fatal("expected error")
				}
			}

			// Check metrics after opening
			processedTotal := breaker.Metrics().Counter(CircuitBreakerProcessedTotal).Value()
			if processedTotal != 3 {
				t.Errorf("expected 3 processed, got %f", processedTotal)
			}

			failuresTotal := breaker.Metrics().Counter(CircuitBreakerFailuresTotal).Value()
			if failuresTotal != 3 {
				t.Errorf("expected 3 failures, got %f", failuresTotal)
			}

			openedTotal := breaker.Metrics().Counter(CircuitBreakerOpenedTotal).Value()
			if openedTotal != 1 {
				t.Errorf("expected 1 circuit opened event, got %f", openedTotal)
			}

			stateTransitions := breaker.Metrics().Counter(CircuitBreakerStateTransitions).Value()
			if stateTransitions != 1 {
				t.Errorf("expected 1 state transition (closed->open), got %f", stateTransitions)
			}

			currentState = breaker.Metrics().Gauge(CircuitBreakerCurrentState).Value()
			if currentState != 1 {
				t.Errorf("expected state gauge 1 (open), got %f", currentState)
			}

			consecutiveFails := breaker.Metrics().Gauge(CircuitBreakerConsecutiveFails).Value()
			if consecutiveFails != 3 {
				t.Errorf("expected 3 consecutive failures, got %f", consecutiveFails)
			}

			// Try a request while open - should be rejected
			_, err := breaker.Process(context.Background(), 99)
			if err == nil {
				t.Fatal("expected circuit open error")
			}

			rejectedTotal := breaker.Metrics().Counter(CircuitBreakerRejectedTotal).Value()
			if rejectedTotal != 1 {
				t.Errorf("expected 1 rejected request, got %f", rejectedTotal)
			}

			// Advance time to transition to half-open
			clock.Advance(6 * time.Second)

			// Next request should succeed (processor now returns success)
			result, err := breaker.Process(context.Background(), 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != 84 {
				t.Errorf("expected 84, got %d", result)
			}

			// Check final metrics
			successesTotal := breaker.Metrics().Counter(CircuitBreakerSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			// Should have transitioned from open->half-open->closed
			stateTransitions = breaker.Metrics().Counter(CircuitBreakerStateTransitions).Value()
			if stateTransitions != 3 { // closed->open, open->half-open, half-open->closed
				t.Errorf("expected 3 state transitions, got %f", stateTransitions)
			}

			currentState = breaker.Metrics().Gauge(CircuitBreakerCurrentState).Value()
			if currentState != 0 {
				t.Errorf("expected state gauge 0 (closed) after recovery, got %f", currentState)
			}

			// Verify spans were captured
			spanMu.Lock()
			spanCount := len(spans)
			spanMu.Unlock()

			if spanCount != 5 { // 3 failures + 1 rejected + 1 success
				t.Errorf("expected 5 spans, got %d", spanCount)
			}

			// Check span details
			spanMu.Lock()
			for i, span := range spans {
				if span.Name != CircuitBreakerProcessSpan {
					t.Errorf("span %d: expected name %s, got %s", i, CircuitBreakerProcessSpan, span.Name)
				}

				if _, ok := span.Tags[CircuitBreakerTagState]; !ok {
					t.Errorf("span %d: missing state tag", i)
				}

				if i == 3 { // The rejected request
					if allowed, ok := span.Tags[CircuitBreakerTagAllowed]; !ok || allowed != "false" {
						t.Errorf("span %d: expected allowed=false for rejected request", i)
					}
				}

				if i == 4 { // The successful request that triggered state changes
					if stateChange, ok := span.Tags[CircuitBreakerTagStateChange]; ok {
						if !strings.Contains(stateChange, "open->half-open") {
							t.Errorf("span %d: expected state change tag to contain 'open->half-open', got %s", i, stateChange)
						}
					}
				}
			}
			spanMu.Unlock()
		})

		t.Run("Metrics - Half-Open Failure", func(t *testing.T) {
			clock := clockz.NewFakeClock()
			callCount := 0
			processor := Apply("test", func(_ context.Context, _ int) (int, error) {
				callCount++
				return 0, errors.New("still broken")
			})

			breaker := NewCircuitBreaker("test-half-open", processor, 2, 5*time.Second)
			breaker.WithClock(clock)
			defer breaker.Close()

			// Open the circuit
			for i := 0; i < 2; i++ {
				breaker.Process(context.Background(), i)
			}

			openedCount := breaker.Metrics().Counter(CircuitBreakerOpenedTotal).Value()
			if openedCount != 1 {
				t.Errorf("expected 1 open event, got %f", openedCount)
			}

			// Move to half-open
			clock.Advance(6 * time.Second)

			// Failure in half-open should reopen
			breaker.Process(context.Background(), 99)

			// Should have opened again
			openedCount = breaker.Metrics().Counter(CircuitBreakerOpenedTotal).Value()
			if openedCount != 2 {
				t.Errorf("expected 2 open events after half-open failure, got %f", openedCount)
			}

			stateTransitions := breaker.Metrics().Counter(CircuitBreakerStateTransitions).Value()
			if stateTransitions != 3 { // closed->open, open->half-open, half-open->open
				t.Errorf("expected 3 state transitions, got %f", stateTransitions)
			}
		})

		t.Run("Metrics - Success Threshold", func(t *testing.T) {
			clock := clockz.NewFakeClock()
			callCount := 0
			processor := Apply("test", func(_ context.Context, n int) (int, error) {
				callCount++
				if callCount <= 2 {
					return 0, errors.New("failure")
				}
				return n * 2, nil
			})

			breaker := NewCircuitBreaker("test-threshold", processor, 2, 5*time.Second)
			breaker.SetSuccessThreshold(3) // Need 3 successes to close
			breaker.WithClock(clock)
			defer breaker.Close()

			// Open the circuit
			for i := 0; i < 2; i++ {
				breaker.Process(context.Background(), i)
			}

			// Move to half-open
			clock.Advance(6 * time.Second)

			// Process 3 successful requests
			for i := 0; i < 3; i++ {
				_, err := breaker.Process(context.Background(), i)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			// Should be closed after 3 successes
			currentState := breaker.Metrics().Gauge(CircuitBreakerCurrentState).Value()
			if currentState != 0 {
				t.Errorf("expected state gauge 0 (closed), got %f", currentState)
			}

			successesTotal := breaker.Metrics().Counter(CircuitBreakerSuccessesTotal).Value()
			if successesTotal != 3 {
				t.Errorf("expected 3 successes, got %f", successesTotal)
			}
		})

		t.Run("Hooks fire on state changes", func(t *testing.T) {
			clock := clockz.NewFakeClock()
			processor := Apply("test", func(_ context.Context, n int) (int, error) {
				if n < 0 {
					return 0, errors.New("negative number")
				}
				return n * 2, nil
			})

			breaker := NewCircuitBreaker("test-hooks", processor, 2, time.Minute)
			breaker.WithClock(clock)
			defer breaker.Close()

			var openedCount atomic.Int32
			var closedCount atomic.Int32

			// Register hooks
			breaker.OnOpened(func(_ context.Context, _ CircuitBreakerStateChange) error {
				openedCount.Add(1)
				return nil
			})

			breaker.OnClosed(func(_ context.Context, _ CircuitBreakerStateChange) error {
				closedCount.Add(1)
				return nil
			})

			// Trigger failures to open circuit
			breaker.Process(context.Background(), -1)
			breaker.Process(context.Background(), -1)

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			if openedCount.Load() != 1 {
				t.Errorf("expected 1 opened event, got %d", openedCount.Load())
			}

			// Move to half-open after timeout
			clock.Advance(time.Minute + time.Second)

			// Successful request should close circuit
			breaker.Process(context.Background(), 5)

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			if closedCount.Load() != 1 {
				t.Errorf("expected 1 closed event, got %d", closedCount.Load())
			}
		})
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
