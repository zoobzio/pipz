package pipz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Error provides rich context about pipeline execution failures.
// It wraps the underlying error with information about where and when
// the failure occurred, what data was being processed, and the complete
// path through the processing chain.
type Error[T any] struct {
	Timestamp time.Time
	InputData T
	Err       error
	Path      []Name
	Duration  time.Duration
	Timeout   bool
	Canceled  bool
}

// Error implements the error interface, providing a detailed error message.
func (e *Error[T]) Error() string {
	if e == nil {
		return "<nil>"
	}
	path := strings.Join(e.Path, " -> ")
	if path == "" {
		path = "unknown"
	}

	if e.Timeout {
		return fmt.Sprintf("%s timed out after %v: %v", path, e.Duration, e.Err)
	}
	if e.Canceled {
		return fmt.Sprintf("%s canceled after %v: %v", path, e.Duration, e.Err)
	}

	return fmt.Sprintf("%s failed after %v: %v", path, e.Duration, e.Err)
}

// Unwrap returns the underlying error, supporting error wrapping patterns.
// This allows use of errors.Is and errors.As with the underlying error,
// maintaining compatibility with Go's standard error handling patterns.
func (e *Error[T]) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsTimeout returns true if the error was caused by a timeout.
// This includes both explicit timeout from the Timeout connector
// and context deadline exceeded. Useful for implementing timeout-specific
// retry logic or monitoring.
func (e *Error[T]) IsTimeout() bool {
	if e == nil {
		return false
	}
	return e.Timeout || errors.Is(e.Err, context.DeadlineExceeded)
}

// IsCanceled returns true if the error was caused by cancellation.
// This typically indicates intentional termination rather than failure,
// useful for distinguishing between errors that should trigger alerts
// versus expected shutdowns.
func (e *Error[T]) IsCanceled() bool {
	if e == nil {
		return false
	}
	return e.Canceled || errors.Is(e.Err, context.Canceled)
}
