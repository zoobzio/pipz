package pipz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// Pipeline modification errors.
var (
	ErrIndexOutOfBounds = errors.New("index out of bounds")
	ErrEmptyPipeline    = errors.New("pipeline is empty")
	ErrInvalidRange     = errors.New("invalid range")
)

// Processor defines a named processing stage that transforms a value of type T.
// It contains a descriptive name for debugging and a function that processes the value.
// The function receives a context for cancellation and timeout control.
//
// Processor is the basic building block created by adapter functions like
// Apply, Transform, Effect, and others. The name field is crucial for debugging,
// appearing in error messages and logs to identify exactly where failures occur.
//
// Best practices for processor names:
//   - Use descriptive, action-oriented names ("validate_email", not "email")
//   - Include the operation type ("parse_json", "fetch_user", "log_event")
//   - Keep names concise but meaningful
//   - Use consistent naming conventions across your application
type Processor[T any] struct {
	Fn   func(context.Context, T) (T, error)
	Name string
}

// Process implements the Chainable interface, allowing individual processors
// to be used directly in connectors.
//
// This means a single Processor can be used anywhere a Chainable is expected:
//
//	validator := pipz.Effect("validate", validateFunc)
//	// Can be used directly
//	result, err := validator.Process(ctx, data)
//	// Or in connectors
//	pipeline := pipz.Sequential(validator, transformer)
func (p Processor[T]) Process(ctx context.Context, data T) (T, error) {
	return p.Fn(ctx, data)
}

// Pipeline provides a type-safe pipeline for processing values of type T.
// It maintains an ordered list of processors that are executed sequentially.
//
// Pipeline offers a richer API than simple Sequential composition, with
// methods to dynamically modify the processor chain. This makes it ideal
// for scenarios where the processing steps need to be configured at runtime
// or modified based on conditions.
//
// Key features:
//   - Thread-safe for concurrent access
//   - Dynamic modification of processor chain
//   - Named processors for debugging
//   - Rich API for reordering and modification
//   - Fail-fast execution with detailed errors
//
// Use Pipeline when you need runtime control over processing steps.
// Use Sequential when the steps are fixed at compile time.
type Pipeline[T any] struct {
	processors []Processor[T]
	mu         sync.RWMutex
}

// NewPipeline creates a new Pipeline with no processors.
// The pipeline is ready to use immediately and can be safely
// accessed concurrently. Processors can be added using Register
// or the various modification methods.
//
// Example:
//
//	pipeline := pipz.NewPipeline[User]()
//	pipeline.Register(
//	    pipz.Effect("validate", validateUser),
//	    pipz.Apply("enrich", enrichUser),
//	    pipz.Effect("audit", auditUser),
//	)
func NewPipeline[T any]() *Pipeline[T] {
	return &Pipeline[T]{
		processors: make([]Processor[T], 0),
	}
}

