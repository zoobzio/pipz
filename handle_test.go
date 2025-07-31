package pipz

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHandle(t *testing.T) {
	t.Run("Success Does Not Trigger Handler", func(t *testing.T) {
		handlerCalled := false
		processor := Transform("success", func(_ context.Context, n int) int {
			return n * 2
		})
		errorHandler := Effect("error-handler", func(_ context.Context, _ *Error[int]) error {
			handlerCalled = true
			return nil
		})

		handle := NewHandle("test-handle", processor, errorHandler)
		result, err := handle.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if handlerCalled {
			t.Error("error handler should not be called on success")
		}
	})

	t.Run("Error Triggers Handler", func(t *testing.T) {
		expectedErr := errors.New("processor failed")
		var capturedErr *Error[int]

		processor := Apply("failing", func(_ context.Context, _ int) (int, error) {
			return 0, expectedErr
		})
		errorHandler := Effect("error-handler", func(_ context.Context, err *Error[int]) error {
			capturedErr = err
			return nil
		})

		handle := NewHandle("test-handle", processor, errorHandler)
		_, err := handle.Process(context.Background(), 5)

		// The error is now wrapped in Error struct, check if it contains the expected error
		if !errors.Is(err, expectedErr) && !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Fatalf("expected original error, got %v", err)
		}
		// The handler receives the full *Error[int] with context
		if capturedErr == nil {
			t.Fatal("handler should have received error")
		}
		if !errors.Is(capturedErr.Err, expectedErr) {
			t.Errorf("handler received wrong error: got %v, want %v", capturedErr.Err, expectedErr)
		}
		// Check that path includes both processors
		if len(capturedErr.Path) < 2 || capturedErr.Path[0] != "test-handle" || capturedErr.Path[1] != "failing" {
			t.Errorf("expected path [test-handle failing], got %v", capturedErr.Path)
		}
	})

	t.Run("Handler Error Is Ignored", func(t *testing.T) {
		processorErr := errors.New("processor failed")
		handlerErr := errors.New("handler failed")

		processor := Apply("failing", func(_ context.Context, _ int) (int, error) {
			return 0, processorErr
		})
		errorHandler := Apply("error-handler", func(_ context.Context, err *Error[int]) (*Error[int], error) {
			return err, handlerErr // Handler itself fails
		})

		handle := NewHandle("test-handle", processor, errorHandler)
		_, err := handle.Process(context.Background(), 5)

		// Should still get original error (wrapped), not handler error
		if !errors.Is(err, processorErr) && !strings.Contains(err.Error(), processorErr.Error()) {
			t.Fatalf("expected original error, got %v", err)
		}
	})

	t.Run("Concurrent Safety", func(t *testing.T) {
		var errorCount int32
		processor := Apply("sometimes-fail", func(_ context.Context, n int) (int, error) {
			if n%2 == 0 {
				return 0, errors.New("even number")
			}
			return n, nil
		})
		errorHandler := Effect("count-errors", func(_ context.Context, _ *Error[int]) error {
			atomic.AddInt32(&errorCount, 1)
			return nil
		})

		handle := NewHandle("test-handle", processor, errorHandler)

		// Run concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(n int) {
				_, err := handle.Process(context.Background(), n)
				// We expect errors for even numbers, no error for odd numbers
				if n%2 == 0 && err == nil {
					t.Errorf("expected error for even number %d", n)
				} else if n%2 != 0 && err != nil {
					t.Errorf("unexpected error for odd number %d: %v", n, err)
				}
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have 5 errors (0, 2, 4, 6, 8)
		if atomic.LoadInt32(&errorCount) != 5 {
			t.Errorf("expected 5 errors, got %d", errorCount)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		processor1 := Transform("p1", func(_ context.Context, n int) int { return n })
		processor2 := Transform("p2", func(_ context.Context, n int) int { return n * 2 })
		handler1 := Effect("h1", func(_ context.Context, _ *Error[int]) error { return nil })
		handler2 := Effect("h2", func(_ context.Context, _ *Error[int]) error { return nil })

		handle := NewHandle("test", processor1, handler1)

		// Test getters - just verify they return something (can't compare functions)
		if handle.GetProcessor() == nil {
			t.Error("GetProcessor returned nil")
		}
		if handle.GetErrorHandler() == nil {
			t.Error("GetErrorHandler returned nil")
		}

		// Test setters
		handle.SetProcessor(processor2)
		handle.SetErrorHandler(handler2)

		result, err := handle.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 { // processor2: 5 * 2
			t.Errorf("expected 10 after SetProcessor, got %d", result)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		processor := Transform("noop", func(_ context.Context, n int) int { return n })
		errorHandler := Effect("noop", func(_ context.Context, _ *Error[int]) error { return nil })
		handle := NewHandle("my-handle", processor, errorHandler)
		if handle.Name() != "my-handle" {
			t.Errorf("expected 'my-handle', got %q", handle.Name())
		}
	})
}
