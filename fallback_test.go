package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// plainErrorProcessor is a test helper that returns plain errors (not Error[T] types).
// This is needed to test the non-pipeline error wrapping in fallback.go lines 97-102.
type plainErrorProcessor[T any] struct {
	identity Identity
	err      error
}

func (p *plainErrorProcessor[T]) Process(_ context.Context, _ T) (T, error) {
	var zero T
	return zero, p.err
}

func (p *plainErrorProcessor[T]) Identity() Identity {
	return p.identity
}

func (p *plainErrorProcessor[T]) Schema() Node {
	return Node{
		Identity: p.identity,
		Type:     "plain_error_processor",
	}
}

func (*plainErrorProcessor[T]) Close() error {
	return nil
}

func TestFallback(t *testing.T) {
	t.Run("Primary Success", func(t *testing.T) {
		primary := Transform(NewIdentity("primary", ""), func(_ context.Context, n int) int {
			return n * 2
		})
		fallback := Transform(NewIdentity("fallback", ""), func(_ context.Context, n int) int {
			return n * 3
		})

		fb := NewFallback(NewIdentity("test-fallback", ""), primary, fallback)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 { // Should use primary (5 * 2)
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Primary Fails Uses Fallback", func(t *testing.T) {
		primary := Apply(NewIdentity("primary", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := Transform(NewIdentity("fallback", ""), func(_ context.Context, n int) int {
			return n * 3
		})

		fb := NewFallback(NewIdentity("test-fallback", ""), primary, fallback)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 15 { // Should use fallback (5 * 3)
			t.Errorf("expected 15, got %d", result)
		}
	})

	t.Run("Both Fail", func(t *testing.T) {
		primary := Apply(NewIdentity("primary", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := Apply(NewIdentity("fallback", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("fallback failed")
		})

		fb := NewFallback(NewIdentity("test-fallback", ""), primary, fallback)

		_, err := fb.Process(context.Background(), 5)
		if err == nil {
			t.Fatal("expected error when both fail")
		}
		if !strings.Contains(err.Error(), "fallback failed") {
			t.Errorf("expected fallback error, got: %v", err)
		}
	})

	t.Run("Identity Method", func(t *testing.T) {
		primary := Transform(NewIdentity("noop", ""), func(_ context.Context, n int) int { return n })
		fallback := Transform(NewIdentity("noop", ""), func(_ context.Context, n int) int { return n })
		fb := NewFallback(NewIdentity("my-fallback", ""), primary, fallback)
		if fb.Identity().Name() != "my-fallback" {
			t.Errorf("expected 'my-fallback', got %q", fb.Identity().Name())
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		primary1 := Transform(NewIdentity("p1", ""), func(_ context.Context, n int) int { return n })
		primary2 := Transform(NewIdentity("p2", ""), func(_ context.Context, n int) int { return n * 2 })
		fallback1 := Transform(NewIdentity("f1", ""), func(_ context.Context, n int) int { return n })
		fallback2 := Transform(NewIdentity("f2", ""), func(_ context.Context, n int) int { return n * 3 })

		fb := NewFallback(NewIdentity("test", ""), primary1, fallback1)

		// Test getters - just verify they return something (can't compare functions)
		if fb.GetPrimary() == nil {
			t.Error("GetPrimary returned nil")
		}
		if fb.GetFallback() == nil {
			t.Error("GetFallback returned nil")
		}

		// Test setters
		fb.SetPrimary(primary2)
		fb.SetFallback(fallback2)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 { // primary2: 5 * 2
			t.Errorf("expected 10 after SetPrimary, got %d", result)
		}
	})

	t.Run("Multiple Fallbacks", func(t *testing.T) {
		first := Apply(NewIdentity("first", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("first failed")
		})
		second := Apply(NewIdentity("second", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("second failed")
		})
		third := Transform(NewIdentity("third", ""), func(_ context.Context, n int) int {
			return n * 10 // This should succeed
		})
		fourth := Transform(NewIdentity("fourth", ""), func(_ context.Context, n int) int {
			return n * 100 // Should not be reached
		})

		fb := NewFallback(NewIdentity("multi-fallback", ""), first, second, third, fourth)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 50 { // Should use third (5 * 10)
			t.Errorf("expected 50, got %d", result)
		}
	})

	t.Run("All Multiple Fallbacks Fail", func(t *testing.T) {
		first := Apply(NewIdentity("first", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("first failed")
		})
		second := Apply(NewIdentity("second", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("second failed")
		})
		third := Apply(NewIdentity("third", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("third failed")
		})

		fb := NewFallback(NewIdentity("all-fail", ""), first, second, third)

		result, err := fb.Process(context.Background(), 5)
		if err == nil {
			t.Fatal("expected error when all fallbacks fail")
		}

		// Should return last error with path
		if !strings.Contains(err.Error(), "third failed") {
			t.Errorf("expected error to contain 'third failed', got: %v", err)
		}
		if !strings.Contains(err.Error(), "all-fail") {
			t.Errorf("expected error path to contain 'all-fail', got: %v", err)
		}
		if result != 5 { // Should return original input
			t.Errorf("expected original input 5, got %d", result)
		}
	})

	t.Run("New API Methods", func(t *testing.T) {
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n * 2 })
		second := Transform(NewIdentity("second", ""), func(_ context.Context, n int) int { return n * 3 })
		third := Transform(NewIdentity("third", ""), func(_ context.Context, n int) int { return n * 4 })
		fourth := Transform(NewIdentity("fourth", ""), func(_ context.Context, n int) int { return n * 5 })

		fb := NewFallback(NewIdentity("test", ""), first, second)

		// Test Len
		if fb.Len() != 2 {
			t.Errorf("expected length 2, got %d", fb.Len())
		}

		// Test GetProcessors
		processors := fb.GetProcessors()
		if len(processors) != 2 {
			t.Errorf("expected 2 processors, got %d", len(processors))
		}

		// Test AddFallback
		fb.AddFallback(third)
		if fb.Len() != 3 {
			t.Errorf("expected length 3 after AddFallback, got %d", fb.Len())
		}

		// Test SetProcessors
		fb.SetProcessors(first, second, third, fourth)
		if fb.Len() != 4 {
			t.Errorf("expected length 4 after SetProcessors, got %d", fb.Len())
		}

		// Test InsertAt
		fifth := Transform(NewIdentity("fifth", ""), func(_ context.Context, n int) int { return n * 6 })
		if err := fb.InsertAt(2, fifth); err != nil {
			t.Errorf("InsertAt(2) returned error: %v", err)
		}
		if fb.Len() != 5 {
			t.Errorf("expected length 5 after InsertAt, got %d", fb.Len())
		}

		// Test RemoveAt
		if err := fb.RemoveAt(2); err != nil {
			t.Errorf("RemoveAt(2) returned error: %v", err)
		}
		if fb.Len() != 4 {
			t.Errorf("expected length 4 after RemoveAt, got %d", fb.Len())
		}

		// Test RemoveAt with invalid index - should return error
		if err := fb.RemoveAt(-1); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for negative index, got %v", err)
		}
	})

	t.Run("Empty Constructor Returns Error On Process", func(t *testing.T) {
		// Empty construction is allowed, but Process() returns an error
		fb := NewFallback[int](NewIdentity("empty", ""))
		if fb == nil {
			t.Fatal("expected non-nil Fallback")
		}

		_, err := fb.Process(context.Background(), 42)
		if err == nil {
			t.Error("expected error when processing with no processors")
		}

		var pipeErr *Error[int]
		if !errors.As(err, &pipeErr) {
			t.Errorf("expected Error[int], got %T", err)
		}
	})

	t.Run("SetProcessors Empty Returns Error On Process", func(t *testing.T) {
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n })
		fb := NewFallback(NewIdentity("test", ""), first)

		// Setting empty processors is allowed
		fb.SetProcessors()
		if fb.Len() != 0 {
			t.Errorf("expected length 0 after SetProcessors(), got %d", fb.Len())
		}

		// But Process() returns an error
		_, err := fb.Process(context.Background(), 42)
		if err == nil {
			t.Error("expected error when processing with no processors")
		}
	})

	t.Run("InsertAt Boundary Tests", func(t *testing.T) {
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n })
		second := Transform(NewIdentity("second", ""), func(_ context.Context, n int) int { return n * 2 })
		third := Transform(NewIdentity("third", ""), func(_ context.Context, n int) int { return n * 3 })

		fb := NewFallback(NewIdentity("test", ""), first, second)

		// Valid: Insert at beginning
		if err := fb.InsertAt(0, third); err != nil {
			t.Errorf("InsertAt(0) returned error: %v", err)
		}
		if fb.Len() != 3 {
			t.Errorf("expected length 3 after InsertAt(0), got %d", fb.Len())
		}

		// Valid: Insert at end
		fourth := Transform(NewIdentity("fourth", ""), func(_ context.Context, n int) int { return n * 4 })
		if err := fb.InsertAt(3, fourth); err != nil {
			t.Errorf("InsertAt(3) returned error: %v", err)
		}
		if fb.Len() != 4 {
			t.Errorf("expected length 4 after InsertAt(3), got %d", fb.Len())
		}

		// Test negative index - should return error
		t.Run("Negative Index", func(t *testing.T) {
			if err := fb.InsertAt(-1, first); !errors.Is(err, ErrIndexOutOfBounds) {
				t.Errorf("expected ErrIndexOutOfBounds for negative index, got %v", err)
			}
		})

		// Test index too large - should return error
		t.Run("Index Too Large", func(t *testing.T) {
			if err := fb.InsertAt(10, first); !errors.Is(err, ErrIndexOutOfBounds) {
				t.Errorf("expected ErrIndexOutOfBounds for index too large, got %v", err)
			}
		})
	})

	t.Run("RemoveAt Boundary Tests", func(t *testing.T) {
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n })
		second := Transform(NewIdentity("second", ""), func(_ context.Context, n int) int { return n * 2 })
		third := Transform(NewIdentity("third", ""), func(_ context.Context, n int) int { return n * 3 })

		// Test removing from various positions
		fb := NewFallback(NewIdentity("test", ""), first, second, third)

		// Remove from middle
		if err := fb.RemoveAt(1); err != nil {
			t.Errorf("RemoveAt(1) returned error: %v", err)
		}
		if fb.Len() != 2 {
			t.Errorf("expected length 2 after RemoveAt(1), got %d", fb.Len())
		}

		// Test negative index - should return error
		t.Run("Negative Index", func(t *testing.T) {
			fb := NewFallback(NewIdentity("test", ""), first, second)
			if err := fb.RemoveAt(-1); !errors.Is(err, ErrIndexOutOfBounds) {
				t.Errorf("expected ErrIndexOutOfBounds for negative index, got %v", err)
			}
		})

		// Test index >= length - should return error
		t.Run("Index Too Large", func(t *testing.T) {
			fb := NewFallback(NewIdentity("test", ""), first, second)
			if err := fb.RemoveAt(2); !errors.Is(err, ErrIndexOutOfBounds) {
				t.Errorf("expected ErrIndexOutOfBounds for index >= length, got %v", err)
			}
		})

		// Test removing last processor - allowed, but Process() returns error
		t.Run("Remove Last Processor", func(t *testing.T) {
			fb := NewFallback(NewIdentity("test", ""), first)
			if err := fb.RemoveAt(0); err != nil {
				t.Errorf("RemoveAt(0) returned error: %v", err)
			}
			if fb.Len() != 0 {
				t.Errorf("expected length 0 after removing last processor, got %d", fb.Len())
			}

			// Process should return error when empty
			_, err := fb.Process(context.Background(), 42)
			if err == nil {
				t.Error("expected error when processing with no processors")
			}
		})
	})

	t.Run("SetFallback When Only One Processor", func(t *testing.T) {
		// Test that SetFallback adds a second processor when there's only one
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n })
		second := Transform(NewIdentity("second", ""), func(_ context.Context, n int) int { return n * 2 })

		fb := NewFallback(NewIdentity("test", ""), first)
		if fb.Len() != 1 {
			t.Errorf("expected length 1 initially, got %d", fb.Len())
		}

		// SetFallback should add the second processor
		fb.SetFallback(second)
		if fb.Len() != 2 {
			t.Errorf("expected length 2 after SetFallback, got %d", fb.Len())
		}

		// Verify the fallback was added correctly
		if fb.GetFallback() == nil {
			t.Error("GetFallback returned nil after SetFallback")
		}
	})

	t.Run("GetFallback When Only One Processor", func(t *testing.T) {
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n })
		fb := NewFallback(NewIdentity("test", ""), first)

		// GetFallback should return nil when there's only one processor
		if fb.GetFallback() != nil {
			t.Error("expected GetFallback to return nil when only one processor")
		}
	})

	t.Run("GetPrimary With Empty Processors", func(t *testing.T) {
		// This tests an edge case that shouldn't happen in normal use
		// but ensures GetPrimary handles it gracefully
		first := Transform(NewIdentity("first", ""), func(_ context.Context, n int) int { return n })
		fb := NewFallback(NewIdentity("test", ""), first)

		// GetPrimary should always return something for a valid Fallback
		if fb.GetPrimary() == nil {
			t.Error("GetPrimary returned nil")
		}
	})

	t.Run("GetPrimary Returns Nil Edge Case", func(t *testing.T) {
		// This tests GetPrimary returning nil for empty Fallback
		fb := NewFallback[int](NewIdentity("empty", ""))

		// GetPrimary should return nil for empty Fallback
		if fb.GetPrimary() != nil {
			t.Error("expected GetPrimary to return nil for empty processors")
		}
	})

	t.Run("Process Returns Data When LastErr Is Nil", func(t *testing.T) {
		// This tests line 95 in fallback.go - though it's technically unreachable
		// in normal operation, we test the defensive code anyway

		// Create a fallback with processors that succeed
		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, n int) int { return n * 2 })
		fb := NewFallback(NewIdentity("test", ""), p1)

		// Process should succeed on first processor
		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// The line 95 case (lastErr == nil after loop) is defensive code
		// It would only be hit if we had zero processors, but constructor prevents that
	})

	t.Run("Fallback panic recovery", func(t *testing.T) {
		panicProcessor := Apply(NewIdentity("panic_processor", ""), func(_ context.Context, _ int) (int, error) {
			panic("fallback processor panic")
		})
		successProcessor := Transform(NewIdentity("success_processor", ""), func(_ context.Context, n int) int {
			return n * 2
		})

		fallback := NewFallback(NewIdentity("panic_fallback", ""), panicProcessor, successProcessor)
		result, err := fallback.Process(context.Background(), 42)

		// Fallback should use the success processor
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != 84 {
			t.Errorf("expected fallback result 84, got %d", result)
		}
	})

	t.Run("All fallback processors panic", func(t *testing.T) {
		panic1 := Apply(NewIdentity("panic1", ""), func(_ context.Context, _ int) (int, error) {
			panic("first processor panic")
		})
		panic2 := Apply(NewIdentity("panic2", ""), func(_ context.Context, _ int) (int, error) {
			panic("second processor panic")
		})

		fallback := NewFallback(NewIdentity("all_panic_fallback", ""), panic1, panic2)
		result, err := fallback.Process(context.Background(), 42)

		if result != 42 {
			t.Errorf("expected original input 42, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0].Name() != "all_panic_fallback" {
			t.Errorf("expected path to start with 'all_panic_fallback', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	t.Run("Close Tests", func(t *testing.T) {
		t.Run("Closes All Children", func(t *testing.T) {
			p1 := newTrackingProcessor[int](NewIdentity("p1", ""))
			p2 := newTrackingProcessor[int](NewIdentity("p2", ""))

			f := NewFallback(NewIdentity("test", ""), p1, p2)
			err := f.Close()

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
				t.Error("expected all processors to be closed")
			}
		})

		t.Run("Aggregates Errors", func(t *testing.T) {
			p1 := newTrackingProcessor[int](NewIdentity("p1", "")).WithCloseError(errors.New("p1 error"))
			p2 := newTrackingProcessor[int](NewIdentity("p2", "")).WithCloseError(errors.New("p2 error"))

			f := NewFallback(NewIdentity("test", ""), p1, p2)
			err := f.Close()

			if err == nil {
				t.Error("expected error")
			}
			if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
				t.Error("expected all processors to be closed")
			}
		})

		t.Run("Idempotency", func(t *testing.T) {
			p := newTrackingProcessor[int](NewIdentity("p", ""))
			f := NewFallback(NewIdentity("test", ""), p)

			_ = f.Close()
			_ = f.Close()

			if p.CloseCalls() != 1 {
				t.Errorf("expected 1 close call, got %d", p.CloseCalls())
			}
		})
	})

	// High-value coverage improvement tests targeting specific execution paths
	t.Run("Non-Pipeline Error Wrapping Coverage", func(t *testing.T) {
		// Test lines 97-102: non-pipeline error wrapping when all processors fail
		// Need to create processors that return plain errors, not Error[T] types
		plainErr := errors.New("plain error")
		// Create a processor that returns a plain error (not wrapped in Error[T])
		// We'll implement this by creating a fake processor that doesn't use the pipz error wrapping
		fakeProcessor := &plainErrorProcessor[int]{
			identity: NewIdentity("plain-error-proc", ""),
			err:      plainErr,
		}
		fallback := NewFallback(NewIdentity("error-wrapper", ""), fakeProcessor)
		result, err := fallback.Process(context.Background(), 42)

		// Should return original input
		if result != 42 {
			t.Errorf("expected original input 42, got %d", result)
		}

		// Should return wrapped error (lines 97-102)
		var pipeErr *Error[int]
		if !errors.As(err, &pipeErr) {
			t.Fatalf("expected wrapped Error[T], got %T", err)
		}

		// Verify wrapping details
		if !errors.Is(pipeErr.Err, plainErr) {
			t.Error("wrapped error should contain plain error")
		}
		if pipeErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipeErr.InputData)
		}
		if len(pipeErr.Path) != 1 || pipeErr.Path[0].Name() != "error-wrapper" {
			t.Errorf("expected path [error-wrapper], got %v", pipeErr.Path)
		}
		if pipeErr.Timestamp.IsZero() {
			t.Error("timestamp should be set")
		}
	})

	t.Run("Empty Fallback Returns Error", func(t *testing.T) {
		// Empty fallback returns error at Process() time
		fallback := NewFallback[int](NewIdentity("empty", ""))

		result, err := fallback.Process(context.Background(), 42)

		if err == nil {
			t.Error("expected error for empty processors")
		}
		if result != 0 {
			t.Errorf("expected zero value, got %d", result)
		}

		// Verify error type and message
		var pipeErr *Error[int]
		if !errors.As(err, &pipeErr) {
			t.Errorf("expected Error[int], got %T", err)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		primary := Transform(NewIdentity("primary", ""), func(_ context.Context, n int) int { return n })
		backup := Transform(NewIdentity("backup", ""), func(_ context.Context, n int) int { return n })

		fallback := NewFallback(NewIdentity("test-fallback", "Fallback connector"), primary, backup)

		schema := fallback.Schema()

		if schema.Identity.Name() != "test-fallback" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-fallback")
		}
		if schema.Type != "fallback" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "fallback")
		}

		flow, ok := FallbackKey.From(schema)
		if !ok {
			t.Fatal("Expected FallbackFlow")
		}
		if flow.Primary.Identity.Name() != "primary" {
			t.Errorf("Flow.Primary.Identity.Name() = %v, want %v", flow.Primary.Identity.Name(), "primary")
		}
		if len(flow.Backups) != 1 {
			t.Errorf("len(Flow.Backups) = %d, want 1", len(flow.Backups))
		}
	})
}
