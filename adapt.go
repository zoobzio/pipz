package pipz

// Transform creates a processor that always modifies the value.
// Use this when your function always changes the data.
func Transform[T any](fn func(T) T) Processor[T] {
	return func(value T) (T, error) {
		return fn(value), nil
	}
}

// Apply creates a processor from a function that can return an error.
// This is useful when your transformation might fail.
func Apply[T any](fn func(T) (T, error)) Processor[T] {
	return fn // Direct pass-through, no wrapping needed
}

// Validate creates a processor that checks data without modifying it.
// Returns an error to stop the pipeline, or nil to continue.
func Validate[T any](fn func(T) error) Processor[T] {
	return func(value T) (T, error) {
		if err := fn(value); err != nil {
			var zero T
			return zero, err
		}
		return value, nil
	}
}

// Mutate creates a processor that conditionally modifies data.
// The transformer function is only called if the condition returns true.
func Mutate[T any](transformer func(T) T, condition func(T) bool) Processor[T] {
	return func(value T) (T, error) {
		if condition(value) {
			return transformer(value), nil
		}
		return value, nil
	}
}

// Effect creates a processor for side effects like logging or metrics.
// The function can return an error to stop the pipeline, but never modifies the data.
func Effect[T any](fn func(T) error) Processor[T] {
	return func(value T) (T, error) {
		if err := fn(value); err != nil {
			var zero T
			return zero, err
		}
		return value, nil
	}
}

// Enrich creates a processor that fetches additional data and merges it.
// If enrichment fails, it returns the original data unchanged instead of failing the pipeline.
func Enrich[T any](fn func(T) (T, error)) Processor[T] {
	return func(value T) (T, error) {
		enriched, err := fn(value)
		if err != nil {
			// Continue with original data - enrichment is best-effort
			return value, nil
		}
		return enriched, nil
	}
}
