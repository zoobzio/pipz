package pipz

import (
	"context"
)

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
//	var UppercaseID = pipz.NewIdentity("uppercase", "Converts text to uppercase")
//	uppercase := pipz.Transform(UppercaseID, func(ctx context.Context, s string) string {
//	    return strings.ToUpper(s)
//	})
func Transform[T any](identity Identity, fn func(context.Context, T) T) Processor[T] {
	return Processor[T]{
		identity: identity,
		fn: func(ctx context.Context, value T) (result T, err error) {
			defer recoverFromPanic(&result, &err, identity, value)
			result = fn(ctx, value)
			return result, nil
		},
	}
}
