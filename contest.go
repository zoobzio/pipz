package pipz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the Contest connector.
const (
	// Metrics.
	ContestProcessedTotal = metricz.Key("contest.processed.total")
	ContestSuccessesTotal = metricz.Key("contest.successes.total")
	ContestWinsTotal      = metricz.Key("contest.wins.total")
	ContestNoConditionMet = metricz.Key("contest.no_condition_met.total")
	ContestAllFailedTotal = metricz.Key("contest.all_failed.total")
	ContestProcessorCount = metricz.Key("contest.processor.count")
	ContestDurationMs     = metricz.Key("contest.duration.ms")

	// Spans.
	ContestProcessSpan   = tracez.Key("contest.process")
	ContestProcessorSpan = tracez.Key("contest.processor")

	// Tags.
	ContestTagProcessorCount = tracez.Tag("contest.processor_count")
	ContestTagProcessorName  = tracez.Tag("contest.processor_name")
	ContestTagWinner         = tracez.Tag("contest.winner")
	ContestTagConditionMet   = tracez.Tag("contest.condition_met")
	ContestTagSuccess        = tracez.Tag("contest.success")
	ContestTagError          = tracez.Tag("contest.error")

	// Hook event keys.
	ContestEventWinner      = hookz.Key("contest.winner")
	ContestEventNoCondition = hookz.Key("contest.no_condition_met")
	ContestEventAllFailed   = hookz.Key("contest.all_failed")
)

// ContestEvent represents a contest outcome event.
// This is emitted via hookz when a processor wins the contest,
// no results meet the condition, or all processors fail.
type ContestEvent struct {
	Name           Name          // Connector name
	Winner         Name          // Name of winning processor (if any)
	ConditionMet   bool          // Whether the winning condition was met
	ProcessorCount int           // Total processors in the contest
	AllFailed      bool          // Whether all processors failed with errors
	Duration       time.Duration // How long the contest took
	Error          error         // Error if contest failed
	Timestamp      time.Time     // When the event occurred
}

// Contest runs all processors in parallel and returns the first result that
// meets a specified condition. Contest combines competitive processing (like Race)
// with conditional selection, allowing you to define what makes a "winner" beyond
// just being first to complete.
//
// Context handling: Contest uses context.WithCancel(ctx) to create a derived context
// that preserves all parent context values (including trace IDs) while allowing
// cancellation of other processors when a winner meeting the condition is found.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// This pattern excels when you have multiple ways to get a result and want the
// fastest one that meets specific criteria:
//   - Finding the cheapest shipping rate under a time constraint
//   - Getting the first API response with required data completeness
//   - Querying multiple sources for the best quality result quickly
//   - Racing services where the "best" result matters more than just "first"
//   - Any scenario where you need speed AND quality criteria
//
// Key behaviors:
//   - First result meeting the condition wins and cancels others
//   - If no results meet the condition, returns the original input with an error
//   - Each processor gets an isolated copy via Clone()
//   - Condition is evaluated as results arrive (no waiting for all)
//   - Can reduce latency while ensuring quality constraints
//
// Example:
//
//	// Find the first shipping rate under $50
//	contest := pipz.NewContest("cheapest-rate",
//	    func(_ context.Context, rate Rate) bool {
//	        return rate.Cost < 50.00
//	    },
//	    fedexRates,
//	    upsRates,
//	    uspsRates,
//	)
//
// # Observability
//
// Contest provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - contest.processed.total: Counter of contest operations
//   - contest.winners.total: Counter of contests with winners
//   - contest.no_condition_met.total: Counter of contests where no result met condition
//   - contest.all_failed.total: Counter of contests where all processors failed
//   - contest.processor.count: Gauge of processors in contest
//   - contest.duration.ms: Gauge of time to find winner
//
// Traces:
//   - contest.process: Parent span for contest operation
//   - contest.processor: Child span for each competing processor
//
// Events (via hooks):
//   - contest.winner: Fired when a winner is found
//   - contest.no_condition_met: Fired when no results meet condition
//   - contest.all_failed: Fired when all processors fail
//
// Example with hooks:
//
//	contest := pipz.NewContest("rate-finder",
//	    func(ctx context.Context, rate Rate) bool {
//	        return rate.Cost < 50.00 && rate.Days <= 3
//	    },
//	    fedexRates,
//	    upsRates,
//	    uspsRates,
//	)
//
//	// Track winning rates
//	contest.OnWinner(func(ctx context.Context, event ContestEvent) error {
//	    log.Printf("Winner: %s completed in %v", event.WinnerName, event.Duration)
//	    return nil
//	})
//
//	// Alert when no good rates found
//	contest.OnNoConditionMet(func(ctx context.Context, event ContestEvent) error {
//	    alert.Send("No shipping rates met criteria")
//	    return nil
//	})
type Contest[T Cloner[T]] struct {
	name       Name
	condition  func(context.Context, T) bool
	processors []Chainable[T]
	mu         sync.RWMutex
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[ContestEvent]
}

