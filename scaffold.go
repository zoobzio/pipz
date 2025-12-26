package pipz

import (
	"context"
	"errors"
	"sync"

	"github.com/zoobzio/capitan"
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
	identity   Identity
	processors []Chainable[T]
	mu         sync.RWMutex
	closeOnce  sync.Once
	closeErr   error
}

// NewScaffold creates a new Scaffold connector.
func NewScaffold[T Cloner[T]](identity Identity, processors ...Chainable[T]) *Scaffold[T] {
	return &Scaffold[T]{
		identity:   identity,
		processors: processors,
	}
}

// Process implements the Chainable interface.
func (s *Scaffold[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, s.identity, input)

	s.mu.RLock()
	processors := make([]Chainable[T], len(s.processors))
	copy(processors, s.processors)
	s.mu.RUnlock()

	if len(processors) == 0 {
		return input, nil
	}

	// Create context that won't be canceled when parent is
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

	// Emit dispatched signal
	capitan.Info(ctx, SignalScaffoldDispatched,
		FieldName.Field(s.identity.Name()),
		FieldIdentityID.Field(s.identity.ID().String()),
		FieldProcessorCount.Field(len(processors)),
	)

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

// Identity returns the identity of this connector.
func (s *Scaffold[T]) Identity() Identity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity
}

// Schema returns a Node representing this connector in the pipeline schema.
func (s *Scaffold[T]) Schema() Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	processors := make([]Node, len(s.processors))
	for i, proc := range s.processors {
		processors[i] = proc.Schema()
	}

	return Node{
		Identity: s.identity,
		Type:     "scaffold",
		Flow:     ScaffoldFlow{Processors: processors},
	}
}

// Close gracefully shuts down the connector and all its child processors.
// Close is idempotent - multiple calls return the same result.
func (s *Scaffold[T]) Close() error {
	s.closeOnce.Do(func() {
		s.mu.RLock()
		defer s.mu.RUnlock()

		var errs []error
		for i := len(s.processors) - 1; i >= 0; i-- {
			if err := s.processors[i].Close(); err != nil {
				errs = append(errs, err)
			}
		}
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}
