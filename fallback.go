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

// Observability constants for the Fallback connector.
const (
	// Metrics.
	FallbackProcessedTotal = metricz.Key("fallback.processed.total")
	FallbackSuccessesTotal = metricz.Key("fallback.successes.total")
	FallbackAllFailedTotal = metricz.Key("fallback.all_failed.total")
	FallbackAttemptsTotal  = metricz.Key("fallback.attempts.total")
	FallbackProcessorCount = metricz.Key("fallback.processor.count")
	FallbackDurationMs     = metricz.Key("fallback.duration.ms")

	// Spans.
	FallbackProcessSpan = tracez.Key("fallback.process")
	FallbackAttemptSpan = tracez.Key("fallback.attempt")

	// Tags.
	FallbackTagProcessorCount      = tracez.Tag("fallback.processor_count")
	FallbackTagProcessorName       = tracez.Tag("fallback.processor_name")
	FallbackTagAttemptNumber       = tracez.Tag("fallback.attempt_number")
	FallbackTagSuccess             = tracez.Tag("fallback.success")
	FallbackTagSuccessfulProcessor = tracez.Tag("fallback.successful_processor")
	FallbackTagError               = tracez.Tag("fallback.error")

	// Hook event keys.
	FallbackEventActivated = hookz.Key("fallback.activated")
	FallbackEventExhausted = hookz.Key("fallback.exhausted")
	FallbackEventRecovered = hookz.Key("fallback.recovered")
)

// FallbackEvent represents a fallback event.
// This is emitted via hookz when fallback processing occurs,
// allowing external systems to monitor service degradation and recovery patterns.
type FallbackEvent struct {
	Name            Name          // Connector name
	PrimaryFailed   Name          // Name of processor that failed (if not first success)
	FallbackUsed    Name          // Name of processor that succeeded (if any)
	AttemptNumber   int           // Which attempt in chain (1-based)
	TotalProcessors int           // Total processors available
	AllFailed       bool          // Whether all fallbacks were exhausted
	Recovered       bool          // Whether a fallback successfully recovered
	Duration        time.Duration // How long this attempt took
	Error           error         // Error from failed processor
	Timestamp       time.Time     // When the event occurred
}

// Fallback attempts processors in order, falling back to the next on error.
// Fallback provides automatic failover through a chain of alternative processors
// when earlier ones fail. This creates resilient processing chains that can recover
// from failures gracefully.
//
// Unlike Retry which attempts the same operation multiple times,
// Fallback switches to completely different implementations. Each processor
// is tried in order until one succeeds or all fail.
//
// Common use cases:
//   - Primary/backup/tertiary service failover
//   - Graceful degradation strategies
//   - Multiple payment provider support
//   - Cache miss handling (try local cache, then redis, then database)
//   - API version compatibility chains
//
// Example:
//
//	fallback := pipz.NewFallback("payment-providers",
//	    stripeProcessor,       // Try Stripe first
//	    paypalProcessor,       // Fall back to PayPal on error
//	    squareProcessor,       // Finally try Square
//	)
//
// IMPORTANT: Avoid circular references between Fallback instances when all processors fail.
// Example of DANGEROUS pattern:
//
//	fallback1 → fallback2 → fallback3 → fallback1
//
// This creates infinite recursion risk if all processors fail, leading to stack overflow.
//
// # Observability
//
// Fallback provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - fallback.processed.total: Counter of fallback operations
//   - fallback.successes.total: Counter of successful completions
//   - fallback.all_failed.total: Counter of completely exhausted fallback chains
//   - fallback.attempts.total: Counter of individual processor attempts
//   - fallback.processor.count: Gauge of processors in the chain
//   - fallback.duration.ms: Gauge of total operation duration
//
// Traces:
//   - fallback.process: Parent span for entire fallback operation
//   - fallback.attempt: Child span for each processor attempt
//
// Events (via hooks):
//   - fallback.activated: Fired when switching to a fallback processor
//   - fallback.recovered: Fired when a non-primary processor succeeds
//   - fallback.exhausted: Fired when all processors fail
//
// Example with hooks:
//
//	fallback := pipz.NewFallback("payment",
//	    stripeProcessor,
//	    paypalProcessor,
//	    squareProcessor,
//	)
//
//	// Monitor service degradation
//	fallback.OnActivated(func(ctx context.Context, event FallbackEvent) error {
//	    metrics.Inc("payment.fallback.activated", event.PrimaryFailed)
//	    log.Warn("Payment processor %s failed, trying %d/%d",
//	        event.PrimaryFailed, event.AttemptNumber, event.TotalProcessors)
//	    return nil
//	})
//
//	// Alert when completely exhausted
//	fallback.OnExhausted(func(ctx context.Context, event FallbackEvent) error {
//	    alert.Critical("All %d payment processors failed", event.TotalProcessors)
//	    return nil
//	})
type Fallback[T any] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[FallbackEvent]
}