// NewContest creates a new Contest connector with the specified winning condition.
// The condition function determines which results are acceptable winners.
// A result must both complete successfully AND meet the condition to win.
func NewContest[T Cloner[T]](name Name, condition func(context.Context, T) bool, processors ...Chainable[T]) *Contest[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(ContestProcessedTotal)
	metrics.Counter(ContestSuccessesTotal)
	metrics.Counter(ContestWinsTotal)
	metrics.Counter(ContestNoConditionMet)
	metrics.Counter(ContestAllFailedTotal)
	metrics.Gauge(ContestProcessorCount)
	metrics.Gauge(ContestDurationMs)

	return &Contest[T]{
		name:       name,
		condition:  condition,
		processors: processors,
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[ContestEvent](),
	}
}

// Process implements the Chainable interface.
func (c *Contest[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, c.name, input)

	c.mu.RLock()
	processors := make([]Chainable[T], len(c.processors))
	copy(processors, c.processors)
	condition := c.condition
	c.mu.RUnlock()

	// Track metrics
	c.metrics.Counter(ContestProcessedTotal).Inc()
	c.metrics.Gauge(ContestProcessorCount).Set(float64(len(processors)))
	start := time.Now()

	// Start main span
	ctx, span := c.tracer.StartSpan(ctx, ContestProcessSpan)
	span.SetTag(ContestTagProcessorCount, fmt.Sprintf("%d", len(processors)))
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		c.metrics.Gauge(ContestDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(ContestTagSuccess, "true")
			c.metrics.Counter(ContestSuccessesTotal).Inc()
		} else {
			span.SetTag(ContestTagSuccess, "false")
			if err != nil {
				span.SetTag(ContestTagError, err.Error())
			}
		}
		span.Finish()
	}()

	if len(processors) == 0 {
		var zero T
		return zero, &Error[T]{
			Path:      []Name{c.name},
			Err:       fmt.Errorf("no processors provided to Contest"),
			InputData: input,
			Timestamp: time.Now(),
			Duration:  0,
		}
	}

	if condition == nil {
		var zero T
		return zero, &Error[T]{
			Path:      []Name{c.name},
			Err:       fmt.Errorf("no condition provided to Contest"),
			InputData: input,
			Timestamp: time.Now(),
			Duration:  0,
		}
	}

	// Create channels for results and completion tracking
	type contestResult struct {
		data T
		err  error
		idx  int
	}

	resultCh := make(chan contestResult, len(processors))
	// Create a cancellable context to stop other processors when one wins
	// This derives from the original context, preserving trace data
	contestCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch all processors
	for i, processor := range processors {
		go func(idx int, p Chainable[T]) {
			// Start span for this processor
			procCtx, procSpan := c.tracer.StartSpan(contestCtx, ContestProcessorSpan)
			procSpan.SetTag(ContestTagProcessorName, p.Name())
			defer procSpan.Finish()

			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Use processor context which includes span
			data, processErr := p.Process(procCtx, inputCopy)
			select {
			case resultCh <- contestResult{data: data, err: processErr, idx: idx}:
			case <-contestCtx.Done():
			}
		}(i, processor)
	}

	// Collect results and check conditions
	var allErrors []error
	completedCount := 0

	for completedCount < len(processors) {
		select {
		case res := <-resultCh:
			completedCount++

			if res.err == nil {
				// Check if this successful result meets the condition
				if condition(ctx, res.data) {
					// Winner! Cancel other goroutines and return
					cancel()
					winnerName := processors[res.idx].Name()
					span.SetTag(ContestTagWinner, winnerName)
					span.SetTag(ContestTagConditionMet, "true")
					c.metrics.Counter(ContestWinsTotal).Inc()

					// Emit winner event
					_ = c.hooks.Emit(ctx, ContestEventWinner, ContestEvent{ //nolint:errcheck
						Name:           c.name,
						Winner:         winnerName,
						ConditionMet:   true,
						ProcessorCount: len(processors),
						Duration:       time.Since(start),
						Timestamp:      time.Now(),
					})

					return res.data, nil
				}
				// Result doesn't meet condition, continue waiting for others
			} else {
				// Track errors for potential return if all fail
				if res.err != nil {
					allErrors = append(allErrors, res.err)
				}
			}

		case <-ctx.Done():
			// Context canceled - return original input
			return input, nil
		}
	}

	// No processor produced a result meeting the condition
	if len(allErrors) == len(processors) {
		// All processors failed with errors
		c.metrics.Counter(ContestAllFailedTotal).Inc()
		err = fmt.Errorf("all processors failed: %d errors", len(allErrors))

		// Emit all failed event
		_ = c.hooks.Emit(ctx, ContestEventAllFailed, ContestEvent{ //nolint:errcheck
			Name:           c.name,
			AllFailed:      true,
			ProcessorCount: len(processors),
			Duration:       time.Since(start),
			Error:          err,
			Timestamp:      time.Now(),
		})
	} else {
		// Some succeeded but none met the condition
		c.metrics.Counter(ContestNoConditionMet).Inc()
		span.SetTag(ContestTagConditionMet, "false")
		err = fmt.Errorf("no processor results met the specified condition")

		// Emit no condition met event
		_ = c.hooks.Emit(ctx, ContestEventNoCondition, ContestEvent{ //nolint:errcheck
			Name:           c.name,
			ConditionMet:   false,
			ProcessorCount: len(processors),
			Duration:       time.Since(start),
			Error:          err,
			Timestamp:      time.Now(),
		})
	}

	return input, &Error[T]{
		Path:      []Name{c.name},
		Err:       err,
		InputData: input,
		Timestamp: time.Now(),
		Duration:  0,
	}
}

