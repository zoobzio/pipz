package pipz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPipelineError(t *testing.T) {
	t.Run("Error Message Formatting", func(t *testing.T) {
		baseErr := errors.New("something went wrong")

		t.Run("Basic Error", func(t *testing.T) {
			err := &PipelineError[string]{
				Err:           baseErr,
				ProcessorName: "validate",
				StageIndex:    0,
				InputData:     "test data",
				Duration:      100 * time.Millisecond,
			}

			msg := err.Error()
			if !strings.Contains(msg, "processor \"validate\"") {
				t.Errorf("expected processor name in error, got: %s", msg)
			}
			if !strings.Contains(msg, "(stage 0)") {
				t.Errorf("expected stage index in error, got: %s", msg)
			}
			if !strings.Contains(msg, "failed after 100ms") {
				t.Errorf("expected duration in error, got: %s", msg)
			}
			if !strings.Contains(msg, "something went wrong") {
				t.Errorf("expected base error in message, got: %s", msg)
			}
		})

		t.Run("Chain Error", func(t *testing.T) {
			err := &PipelineError[string]{
				Err:           baseErr,
				ProcessorName: "transform",
				StageIndex:    2,
				InputData:     "test",
				Duration:      50 * time.Millisecond,
			}

			msg := err.Error()
			if !strings.Contains(msg, "processor \"transform\"") {
				t.Errorf("expected processor name in error, got: %s", msg)
			}
			if !strings.Contains(msg, "(stage 2)") {
				t.Errorf("expected stage index in error, got: %s", msg)
			}
		})

		t.Run("Timeout Error", func(t *testing.T) {
			err := &PipelineError[string]{
				Err:           context.DeadlineExceeded,
				ProcessorName: "slow_process",
				StageIndex:    1,
				InputData:     "data",
				Timeout:       true,
				Duration:      5 * time.Second,
			}

			msg := err.Error()
			if !strings.Contains(msg, "timed out after 5s") {
				t.Errorf("expected timeout message, got: %s", msg)
			}
		})

		t.Run("Canceled Error", func(t *testing.T) {
			err := &PipelineError[string]{
				Err:           context.Canceled,
				ProcessorName: "process",
				StageIndex:    0,
				InputData:     "data",
				Canceled:      true,
				Duration:      200 * time.Millisecond,
			}

			msg := err.Error()
			if !strings.Contains(msg, "canceled after 200ms") {
				t.Errorf("expected canceled message, got: %s", msg)
			}
		})
	})

	t.Run("Unwrap", func(t *testing.T) {
		baseErr := errors.New("base error")
		pipelineErr := &PipelineError[int]{
			Err:           baseErr,
			ProcessorName: "test",
			StageIndex:    0,
			InputData:     42,
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
			name     string
			err      error
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
				err := &PipelineError[string]{
					Err:     tt.err,
					Timeout: tt.timeout,
				}

				if got := err.IsTimeout(); got != tt.expected {
					t.Errorf("IsTimeout() = %v, want %v", got, tt.expected)
				}
			})
		}
	})

	t.Run("IsCanceled", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
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
				err := &PipelineError[string]{
					Err:      tt.err,
					Canceled: tt.canceled,
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
			err := &PipelineError[string]{
				Err:           errors.New("failed"),
				ProcessorName: "string_processor",
				InputData:     "hello world",
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
			err := &PipelineError[User]{
				Err:           errors.New("failed"),
				ProcessorName: "user_processor",
				InputData:     user,
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
		err := &PipelineError[int]{
			Err: errors.New("error"),
		}

		msg := err.Error()
		if !strings.Contains(msg, "processor \"\"") {
			t.Errorf("should handle empty processor name")
		}
		if !strings.Contains(msg, "(stage 0)") {
			t.Errorf("should handle zero stage index")
		}
		if !strings.Contains(msg, "failed after 0s") {
			t.Errorf("should handle zero duration")
		}
	})
}
