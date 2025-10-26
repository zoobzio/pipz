package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/clockz"
)

// Timeout enforces a timeout on the processor's execution.
// Timeout wraps any processor with a hard time limit, ensuring operations
// complete within acceptable bounds. If the timeout expires, the operation
// is canceled via context and a timeout error is returned.
//
// This connector is critical for:
//   - Preventing hung operations
//   - Meeting SLA requirements
//   - Protecting against slow external services
//   - Ensuring predictable system behavior
//   - Resource management in concurrent systems
//
// The wrapped operation should respect context cancellation for
// immediate termination. Operations that ignore context may continue
// running in the background even after timeout.
//
// Timeout is often combined with Retry for robust error handling:
//
//	pipz.NewRetry(pipz.NewTimeout(operation, 5*time.Second), 3)
//
// Example:
//
//	timeout := pipz.NewTimeout(
//	    userServiceCall,
//	    2*time.Second,  // Must complete within 2 seconds
//	)
type Timeout[T any] struct {
	processor Chainable[T]
	clock     clockz.Clock
	name      Name
	duration  time.Duration
	mu        sync.RWMutex
}

// NewTimeout creates a new Timeout connector.
func NewTimeout[T any](name Name, processor Chainable[T], duration time.Duration) *Timeout[T] {
	return &Timeout[T]{
		name:      name,
		processor: processor,
		duration:  duration,
	}
}

// Process implements the Chainable interface.
func (t *Timeout[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, t.name, data)

	t.mu.RLock()
	processor := t.processor
	duration := t.duration
	clock := t.getClock()
	t.mu.RUnlock()

	ctx, cancel := clock.WithTimeout(ctx, duration)
	defer cancel()

	// Channel to receive the result from the goroutine
	type processResult struct {
		result T
		err    error
	}
	resultCh := make(chan processResult, 1)

	go func() {
		defer func() {
			// Ensure goroutine cleanup on panic
			if r := recover(); r != nil {
				// Convert panic to error and send it
				var zero T
				panicErr := &Error[T]{
					Path:      []Name{t.name},
					InputData: data,
					Err:       &panicError{processorName: t.name, sanitized: sanitizePanicMessage(r)},
					Timestamp: time.Now(),
					Duration:  0,
					Timeout:   false,
					Canceled:  false,
				}
				select {
				case resultCh <- processResult{result: zero, err: panicErr}:
				case <-ctx.Done():
					// Context was canceled, don't block
				}
			}
		}()
		result, err := processor.Process(ctx, data)
		// Use non-blocking send to avoid goroutine leak if main function has already returned
		select {
		case resultCh <- processResult{result: result, err: err}:
		case <-ctx.Done():
			// Context was canceled, don't block trying to send result
		}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			// Operation failed (but didn't timeout)
			var pipeErr *Error[T]
			if errors.As(res.err, &pipeErr) {
				// Prepend this timeout's name to the path
				pipeErr.Path = append([]Name{t.name}, pipeErr.Path...)
				return res.result, pipeErr
			}
			// Handle non-pipeline errors by wrapping them
			return res.result, &Error[T]{
				Timestamp: time.Now(),
				InputData: data,
				Err:       res.err,
				Path:      []Name{t.name},
			}
		}
		// Success!
		return res.result, nil
	case <-ctx.Done():
		// Timeout or cancellation occurred
		return data, &Error[T]{
			Err:       ctx.Err(),
			InputData: data,
			Path:      []Name{t.name},
			Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
			Canceled:  errors.Is(ctx.Err(), context.Canceled),
			Timestamp: time.Now(),
		}
	}
}

// SetDuration updates the timeout duration.
func (t *Timeout[T]) SetDuration(d time.Duration) *Timeout[T] {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.duration = d
	return t
}

// GetDuration returns the current timeout duration.
func (t *Timeout[T]) GetDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.duration
}

// Name returns the name of this connector.
func (t *Timeout[T]) Name() Name {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

// WithClock sets a custom clock for testing.
func (t *Timeout[T]) WithClock(clock clockz.Clock) *Timeout[T] {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clock = clock
	return t
}

// getClock returns the clock to use.
func (t *Timeout[T]) getClock() clockz.Clock {
	if t.clock == nil {
		return clockz.RealClock
	}
	return t.clock
}

// Close gracefully shuts down the timeout connector.
func (*Timeout[T]) Close() error {
	return nil
}
