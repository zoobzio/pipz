package pipz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// Sequence modification errors.
var (
	ErrIndexOutOfBounds = errors.New("index out of bounds")
	ErrEmptySequence    = errors.New("sequence is empty")
	ErrInvalidRange     = errors.New("invalid range")
)

// Sequence provides a type-safe sequence for processing values of type T.
// It maintains an ordered list of processors that are executed sequentially.
//
// Sequence offers a rich API with methods to dynamically modify the processor
// chain. This makes it ideal for scenarios where the processing steps need
// to be configured at runtime or modified based on conditions.
//
// Key features:
//   - Thread-safe for concurrent access
//   - Dynamic modification of processor chain
//   - Named processors for debugging
//   - Rich API for reordering and modification
//   - Fail-fast execution with detailed errors
//
// Sequence is the primary way to chain processors together.
type Sequence[T any] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewSequence creates a new Sequence with optional initial processors.
// The sequence is ready to use immediately and can be safely
// accessed concurrently. Additional processors can be added using Register
// or the various modification methods.
//
// Example:
//
//	// Single line declaration
//	const (
//	    UserProcessingName = pipz.Name("user-processing")
//	    ValidateName = pipz.Name("validate")
//	    EnrichName = pipz.Name("enrich")
//	    AuditName = pipz.Name("audit")
//	)
//	sequence := pipz.NewSequence(UserProcessingName,
//	    pipz.Effect(ValidateName, validateUser),
//	    pipz.Apply(EnrichName, enrichUser),
//	    pipz.Effect(AuditName, auditUser),
//	)
//
//	// Or create empty and add later
//	sequence := pipz.NewSequence[User](UserProcessingName)
//	sequence.Register(validateUser, enrichUser)
func NewSequence[T any](name Name, processors ...Chainable[T]) *Sequence[T] {
	return &Sequence[T]{
		name:       name,
		processors: slices.Clone(processors),
	}
}

// Register adds processors to this Sequence.
// Processors are executed in the order they are registered.
//
// This method is thread-safe and can be called concurrently.
// New processors are appended to the existing chain, making
// Register ideal for building sequences incrementally:
//
//	sequence := pipz.NewSequence[Order]("order-processing")
//	sequence.Register(validateOrder)
//	sequence.Register(calculateTax, applyDiscount)
//	if config.RequiresApproval {
//	    sequence.Register(requireApproval)
//	}
func (c *Sequence[T]) Register(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// Process executes all registered processors on the input value.
// Each processor receives the output of the previous processor.
// The context is checked before each processor execution - if the context
// is canceled or expired, processing stops immediately.
// If any processor returns an error, execution stops and a Error
// is returned with rich debugging information.
//
// Process is thread-safe and can be called concurrently. The sequence's
// processor list is locked during execution to prevent modifications.
//
// Error handling includes:
//   - Processor name and stage index for debugging
//   - Original input data that caused the failure
//   - Execution duration for performance analysis
//   - Timeout/cancellation detection
//
// Context best practices:
//   - Always use context with timeout for production
//   - Check ctx.Err() in long-running processors
//   - Pass context through to external calls
func (c *Sequence[T]) Process(ctx context.Context, value T) (result T, err error) {
	defer recoverFromPanic(&result, &err, c.name, value)

	c.mu.RLock()
	processors := make([]Chainable[T], len(c.processors))
	copy(processors, c.processors)
	c.mu.RUnlock()

	// Handle nil context
	if ctx == nil {
		ctx = context.Background()
	}

	result = value

	for _, proc := range processors {
		// Check context before starting processor
		select {
		case <-ctx.Done():
			// Context canceled/timed out - create appropriate error
			return result, &Error[T]{
				Err:       ctx.Err(),
				InputData: value,
				Path:      []Name{c.name},
				Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
				Canceled:  errors.Is(ctx.Err(), context.Canceled),
				Timestamp: time.Now(),
			}
		default:
			result, err = proc.Process(ctx, result)
			if err != nil {
				var pipeErr *Error[T]
				if errors.As(err, &pipeErr) {
					// Prepend this sequence's name to the path
					pipeErr.Path = append([]Name{c.name}, pipeErr.Path...)
					return result, pipeErr
				}
				// Handle non-pipeline errors by wrapping them
				return result, &Error[T]{
					Timestamp: time.Now(),
					InputData: value,
					Err:       err,
					Path:      []Name{c.name},
				}
			}
		}
	}

	return result, nil
}

// Len returns the number of processors in the Sequence.
func (c *Sequence[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Clear removes all processors from the Sequence.
func (c *Sequence[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = c.processors[:0]
}

// Unshift adds processors to the front of the Sequence (runs first).
func (c *Sequence[T]) Unshift(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = slices.Insert(c.processors, 0, processors...)
}

// Push adds processors to the back of the Sequence (runs last).
func (c *Sequence[T]) Push(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// Shift removes and returns the first processor.
func (c *Sequence[T]) Shift() (Chainable[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Chainable[T]
		return zero, ErrEmptySequence
	}

	processor := c.processors[0]
	c.processors = c.processors[1:]
	return processor, nil
}

// Pop removes and returns the last processor.
func (c *Sequence[T]) Pop() (Chainable[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Chainable[T]
		return zero, ErrEmptySequence
	}

	lastIndex := len(c.processors) - 1
	processor := c.processors[lastIndex]
	c.processors = c.processors[:lastIndex]
	return processor, nil
}

// Names returns the names of all processors in order.
func (c *Sequence[T]) Names() []Name {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]Name, len(c.processors))
	for i, proc := range c.processors {
		names[i] = proc.Name()
	}
	return names
}

// Remove removes the first processor with the specified name.
func (c *Sequence[T]) Remove(name Name) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == name {
			c.processors = slices.Delete(c.processors, i, i+1)
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", name)
}

// Replace replaces the first processor with the specified name.
func (c *Sequence[T]) Replace(name Name, processor Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == name {
			c.processors[i] = processor
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", name)
}

// After inserts processors after the first processor with the specified name.
func (c *Sequence[T]) After(afterName Name, processors ...Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == afterName {
			c.processors = slices.Insert(c.processors, i+1, processors...)
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", afterName)
}

// Before inserts processors before the first processor with the specified name.
func (c *Sequence[T]) Before(beforeName Name, processors ...Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == beforeName {
			c.processors = slices.Insert(c.processors, i, processors...)
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", beforeName)
}

// Name returns the name of this sequence.
func (c *Sequence[T]) Name() Name {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.name
}

// Close gracefully shuts down the connector.
func (*Sequence[T]) Close() error {
	return nil
}
