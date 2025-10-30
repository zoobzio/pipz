package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"github.com/zoobzio/clockz"
)

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
type Backoff[T any] struct {
	processor   Chainable[T]
	clock       clockz.Clock
	name        Name
	baseDelay   time.Duration
	mu          sync.RWMutex
	maxAttempts int
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
	clock := b.getClock()
	b.mu.RUnlock()

	var lastErr error
	var lastResult T
	delay := baseDelay

	for i := 0; i < maxAttempts; i++ {
		result, err := processor.Process(ctx, data)
		if err == nil {
			// Success!
			return result, nil
		}

		// Attempt failed
		lastErr = err
		lastResult = result

		// Don't sleep after the last attempt
		if i < maxAttempts-1 {
			// Emit backoff waiting signal
			nextDelay := delay * 2
			capitan.Warn(context.Background(), SignalBackoffWaiting,
				FieldName.Field(string(b.name)),
				FieldAttempt.Field(i+1),
				FieldMaxAttempts.Field(maxAttempts),
				FieldDelay.Field(delay.Seconds()),
				FieldNextDelay.Field(nextDelay.Seconds()),
				FieldTimestamp.Field(float64(time.Now().Unix())),
			)

			select {
			case <-clock.After(delay):
				delay = nextDelay // Exponential backoff
			case <-ctx.Done():
				// Context canceled/timed out
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
	return lastResult, nil
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

// Close gracefully shuts down the connector.
func (*Backoff[T]) Close() error {
	return nil
}

// WithClock sets a custom clock for testing.
func (b *Backoff[T]) WithClock(clock clockz.Clock) *Backoff[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clock = clock
	return b
}

// getClock returns the clock to use.
func (b *Backoff[T]) getClock() clockz.Clock {
	if b.clock == nil {
		return clockz.RealClock
	}
	return b.clock
}
