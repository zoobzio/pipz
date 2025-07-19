package pipz

import "context"

// Transform creates a processor that always modifies the value.
// Use this when your function always changes the data.
func Transform[T any](name string, fn func(context.Context, T) T) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			return fn(ctx, value), nil
		},
	}
}

// Apply creates a processor from a function that can return an error.
// This is useful when your transformation might fail.
func Apply[T any](name string, fn func(context.Context, T) (T, error)) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn:   fn,
	}
}

// Validate creates a processor that checks data without modifying it.
// Returns an error to stop the pipeline, or nil to continue.
func Validate[T any](name string, fn func(context.Context, T) error) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			if err := fn(ctx, value); err != nil {
				var zero T
				return zero, err
			}
			return value, nil
		},
	}
}

// Mutate creates a processor that conditionally modifies data.
// The transformer function is only called if the condition returns true.
func Mutate[T any](name string, transformer func(context.Context, T) T, condition func(context.Context, T) bool) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			if condition(ctx, value) {
				return transformer(ctx, value), nil
			}
			return value, nil
		},
	}
}

// Effect creates a processor for side effects like logging or metrics.
// The function can return an error to stop the pipeline, but never modifies the data.
func Effect[T any](name string, fn func(context.Context, T) error) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			if err := fn(ctx, value); err != nil {
				var zero T
				return zero, err
			}
			return value, nil
		},
	}
}

// Enrich creates a processor that fetches additional data and merges it.
// If enrichment fails, it returns the original data unchanged instead of failing the pipeline.
func Enrich[T any](name string, fn func(context.Context, T) (T, error)) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			enriched, err := fn(ctx, value)
			if err != nil {
				// Continue with original data - enrichment is best-effort
				return value, nil
			}
			return enriched, nil
		},
	}
}
