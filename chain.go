package pipz

import (
	"slices"
	"sync"
)

// Chainable defines the interface for any component that can process
// values of type T in a chain. This interface enables composition
// of different processing components that operate on the same type.
type Chainable[T any] interface {
	Process(T) (T, error)
}

// Chain manages a sequence of Chainable processors that operate on the same type T.
// It executes processors in the order they were added, with each processor's
// output becoming the next processor's input.
type Chain[T any] struct {
	processors []Chainable[T]
	mu         sync.RWMutex
}

// NewChain creates a new, empty Chain for processing values of type T.
func NewChain[T any]() *Chain[T] {
	return &Chain[T]{
		processors: make([]Chainable[T], 0),
	}
}

// Add appends one or more Chainable processors to the chain.
// Returns the chain instance to allow method chaining.
// Processors will be executed in the order they are added.
func (c *Chain[T]) Add(processors ...Chainable[T]) *Chain[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
	return c
}

// Process executes all processors in the chain sequentially.
// Each processor receives the output of the previous processor as input.
// If any processor returns an error, execution stops immediately
// and the zero value of T is returned along with the error.
func (c *Chain[T]) Process(value T) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := value
	for _, processor := range c.processors {
		var err error
		result, err = processor.Process(result)
		if err != nil {
			var zero T
			return zero, err
		}
	}
	return result, nil
}

// Len returns the number of processors in the chain.
func (c *Chain[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// IsEmpty returns true if the chain has no processors.
func (c *Chain[T]) IsEmpty() bool {
	return c.Len() == 0
}

// Clear removes all processors from the chain.
func (c *Chain[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = c.processors[:0]
}

// PushHead adds processors to the front of the chain (runs first).
func (c *Chain[T]) PushHead(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = slices.Insert(c.processors, 0, processors...)
}

// PushTail adds processors to the back of the chain (runs last).
func (c *Chain[T]) PushTail(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// PopHead removes and returns the first processor.
func (c *Chain[T]) PopHead() (Chainable[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Chainable[T]
		return zero, ErrEmptyPipeline
	}

	processor := c.processors[0]
	c.processors = c.processors[1:]
	return processor, nil
}

// PopTail removes and returns the last processor.
func (c *Chain[T]) PopTail() (Chainable[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Chainable[T]
		return zero, ErrEmptyPipeline
	}

	lastIndex := len(c.processors) - 1
	processor := c.processors[lastIndex]
	c.processors = c.processors[:lastIndex]
	return processor, nil
}

// MoveToHead moves the processor at the given index to the front.
func (c *Chain[T]) MoveToHead(index int) error {
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
func (c *Chain[T]) MoveToTail(index int) error {
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
func (c *Chain[T]) MoveTo(from, to int) error {
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
func (c *Chain[T]) Swap(i, j int) error {
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

// Reverse reverses the order of all processors in the chain.
func (c *Chain[T]) Reverse() {
	c.mu.Lock()
	defer c.mu.Unlock()
	slices.Reverse(c.processors)
}

// InsertAt inserts processors at the specified index.
func (c *Chain[T]) InsertAt(index int, processors ...Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index > len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = slices.Insert(c.processors, index, processors...)
	return nil
}

// RemoveAt removes the processor at the specified index.
func (c *Chain[T]) RemoveAt(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = slices.Delete(c.processors, index, index+1)
	return nil
}

// ReplaceAt replaces the processor at the specified index.
func (c *Chain[T]) ReplaceAt(index int, processor Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors[index] = processor
	return nil
}
