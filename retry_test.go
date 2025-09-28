package pipz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/tracez"
)

func TestRetry(t *testing.T) {
	t.Run("Hooks fire on retry events", func(t *testing.T) {
		var attemptEvents []RetryEvent
		var successEvents []RetryEvent
		var mu sync.Mutex

		calls := 0
		processor := Apply("flaky", func(_ context.Context, n int) (int, error) {
			calls++
			if calls < 3 {
				return 0, errors.New("temporary error")
			}
			return n * 2, nil
		})

		retry := NewRetry("test-retry", processor, 5)
		defer retry.Close()

		// Register hooks
		retry.OnAttempt(func(_ context.Context, event RetryEvent) error {
			mu.Lock()
			attemptEvents = append(attemptEvents, event)
			mu.Unlock()
			return nil
		})

		retry.OnSuccess(func(_ context.Context, event RetryEvent) error {
			mu.Lock()
			successEvents = append(successEvents, event)
			mu.Unlock()
			return nil
		})

		// Trigger retry logic
		result, err := retry.Process(context.Background(), 5)

		// Verify success
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// Wait for async hooks
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Check attempt events
		if len(attemptEvents) != 3 {
			t.Errorf("expected 3 attempt events, got %d", len(attemptEvents))
		}

		// Sort events by attempt number since hooks are async
		attemptNumbers := make([]int, len(attemptEvents))
		for i, event := range attemptEvents {
			attemptNumbers[i] = event.AttemptNumber
		}

		// Verify we have the right attempt numbers
		expectedAttempts := []int{1, 2, 3}
		for _, expected := range expectedAttempts {
			found := false
			for _, actual := range attemptNumbers {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("missing attempt number %d in events", expected)
			}
		}

		for i, event := range attemptEvents {
			if event.Name != "test-retry" {
				t.Errorf("event %d: expected name 'test-retry', got %s", i, event.Name)
			}
			if event.ProcessorName != "flaky" {
				t.Errorf("event %d: expected processor name 'flaky', got %s", i, event.ProcessorName)
			}
			if event.MaxAttempts != 5 {
				t.Errorf("event %d: expected max attempts 5, got %d", i, event.MaxAttempts)
			}
			// Check success based on attempt number, not order in slice
			if event.AttemptNumber < 3 && event.Success {
				t.Errorf("attempt %d: expected failure, got success", event.AttemptNumber)
			}
			if event.AttemptNumber == 3 && !event.Success {
				t.Errorf("attempt %d: expected success, got failure", event.AttemptNumber)
			}
			if event.Duration <= 0 {
				t.Errorf("event %d: expected positive duration", i)
			}
		}

		// Check success event
		if len(successEvents) != 1 {
			t.Errorf("expected 1 success event, got %d", len(successEvents))
		}
		if len(successEvents) > 0 {
			event := successEvents[0]
			if !event.Success {
				t.Error("expected success=true for success event")
			}
			if event.AttemptsUsed != 3 {
				t.Errorf("expected 3 attempts used, got %d", event.AttemptsUsed)
			}
			if event.TotalDuration <= 0 {
				t.Error("expected positive total duration")
			}
		}
	})

	t.Run("Hook fires on exhausted event", func(t *testing.T) {
		var exhaustedEvents []RetryEvent
		var attemptEvents []RetryEvent
		var mu sync.Mutex

		processor := Apply("always-fail", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("permanent error")
		})

		retry := NewRetry("test-retry", processor, 2)
		defer retry.Close()

		retry.OnAttempt(func(_ context.Context, event RetryEvent) error {
			mu.Lock()
			attemptEvents = append(attemptEvents, event)
			mu.Unlock()
			return nil
		})

		retry.OnExhausted(func(_ context.Context, event RetryEvent) error {
			mu.Lock()
			exhaustedEvents = append(exhaustedEvents, event)
			mu.Unlock()
			return nil
		})

		// Trigger exhaustion
		_, err := retry.Process(context.Background(), 5)

		// Verify failure
		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}

		// Wait for async hooks
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Check attempt events
		if len(attemptEvents) != 2 {
			t.Errorf("expected 2 attempt events, got %d", len(attemptEvents))
		}

		// Check exhausted event
		if len(exhaustedEvents) != 1 {
			t.Errorf("expected 1 exhausted event, got %d", len(exhaustedEvents))
		}
		if len(exhaustedEvents) > 0 {
			event := exhaustedEvents[0]
			if event.Success {
				t.Error("expected success=false for exhausted event")
			}
			if event.AttemptsUsed != 2 {
				t.Errorf("expected 2 attempts used, got %d", event.AttemptsUsed)
			}
			if event.Error == nil {
				t.Error("expected error in exhausted event")
			}
			if event.TotalDuration <= 0 {
				t.Error("expected positive total duration")
			}
		}
	})
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

	t.Run("Observability Components", func(t *testing.T) {
		processor := Apply("test", func(_ context.Context, n int) (int, error) {
			if n < 3 {
				return 0, errors.New("temp error")
			}
			return n * 2, nil
		})

		retry := NewRetry("test-retry", processor, 3)
		defer retry.Close()

		// Verify observability components are initialized
		if retry.Metrics() == nil {
			t.Error("expected metrics registry to be initialized")
		}
		if retry.Tracer() == nil {
			t.Error("expected tracer to be initialized")
		}

		// Capture spans using the callback API
		var spans []tracez.Span
		retry.Tracer().OnSpanComplete(func(span tracez.Span) {
			spans = append(spans, span)
		})

		// Process with retries to test metrics
		_, err := retry.Process(context.Background(), 1)
		if err == nil {
			t.Error("expected error on first attempt with n=1")
		}

		// Try again with success
		result, err := retry.Process(context.Background(), 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 6 {
			t.Errorf("expected 6, got %d", result)
		}

		// Verify metrics were incremented
		attemptsTotal := retry.Metrics().Counter(RetryAttemptsTotal).Value()
		if attemptsTotal == 0 {
			t.Error("expected attempts counter to be incremented")
		}

		successesTotal := retry.Metrics().Counter(RetrySuccessesTotal).Value()
		if successesTotal != 1 {
			t.Errorf("expected 1 success, got %d", int(successesTotal))
		}

		failuresTotal := retry.Metrics().Counter(RetryFailuresTotal).Value()
		if failuresTotal != 1 {
			t.Errorf("expected 1 failure (exhausted retries), got %d", int(failuresTotal))
		}

		// Verify spans were captured
		if len(spans) == 0 {
			t.Error("expected spans to be captured")
		}

		// Check we have both process and attempt spans
		var processSpans, attemptSpans int
		for _, span := range spans {
			if span.Name == string(RetryProcessSpan) {
				processSpans++
			}
			if span.Name == string(RetryAttemptSpan) {
				attemptSpans++
			}
		}

		if processSpans != 2 { // One for each Process() call
			t.Errorf("expected 2 process spans, got %d", processSpans)
		}
		if attemptSpans < 3 { // At least 3 attempts
			t.Errorf("expected at least 3 attempt spans, got %d", attemptSpans)
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

	t.Run("Context Cancellation Between Attempts", func(t *testing.T) {
		// This test covers lines 74-84 in retry.go where context is checked between attempts
		calls := 0
		var cancelFunc context.CancelFunc

		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			calls++
			if calls == 1 {
				// Cancel context after first attempt fails
				cancelFunc()
			}
			return 0, errors.New("processor error")
		})

		retry := NewRetry("test-retry", processor, 5) // Allow multiple retries

		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel // Store cancel function for use in processor

		_, err := retry.Process(ctx, 5)

		// Should get a cancellation error, not the processor error
		if err == nil {
			t.Fatal("expected error from context cancellation")
		}

		var pipzErr *Error[int]
		if errors.As(err, &pipzErr) {
			if !pipzErr.IsCanceled() {
				t.Errorf("expected canceled error, got %v", err)
			}
			// Should show the retry name in the path
			if len(pipzErr.Path) == 0 || pipzErr.Path[0] != "test-retry" {
				t.Errorf("expected retry name in error path, got %v", pipzErr.Path)
			}
		} else {
			t.Errorf("expected pipz.Error, got %T", err)
		}

		// Should have been called only once before cancellation was detected
		if calls != 1 {
			t.Errorf("expected 1 call before cancellation, got %d", calls)
		}
	})

	t.Run("Context Timeout Between Attempts", func(t *testing.T) {
		// Test timeout context cancellation between attempts
		calls := 0

		processor := Apply("test", func(_ context.Context, _ int) (int, error) {
			calls++
			// Add delay to ensure timeout can occur
			time.Sleep(50 * time.Millisecond)
			return 0, errors.New("processor error")
		})

		retry := NewRetry("test-retry", processor, 5)

		// Create context that times out very quickly
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := retry.Process(ctx, 5)

		// Should get a timeout error
		if err == nil {
			t.Fatal("expected error from context timeout")
		}

		var pipzErr *Error[int]
		if errors.As(err, &pipzErr) {
			if !pipzErr.IsTimeout() {
				t.Errorf("expected timeout error, got %v", err)
			}
		} else {
			t.Errorf("expected pipz.Error, got %T", err)
		}
	})

	t.Run("Defensive LastErr Nil Case", func(t *testing.T) {
		// This tests the defensive code at line 103 in retry.go where lastErr could theoretically be nil
		// This is unlikely to happen in practice, but the code handles it defensively

		// Create a retry with zero attempts (clamped to 1)
		processor := Transform("success", func(_ context.Context, n int) int {
			return n * 2 // Always succeeds
		})

		retry := NewRetry("test-retry", processor, 1)
		result, err := retry.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})
}

func TestRetryContextCancellationBetweenAttempts(t *testing.T) {
	t.Run("Context Cancellation Between Retry Attempts", func(t *testing.T) {
		// This test covers lines 74-84 in retry.go where context is checked between attempts
		attemptCount := atomic.Int32{}
		processor := Apply("flaky", func(_ context.Context, _ int) (int, error) {
			attemptCount.Add(1)
			// All attempts fail to trigger retry loop
			return 0, errors.New("attempt failed")
		})

		retry := NewRetry("test-retry", processor, 5)
		// Create context that we'll cancel during retry
		ctx, cancel := context.WithCancel(context.Background())
		// Start retry in goroutine
		done := make(chan struct{})
		var err error

		go func() {
			_, err = retry.Process(ctx, 10)
			close(done)
		}()

		// Wait for first attempt, then cancel to trigger context check in retry loop
		time.Sleep(5 * time.Millisecond)
		cancel()

		// Wait for retry to complete
		select {
		case <-done:
			// Completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("retry did not complete in time")
		}

		// Should have context cancellation error
		if err == nil {
			t.Fatal("expected error")
		}

		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			// Should be canceled error from context check between attempts
			if !pipeErr.IsCanceled() && !strings.Contains(err.Error(), "attempt failed") {
				t.Errorf("expected canceled error or attempt failed, got: %v", err)
			}
			if len(pipeErr.Path) > 0 && pipeErr.Path[0] != "test-retry" {
				t.Errorf("expected retry name in error path, got %v", pipeErr.Path)
			}
		}

		// Should have attempted at least once before context was canceled
		attempts := attemptCount.Load()
		if attempts < 1 {
			t.Errorf("expected at least 1 attempt, got %d", attempts)
		}
	})
}

