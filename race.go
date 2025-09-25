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

// Observability constants for the Race connector.
const (
	// Metrics.
	RaceProcessedTotal = metricz.Key("race.processed.total")
	RaceSuccessesTotal = metricz.Key("race.successes.total")
	RaceWinsTotal      = metricz.Key("race.wins.total")
	RaceAllFailedTotal = metricz.Key("race.all_failed.total")
	RaceProcessorCount = metricz.Key("race.processor.count")
	RaceDurationMs     = metricz.Key("race.duration.ms")

	// Spans.
	RaceProcessSpan   = tracez.Key("race.process")
	RaceProcessorSpan = tracez.Key("race.processor")

	// Tags.
	RaceTagProcessorCount = tracez.Tag("race.processor_count")
	RaceTagProcessorName  = tracez.Tag("race.processor_name")
	RaceTagWinner         = tracez.Tag("race.winner")
	RaceTagSuccess        = tracez.Tag("race.success")
	RaceTagError          = tracez.Tag("race.error")

	// Hook event keys.
	RaceEventWinner    = hookz.Key("race.winner")
	RaceEventAllFailed = hookz.Key("race.all_failed")
)

// RaceEvent represents a race outcome event.
// This is emitted via hookz when a processor wins the race or all fail,
// allowing external systems to monitor performance patterns and failures.
type RaceEvent struct {
	Name           Name          // Connector name
	Winner         Name          // Name of winning processor (if any)
	ProcessorCount int           // Total processors in the race
	Duration       time.Duration // How long until winner (or all failed)
	AllFailed      bool          // Whether all processors failed
	Error          error         // Error if all failed
	Timestamp      time.Time     // When the event occurred
}

// Race runs all processors in parallel and returns the result of the first
// to complete successfully. Race implements competitive processing where speed
// matters more than which specific processor succeeds. The first successful
// result wins and cancels all other processors.
//
// Context handling: Race uses context.WithCancel(ctx) to create a derived context
// that preserves all parent context values (including trace IDs) while allowing
// cancellation of losing processors when a winner is found.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// This pattern excels when you have multiple ways to get the same result
// and want the fastest one:
//   - Querying multiple replicas or regions
//   - Trying different algorithms with varying performance
//   - Fetching from multiple caches
//   - Calling primary and backup services simultaneously
//   - Any scenario where latency matters more than specific source
//
// Key behaviors:
//   - First success wins and cancels others
//   - All failures returns the last error
//   - Each processor gets an isolated copy via Clone()
//   - Useful for reducing p99 latencies
//   - Can increase load (all processors run)
//
// Example:
//
//	// UserQuery must implement Cloner[UserQuery]
//	race := pipz.NewRace(
//	    fetchFromLocalCache,
//	    fetchFromRegionalCache,
//	    fetchFromDatabase,
//	)
//
// # Observability
//
// Race provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - race.processed.total: Counter of race operations
//   - race.successes.total: Counter of races with winners
//   - race.wins.total: Counter of winning processors
//   - race.all_failed.total: Counter of races where all processors failed
//   - race.processor.count: Gauge of processors in race
//   - race.duration.ms: Gauge of time to find winner
//
// Traces:
//   - race.process: Parent span for race operation
//   - race.processor: Child span for each competing processor
//
// Events (via hooks):
//   - race.winner: Fired when a processor wins the race
//   - race.all_failed: Fired when all processors fail
//
// Example with hooks:
//
//	race := pipz.NewRace("cache-race",
//	    localCache,
//	    regionalCache,
//	    globalCache,
//	)
//
//	// Monitor performance patterns
//	race.OnWinner(func(ctx context.Context, event RaceEvent) error {
//	    metrics.Inc("cache.winner", event.Winner)
//	    log.Debug("%s won in %v", event.Winner, event.Duration)
//	    return nil
//	})
//
//	// Alert on total failures
//	race.OnAllFailed(func(ctx context.Context, event RaceEvent) error {
//	    alert.Error("All %d cache layers failed", event.ProcessorCount)
//	    return nil
//	})
type Race[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[RaceEvent]
}

// NewRace creates a new Race connector.
func NewRace[T Cloner[T]](name Name, processors ...Chainable[T]) *Race[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(RaceProcessedTotal)
	metrics.Counter(RaceSuccessesTotal)
	metrics.Counter(RaceWinsTotal)
	metrics.Counter(RaceAllFailedTotal)
	metrics.Gauge(RaceProcessorCount)
	metrics.Gauge(RaceDurationMs)

	return &Race[T]{
		name:       name,
		processors: processors,
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[RaceEvent](),
	}
}

