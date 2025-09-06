package pipz

import "context"

// Mutate creates a Processor that conditionally transforms data based on a predicate.
// Mutate combines a condition check with a transformation, applying the transformer
// only when the condition returns true. When false, data passes through unchanged.
//
// This pattern is cleaner than embedding if-statements in Transform functions and
// makes the condition explicit and testable. Use Mutate for:
//   - Feature flags (transform only for enabled users)
//   - A/B testing (apply changes to test group)
//   - Conditional formatting based on data values
//   - Environment-specific transformations
//   - Business rules that apply to subset of data
//
// The condition and transformer are separate functions for better testability
// and reusability. The transformer cannot fail - use Apply with conditional
// logic if you need error handling.
//
// Example:
//
//	discountPremium := pipz.Mutate("premium_discount",
//	    func(ctx context.Context, order Order) Order {
//	        order.Total *= 0.9  // 10% discount
//	        return order
//	    },
//	    func(ctx context.Context, order Order) bool {
//	        return order.CustomerTier == "premium" && order.Total > 100
//	    },
//	)
func Mutate[T any](name Name, transformer func(context.Context, T) T, condition func(context.Context, T) bool) Processor[T] {
	return Processor[T]{
		name: name,
		fn: func(ctx context.Context, value T) (result T, err error) {
			defer recoverFromPanic(&result, &err, name, value)
			if condition(ctx, value) {
				result = transformer(ctx, value)
			} else {
				result = value
			}
			return result, nil
		},
	}
}
