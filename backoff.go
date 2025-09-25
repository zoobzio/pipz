package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the Backoff connector.
const (
	// Metrics.
	BackoffAttemptsTotal  = metricz.Key("backoff.attempts.total")
	BackoffSuccessesTotal = metricz.Key("backoff.successes.total")
	BackoffFailuresTotal  = metricz.Key("backoff.failures.total")
	BackoffAttemptCurrent = metricz.Key("backoff.attempt.current")
	BackoffDelayTotal     = metricz.Key("backoff.delay.total.ms")

	// Spans.
	BackoffProcessSpan = tracez.Key("backoff.process")
	BackoffAttemptSpan = tracez.Key("backoff.attempt")

	// Tags.
	BackoffTagMaxAttempts  = tracez.Tag("backoff.max_attempts")
	BackoffTagBaseDelay    = tracez.Tag("backoff.base_delay")
	BackoffTagAttemptNum   = tracez.Tag("backoff.attempt_num")
	BackoffTagSuccess      = tracez.Tag("backoff.success")
	BackoffTagError        = tracez.Tag("backoff.error")
	BackoffTagExhausted    = tracez.Tag("backoff.exhausted")
	BackoffTagCanceled     = tracez.Tag("backoff.canceled")
	BackoffTagDelay        = tracez.Tag("backoff.delay")
	BackoffTagAttemptsUsed = tracez.Tag("backoff.attempts_used")

	// Hook event keys.
	BackoffEventAttempt   = hookz.Key("backoff.attempt")
	BackoffEventExhausted = hookz.Key("backoff.exhausted")
	BackoffEventSuccess   = hookz.Key("backoff.success")
)

// BackoffEvent represents a backoff retry event.
// This is emitted via hookz for each retry attempt and final outcomes,
// allowing external systems to monitor retry patterns and failures.
type BackoffEvent struct {
	Name        Name          // Connector name
	AttemptNum  int           // Current attempt number (1-based)
	MaxAttempts int           // Maximum attempts configured
	Delay       time.Duration // Delay before this attempt (0 for first attempt)
	TotalDelay  time.Duration // Total delay accumulated so far
	Success     bool          // Whether attempt succeeded
	Exhausted   bool          // Whether all attempts exhausted
	Error       error         // Error from this attempt (if failed)
	Timestamp   time.Time     // When the event occurred
}

// Backoff attempts the processor with exponential backoff between attempts.
// Backoff adds intelligent spacing between retry attempts, starting with
// baseDelay and doubling after each failure. This prevents overwhelming failed
// services and allows time for transient issues to resolve.
//
// The exponential backoff pattern (delay, 2*delay, 4*delay, ...) is widely
// used for its effectiveness in handling various failure scenarios without
// overwhelming systems. The operation can be canceled via context during waits.
//
// Ideal for:
//   - API calls to rate-limited services
//   - Database operations during high load
//   - Distributed system interactions
//   - Any operation where immediate retry is counterproductive
//
// The total time spent can be significant with multiple retries.
// For example, with baseDelay=1s and maxAttempts=5:
//
//	Delays: 1s, 2s, 4s, 8s (total wait: 15s plus processing time)
//
// # Observability
//
// Backoff provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - backoff.attempts.total: Counter of total retry attempts
//   - backoff.successes.total: Counter of successful completions
//   - backoff.failures.total: Counter of exhausted retries
//   - backoff.attempt.current: Gauge showing current attempt number
//   - backoff.delay.total.ms: Gauge of total delay accumulated
//
// Traces:
//   - backoff.process: Parent span for entire backoff operation
//   - backoff.attempt: Child span for each retry attempt
//
// Events (via hooks):
//   - backoff.attempt: Fired after each attempt with delay and result info
//   - backoff.success: Fired when operation succeeds with total stats
//   - backoff.exhausted: Fired when all attempts fail with failure details
//
// Example with hooks:
//
//	backoff := pipz.NewBackoff(
//	    "api-backoff",
//	    apiProcessor,
//	    5,                    // Max 5 attempts
//	    100*time.Millisecond, // Start with 100ms delay
//	)
//
//	// Monitor retry patterns
//	backoff.OnAttempt(func(ctx context.Context, event BackoffEvent) error {
//	    log.Printf("Attempt %d/%d after %v delay",
//	        event.AttemptNum, event.MaxAttempts, event.Delay)
//	    return nil
//	})
//
//	// Alert on exhaustion
//	backoff.OnExhausted(func(ctx context.Context, event BackoffEvent) error {
//	    alert.Send("API calls failing after %d attempts", event.MaxAttempts)
//	    return nil
//	})
type Backoff[T any] struct {
	processor   Chainable[T]
	clock       clockz.Clock
	name        Name
	baseDelay   time.Duration
	mu          sync.RWMutex
	maxAttempts int
	metrics     *metricz.Registry
	tracer      *tracez.Tracer
	hooks       *hookz.Hooks[BackoffEvent]
}