// Process implements the Chainable interface.
func (r *Race[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, r.name, input)

	r.mu.RLock()
	processors := make([]Chainable[T], len(r.processors))
	copy(processors, r.processors)
	r.mu.RUnlock()

	// Track metrics
	r.metrics.Counter(RaceProcessedTotal).Inc()
	r.metrics.Gauge(RaceProcessorCount).Set(float64(len(processors)))
	start := time.Now()

	// Start main span
	ctx, span := r.tracer.StartSpan(ctx, RaceProcessSpan)
	span.SetTag(RaceTagProcessorCount, fmt.Sprintf("%d", len(processors)))
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		r.metrics.Gauge(RaceDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(RaceTagSuccess, "true")
			r.metrics.Counter(RaceSuccessesTotal).Inc()
		} else {
			span.SetTag(RaceTagSuccess, "false")
			if err != nil {
				span.SetTag(RaceTagError, err.Error())
			}
		}
		span.Finish()
	}()

	if len(processors) == 0 {
		var zero T
		return zero, &Error[T]{
			Path:      []Name{r.name},
			Err:       fmt.Errorf("no processors provided to Race"),
			InputData: input,
			Timestamp: time.Now(),
			Duration:  0,
		}
	}

	// Create channels for results and errors
	type raceResult struct {
		data T
		err  error
		idx  int
	}

	resultCh := make(chan raceResult, len(processors))
	// Create a cancellable context to stop other processors when one wins
	// This derives from the original context, preserving trace data
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch all processors
	for i, processor := range processors {
		go func(idx int, p Chainable[T]) {
			// Start span for this processor
			procCtx, procSpan := r.tracer.StartSpan(raceCtx, RaceProcessorSpan)
			procSpan.SetTag(RaceTagProcessorName, p.Name())
			defer procSpan.Finish()

			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Use processor context which includes span
			data, err := p.Process(procCtx, inputCopy)
			select {
			case resultCh <- raceResult{data: data, err: err, idx: idx}:
			case <-raceCtx.Done():
			}
		}(i, processor)
	}

	// Collect results
	var lastErr error
	for i := 0; i < len(processors); i++ {
		select {
		case res := <-resultCh:
			if res.err == nil {
				// First success wins
				cancel() // Cancel other goroutines
				winnerName := processors[res.idx].Name()
				span.SetTag(RaceTagWinner, winnerName)
				r.metrics.Counter(RaceWinsTotal).Inc()

				// Emit winner event
				_ = r.hooks.Emit(ctx, RaceEventWinner, RaceEvent{ //nolint:errcheck
					Name:           r.name,
					Winner:         winnerName,
					ProcessorCount: len(processors),
					Duration:       time.Since(start),
					Timestamp:      time.Now(),
				})

				return res.data, nil
			}
			lastErr = res.err
		case <-ctx.Done():
			// Context done means we're complete - return current input
			return input, nil
		}
	}

	// All failed - return the last error
	if lastErr != nil {
		r.metrics.Counter(RaceAllFailedTotal).Inc()

		// Emit all failed event
		_ = r.hooks.Emit(ctx, RaceEventAllFailed, RaceEvent{ //nolint:errcheck
			Name:           r.name,
			ProcessorCount: len(processors),
			Duration:       time.Since(start),
			AllFailed:      true,
			Error:          lastErr,
			Timestamp:      time.Now(),
		})

		var pipeErr *Error[T]
		if errors.As(lastErr, &pipeErr) {
			// Prepend this race's name to the path
			pipeErr.Path = append([]Name{r.name}, pipeErr.Path...)
			return input, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return input, &Error[T]{
			Timestamp: time.Now(),
			InputData: input,
			Err:       lastErr,
			Path:      []Name{r.name},
		}
	}
	return input, nil
}

// Add appends a processor to the race execution list.
func (r *Race[T]) Add(processor Chainable[T]) *Race[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors = append(r.processors, processor)
	return r
}

// Remove removes the processor at the specified index.
func (r *Race[T]) Remove(index int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if index < 0 || index >= len(r.processors) {
		return ErrIndexOutOfBounds
	}

	r.processors = append(r.processors[:index], r.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (r *Race[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.processors)
}

// Clear removes all processors from the race execution list.
func (r *Race[T]) Clear() *Race[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors = nil
	return r
}

// SetProcessors replaces all processors atomically.
func (r *Race[T]) SetProcessors(processors ...Chainable[T]) *Race[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors = make([]Chainable[T], len(processors))
	copy(r.processors, processors)
	return r
}

// Name returns the name of this connector.
func (r *Race[T]) Name() Name {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.name
}

// Metrics returns the metrics registry for this connector.
func (r *Race[T]) Metrics() *metricz.Registry {
	return r.metrics
}

// Tracer returns the tracer for this connector.
func (r *Race[T]) Tracer() *tracez.Tracer {
	return r.tracer
}

// Close gracefully shuts down observability components.
func (r *Race[T]) Close() error {
	if r.tracer != nil {
		r.tracer.Close()
	}
	r.hooks.Close()
	return nil
}

// OnWinner registers a handler for when a processor wins the race.
// The handler is called asynchronously when the first processor completes successfully.
func (r *Race[T]) OnWinner(handler func(context.Context, RaceEvent) error) error {
	_, err := r.hooks.Hook(RaceEventWinner, handler)
	return err
}

// OnAllFailed registers a handler for when all processors fail.
// The handler is called asynchronously when every processor returns an error.
func (r *Race[T]) OnAllFailed(handler func(context.Context, RaceEvent) error) error {
	_, err := r.hooks.Hook(RaceEventAllFailed, handler)
	return err
}
