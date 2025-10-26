package pipz

import (
	"context"
	"sync"
)

// Concurrent runs all processors in parallel with the original context preserved.
// Unlike fire-and-forget operations, this connector passes the original context
// directly to each processor, preserving distributed tracing information, spans,
// and other context values. Each processor receives a deep copy of the input,
// ensuring complete isolation. The original input is always returned unchanged.
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
//
// Common use cases:
//   - Sending traced notifications to multiple channels
//   - Updating multiple external systems with trace context
//   - Parallel logging with trace IDs preserved
//   - Triggering workflows that need distributed tracing
//   - Operations that must all complete or be canceled together
//
// Important characteristics:
//   - Input type must implement Cloner[T] interface
//   - All processors run regardless of individual failures
//   - Original input always returned (processors can't modify it)
//   - Context cancellation immediately affects all processors
//   - Preserves trace context and spans for distributed tracing
//   - Waits for all processors to complete
//
// Example:
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
//	    sendEmailNotification,
//	    sendSMSNotification,
//	    updateInventorySystem,
//	    logToAnalytics,
//	)
type Concurrent[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewConcurrent creates a new Concurrent connector.
func NewConcurrent[T Cloner[T]](name Name, processors ...Chainable[T]) *Concurrent[T] {
	return &Concurrent[T]{
		name:       name,
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

			// Process with the context (errors intentionally ignored in fire-and-forget)
			_, _ = p.Process(ctx, inputCopy) //nolint:errcheck
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
		return input, nil
	case <-ctx.Done():
		// Context canceled - return without error as processors run independently
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
