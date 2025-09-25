package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the Timeout connector.
const (
	// Metrics.
	TimeoutProcessedTotal = metricz.Key("timeout.processed.total")
	TimeoutSuccessesTotal = metricz.Key("timeout.successes.total")
	TimeoutTimeoutsTotal  = metricz.Key("timeout.timeouts.total")
	TimeoutCancellations  = metricz.Key("timeout.cancellations.total")
	TimeoutDurationMs     = metricz.Key("timeout.duration.ms")

	// Spans.
	TimeoutProcessSpan = tracez.Key("timeout.process")

	// Tags.
	TimeoutTagDuration = tracez.Tag("timeout.duration")
	TimeoutTagSuccess  = tracez.Tag("timeout.success")
	TimeoutTagError    = tracez.Tag("timeout.error")
	TimeoutTagTimedOut = tracez.Tag("timeout.timed_out")
	TimeoutTagCanceled = tracez.Tag("timeout.canceled")
	TimeoutTagElapsed  = tracez.Tag("timeout.elapsed")

	// Hook event keys.
	TimeoutEventTimeout     = hookz.Key("timeout.timeout")
	TimeoutEventNearTimeout = hookz.Key("timeout.near_timeout")
)

// TimeoutEvent represents a timeout event.
// This is emitted via hookz when operations timeout or come close,
// allowing external systems to monitor and react to timeout patterns.
type TimeoutEvent struct {
	Name        Name          // Connector name
	Duration    time.Duration // Configured timeout duration
	Elapsed     time.Duration // How long the operation took
	TimedOut    bool          // Whether it actually timed out
	NearTimeout bool          // Whether it was close to timeout (>80% of duration)
	PercentUsed float64       // Percentage of timeout duration used
	Error       error         // Error if operation failed
	Timestamp   time.Time     // When the event occurred
}

// Timeout enforces a timeout on the processor's execution.
// Timeout wraps any processor with a hard time limit, ensuring operations
// complete within acceptable bounds. If the timeout expires, the operation
// is canceled via context and a timeout error is returned.
//
// This connector is critical for:
//   - Preventing hung operations
//   - Meeting SLA requirements
//   - Protecting against slow external services
//   - Ensuring predictable system behavior
//   - Resource management in concurrent systems
//
// The wrapped operation should respect context cancellation for
// immediate termination. Operations that ignore context may continue
// running in the background even after timeout.
//
// Timeout is often combined with Retry for robust error handling:
//
//	pipz.NewRetry(pipz.NewTimeout(operation, 5*time.Second), 3)
//
// Example:
//
//	timeout := pipz.NewTimeout(
//	    userServiceCall,
//	    2*time.Second,  // Must complete within 2 seconds
//	)
//
// # Observability
//
// Timeout provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - timeout.processed.total: Counter of timeout operations
//   - timeout.successes.total: Counter of operations that completed in time
//   - timeout.timeouts.total: Counter of operations that exceeded timeout
//   - timeout.cancellations.total: Counter of operations canceled by upstream
//   - timeout.duration.ms: Gauge of actual operation duration
//
// Traces:
//   - timeout.process: Span for timeout-wrapped operation
//
// Events (via hooks):
//   - timeout.timeout: Fired when an operation times out
//   - timeout.near_timeout: Fired when operation uses >80% of timeout duration
//
// Example with hooks:
//
//	timeout := pipz.NewTimeout("api-timeout", apiCall, 5*time.Second)
//
//	// Alert on timeouts
//	timeout.OnTimeout(func(ctx context.Context, event TimeoutEvent) error {
//	    alert.Error("API call timed out after %v", event.Elapsed)
//	    metrics.Inc("api.timeouts")
//	    return nil
//	})
//
//	// Warn on slow operations
//	timeout.OnNearTimeout(func(ctx context.Context, event TimeoutEvent) error {
//	    log.Warn("API call used %.0f%% of timeout (%v/%v)",
//	        event.PercentUsed, event.Elapsed, event.Duration)
//	    if event.PercentUsed > 90 {
//	        alert.Warn("API call critically slow")
//	    }
//	    return nil
//	})
type Timeout[T any] struct {
	processor Chainable[T]
	clock     clockz.Clock
	name      Name
	duration  time.Duration
	mu        sync.RWMutex
	metrics   *metricz.Registry
	tracer    *tracez.Tracer
	hooks     *hookz.Hooks[TimeoutEvent]
}

