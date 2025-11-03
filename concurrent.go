package pipz

import (
	"context"
	"sync"
)

// Concurrent runs all processors in parallel with the original context preserved.
// This connector passes the original context directly to each processor, preserving
// distributed tracing information, spans, and other context values. Each processor
// receives a deep copy of the input, ensuring complete isolation.
//
// Concurrent supports two modes:
//   - Without reducer (nil): Returns the original input unchanged after all processors complete
//   - With reducer: Collects all results and errors, then calls the reducer function to produce the final output
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// Use Concurrent when you need:
//   - Distributed tracing to work across concurrent operations
//   - All processors to respect the original context's cancellation
//   - To wait for all processors to complete before continuing
//   - Multiple side effects to happen simultaneously
//   - To aggregate results from parallel operations (with reducer)
//
// Common use cases:
//   - Sending traced notifications to multiple channels
//   - Updating multiple external systems with trace context
//   - Parallel logging with trace IDs preserved
//   - Fetching data from multiple sources and merging results
//   - Operations that must all complete or be canceled together
//
// Important characteristics:
//   - Input type must implement Cloner[T] interface
//   - All processors run regardless of individual failures
//   - Context cancellation immediately affects all processors
//   - Preserves trace context and spans for distributed tracing
//   - Waits for all processors to complete
//   - Reducer receives map[Name]T for results and map[Name]error for errors
//
// Example without reducer (side effects):
//
//	type Order struct {
//	    ID     string
//	    Items  []Item
//	    Status string
//	}
//
//	func (o Order) Clone() Order {
//	    items := make([]Item, len(o.Items))
//	    copy(items, o.Items)
//	    return Order{
//	        ID:     o.ID,
//	        Items:  items,
//	        Status: o.Status,
//	    }
//	}
//
//	concurrent := pipz.NewConcurrent(
//	    "notify-order",
//	    nil, // no reducer, just run side effects
//	    sendEmailNotification,
//	    sendSMSNotification,
//	    updateInventorySystem,
//	    logToAnalytics,
//	)
//
// Example with reducer (aggregate results):
//
//	type PriceCheck struct {
//	    ProductID string
//	    BestPrice float64
//	}
//
//	func (p PriceCheck) Clone() PriceCheck {
//	    return p
//	}
//
//	reducer := func(original PriceCheck, results map[Name]PriceCheck, errors map[Name]error) PriceCheck {
//	    bestPrice := original.BestPrice
//	    for _, result := range results {
//	        if result.BestPrice < bestPrice {
//	            bestPrice = result.BestPrice
//	        }
//	    }
//	    return PriceCheck{ProductID: original.ProductID, BestPrice: bestPrice}
//	}
//
//	concurrent := pipz.NewConcurrent(
//	    "check-prices",
//	    reducer,
//	    checkAmazon,
//	    checkWalmart,
//	    checkTarget,
//	)
type Concurrent[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	reducer    func(original T, results map[Name]T, errors map[Name]error) T
	mu         sync.RWMutex
}

// NewConcurrent creates a new Concurrent connector.
// If reducer is nil, the original input is returned unchanged.
// If reducer is provided, it receives the original input, all processor results,
// and any errors, allowing you to aggregate or merge results into a new T.
func NewConcurrent[T Cloner[T]](name Name, reducer func(original T, results map[Name]T, errors map[Name]error) T, processors ...Chainable[T]) *Concurrent[T] {
	return &Concurrent[T]{
		name:       name,
		reducer:    reducer,
		processors: processors,
	}
}

// Process implements the Chainable interface.
func (c *Concurrent[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, c.name, input)

	c.mu.RLock()
	processors := make([]Chainable[T], len(c.processors))
	copy(processors, c.processors)
	c.mu.RUnlock()

	if len(processors) == 0 {
		return input, nil
	}

	var wg sync.WaitGroup
	wg.Add(len(processors))

	// Collect results if reducer is provided
	var resultsMu sync.Mutex
	var results map[Name]T
	var errors map[Name]error
	if c.reducer != nil {
		results = make(map[Name]T, len(processors))
		errors = make(map[Name]error, len(processors))
	}

	// Process all with the original context to preserve tracing
	for _, processor := range processors {
		go func(p Chainable[T]) {
			defer func() {
				// Always call wg.Done() even if Clone() or Process() panics
				// This prevents deadlock in wg.Wait()
				if r := recover(); r != nil {
					// Panic occurred, but we must complete wg.Done()
					// The goroutine can die after this, we just prevent deadlock
					_ = r // Acknowledge the panic but continue
				}
				wg.Done()
			}()

			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Process with the context
			res, err := p.Process(ctx, inputCopy)

			// Collect results if reducer is provided
			if c.reducer != nil {
				resultsMu.Lock()
				if err != nil {
					errors[p.Name()] = err
				} else {
					results[p.Name()] = res
				}
				resultsMu.Unlock()
			}
		}(processor)
	}

	// Wait for completion or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All processors completed
		if c.reducer != nil {
			return c.reducer(input, results, errors), nil
		}
		return input, nil
	case <-ctx.Done():
		// Context canceled
		if c.reducer != nil {
			// Call reducer with whatever results we have so far
			return c.reducer(input, results, errors), nil
		}
		return input, nil
	}
}

// Add appends a processor to the concurrent execution list.
func (c *Concurrent[T]) Add(processor Chainable[T]) *Concurrent[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processor)
	return c
}

// Remove removes the processor at the specified index.
func (c *Concurrent[T]) Remove(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = append(c.processors[:index], c.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (c *Concurrent[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Clear removes all processors from the concurrent execution list.
func (c *Concurrent[T]) Clear() *Concurrent[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = nil
	return c
}

// SetProcessors replaces all processors atomically.
func (c *Concurrent[T]) SetProcessors(processors ...Chainable[T]) *Concurrent[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = make([]Chainable[T], len(processors))
	copy(c.processors, processors)
	return c
}

// Name returns the name of this connector.
func (c *Concurrent[T]) Name() Name {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.name
}

// Close gracefully shuts down the connector.
func (*Concurrent[T]) Close() error {
	return nil
}
