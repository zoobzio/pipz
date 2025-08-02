package pipz

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Filter creates a conditional processor that either continues the pipeline unchanged
// or executes a processor based on a predicate function.
//
// Filter provides a clean way to implement conditional processing without complex
// if-else logic scattered throughout your code. When the condition returns true,
// the processor is executed. When false, data passes through unchanged with no errors.
//
// This is ideal for:
//   - Feature flags (process only for enabled users)
//   - A/B testing (apply changes to test group)
//   - Optional processing steps based on data state
//   - Business rules that apply to subset of data
//   - Conditional enrichment or validation
//   - Performance optimizations (skip expensive operations)
//
// Unlike Switch which routes to different processors, Filter either processes
// or passes through. Unlike Mutate which only supports transformations that
// cannot fail, Filter can execute any Chainable including ones that may error.
//
// Example - Feature flag processing:
//
//	enableNewFeature := pipz.NewFilter("feature-flag",
//	    func(ctx context.Context, user User) bool {
//	        return user.BetaEnabled && isFeatureEnabled(ctx, "new-algorithm")
//	    },
//	    newAlgorithmProcessor,
//	)
//
// Example - Conditional validation:
//
//	validatePremium := pipz.NewFilter("premium-validation",
//	    func(ctx context.Context, order Order) bool {
//	        return order.CustomerTier == "premium"
//	    },
//	    pipz.NewSequence("premium-checks",
//	        validateCreditLimit,
//	        checkFraudScore,
//	        verifyIdentity,
//	    ),
//	)
//
// The Filter connector is thread-safe and can be safely used in concurrent scenarios.
// The condition function and processor can be updated at runtime for dynamic behavior.
type Filter[T any] struct {
	processor Chainable[T]
	condition func(context.Context, T) bool
	name      Name
	mu        sync.RWMutex
}

// NewFilter creates a new Filter connector with the given condition and processor.
// When condition returns true, processor is executed. When false, data passes through unchanged.
func NewFilter[T any](name Name, condition func(context.Context, T) bool, processor Chainable[T]) *Filter[T] {
	return &Filter[T]{
		name:      name,
		condition: condition,
		processor: processor,
	}
}

// Process implements the Chainable interface.
// Evaluates the condition and either executes the processor or passes data through unchanged.
func (f *Filter[T]) Process(ctx context.Context, data T) (T, error) {
	f.mu.RLock()
	condition := f.condition
	processor := f.processor
	f.mu.RUnlock()

	// Evaluate condition
	if !condition(ctx, data) {
		// Condition false - pass through unchanged
		return data, nil
	}

	// Condition true - execute processor
	result, err := processor.Process(ctx, data)
	if err != nil {
		// Prepend this filter's name to the error path
		var pipeErr *Error[T]
		if errors.As(err, &pipeErr) {
			pipeErr.Path = append([]Name{f.name}, pipeErr.Path...)
			return result, pipeErr
		}
		// Wrap non-pipeline errors
		return data, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       err,
			Path:      []Name{f.name},
		}
	}
	return result, nil
}

// SetCondition updates the condition function.
// This allows for dynamic behavior changes at runtime.
func (f *Filter[T]) SetCondition(condition func(context.Context, T) bool) *Filter[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.condition = condition
	return f
}

// SetProcessor updates the processor to execute when condition is true.
// This allows for dynamic processor changes at runtime.
func (f *Filter[T]) SetProcessor(processor Chainable[T]) *Filter[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processor = processor
	return f
}

// Condition returns a copy of the current condition function.
// Note: This returns the function reference, not a deep copy.
func (f *Filter[T]) Condition() func(context.Context, T) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.condition
}

// Processor returns the current processor.
func (f *Filter[T]) Processor() Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.processor
}

// Name returns the name of this connector.
func (f *Filter[T]) Name() Name {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.name
}
