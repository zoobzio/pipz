package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Race runs all processors in parallel and returns the result of the first
// to complete successfully. Race implements competitive processing where speed
// matters more than which specific processor succeeds. The first successful
// result wins and cancels all other processors.
//
// Context handling: Race uses context.WithCancel(ctx) to create a derived context
// that preserves all parent context values (including trace IDs) while allowing
// cancellation of losing processors when a winner is found.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// This pattern excels when you have multiple ways to get the same result
// and want the fastest one:
//   - Querying multiple replicas or regions
//   - Trying different algorithms with varying performance
//   - Fetching from multiple caches
//   - Calling primary and backup services simultaneously
//   - Any scenario where latency matters more than specific source
//
// Key behaviors:
//   - First success wins and cancels others
//   - All failures returns the last error
//   - Each processor gets an isolated copy via Clone()
//   - Useful for reducing p99 latencies
//   - Can increase load (all processors run)
//
// Example:
//
//	// UserQuery must implement Cloner[UserQuery]
//	race := pipz.NewRace(
//	    fetchFromLocalCache,
//	    fetchFromRegionalCache,
//	    fetchFromDatabase,
//	)
type Race[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewRace creates a new Race connector.
func NewRace[T Cloner[T]](name Name, processors ...Chainable[T]) *Race[T] {
	return &Race[T]{
		name:       name,
		processors: processors,
	}
}

// Process implements the Chainable interface.
func (r *Race[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, r.name, input)

	r.mu.RLock()
	processors := make([]Chainable[T], len(r.processors))
	copy(processors, r.processors)
	r.mu.RUnlock()

	if len(processors) == 0 {
		var zero T
		return zero, &Error[T]{
			Path:      []Name{r.name},
			Err:       fmt.Errorf("no processors provided to Race"),
			InputData: input,
			Timestamp: time.Now(),
			Duration:  0,
		}
	}

	// Create channels for results and errors
	type raceResult struct {
		data T
		err  error
		idx  int
	}

	resultCh := make(chan raceResult, len(processors))
	// Create a cancellable context to stop other processors when one wins
	// This derives from the original context, preserving trace data
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch all processors
	for i, processor := range processors {
		go func(idx int, p Chainable[T]) {
			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Process
			data, err := p.Process(raceCtx, inputCopy)
			select {
			case resultCh <- raceResult{data: data, err: err, idx: idx}:
			case <-raceCtx.Done():
			}
		}(i, processor)
	}

	// Collect results
	var lastErr error
	for i := 0; i < len(processors); i++ {
		select {
		case res := <-resultCh:
			if res.err == nil {
				// First success wins
				cancel() // Cancel other goroutines
				return res.data, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			// Context done means we're complete - return current input
			return input, nil
		}
	}

	// All failed - return the last error
	if lastErr != nil {
		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			// Prepend this race's name to the path
			pipeErr.Path = append([]Name{r.name}, pipeErr.Path...)
			return input, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return input, &Error[T]{
			Timestamp: time.Now(),
			InputData: input,
			Err:       lastErr,
			Path:      []Name{r.name},
		}
	}
	return input, nil
}

// Add appends a processor to the race execution list.
func (r *Race[T]) Add(processor Chainable[T]) *Race[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors = append(r.processors, processor)
	return r
}

// Remove removes the processor at the specified index.
func (r *Race[T]) Remove(index int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if index < 0 || index >= len(r.processors) {
		return ErrIndexOutOfBounds
	}

	r.processors = append(r.processors[:index], r.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (r *Race[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.processors)
}

// Clear removes all processors from the race execution list.
func (r *Race[T]) Clear() *Race[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors = nil
	return r
}

// SetProcessors replaces all processors atomically.
func (r *Race[T]) SetProcessors(processors ...Chainable[T]) *Race[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors = make([]Chainable[T], len(processors))
	copy(r.processors, processors)
	return r
}

// Name returns the name of this connector.
func (r *Race[T]) Name() Name {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.name
}

// Close gracefully shuts down the connector.
func (*Race[T]) Close() error {
	return nil
}