// Register adds processors to this Pipeline's pipeline.
// Processors are executed in the order they are registered.
//
// This method is thread-safe and can be called concurrently.
// New processors are appended to the existing chain, making
// Register ideal for building pipelines incrementally:
//
//	pipeline := pipz.NewPipeline[Order]()
//	pipeline.Register(validateOrder)
//	pipeline.Register(calculateTax, applyDiscount)
//	if config.RequiresApproval {
//	    pipeline.Register(requireApproval)
//	}
func (c *Pipeline[T]) Register(processors ...Processor[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// Process executes all registered processors on the input value.
// Each processor receives the output of the previous processor.
// The context is checked before each processor execution - if the context
// is canceled or expired, processing stops immediately.
// If any processor returns an error, execution stops and a PipelineError
// is returned with rich debugging information.
//
// Process is thread-safe and can be called concurrently. The pipeline's
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
func (c *Pipeline[T]) Process(ctx context.Context, value T) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := value
	startTime := time.Now()

	for i, proc := range c.processors {
		// Pipeline-level check before starting processor
		select {
		case <-ctx.Done():
			var zero T
			return zero, &PipelineError[T]{
				Err:           ctx.Err(),
				ProcessorName: proc.Name,
				StageIndex:    i,
				InputData:     result,
				Timeout:       errors.Is(ctx.Err(), context.DeadlineExceeded),
				Canceled:      errors.Is(ctx.Err(), context.Canceled),
				Timestamp:     time.Now(),
				Duration:      time.Since(startTime),
			}
		default:
			// Pass context to processor for its own checking
			procStart := time.Now()
			var err error
			result, err = proc.Fn(ctx, result)
			if err != nil {
				var zero T
				return zero, &PipelineError[T]{
					Err:           err,
					ProcessorName: proc.Name,
					StageIndex:    i,
					InputData:     result,
					Timeout:       errors.Is(err, context.DeadlineExceeded),
					Canceled:      errors.Is(err, context.Canceled),
					Timestamp:     time.Now(),
					Duration:      time.Since(procStart),
				}
			}
		}
	}
	return result, nil
}

// Link returns the Pipeline as a Chainable interface, allowing it to be
// composed with other Pipelines that process the same type T.
//
// This enables powerful composition patterns where entire pipelines
// become building blocks in larger systems:
//
//	validationPipeline := createValidationPipeline()
//	processingPipeline := createProcessingPipeline()
//
//	combined := pipz.Sequential(
//	    validationPipeline.Link(),
//	    processingPipeline.Link(),
//	    pipz.Effect("log_completion", logCompletion),
//	)
func (c *Pipeline[T]) Link() Chainable[T] {
	return c
}

// Len returns the number of processors in the Pipeline.
func (c *Pipeline[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// IsEmpty returns true if the Pipeline has no processors.
func (c *Pipeline[T]) IsEmpty() bool {
	return c.Len() == 0
}

// Clear removes all processors from the Pipeline.
func (c *Pipeline[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = c.processors[:0]
}

// PushHead adds processors to the front of the Pipeline (runs first).
func (c *Pipeline[T]) PushHead(processors ...Processor[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = slices.Insert(c.processors, 0, processors...)
}

// PushTail adds processors to the back of the Pipeline (runs last).
func (c *Pipeline[T]) PushTail(processors ...Processor[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// PopHead removes and returns the first processor.
func (c *Pipeline[T]) PopHead() (Processor[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Processor[T]
		return zero, ErrEmptyPipeline
	}

	processor := c.processors[0]
	c.processors = c.processors[1:]
	return processor, nil
}

// PopTail removes and returns the last processor.
func (c *Pipeline[T]) PopTail() (Processor[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Processor[T]
		return zero, ErrEmptyPipeline
	}

	lastIndex := len(c.processors) - 1
	processor := c.processors[lastIndex]
	c.processors = c.processors[:lastIndex]
	return processor, nil
}

// MoveToHead moves the processor at the given index to the front.
func (c *Pipeline[T]) MoveToHead(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	if index == 0 {
		return nil
	}

	processor := c.processors[index]
	c.processors = slices.Delete(c.processors, index, index+1)
	c.processors = slices.Insert(c.processors, 0, processor)
	return nil
}

// MoveToTail moves the processor at the given index to the back.
func (c *Pipeline[T]) MoveToTail(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	lastIndex := len(c.processors) - 1
	if index == lastIndex {
		return nil
	}

	processor := c.processors[index]
	c.processors = slices.Delete(c.processors, index, index+1)
	c.processors = append(c.processors, processor)
	return nil
}

// MoveTo moves the processor from the 'from' index to the 'to' index.
func (c *Pipeline[T]) MoveTo(from, to int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if from < 0 || from >= len(c.processors) || to < 0 || to >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	if from == to {
		return nil
	}

	processor := c.processors[from]
	c.processors = slices.Delete(c.processors, from, from+1)

	if from < to {
		to--
	}

	c.processors = slices.Insert(c.processors, to, processor)
	return nil
}

// Swap exchanges the processors at indices i and j.
func (c *Pipeline[T]) Swap(i, j int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if i < 0 || i >= len(c.processors) || j < 0 || j >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	if i != j {
		c.processors[i], c.processors[j] = c.processors[j], c.processors[i]
	}
	return nil
}

// Reverse reverses the order of all processors in the Pipeline.
func (c *Pipeline[T]) Reverse() {
	c.mu.Lock()
	defer c.mu.Unlock()
	slices.Reverse(c.processors)
}

// InsertAt inserts processors at the specified index.
func (c *Pipeline[T]) InsertAt(index int, processors ...Processor[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index > len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = slices.Insert(c.processors, index, processors...)
	return nil
}

// RemoveAt removes the processor at the specified index.
func (c *Pipeline[T]) RemoveAt(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = slices.Delete(c.processors, index, index+1)
	return nil
}

// ReplaceAt replaces the processor at the specified index.
func (c *Pipeline[T]) ReplaceAt(index int, processor Processor[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors[index] = processor
	return nil
}

// Names returns the names of all processors in order.
func (c *Pipeline[T]) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, len(c.processors))
	for i, proc := range c.processors {
		names[i] = proc.Name
	}
	return names
}

// Find returns the first processor with the specified name.
func (c *Pipeline[T]) Find(name string) (Processor[T], error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, proc := range c.processors {
		if proc.Name == name {
			return proc, nil
		}
	}

	var zero Processor[T]
	return zero, fmt.Errorf("processor %q not found", name)
}