// NewFallback creates a new Fallback connector that tries processors in order.
// At least one processor must be provided. Each processor is tried in order
// until one succeeds or all fail.
//
// Examples:
//
//	fallback := pipz.NewFallback("payment", stripe, paypal, square)
//	fallback := pipz.NewFallback("cache", redis, database)
func NewFallback[T any](name Name, processors ...Chainable[T]) *Fallback[T] {
	if len(processors) == 0 {
		panic("NewFallback requires at least one processor")
	}

	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(FallbackProcessedTotal)
	metrics.Counter(FallbackSuccessesTotal)
	metrics.Counter(FallbackAllFailedTotal)
	metrics.Counter(FallbackAttemptsTotal)
	metrics.Gauge(FallbackProcessorCount)
	metrics.Gauge(FallbackDurationMs)

	return &Fallback[T]{
		name:       name,
		processors: processors,
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[FallbackEvent](),
	}
}

// Process implements the Chainable interface.
// Tries each processor in order until one succeeds or all fail.
func (f *Fallback[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, f.name, data)

	f.mu.RLock()
	processors := make([]Chainable[T], len(f.processors))
	copy(processors, f.processors)
	f.mu.RUnlock()

	// Track metrics
	f.metrics.Counter(FallbackProcessedTotal).Inc()
	f.metrics.Gauge(FallbackProcessorCount).Set(float64(len(processors)))
	start := time.Now()

	// Start main span
	ctx, span := f.tracer.StartSpan(ctx, FallbackProcessSpan)
	span.SetTag(FallbackTagProcessorCount, fmt.Sprintf("%d", len(processors)))
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		f.metrics.Gauge(FallbackDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(FallbackTagSuccess, "true")
			f.metrics.Counter(FallbackSuccessesTotal).Inc()
		} else {
			span.SetTag(FallbackTagSuccess, "false")
			if err != nil {
				span.SetTag(FallbackTagError, err.Error())
			}
		}
		span.Finish()
	}()

	var lastErr error

	for i, processor := range processors {
		// Start span for this attempt
		attemptCtx, attemptSpan := f.tracer.StartSpan(ctx, FallbackAttemptSpan)
		attemptSpan.SetTag(FallbackTagProcessorName, string(processor.Name()))
		attemptSpan.SetTag(FallbackTagAttemptNumber, fmt.Sprintf("%d", i+1))

		f.metrics.Counter(FallbackAttemptsTotal).Inc()

		attemptStart := time.Now()
		result, err := processor.Process(attemptCtx, data)
		attemptDuration := time.Since(attemptStart)

		if err == nil {
			// Success! Return immediately
			attemptSpan.SetTag(FallbackTagSuccess, "true")
			attemptSpan.Finish()
			span.SetTag(FallbackTagSuccessfulProcessor, string(processor.Name()))

			// If this isn't the first processor, emit recovery event
			if i > 0 {
				if f.hooks.ListenerCount(FallbackEventRecovered) > 0 {
					_ = f.hooks.Emit(ctx, FallbackEventRecovered, FallbackEvent{ //nolint:errcheck
						Name:            f.name,
						PrimaryFailed:   processors[0].Name(),
						FallbackUsed:    processor.Name(),
						AttemptNumber:   i + 1,
						TotalProcessors: len(processors),
						Recovered:       true,
						Duration:        attemptDuration,
						Timestamp:       time.Now(),
					})
				}
			}

			return result, nil
		}

		// Failed, record error
		attemptSpan.SetTag(FallbackTagSuccess, "false")
		attemptSpan.SetTag(FallbackTagError, err.Error())
		attemptSpan.Finish()

		// Store the error for potential return
		lastErr = err

		// If this isn't the last processor, emit fallback activation event
		if i < len(processors)-1 {
			if f.hooks.ListenerCount(FallbackEventActivated) > 0 {
				_ = f.hooks.Emit(ctx, FallbackEventActivated, FallbackEvent{ //nolint:errcheck
					Name:            f.name,
					PrimaryFailed:   processor.Name(),
					AttemptNumber:   i + 1,
					TotalProcessors: len(processors),
					Duration:        attemptDuration,
					Error:           err,
					Timestamp:       time.Now(),
				})
			}
		}

		// Continue to next processor (if any)
	}

	// All processors failed, return the last error with path
	if lastErr != nil {
		f.metrics.Counter(FallbackAllFailedTotal).Inc()

		// Emit exhausted event
		if f.hooks.ListenerCount(FallbackEventExhausted) > 0 {
			_ = f.hooks.Emit(ctx, FallbackEventExhausted, FallbackEvent{ //nolint:errcheck
				Name:            f.name,
				AttemptNumber:   len(processors),
				TotalProcessors: len(processors),
				AllFailed:       true,
				Duration:        time.Since(start),
				Error:           lastErr,
				Timestamp:       time.Now(),
			})
		}

		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			pipeErr.Path = append([]Name{f.name}, pipeErr.Path...)
			return data, pipeErr
		}
		// Wrap non-pipeline errors
		return data, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       lastErr,
			Path:      []Name{f.name},
		}
	}
	return data, nil
}

