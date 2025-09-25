package pipz

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/tracez"
)

func TestBackoff(t *testing.T) {
	t.Run("Success On First Try", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			calls++
			return n * 2, nil
		})

		backoff := NewBackoff("test-backoff", processor, 3, 10*time.Millisecond)
		result, err := backoff.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if calls != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}
	})

	t.Run("Backoff Timing With Clock", func(t *testing.T) {
		var calls int32
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			atomic.AddInt32(&calls, 1)
			if atomic.LoadInt32(&calls) < 3 {
				return 0, errors.New("temporary error")
			}
			return n * 2, nil
		})

		clock := clockz.NewFakeClock()
		backoff := NewBackoff("test-backoff", processor, 3, 50*time.Millisecond).WithClock(clock)

		// Run in goroutine so we can advance the clock
		done := make(chan struct{})
		var result int
		var err error
		go func() {
			result, err = backoff.Process(context.Background(), 5)
			close(done)
		}()

		// Allow goroutine to start
		time.Sleep(10 * time.Millisecond)

		// First retry delay: 50ms
		clock.Advance(50 * time.Millisecond)
		clock.BlockUntilReady()
		time.Sleep(10 * time.Millisecond) // Let goroutine process timer

		// Second retry delay: 100ms (exponential backoff)
		clock.Advance(100 * time.Millisecond)
		clock.BlockUntilReady()
		time.Sleep(10 * time.Millisecond) // Let goroutine process timer

		// Wait for completion with timeout
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("test timed out")
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if atomic.LoadInt32(&calls) != 3 {
			t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&calls))
		}
	})

	t.Run("Backoff Timing Without Clock", func(t *testing.T) {
		var calls int32
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			atomic.AddInt32(&calls, 1)
			if atomic.LoadInt32(&calls) < 3 {
				return 0, errors.New("temporary error")
			}
			return n * 2, nil
		})

		backoff := NewBackoff("test-backoff", processor, 3, 50*time.Millisecond)

		start := time.Now()
		result, err := backoff.Process(context.Background(), 5)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		// First retry: 50ms, Second retry: 100ms, Total: ~150ms
		if duration < 150*time.Millisecond {
			t.Errorf("expected at least 150ms, got %v", duration)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		backoff := NewBackoff("test", processor, 3, 100*time.Millisecond)

		if backoff.GetMaxAttempts() != 3 {
			t.Errorf("expected 3, got %d", backoff.GetMaxAttempts())
		}
		if backoff.GetBaseDelay() != 100*time.Millisecond {
			t.Errorf("expected 100ms, got %v", backoff.GetBaseDelay())
		}

		backoff.SetMaxAttempts(5)
		backoff.SetBaseDelay(200 * time.Millisecond)

		if backoff.GetMaxAttempts() != 5 {
			t.Errorf("expected 5, got %d", backoff.GetMaxAttempts())
		}
		if backoff.GetBaseDelay() != 200*time.Millisecond {
			t.Errorf("expected 200ms, got %v", backoff.GetBaseDelay())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Transform("noop", func(_ context.Context, n int) int { return n })
		backoff := NewBackoff("my-backoff", processor, 3, time.Second)
		if backoff.Name() != "my-backoff" {
			t.Errorf("expected 'my-backoff', got %q", backoff.Name())
		}
	})

	t.Run("Context Cancellation During Delay", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			calls++
			if calls == 1 {
				return 0, errors.New("first attempt error")
			}
			// Should not reach here if context is canceled during delay
			return n * 2, nil
		})

		backoff := NewBackoff("test", processor, 3, 100*time.Millisecond)

		// Cancel context after 50ms (during first backoff delay)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := backoff.Process(ctx, 5)

		// Should get context error
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}

		// Should have only tried once
		if calls != 1 {
			t.Errorf("expected 1 call before context cancellation, got %d", calls)
		}
	})

	t.Run("Constructor With Invalid MaxAttempts", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })

		// Test with 0 attempts - should be clamped to 1
		backoff := NewBackoff("test", processor, 0, time.Millisecond)
		if backoff.GetMaxAttempts() != 1 {
			t.Errorf("expected maxAttempts to be clamped to 1, got %d", backoff.GetMaxAttempts())
		}

		// Test with negative attempts - should be clamped to 1
		backoff = NewBackoff("test", processor, -5, time.Millisecond)
		if backoff.GetMaxAttempts() != 1 {
			t.Errorf("expected maxAttempts to be clamped to 1, got %d", backoff.GetMaxAttempts())
		}
	})

	t.Run("SetMaxAttempts With Invalid Value", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		backoff := NewBackoff("test", processor, 3, time.Millisecond)

		// Set to 0 - should be clamped to 1
		backoff.SetMaxAttempts(0)
		if backoff.GetMaxAttempts() != 1 {
			t.Errorf("expected maxAttempts to be clamped to 1, got %d", backoff.GetMaxAttempts())
		}

		// Set to negative - should be clamped to 1
		backoff.SetMaxAttempts(-10)
		if backoff.GetMaxAttempts() != 1 {
			t.Errorf("expected maxAttempts to be clamped to 1, got %d", backoff.GetMaxAttempts())
		}
	})

	t.Run("Exhausts Retries Returns Data", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			calls++
			return 0, errors.New("persistent error")
		})

		backoff := NewBackoff("test-backoff", processor, 2, 10*time.Millisecond)
		result, err := backoff.Process(context.Background(), 42)

		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		// Should return the original data value on failure
		if result != 42 {
			t.Errorf("expected original data 42 to be returned on failure, got %d", result)
		}
		if calls != 2 {
			t.Errorf("expected 2 calls, got %d", calls)
		}
	})

	t.Run("Observability", func(t *testing.T) {
		t.Run("Metrics and Spans", func(t *testing.T) {
			attempts := 0
			processor := Apply("test", func(_ context.Context, n int) (int, error) {
				attempts++
				if attempts < 3 {
					return 0, errors.New("temporary failure")
				}
				return n * 2, nil
			})

			backoff := NewBackoff("test-backoff", processor, 5, 10*time.Millisecond)
			defer backoff.Close()

			// Verify observability components are initialized
			if backoff.Metrics() == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if backoff.Tracer() == nil {
				t.Error("expected tracer to be initialized")
			}

			// Capture spans using the callback API
			var spans []tracez.Span
			backoff.Tracer().OnSpanComplete(func(span tracez.Span) {
				spans = append(spans, span)
			})

			// Process should succeed on third attempt
			result, err := backoff.Process(context.Background(), 5)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != 10 {
				t.Errorf("expected 10, got %d", result)
			}

			// Verify metrics
			attemptsTotal := backoff.Metrics().Counter(BackoffAttemptsTotal).Value()
			if attemptsTotal != 3 {
				t.Errorf("expected 3 total attempts, got %f", attemptsTotal)
			}

			successesTotal := backoff.Metrics().Counter(BackoffSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			// Check that delay was tracked
			delayTotal := backoff.Metrics().Counter(BackoffDelayTotal).Value()
			if delayTotal < 10 { // At least first delay of 10ms
				t.Errorf("expected delay total >= 10ms, got %f", delayTotal)
			}

			// Verify spans were captured
			if len(spans) < 4 { // 1 main span + 3 attempt spans
				t.Errorf("expected at least 4 spans, got %d", len(spans))
			}

			// Verify gauge is reset
			currentAttempt := backoff.Metrics().Gauge(BackoffAttemptCurrent).Value()
			if currentAttempt != 0 {
				t.Errorf("expected attempt gauge to be reset to 0, got %f", currentAttempt)
			}
		})

		t.Run("Failure Metrics", func(t *testing.T) {
			processor := Apply("always-fails", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("permanent error")
			})

			backoff := NewBackoff("test-backoff", processor, 2, 5*time.Millisecond)
			defer backoff.Close()

			_, err := backoff.Process(context.Background(), 5)
			if err == nil {
				t.Fatal("expected error")
			}

			// Check failure metrics
			failuresTotal := backoff.Metrics().Counter(BackoffFailuresTotal).Value()
			if failuresTotal != 1 {
				t.Errorf("expected 1 failure, got %f", failuresTotal)
			}

			attemptsTotal := backoff.Metrics().Counter(BackoffAttemptsTotal).Value()
			if attemptsTotal != 2 {
				t.Errorf("expected 2 attempts, got %f", attemptsTotal)
			}
		})

		t.Run("Hooks fire on retry events", func(t *testing.T) {
			var attemptCount atomic.Int32

			processor := Apply("test", func(_ context.Context, n int) (int, error) {
				count := attemptCount.Add(1)
				if count < 3 { // Fail first 2 attempts
					return 0, errors.New("temporary failure")
				}
				return n * 2, nil
			})

			backoff := NewBackoff("test-hooks", processor, 3, time.Millisecond)
			defer backoff.Close()

			var attemptEvents []BackoffEvent
			var successEvents []BackoffEvent
			var mu sync.Mutex

			// Register hooks
			backoff.OnAttempt(func(_ context.Context, event BackoffEvent) error {
				mu.Lock()
				attemptEvents = append(attemptEvents, event)
				mu.Unlock()
				return nil
			})

			backoff.OnSuccess(func(_ context.Context, event BackoffEvent) error {
				mu.Lock()
				successEvents = append(successEvents, event)
				mu.Unlock()
				return nil
			})

			// Process synchronously - real clock with very short delays
			result, err := backoff.Process(context.Background(), 10)
			if err != nil {
				t.Errorf("expected success, got error: %v", err)
			}
			if result != 20 {
				t.Errorf("expected result 20, got %d", result)
			}

			// Wait for async hooks
			time.Sleep(10 * time.Millisecond)

			// Should have 2 attempt events (before retry 2 and 3, not before first attempt)
			mu.Lock()
			attemptEventCount := len(attemptEvents)
			successCount := len(successEvents)
			var firstAttemptNum int
			var successFlag bool
			if len(attemptEvents) > 0 {
				firstAttemptNum = attemptEvents[0].AttemptNum
			}
			if len(successEvents) > 0 {
				successFlag = successEvents[0].Success
			}
			mu.Unlock()

			if attemptEventCount != 2 {
				t.Errorf("expected 2 attempt events, got %d", attemptEventCount)
			}

			// Verify attempt event details
			if attemptEventCount > 0 {
				if firstAttemptNum != 2 {
					t.Errorf("expected first attempt event for attempt 2, got %d", firstAttemptNum)
				}
			}

			// Should have 1 success event
			if successCount != 1 {
				t.Errorf("expected 1 success event, got %d", successCount)
			}

			if successCount > 0 && !successFlag {
				t.Error("expected success event to have Success=true")
			}
		})

		t.Run("Exhausted hook fires when all attempts fail", func(t *testing.T) {
			processor := Apply("test", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("persistent failure")
			})

			backoff := NewBackoff("test-exhausted", processor, 2, time.Millisecond)
			defer backoff.Close()

			var exhaustedEvents []BackoffEvent
			var mu sync.Mutex

			backoff.OnExhausted(func(_ context.Context, event BackoffEvent) error {
				mu.Lock()
				exhaustedEvents = append(exhaustedEvents, event)
				mu.Unlock()
				return nil
			})

			// Process synchronously
			_, err := backoff.Process(context.Background(), 10)
			if err == nil {
				t.Error("expected failure")
			}

			// Wait for async hooks
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			exhaustedCount := len(exhaustedEvents)
			var exhaustedFlag bool
			var attemptNum int
			var hasError bool
			if len(exhaustedEvents) > 0 {
				event := exhaustedEvents[0]
				exhaustedFlag = event.Exhausted
				attemptNum = event.AttemptNum
				hasError = event.Error != nil
			}
			mu.Unlock()

			if exhaustedCount != 1 {
				t.Errorf("expected 1 exhausted event, got %d", exhaustedCount)
			}

			if exhaustedCount > 0 {
				if !exhaustedFlag {
					t.Error("expected exhausted event to have Exhausted=true")
				}
				if attemptNum != 2 {
					t.Errorf("expected 2 attempts, got %d", attemptNum)
				}
				if !hasError {
					t.Error("expected error in exhausted event")
				}
			}
		})
	})
}