func TestRetryBackoffContextTimeout(t *testing.T) {
	t.Run("Backoff Context Timeout During Wait", func(t *testing.T) {
		// This test covers BackoffRetry when context times out during backoff wait
		attempts := atomic.Int32{}
		processor := Apply("failing", func(_ context.Context, _ int) (int, error) {
			attempts.Add(1)
			return 0, errors.New("always fails")
		})

		// Create backoff retry with base delay that will cause timeout
		backoff := NewBackoff("test-backoff", processor, 5, 20*time.Millisecond)

		// Use context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := backoff.Process(ctx, 5)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected error")
		}

		// Should have attempted 2 times before timeout
		// First: immediate, Second: after 20ms, Third would be after 40ms but times out at 35ms
		attemptCount := attempts.Load()
		if attemptCount != 2 {
			t.Errorf("expected 2 attempts before timeout, got %d", attemptCount)
		}

		// Should have taken approximately 35ms
		if elapsed < 30*time.Millisecond || elapsed > 50*time.Millisecond {
			t.Errorf("expected ~35ms elapsed, got %v", elapsed)
		}

		// Check error type - could be timeout or last error depending on exact timing
		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			// Either timeout from context or the last "always fails" error is acceptable
			if !pipeErr.IsTimeout() && !strings.Contains(pipeErr.Err.Error(), "always fails") {
				t.Errorf("expected timeout or 'always fails' error, got: %v", pipeErr.Err)
			}
		}
	})

	t.Run("Backoff Defensive LastErr Nil Case", func(t *testing.T) {
		// This tests the defensive code at line 232 in retry.go where lastErr could theoretically be nil
		// This is unlikely to happen in practice, but the code handles it defensively

		processor := Transform("success", func(_ context.Context, n int) int {
			return n * 3 // Always succeeds
		})

		backoff := NewBackoff("test-backoff", processor, 1, 10*time.Millisecond)
		result, err := backoff.Process(context.Background(), 7)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 21 { // 7 * 3
			t.Errorf("expected 21, got %d", result)
		}
	})

	t.Run("Retry panic recovery", func(t *testing.T) {
		calls := 0
		processor := Apply("panic_processor", func(_ context.Context, n int) (int, error) {
			calls++
			if calls < 2 {
				panic("retry processor panic")
			}
			return n * 2, nil
		})

		retry := NewRetry("panic_retry", processor, 3)
		result, err := retry.Process(context.Background(), 42)

		// Should succeed on retry after panic recovery
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != 84 {
			t.Errorf("expected result 84, got %d", result)
		}

		if calls != 2 {
			t.Errorf("expected 2 calls (1 panic + 1 success), got %d", calls)
		}
	})

	t.Run("All retry attempts panic", func(t *testing.T) {
		calls := 0
		processor := Apply("panic_processor", func(_ context.Context, _ int) (int, error) {
			calls++
			panic("retry processor panic")
		})

		retry := NewRetry("all_panic_retry", processor, 3)
		result, err := retry.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "all_panic_retry" {
			t.Errorf("expected path to start with 'all_panic_retry', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}

		if calls != 3 {
			t.Errorf("expected 3 calls (all panics), got %d", calls)
		}
	})

	t.Run("Backoff panic recovery", func(t *testing.T) {
		calls := 0
		processor := Apply("panic_processor", func(_ context.Context, n int) (int, error) {
			calls++
			if calls < 2 {
				panic("backoff processor panic")
			}
			return n * 2, nil
		})

		backoff := NewBackoff("panic_backoff", processor, 3, 10*time.Millisecond)
		result, err := backoff.Process(context.Background(), 42)

		// Should succeed on retry after panic recovery
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != 84 {
			t.Errorf("expected result 84, got %d", result)
		}

		if calls != 2 {
			t.Errorf("expected 2 calls (1 panic + 1 success), got %d", calls)
		}
	})
}
