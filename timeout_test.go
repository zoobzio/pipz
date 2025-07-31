package pipz

import (
	"context"
	"errors"
	"testing"
	"time"
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
}
