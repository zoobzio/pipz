package pipz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
			if !strings.Contains(err.Error(), "service error") {
				t.Errorf("unexpected error: %v", err)
			}
		}

		// Circuit should now be open
		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state after failures, got %s", state)
		}

		// Next request should fail immediately without calling processor
		_, err := breaker.Process(context.Background(), 99)
		if err == nil {
			t.Fatal("expected circuit open error")
		}
		if !strings.Contains(err.Error(), "circuit breaker is open") {
			t.Errorf("unexpected error: %v", err)
		}

		// Processor should not have been called again
		if failureCount != 3 {
			t.Errorf("expected 3 calls, got %d", failureCount)
		}
	})

	t.Run("Half-Open State Recovery", func(t *testing.T) {
		attempts := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			attempts++
			if attempts <= 3 {
				return 0, errors.New("still failing")
			}
			return n * 2, nil // Success after 3 attempts
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 2, 100*time.Millisecond)

		// Trigger circuit open with 2 failures
		for i := 0; i < 2; i++ {
			_, err := breaker.Process(context.Background(), i)
			if err == nil {
				t.Errorf("expected error on attempt %d", i)
			}
		}

		if state := breaker.GetState(); state != "open" {
			t.Fatalf("expected open state, got %s", state)
		}

		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)

		// Should be half-open now
		if state := breaker.GetState(); state != "half-open" {
			t.Errorf("expected half-open state after timeout, got %s", state)
		}

		// First attempt in half-open fails, should reopen
		_, err := breaker.Process(context.Background(), 10)
		if err == nil {
			t.Fatal("expected error in half-open")
		}

		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state after half-open failure, got %s", state)
		}

		// Wait again for reset timeout
		time.Sleep(150 * time.Millisecond)

		// Now it should succeed and close the circuit
		result, err := breaker.Process(context.Background(), 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}

		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after recovery, got %s", state)
		}
	})

	t.Run("Success Threshold for Recovery", func(t *testing.T) {
		successes := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			if n < 0 {
				return 0, errors.New("negative input")
			}
			successes++
			return n, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 2, 100*time.Millisecond)
		breaker.SetSuccessThreshold(3) // Need 3 successes to close

		// Open the circuit
		for i := 0; i < 2; i++ {
			_, err := breaker.Process(context.Background(), -1)
			if err == nil {
				t.Errorf("expected error on attempt %d", i)
			}
		}

		// Wait for half-open
		time.Sleep(150 * time.Millisecond)

		// First two successes shouldn't close the circuit
		for i := 0; i < 2; i++ {
			_, err := breaker.Process(context.Background(), i)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Should still be half-open
			if state := breaker.GetState(); state != "half-open" {
				t.Errorf("expected half-open state after %d successes, got %s", i+1, state)
			}
		}

		// Third success should close it
		_, err := breaker.Process(context.Background(), 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after 3 successes, got %s", state)
		}
	})

	t.Run("Manual Reset", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("always fails")
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 1, time.Hour) // Long timeout

		// Open the circuit
		_, err := breaker.Process(context.Background(), 1)
		if err == nil {
			t.Error("expected error")
		}

		if state := breaker.GetState(); state != "open" {
			t.Fatalf("expected open state, got %s", state)
		}

		// Manual reset
		breaker.Reset()

		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after reset, got %s", state)
		}

		// Should allow requests again (even though they fail)
		_, err = breaker.Process(context.Background(), 2)
		if err == nil {
			t.Fatal("expected error from processor")
		}
		if strings.Contains(err.Error(), "circuit breaker is open") {
			t.Error("circuit should not be open after reset")
		}
	})

	t.Run("Concurrent Access Safety", func(t *testing.T) {
		var failureToggle atomic.Bool
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			if failureToggle.Load() {
				return 0, errors.New("failure")
			}
			return n, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 5, 50*time.Millisecond)

		var wg sync.WaitGroup
		errors := make(chan error, 1000)

		// Multiple goroutines making requests
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					// Toggle failures periodically
					if j%5 == 0 {
						failureToggle.Store(!failureToggle.Load())
					}

					_, err := breaker.Process(context.Background(), id*100+j)
					if err != nil && !strings.Contains(err.Error(), "failure") &&
						!strings.Contains(err.Error(), "circuit breaker is open") {
						errors <- err
					}

					time.Sleep(10 * time.Millisecond)
				}
			}(i)
		}

		// Goroutine modifying configuration
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				breaker.SetFailureThreshold(3 + i%3)
				breaker.SetSuccessThreshold(1 + i%2)
				breaker.SetResetTimeout(time.Duration(50+i*10) * time.Millisecond)
				time.Sleep(20 * time.Millisecond)
			}
		}()

		wg.Wait()
		close(errors)

		// Check for any unexpected errors
		for err := range errors {
			t.Errorf("concurrent access error: %v", err)
		}
	})

	t.Run("Runtime Configuration Changes", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			return n, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 5, time.Second)

		// Verify initial settings
		if threshold := breaker.GetFailureThreshold(); threshold != 5 {
			t.Errorf("expected failure threshold 5, got %d", threshold)
		}
		if threshold := breaker.GetSuccessThreshold(); threshold != 1 {
			t.Errorf("expected success threshold 1, got %d", threshold)
		}
		if timeout := breaker.GetResetTimeout(); timeout != time.Second {
			t.Errorf("expected reset timeout 1s, got %v", timeout)
		}

		// Update settings
		breaker.SetFailureThreshold(3).
			SetSuccessThreshold(2).
			SetResetTimeout(500 * time.Millisecond)

		// Verify updates
		if threshold := breaker.GetFailureThreshold(); threshold != 3 {
			t.Errorf("expected failure threshold 3, got %d", threshold)
		}
		if threshold := breaker.GetSuccessThreshold(); threshold != 2 {
			t.Errorf("expected success threshold 2, got %d", threshold)
		}
		if timeout := breaker.GetResetTimeout(); timeout != 500*time.Millisecond {
			t.Errorf("expected reset timeout 500ms, got %v", timeout)
		}
	})

	t.Run("Error Path Preservation", func(t *testing.T) {
		innerProcessor := Apply("inner", func(_ context.Context, _ int) (int, error) {
			return 0, &Error[int]{
				Err:       errors.New("inner error"),
				Path:      []Name{"deep", "nested"},
				Timestamp: time.Now(),
			}
		})

		breaker := NewCircuitBreaker("breaker", innerProcessor, 3, time.Second)

		_, err := breaker.Process(context.Background(), 1)
		if err == nil {
			t.Fatal("expected error")
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatalf("expected Error type, got %T", err)
		}

		// Path should include circuit breaker prepended to the existing path
		expectedPath := []Name{"breaker", "inner"}
		if len(pipzErr.Path) != len(expectedPath) {
			t.Errorf("expected path length %d, got %d", len(expectedPath), len(pipzErr.Path))
		}
		for i, name := range expectedPath {
			if i < len(pipzErr.Path) && pipzErr.Path[i] != name {
				t.Errorf("expected path[%d] = %s, got %s", i, name, pipzErr.Path[i])
			}
		}
	})

	t.Run("Integration with Pipeline", func(t *testing.T) {
		attempts := atomic.Int32{}
		processor := Apply("flaky", func(_ context.Context, n int) (int, error) {
			attempt := attempts.Add(1)
			if attempt <= 2 {
				return 0, errors.New("temporary failure")
			}
			return n * 2, nil
		})

		// Circuit breaker that opens after 3 failures
		breaker := NewCircuitBreaker("breaker", processor, 3, 100*time.Millisecond)

		// Retry that tries 5 times
		retry := NewRetry("retry", breaker, 5)

		// First call will fail twice then succeed on third attempt
		result, err := retry.Process(context.Background(), 10)
		if err != nil {
			t.Fatalf("unexpected error on first call: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}

		// Now make the processor always fail
		failingProcessor := Apply("always-fail", func(_ context.Context, _ int) (int, error) {
			attempts.Add(1)
			return 0, errors.New("permanent failure")
		})

		// Create new circuit breaker with the failing processor
		failBreaker := NewCircuitBreaker("fail-breaker", failingProcessor, 3, 100*time.Millisecond)
		failRetry := NewRetry("fail-retry", failBreaker, 5)

		// Reset counter
		attempts.Store(0)

		// This should fail 3 times and open the circuit
		_, err = failRetry.Process(context.Background(), 20)
		if err == nil {
			t.Fatal("expected error")
		}

		// The circuit should open after 3 failures, preventing further retries
		finalAttempts := attempts.Load()
		if finalAttempts != 3 {
			t.Errorf("expected 3 attempts before circuit opens, got %d", finalAttempts)
		}

		// Verify the error mentions circuit breaker
		if !strings.Contains(err.Error(), "circuit breaker is open") && !strings.Contains(err.Error(), "permanent failure") {
			t.Errorf("expected circuit breaker or permanent failure error, got: %v", err)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			return n, nil
		})

		breaker := NewCircuitBreaker("test-circuit-breaker", processor, 3, time.Second)
		// Test Name() method
		name := breaker.Name()
		if name != "test-circuit-breaker" {
			t.Errorf("expected name 'test-circuit-breaker', got '%s'", name)
		}
		// Test name is preserved during concurrent access
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					n := breaker.Name()
					if n != "test-circuit-breaker" {
						t.Errorf("expected name 'test-circuit-breaker', got '%s'", n)
					}
				}
			}()
		}
		wg.Wait()
	})

	t.Run("Constructor Edge Cases", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			return n, nil
		})
		// Test with zero failure threshold - should default to 1
		breaker := NewCircuitBreaker("test-breaker", processor, 0, time.Second)
		if threshold := breaker.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected failure threshold to default to 1, got %d", threshold)
		}

		// Test with negative failure threshold - should default to 1
		breaker2 := NewCircuitBreaker("test-breaker", processor, -5, time.Second)
		if threshold := breaker2.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected failure threshold to default to 1, got %d", threshold)
		}

		// Test SetFailureThreshold with invalid values
		breaker.SetFailureThreshold(0)
		if threshold := breaker.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected failure threshold to default to 1 after setting to 0, got %d", threshold)
		}

		breaker.SetFailureThreshold(-10)
		if threshold := breaker.GetFailureThreshold(); threshold != 1 {
			t.Errorf("expected failure threshold to default to 1 after setting to negative, got %d", threshold)
		}

		// Test SetSuccessThreshold with invalid values
		breaker.SetSuccessThreshold(0)
		if threshold := breaker.GetSuccessThreshold(); threshold != 1 {
			t.Errorf("expected success threshold to default to 1 after setting to 0, got %d", threshold)
		}

		breaker.SetSuccessThreshold(-5)
		if threshold := breaker.GetSuccessThreshold(); threshold != 1 {
			t.Errorf("expected success threshold to default to 1 after setting to negative, got %d", threshold)
		}
	})

	t.Run("Generation Race Condition", func(t *testing.T) {
		// This test ensures that if the generation changes while a request is being processed,
		// the result of that request doesn't affect the new generation's state
		var processingDelay atomic.Bool
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			if processingDelay.Load() {
				time.Sleep(100 * time.Millisecond)
			}
			if n < 0 {
				return 0, errors.New("negative input")
			}
			return n, nil
		})

		breaker := NewCircuitBreaker("test-breaker", processor, 2, 50*time.Millisecond)

		// Open the circuit
		for i := 0; i < 2; i++ {
			_, err := breaker.Process(context.Background(), -1)
			if err == nil {
				t.Error("expected error")
			}
		}

		if state := breaker.GetState(); state != "open" {
			t.Fatalf("expected open state, got %s", state)
		}

		// Wait for half-open
		time.Sleep(60 * time.Millisecond)

		// Start a slow request in half-open state
		processingDelay.Store(true)
		var wg sync.WaitGroup
		wg.Add(1)
		var slowErr error
		go func() {
			defer wg.Done()
			_, slowErr = breaker.Process(context.Background(), 1)
		}()

		// While the slow request is processing, reset the circuit (changes generation)
		time.Sleep(10 * time.Millisecond)
		breaker.Reset()

		// The slow request should complete but not affect the new generation's state
		wg.Wait()

		// Circuit should still be closed after reset, regardless of slow request outcome
		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after reset, got %s", state)
		}

		// Verify the slow request completed successfully
		if slowErr != nil {
			t.Errorf("unexpected error from slow request: %v", slowErr)
		}
	})

	t.Run("CircuitBreaker panic recovery", func(t *testing.T) {
		calls := 0
		processor := Apply("panic_processor", func(_ context.Context, n int) (int, error) {
			calls++
			if calls < 2 {
				panic("circuit breaker processor panic")
			}
			return n * 2, nil
		})

		breaker := NewCircuitBreaker("panic_breaker", processor, 3, 100*time.Millisecond)
		result, err := breaker.Process(context.Background(), 42)

		// First call should panic and be recovered as error, incrementing failure count
		// When panic is recovered, the zero value is returned, not original input
		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "panic_breaker" {
			t.Errorf("expected path to start with 'panic_breaker', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}

		if calls != 1 {
			t.Errorf("expected 1 call (panic recovered), got %d", calls)
		}

		// Circuit should still be closed (1 failure < 3 threshold)
		if state := breaker.GetState(); state != "closed" {
			t.Errorf("expected closed state after single panic, got %s", state)
		}

		// Second call should succeed
		result2, err2 := breaker.Process(context.Background(), 42)
		if err2 != nil {
			t.Fatalf("unexpected error on second call: %v", err2)
		}

		if result2 != 84 {
			t.Errorf("expected 84 on second call, got %d", result2)
		}

		if calls != 2 {
			t.Errorf("expected 2 calls total, got %d", calls)
		}
	})

	t.Run("CircuitBreaker opens on repeated panics", func(t *testing.T) {
		calls := 0
		processor := Apply("panic_processor", func(_ context.Context, _ int) (int, error) {
			calls++
			panic("circuit breaker processor panic")
		})

		breaker := NewCircuitBreaker("panic_breaker", processor, 3, 100*time.Millisecond)

		// Make 3 calls to open the circuit
		for i := 0; i < 3; i++ {
			result, err := breaker.Process(context.Background(), 42)
			if result != 0 {
				t.Errorf("expected zero value 0 on call %d, got %d", i+1, result)
			}
			if err == nil {
				t.Errorf("expected error on call %d", i+1)
			}
		}

		if calls != 3 {
			t.Errorf("expected 3 calls before circuit opens, got %d", calls)
		}

		// Circuit should now be open
		if state := breaker.GetState(); state != "open" {
			t.Errorf("expected open state after 3 panics, got %s", state)
		}

		// Fourth call should be rejected without calling processor
		result, err := breaker.Process(context.Background(), 42)
		if result != 42 {
			t.Errorf("expected original input 42 (circuit open), got %d", result)
		}
		if err == nil {
			t.Fatal("expected circuit open error")
		}
		if !strings.Contains(err.Error(), "circuit breaker is open") {
			t.Errorf("expected circuit open error, got: %v", err)
		}

		// Processor should not have been called again
		if calls != 3 {
			t.Errorf("expected calls to remain at 3 after circuit open, got %d", calls)
		}
	})
}

func BenchmarkCircuitBreaker(b *testing.B) {
	b.Run("Closed State", func(b *testing.B) {
		processor := Apply("bench", func(_ context.Context, n int) (int, error) {
			return n, nil
		})
		breaker := NewCircuitBreaker("bench-breaker", processor, 10, time.Second)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := breaker.Process(ctx, i)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Open State", func(b *testing.B) {
		processor := Apply("bench", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("failure")
		})
		breaker := NewCircuitBreaker("bench-breaker", processor, 1, time.Hour)
		ctx := context.Background()

		// Open the circuit
		_, err := breaker.Process(ctx, 0)
		_ = err // Intentionally ignore error in benchmark

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := breaker.Process(ctx, i)
			if err == nil {
				b.Fatal("expected error")
			}
		}
	})

	b.Run("State Transitions", func(b *testing.B) {
		var shouldFail atomic.Bool
		processor := Apply("bench", func(_ context.Context, n int) (int, error) {
			if shouldFail.Load() {
				return 0, errors.New("failure")
			}
			return n, nil
		})
		breaker := NewCircuitBreaker("bench-breaker", processor, 2, 50*time.Millisecond)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Alternate between success and failure patterns
			shouldFail.Store(i%10 < 3)
			_, err := breaker.Process(ctx, i)
			_ = err // Intentionally ignore error in benchmark

			// Occasionally wait for reset timeout
			if i%100 == 0 {
				time.Sleep(60 * time.Millisecond)
			}
		}
	})
}