// NewBackoff creates a new Backoff connector.
func NewBackoff[T any](name Name, processor Chainable[T], maxAttempts int, baseDelay time.Duration) *Backoff[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(BackoffAttemptsTotal)
	metrics.Counter(BackoffSuccessesTotal)
	metrics.Counter(BackoffFailuresTotal)
	metrics.Counter(BackoffDelayTotal) // Total milliseconds spent waiting
	metrics.Gauge(BackoffAttemptCurrent)

	return &Backoff[T]{
		name:        name,
		processor:   processor,
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		metrics:     metrics,
		tracer:      tracez.New(),
		hooks:       hookz.New[BackoffEvent](),
	}
}

// Process implements the Chainable interface.
func (b *Backoff[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, b.name, data)
	b.mu.RLock()
	processor := b.processor
	maxAttempts := b.maxAttempts
	baseDelay := b.baseDelay
	clock := b.getClock()
	b.mu.RUnlock()

	// Start main span
	ctx, span := b.tracer.StartSpan(ctx, BackoffProcessSpan)
	span.SetTag(BackoffTagMaxAttempts, fmt.Sprintf("%d", maxAttempts))
	span.SetTag(BackoffTagBaseDelay, baseDelay.String())
	defer span.Finish()

	var lastErr error
	var lastResult T
	delay := baseDelay
	totalDelay := time.Duration(0)

	for i := 0; i < maxAttempts; i++ {
		attemptNum := i + 1

		// Update current attempt gauge
		b.metrics.Gauge(BackoffAttemptCurrent).Set(float64(attemptNum))

		// Start attempt span
		attemptCtx, attemptSpan := b.tracer.StartSpan(ctx, BackoffAttemptSpan)
		attemptSpan.SetTag(BackoffTagAttemptNum, fmt.Sprintf("%d", attemptNum))
		if i > 0 {
			attemptSpan.SetTag(BackoffTagDelay, delay.String())
		}

		// Increment attempts counter
		b.metrics.Counter(BackoffAttemptsTotal).Inc()

		// Emit attempt event
		if i > 0 {
			// Don't emit for first attempt since there's no delay
			_ = b.hooks.Emit(ctx, BackoffEventAttempt, BackoffEvent{ //nolint:errcheck
				Name:        b.name,
				AttemptNum:  attemptNum,
				MaxAttempts: maxAttempts,
				Delay:       delay,
				TotalDelay:  totalDelay,
				Timestamp:   clock.Now(),
			})
		}

		result, err := processor.Process(attemptCtx, data)
		if err == nil {
			// Success!
			attemptSpan.SetTag(BackoffTagSuccess, "true")
			attemptSpan.Finish()

			span.SetTag(BackoffTagSuccess, "true")
			span.SetTag(BackoffTagAttemptsUsed, fmt.Sprintf("%d", attemptNum))

			// Update metrics
			b.metrics.Counter(BackoffSuccessesTotal).Inc()
			b.metrics.Counter(BackoffDelayTotal).Add(float64(totalDelay.Milliseconds()))
			b.metrics.Gauge(BackoffAttemptCurrent).Set(0) // Reset gauge

			// Emit success event
			_ = b.hooks.Emit(ctx, BackoffEventSuccess, BackoffEvent{ //nolint:errcheck
				Name:        b.name,
				AttemptNum:  attemptNum,
				MaxAttempts: maxAttempts,
				TotalDelay:  totalDelay,
				Success:     true,
				Timestamp:   clock.Now(),
			})

			return result, nil
		}

		// Attempt failed
		attemptSpan.SetTag(BackoffTagSuccess, "false")
		attemptSpan.SetTag(BackoffTagError, err.Error())
		attemptSpan.Finish()

		lastErr = err
		lastResult = result

		// Don't sleep after the last attempt
		if i < maxAttempts-1 {
			select {
			case <-clock.After(delay):
				totalDelay += delay
				delay *= 2 // Exponential backoff
			case <-ctx.Done():
				// Context canceled/timed out
				span.SetTag(BackoffTagSuccess, "false")
				span.SetTag(BackoffTagCanceled, "true")
				b.metrics.Counter(BackoffDelayTotal).Add(float64(totalDelay.Milliseconds()))
				b.metrics.Gauge(BackoffAttemptCurrent).Set(0) // Reset gauge

				return data, &Error[T]{
					Err:       ctx.Err(),
					InputData: data,
					Path:      []Name{b.name},
					Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
					Canceled:  errors.Is(ctx.Err(), context.Canceled),
					Timestamp: time.Now(),
				}
			}
		}
	}

	// All attempts failed
	span.SetTag(BackoffTagSuccess, "false")
	span.SetTag(BackoffTagExhausted, "true")
	span.SetTag(BackoffTagAttemptsUsed, fmt.Sprintf("%d", maxAttempts))

	// Update metrics
	b.metrics.Counter(BackoffFailuresTotal).Inc()
	b.metrics.Counter(BackoffDelayTotal).Add(float64(totalDelay.Milliseconds()))
	b.metrics.Gauge(BackoffAttemptCurrent).Set(0) // Reset gauge

	// Emit exhausted event
	_ = b.hooks.Emit(ctx, BackoffEventExhausted, BackoffEvent{ //nolint:errcheck
		Name:        b.name,
		AttemptNum:  maxAttempts,
		MaxAttempts: maxAttempts,
		TotalDelay:  totalDelay,
		Exhausted:   true,
		Error:       lastErr,
		Timestamp:   clock.Now(),
	})

	// Return the last error
	if lastErr != nil {
		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			// Prepend this backoff's name to the path
			pipeErr.Path = append([]Name{b.name}, pipeErr.Path...)
			return data, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return data, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       lastErr,
			Path:      []Name{b.name},
		}
	}
	return lastResult, nil
}

