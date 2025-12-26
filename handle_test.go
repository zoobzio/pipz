package pipz

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestHandle(t *testing.T) {
	t.Run("Success Does Not Trigger Handler", func(t *testing.T) {
		handlerCalled := false
		processor := Transform(NewIdentity("success", ""), func(_ context.Context, n int) int {
			return n * 2
		})
		errorHandler := Effect(NewIdentity("error-handler", ""), func(_ context.Context, _ *Error[int]) error {
			handlerCalled = true
			return nil
		})

		handle := NewHandle(NewIdentity("test-handle", "test handle"), processor, errorHandler)
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

		processor := Apply(NewIdentity("failing", ""), func(_ context.Context, _ int) (int, error) {
			return 0, expectedErr
		})
		errorHandler := Effect(NewIdentity("error-handler", ""), func(_ context.Context, err *Error[int]) error {
			capturedErr = err
			return nil
		})

		handle := NewHandle(NewIdentity("test-handle", "test handle"), processor, errorHandler)
		result, err := handle.Process(context.Background(), 5)

		// Error should pass through after handler runs
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected original error, got %v", err)
		}
		// Result should be the zero value from the failed processor
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
		// The handler receives the full *Error[int] with context
		if capturedErr == nil {
			t.Fatal("handler should have received error")
		}
		if !errors.Is(capturedErr.Err, expectedErr) {
			t.Errorf("handler received wrong error: got %v, want %v", capturedErr.Err, expectedErr)
		}
		// Check that path includes both processors
		if len(capturedErr.Path) < 2 || capturedErr.Path[0].Name() != "test-handle" || capturedErr.Path[1].Name() != "failing" {
			t.Errorf("expected path [test-handle failing], got %v", capturedErr.Path)
		}
	})

	t.Run("Handler Error Propagates", func(t *testing.T) {
		processorErr := errors.New("processor failed")
		handlerErr := errors.New("critical error - must fail")

		processor := Apply(NewIdentity("failing", ""), func(_ context.Context, _ int) (int, error) {
			return 0, processorErr
		})
		errorHandler := Apply(NewIdentity("error-handler", ""), func(_ context.Context, err *Error[int]) (*Error[int], error) {
			return err, handlerErr // Handler explicitly fails
		})

		handle := NewHandle(NewIdentity("test-handle", "test handle"), processor, errorHandler)
		result, err := handle.Process(context.Background(), 5)

		// Should get original processor error, not handler error
		if !errors.Is(err, processorErr) {
			t.Fatalf("expected processor error, got %v", err)
		}
		// Result should be the zero value from the failed processor
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("Concurrent Safety", func(t *testing.T) {
		var errorCount int32
		processor := Apply(NewIdentity("sometimes-fail", ""), func(_ context.Context, n int) (int, error) {
			if n%2 == 0 {
				return 0, errors.New("even number")
			}
			return n, nil
		})
		errorHandler := Effect(NewIdentity("count-errors", ""), func(_ context.Context, _ *Error[int]) error {
			atomic.AddInt32(&errorCount, 1)
			return nil
		})

		handle := NewHandle(NewIdentity("test-handle", "test handle"), processor, errorHandler)

		// Run concurrently
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(n int) {
				result, err := handle.Process(context.Background(), n)
				// Errors should pass through
				if n%2 == 0 && err == nil {
					t.Errorf("expected error for even number %d", n)
				} else if n%2 != 0 && err != nil {
					t.Errorf("unexpected error for odd number %d: %v", n, err)
				}
				// Result should be 0 for errors, n for success
				if n%2 == 0 && result != 0 {
					t.Errorf("expected 0 for error case, got %d", result)
				} else if n%2 != 0 && result != n {
					t.Errorf("expected %d, got %d", n, result)
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
		processor1 := Transform(NewIdentity("p1", ""), func(_ context.Context, n int) int { return n })
		processor2 := Transform(NewIdentity("p2", ""), func(_ context.Context, n int) int { return n * 2 })
		handler1 := Effect(NewIdentity("h1", ""), func(_ context.Context, _ *Error[int]) error { return nil })
		handler2 := Effect(NewIdentity("h2", ""), func(_ context.Context, _ *Error[int]) error { return nil })

		handle := NewHandle(NewIdentity("test", ""), processor1, handler1)

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
		processor := Transform(NewIdentity("noop", ""), func(_ context.Context, n int) int { return n })
		errorHandler := Effect(NewIdentity("noop", ""), func(_ context.Context, _ *Error[int]) error { return nil })
		handle := NewHandle(NewIdentity("my-handle", "my handle"), processor, errorHandler)
		if handle.Identity().Name() != "my-handle" {
			t.Errorf("expected 'my-handle', got %q", handle.Identity().Name())
		}
	})

	t.Run("Error Pass Through", func(t *testing.T) {
		expectedErr := errors.New("operation failed")
		processor := Apply(NewIdentity("failing", ""), func(_ context.Context, _ int) (int, error) {
			return 0, expectedErr
		})
		errorHandler := Effect(NewIdentity("log", ""), func(_ context.Context, _ *Error[int]) error {
			// Just log
			return nil
		})

		handle := NewHandle(NewIdentity("observed", "observed handle"), processor, errorHandler)
		result, err := handle.Process(context.Background(), 42)

		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected original error, got %v", err)
		}
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("Handler Failure Ignored", func(t *testing.T) {
		processorErr := errors.New("processor failed")
		handlerErr := errors.New("handler failed")
		processor := Apply(NewIdentity("failing", ""), func(_ context.Context, _ int) (int, error) {
			return 0, processorErr
		})
		errorHandler := Apply(NewIdentity("failing-handler", ""), func(_ context.Context, err *Error[int]) (*Error[int], error) {
			return err, handlerErr
		})

		handle := NewHandle(NewIdentity("test", "test handle"), processor, errorHandler)
		result, err := handle.Process(context.Background(), 42)

		// Should still get processor error, not handler error
		if !errors.Is(err, processorErr) {
			t.Fatalf("expected processor error, got %v", err)
		}
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("Non-Pipeline Error Wrapping", func(t *testing.T) {
		// This test covers lines 82-91 in handle.go where non-Error types are wrapped
		plainErr := errors.New("plain error not wrapped in Error type")
		var capturedErr *Error[int]
		processor := Apply(NewIdentity("plain-error", ""), func(_ context.Context, _ int) (int, error) {
			return 0, plainErr // Return error with zero value, not 42
		})
		errorHandler := Effect(NewIdentity("capture-handler", ""), func(_ context.Context, err *Error[int]) error {
			capturedErr = err
			return nil
		})
		handle := NewHandle(NewIdentity("test-handle", "test handle"), processor, errorHandler)
		result, err := handle.Process(context.Background(), 5)

		// Original error should pass through
		if !errors.Is(err, plainErr) {
			t.Fatalf("expected plain error to pass through, got %v", err)
		}
		if result != 0 {
			t.Errorf("expected result 0 on error, got %d", result)
		}

		// Handler should have been called with wrapped error
		if capturedErr == nil {
			t.Fatal("error handler not called")
		}
		if capturedErr.InputData != 5 {
			t.Errorf("expected input data 5, got %d", capturedErr.InputData)
		}
		if !errors.Is(capturedErr.Err, plainErr) {
			t.Error("wrapped error should contain plain error")
		}
		// Path should include both handle name and processor name
		if len(capturedErr.Path) != 2 {
			t.Errorf("expected path length 2, got %d", len(capturedErr.Path))
		}
		if capturedErr.Path[0].Name() != "test-handle" {
			t.Errorf("expected first path element 'test-handle', got %s", capturedErr.Path[0])
		}
		if capturedErr.Path[1].Name() != "plain-error" {
			t.Errorf("expected second path element 'plain-error', got %s", capturedErr.Path[1])
		}
	})

	t.Run("Pipeline Error Path Extension", func(t *testing.T) {
		// This test covers lines 74-80 in handle.go where *Error[T] types get path extension
		// We need to create a processor that passes pipeline errors through unchanged
		// We'll use a Sequence with a failing processor inside
		expectedErr := errors.New("underlying processor failed")
		var capturedErr *Error[int]

		// Create a failing processor inside a sequence
		failingProcessor := Apply(NewIdentity("inner", ""), func(_ context.Context, _ int) (int, error) {
			return 0, expectedErr
		})

		// Sequence will preserve the pipeline error from the Apply processor
		sequenceProcessor := NewSequence(NewIdentity("sequence", ""), failingProcessor)

		errorHandler := Effect(NewIdentity("capture-pipeline-error", ""), func(_ context.Context, err *Error[int]) error {
			capturedErr = err
			return nil
		})

		handle := NewHandle(NewIdentity("test-handle", ""), sequenceProcessor, errorHandler)
		result, err := handle.Process(context.Background(), 5)

		// Original pipeline error should pass through
		var pipeErr *Error[int]
		if !errors.As(err, &pipeErr) {
			t.Fatalf("expected pipeline error, got %T: %v", err, err)
		}
		if result != 0 {
			t.Errorf("expected result 0 on error, got %d", result)
		}

		// Handler should have been called with the same pipeline error
		if capturedErr == nil {
			t.Fatal("error handler not called")
		}
		if capturedErr.InputData != 5 {
			t.Errorf("expected input data 5, got %d", capturedErr.InputData)
		}
		if !errors.Is(capturedErr.Err, expectedErr) {
			t.Error("pipeline error should contain original error")
		}

		// Path should be extended: [test-handle, sequence, inner]
		// Handle prepends "test-handle" to the sequence's path
		if len(capturedErr.Path) != 3 {
			t.Errorf("expected path length 3, got %d: %v", len(capturedErr.Path), capturedErr.Path)
		}
		if capturedErr.Path[0].Name() != "test-handle" {
			t.Errorf("expected first path element 'test-handle', got %s", capturedErr.Path[0])
		}
		if capturedErr.Path[1].Name() != "sequence" {
			t.Errorf("expected second path element 'sequence', got %s", capturedErr.Path[1])
		}
		if capturedErr.Path[2].Name() != "inner" {
			t.Errorf("expected third path element 'inner', got %s", capturedErr.Path[2])
		}
	})

	t.Run("Success With Non-Zero Result", func(t *testing.T) {
		// This test specifically covers the success path return (line 93) with non-zero result
		handlerCalled := false
		processor := Transform(NewIdentity("multiply", ""), func(_ context.Context, n int) int {
			return n * 42 // Non-zero result
		})
		errorHandler := Effect(NewIdentity("should-not-run", ""), func(_ context.Context, _ *Error[int]) error {
			handlerCalled = true
			return nil
		})

		handle := NewHandle(NewIdentity("success-handle", "success handle"), processor, errorHandler)
		result, err := handle.Process(context.Background(), 3)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 126 { // 3 * 42
			t.Errorf("expected 126, got %d", result)
		}
		if handlerCalled {
			t.Error("error handler should not be called on success")
		}
	})

	t.Run("Handle panic recovery", func(t *testing.T) {
		panicProcessor := Apply(NewIdentity("panic_processor", ""), func(_ context.Context, _ int) (int, error) {
			panic("handle processor panic")
		})

		errorHandler := Effect(NewIdentity("error_handler", ""), func(_ context.Context, _ *Error[int]) error {
			return nil // Just handle the error
		})

		handle := NewHandle(NewIdentity("panic_handle", ""), panicProcessor, errorHandler)
		result, err := handle.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}

		// Check that panic message is properly wrapped
		var panicErr *panicError
		if !errors.As(pipzErr.Err, &panicErr) {
			t.Fatal("expected panicError")
		}

		expectedMsg := "panic occurred: handle processor panic"
		if panicErr.sanitized != expectedMsg {
			t.Errorf("expected %q, got %q", expectedMsg, panicErr.sanitized)
		}
	})

	// High-value coverage improvement tests targeting specific execution paths
	t.Run("Error Handler Execution Path Coverage", func(t *testing.T) {
		t.Run("Pipeline Error Handler Execution", func(t *testing.T) {
			// Test line 79: errorHandler.Process(ctx, pipeErr) for pipeline errors
			var handlerCallCount int32
			var capturedError *Error[int]
			// Create a processor that returns pipeline error (Error[T])
			failingSequence := NewSequence(NewIdentity("failing-seq", ""),
				Apply(NewIdentity("fail", ""), func(_ context.Context, _ int) (int, error) {
					return 0, errors.New("sequence processor failed")
				}))
			errorHandler := Effect(NewIdentity("handler", ""), func(_ context.Context, err *Error[int]) error {
				atomic.AddInt32(&handlerCallCount, 1)
				capturedError = err
				return nil
			})

			handle := NewHandle(NewIdentity("test-handle", ""), failingSequence, errorHandler)
			result, err := handle.Process(context.Background(), 42)

			// Verify error pass-through
			if err == nil {
				t.Fatal("expected error from failing processor")
			}
			if result != 0 {
				t.Errorf("expected zero result on error, got %d", result)
			}

			// Verify handler was called (line 79)
			if atomic.LoadInt32(&handlerCallCount) != 1 {
				t.Errorf("expected handler to be called once, got %d", handlerCallCount)
			}

			// Verify handler received properly structured error
			if capturedError == nil {
				t.Fatal("handler should have received error")
			}
			if capturedError.InputData != 42 {
				t.Errorf("expected input data 42, got %d", capturedError.InputData)
			}
			if len(capturedError.Path) < 3 { // [test-handle, failing-seq, fail]
				t.Errorf("expected path with 3+ elements, got %v", capturedError.Path)
			}
		})

		t.Run("Non-Pipeline Error Handler Execution", func(t *testing.T) {
			// Test line 93: errorHandler.Process(ctx, wrappedErr) for non-pipeline errors
			var handlerCallCount int32
			var capturedError *Error[int]

			plainErr := errors.New("plain error")
			processor := Apply(NewIdentity("plain-fail", ""), func(_ context.Context, _ int) (int, error) {
				return 0, plainErr
			})

			errorHandler := Effect(NewIdentity("wrapper-handler", ""), func(_ context.Context, err *Error[int]) error {
				atomic.AddInt32(&handlerCallCount, 1)
				capturedError = err
				return errors.New("handler processing error") // Handler error should be ignored
			})

			handle := NewHandle(NewIdentity("wrapper-handle", "wrapper handle"), processor, errorHandler)
			result, err := handle.Process(context.Background(), 84)

			// Verify original error passes through despite handler error
			if !errors.Is(err, plainErr) {
				t.Errorf("expected original error, got %v", err)
			}
			if result != 0 {
				t.Errorf("expected zero result on error, got %d", result)
			}

			// Verify handler was called (line 93)
			if atomic.LoadInt32(&handlerCallCount) != 1 {
				t.Errorf("expected handler to be called once, got %d", handlerCallCount)
			}

			// Verify wrapped error structure
			if capturedError == nil {
				t.Fatal("handler should have received wrapped error")
			}
			if capturedError.InputData != 84 {
				t.Errorf("expected input data 84, got %d", capturedError.InputData)
			}
			if !errors.Is(capturedError.Err, plainErr) {
				t.Error("wrapped error should contain original error")
			}
			if len(capturedError.Path) != 2 {
				t.Errorf("expected path [wrapper-handle, plain-fail], got %v", capturedError.Path)
			}
		})
	})

	t.Run("Concurrent Modification Safety", func(t *testing.T) {
		t.Run("Concurrent Processor Swap During Processing", func(t *testing.T) {
			// Test concurrent SetProcessor calls during active Process execution
			slowProcessor := Apply(NewIdentity("slow", ""), func(ctx context.Context, n int) (int, error) {
				select {
				case <-time.After(100 * time.Millisecond):
					return n * 2, nil
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			})

			fastProcessor := Transform(NewIdentity("fast", ""), func(_ context.Context, n int) int {
				return n * 3
			})

			errorHandler := Effect(NewIdentity("noop", ""), func(_ context.Context, _ *Error[int]) error {
				return nil
			})

			handle := NewHandle(NewIdentity("concurrent-test", ""), slowProcessor, errorHandler)

			var wg sync.WaitGroup
			results := make(chan int, 10)
			errs := make(chan error, 10)

			// Start multiple concurrent Process calls
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func(input int) {
					defer wg.Done()
					result, err := handle.Process(context.Background(), input)
					results <- result
					errs <- err
				}(i + 1)
			}

			// Concurrently modify the processor
			time.Sleep(25 * time.Millisecond)
			handle.SetProcessor(fastProcessor)

			wg.Wait()
			close(results)
			close(errs)

			// Verify all operations completed without race conditions
			var resultCount, errorCount int
			for result := range results {
				if result > 0 {
					resultCount++
				}
			}
			for err := range errs {
				if err != nil {
					errorCount++
				}
			}

			// Should have 5 results with no data races
			if resultCount != 5 {
				t.Errorf("expected 5 results, got %d", resultCount)
			}
		})

		t.Run("Concurrent Handler Modification", func(t *testing.T) {
			// Test concurrent SetErrorHandler during error processing
			failingProcessor := Apply(NewIdentity("fail", ""), func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("processor error")
			})

			var handler1Calls, handler2Calls int32
			handler1 := Effect(NewIdentity("handler1", ""), func(_ context.Context, _ *Error[int]) error {
				atomic.AddInt32(&handler1Calls, 1)
				time.Sleep(50 * time.Millisecond) // Slow handler
				return nil
			})

			handler2 := Effect(NewIdentity("handler2", ""), func(_ context.Context, _ *Error[int]) error {
				atomic.AddInt32(&handler2Calls, 1)
				return nil
			})

			handle := NewHandle(NewIdentity("handler-swap", ""), failingProcessor, handler1)

			var wg sync.WaitGroup

			// Start processing
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, _ = handle.Process(context.Background(), 42)
				}()
			}

			// Swap handler during processing
			time.Sleep(25 * time.Millisecond)
			handle.SetErrorHandler(handler2)

			wg.Wait()

			// Both handlers should have been called
			total := atomic.LoadInt32(&handler1Calls) + atomic.LoadInt32(&handler2Calls)
			if total != 3 {
				t.Errorf("expected 3 total handler calls, got %d", total)
			}
		})
	})

	t.Run("Name Concurrent Access", func(t *testing.T) {
		processor := Transform(NewIdentity("noop", ""), func(_ context.Context, n int) int { return n })
		errorHandler := Effect(NewIdentity("noop", ""), func(_ context.Context, _ *Error[int]) error { return nil })
		handle := NewHandle(NewIdentity("concurrent-name-test", "concurrent name test handle"), processor, errorHandler)

		var wg sync.WaitGroup
		results := make(chan string, 100)

		// Concurrent Name() calls
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				name := handle.Identity().Name()
				results <- name
			}()
		}

		// Concurrent processing
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(input int) {
				defer wg.Done()
				_, _ = handle.Process(context.Background(), input)
				name := handle.Identity().Name() // Name during processing
				results <- name
			}(i)
		}

		wg.Wait()
		close(results)

		// All Name() calls should return consistent value
		count := 0
		for name := range results {
			if name != "concurrent-name-test" {
				t.Errorf("expected 'concurrent-name-test', got %q", name)
			}
			count++
		}

		if count != 100 {
			t.Errorf("expected 100 name results, got %d", count)
		}
	})

	t.Run("Non-Pipeline Error Processor Name Access Coverage", func(t *testing.T) {
		// Test lines 84-86: h.mu.RLock() processorName := h.processor.Name() h.mu.RUnlock()
		// This path is taken when handling non-pipeline errors
		plainErr := errors.New("plain error")

		// Create a processor that returns a plain error (not Error[T])
		fakeProcessor := &plainErrorProcessorHandle[int]{
			identity: NewIdentity("name-access-test", ""),
			err:      plainErr,
		}

		var capturedError *Error[int]
		errorHandler := Effect(NewIdentity("capture", ""), func(_ context.Context, err *Error[int]) error {
			capturedError = err
			return nil
		})

		handle := NewHandle(NewIdentity("test-handle", ""), fakeProcessor, errorHandler)

		// Process should trigger non-pipeline error path and access processor name
		result, err := handle.Process(context.Background(), 42)

		if !errors.Is(err, plainErr) {
			t.Errorf("expected original error, got %v", err)
		}
		if result != 0 {
			t.Errorf("expected zero result, got %d", result)
		}

		// Verify that processor name was accessed (lines 84-86) and included in path
		if capturedError == nil {
			t.Fatal("error handler should have been called")
		}
		if len(capturedError.Path) != 2 {
			t.Errorf("expected path length 2, got %d: %v", len(capturedError.Path), capturedError.Path)
		}
		if capturedError.Path[1].Name() != "name-access-test" {
			t.Errorf("expected processor name in path, got %v", capturedError.Path)
		}
	})
}