// NewTimeout creates a new Timeout connector.
func NewTimeout[T any](name Name, processor Chainable[T], duration time.Duration) *Timeout[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(TimeoutProcessedTotal)
	metrics.Counter(TimeoutSuccessesTotal)
	metrics.Counter(TimeoutTimeoutsTotal)
	metrics.Counter(TimeoutCancellations)
	metrics.Gauge(TimeoutDurationMs)

	return &Timeout[T]{
		name:      name,
		processor: processor,
		duration:  duration,
		metrics:   metrics,
		tracer:    tracez.New(),
		hooks:     hookz.New[TimeoutEvent](),
	}
}

// Process implements the Chainable interface.
func (t *Timeout[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, t.name, data)

	t.mu.RLock()
	processor := t.processor
	duration := t.duration
	clock := t.getClock()
	t.mu.RUnlock()

	// Start tracking metrics
	t.metrics.Counter(TimeoutProcessedTotal).Inc()
	start := time.Now()

	// Start main span
	ctx, span := t.tracer.StartSpan(ctx, TimeoutProcessSpan)
	span.SetTag(TimeoutTagDuration, duration.String())
	defer func() {
		// Record elapsed time
		elapsed := time.Since(start)
		t.metrics.Gauge(TimeoutDurationMs).Set(float64(elapsed.Milliseconds()))
		span.SetTag(TimeoutTagElapsed, elapsed.String())
		span.Finish()
	}()

	ctx, cancel := clock.WithTimeout(ctx, duration)
	defer cancel()

	// Channel to receive the result from the goroutine
	type processResult struct {
		result T
		err    error
	}
	resultCh := make(chan processResult, 1)

	go func() {
		defer func() {
			// Ensure goroutine cleanup on panic
			if r := recover(); r != nil {
				// Convert panic to error and send it
				var zero T
				panicErr := &Error[T]{
					Path:      []Name{t.name},
					InputData: data,
					Err:       &panicError{processorName: t.name, sanitized: sanitizePanicMessage(r)},
					Timestamp: time.Now(),
					Duration:  0,
					Timeout:   false,
					Canceled:  false,
				}
				select {
				case resultCh <- processResult{result: zero, err: panicErr}:
				case <-ctx.Done():
					// Context was canceled, don't block
				}
			}
		}()
		result, err := processor.Process(ctx, data)
		// Use non-blocking send to avoid goroutine leak if main function has already returned
		select {
		case resultCh <- processResult{result: result, err: err}:
		case <-ctx.Done():
			// Context was canceled, don't block trying to send result
		}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			// Operation failed (but didn't timeout)
			span.SetTag(TimeoutTagSuccess, "false")
			span.SetTag(TimeoutTagError, res.err.Error())

			var pipeErr *Error[T]
			if errors.As(res.err, &pipeErr) {
				// Check if the error was due to context cancellation from downstream
				if pipeErr.Canceled {
					t.metrics.Counter(TimeoutCancellations).Inc()
					span.SetTag(TimeoutTagCanceled, "true")
				}
				// Prepend this timeout's name to the path
				pipeErr.Path = append([]Name{t.name}, pipeErr.Path...)
				return res.result, pipeErr
			}
			// Handle non-pipeline errors by wrapping them
			return res.result, &Error[T]{
				Timestamp: time.Now(),
				InputData: data,
				Err:       res.err,
				Path:      []Name{t.name},
			}
		}
		// Success!
		span.SetTag(TimeoutTagSuccess, "true")
		t.metrics.Counter(TimeoutSuccessesTotal).Inc()

		// Check if operation was close to timeout (>80% of duration)
		elapsed := time.Since(start)
		percentUsed := float64(elapsed) / float64(duration) * 100
		if percentUsed > 80 {
			_ = t.hooks.Emit(ctx, TimeoutEventNearTimeout, TimeoutEvent{ //nolint:errcheck
				Name:        t.name,
				Duration:    duration,
				Elapsed:     elapsed,
				NearTimeout: true,
				PercentUsed: percentUsed,
				Timestamp:   clock.Now(),
			})
		}

		return res.result, nil
	case <-ctx.Done():
		// Timeout or cancellation occurred
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// Actual timeout
			span.SetTag(TimeoutTagSuccess, "false")
			span.SetTag(TimeoutTagTimedOut, "true")
			t.metrics.Counter(TimeoutTimeoutsTotal).Inc()

			// Emit timeout event
			elapsed := time.Since(start)
			_ = t.hooks.Emit(ctx, TimeoutEventTimeout, TimeoutEvent{ //nolint:errcheck
				Name:        t.name,
				Duration:    duration,
				Elapsed:     elapsed,
				TimedOut:    true,
				PercentUsed: 100.0, // Exceeded timeout
				Error:       ctx.Err(),
				Timestamp:   clock.Now(),
			})
		} else {
			// Context was canceled
			span.SetTag(TimeoutTagSuccess, "false")
			span.SetTag(TimeoutTagCanceled, "true")
			t.metrics.Counter(TimeoutCancellations).Inc()
		}

		return data, &Error[T]{
			Err:       ctx.Err(),
			InputData: data,
			Path:      []Name{t.name},
			Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
			Canceled:  errors.Is(ctx.Err(), context.Canceled),
			Timestamp: time.Now(),
		}
	}
}

