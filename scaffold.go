package pipz

import (
	"context"
	"sync"
)

// Scaffold runs all processors in parallel with context isolation for true fire-and-forget behavior.
// Unlike Concurrent, Scaffold uses context.WithoutCancel to ensure processors continue
// running even if the parent context is canceled. This is ideal for operations that
// must complete regardless of the main pipeline's state.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// Use Scaffold when you need:
//   - True fire-and-forget operations that outlive the request
//   - Background tasks that shouldn't be canceled with the main flow
//   - Cleanup or logging operations that must complete
//   - Non-critical side effects that shouldn't block the pipeline
//
// Common use cases:
//   - Asynchronous audit logging
//   - Background cache warming
//   - Non-critical notifications
//   - Metrics collection
//   - Cleanup tasks that should complete independently
//
// Important characteristics:
//   - Input type must implement Cloner[T] interface
//   - Processors continue even after parent context cancellation
//   - Returns immediately without waiting for completion
//   - Original input always returned unchanged
//   - No error reporting from background processors
//   - Trace context is preserved (but cancellation is not)
//
// Example:
//
//	scaffold := pipz.NewScaffold(
//	    "async-operations",
//	    asyncAuditLog,
//	    warmCache,
//	    collectMetrics,
//	)
//
//	// Returns immediately, processors run in background
//	result, err := scaffold.Process(ctx, order)
type Scaffold[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewScaffold creates a new Scaffold connector.
func NewScaffold[T Cloner[T]](name Name, processors ...Chainable[T]) *Scaffold[T] {
	return &Scaffold[T]{
		name:       name,
		processors: processors,
	}
}

// Process implements the Chainable interface.
func (s *Scaffold[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, s.name, input)
	s.mu.RLock()
	processors := make([]Chainable[T], len(s.processors))
	copy(processors, s.processors)
	s.mu.RUnlock()

	if len(processors) == 0 {
		return input, nil
	}

	// Create context that won't be canceled when parent is
	// This preserves values like trace IDs while removing cancellation
	bgCtx := context.WithoutCancel(ctx)

	// Launch all processors in background without waiting
	for _, processor := range processors {
		go func(p Chainable[T]) {
			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Process with isolated context - continues even if parent canceled
			if _, err := p.Process(bgCtx, inputCopy); err != nil {
				// Fire-and-forget: errors are not reported back
				_ = err
			}
		}(processor)
	}

	// Return immediately without waiting
	return input, nil
}

// Add appends a processor to the scaffold execution list.
func (s *Scaffold[T]) Add(processor Chainable[T]) *Scaffold[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processors = append(s.processors, processor)
	return s
}

// Remove removes the processor at the specified index.
func (s *Scaffold[T]) Remove(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.processors) {
		return ErrIndexOutOfBounds
	}

	s.processors = append(s.processors[:index], s.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (s *Scaffold[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.processors)
}

// Clear removes all processors from the scaffold execution list.
func (s *Scaffold[T]) Clear() *Scaffold[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processors = nil
	return s
}

// SetProcessors replaces all processors atomically.
func (s *Scaffold[T]) SetProcessors(processors ...Chainable[T]) *Scaffold[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processors = make([]Chainable[T], len(processors))
	copy(s.processors, processors)
	return s
}

// Name returns the name of this connector.
func (s *Scaffold[T]) Name() Name {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}
