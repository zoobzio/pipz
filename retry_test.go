package pipz

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	t.Run("Success On First Try", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			calls++
			return n * 2, nil
		})

		retry := NewRetry("test-retry", processor, 3)
		result, err := retry.Process(context.Background(), 5)

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

	t.Run("Success After Retries", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			calls++
			if calls < 3 {
				return 0, errors.New("temporary error")
			}
			return n * 2, nil
		})

		retry := NewRetry("test-retry", processor, 3)
		result, err := retry.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if calls != 3 {
			t.Errorf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("Exhausts Retries", func(t *testing.T) {
		calls := 0
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			calls++
			return 0, errors.New("permanent error")
		})

		retry := NewRetry("test-retry", processor, 3)
		_, err := retry.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		// The error should contain the original error message
		if !strings.Contains(err.Error(), "permanent error") {
			t.Errorf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Errorf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("error")
		})

		retry := NewRetry("test-retry", processor, 5)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := retry.Process(ctx, 5)
		if err == nil {
			t.Error("expected cancellation error")
		}

		var pipzErr *Error[int]
		if errors.As(err, &pipzErr) {
			if !pipzErr.IsCanceled() {
				t.Errorf("expected canceled error, got %v", err)
			}
		} else {
			t.Errorf("expected pipz.Error, got %T", err)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		retry := NewRetry("test", processor, 3)

		if retry.GetMaxAttempts() != 3 {
			t.Errorf("expected 3, got %d", retry.GetMaxAttempts())
		}

		retry.SetMaxAttempts(5)
		if retry.GetMaxAttempts() != 5 {
			t.Errorf("expected 5, got %d", retry.GetMaxAttempts())
		}

		// Test minimum attempts
		retry.SetMaxAttempts(0)
		if retry.GetMaxAttempts() != 1 {
			t.Errorf("expected 1 (minimum), got %d", retry.GetMaxAttempts())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Transform("noop", func(_ context.Context, n int) int { return n })
		retry := NewRetry("my-retry", processor, 3)
		if retry.Name() != "my-retry" {
			t.Errorf("expected 'my-retry', got %q", retry.Name())
		}
	})

	t.Run("Constructor With Invalid MaxAttempts", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })

		// Test with 0 attempts - should be clamped to 1
		retry := NewRetry("test", processor, 0)
		if retry.GetMaxAttempts() != 1 {
			t.Errorf("expected maxAttempts to be clamped to 1, got %d", retry.GetMaxAttempts())
		}

		// Test with negative attempts - should be clamped to 1
		retry = NewRetry("test", processor, -5)
		if retry.GetMaxAttempts() != 1 {
			t.Errorf("expected maxAttempts to be clamped to 1, got %d", retry.GetMaxAttempts())
		}
	})
}

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

	t.Run("Backoff Timing", func(t *testing.T) {
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
}
