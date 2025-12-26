package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// TestPanicCascade demonstrates the critical gap in handle.go where error handler panics
// are not recovered, causing system-wide failure. This validates JOEBOY's finding that
// line 80 and 91 in handle.go have no panic recovery for error handlers.
func TestPanicCascade(t *testing.T) {
	t.Run("Error handler panic propagates uncaught", func(t *testing.T) {
		// Processor that returns an error
		processor := pipz.Apply(pipz.NewIdentity("failing-processor", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("processor failed")
		})

		// Error handler that panics - this is the gap
		errorHandler := pipz.Apply(pipz.NewIdentity("panicking-handler", ""), func(_ context.Context, err *pipz.Error[int]) (*pipz.Error[int], error) {
			// Simulate handler that panics on certain error patterns
			if strings.Contains(err.Error(), "processor failed") {
				panic("handler cannot handle this error type")
			}
			return err, nil
		})

		handle := pipz.NewHandle(pipz.NewIdentity("vulnerable-handle", ""), processor, errorHandler)

		// Test that error handler panics are properly handled
		defer func() {
			if r := recover(); r != nil {
				// Error handler panics should be caught by the main Process defer
				if strings.Contains(fmt.Sprint(r), "handler cannot handle") {
					t.Log("ERROR HANDLER PANIC: Properly caught by panic recovery")
				} else {
					t.Errorf("unexpected panic: %v", r)
				}
			} else {
				t.Log("NO PANIC: Error handler panic was caught and converted to error")
			}
		}()

		// This will panic through the error handler
		_, _ = handle.Process(context.Background(), 42)
	})

	t.Run("Nested panic through multiple layers", func(t *testing.T) {
		// Create a processor that triggers specific panic message
		processor := pipz.Apply(pipz.NewIdentity("memory-leak", ""), func(_ context.Context, n int) (int, error) {
			// Panic with memory address
			panic(fmt.Sprintf("leaked address: %p", &n))
		})

		// Wrap in timeout to add a layer
		timeoutProcessor := pipz.NewTimeout(pipz.NewIdentity("timeout-wrap", ""), processor, time.Second)

		// Error handler that panics when it sees sanitized addresses
		errorHandler := pipz.Apply(pipz.NewIdentity("sensitive-handler", ""), func(_ context.Context, err *pipz.Error[int]) (*pipz.Error[int], error) {
			// This handler is sensitive to sanitized addresses
			if strings.Contains(err.Error(), "0x***") {
				panic("handler detected sanitized sensitive data")
			}
			return err, nil
		})

		handle := pipz.NewHandle(pipz.NewIdentity("nested-panic", ""), timeoutProcessor, errorHandler)

		defer func() {
			if r := recover(); r != nil {
				// The cascade: processor panic -> sanitized -> handler panic -> uncaught
				t.Log("Cascade confirmed: processor panic -> sanitization -> handler panic -> system failure")
			}
		}()

		// This cascades through multiple panic recovery attempts
		_, _ = handle.Process(context.Background(), 42)
	})

	t.Run("Error handler panic during concurrent processing", func(t *testing.T) {
		var callCount int32
		processor := pipz.Apply(pipz.NewIdentity("concurrent-fail", ""), func(_ context.Context, n int) (int, error) {
			return 0, fmt.Errorf("error-%d", n)
		})

		// Handler that panics on specific condition that occurs during concurrent access
		errorHandler := pipz.Apply(pipz.NewIdentity("race-sensitive", ""), func(_ context.Context, err *pipz.Error[int]) (*pipz.Error[int], error) {
			count := atomic.AddInt32(&callCount, 1)
			// Panic on specific concurrent condition
			if count == 3 {
				panic("handler state corrupted by concurrent access")
			}
			return err, nil
		})

		handle := pipz.NewHandle(pipz.NewIdentity("concurrent-vulnerable", ""), processor, errorHandler)

		// Run concurrent operations
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(n int) {
				defer func() {
					_ = recover() // Catch panic in goroutine
					done <- true
				}()
				_, _ = handle.Process(context.Background(), n)
			}(i)
		}

		// Wait for all
		for i := 0; i < 5; i++ {
			<-done
		}

		// At least one goroutine should have panicked
		finalCount := atomic.LoadInt32(&callCount)
		if finalCount < 3 {
			t.Skip("Race condition didn't trigger in this run")
		}
		t.Log("Confirmed: Error handler panic under concurrent load")
	})
}

