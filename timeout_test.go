package pipz

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
	"github.com/zoobzio/clockz"
)

func TestTimeout(t *testing.T) {
	t.Run("Completes Within Timeout", func(t *testing.T) {
		processor := Apply(NewIdentity("fast", ""), func(_ context.Context, n int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return n * 2, nil
		})

		timeout := NewTimeout(NewIdentity("test-timeout", ""), processor, 100*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Exceeds Timeout", func(t *testing.T) {
		processor := Apply(NewIdentity("slow", ""), func(_ context.Context, n int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return n * 2, nil
		})

		timeout := NewTimeout(NewIdentity("test-timeout", ""), processor, 50*time.Millisecond)
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
		processor := Apply(NewIdentity("slow", ""), func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		})

		timeout := NewTimeout(NewIdentity("test-timeout", ""), processor, 50*time.Millisecond)
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
		processor := Transform(NewIdentity("test", ""), func(_ context.Context, n int) int { return n })
		timeout := NewTimeout(NewIdentity("test", ""), processor, 100*time.Millisecond)

		if timeout.GetDuration() != 100*time.Millisecond {
			t.Errorf("expected 100ms, got %v", timeout.GetDuration())
		}

		timeout.SetDuration(200 * time.Millisecond)
		if timeout.GetDuration() != 200*time.Millisecond {
			t.Errorf("expected 200ms, got %v", timeout.GetDuration())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Transform(NewIdentity("noop", ""), func(_ context.Context, n int) int { return n })
		timeout := NewTimeout(NewIdentity("my-timeout", ""), processor, time.Second)
		if timeout.Identity().Name() != "my-timeout" {
			t.Errorf("expected 'my-timeout', got %q", timeout.Identity().Name())
		}
	})

	t.Run("Processor Error Within Timeout", func(t *testing.T) {
		// Processor that completes within timeout but returns an error
		processor := Apply(NewIdentity("error-proc", ""), func(_ context.Context, _ int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return 0, errors.New("processor failed")
		})

		timeout := NewTimeout(NewIdentity("error-timeout", ""), processor, 100*time.Millisecond)
		_, err := timeout.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error from processor")
		}

		// Check error path includes timeout name
		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			expectedPath := []string{"error-timeout", "error-proc"}
			if len(pipeErr.Path) != 2 || pipeErr.Path[0].Name() != expectedPath[0] || pipeErr.Path[1].Name() != expectedPath[1] {
				t.Errorf("expected error path %v, got %v", expectedPath, pipeErr.Path)
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[int]")
		}
	})

	t.Run("Non-Pipeline Error Wrapping", func(t *testing.T) {
		// Test that raw errors (not pipeline errors) get wrapped correctly
		processor := Apply(NewIdentity("raw-error", ""), func(_ context.Context, _ int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			// Return a raw error, not a pipeline error
			return 42, errors.New("raw error message")
		})

		timeout := NewTimeout(NewIdentity("wrapper-timeout", ""), processor, 100*time.Millisecond)
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
			expectedPath := []string{"wrapper-timeout", "raw-error"}
			if len(pipeErr.Path) != 2 || pipeErr.Path[0].Name() != expectedPath[0] || pipeErr.Path[1].Name() != expectedPath[1] {
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
		processor := Apply(NewIdentity("slow", ""), func(ctx context.Context, _ int) (int, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return 0, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		})

		timeout := NewTimeout(NewIdentity("cancel-timeout", ""), processor, 300*time.Millisecond) // Longer than processor delay

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
		panicProcessor := Apply(NewIdentity("panic_processor", ""), func(_ context.Context, _ int) (int, error) {
			panic("timeout processor panic")
		})

		timeout := NewTimeout(NewIdentity("panic_timeout", ""), panicProcessor, 100*time.Millisecond)
		result, err := timeout.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0].Name() != "panic_timeout" {
			t.Errorf("expected path to start with 'panic_timeout', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	t.Run("Deterministic Timeout With Fake Clock", func(t *testing.T) {
		clock := clockz.NewFakeClock()

		// Processor that waits for context cancellation
		processor := Apply(NewIdentity("wait-for-timeout", ""), func(ctx context.Context, n int) (int, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(1 * time.Second):
				// This should never happen with fake clock
				return n * 2, nil
			}
		})

		timeout := NewTimeout(NewIdentity("fake-timeout", ""), processor, 100*time.Millisecond).WithClock(clock)

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
		processor := Transform(NewIdentity("test", ""), func(_ context.Context, n int) int { return n })
		timeout := NewTimeout(NewIdentity("test", ""), processor, 100*time.Millisecond)

		clock := clockz.NewFakeClock()
		timeout2 := timeout.WithClock(clock)

		// Should return same instance for chaining
		if timeout2 != timeout {
			t.Error("WithClock should return same instance for chaining")
		}
	})

	t.Run("Emits timeout.triggered hook", func(t *testing.T) {
		clock := clockz.NewFakeClock()

		var mu sync.Mutex
		var triggered bool
		var hookName string
		var hookDuration float64

		listener := capitan.Hook(SignalTimeoutTriggered, func(_ context.Context, e *capitan.Event) {
			mu.Lock()
			defer mu.Unlock()
			triggered = true
			hookName, _ = FieldName.From(e)
			hookDuration, _ = FieldDuration.From(e)
		})
		defer listener.Close()

		processor := Apply(NewIdentity("slow", ""), func(ctx context.Context, n int) (int, error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-clock.After(100 * time.Millisecond):
				return n * 2, nil
			}
		})

		timeout := NewTimeout(NewIdentity("timeout-hook-test", ""), processor, 50*time.Millisecond).WithClock(clock)

		// Run in goroutine so we can advance clock
		done := make(chan struct{})
		var result int
		var err error
		go func() {
			defer close(done)
			result, err = timeout.Process(context.Background(), 5)
		}()

		// Allow goroutine to start
		time.Sleep(10 * time.Millisecond)

		// Advance clock past timeout duration
		clock.Advance(50 * time.Millisecond)
		clock.BlockUntilReady()

		// Give time for processing
		time.Sleep(10 * time.Millisecond)

		// Wait for completion
		<-done

		if err == nil {
			t.Fatal("expected timeout error")
		}

		if result != 5 {
			t.Errorf("expected original input 5, got %d", result)
		}

		// Wait for async hook processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if !triggered {
			t.Error("expected timeout.triggered signal to be emitted")
		}

		if hookName != "timeout-hook-test" {
			t.Errorf("expected name 'timeout-hook-test', got %q", hookName)
		}

		if hookDuration != 0.05 {
			t.Errorf("expected duration 0.05s, got %.2f", hookDuration)
		}
	})

	t.Run("Close Tests", func(t *testing.T) {
		t.Run("Closes Child Processor", func(t *testing.T) {
			p := newTrackingProcessor[int](NewIdentity("p", ""))

			to := NewTimeout(NewIdentity("test", ""), p, 100*time.Millisecond)
			err := to.Close()

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if p.CloseCalls() != 1 {
				t.Errorf("expected 1 close call, got %d", p.CloseCalls())
			}
		})

		t.Run("Propagates Close Error", func(t *testing.T) {
			p := newTrackingProcessor[int](NewIdentity("p", "")).WithCloseError(errors.New("close error"))

			to := NewTimeout(NewIdentity("test", ""), p, 100*time.Millisecond)
			err := to.Close()

			if err == nil {
				t.Error("expected error")
			}
			if p.CloseCalls() != 1 {
				t.Errorf("expected 1 close call, got %d", p.CloseCalls())
			}
		})

		t.Run("Idempotency", func(t *testing.T) {
			p := newTrackingProcessor[int](NewIdentity("p", ""))
			to := NewTimeout(NewIdentity("test", ""), p, 100*time.Millisecond)

			_ = to.Close()
			_ = to.Close()

			if p.CloseCalls() != 1 {
				t.Errorf("expected 1 close call, got %d", p.CloseCalls())
			}
		})
	})

	t.Run("Does not emit hook on cancellation", func(t *testing.T) {
		var mu sync.Mutex
		var triggered bool

		listener := capitan.Hook(SignalTimeoutTriggered, func(_ context.Context, _ *capitan.Event) {
			mu.Lock()
			defer mu.Unlock()
			triggered = true
		})
		defer listener.Close()

		processor := Apply(NewIdentity("slow", ""), func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		})

		timeout := NewTimeout(NewIdentity("cancel-test", ""), processor, 200*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := timeout.Process(ctx, 5)

		if err == nil {
			t.Fatal("expected cancellation error")
		}

		// Wait for any potential async hook processing
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if triggered {
			t.Error("should not emit timeout.triggered on cancellation")
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc := Transform(NewIdentity("inner-proc", ""), func(_ context.Context, n int) int { return n })
		timeout := NewTimeout(NewIdentity("test-timeout", "Timeout connector"), proc, 5*time.Second)

		schema := timeout.Schema()

		if schema.Identity.Name() != "test-timeout" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-timeout")
		}
		if schema.Type != "timeout" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "timeout")
		}

		flow, ok := TimeoutKey.From(schema)
		if !ok {
			t.Fatal("Expected TimeoutFlow")
		}
		if flow.Processor.Identity.Name() != "inner-proc" {
			t.Errorf("Flow.Processor.Identity.Name() = %v, want %v", flow.Processor.Identity.Name(), "inner-proc")
		}
		if schema.Metadata["duration"] != "5s" {
			t.Errorf("Metadata[duration] = %v, want 5s", schema.Metadata["duration"])
		}
	})
}
