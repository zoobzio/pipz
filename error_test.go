package pipz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestError(t *testing.T) {
	t.Run("Error Message Formatting", func(t *testing.T) {
		baseErr := errors.New("something went wrong")

		t.Run("Basic Error", func(t *testing.T) {
			err := &Error[string]{
				Err:       baseErr,
				Path:      []string{"sequence", "validate"},
				InputData: "test data",
				Duration:  100 * time.Millisecond,
				Timestamp: time.Now(),
			}

			msg := err.Error()
			if !strings.Contains(msg, "sequence -> validate") {
				t.Errorf("expected path elements joined in error, got: %s", msg)
			}
			if !strings.Contains(msg, "failed after 100ms") {
				t.Errorf("expected duration in error, got: %s", msg)
			}
			if !strings.Contains(msg, "something went wrong") {
				t.Errorf("expected base error in message, got: %s", msg)
			}
		})

		t.Run("Connector With Processor Error", func(t *testing.T) {
			err := &Error[string]{
				Err:       baseErr,
				Path:      []string{"pipeline", "transform"},
				InputData: "test",
				Duration:  50 * time.Millisecond,
				Timestamp: time.Now(),
			}

			msg := err.Error()
			if !strings.Contains(msg, "pipeline -> transform") {
				t.Errorf("expected path elements joined in error, got: %s", msg)
			}
			if !strings.Contains(msg, "failed after 50ms") {
				t.Errorf("expected duration in error, got: %s", msg)
			}
		})

		t.Run("Timeout Error", func(t *testing.T) {
			err := &Error[string]{
				Err:       context.DeadlineExceeded,
				Path:      []string{"api", "slow_process"},
				InputData: "data",
				Timeout:   true,
				Duration:  5 * time.Second,
				Timestamp: time.Now(),
			}

			msg := err.Error()
			if !strings.Contains(msg, "api -> slow_process timed out after 5s") {
				t.Errorf("expected timeout message, got: %s", msg)
			}
		})

		t.Run("Canceled Error", func(t *testing.T) {
			err := &Error[string]{
				Err:       context.Canceled,
				Path:      []string{"worker", "process"},
				InputData: "data",
				Canceled:  true,
				Duration:  200 * time.Millisecond,
				Timestamp: time.Now(),
			}

			msg := err.Error()
			if !strings.Contains(msg, "worker -> process canceled after 200ms") {
				t.Errorf("expected canceled message, got: %s", msg)
			}
		})

		t.Run("Single Path Element Error", func(t *testing.T) {
			err := &Error[string]{
				Err:       baseErr,
				Path:      []string{"http"},
				InputData: "request data",
				Duration:  75 * time.Millisecond,
				Timestamp: time.Now(),
			}

			msg := err.Error()
			if !strings.Contains(msg, "http failed after 75ms") {
				t.Errorf("expected single path element error format, got: %s", msg)
			}
			if strings.Contains(msg, " -> ") {
				t.Errorf("should not contain arrow when only one path element, got: %s", msg)
			}
		})
	})

	t.Run("Unwrap", func(t *testing.T) {
		baseErr := errors.New("base error")
		pipelineErr := &Error[int]{
			Err:       baseErr,
			Path:      []string{"pipeline", "test"},
			InputData: 42,
			Timestamp: time.Now(),
		}

		unwrapped := pipelineErr.Unwrap()
		if unwrapped != baseErr { //nolint:errorlint // Unwrap() returns the exact error, not wrapped
			t.Errorf("Unwrap() should return base error")
		}

		// Test with errors.Is
		if !errors.Is(pipelineErr, baseErr) {
			t.Errorf("errors.Is should work with wrapped error")
		}
	})

	t.Run("IsTimeout", func(t *testing.T) {
		tests := []struct {
			err      error
			name     string
			timeout  bool
			expected bool
		}{
			{
				name:     "explicit timeout flag",
				err:      errors.New("some error"),
				timeout:  true,
				expected: true,
			},
			{
				name:     "deadline exceeded error",
				err:      context.DeadlineExceeded,
				timeout:  false,
				expected: true,
			},
			{
				name:     "wrapped deadline exceeded",
				err:      fmt.Errorf("wrapper: %w", context.DeadlineExceeded),
				timeout:  false,
				expected: true,
			},
			{
				name:     "regular error",
				err:      errors.New("regular error"),
				timeout:  false,
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := &Error[string]{
					Err:       tt.err,
					Timeout:   tt.timeout,
					Path:      []string{"test"},
					Timestamp: time.Now(),
				}

				if got := err.IsTimeout(); got != tt.expected {
					t.Errorf("IsTimeout() = %v, want %v", got, tt.expected)
				}
			})
		}
	})

	t.Run("IsCanceled", func(t *testing.T) {
		tests := []struct {
			err      error
			name     string
			canceled bool
			expected bool
		}{
			{
				name:     "explicit canceled flag",
				err:      errors.New("some error"),
				canceled: true,
				expected: true,
			},
			{
				name:     "context canceled error",
				err:      context.Canceled,
				canceled: false,
				expected: true,
			},
			{
				name:     "wrapped canceled",
				err:      fmt.Errorf("wrapper: %w", context.Canceled),
				canceled: false,
				expected: true,
			},
			{
				name:     "regular error",
				err:      errors.New("regular error"),
				canceled: false,
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := &Error[string]{
					Err:       tt.err,
					Canceled:  tt.canceled,
					Path:      []string{"test"},
					Timestamp: time.Now(),
				}

				if got := err.IsCanceled(); got != tt.expected {
					t.Errorf("IsCanceled() = %v, want %v", got, tt.expected)
				}
			})
		}
	})

	t.Run("Type Safety", func(t *testing.T) {
		// Test with different types
		t.Run("String Type", func(t *testing.T) {
			err := &Error[string]{
				Err:       errors.New("failed"),
				Path:      []string{"test", "string_processor"},
				InputData: "hello world",
				Timestamp: time.Now(),
			}

			// Should be able to access typed InputData
			if err.InputData != "hello world" {
				t.Errorf("InputData should preserve type")
			}
		})

		t.Run("Struct Type", func(t *testing.T) {
			type User struct {
				Name string
				Age  int
			}

			user := User{Name: "Alice", Age: 30}
			err := &Error[User]{
				Err:       errors.New("failed"),
				Path:      []string{"test", "user_processor"},
				InputData: user,
				Timestamp: time.Now(),
			}

			// Should be able to access typed fields
			if err.InputData.Name != "Alice" {
				t.Errorf("InputData should preserve struct fields")
			}
			if err.InputData.Age != 30 {
				t.Errorf("InputData should preserve struct fields")
			}
		})
	})

	t.Run("Zero Values", func(t *testing.T) {
		// Test with minimal/zero values
		err := &Error[int]{
			Err:       errors.New("error"),
			Timestamp: time.Now(),
		}

		msg := err.Error()
		if !strings.Contains(msg, "unknown failed after 0s") {
			t.Errorf("should handle zero duration and empty path, got: %s", msg)
		}
	})

	t.Run("Nil Receiver", func(t *testing.T) {
		var err *Error[string]

		// Error() should handle nil receiver
		if err.Error() != "<nil>" {
			t.Errorf("nil error should return '<nil>', got: %s", err.Error())
		}

		// Unwrap() should handle nil receiver
		if err.Unwrap() != nil {
			t.Error("nil error Unwrap should return nil")
		}

		// IsTimeout() should handle nil receiver
		if err.IsTimeout() {
			t.Error("nil error IsTimeout should return false")
		}

		// IsCanceled() should handle nil receiver
		if err.IsCanceled() {
			t.Error("nil error IsCanceled should return false")
		}
	})

	t.Run("PanicError", func(t *testing.T) {
		t.Run("panicError implements error", func(t *testing.T) {
			pe := &panicError{
				processorName: "test_proc",
				sanitized:     "test panic message",
			}

			expected := `panic in processor "test_proc": test panic message`
			if pe.Error() != expected {
				t.Errorf("expected %q, got %q", expected, pe.Error())
			}
		})
	})

	t.Run("PanicMessageSanitization", func(t *testing.T) {
		testCases := []struct {
			name     string
			panic    interface{}
			expected string
		}{
			{
				name:     "simple string panic",
				panic:    "simple error",
				expected: "panic occurred: simple error",
			},
			{
				name:     "nil panic",
				panic:    nil,
				expected: "unknown panic (nil value)",
			},
			{
				name:     "memory address sanitization",
				panic:    "error at 0x1234567890abcdef",
				expected: "panic occurred: error at 0x***",
			},
			{
				name:     "file path sanitization",
				panic:    "/sensitive/path/file.go:123 error",
				expected: "panic occurred (file path sanitized)",
			},
			{
				name:     "windows path sanitization",
				panic:    "C:\\sensitive\\path\\file.go:123 error",
				expected: "panic occurred (file path sanitized)",
			},
			{
				name:     "long message truncation",
				panic:    strings.Repeat("a", 250),
				expected: "panic occurred (message truncated for security)",
			},
			{
				name:     "stack trace sanitization",
				panic:    "error\ngoroutine 1 [running]:\nruntime.main()",
				expected: "panic occurred (stack trace sanitized)",
			},
			{
				name:     "runtime function sanitization",
				panic:    "runtime.doPanic called",
				expected: "panic occurred (stack trace sanitized)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				sanitized := sanitizePanicMessage(tc.panic)
				if sanitized != tc.expected {
					t.Errorf("expected %q, got %q", tc.expected, sanitized)
				}
			})
		}
	})
}
