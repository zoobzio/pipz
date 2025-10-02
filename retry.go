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

// Metric keys for Retry connector observability.
const (
	RetryAttemptsTotal  = metricz.Key("retry.attempts.total")
	RetrySuccessesTotal = metricz.Key("retry.successes.total")
	RetryFailuresTotal  = metricz.Key("retry.failures.total")
	RetryAttemptCurrent = metricz.Key("retry.attempt.current")
)

// Span names for Retry connector.
const (
	RetryProcessSpan = tracez.Key("retry.process")
	RetryAttemptSpan = tracez.Key("retry.attempt")
)

// Span tags for Retry connector.
const (
	RetryTagConnector    = tracez.Tag("retry.connector")
	RetryTagMaxAttempts  = tracez.Tag("retry.max_attempts")
	RetryTagAttempt      = tracez.Tag("retry.attempt")
	RetryTagAttemptsUsed = tracez.Tag("retry.attempts_used")
	RetryTagSuccess      = tracez.Tag("retry.success")
	RetryTagExhausted    = tracez.Tag("retry.exhausted")
	RetryTagError        = tracez.Tag("retry.error")
	RetryTagCanceled     = tracez.Tag("retry.canceled")

	// Hook event keys.
	RetryEventAttempt   = hookz.Key("retry.attempt")
	RetryEventSuccess   = hookz.Key("retry.success")
	RetryEventExhausted = hookz.Key("retry.exhausted")
)

// RetryEvent represents a retry event.
// This is emitted via hookz when retry attempts are made, succeed, or when
// all attempts are exhausted, providing visibility into retry behavior.
type RetryEvent struct {
	Name          Name          // Connector name
	ProcessorName Name          // Name of the processor being retried
	AttemptNumber int           // Current attempt number (1-based)
	MaxAttempts   int           // Maximum number of attempts allowed
	Success       bool          // Whether this attempt succeeded
	Error         error         // Error from this attempt (if failed)
	Duration      time.Duration // How long this attempt took
	TotalDuration time.Duration // Total time spent across all attempts (for exhausted/success)
	AttemptsUsed  int           // Total attempts made (for exhausted/success)
	Timestamp     time.Time     // When the event occurred
}

// Retry attempts the processor up to maxAttempts times.
// Retry provides simple retry logic for operations that may fail
// transiently. It immediately retries on failure without delay,
// making it suitable for quick operations or when failures are
// expected to clear immediately.
//
// Each retry uses the same input data. Context cancellation is
// checked between attempts to allow for early termination.
// If all attempts fail, the last error is returned with attempt
// count information for debugging.
//
// Use Retry for:
//   - Network calls with transient failures
//   - Database operations during brief contentions
//   - File operations with temporary locks
//   - Any operation with intermittent failures
//
// For operations needing delay between retries, use RetryWithBackoff.
// For trying different approaches, use Fallback instead.
//
// Example:
//
//	retry := pipz.NewRetry(
//	    databaseWriter,
//	    3,  // Try up to 3 times
//	)
//
// # Observability
//
// Retry provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - retry.attempts.total: Counter of individual retry attempts
//   - retry.successes.total: Counter of successful operations
//   - retry.failures.total: Counter of exhausted retry sequences
//   - retry.attempt.current: Gauge showing current attempt number
//
// Traces:
//   - retry.process: Parent span for entire retry operation
//   - retry.attempt: Child span for each individual attempt
//
// Events (via hooks):
//   - retry.attempt: Fired after each attempt (success or failure)
//   - retry.success: Fired when an attempt succeeds
//   - retry.exhausted: Fired when all attempts fail
//
// Example with hooks:
//
//	retry := pipz.NewRetry("db-retry", dbProcessor, 3)
//
//	// Monitor retry patterns
//	retry.OnAttempt(func(ctx context.Context, event RetryEvent) error {
//	    if !event.Success {
//	        log.Debug("Attempt %d/%d failed: %v",
//	            event.AttemptNumber, event.MaxAttempts, event.Error)
//	    }
//	    return nil
//	})
//
//	// Alert on exhaustion
//	retry.OnExhausted(func(ctx context.Context, event RetryEvent) error {
//	    alert.Error("All %d attempts failed after %v",
//	        event.MaxAttempts, event.TotalDuration)
//	    return nil
//	})
type Retry[T any] struct {
	processor   Chainable[T]
	name        Name
	maxAttempts int
	mu          sync.RWMutex

	// Observability
	metrics *metricz.Registry
	tracer  *tracez.Tracer
	hooks   *hookz.Hooks[RetryEvent]
}