// plainErrorProcessorHandle for Handle tests - returns plain errors (not Error[T] types).
type plainErrorProcessorHandle[T any] struct {
	identity Identity
	err      error
}

func (p *plainErrorProcessorHandle[T]) Process(_ context.Context, _ T) (T, error) {
	var zero T
	return zero, p.err
}

func (p *plainErrorProcessorHandle[T]) Identity() Identity {
	return p.identity
}

func (p *plainErrorProcessorHandle[T]) Schema() Node {
	return Node{
		Identity: p.identity,
		Type:     "plain_error_processor_handle",
	}
}

func (*plainErrorProcessorHandle[T]) Close() error {
	return nil
}

func TestHandleClose(t *testing.T) {
	t.Run("Closes Both Processors", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("processor", ""))
		h := newTrackingProcessor[*Error[int]](NewIdentity("handler", ""))

		handle := NewHandle(NewIdentity("test", ""), p, h)
		err := handle.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected processor close call, got %d", p.CloseCalls())
		}
		if h.CloseCalls() != 1 {
			t.Errorf("expected handler close call, got %d", h.CloseCalls())
		}
	})

	t.Run("Aggregates Errors", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("processor", "")).WithCloseError(errors.New("p error"))
		h := newTrackingProcessor[*Error[int]](NewIdentity("handler", "")).WithCloseError(errors.New("h error"))

		handle := NewHandle(NewIdentity("test", ""), p, h)
		err := handle.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p.CloseCalls() != 1 || h.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("processor", ""))
		h := newTrackingProcessor[*Error[int]](NewIdentity("handler", ""))
		handle := NewHandle(NewIdentity("test", ""), p, h)

		_ = handle.Close()
		_ = handle.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 processor close call, got %d", p.CloseCalls())
		}
		if h.CloseCalls() != 1 {
			t.Errorf("expected 1 handler close call, got %d", h.CloseCalls())
		}
	})
}