// SetDuration updates the timeout duration.
func (t *Timeout[T]) SetDuration(d time.Duration) *Timeout[T] {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.duration = d
	return t
}

// GetDuration returns the current timeout duration.
func (t *Timeout[T]) GetDuration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.duration
}

// Name returns the name of this connector.
func (t *Timeout[T]) Name() Name {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.name
}

// WithClock sets a custom clock for testing.
func (t *Timeout[T]) WithClock(clock clockz.Clock) *Timeout[T] {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clock = clock
	return t
}

// getClock returns the clock to use.
func (t *Timeout[T]) getClock() clockz.Clock {
	if t.clock == nil {
		return clockz.RealClock
	}
	return t.clock
}

// Metrics returns the metrics registry for this connector.
func (t *Timeout[T]) Metrics() *metricz.Registry {
	return t.metrics
}

// Tracer returns the tracer for this connector.
func (t *Timeout[T]) Tracer() *tracez.Tracer {
	return t.tracer
}

// Close gracefully shuts down observability components.
func (t *Timeout[T]) Close() error {
	if t.tracer != nil {
		t.tracer.Close()
	}
	t.hooks.Close()
	return nil
}

// OnTimeout registers a handler for when operations timeout.
// The handler is called asynchronously when an operation exceeds the timeout duration.
func (t *Timeout[T]) OnTimeout(handler func(context.Context, TimeoutEvent) error) error {
	_, err := t.hooks.Hook(TimeoutEventTimeout, handler)
	return err
}

// OnNearTimeout registers a handler for when operations are close to timing out.
// The handler is called asynchronously when an operation completes but took >80% of the timeout duration.
// This is useful for identifying operations that are at risk of timing out.
func (t *Timeout[T]) OnNearTimeout(handler func(context.Context, TimeoutEvent) error) error {
	_, err := t.hooks.Hook(TimeoutEventNearTimeout, handler)
	return err
}