// NewRetry creates a new Retry connector.
func NewRetry[T any](name Name, processor Chainable[T], maxAttempts int) *Retry[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Initialize observability components
	registry := metricz.New()
	tracer := tracez.New()

	// Register metrics
	registry.Counter(RetryAttemptsTotal)
	registry.Counter(RetrySuccessesTotal)
	registry.Counter(RetryFailuresTotal)
	registry.Gauge(RetryAttemptCurrent)

	return &Retry[T]{
		name:        name,
		processor:   processor,
		maxAttempts: maxAttempts,
		metrics:     registry,
		tracer:      tracer,
		hooks:       hookz.New[RetryEvent](),
	}
}

// Process implements the Chainable interface.
func (r *Retry[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, r.name, data)
	r.mu.RLock()
	processor := r.processor
	maxAttempts := r.maxAttempts
	r.mu.RUnlock()

	// Start parent span for entire retry operation
	ctx, span := r.tracer.StartSpan(ctx, RetryProcessSpan)
	defer span.Finish()
	span.SetTag(RetryTagMaxAttempts, fmt.Sprintf("%d", maxAttempts))
	span.SetTag(RetryTagConnector, string(r.name))

	var lastErr error
	var lastResult T
	totalStart := time.Now()

	for i := 0; i < maxAttempts; i++ {
		attemptNum := i + 1

		// Update current attempt gauge
		r.metrics.Gauge(RetryAttemptCurrent).Set(float64(attemptNum))

		// Start span for this attempt
		attemptCtx, attemptSpan := r.tracer.StartSpan(ctx, RetryAttemptSpan)
		attemptSpan.SetTag(RetryTagAttempt, fmt.Sprintf("%d", attemptNum))

		// Increment total attempts counter
		r.metrics.Counter(RetryAttemptsTotal).Inc()

		attemptStart := time.Now()
		result, err := processor.Process(attemptCtx, data)
		attemptDuration := time.Since(attemptStart)

		// Emit attempt event
		if r.hooks.ListenerCount(RetryEventAttempt) > 0 {
			_ = r.hooks.Emit(ctx, RetryEventAttempt, RetryEvent{ //nolint:errcheck
				Name:          r.name,
				ProcessorName: processor.Name(),
				AttemptNumber: attemptNum,
				MaxAttempts:   maxAttempts,
				Success:       err == nil,
				Error:         err,
				Duration:      attemptDuration,
				Timestamp:     time.Now(),
			})
		}

		if err == nil {
			// Success!
			attemptSpan.SetTag(RetryTagSuccess, "true")
			attemptSpan.Finish()

			span.SetTag(RetryTagSuccess, "true")
			span.SetTag(RetryTagAttemptsUsed, fmt.Sprintf("%d", attemptNum))

			// Update metrics
			r.metrics.Counter(RetrySuccessesTotal).Inc()
			r.metrics.Gauge(RetryAttemptCurrent).Set(0) // Reset gauge

			// Emit success event
			totalDuration := time.Since(totalStart)
			if r.hooks.ListenerCount(RetryEventSuccess) > 0 {
				_ = r.hooks.Emit(ctx, RetryEventSuccess, RetryEvent{ //nolint:errcheck
					Name:          r.name,
					ProcessorName: processor.Name(),
					AttemptNumber: attemptNum,
					MaxAttempts:   maxAttempts,
					Success:       true,
					TotalDuration: totalDuration,
					AttemptsUsed:  attemptNum,
					Timestamp:     time.Now(),
				})
			}

			return result, nil
		}

		// Attempt failed
		attemptSpan.SetTag(RetryTagSuccess, "false")
		attemptSpan.SetTag(RetryTagError, err.Error())
		attemptSpan.Finish()

		lastErr = err
		lastResult = result

		// Check if context is canceled between attempts
		if ctx.Err() != nil {
			// Context canceled/timed out - return error
			span.SetTag(RetryTagSuccess, "false")
			span.SetTag(RetryTagCanceled, "true")
			r.metrics.Gauge(RetryAttemptCurrent).Set(0) // Reset gauge

			return data, &Error[T]{
				Err:       ctx.Err(),
				InputData: data,
				Path:      []Name{r.name},
				Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
				Canceled:  errors.Is(ctx.Err(), context.Canceled),
				Timestamp: time.Now(),
			}
		}
	}

	// All attempts failed
	span.SetTag(RetryTagSuccess, "false")
	span.SetTag(RetryTagExhausted, "true")
	span.SetTag(RetryTagAttemptsUsed, fmt.Sprintf("%d", maxAttempts))

	// Update metrics
	r.metrics.Counter(RetryFailuresTotal).Inc()
	r.metrics.Gauge(RetryAttemptCurrent).Set(0) // Reset gauge

	// Emit exhausted event
	totalDuration := time.Since(totalStart)
	if r.hooks.ListenerCount(RetryEventExhausted) > 0 {
		_ = r.hooks.Emit(ctx, RetryEventExhausted, RetryEvent{ //nolint:errcheck
			Name:          r.name,
			ProcessorName: processor.Name(),
			MaxAttempts:   maxAttempts,
			Success:       false,
			Error:         lastErr,
			TotalDuration: totalDuration,
			AttemptsUsed:  maxAttempts,
			Timestamp:     time.Now(),
		})
	}

	// Return the last error
	if lastErr != nil {
		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			// Prepend this retry's name to the path
			pipeErr.Path = append([]Name{r.name}, pipeErr.Path...)
			return lastResult, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return lastResult, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       lastErr,
			Path:      []Name{r.name},
		}
	}
	return lastResult, nil
}

