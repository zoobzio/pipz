package pipz

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PipelineError provides rich context about pipeline execution failures.
// It wraps the underlying error with information about where and when
// the failure occurred, what data was being processed, and whether
// the failure was due to timeout or cancellation.
type PipelineError[T any] struct {
	InputData     T
	Timestamp     time.Time
	Err           error
	ProcessorName string
	Duration      time.Duration
	StageIndex    int
	Timeout       bool
	Canceled      bool
}

// Error implements the error interface, providing a detailed error message.
// The message includes the processor name, stage index, duration, and specific
// timeout/cancellation status to aid in debugging pipeline failures.
func (e *PipelineError[T]) Error() string {
	location := fmt.Sprintf("processor %q (stage %d)", e.ProcessorName, e.StageIndex)

	if e.Timeout {
		return fmt.Sprintf("%s timed out after %v: %v", location, e.Duration, e.Err)
	}
	if e.Canceled {
		return fmt.Sprintf("%s canceled after %v: %v", location, e.Duration, e.Err)
	}
	return fmt.Sprintf("%s failed after %v: %v", location, e.Duration, e.Err)
}

// Unwrap returns the underlying error, supporting error wrapping patterns.
// This allows use of errors.Is and errors.As with the underlying error,
// maintaining compatibility with Go's standard error handling patterns.
func (e *PipelineError[T]) Unwrap() error {
	return e.Err
}

// IsTimeout returns true if the error was caused by a timeout.
// This includes both explicit timeout from the Timeout connector
// and context deadline exceeded. Useful for implementing timeout-specific
// retry logic or monitoring.
func (e *PipelineError[T]) IsTimeout() bool {
	return e.Timeout || errors.Is(e.Err, context.DeadlineExceeded)
}

// IsCanceled returns true if the error was caused by cancellation.
// This typically indicates intentional termination rather than failure,
// useful for distinguishing between errors that should trigger alerts
// versus expected shutdowns.
func (e *PipelineError[T]) IsCanceled() bool {
	return e.Canceled || errors.Is(e.Err, context.Canceled)
}
