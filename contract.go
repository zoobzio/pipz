package pipz

// Processor defines a function that processes a value of type T.
// It returns the potentially modified value and an error if processing fails.
type Processor[T any] func(T) (T, error)

// Contract provides a type-safe pipeline for processing values of type T.
// It maintains an ordered list of processors that are executed sequentially.
type Contract[T any] struct {
	processors []Processor[T]
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
	c.processors = append(c.processors, processors...)
}

// Process executes all registered processors on the input value.
// Each processor receives the output of the previous processor.
// If any processor returns an error, execution stops and the zero
// value of T is returned along with the error.
func (c *Contract[T]) Process(value T) (T, error) {
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