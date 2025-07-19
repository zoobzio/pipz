package pipz

import (
	"context"
	"fmt"
	"time"
)

// Chainable defines the interface for any component that can process
// values of type T. This interface enables composition of different
// processing components that operate on the same type.
type Chainable[T any] interface {
	Process(context.Context, T) (T, error)
}

// ProcessorFunc is a function adapter that implements Chainable.
type ProcessorFunc[T any] func(context.Context, T) (T, error)

// Process implements the Chainable interface.
func (f ProcessorFunc[T]) Process(ctx context.Context, data T) (T, error) {
	return f(ctx, data)
}

// Condition determines routing based on input data.
// Returns a route string (not boolean) for multi-way branching.
type Condition[T any] func(context.Context, T) string

// Sequential runs chainables in order, passing output to input.
// This replaces the old Chain implementation with a simpler approach.
func Sequential[T any](chainables ...Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		var err error
		for _, chainable := range chainables {
			data, err = chainable.Process(ctx, data)
			if err != nil {
				return data, err
			}
		}
		return data, nil
	})
}

// Switch routes to different chainables based on condition result.
// The condition returns a string key that maps to a chainable.
// If no route matches and a "default" route exists, it will be used.
func Switch[T any](condition Condition[T], routes map[string]Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		route := condition(ctx, data)
		chainable, exists := routes[route]
		if !exists {
			if defaultChainable, hasDefault := routes["default"]; hasDefault {
				return defaultChainable.Process(ctx, data)
			}
			return data, fmt.Errorf("no route for condition result: %s", route)
		}
		return chainable.Process(ctx, data)
	})
}

// Fallback attempts the primary chainable, falling back to secondary on error.
// This is useful for retry logic or alternative processing paths.
func Fallback[T any](primary, fallback Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		result, err := primary.Process(ctx, data)
		if err != nil {
			return fallback.Process(ctx, data)
		}
		return result, nil
	})
}

// Retry attempts the chainable up to maxAttempts times.
// If all attempts fail, returns the last error wrapped with attempt count.
func Retry[T any](chainable Chainable[T], maxAttempts int) Chainable[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		var lastErr error
		for i := 0; i < maxAttempts; i++ {
			result, err := chainable.Process(ctx, data)
			if err == nil {
				return result, nil
			}
			lastErr = err
			// Check if context is canceled between attempts
			if ctx.Err() != nil {
				return data, ctx.Err()
			}
		}
		return data, fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
	})
}

// RetryWithBackoff attempts the chainable with exponential backoff between attempts.
// baseDelay is the initial delay, which doubles after each failed attempt.
func RetryWithBackoff[T any](chainable Chainable[T], maxAttempts int, baseDelay time.Duration) Chainable[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		var lastErr error
		delay := baseDelay

		for i := 0; i < maxAttempts; i++ {
			result, err := chainable.Process(ctx, data)
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
					return data, ctx.Err()
				}
			}
		}
		return data, fmt.Errorf("failed after %d attempts with backoff: %w", maxAttempts, lastErr)
	})
}

// Timeout enforces a timeout on the chainable's execution.
// If the timeout is exceeded, the context is canceled and an error is returned.
func Timeout[T any](chainable Chainable[T], duration time.Duration) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		ctx, cancel := context.WithTimeout(ctx, duration)
		defer cancel()

		done := make(chan struct{})
		var result T
		var err error

		go func() {
			result, err = chainable.Process(ctx, data)
			close(done)
		}()

		select {
		case <-done:
			return result, err
		case <-ctx.Done():
			return data, fmt.Errorf("timeout after %v: %w", duration, ctx.Err())
		}
	})
}