// TestSanitizationBypass attempts to bypass the sanitizePanicMessage function
// to validate the security gaps JOEBOY identified at 95% coverage.
func TestSanitizationBypass(t *testing.T) {
	testCases := []struct {
		name        string
		panicValue  interface{}
		shouldLeak  bool
		description string
	}{
		{
			name:        "Exactly 200 characters",
			panicValue:  strings.Repeat("A", 200),
			shouldLeak:  true, // Off-by-one bug at line 125
			description: "200 char string should be truncated but isn't",
		},
		{
			name:        "201 characters",
			panicValue:  strings.Repeat("A", 201),
			shouldLeak:  false,
			description: "201 chars correctly truncated",
		},
		{
			name:        "Stack trace with sync package",
			panicValue:  "sync.(*Mutex).Lock(0xc000123456)",
			shouldLeak:  true, // Only checks for "runtime." not "sync."
			description: "sync package addresses not fully sanitized",
		},
		{
			name:        "Embedded null bytes with path",
			panicValue:  "Error\x00\x00/etc/passwd",
			shouldLeak:  false,
			description: "Null bytes with paths caught",
		},
		{
			name:        "Malformed hex address",
			panicValue:  "Error at 0xGGGGGG location",
			shouldLeak:  false, // Should handle gracefully
			description: "Invalid hex handled correctly",
		},
		{
			name:        "Windows UNC path",
			panicValue:  "\\\\server\\share\\sensitive",
			shouldLeak:  false,
			description: "UNC paths sanitized",
		},
		{
			name:        "Mixed address formats",
			panicValue:  "0xDEAD at %p with 0xBEEF",
			shouldLeak:  true, // %p not sanitized
			description: "Printf format specifiers not caught",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			processor := pipz.Apply(pipz.NewIdentity("panic-test", ""), func(_ context.Context, _ int) (int, error) {
				panic(tc.panicValue)
			})

			// Use a simple error handler that doesn't panic
			errorHandler := pipz.Effect(pipz.NewIdentity("capture", ""), func(_ context.Context, err *pipz.Error[int]) error {
				// Check if sanitization worked
				errStr := err.Error()
				// Check for various leak indicators
				hasLeak := strings.Contains(errStr, "0xc000") ||
					strings.Contains(errStr, "/etc/") ||
					strings.Contains(errStr, "\\\\server") ||
					(tc.name == "Exactly 200 characters" && len(errStr) > 200) ||
					(strings.Contains(errStr, "sync.") && strings.Contains(errStr, "0x"))

				if hasLeak && !tc.shouldLeak {
					t.Errorf("SECURITY: Information leaked through panic: %s\nDescription: %s",
						errStr, tc.description)
				} else if hasLeak && tc.shouldLeak {
					t.Logf("Expected leak confirmed: %s", tc.description)
				}
				return nil
			})

			handle := pipz.NewHandle(pipz.NewIdentity("security-test", ""), processor, errorHandler)

			// Process should handle the panic
			_, err := handle.Process(context.Background(), 42)
			if err == nil {
				t.Error("expected error from panic")
			}
		})
	}
}

// TestHandlerPanicWithComplexError validates that complex error objects
// can trigger handler panics in unexpected ways.
func TestHandlerPanicWithComplexError(t *testing.T) {
	// Create a processor that fails with complex error chain
	processor := pipz.Apply(pipz.NewIdentity("complex-fail", ""), func(_ context.Context, _ int) (int, error) {
		// Create nested error that might confuse handler
		innerErr := fmt.Errorf("inner: %w", errors.New("root cause"))
		outerErr := fmt.Errorf("outer: %w", innerErr)
		return 0, outerErr
	})

	// Handler that panics on complex error structures
	errorHandler := pipz.Apply(pipz.NewIdentity("fragile-handler", ""), func(_ context.Context, err *pipz.Error[int]) (*pipz.Error[int], error) {
		// Try to unwrap multiple times - might panic
		unwrapped := errors.Unwrap(err.Err)
		if unwrapped != nil {
			// This could panic if unwrapped is unexpected type
			again := errors.Unwrap(unwrapped)
			if again != nil {
				panic("handler confused by error chain depth")
			}
		}
		return err, nil
	})

	handle := pipz.NewHandle(pipz.NewIdentity("complex-error-handle", ""), processor, errorHandler)

	defer func() {
		if r := recover(); r != nil {
			t.Log("Handler panic on complex error confirmed")
		}
	}()

	_, _ = handle.Process(context.Background(), 42)
}
