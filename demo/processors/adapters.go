package processors

import "pipz"

// Adapt converts a simple processor function into a pipz.Processor.
// This helper reduces boilerplate when registering processors that always
// modify their input values.
func Adapt[T any](fn func(T) (T, error)) pipz.Processor[T] {
	return func(value T) ([]byte, error) {
		result, err := fn(value)
		if err != nil {
			return nil, err
		}
		return pipz.Encode(result)
	}
}

// AdaptReadOnly converts a read-only processor function into a pipz.Processor.
// Use this for processors that validate or check data without modifying it.
// Returning nil avoids unnecessary re-encoding for better performance.
func AdaptReadOnly[T any](fn func(T) error) pipz.Processor[T] {
	return func(value T) ([]byte, error) {
		if err := fn(value); err != nil {
			return nil, err
		}
		return nil, nil // No modification
	}
}

// AdaptConditional converts a processor that may or may not modify data.
// The function returns (value, modified, error) where modified indicates
// whether the value was changed.
func AdaptConditional[T any](fn func(T) (T, bool, error)) pipz.Processor[T] {
	return func(value T) ([]byte, error) {
		result, modified, err := fn(value)
		if err != nil {
			return nil, err
		}
		if !modified {
			return nil, nil // No modification
		}
		return pipz.Encode(result)
	}
}