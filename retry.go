package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
)

// Retry attempts the processor up to maxAttempts times.
// Retry provides simple retry logic for operations that may fail
// transiently. It immediately retries on failure without delay,
// making it suitable for quick operations or when failures are
// expected to clear immediately.
//
// Each retry uses the same input data. Context cancellation is
// checked between attempts to allow for early termination.
// If all attempts fail, the last error is returned with attempt
// count information for debugging.
//
// Use Retry for:
//   - Network calls with transient failures
//   - Database operations during brief contentions
//   - File operations with temporary locks
//   - Any operation with intermittent failures
//
// For operations needing delay between retries, use RetryWithBackoff.
// For trying different approaches, use Fallback instead.
//
// Example:
//
//	retry := pipz.NewRetry(
//	    databaseWriter,
//	    3,  // Try up to 3 times
//	)
type Retry[T any] struct {
	processor   Chainable[T]
	name        Name
	maxAttempts int
	mu          sync.RWMutex
	closeOnce   sync.Once
	closeErr    error
}

// NewRetry creates a new Retry connector.
func NewRetry[T any](name Name, processor Chainable[T], maxAttempts int) *Retry[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	return &Retry[T]{
		name:        name,
		processor:   processor,
		maxAttempts: maxAttempts,
	}
}

// Process implements the Chainable interface.
func (r *Retry[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, r.name, data)
	r.mu.RLock()
	processor := r.processor
	maxAttempts := r.maxAttempts
	r.mu.RUnlock()

	var lastErr error
	var lastResult T
	name := string(r.name)

	for i := 0; i < maxAttempts; i++ {
		attempt := i + 1

		// Emit attempt start signal
		capitan.Info(ctx, SignalRetryAttemptStart,
			FieldName.Field(name),
			FieldAttempt.Field(attempt),
			FieldMaxAttempts.Field(maxAttempts),
		)

		result, err := processor.Process(ctx, data)
		if err == nil {
			// Success!
			return result, nil
		}

		// Attempt failed
		lastErr = err
		lastResult = result

		// Emit attempt fail signal
		capitan.Warn(ctx, SignalRetryAttemptFail,
			FieldName.Field(name),
			FieldAttempt.Field(attempt),
			FieldMaxAttempts.Field(maxAttempts),
			FieldError.Field(err.Error()),
		)

		// Check if context is canceled between attempts
		if ctx.Err() != nil {
			// Context canceled/timed out - return error
			return data, &Error[T]{
				Err:       ctx.Err(),
				InputData: data,
				Path:      []Name{r.name},
				Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
				Canceled:  errors.Is(ctx.Err(), context.Canceled),
				Timestamp: time.Now(),
			}
		}
	}

	// All attempts failed - emit exhausted signal
	capitan.Error(ctx, SignalRetryExhausted,
		FieldName.Field(name),
		FieldMaxAttempts.Field(maxAttempts),
		FieldError.Field(lastErr.Error()),
	)

	// All attempts failed - return the last error
	if lastErr != nil {
		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			// Prepend this retry's name to the path
			pipeErr.Path = append([]Name{r.name}, pipeErr.Path...)
			return lastResult, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return lastResult, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       lastErr,
			Path:      []Name{r.name},
		}
	}
	return lastResult, nil
}

// SetMaxAttempts updates the maximum number of retry attempts.
func (r *Retry[T]) SetMaxAttempts(n int) *Retry[T] {
	if n < 1 {
		n = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxAttempts = n
	return r
}

// GetMaxAttempts returns the current maximum attempts setting.
func (r *Retry[T]) GetMaxAttempts() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxAttempts
}

// Name returns the name of this connector.
func (r *Retry[T]) Name() Name {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.name
}

// Close gracefully shuts down the connector and its child processor.
// Close is idempotent - multiple calls return the same result.
func (r *Retry[T]) Close() error {
	r.closeOnce.Do(func() {
		r.mu.RLock()
		defer r.mu.RUnlock()
		r.closeErr = r.processor.Close()
	})
	return r.closeErr
}
