package pipz

import (
	"context"
	"errors"
	"sync"
	"time"
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

	for i := 0; i < maxAttempts; i++ {
		result, err := processor.Process(ctx, data)
		if err == nil {
			return result, nil
		}
		lastErr = err
		lastResult = result

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

// Backoff attempts the processor with exponential backoff between attempts.
// Backoff adds intelligent spacing between retry attempts, starting with
// baseDelay and doubling after each failure. This prevents overwhelming failed
// services and allows time for transient issues to resolve.
//
// The exponential backoff pattern (delay, 2*delay, 4*delay, ...) is widely
// used for its effectiveness in handling various failure scenarios without
// overwhelming systems. The operation can be canceled via context during waits.
//
// Ideal for:
//   - API calls to rate-limited services
//   - Database operations during high load
//   - Distributed system interactions
//   - Any operation where immediate retry is counterproductive
//
// The total time spent can be significant with multiple retries.
// For example, with baseDelay=1s and maxAttempts=5:
//
//	Delays: 1s, 2s, 4s, 8s (total wait: 15s plus processing time)
//
// Example:
//
//	backoff := pipz.NewBackoff(
//	    "api-backoff",
//	    apiProcessor,
//	    5,                    // Max 5 attempts
//	    100*time.Millisecond, // Start with 100ms delay
//	)
type Backoff[T any] struct {
	processor   Chainable[T]
	name        Name
	maxAttempts int
	baseDelay   time.Duration
	mu          sync.RWMutex
}

// NewBackoff creates a new Backoff connector.
func NewBackoff[T any](name Name, processor Chainable[T], maxAttempts int, baseDelay time.Duration) *Backoff[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Backoff[T]{
		name:        name,
		processor:   processor,
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
	}
}

// Process implements the Chainable interface.
func (b *Backoff[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, b.name, data)
	b.mu.RLock()
	processor := b.processor
	maxAttempts := b.maxAttempts
	baseDelay := b.baseDelay
	b.mu.RUnlock()

	var lastErr error
	delay := baseDelay

	for i := 0; i < maxAttempts; i++ {
		result, err := processor.Process(ctx, data)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Don't sleep after the last attempt
		if i < maxAttempts-1 {
			select {
			case <-time.After(delay):
				delay *= 2 // Exponential backoff
			case <-ctx.Done():
				// Context canceled/timed out - create appropriate error
				return data, &Error[T]{
					Err:       ctx.Err(),
					InputData: data,
					Path:      []Name{b.name},
					Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
					Canceled:  errors.Is(ctx.Err(), context.Canceled),
					Timestamp: time.Now(),
				}
			}
		}
	}
	// All attempts failed - return the last error
	if lastErr != nil {
		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			// Prepend this backoff's name to the path
			pipeErr.Path = append([]Name{b.name}, pipeErr.Path...)
			return data, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return data, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       lastErr,
			Path:      []Name{b.name},
		}
	}
	return data, nil
}

// SetMaxAttempts updates the maximum number of retry attempts.
func (b *Backoff[T]) SetMaxAttempts(n int) *Backoff[T] {
	if n < 1 {
		n = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxAttempts = n
	return b
}

// SetBaseDelay updates the base delay duration.
func (b *Backoff[T]) SetBaseDelay(d time.Duration) *Backoff[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.baseDelay = d
	return b
}

// GetMaxAttempts returns the current maximum attempts setting.
func (b *Backoff[T]) GetMaxAttempts() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.maxAttempts
}

// GetBaseDelay returns the current base delay setting.
func (b *Backoff[T]) GetBaseDelay() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.baseDelay
}

// Name returns the name of this connector.
func (b *Backoff[T]) Name() Name {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.name
}