func TestHandleSignals(t *testing.T) {
	t.Run("Emits Error Handled Signal On Failure", func(t *testing.T) {
		var signalReceived bool
		var signalName string
		var signalError string

		listener := capitan.Hook(SignalHandleErrorHandled, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalName, _ = FieldName.From(e)
			signalError, _ = FieldError.From(e)
		})
		defer listener.Close()

		failingProcessor := Apply(NewIdentity("failing", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("intentional failure")
		})
		errorHandler := Effect(NewIdentity("handler", ""), func(_ context.Context, _ *Error[int]) error {
			return nil
		})

		handle := NewHandle(NewIdentity("signal-test-handle", ""), failingProcessor, errorHandler)

		_, err := handle.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error")
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalName != "signal-test-handle" {
			t.Errorf("expected name 'signal-test-handle', got %q", signalName)
		}
		if signalError != "intentional failure" {
			t.Errorf("expected error 'intentional failure', got %q", signalError)
		}
	})

	t.Run("Does Not Emit Signal On Success", func(t *testing.T) {
		var signalReceived bool

		listener := capitan.Hook(SignalHandleErrorHandled, func(_ context.Context, _ *capitan.Event) {
			signalReceived = true
		})
		defer listener.Close()

		successProcessor := Transform(NewIdentity("success", ""), func(_ context.Context, n int) int {
			return n * 2
		})
		errorHandler := Effect(NewIdentity("handler", ""), func(_ context.Context, _ *Error[int]) error {
			return nil
		})

		handle := NewHandle(NewIdentity("signal-success-handle", ""), successProcessor, errorHandler)

		_, err := handle.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if signalReceived {
			t.Error("signal should not be emitted on success")
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc := Transform(NewIdentity("processor", ""), func(_ context.Context, n int) int { return n })
		handler := Transform(NewIdentity("error-handler", ""), func(_ context.Context, e *Error[int]) *Error[int] { return e })

		handle := NewHandle(NewIdentity("test-handle", "Handle connector"), proc, handler)

		schema := handle.Schema()

		if schema.Identity.Name() != "test-handle" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-handle")
		}
		if schema.Type != "handle" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "handle")
		}

		flow, ok := HandleKey.From(schema)
		if !ok {
			t.Fatal("Expected HandleFlow")
		}
		if flow.Processor.Identity.Name() != "processor" {
			t.Errorf("Flow.Processor.Identity.Name() = %v, want %v", flow.Processor.Identity.Name(), "processor")
		}
		if flow.ErrorHandler.Identity.Name() != "error-handler" {
			t.Errorf("Flow.ErrorHandler.Identity.Name() = %v, want %v", flow.ErrorHandler.Identity.Name(), "error-handler")
		}
	})
}
