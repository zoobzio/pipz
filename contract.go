package pipz

import (
	"fmt"
)

// Processor defines a generic function that processes a value of type T.
// It returns encoded bytes if the value was modified, nil if unchanged,
// or an error if processing fails. This design allows read-only processors
// to avoid serialization overhead by returning nil.
type Processor[T any] func(T) ([]byte, error)

// Contract provides a type-safe interface to the byte-based processor registry.
// It uses generic types K and T to create unique pipeline identifiers,
// where K represents the contract's domain and T represents the data type.
// Contracts handle automatic serialization and deserialization using gob encoding.
type Contract[K comparable, T any] struct {
	key         K
	registryKey string
}

// GetContract retrieves or creates a Contract with the specified key and type parameters.
// The contract's registry key is computed from the type names of K and T combined
// with the key value, ensuring global uniqueness for each contract instance.
// Multiple calls with the same types and key value return contracts that access
// the same underlying pipeline.
func GetContract[K comparable, T any](key K) *Contract[K, T] {
	registryKey := Signature[K, T](key)
	return &Contract[K, T]{
		key:         key,
		registryKey: registryKey,
	}
}

// String returns the string representation of the contract,
// showing the key type, key value, and value type.
func (c *Contract[K, T]) String() string {
	return c.registryKey
}

// Register associates one or more Processor functions with this contract.
// The processors are wrapped to handle gob decoding transparently.
// Processors return nil if they don't modify the value, avoiding re-encoding.
// Processors are executed in the order they are registered.
func (c *Contract[K, T]) Register(processors ...Processor[T]) error {
	// Create a single byte processor that handles all the type-safe processors
	// This closure maintains cached state to minimize decode operations
	combinedProcessor := func(input []byte) ([]byte, error) {
		// State maintained across processor calls
		var (
			cachedValue  T
			cachedBytes  []byte
			valueDecoded bool
		)

		// Initialize with input bytes
		cachedBytes = input

		// Run all processors in sequence
		for i, proc := range processors {
			// Decode only when needed (lazy decoding)
			if !valueDecoded {
				value, err := Decode[T](cachedBytes)
				if err != nil {
					return nil, fmt.Errorf("processor %d: failed to decode: %w", i, err)
				}
				cachedValue = value
				valueDecoded = true
			}

			// Apply the processor
			result, err := proc(cachedValue)
			if err != nil {
				return nil, err
			}

			// If processor returned bytes, it modified the value
			if result != nil {
				// Update cached bytes
				cachedBytes = result
				// Mark value as needing re-decode for next processor
				valueDecoded = false
			}
			// If nil, processor didn't modify - cached value remains valid
		}

		// Return the final bytes
		return cachedBytes, nil
	}

	return Register(c.registryKey, combinedProcessor)
}

// Process executes the contract's registered processor chain on the input value.
// It handles gob encoding of the input, executes the byte processor chain,
// and decodes the result back to type T. If no processors are registered
// for this contract, an error is returned.
func (c *Contract[K, T]) Process(value T) (T, error) {
	// Encode input to bytes
	input, err := Encode(value)
	if err != nil {
		return value, fmt.Errorf("failed to encode input: %w", err)
	}

	// Process through singleton
	result, err := Process(c.registryKey, input)
	if err != nil {
		return value, err
	}

	// Decode result back to T
	output, err := Decode[T](result)
	if err != nil {
		return value, fmt.Errorf("failed to decode output: %w", err)
	}

	return output, nil
}

// Link returns the contract as a Chainable interface, allowing it to be
// composed with other contracts that process the same type T.
// This enables building complex processing workflows by chaining
// multiple contracts together.
func (c *Contract[K, T]) Link() Chainable[T] {
	return c
}
