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

// panicError is a security-focused panic recovery error.
// It represents a panic that occurred during processing, with sensitive
// information sanitized to prevent information leakage through panic messages.
type panicError struct {
	processorName Name
	sanitized     string
}

func (pe *panicError) Error() string {
	return fmt.Sprintf("panic in processor %q: %s", pe.processorName, pe.sanitized)
}

// sanitizePanicMessage removes potentially sensitive information from panic messages.
// This prevents accidental exposure of internal details, memory addresses, or
// other sensitive data that might be contained in panic messages.
func sanitizePanicMessage(panicValue interface{}) string {
	if panicValue == nil {
		return "unknown panic (nil value)"
	}

	msg := fmt.Sprintf("%v", panicValue)

	// Remove memory addresses (replace hex digits after 0x)
	for strings.Contains(msg, "0x") {
		start := strings.Index(msg, "0x")
		if start == -1 {
			break
		}
		end := start + 2
		// Find end of hex address
		for end < len(msg) && ((msg[end] >= '0' && msg[end] <= '9') ||
			(msg[end] >= 'a' && msg[end] <= 'f') ||
			(msg[end] >= 'A' && msg[end] <= 'F')) {
			end++
		}
		if end > start+2 {
			msg = msg[:start] + "0x***" + msg[end:]
		} else {
			break
		}
	}

	// Remove file paths that might contain sensitive directory names
	if strings.Contains(msg, "/") || strings.Contains(msg, "\\") {
		return "panic occurred (file path sanitized)"
	}

	// If message is very long, truncate to prevent excessive log spam
	if len(msg) > 200 {
		return "panic occurred (message truncated for security)"
	}

	// Remove any potential stack trace information and package addresses
	if strings.Contains(msg, "goroutine") || strings.Contains(msg, "runtime.") ||
		strings.Contains(msg, "sync.") || strings.Contains(msg, "net.") ||
		strings.Contains(msg, "os.") || strings.Contains(msg, "fmt.") ||
		strings.Contains(msg, "io.") {
		return "panic occurred (stack trace sanitized)"
	}

	return fmt.Sprintf("panic occurred: %s", msg)
}

// recoverFromPanic provides security-focused panic recovery for Process methods.
// It captures panics, sanitizes the panic message to prevent information leakage,
// and converts them to proper Error[T] instances.
//
// This function should be used as a deferred call at the beginning of all Process methods:
//
//	defer recoverFromPanic(&result, &err, processorName, inputData)
func recoverFromPanic[T any](result *T, err *error, processorName Name, inputData T) {
	if r := recover(); r != nil {
		var zero T
		*result = zero
		*err = &Error[T]{
			Path:      []Name{processorName},
			InputData: inputData,
			Err:       &panicError{processorName: processorName, sanitized: sanitizePanicMessage(r)},
			Timestamp: time.Now(),
			Duration:  0, // We don't track duration for panics
			Timeout:   false,
			Canceled:  false,
		}
	}
}
