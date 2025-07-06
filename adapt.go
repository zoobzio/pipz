package pipz

// Transform creates a processor that always modifies and encodes the result.
// This is the most common adapter - use it when your function always changes the data.
func Transform[T any](fn func(T) T) Processor[T] {
	return func(value T) ([]byte, error) {
		result := fn(value)
		return Encode(result)
	}
}

// Apply creates a processor from a function that can return an error.
// This is useful when your transformation might fail (e.g., validation with modification).
// The result is always encoded if successful.
func Apply[T any](fn func(T) (T, error)) Processor[T] {
	return func(value T) ([]byte, error) {
		result, err := fn(value)
		if err != nil {
			return nil, err
		}
		return Encode(result)
	}
}

// Validate creates a processor that checks data without modifying it.
// Returns an error to stop the pipeline, or nil to continue.
// No encoding happens, making this efficient for validation-only steps.
func Validate[T any](fn func(T) error) Processor[T] {
	return func(value T) ([]byte, error) {
		if err := fn(value); err != nil {
			return nil, err
		}
		return nil, nil // No modification
	}
}

// Mutate creates a processor that conditionally modifies data.
// The transformer function is only called if the condition returns true.
// This is useful for business rules that apply only in certain cases.
func Mutate[T any](transformer func(T) T, condition func(T) bool) Processor[T] {
	return func(value T) ([]byte, error) {
		if condition(value) {
			result := transformer(value)
			return Encode(result)
		}
		return nil, nil // No modification
	}
}

// Effect creates a processor for side effects like logging, metrics, or notifications.
// The function can return an error to stop the pipeline, but never modifies the data.
// Use this for operations that need to happen but don't change the data flow.
func Effect[T any](fn func(T) error) Processor[T] {
	return func(value T) ([]byte, error) {
		if err := fn(value); err != nil {
			return nil, err
		}
		return nil, nil // Never modifies
	}
}


// Enrich creates a processor that fetches additional data and merges it.
// If enrichment fails, it returns the original data unchanged instead of failing the pipeline.
// This is useful for optional data enhancement that shouldn't break processing.
func Enrich[T any](fn func(T) (T, error)) Processor[T] {
	return func(value T) ([]byte, error) {
		enriched, err := fn(value)
		if err != nil {
			// Continue with original data - enrichment is best-effort
			return nil, nil
		}
		return Encode(enriched)
	}
}