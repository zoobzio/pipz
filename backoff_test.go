package pipz

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
	"github.com/zoobzio/clockz"
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

	t.Run("Emits backoff.waiting hook", func(t *testing.T) {
		var mu sync.Mutex
		var waitingEvents []struct {
			name        string
			attempt     int
			maxAttempts int
			delay       float64
			nextDelay   float64
		}

		listener := capitan.Hook(SignalBackoffWaiting, func(_ context.Context, e *capitan.Event) {
			mu.Lock()
			defer mu.Unlock()

			name, _ := FieldName.From(e)
			attempt, _ := FieldAttempt.From(e)
			maxAttempts, _ := FieldMaxAttempts.From(e)
			delay, _ := FieldDelay.From(e)
			nextDelay, _ := FieldNextDelay.From(e)

			waitingEvents = append(waitingEvents, struct {
				name        string
				attempt     int
				maxAttempts int
				delay       float64
				nextDelay   float64
			}{name, attempt, maxAttempts, delay, nextDelay})
		})
		defer listener.Close()

		calls := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			calls++
			if calls < 3 {
				return 0, errors.New("temporary error")
			}
			return n * 2, nil
		})

		backoff := NewBackoff("backoff-hook-test", processor, 3, 100*time.Millisecond)
		result, err := backoff.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// Wait for async hook processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Should emit 2 waiting signals (after 1st and 2nd failed attempts)
		if len(waitingEvents) != 2 {
			t.Fatalf("expected 2 backoff.waiting signals, got %d", len(waitingEvents))
		}

		// Check first backoff event
		if waitingEvents[0].name != "backoff-hook-test" {
			t.Errorf("expected name 'backoff-hook-test', got %q", waitingEvents[0].name)
		}
		if waitingEvents[0].attempt != 1 {
			t.Errorf("expected attempt 1, got %d", waitingEvents[0].attempt)
		}
		if waitingEvents[0].maxAttempts != 3 {
			t.Errorf("expected maxAttempts 3, got %d", waitingEvents[0].maxAttempts)
		}
		if waitingEvents[0].delay != 0.1 {
			t.Errorf("expected delay 0.1s, got %.3f", waitingEvents[0].delay)
		}
		if waitingEvents[0].nextDelay != 0.2 {
			t.Errorf("expected nextDelay 0.2s, got %.3f", waitingEvents[0].nextDelay)
		}

		// Check second backoff event (exponential increase)
		if waitingEvents[1].attempt != 2 {
			t.Errorf("expected attempt 2, got %d", waitingEvents[1].attempt)
		}
		if waitingEvents[1].delay != 0.2 {
			t.Errorf("expected delay 0.2s, got %.3f", waitingEvents[1].delay)
		}
		if waitingEvents[1].nextDelay != 0.4 {
			t.Errorf("expected nextDelay 0.4s, got %.3f", waitingEvents[1].nextDelay)
		}
	})

	t.Run("No hook on final attempt", func(t *testing.T) {
		var mu sync.Mutex
		var eventCount int

		listener := capitan.Hook(SignalBackoffWaiting, func(_ context.Context, _ *capitan.Event) {
			mu.Lock()
			defer mu.Unlock()
			eventCount++
		})
		defer listener.Close()

		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("always fails")
		})

		backoff := NewBackoff("final-attempt-test", processor, 3, 10*time.Millisecond)
		_, err := backoff.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error")
		}

		// Wait for async hook processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Should emit 2 signals (before attempt 2 and 3, but not after final attempt)
		if eventCount != 2 {
			t.Errorf("expected 2 backoff.waiting signals, got %d", eventCount)
		}
	})
}
