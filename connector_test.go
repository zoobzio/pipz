package pipz

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSequential(t *testing.T) {
	t.Run("Empty Sequential", func(t *testing.T) {
		seq := Sequential[int]()
		result, err := seq.Process(context.Background(), 10)
		if err != nil {
			t.Errorf("empty sequential should not error: %v", err)
		}
		if result != 10 {
			t.Errorf("empty sequential should return input unchanged: got %d, want 10", result)
		}
	})

	t.Run("Single Chainable", func(t *testing.T) {
		double := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})

		seq := Sequential(double)
		result, err := seq.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Multiple Chainables", func(t *testing.T) {
		double := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		addTen := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 10, nil
		})

		seq := Sequential(double, addTen)
		result, err := seq.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 20 { // (5 * 2) + 10
			t.Errorf("expected 20, got %d", result)
		}
	})

	t.Run("Error Propagation", func(t *testing.T) {
		failing := ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("test error")
		})
		never := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			t.Error("this should never be called")
			return n, nil
		})

		seq := Sequential(failing, never)
		_, err := seq.Process(context.Background(), 5)
		if err == nil {
			t.Error("expected error from failing processor")
		}
	})

	t.Run("Using Processors", func(t *testing.T) {
		proc1 := Transform("double", func(_ context.Context, n int) int {
			return n * 2
		})
		proc2 := Apply("add_10", func(_ context.Context, n int) (int, error) {
			return n + 10, nil
		})

		seq := Sequential(proc1, proc2)
		result, err := seq.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}
	})

	t.Run("Using Pipeline", func(t *testing.T) {
		pipeline := NewPipeline[int]()
		pipeline.Register(
			Transform("double", func(_ context.Context, n int) int {
				return n * 2
			}),
			Transform("add_10", func(_ context.Context, n int) int {
				return n + 10
			}),
		)

		seq := Sequential(pipeline.Link())
		result, err := seq.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}
	})
}

func TestSwitch(t *testing.T) {
	t.Run("Basic Switch", func(t *testing.T) {
		condition := func(_ context.Context, n int) string {
			if n > 10 {
				return "large"
			}
			return "small"
		}

		routes := map[string]Chainable[int]{
			"large": ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n * 2, nil
			}),
			"small": ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n + 100, nil
			}),
		}

		sw := Switch(condition, routes)

		// Test small route
		result, err := sw.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 105 {
			t.Errorf("expected 105, got %d", result)
		}

		// Test large route
		result, err = sw.Process(context.Background(), 20)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 40 {
			t.Errorf("expected 40, got %d", result)
		}
	})

	t.Run("Default Route", func(t *testing.T) {
		condition := func(_ context.Context, s string) string {
			return s
		}

		routes := map[string]Chainable[string]{
			"a": ProcessorFunc[string](func(_ context.Context, s string) (string, error) {
				return s + "_a", nil
			}),
			"default": ProcessorFunc[string](func(_ context.Context, s string) (string, error) {
				return s + "_default", nil
			}),
		}

		sw := Switch(condition, routes)

		result, err := sw.Process(context.Background(), "unknown")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "unknown_default" {
			t.Errorf("expected 'unknown_default', got %s", result)
		}
	})

	t.Run("No Matching Route", func(t *testing.T) {
		condition := func(_ context.Context, _ string) string {
			return "missing"
		}

		routes := map[string]Chainable[string]{
			"a": ProcessorFunc[string](func(_ context.Context, s string) (string, error) {
				return s, nil
			}),
		}

		sw := Switch(condition, routes)

		_, err := sw.Process(context.Background(), "test")
		if err == nil {
			t.Error("expected error for missing route")
		}
	})
}

func TestFallback(t *testing.T) {
	t.Run("Primary Success", func(t *testing.T) {
		primary := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		fallback := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			t.Error("fallback should not be called")
			return n + 100, nil
		})

		fb := Fallback(primary, fallback)
		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Primary Failure", func(t *testing.T) {
		primary := ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 100, nil
		})

		fb := Fallback(primary, fallback)
		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 105 {
			t.Errorf("expected 105, got %d", result)
		}
	})
}

func TestConnectorComposition(t *testing.T) {
	t.Run("Sequential of Switch", func(t *testing.T) {
		// First stage: categorize
		categorize := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n, nil // pass through
		})

		// Second stage: switch based on value
		condition := func(_ context.Context, n int) string {
			if n > 10 {
				return "large"
			}
			return "small"
		}

		routes := map[string]Chainable[int]{
			"large": ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n * 2, nil
			}),
			"small": ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
				return n + 100, nil
			}),
		}

		// Compose
		workflow := Sequential(
			categorize,
			Switch(condition, routes),
		)

		result, err := workflow.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 105 {
			t.Errorf("expected 105, got %d", result)
		}
	})
}

