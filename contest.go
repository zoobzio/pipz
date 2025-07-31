package pipz

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Contest runs all processors in parallel and returns the first result that
// meets a specified condition. Contest combines competitive processing (like Race)
// with conditional selection, allowing you to define what makes a "winner" beyond
// just being first to complete.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// This pattern excels when you have multiple ways to get a result and want the
// fastest one that meets specific criteria:
//   - Finding the cheapest shipping rate under a time constraint
//   - Getting the first API response with required data completeness
//   - Querying multiple sources for the best quality result quickly
//   - Racing services where the "best" result matters more than just "first"
//   - Any scenario where you need speed AND quality criteria
//
// Key behaviors:
//   - First result meeting the condition wins and cancels others
//   - If no results meet the condition, returns the original input with an error
//   - Each processor gets an isolated copy via Clone()
//   - Condition is evaluated as results arrive (no waiting for all)
//   - Can reduce latency while ensuring quality constraints
//
// Example:
//
//	// Find the first shipping rate under $50
//	contest := pipz.NewContest("cheapest-rate",
//	    func(_ context.Context, rate Rate) bool {
//	        return rate.Cost < 50.00
//	    },
//	    fedexRates,
//	    upsRates,
//	    uspsRates,
//	)
type Contest[T Cloner[T]] struct {
	name       Name
	condition  func(context.Context, T) bool
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewContest creates a new Contest connector with the specified winning condition.
// The condition function determines which results are acceptable winners.
// A result must both complete successfully AND meet the condition to win.
func NewContest[T Cloner[T]](name Name, condition func(context.Context, T) bool, processors ...Chainable[T]) *Contest[T] {
	return &Contest[T]{
		name:       name,
		condition:  condition,
		processors: processors,
	}
}

// Process implements the Chainable interface.
func (c *Contest[T]) Process(ctx context.Context, input T) (T, *Error[T]) {
	c.mu.RLock()
	processors := make([]Chainable[T], len(c.processors))
	copy(processors, c.processors)
	condition := c.condition
	c.mu.RUnlock()

	if len(processors) == 0 {
		var zero T
		return zero, &Error[T]{
			Path:      []Name{c.name},
			Err:       fmt.Errorf("no processors provided to Contest"),
			InputData: input,
			Timestamp: time.Now(),
			Duration:  0,
		}
	}

	if condition == nil {
		var zero T
		return zero, &Error[T]{
			Path:      []Name{c.name},
			Err:       fmt.Errorf("no condition provided to Contest"),
			InputData: input,
			Timestamp: time.Now(),
			Duration:  0,
		}
	}

	// Create channels for results and completion tracking
	type result struct {
		data T
		err  *Error[T]
		idx  int
	}

	resultCh := make(chan result, len(processors))
	contestCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch all processors
	for i, processor := range processors {
		go func(idx int, p Chainable[T]) {
			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			data, err := p.Process(contestCtx, inputCopy)
			select {
			case resultCh <- result{data: data, err: err, idx: idx}:
			case <-contestCtx.Done():
			}
		}(i, processor)
	}

	// Collect results and check conditions
	var allErrors []*Error[T]
	completedCount := 0

	for completedCount < len(processors) {
		select {
		case res := <-resultCh:
			completedCount++

			if res.err == nil {
				// Check if this successful result meets the condition
				if condition(ctx, res.data) {
					// Winner! Cancel other goroutines and return
					cancel()
					return res.data, nil
				}
				// Result doesn't meet condition, continue waiting for others
			} else {
				// Track errors for potential return if all fail
				if res.err != nil {
					// Prepend this contest's name to the path
					res.err.Path = append([]Name{c.name}, res.err.Path...)
					allErrors = append(allErrors, res.err)
				}
			}

		case <-ctx.Done():
			// Context canceled - return original input
			return input, nil
		}
	}

	// No processor produced a result meeting the condition
	var err error
	if len(allErrors) == len(processors) {
		// All processors failed with errors
		err = fmt.Errorf("all processors failed: %d errors", len(allErrors))
	} else {
		// Some succeeded but none met the condition
		err = fmt.Errorf("no processor results met the specified condition")
	}

	return input, &Error[T]{
		Path:      []Name{c.name},
		Err:       err,
		InputData: input,
		Timestamp: time.Now(),
		Duration:  0,
	}
}

// SetCondition updates the winning condition.
// This allows changing the criteria at runtime.
func (c *Contest[T]) SetCondition(condition func(context.Context, T) bool) *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.condition = condition
	return c
}

// Add appends a processor to the contest execution list.
func (c *Contest[T]) Add(processor Chainable[T]) *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processor)
	return c
}

// Remove removes the processor at the specified index.
func (c *Contest[T]) Remove(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = append(c.processors[:index], c.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (c *Contest[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Clear removes all processors from the contest execution list.
func (c *Contest[T]) Clear() *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = nil
	return c
}

// SetProcessors replaces all processors atomically.
func (c *Contest[T]) SetProcessors(processors ...Chainable[T]) *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = make([]Chainable[T], len(processors))
	copy(c.processors, processors)
	return c
}

// Name returns the name of this connector.
func (c *Contest[T]) Name() Name {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.name
}
