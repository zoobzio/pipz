package pipz

import (
	"context"
	"errors"
	"sync"
	"time"
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
func (t *Timeout[T]) Process(ctx context.Context, data T) (T, error) {
	t.mu.RLock()
	processor := t.processor
	duration := t.duration
	t.mu.RUnlock()

	// Add this timeout to the processing path

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	done := make(chan struct{})
	var result T
	var err error

	go func() {
		result, err = processor.Process(ctx, data)
		close(done)
	}()

	select {
	case <-done:
		if err != nil {
			var pipeErr *Error[T]
			if errors.As(err, &pipeErr) {
				// Prepend this timeout's name to the path
				pipeErr.Path = append([]Name{t.name}, pipeErr.Path...)
				return result, pipeErr
			}
			// Handle non-pipeline errors by wrapping them
			return result, &Error[T]{
				Timestamp: time.Now(),
				InputData: data,
				Err:       err,
				Path:      []Name{t.name},
			}
		}
		return result, nil
	case <-ctx.Done():
		// Timeout occurred - return error
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