// SetCondition updates the winning condition.
// This allows changing the criteria at runtime.
func (c *Contest[T]) SetCondition(condition func(context.Context, T) bool) *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.condition = condition
	return c
}

// Add appends a processor to the contest execution list.
func (c *Contest[T]) Add(processor Chainable[T]) *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processor)
	return c
}

// Remove removes the processor at the specified index.
func (c *Contest[T]) Remove(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = append(c.processors[:index], c.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (c *Contest[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Clear removes all processors from the contest execution list.
func (c *Contest[T]) Clear() *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = nil
	return c
}

// SetProcessors replaces all processors atomically.
func (c *Contest[T]) SetProcessors(processors ...Chainable[T]) *Contest[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = make([]Chainable[T], len(processors))
	copy(c.processors, processors)
	return c
}

// Name returns the name of this connector.
func (c *Contest[T]) Name() Name {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.name
}

// Metrics returns the metrics registry for this connector.
func (c *Contest[T]) Metrics() *metricz.Registry {
	return c.metrics
}

// Tracer returns the tracer for this connector.
func (c *Contest[T]) Tracer() *tracez.Tracer {
	return c.tracer
}

// Close gracefully shuts down observability components.
func (c *Contest[T]) Close() error {
	if c.tracer != nil {
		c.tracer.Close()
	}
	c.hooks.Close()
	return nil
}

// OnWinner registers a handler for when a processor wins the contest.
// The handler is called asynchronously when a result meets the winning condition.
func (c *Contest[T]) OnWinner(handler func(context.Context, ContestEvent) error) error {
	_, err := c.hooks.Hook(ContestEventWinner, handler)
	return err
}

// OnNoCondition registers a handler for when no results meet the condition.
// The handler is called asynchronously when all processors complete but none meet the condition.
func (c *Contest[T]) OnNoCondition(handler func(context.Context, ContestEvent) error) error {
	_, err := c.hooks.Hook(ContestEventNoCondition, handler)
	return err
}

// OnAllFailed registers a handler for when all processors fail with errors.
// The handler is called asynchronously when every processor returns an error.
func (c *Contest[T]) OnAllFailed(handler func(context.Context, ContestEvent) error) error {
	_, err := c.hooks.Hook(ContestEventAllFailed, handler)
	return err
}