// SetMaxAttempts updates the maximum number of retry attempts.
func (b *Backoff[T]) SetMaxAttempts(n int) *Backoff[T] {
	if n < 1 {
		n = 1
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxAttempts = n
	return b
}

// SetBaseDelay updates the base delay duration.
func (b *Backoff[T]) SetBaseDelay(d time.Duration) *Backoff[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.baseDelay = d
	return b
}

// GetMaxAttempts returns the current maximum attempts setting.
func (b *Backoff[T]) GetMaxAttempts() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.maxAttempts
}

// GetBaseDelay returns the current base delay setting.
func (b *Backoff[T]) GetBaseDelay() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.baseDelay
}

// Name returns the name of this connector.
func (b *Backoff[T]) Name() Name {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.name
}

// Metrics returns the metrics registry for this connector.
func (b *Backoff[T]) Metrics() *metricz.Registry {
	return b.metrics
}

// Tracer returns the tracer for this connector.
func (b *Backoff[T]) Tracer() *tracez.Tracer {
	return b.tracer
}

// Close gracefully shuts down observability components.
func (b *Backoff[T]) Close() error {
	if b.tracer != nil {
		b.tracer.Close()
	}
	b.hooks.Close()
	return nil
}

// OnAttempt registers a handler for retry attempt events.
// The handler is called asynchronously before each retry attempt (not the first attempt).
func (b *Backoff[T]) OnAttempt(handler func(context.Context, BackoffEvent) error) error {
	_, err := b.hooks.Hook(BackoffEventAttempt, handler)
	return err
}

// OnExhausted registers a handler for when all attempts are exhausted.
// The handler is called asynchronously when all retry attempts have failed.
func (b *Backoff[T]) OnExhausted(handler func(context.Context, BackoffEvent) error) error {
	_, err := b.hooks.Hook(BackoffEventExhausted, handler)
	return err
}

// OnSuccess registers a handler for successful completion.
// The handler is called asynchronously when an attempt succeeds.
func (b *Backoff[T]) OnSuccess(handler func(context.Context, BackoffEvent) error) error {
	_, err := b.hooks.Hook(BackoffEventSuccess, handler)
	return err
}

// WithClock sets a custom clock for testing.
func (b *Backoff[T]) WithClock(clock clockz.Clock) *Backoff[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clock = clock
	return b
}

// getClock returns the clock to use.
func (b *Backoff[T]) getClock() clockz.Clock {
	if b.clock == nil {
		return clockz.RealClock
	}
	return b.clock
}
