package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Metric keys for Filter connector observability.
const (
	FilterProcessedTotal = metricz.Key("filter.processed.total")
	FilterPassedTotal    = metricz.Key("filter.passed.total")
	FilterSkippedTotal   = metricz.Key("filter.skipped.total")
)

// Span names for Filter connector.
const (
	FilterProcessSpan = tracez.Key("filter.process")
)

// Span tags for Filter connector.
const (
	FilterTagConnector    = tracez.Tag("filter.connector")
	FilterTagConditionMet = tracez.Tag("filter.condition_met")
	FilterTagSuccess      = tracez.Tag("filter.success")
	FilterTagError        = tracez.Tag("filter.error")

	// Hook event keys.
	FilterEventPassed  = hookz.Key("filter.passed")
	FilterEventSkipped = hookz.Key("filter.skipped")
)

// FilterEvent represents a filter decision event.
// This is emitted via hookz when the filter evaluates its condition,
// allowing external systems to track filtering decisions.
type FilterEvent struct {
	Name          Name          // Connector name
	ConditionMet  bool          // Whether the condition was met
	Passed        bool          // Whether data passed to processor
	Skipped       bool          // Whether data was skipped
	ProcessorName Name          // Name of processor (if executed)
	Success       bool          // Whether processor succeeded (if executed)
	Error         error         // Error if processor failed
	Duration      time.Duration // Processing time (if executed)
	Timestamp     time.Time     // When the event occurred
}

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
//
// # Observability
//
// Filter provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - filter.processed.total: Counter of filter evaluations
//   - filter.passed.total: Counter of items that met condition and were processed
//   - filter.skipped.total: Counter of items that didn't meet condition
//
// Traces:
//   - filter.process: Span for filter evaluation and processing
//
// Events (via hooks):
//   - filter.passed: Fired when condition is true and processor executes
//   - filter.skipped: Fired when condition is false and data passes through
//
// Example with hooks:
//
//	filter := pipz.NewFilter("premium-only",
//	    func(ctx context.Context, user User) bool {
//	        return user.IsPremium
//	    },
//	    premiumProcessor,
//	)
//
//	// Track filtering decisions
//	filter.OnSkipped(func(ctx context.Context, event FilterEvent) error {
//	    log.Debug("Non-premium user skipped: %s", user.ID)
//	    metrics.Inc("filter.non_premium.skipped")
//	    return nil
//	})
//
//	// Monitor premium processing
//	filter.OnPassed(func(ctx context.Context, event FilterEvent) error {
//	    if !event.Success {
//	        alert.Warn("Premium processing failed: %v", event.Error)
//	    }
//	    return nil
//	})
type Filter[T any] struct {
	processor Chainable[T]
	condition func(context.Context, T) bool
	name      Name
	mu        sync.RWMutex

	// Observability
	metrics *metricz.Registry
	tracer  *tracez.Tracer
	hooks   *hookz.Hooks[FilterEvent]
}

// NewFilter creates a new Filter connector with the given condition and processor.
// When condition returns true, processor is executed. When false, data passes through unchanged.
func NewFilter[T any](name Name, condition func(context.Context, T) bool, processor Chainable[T]) *Filter[T] {
	// Initialize observability components
	registry := metricz.New()
	tracer := tracez.New()

	// Register metrics
	registry.Counter(FilterProcessedTotal)
	registry.Counter(FilterPassedTotal)
	registry.Counter(FilterSkippedTotal)

	return &Filter[T]{
		name:      name,
		condition: condition,
		processor: processor,
		metrics:   registry,
		tracer:    tracer,
		hooks:     hookz.New[FilterEvent](),
	}
}

// Process implements the Chainable interface.
// Evaluates the condition and either executes the processor or passes data through unchanged.
func (f *Filter[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, f.name, data)

	f.mu.RLock()
	condition := f.condition
	processor := f.processor
	f.mu.RUnlock()

	// Start span for filter operation
	ctx, span := f.tracer.StartSpan(ctx, FilterProcessSpan)
	defer span.Finish()
	span.SetTag(FilterTagConnector, string(f.name))

	// Increment processed counter
	f.metrics.Counter(FilterProcessedTotal).Inc()

	// Evaluate condition
	conditionMet := condition(ctx, data)
	span.SetTag(FilterTagConditionMet, fmt.Sprintf("%t", conditionMet))

	if !conditionMet {
		// Condition false - pass through unchanged
		f.metrics.Counter(FilterSkippedTotal).Inc()
		span.SetTag(FilterTagSuccess, "true")

		// Emit skipped event
		_ = f.hooks.Emit(ctx, FilterEventSkipped, FilterEvent{ //nolint:errcheck
			Name:         f.name,
			ConditionMet: false,
			Passed:       false,
			Skipped:      true,
			Timestamp:    time.Now(),
		})

		return data, nil
	}

	// Condition true - execute processor
	f.metrics.Counter(FilterPassedTotal).Inc()

	processorStart := time.Now()
	result, err = processor.Process(ctx, data)
	processorDuration := time.Since(processorStart)
	if err != nil {
		span.SetTag(FilterTagSuccess, "false")
		span.SetTag(FilterTagError, err.Error())

		// Emit passed event with error
		_ = f.hooks.Emit(ctx, FilterEventPassed, FilterEvent{ //nolint:errcheck
			Name:          f.name,
			ConditionMet:  true,
			Passed:        true,
			Skipped:       false,
			ProcessorName: processor.Name(),
			Success:       false,
			Error:         err,
			Duration:      processorDuration,
			Timestamp:     time.Now(),
		})

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

	// Emit passed event with success
	_ = f.hooks.Emit(ctx, FilterEventPassed, FilterEvent{ //nolint:errcheck
		Name:          f.name,
		ConditionMet:  true,
		Passed:        true,
		Skipped:       false,
		ProcessorName: processor.Name(),
		Success:       true,
		Error:         nil,
		Duration:      processorDuration,
		Timestamp:     time.Now(),
	})

	span.SetTag(FilterTagSuccess, "true")
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

// Metrics returns the metrics registry for this connector.
func (f *Filter[T]) Metrics() *metricz.Registry {
	return f.metrics
}

// Tracer returns the tracer for this connector.
func (f *Filter[T]) Tracer() *tracez.Tracer {
	return f.tracer
}

// Close gracefully shuts down observability components.
func (f *Filter[T]) Close() error {
	if f.tracer != nil {
		f.tracer.Close()
	}
	f.hooks.Close()
	return nil
}

// OnPassed registers a handler for when the condition is true and processor executes.
// The handler is called asynchronously after the processor completes (success or failure).
func (f *Filter[T]) OnPassed(handler func(context.Context, FilterEvent) error) error {
	_, err := f.hooks.Hook(FilterEventPassed, handler)
	return err
}

// OnSkipped registers a handler for when the condition is false and data passes through.
// The handler is called asynchronously when the filter skips processing.
func (f *Filter[T]) OnSkipped(handler func(context.Context, FilterEvent) error) error {
	_, err := f.hooks.Hook(FilterEventSkipped, handler)
	return err
}
