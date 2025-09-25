package pipz

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/tracez"
)

func TestTimeout(t *testing.T) {
	t.Run("Completes Within Timeout", func(t *testing.T) {
		processor := Apply("fast", func(_ context.Context, n int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return n * 2, nil
		})

		timeout := NewTimeout("test-timeout", processor, 100*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Exceeds Timeout", func(t *testing.T) {
		processor := Apply("slow", func(_ context.Context, n int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return n * 2, nil
		})

		timeout := NewTimeout("test-timeout", processor, 50*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)

		if err == nil {
			t.Error("expected timeout error")
		}
		var pipzErr *Error[int]
		if errors.As(err, &pipzErr) {
			if !pipzErr.IsTimeout() {
				t.Errorf("expected timeout error, got %v", err)
			}
		} else {
			t.Errorf("expected pipz.Error, got %T", err)
		}
		if result != 5 {
			t.Errorf("expected original input 5, got %d", result)
		}
	})

	t.Run("Respects Context", func(t *testing.T) {
		processor := Apply("slow", func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		})

		timeout := NewTimeout("test-timeout", processor, 50*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)

		if err == nil {
			t.Error("expected timeout error")
		}
		var pipzErr *Error[int]
		if errors.As(err, &pipzErr) {
			if !pipzErr.IsTimeout() {
				t.Errorf("expected timeout error, got %v", err)
			}
		} else {
			t.Errorf("expected pipz.Error, got %T", err)
		}
		// Result should be 5 (original input) since timeout returns input data
		if result != 5 {
			t.Errorf("expected original input 5, got %d", result)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		timeout := NewTimeout("test", processor, 100*time.Millisecond)

		if timeout.GetDuration() != 100*time.Millisecond {
			t.Errorf("expected 100ms, got %v", timeout.GetDuration())
		}

		timeout.SetDuration(200 * time.Millisecond)
		if timeout.GetDuration() != 200*time.Millisecond {
			t.Errorf("expected 200ms, got %v", timeout.GetDuration())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Transform("noop", func(_ context.Context, n int) int { return n })
		timeout := NewTimeout("my-timeout", processor, time.Second)
		if timeout.Name() != "my-timeout" {
			t.Errorf("expected 'my-timeout', got %q", timeout.Name())
		}
	})

	t.Run("Processor Error Within Timeout", func(t *testing.T) {
		// Processor that completes within timeout but returns an error
		processor := Apply("error-proc", func(_ context.Context, _ int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return 0, errors.New("processor failed")
		})

		timeout := NewTimeout("error-timeout", processor, 100*time.Millisecond)
		_, err := timeout.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error from processor")
		}

		// Check error path includes timeout name
		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			expectedPath := []Name{"error-timeout", "error-proc"}
			if len(pipeErr.Path) != 2 || pipeErr.Path[0] != "error-timeout" || pipeErr.Path[1] != "error-proc" {
				t.Errorf("expected error path %v, got %v", expectedPath, pipeErr.Path)
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[int]")
		}
	})

	t.Run("Non-Pipeline Error Wrapping", func(t *testing.T) {
		// Test that raw errors (not pipeline errors) get wrapped correctly
		processor := Apply("raw-error", func(_ context.Context, _ int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			// Return a raw error, not a pipeline error
			return 42, errors.New("raw error message")
		})

		timeout := NewTimeout("wrapper-timeout", processor, 100*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error from processor")
		}

		// Should get the zero result from the processor (Apply returns zero on error)
		if result != 0 {
			t.Errorf("expected result 0, got %d", result)
		}

		// Check that the pipeline error gets path extension
		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			expectedPath := []Name{"wrapper-timeout", "raw-error"}
			if len(pipeErr.Path) != 2 || pipeErr.Path[0] != "wrapper-timeout" || pipeErr.Path[1] != "raw-error" {
				t.Errorf("expected error path %v, got %v", expectedPath, pipeErr.Path)
			}
			if pipeErr.Err.Error() != "raw error message" {
				t.Errorf("expected wrapped raw error message, got %v", pipeErr.Err)
			}
		} else {
			t.Error("expected error to be wrapped as *pipz.Error[int]")
		}
	})

	t.Run("Context Cancellation vs Timeout", func(t *testing.T) {
		// Test context cancellation (not timeout) to cover the Canceled flag
		processor := Apply("slow", func(ctx context.Context, _ int) (int, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return 0, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		})

		timeout := NewTimeout("cancel-timeout", processor, 300*time.Millisecond) // Longer than processor delay

		// Create context that will be canceled manually
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after a short delay (before both processor and timeout complete)
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		result, err := timeout.Process(ctx, 5)

		if err == nil {
			t.Fatal("expected cancellation error")
		}

		// Should return original input data
		if result != 5 {
			t.Errorf("expected original input 5, got %d", result)
		}

		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			if !pipeErr.IsCanceled() {
				t.Errorf("expected canceled error, got %v", err)
			}
			if pipeErr.IsTimeout() {
				t.Error("should not be marked as timeout when context was explicitly canceled")
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[int]")
		}
	})

	t.Run("Timeout panic recovery", func(t *testing.T) {
		panicProcessor := Apply("panic_processor", func(_ context.Context, _ int) (int, error) {
			panic("timeout processor panic")
		})

		timeout := NewTimeout("panic_timeout", panicProcessor, 100*time.Millisecond)
		result, err := timeout.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "panic_timeout" {
			t.Errorf("expected path to start with 'panic_timeout', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	t.Run("Deterministic Timeout With Fake Clock", func(t *testing.T) {
		clock := clockz.NewFakeClock()

		// Processor that waits for context cancellation
		processor := Apply("wait-for-timeout", func(ctx context.Context, n int) (int, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(1 * time.Second):
				// This should never happen with fake clock
				return n * 2, nil
			}
		})

		timeout := NewTimeout("fake-timeout", processor, 100*time.Millisecond).WithClock(clock)

		// Run timeout processing in goroutine
		done := make(chan struct{})
		var result int
		var err error
		go func() {
			defer close(done)
			result, err = timeout.Process(context.Background(), 42)
		}()

		// Allow the goroutine to start
		time.Sleep(10 * time.Millisecond)

		// Advance the fake clock past the timeout duration
		clock.Advance(100 * time.Millisecond)
		clock.BlockUntilReady()

		// Give time for the timeout to be processed
		time.Sleep(10 * time.Millisecond)

		// Wait for completion
		<-done

		// Verify timeout occurred
		if err == nil {
			t.Fatal("expected timeout error")
		}

		var pipeErr *Error[int]
		if !errors.As(err, &pipeErr) {
			t.Fatalf("expected *Error[int], got %T", err)
		}

		if !pipeErr.IsTimeout() {
			t.Errorf("expected timeout error, got: %v", err)
		}

		// Timeout should return original input
		if result != 42 {
			t.Errorf("expected original input 42, got %d", result)
		}
	})

	t.Run("WithClock Method", func(t *testing.T) {
		processor := Transform("test", func(_ context.Context, n int) int { return n })
		timeout := NewTimeout("test", processor, 100*time.Millisecond)

		clock := clockz.NewFakeClock()
		timeout2 := timeout.WithClock(clock)

		// Should return same instance for chaining
		if timeout2 != timeout {
			t.Error("WithClock should return same instance for chaining")
		}
	})

	t.Run("Observability", func(t *testing.T) {
		t.Run("Metrics and Spans", func(t *testing.T) {
			processor := Apply("slow", func(ctx context.Context, n int) (int, error) {
				select {
				case <-time.After(50 * time.Millisecond):
					return n * 2, nil
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			})

			timeout := NewTimeout("test-timeout", processor, 100*time.Millisecond)
			defer timeout.Close()

			// Verify observability components are initialized
			if timeout.Metrics() == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if timeout.Tracer() == nil {
				t.Error("expected tracer to be initialized")
			}

			// Capture spans using the callback API
			var spans []tracez.Span
			timeout.Tracer().OnSpanComplete(func(span tracez.Span) {
				spans = append(spans, span)
			})

			// Process item that succeeds
			result, err := timeout.Process(context.Background(), 5)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != 10 {
				t.Errorf("expected 10, got %d", result)
			}

			// Verify metrics
			processedTotal := timeout.Metrics().Counter(TimeoutProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := timeout.Metrics().Counter(TimeoutSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			// Check duration was recorded
			duration := timeout.Metrics().Gauge(TimeoutDurationMs).Value()
			if duration < 50 { // Should be at least 50ms
				t.Errorf("expected duration >= 50ms, got %f", duration)
			}

			// Verify spans were captured
			if len(spans) < 1 {
				t.Errorf("expected at least 1 span, got %d", len(spans))
			}
		})

		t.Run("Timeout Metrics", func(t *testing.T) {
			processor := Apply("very-slow", func(ctx context.Context, n int) (int, error) {
				select {
				case <-time.After(100 * time.Millisecond):
					return n * 2, nil
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			})

			timeout := NewTimeout("test-timeout", processor, 50*time.Millisecond)
			defer timeout.Close()

			_, err := timeout.Process(context.Background(), 5)
			if err == nil {
				t.Fatal("expected timeout error")
			}

			// Check timeout metrics
			timeoutsTotal := timeout.Metrics().Counter(TimeoutTimeoutsTotal).Value()
			if timeoutsTotal != 1 {
				t.Errorf("expected 1 timeout, got %f", timeoutsTotal)
			}

			processedTotal := timeout.Metrics().Counter(TimeoutProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := timeout.Metrics().Counter(TimeoutSuccessesTotal).Value()
			if successesTotal != 0 {
				t.Errorf("expected 0 successes, got %f", successesTotal)
			}
		})

		t.Run("Hooks fire on timeout events", func(t *testing.T) {
			// Test actual timeout
			processor := Apply("slow", func(_ context.Context, n int) (int, error) {
				time.Sleep(50 * time.Millisecond)
				return n * 2, nil
			})

			timeout := NewTimeout("test-timeout", processor, 10*time.Millisecond)
			defer timeout.Close()

			var timeoutEvents []TimeoutEvent
			var mu sync.Mutex

			timeout.OnTimeout(func(_ context.Context, event TimeoutEvent) error {
				mu.Lock()
				timeoutEvents = append(timeoutEvents, event)
				mu.Unlock()
				return nil
			})

			// Process should timeout
			_, err := timeout.Process(context.Background(), 10)
			if err == nil {
				t.Error("expected timeout error")
			}

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			timeoutCount := len(timeoutEvents)
			var timedOutFlag bool
			var duration time.Duration
			if len(timeoutEvents) > 0 {
				event := timeoutEvents[0]
				timedOutFlag = event.TimedOut
				duration = event.Duration
			}
			mu.Unlock()

			if timeoutCount != 1 {
				t.Errorf("expected 1 timeout event, got %d", timeoutCount)
			}

			if timeoutCount > 0 {
				if !timedOutFlag {
					t.Error("expected TimedOut=true")
				}
				if duration != 10*time.Millisecond {
					t.Errorf("expected duration 10ms, got %v", duration)
				}
			}
		})

		t.Run("Near timeout hook fires for slow operations", func(t *testing.T) {
			// Test operation that completes but is close to timeout
			processor := Apply("slow", func(_ context.Context, n int) (int, error) {
				time.Sleep(18 * time.Millisecond) // 90% of 20ms timeout
				return n * 2, nil
			})

			timeout := NewTimeout("test-near", processor, 20*time.Millisecond)
			defer timeout.Close()

			var nearTimeoutEvents []TimeoutEvent
			var mu sync.Mutex

			timeout.OnNearTimeout(func(_ context.Context, event TimeoutEvent) error {
				mu.Lock()
				nearTimeoutEvents = append(nearTimeoutEvents, event)
				mu.Unlock()
				return nil
			})

			// Process should succeed but trigger near timeout
			result, err := timeout.Process(context.Background(), 10)
			if err != nil {
				t.Errorf("expected success, got error: %v", err)
			}
			if result != 20 {
				t.Errorf("expected result 20, got %d", result)
			}

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			nearCount := len(nearTimeoutEvents)
			var nearFlag bool
			var timedOutFlag bool
			var percentUsed float64
			if len(nearTimeoutEvents) > 0 {
				event := nearTimeoutEvents[0]
				nearFlag = event.NearTimeout
				timedOutFlag = event.TimedOut
				percentUsed = event.PercentUsed
			}
			mu.Unlock()

			if nearCount != 1 {
				t.Errorf("expected 1 near timeout event, got %d", nearCount)
			}

			if nearCount > 0 {
				if !nearFlag {
					t.Error("expected NearTimeout=true")
				}
				if timedOutFlag {
					t.Error("expected TimedOut=false")
				}
				if percentUsed <= 80 {
					t.Errorf("expected percent used >80, got %f", percentUsed)
				}
			}
		})
	})
}
