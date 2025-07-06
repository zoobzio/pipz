package pipz

// SimpleContract provides a non-global alternative to Contract.
// It holds its own pipeline internally, avoiding the global registry lookup.
// This trades discoverability for potential performance benefits.
type SimpleContract[T any] struct {
	processors []Processor[T]
}

// NewSimpleContract creates a new SimpleContract with no processors.
func NewSimpleContract[T any]() *SimpleContract[T] {
	return &SimpleContract[T]{
		processors: make([]Processor[T], 0),
	}
}

// Register adds processors to this contract's internal pipeline.
// Unlike Contract, this doesn't use the global registry.
func (sc *SimpleContract[T]) Register(processors ...Processor[T]) {
	sc.processors = append(sc.processors, processors...)
}

// Process executes all registered processors on the input value.
// This avoids the global registry lookup and mutex locking.
func (sc *SimpleContract[T]) Process(value T) (T, error) {
	// Initial encoding
	currentBytes, err := Encode(value)
	if err != nil {
		return value, err
	}
	
	// Track if any processor modified the data
	modified := false
	
	// Run all processors in sequence
	for i, proc := range sc.processors {
		// Apply the processor with current value
		result, err := proc(value)
		if err != nil {
			return value, err
		}
		
		// If processor returned bytes, it modified the value
		if result != nil {
			modified = true
			currentBytes = result
			
			// Decode for next processor (skip for last)
			if i < len(sc.processors)-1 {
				value, err = Decode[T](result)
				if err != nil {
					return value, err
				}
			}
		}
	}
	
	// Return final value
	if modified {
		return Decode[T](currentBytes)
	}
	return value, nil
}

// Link returns the contract as a Chainable interface.
func (sc *SimpleContract[T]) Link() Chainable[T] {
	return sc
}