// SetMaxAttempts updates the maximum number of retry attempts.
func (r *Retry[T]) SetMaxAttempts(n int) *Retry[T] {
	if n < 1 {
		n = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxAttempts = n
	return r
}

// GetMaxAttempts returns the current maximum attempts setting.
func (r *Retry[T]) GetMaxAttempts() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxAttempts
}

// Name returns the name of this connector.
func (r *Retry[T]) Name() Name {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.name
}

// Metrics returns the metrics registry for this connector.
func (r *Retry[T]) Metrics() *metricz.Registry {
	return r.metrics
}

// Tracer returns the tracer for this connector.
func (r *Retry[T]) Tracer() *tracez.Tracer {
	return r.tracer
}

// Close gracefully shuts down observability components.
func (r *Retry[T]) Close() error {
	if r.tracer != nil {
		r.tracer.Close()
	}
	r.hooks.Close()
	return nil
}

// OnAttempt registers a handler for when a retry attempt is made.
// The handler is called asynchronously after each attempt completes,
// whether it succeeds or fails.
func (r *Retry[T]) OnAttempt(handler func(context.Context, RetryEvent) error) error {
	_, err := r.hooks.Hook(RetryEventAttempt, handler)
	return err
}

// OnSuccess registers a handler for when the operation succeeds.
// The handler is called asynchronously when any attempt succeeds,
// including aggregate information about all attempts made.
func (r *Retry[T]) OnSuccess(handler func(context.Context, RetryEvent) error) error {
	_, err := r.hooks.Hook(RetryEventSuccess, handler)
	return err
}

// OnExhausted registers a handler for when all retry attempts are exhausted.
// The handler is called asynchronously after all attempts have failed,
// providing visibility into persistent failures.
func (r *Retry[T]) OnExhausted(handler func(context.Context, RetryEvent) error) error {
	_, err := r.hooks.Hook(RetryEventExhausted, handler)
	return err
}
