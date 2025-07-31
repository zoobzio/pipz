package pipz

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Race runs all processors in parallel and returns the result of the first
// to complete successfully. Race implements competitive processing where speed
// matters more than which specific processor succeeds. The first successful
// result wins and cancels all other processors.
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
func (r *Race[T]) Process(ctx context.Context, input T) (T, *Error[T]) {
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
	type result struct {
		data T
		err  *Error[T]
		idx  int
	}

	resultCh := make(chan result, len(processors))
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch all processors
	for i, processor := range processors {
		go func(idx int, p Chainable[T]) {
			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			data, err := p.Process(raceCtx, inputCopy)
			select {
			case resultCh <- result{data: data, err: err, idx: idx}:
			case <-raceCtx.Done():
			}
		}(i, processor)
	}

	// Collect results
	var lastErr *Error[T]
	for i := 0; i < len(processors); i++ {
		select {
		case res := <-resultCh:
			if res.err == nil {
				// First success wins
				cancel() // Cancel other goroutines
				return res.data, nil
			}
			lastErr = res.err
			if lastErr != nil {
				// Prepend this race's name to the path
				lastErr.Path = append([]Name{r.name}, lastErr.Path...)
			}
		case <-ctx.Done():
			// Context done means we're complete - return current input
			return input, nil
		}
	}

	// All failed - return the last error which should already be wrapped
	return input, lastErr
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