func TestRetry(t *testing.T) {
	t.Run("Success on First Attempt", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n * 2, nil
		})

		retry := Retry(proc, 3)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Success on Second Attempt", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			if attempts < 2 {
				return 0, errors.New("temporary failure")
			}
			return n * 2, nil
		})

		retry := Retry(proc, 3)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("All Attempts Fail", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			attempts++
			return 0, errors.New("permanent failure")
		})

		retry := Retry(proc, 3)
		_, err := retry.Process(context.Background(), 5)
		if err == nil {
			t.Error("expected error after all attempts fail")
		}
		if !errors.Is(err, errors.New("permanent failure")) && !strings.Contains(err.Error(), "failed after 3 attempts") {
			t.Errorf("unexpected error: %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			attempts++
			if attempts == 1 {
				cancel() // Cancel after first attempt
			}
			return 0, errors.New("failure")
		})

		retry := Retry(proc, 5)
		_, err := retry.Process(ctx, 5)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if attempts > 2 {
			t.Errorf("expected at most 2 attempts before cancellation, got %d", attempts)
		}
	})

	t.Run("Zero Max Attempts", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n * 2, nil
		})

		// Should default to 1 attempt when maxAttempts is 0
		retry := Retry(proc, 0)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 1 {
			t.Errorf("expected exactly 1 attempt with maxAttempts=0, got %d", attempts)
		}
	})

	t.Run("Negative Max Attempts", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n * 2, nil
		})

		// Should default to 1 attempt when maxAttempts is negative
		retry := Retry(proc, -5)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 1 {
			t.Errorf("expected exactly 1 attempt with negative maxAttempts, got %d", attempts)
		}
	})
}

func TestRetryWithBackoff(t *testing.T) {
	t.Run("Exponential Backoff", func(t *testing.T) {
		attempts := 0
		start := time.Now()
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			if attempts < 3 {
				return 0, errors.New("temporary failure")
			}
			return n * 2, nil
		})

		retry := RetryWithBackoff(proc, 3, 10*time.Millisecond)
		result, err := retry.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		// Should have delays: 10ms + 20ms = 30ms minimum
		if elapsed < 30*time.Millisecond {
			t.Errorf("expected at least 30ms delay, got %v", elapsed)
		}
	})

	t.Run("All Attempts Fail With Backoff", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			attempts++
			return 0, errors.New("permanent failure")
		})

		retry := RetryWithBackoff(proc, 3, time.Microsecond)
		_, err := retry.Process(context.Background(), 5)
		if err == nil {
			t.Error("expected error after all attempts fail")
		}
		if !strings.Contains(err.Error(), "failed after 3 attempts with backoff") {
			t.Errorf("unexpected error message: %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Context Cancellation During Backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, _ int) (int, error) {
			attempts++
			if attempts == 1 {
				go func() {
					time.Sleep(5 * time.Millisecond)
					cancel()
				}()
			}
			return 0, errors.New("failure")
		})

		retry := RetryWithBackoff(proc, 5, 50*time.Millisecond)
		_, err := retry.Process(ctx, 5)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt before cancellation, got %d", attempts)
		}
	})

	t.Run("Zero Max Attempts With Backoff", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n * 2, nil
		})

		// Should default to 1 attempt when maxAttempts is 0
		retry := RetryWithBackoff(proc, 0, time.Millisecond)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 1 {
			t.Errorf("expected exactly 1 attempt with maxAttempts=0, got %d", attempts)
		}
	})

	t.Run("Negative Max Attempts With Backoff", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n * 2, nil
		})

		// Should default to 1 attempt when maxAttempts is negative
		retry := RetryWithBackoff(proc, -3, time.Millisecond)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 1 {
			t.Errorf("expected exactly 1 attempt with negative maxAttempts, got %d", attempts)
		}
	})
}

func TestTimeout(t *testing.T) {
	t.Run("Success Within Timeout", func(t *testing.T) {
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return n * 2, nil
		})

		timeout := Timeout(proc, 100*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Timeout Exceeded", func(t *testing.T) {
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return n * 2, nil
		})

		timeout := Timeout(proc, 20*time.Millisecond)
		_, err := timeout.Process(context.Background(), 5)
		if err == nil {
			t.Error("expected timeout error")
		}
		if !strings.Contains(err.Error(), "timeout after") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Respects Context", func(t *testing.T) {
		proc := ProcessorFunc[int](func(ctx context.Context, n int) (int, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			}
		})

		timeout := Timeout(proc, 50*time.Millisecond)
		_, err := timeout.Process(context.Background(), 5)
		if err == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestAdvancedConnectorComposition(t *testing.T) {
	t.Run("Retry with Timeout", func(t *testing.T) {
		var attempts int32
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			currentAttempt := atomic.AddInt32(&attempts, 1)
			if currentAttempt < 2 {
				time.Sleep(50 * time.Millisecond)
				return 0, errors.New("slow failure")
			}
			return n * 2, nil
		})

		// Each attempt gets 30ms timeout, retry up to 3 times
		workflow := Retry(Timeout(proc, 30*time.Millisecond), 3)

		start := time.Now()
		result, err := workflow.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		// First attempt should timeout after 30ms, second should succeed
		if elapsed > 100*time.Millisecond {
			t.Errorf("took too long: %v", elapsed)
		}
	})

}
