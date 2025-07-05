package pipz

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
	c.processors = append(c.processors, processors...)
	return c
}

// Process executes all processors in the chain sequentially.
// Each processor receives the output of the previous processor as input.
// If any processor returns an error, execution stops immediately
// and the original input value is returned along with the error.
func (c *Chain[T]) Process(value T) (T, error) {
	result := value
	for _, processor := range c.processors {
		var err error
		result, err = processor.Process(result)
		if err != nil {
			return value, err
		}
	}
	return result, nil
}