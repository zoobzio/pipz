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
func (e *PipelineError[T]) Unwrap() error {
	return e.Err
}

// IsTimeout returns true if the error was caused by a timeout.
func (e *PipelineError[T]) IsTimeout() bool {
	return e.Timeout || errors.Is(e.Err, context.DeadlineExceeded)
}

// IsCanceled returns true if the error was caused by cancellation.
func (e *PipelineError[T]) IsCanceled() bool {
	return e.Canceled || errors.Is(e.Err, context.Canceled)
}
