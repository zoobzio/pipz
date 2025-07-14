package pipz

import (
	"errors"
	"slices"
	"sync"
)

// Pipeline modification errors.
var (
	ErrIndexOutOfBounds = errors.New("index out of bounds")
	ErrEmptyPipeline    = errors.New("pipeline is empty")
	ErrInvalidRange     = errors.New("invalid range")
)

// Processor defines a function that processes a value of type T.
// It returns the potentially modified value and an error if processing fails.
type Processor[T any] func(T) (T, error)

// Contract provides a type-safe pipeline for processing values of type T.
// It maintains an ordered list of processors that are executed sequentially.
type Contract[T any] struct {
	processors []Processor[T]
	mu         sync.RWMutex
}

// NewContract creates a new Contract with no processors.
func NewContract[T any]() *Contract[T] {
	return &Contract[T]{
		processors: make([]Processor[T], 0),
	}
}

// Register adds processors to this contract's pipeline.
// Processors are executed in the order they are registered.
func (c *Contract[T]) Register(processors ...Processor[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// Process executes all registered processors on the input value.
// Each processor receives the output of the previous processor.
// If any processor returns an error, execution stops and the zero
// value of T is returned along with the error.
func (c *Contract[T]) Process(value T) (T, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := value
	for _, proc := range c.processors {
		var err error
		result, err = proc(result)
		if err != nil {
			var zero T
			return zero, err
		}
	}
	return result, nil
}

// Link returns the contract as a Chainable interface, allowing it to be
// composed with other contracts that process the same type T.
func (c *Contract[T]) Link() Chainable[T] {
	return c
}

// Len returns the number of processors in the contract.
func (c *Contract[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// IsEmpty returns true if the contract has no processors.
func (c *Contract[T]) IsEmpty() bool {
	return c.Len() == 0
}

// Clear removes all processors from the contract.
func (c *Contract[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = c.processors[:0]
}

// PushHead adds processors to the front of the contract (runs first).
func (c *Contract[T]) PushHead(processors ...Processor[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = slices.Insert(c.processors, 0, processors...)
}

// PushTail adds processors to the back of the contract (runs last).
func (c *Contract[T]) PushTail(processors ...Processor[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// PopHead removes and returns the first processor.
func (c *Contract[T]) PopHead() (Processor[T], error) {
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
func (c *Contract[T]) PopTail() (Processor[T], error) {
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
func (c *Contract[T]) MoveToHead(index int) error {
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
func (c *Contract[T]) MoveToTail(index int) error {
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
func (c *Contract[T]) MoveTo(from, to int) error {
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
func (c *Contract[T]) Swap(i, j int) error {
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

// Reverse reverses the order of all processors in the contract.
func (c *Contract[T]) Reverse() {
	c.mu.Lock()
	defer c.mu.Unlock()
	slices.Reverse(c.processors)
}

// InsertAt inserts processors at the specified index.
func (c *Contract[T]) InsertAt(index int, processors ...Processor[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index > len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = slices.Insert(c.processors, index, processors...)
	return nil
}

// RemoveAt removes the processor at the specified index.
func (c *Contract[T]) RemoveAt(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = slices.Delete(c.processors, index, index+1)
	return nil
}

// ReplaceAt replaces the processor at the specified index.
func (c *Contract[T]) ReplaceAt(index int, processor Processor[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors[index] = processor
	return nil
}
