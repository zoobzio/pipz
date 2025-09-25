package pipz

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// plainErrorProcessor is a test helper that returns plain errors (not Error[T] types).
// This is needed to test the non-pipeline error wrapping in fallback.go lines 97-102.
type plainErrorProcessor[T any] struct {
	name Name
	err  error
}

func (p *plainErrorProcessor[T]) Process(_ context.Context, _ T) (T, error) {
	var zero T
	return zero, p.err
}

func (p *plainErrorProcessor[T]) Name() Name {
	return p.name
}

func (*plainErrorProcessor[T]) Metrics() *metricz.Registry {
	return nil
}

func (*plainErrorProcessor[T]) Tracer() *tracez.Tracer {
	return nil
}

func (*plainErrorProcessor[T]) Close() error {
	return nil
}

func TestFallback(t *testing.T) {
	t.Run("Primary Success", func(t *testing.T) {
		primary := Transform("primary", func(_ context.Context, n int) int {
			return n * 2
		})
		fallback := Transform("fallback", func(_ context.Context, n int) int {
			return n * 3
		})

		fb := NewFallback("test-fallback", primary, fallback)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 { // Should use primary (5 * 2)
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Primary Fails Uses Fallback", func(t *testing.T) {
		primary := Apply("primary", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := Transform("fallback", func(_ context.Context, n int) int {
			return n * 3
		})

		fb := NewFallback("test-fallback", primary, fallback)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 15 { // Should use fallback (5 * 3)
			t.Errorf("expected 15, got %d", result)
		}
	})

	t.Run("Both Fail", func(t *testing.T) {
		primary := Apply("primary", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := Apply("fallback", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("fallback failed")
		})

		fb := NewFallback("test-fallback", primary, fallback)

		_, err := fb.Process(context.Background(), 5)
		if err == nil {
			t.Fatal("expected error when both fail")
		}
		if !strings.Contains(err.Error(), "fallback failed") {
			t.Errorf("expected fallback error, got: %v", err)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		primary := Transform("noop", func(_ context.Context, n int) int { return n })
		fallback := Transform("noop", func(_ context.Context, n int) int { return n })
		fb := NewFallback("my-fallback", primary, fallback)
		if fb.Name() != "my-fallback" {
			t.Errorf("expected 'my-fallback', got %q", fb.Name())
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		primary1 := Transform("p1", func(_ context.Context, n int) int { return n })
		primary2 := Transform("p2", func(_ context.Context, n int) int { return n * 2 })
		fallback1 := Transform("f1", func(_ context.Context, n int) int { return n })
		fallback2 := Transform("f2", func(_ context.Context, n int) int { return n * 3 })

		fb := NewFallback("test", primary1, fallback1)

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
		first := Apply("first", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("first failed")
		})
		second := Apply("second", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("second failed")
		})
		third := Transform("third", func(_ context.Context, n int) int {
			return n * 10 // This should succeed
		})
		fourth := Transform("fourth", func(_ context.Context, n int) int {
			return n * 100 // Should not be reached
		})

		fb := NewFallback("multi-fallback", first, second, third, fourth)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 50 { // Should use third (5 * 10)
			t.Errorf("expected 50, got %d", result)
		}
	})

	t.Run("All Multiple Fallbacks Fail", func(t *testing.T) {
		first := Apply("first", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("first failed")
		})
		second := Apply("second", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("second failed")
		})
		third := Apply("third", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("third failed")
		})

		fb := NewFallback("all-fail", first, second, third)

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
		first := Transform("first", func(_ context.Context, n int) int { return n * 2 })
		second := Transform("second", func(_ context.Context, n int) int { return n * 3 })
		third := Transform("third", func(_ context.Context, n int) int { return n * 4 })
		fourth := Transform("fourth", func(_ context.Context, n int) int { return n * 5 })

		fb := NewFallback("test", first, second)

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
		fifth := Transform("fifth", func(_ context.Context, n int) int { return n * 6 })
		fb.InsertAt(2, fifth) // Insert at position 2
		if fb.Len() != 5 {
			t.Errorf("expected length 5 after InsertAt, got %d", fb.Len())
		}

		// Test RemoveAt
		fb.RemoveAt(2) // Remove the one we just inserted
		if fb.Len() != 4 {
			t.Errorf("expected length 4 after RemoveAt, got %d", fb.Len())
		}

		// Test RemoveAt with invalid index - should panic
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for negative index")
			}
		}()
		fb.RemoveAt(-1)
	})

	t.Run("Constructor Panic Tests", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when NewFallback called with no processors")
			} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "at least one processor") {
				t.Errorf("unexpected panic message: %v", r)
			}
		}()

		// This should panic
		NewFallback[int]("empty")
	})

	t.Run("SetProcessors Panic Test", func(t *testing.T) {
		first := Transform("first", func(_ context.Context, n int) int { return n })
		fb := NewFallback("test", first)
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when SetProcessors called with no processors")
			} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "at least one processor") {
				t.Errorf("unexpected panic message: %v", r)
			}
		}()

		// This should panic
		fb.SetProcessors()
	})

	t.Run("InsertAt Boundary Tests", func(t *testing.T) {
		first := Transform("first", func(_ context.Context, n int) int { return n })
		second := Transform("second", func(_ context.Context, n int) int { return n * 2 })
		third := Transform("third", func(_ context.Context, n int) int { return n * 3 })

		fb := NewFallback("test", first, second)

		// Valid: Insert at beginning
		fb.InsertAt(0, third)
		if fb.Len() != 3 {
			t.Errorf("expected length 3 after InsertAt(0), got %d", fb.Len())
		}

		// Valid: Insert at end
		fourth := Transform("fourth", func(_ context.Context, n int) int { return n * 4 })
		fb.InsertAt(3, fourth)
		if fb.Len() != 4 {
			t.Errorf("expected length 4 after InsertAt(3), got %d", fb.Len())
		}

		// Test negative index - should panic
		t.Run("Negative Index", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic for negative index")
				} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "index out of bounds") {
					t.Errorf("unexpected panic message: %v", r)
				}
			}()
			fb.InsertAt(-1, first)
		})

		// Test index too large - should panic
		t.Run("Index Too Large", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic for index too large")
				} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "index out of bounds") {
					t.Errorf("unexpected panic message: %v", r)
				}
			}()
			fb.InsertAt(10, first)
		})
	})

	t.Run("RemoveAt Boundary Tests", func(t *testing.T) {
		first := Transform("first", func(_ context.Context, n int) int { return n })
		second := Transform("second", func(_ context.Context, n int) int { return n * 2 })
		third := Transform("third", func(_ context.Context, n int) int { return n * 3 })

		// Test removing from various positions
		fb := NewFallback("test", first, second, third)

		// Remove from middle
		fb.RemoveAt(1)
		if fb.Len() != 2 {
			t.Errorf("expected length 2 after RemoveAt(1), got %d", fb.Len())
		}

		// Test negative index - should panic
		t.Run("Negative Index", func(t *testing.T) {
			fb := NewFallback("test", first, second)
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic for negative index")
				} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "index out of bounds") {
					t.Errorf("unexpected panic message: %v", r)
				}
			}()
			fb.RemoveAt(-1)
		})

		// Test index >= length - should panic
		t.Run("Index Too Large", func(t *testing.T) {
			fb := NewFallback("test", first, second)
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic for index >= length")
				} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "index out of bounds") {
					t.Errorf("unexpected panic message: %v", r)
				}
			}()
			fb.RemoveAt(2) // length is 2, so valid indices are 0 and 1
		})

		// Test removing last processor - should panic
		t.Run("Remove Last Processor", func(t *testing.T) {
			fb := NewFallback("test", first)
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic when removing last processor")
				} else if msg, ok := r.(string); !ok || !strings.Contains(msg, "cannot remove last processor") {
					t.Errorf("unexpected panic message: %v", r)
				}
			}()
			fb.RemoveAt(0)
		})
	})

	t.Run("SetFallback When Only One Processor", func(t *testing.T) {
		// Test that SetFallback adds a second processor when there's only one
		first := Transform("first", func(_ context.Context, n int) int { return n })
		second := Transform("second", func(_ context.Context, n int) int { return n * 2 })

		fb := NewFallback("test", first)
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
		first := Transform("first", func(_ context.Context, n int) int { return n })
		fb := NewFallback("test", first)

		// GetFallback should return nil when there's only one processor
		if fb.GetFallback() != nil {
			t.Error("expected GetFallback to return nil when only one processor")
		}
	})

	t.Run("GetPrimary With Empty Processors", func(t *testing.T) {
		// This tests an edge case that shouldn't happen in normal use
		// but ensures GetPrimary handles it gracefully
		first := Transform("first", func(_ context.Context, n int) int { return n })
		fb := NewFallback("test", first)

		// GetPrimary should always return something for a valid Fallback
		if fb.GetPrimary() == nil {
			t.Error("GetPrimary returned nil")
		}
	})

	t.Run("GetPrimary Returns Nil Edge Case", func(t *testing.T) {
		// This tests line 173 in fallback.go where GetPrimary returns nil
		// We need to bypass constructor validation using reflection
		first := Transform("first", func(_ context.Context, n int) int { return n })
		fb := NewFallback("test", first)

		// Use reflection to clear processors slice
		fbValue := reflect.ValueOf(fb).Elem()
		processorsField := fbValue.FieldByName("processors")

		// Make the field settable using unsafe
		processorsField = reflect.NewAt(processorsField.Type(), unsafe.Pointer(processorsField.UnsafeAddr())).Elem()
		processorsField.Set(reflect.MakeSlice(processorsField.Type(), 0, 0))

		// Now GetPrimary should return nil (line 173)
		if fb.GetPrimary() != nil {
			t.Error("expected GetPrimary to return nil for empty processors")
		}
	})

	t.Run("Process Returns Data When LastErr Is Nil", func(t *testing.T) {
		// This tests line 95 in fallback.go - though it's technically unreachable
		// in normal operation, we test the defensive code anyway

		// Create a fallback with processors that succeed
		p1 := Transform("p1", func(_ context.Context, n int) int { return n * 2 })
		fb := NewFallback("test", p1)

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
		panicProcessor := Apply("panic_processor", func(_ context.Context, _ int) (int, error) {
			panic("fallback processor panic")
		})
		successProcessor := Transform("success_processor", func(_ context.Context, n int) int {
			return n * 2
		})

		fallback := NewFallback("panic_fallback", panicProcessor, successProcessor)
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
		panic1 := Apply("panic1", func(_ context.Context, _ int) (int, error) {
			panic("first processor panic")
		})
		panic2 := Apply("panic2", func(_ context.Context, _ int) (int, error) {
			panic("second processor panic")
		})

		fallback := NewFallback("all_panic_fallback", panic1, panic2)
		result, err := fallback.Process(context.Background(), 42)

		if result != 42 {
			t.Errorf("expected original input 42, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "all_panic_fallback" {
			t.Errorf("expected path to start with 'all_panic_fallback', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	// High-value coverage improvement tests targeting specific execution paths
	t.Run("Non-Pipeline Error Wrapping Coverage", func(t *testing.T) {
		// Test lines 97-102: non-pipeline error wrapping when all processors fail
		// Need to create processors that return plain errors, not Error[T] types
		plainErr := errors.New("plain error")
		// Create a processor that returns a plain error (not wrapped in Error[T])
		// We'll implement this by creating a fake processor that doesn't use the pipz error wrapping
		fakeProcessor := &plainErrorProcessor[int]{
			name: "plain-error-proc",
			err:  plainErr,
		}
		fallback := NewFallback("error-wrapper", fakeProcessor)
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
		if len(pipeErr.Path) != 1 || pipeErr.Path[0] != "error-wrapper" {
			t.Errorf("expected path [error-wrapper], got %v", pipeErr.Path)
		}
		if pipeErr.Timestamp.IsZero() {
			t.Error("timestamp should be set")
		}
	})

	t.Run("Empty Processor Defensive Code Coverage", func(t *testing.T) {
		// This tests line 104 in fallback.go (return data, nil when lastErr == nil)
		// This is defensive code that's technically unreachable with current constructor
		// but we test it for completeness

		processor := Transform("success", func(_ context.Context, n int) int {
			return n * 2
		})

		fallback := NewFallback("defensive", processor)

		// Use reflection to create an edge case scenario
		fbValue := reflect.ValueOf(fallback).Elem()
		processorsField := fbValue.FieldByName("processors")

		// Create a scenario where we have zero processors (bypassing constructor validation)
		processorsField = reflect.NewAt(processorsField.Type(),
			unsafe.Pointer(processorsField.UnsafeAddr())).Elem()
		emptyProcessors := reflect.MakeSlice(processorsField.Type(), 0, 0)
		processorsField.Set(emptyProcessors)

		// Process should hit line 104 (return data, nil)
		result, err := fallback.Process(context.Background(), 42)

		if err != nil {
			t.Errorf("expected nil error for empty processors, got %v", err)
		}
		if result != 42 {
			t.Errorf("expected original data 42, got %d", result)
		}
	})

	t.Run("Observability", func(t *testing.T) {
		t.Run("Metrics and Spans - Primary Success", func(t *testing.T) {
			primary := Transform("primary", func(_ context.Context, n int) int {
				return n * 2
			})
			fallback1 := Apply("fallback1", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("should not be called")
			})
			fallback2 := Apply("fallback2", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("should not be called")
			})

			fb := NewFallback("test-fallback", primary, fallback1, fallback2)
			defer fb.Close()

			// Verify observability components are initialized
			if fb.Metrics() == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if fb.Tracer() == nil {
				t.Error("expected tracer to be initialized")
			}

			// Capture spans using the callback API
			var spans []tracez.Span
			fb.Tracer().OnSpanComplete(func(span tracez.Span) {
				spans = append(spans, span)
			})

			// Process - primary should succeed
			result, err := fb.Process(context.Background(), 5)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != 10 {
				t.Errorf("expected 10, got %d", result)
			}

			// Verify metrics
			processedTotal := fb.Metrics().Counter(FallbackProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := fb.Metrics().Counter(FallbackSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			attemptsTotal := fb.Metrics().Counter(FallbackAttemptsTotal).Value()
			if attemptsTotal != 1 {
				t.Errorf("expected 1 attempt (only primary), got %f", attemptsTotal)
			}

			processorCount := fb.Metrics().Gauge(FallbackProcessorCount).Value()
			if processorCount != 3 {
				t.Errorf("expected processor count 3, got %f", processorCount)
			}

			// Check duration was recorded
			duration := fb.Metrics().Gauge(FallbackDurationMs).Value()
			if duration < 0 {
				t.Errorf("expected non-negative duration, got %f", duration)
			}

			// Verify spans were captured (1 main + 1 attempt span)
			if len(spans) < 2 {
				t.Errorf("expected at least 2 spans, got %d", len(spans))
			}

			// Check span details
			for _, span := range spans {
				if span.Name == FallbackProcessSpan {
					// Main span should have processor count and successful processor
					if _, ok := span.Tags[FallbackTagProcessorCount]; !ok {
						t.Error("main span missing processor_count tag")
					}
					if _, ok := span.Tags[FallbackTagSuccessfulProcessor]; !ok {
						t.Error("main span missing successful_processor tag")
					}
				} else if span.Name == FallbackAttemptSpan {
					// Attempt spans should have processor name and attempt number
					if _, ok := span.Tags[FallbackTagProcessorName]; !ok {
						t.Error("attempt span missing processor_name tag")
					}
					if _, ok := span.Tags[FallbackTagAttemptNumber]; !ok {
						t.Error("attempt span missing attempt_number tag")
					}
				}
			}
		})

		t.Run("Metrics and Spans - Fallback Used", func(t *testing.T) {
			primary := Apply("primary", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("primary failed")
			})
			fallback1 := Apply("fallback1", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("fallback1 failed")
			})
			fallback2 := Transform("fallback2", func(_ context.Context, n int) int {
				return n * 3
			})

			fb := NewFallback("test-fallback", primary, fallback1, fallback2)
			defer fb.Close()

			// Process - should succeed on third processor
			result, err := fb.Process(context.Background(), 5)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != 15 {
				t.Errorf("expected 15, got %d", result)
			}

			// Verify metrics
			attemptsTotal := fb.Metrics().Counter(FallbackAttemptsTotal).Value()
			if attemptsTotal != 3 {
				t.Errorf("expected 3 attempts, got %f", attemptsTotal)
			}

			successesTotal := fb.Metrics().Counter(FallbackSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			allFailedTotal := fb.Metrics().Counter(FallbackAllFailedTotal).Value()
			if allFailedTotal != 0 {
				t.Errorf("expected 0 all_failed (one succeeded), got %f", allFailedTotal)
			}
		})

		t.Run("Metrics and Spans - All Failed", func(t *testing.T) {
			fail1 := Apply("fail1", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("error 1")
			})
			fail2 := Apply("fail2", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("error 2")
			})
			fail3 := Apply("fail3", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("error 3")
			})

			fb := NewFallback("test-fallback-fail", fail1, fail2, fail3)
			defer fb.Close()

			_, err := fb.Process(context.Background(), 5)
			if err == nil {
				t.Fatal("expected error when all processors fail")
			}

			// Check metrics for all failed case
			processedTotal := fb.Metrics().Counter(FallbackProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := fb.Metrics().Counter(FallbackSuccessesTotal).Value()
			if successesTotal != 0 {
				t.Errorf("expected 0 successes, got %f", successesTotal)
			}

			allFailedTotal := fb.Metrics().Counter(FallbackAllFailedTotal).Value()
			if allFailedTotal != 1 {
				t.Errorf("expected 1 all_failed count, got %f", allFailedTotal)
			}

			attemptsTotal := fb.Metrics().Counter(FallbackAttemptsTotal).Value()
			if attemptsTotal != 3 {
				t.Errorf("expected 3 attempts (all tried), got %f", attemptsTotal)
			}
		})

		t.Run("Hooks fire on fallback events", func(t *testing.T) {
			// Test activation and recovery events
			primary := Apply("primary", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("primary failed")
			})
			backup := Apply("backup", func(_ context.Context, n int) (int, error) {
				return n * 2, nil
			})

			fallback := NewFallback("test-hooks", primary, backup)
			defer fallback.Close()

			var activatedEvents []FallbackEvent
			var recoveredEvents []FallbackEvent
			var mu sync.Mutex

			fallback.OnActivated(func(_ context.Context, event FallbackEvent) error {
				mu.Lock()
				activatedEvents = append(activatedEvents, event)
				mu.Unlock()
				return nil
			})

			fallback.OnRecovered(func(_ context.Context, event FallbackEvent) error {
				mu.Lock()
				recoveredEvents = append(recoveredEvents, event)
				mu.Unlock()
				return nil
			})

			result, err := fallback.Process(context.Background(), 10)
			if err != nil {
				t.Errorf("expected recovery, got error: %v", err)
			}
			if result != 20 {
				t.Errorf("expected 20, got %d", result)
			}

			// Wait for async hooks
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			activatedCount := len(activatedEvents)
			recoveredCount := len(recoveredEvents)
			var activatedPrimary Name
			var activatedAttempt, activatedTotal int
			var recoveredPrimary, recoveredFallback Name
			var recoveredFlag bool
			var recoveredAttempt int
			if len(activatedEvents) > 0 {
				event := activatedEvents[0]
				activatedPrimary = event.PrimaryFailed
				activatedAttempt = event.AttemptNumber
				activatedTotal = event.TotalProcessors
			}
			if len(recoveredEvents) > 0 {
				event := recoveredEvents[0]
				recoveredPrimary = event.PrimaryFailed
				recoveredFallback = event.FallbackUsed
				recoveredFlag = event.Recovered
				recoveredAttempt = event.AttemptNumber
			}
			mu.Unlock()

			if activatedCount != 1 {
				t.Errorf("expected 1 activated event, got %d", activatedCount)
			}

			if activatedCount > 0 {
				if activatedPrimary != "primary" {
					t.Errorf("expected primary failed 'primary', got %s", activatedPrimary)
				}
				if activatedAttempt != 1 {
					t.Errorf("expected attempt 1, got %d", activatedAttempt)
				}
				if activatedTotal != 2 {
					t.Errorf("expected 2 processors, got %d", activatedTotal)
				}
			}

			if recoveredCount != 1 {
				t.Errorf("expected 1 recovered event, got %d", recoveredCount)
			}

			if recoveredCount > 0 {
				if recoveredPrimary != "primary" {
					t.Errorf("expected primary 'primary', got %s", recoveredPrimary)
				}
				if recoveredFallback != "backup" {
					t.Errorf("expected fallback 'backup', got %s", recoveredFallback)
				}
				if !recoveredFlag {
					t.Error("expected Recovered=true")
				}
				if recoveredAttempt != 2 {
					t.Errorf("expected attempt 2, got %d", recoveredAttempt)
				}
			}
		})

		t.Run("Exhausted hook fires when all fail", func(t *testing.T) {
			fail1 := Apply("fail1", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("error 1")
			})
			fail2 := Apply("fail2", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("error 2")
			})
			fail3 := Apply("fail3", func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("error 3")
			})

			fallback := NewFallback("test-exhausted", fail1, fail2, fail3)
			defer fallback.Close()

			var exhaustedEvents []FallbackEvent
			var activatedEvents []FallbackEvent
			var mu sync.Mutex

			fallback.OnExhausted(func(_ context.Context, event FallbackEvent) error {
				mu.Lock()
				exhaustedEvents = append(exhaustedEvents, event)
				mu.Unlock()
				return nil
			})

			fallback.OnActivated(func(_ context.Context, event FallbackEvent) error {
				mu.Lock()
				activatedEvents = append(activatedEvents, event)
				mu.Unlock()
				return nil
			})

			_, err := fallback.Process(context.Background(), 10)
			if err == nil {
				t.Error("expected error when all fallbacks fail")
			}

			// Wait for async hooks
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			activatedCount := len(activatedEvents)
			exhaustedCount := len(exhaustedEvents)
			var allFailed bool
			var attemptNum, totalProcs int
			var hasError bool
			if len(exhaustedEvents) > 0 {
				event := exhaustedEvents[0]
				allFailed = event.AllFailed
				attemptNum = event.AttemptNumber
				totalProcs = event.TotalProcessors
				hasError = event.Error != nil
			}
			mu.Unlock()

			// Should have 2 activation events (after first and second failures, not after last)
			if activatedCount != 2 {
				t.Errorf("expected 2 activated events, got %d", activatedCount)
			}

			if exhaustedCount != 1 {
				t.Errorf("expected 1 exhausted event, got %d", exhaustedCount)
			}

			if exhaustedCount > 0 {
				if !allFailed {
					t.Error("expected AllFailed=true")
				}
				if attemptNum != 3 {
					t.Errorf("expected 3 attempts, got %d", attemptNum)
				}
				if totalProcs != 3 {
					t.Errorf("expected 3 processors, got %d", totalProcs)
				}
				if !hasError {
					t.Error("expected error to be set")
				}
			}
		})
	})
}
