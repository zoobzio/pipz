package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
)

// Fallback attempts processors in order, falling back to the next on error.
// Fallback provides automatic failover through a chain of alternative processors
// when earlier ones fail. This creates resilient processing chains that can recover
// from failures gracefully.
//
// Unlike Retry which attempts the same operation multiple times,
// Fallback switches to completely different implementations. Each processor
// is tried in order until one succeeds or all fail.
//
// Common use cases:
//   - Primary/backup/tertiary service failover
//   - Graceful degradation strategies
//   - Multiple payment provider support
//   - Cache miss handling (try local cache, then redis, then database)
//   - API version compatibility chains
//
// Example:
//
//	fallback := pipz.NewFallback("payment-providers",
//	    stripeProcessor,       // Try Stripe first
//	    paypalProcessor,       // Fall back to PayPal on error
//	    squareProcessor,       // Finally try Square
//	)
//
// IMPORTANT: Avoid circular references between Fallback instances when all processors fail.
// Example of DANGEROUS pattern:
//
//	fallback1 → fallback2 → fallback3 → fallback1
//
// This creates infinite recursion risk if all processors fail, leading to stack overflow.
type Fallback[T any] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewFallback creates a new Fallback connector that tries processors in order.
// At least one processor must be provided. Each processor is tried in order
// until one succeeds or all fail.
//
// Examples:
//
//	fallback := pipz.NewFallback("payment", stripe, paypal, square)
//	fallback := pipz.NewFallback("cache", redis, database)
func NewFallback[T any](name Name, processors ...Chainable[T]) *Fallback[T] {
	if len(processors) == 0 {
		panic("NewFallback requires at least one processor")
	}

	return &Fallback[T]{
		name:       name,
		processors: processors,
	}
}

// Process implements the Chainable interface.
// Tries each processor in order until one succeeds or all fail.
func (f *Fallback[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, f.name, data)

	f.mu.RLock()
	processors := make([]Chainable[T], len(f.processors))
	copy(processors, f.processors)
	f.mu.RUnlock()

	var lastErr error
	name := string(f.name)

	for i, processor := range processors {
		// Get processor name if available
		procName := "unknown"
		if named, ok := interface{}(processor).(interface{ Name() Name }); ok {
			procName = string(named.Name())
		}

		// Emit attempt signal
		capitan.Emit(ctx, SignalFallbackAttempt,
			FieldName.Field(name),
			FieldProcessorIndex.Field(i),
			FieldProcessorName.Field(procName),
		)

		result, err := processor.Process(ctx, data)
		if err == nil {
			// Success! Return immediately
			return result, nil
		}

		// Store the error for potential return
		lastErr = err
	}

	// All processors failed, return the last error with path
	if lastErr != nil {
		// Emit failed signal
		capitan.Emit(ctx, SignalFallbackFailed,
			FieldName.Field(name),
			FieldError.Field(lastErr.Error()),
		)

		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			pipeErr.Path = append([]Name{f.name}, pipeErr.Path...)
			return data, pipeErr
		}
		// Wrap non-pipeline errors
		return data, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       lastErr,
			Path:      []Name{f.name},
		}
	}
	return data, nil
}

// SetProcessors replaces all processors with the provided ones.
func (f *Fallback[T]) SetProcessors(processors ...Chainable[T]) *Fallback[T] {
	if len(processors) == 0 {
		panic("SetProcessors requires at least one processor")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processors = make([]Chainable[T], len(processors))
	copy(f.processors, processors)
	return f
}

// AddFallback appends a processor to the end of the fallback chain.
func (f *Fallback[T]) AddFallback(processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processors = append(f.processors, processor)
	return f
}

// InsertAt inserts a processor at the specified index.
func (f *Fallback[T]) InsertAt(index int, processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index > len(f.processors) {
		panic("index out of bounds")
	}
	f.processors = append(f.processors[:index], append([]Chainable[T]{processor}, f.processors[index:]...)...)
	return f
}

// RemoveAt removes the processor at the specified index.
func (f *Fallback[T]) RemoveAt(index int) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.processors) {
		panic("index out of bounds")
	}
	if len(f.processors) == 1 {
		panic("cannot remove last processor from fallback")
	}
	f.processors = append(f.processors[:index], f.processors[index+1:]...)
	return f
}

// Name returns the name of this connector.
func (f *Fallback[T]) Name() Name {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.name
}

// Close gracefully shuts down the connector.
func (*Fallback[T]) Close() error {
	return nil
}

// GetProcessors returns a copy of all processors in order.
func (f *Fallback[T]) GetProcessors() []Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	processors := make([]Chainable[T], len(f.processors))
	copy(processors, f.processors)
	return processors
}

// Len returns the number of processors in the fallback chain.
func (f *Fallback[T]) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.processors)
}

// GetPrimary returns the first processor (for backward compatibility).
func (f *Fallback[T]) GetPrimary() Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.processors) > 0 {
		return f.processors[0]
	}
	return nil
}

// GetFallback returns the second processor (for backward compatibility).
// Returns nil if there's no second processor.
func (f *Fallback[T]) GetFallback() Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.processors) > 1 {
		return f.processors[1]
	}
	return nil
}

// SetPrimary updates the first processor (for backward compatibility).
func (f *Fallback[T]) SetPrimary(processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.processors) > 0 {
		f.processors[0] = processor
	}
	return f
}

// SetFallback updates the second processor (for backward compatibility).
// If there's no second processor, adds one.
func (f *Fallback[T]) SetFallback(processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.processors) > 1 {
		f.processors[1] = processor
	} else if len(f.processors) == 1 {
		f.processors = append(f.processors, processor)
	}
	return f
}