// SetProcessors replaces all processors with the provided ones.
func (f *Fallback[T]) SetProcessors(processors ...Chainable[T]) *Fallback[T] {
	if len(processors) == 0 {
		panic("SetProcessors requires at least one processor")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processors = make([]Chainable[T], len(processors))
	copy(f.processors, processors)
	return f
}

// AddFallback appends a processor to the end of the fallback chain.
func (f *Fallback[T]) AddFallback(processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processors = append(f.processors, processor)
	return f
}

// InsertAt inserts a processor at the specified index.
func (f *Fallback[T]) InsertAt(index int, processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index > len(f.processors) {
		panic("index out of bounds")
	}
	f.processors = append(f.processors[:index], append([]Chainable[T]{processor}, f.processors[index:]...)...)
	return f
}

// RemoveAt removes the processor at the specified index.
func (f *Fallback[T]) RemoveAt(index int) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.processors) {
		panic("index out of bounds")
	}
	if len(f.processors) == 1 {
		panic("cannot remove last processor from fallback")
	}
	f.processors = append(f.processors[:index], f.processors[index+1:]...)
	return f
}

// Name returns the name of this connector.
func (f *Fallback[T]) Name() Name {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.name
}

// Metrics returns the metrics registry for this connector.
func (f *Fallback[T]) Metrics() *metricz.Registry {
	return f.metrics
}

// Tracer returns the tracer for this connector.
func (f *Fallback[T]) Tracer() *tracez.Tracer {
	return f.tracer
}

// Close gracefully shuts down observability components.
func (f *Fallback[T]) Close() error {
	if f.tracer != nil {
		f.tracer.Close()
	}
	f.hooks.Close()
	return nil
}

// OnActivated registers a handler for when a fallback processor is activated.
// The handler is called asynchronously when a processor fails and the next one is tried.
func (f *Fallback[T]) OnActivated(handler func(context.Context, FallbackEvent) error) error {
	_, err := f.hooks.Hook(FallbackEventActivated, handler)
	return err
}

// OnRecovered registers a handler for when a fallback successfully recovers.
// The handler is called asynchronously when a non-primary processor succeeds.
func (f *Fallback[T]) OnRecovered(handler func(context.Context, FallbackEvent) error) error {
	_, err := f.hooks.Hook(FallbackEventRecovered, handler)
	return err
}

// OnExhausted registers a handler for when all fallback options are exhausted.
// The handler is called asynchronously when every processor in the chain fails.
func (f *Fallback[T]) OnExhausted(handler func(context.Context, FallbackEvent) error) error {
	_, err := f.hooks.Hook(FallbackEventExhausted, handler)
	return err
}

// GetProcessors returns a copy of all processors in order.
func (f *Fallback[T]) GetProcessors() []Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	processors := make([]Chainable[T], len(f.processors))
	copy(processors, f.processors)
	return processors
}

// Len returns the number of processors in the fallback chain.
func (f *Fallback[T]) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.processors)
}

// GetPrimary returns the first processor (for backward compatibility).
func (f *Fallback[T]) GetPrimary() Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.processors) > 0 {
		return f.processors[0]
	}
	return nil
}

// GetFallback returns the second processor (for backward compatibility).
// Returns nil if there's no second processor.
func (f *Fallback[T]) GetFallback() Chainable[T] {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.processors) > 1 {
		return f.processors[1]
	}
	return nil
}

// SetPrimary updates the first processor (for backward compatibility).
func (f *Fallback[T]) SetPrimary(processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.processors) > 0 {
		f.processors[0] = processor
	}
	return f
}

// SetFallback updates the second processor (for backward compatibility).
// If there's no second processor, adds one.
func (f *Fallback[T]) SetFallback(processor Chainable[T]) *Fallback[T] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.processors) > 1 {
		f.processors[1] = processor
	} else if len(f.processors) == 1 {
		f.processors = append(f.processors, processor)
	}
	return f
}
