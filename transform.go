package pipz

import "context"

// Transform creates a Processor that applies a pure transformation function to data.
// Transform is the simplest processor - use it when your operation always succeeds
// and always modifies the data in a predictable way.
//
// The transformation function cannot fail, making Transform ideal for:
//   - Data formatting (uppercase, trimming, parsing that can't fail)
//   - Mathematical calculations that can't error
//   - Field mapping or restructuring
//   - Adding computed fields
//
// If your transformation might fail (e.g., parsing, validation), use Apply instead.
// If you need conditional transformation, use Mutate.
//
// Example:
//
//	uppercase := pipz.Transform("uppercase", func(ctx context.Context, s string) string {
//	    return strings.ToUpper(s)
//	})
func Transform[T any](name Name, fn func(context.Context, T) T) Processor[T] {
	return Processor[T]{
		name: name,
		fn: func(ctx context.Context, value T) (T, *Error[T]) {
			return fn(ctx, value), nil
		},
	}
}
