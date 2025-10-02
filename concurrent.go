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

// Observability constants for the Concurrent connector.
const (
	// Metrics.
	ConcurrentProcessedTotal = metricz.Key("concurrent.processed.total")
	ConcurrentSuccessesTotal = metricz.Key("concurrent.successes.total")
	ConcurrentProcessorCount = metricz.Key("concurrent.processor.count")
	ConcurrentDurationMs     = metricz.Key("concurrent.duration.ms")

	// Spans.
	ConcurrentProcessSpan   = tracez.Key("concurrent.process")
	ConcurrentProcessorSpan = tracez.Key("concurrent.processor")

	// Tags.
	ConcurrentTagProcessorCount = tracez.Tag("concurrent.processor_count")
	ConcurrentTagProcessorName  = tracez.Tag("concurrent.processor_name")
	ConcurrentTagSuccess        = tracez.Tag("concurrent.success")
	ConcurrentTagError          = tracez.Tag("concurrent.error")

	// Hook event keys.
	ConcurrentEventProcessorComplete = hookz.Key("concurrent.processor_complete")
	ConcurrentEventAllComplete       = hookz.Key("concurrent.all_complete")
)

// ConcurrentEvent represents a concurrent processing event.
// This is emitted via hookz when individual processors complete or when
// all processors have finished, allowing external systems to react to
// individual completions or aggregate results.
type ConcurrentEvent struct {
	Name            Name          // Connector name
	ProcessorName   Name          // Name of processor (for individual completion)
	ProcessorIndex  int           // Index of processor in the list
	TotalProcessors int           // Total number of processors
	Success         bool          // Whether the processor succeeded
	Error           error         // Error if processor failed
	Duration        time.Duration // How long this processor took
	CompletedCount  int           // Number of processors completed (for all_complete)
	SuccessCount    int           // Number of successful processors (for all_complete)
	FailureCount    int           // Number of failed processors (for all_complete)
	TotalDuration   time.Duration // Total time for all processors (for all_complete)
	Timestamp       time.Time     // When the event occurred
}

// Concurrent runs all processors in parallel with the original context preserved.
// Unlike fire-and-forget operations, this connector passes the original context
// directly to each processor, preserving distributed tracing information, spans,
// and other context values. Each processor receives a deep copy of the input,
// ensuring complete isolation. The original input is always returned unchanged.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// Use Concurrent when you need:
//   - Distributed tracing to work across concurrent operations
//   - All processors to respect the original context's cancellation
//   - To wait for all processors to complete before continuing
//   - Multiple side effects to happen simultaneously
//
// Common use cases:
//   - Sending traced notifications to multiple channels
//   - Updating multiple external systems with trace context
//   - Parallel logging with trace IDs preserved
//   - Triggering workflows that need distributed tracing
//   - Operations that must all complete or be canceled together
//
// Important characteristics:
//   - Input type must implement Cloner[T] interface
//   - All processors run regardless of individual failures
//   - Original input always returned (processors can't modify it)
//   - Context cancellation immediately affects all processors
//   - Preserves trace context and spans for distributed tracing
//   - Waits for all processors to complete
//
// Example:
//
//	type Order struct {
//	    ID     string
//	    Items  []Item
//	    Status string
//	}
//
//	func (o Order) Clone() Order {
//	    items := make([]Item, len(o.Items))
//	    copy(items, o.Items)
//	    return Order{
//	        ID:     o.ID,
//	        Items:  items,
//	        Status: o.Status,
//	    }
//	}
//
//	concurrent := pipz.NewConcurrent(
//	    sendEmailNotification,
//	    sendSMSNotification,
//	    updateInventorySystem,
//	    logToAnalytics,
//	)
//
// # Observability
//
// Concurrent provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - concurrent.processed.total: Counter of total concurrent operations
//   - concurrent.successes.total: Counter of fully successful operations
//   - concurrent.processor.count: Gauge showing number of processors
//   - concurrent.duration.ms: Gauge of total operation duration
//
// Traces:
//   - concurrent.process: Parent span for concurrent operation
//   - concurrent.processor: Child span for each parallel processor
//
// Events (via hooks):
//   - concurrent.processor_complete: Fired as each processor completes
//   - concurrent.all_complete: Fired when all processors finish
//
// Example with hooks:
//
//	concurrent := pipz.NewConcurrent(
//	    "multi-fetch",
//	    fetchUserData,
//	    fetchOrderHistory,
//	    fetchPreferences,
//	)
//
//	// Track individual completions
//	concurrent.OnProcessorComplete(func(ctx context.Context, event ConcurrentEvent) error {
//	    log.Printf("Processor %s completed in %v", event.ProcessorName, event.Duration)
//	    return nil
//	})
//
//	// Monitor overall performance
//	concurrent.OnAllComplete(func(ctx context.Context, event ConcurrentEvent) error {
//	    if event.FailureCount > 0 {
//	        log.Warn("%d/%d processors failed", event.FailureCount, event.ProcessorCount)
//	    }
//	    return nil
//	})
type Concurrent[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[ConcurrentEvent]
}

// NewConcurrent creates a new Concurrent connector.
func NewConcurrent[T Cloner[T]](name Name, processors ...Chainable[T]) *Concurrent[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(ConcurrentProcessedTotal)
	metrics.Counter(ConcurrentSuccessesTotal)
	metrics.Gauge(ConcurrentProcessorCount)
	metrics.Gauge(ConcurrentDurationMs)

	return &Concurrent[T]{
		name:       name,
		processors: processors,
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[ConcurrentEvent](),
	}
}

// Process implements the Chainable interface.
func (c *Concurrent[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, c.name, input)

	c.mu.RLock()
	processors := make([]Chainable[T], len(c.processors))
	copy(processors, c.processors)
	c.mu.RUnlock()

	// Track metrics
	c.metrics.Counter(ConcurrentProcessedTotal).Inc()
	c.metrics.Gauge(ConcurrentProcessorCount).Set(float64(len(processors)))
	start := time.Now()

	// Start main span
	ctx, span := c.tracer.StartSpan(ctx, ConcurrentProcessSpan)
	span.SetTag(ConcurrentTagProcessorCount, fmt.Sprintf("%d", len(processors)))
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		c.metrics.Gauge(ConcurrentDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(ConcurrentTagSuccess, "true")
			c.metrics.Counter(ConcurrentSuccessesTotal).Inc()
		} else {
			span.SetTag(ConcurrentTagSuccess, "false")
			span.SetTag(ConcurrentTagError, err.Error())
		}
		span.Finish()
	}()

	if len(processors) == 0 {
		return input, nil
	}

	var wg sync.WaitGroup
	wg.Add(len(processors))

	// Track completion statistics
	var completedMu sync.Mutex
	var successCount, failureCount int

	// Process all with the original context to preserve tracing
	for idx, processor := range processors {
		go func(index int, p Chainable[T]) {
			processorStart := time.Now()
			var processorErr error

			defer func() {
				// Always call wg.Done() even if Clone() or Process() panics
				// This prevents deadlock in wg.Wait()
				if r := recover(); r != nil {
					// Panic occurred, but we must complete wg.Done()
					// The goroutine can die after this, we just prevent deadlock
					_ = r // Acknowledge the panic but continue
					processorErr = fmt.Errorf("panic: %v", r)
				}

				// Emit processor complete event
				duration := time.Since(processorStart)
				success := processorErr == nil

				// Update counts
				completedMu.Lock()
				if success {
					successCount++
				} else {
					failureCount++
				}
				completedMu.Unlock()

				// Emit event
				if c.hooks.ListenerCount(ConcurrentEventProcessorComplete) > 0 {
					_ = c.hooks.Emit(ctx, ConcurrentEventProcessorComplete, ConcurrentEvent{ //nolint:errcheck
						Name:            c.name,
						ProcessorName:   p.Name(),
						ProcessorIndex:  index,
						TotalProcessors: len(processors),
						Success:         success,
						Error:           processorErr,
						Duration:        duration,
						Timestamp:       time.Now(),
					})
				}

				wg.Done()
			}()

			// Start span for this processor
			procCtx, procSpan := c.tracer.StartSpan(ctx, ConcurrentProcessorSpan)
			procSpan.SetTag(ConcurrentTagProcessorName, string(p.Name()))
			defer procSpan.Finish()

			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Process with the span context
			if _, err := p.Process(procCtx, inputCopy); err != nil {
				// Record error in span
				procSpan.SetTag(ConcurrentTagError, err.Error())
				processorErr = err
			}
		}(idx, processor)
	}

	// Wait for completion or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All processors completed - emit all_complete event
		totalDuration := time.Since(start)
		if c.hooks.ListenerCount(ConcurrentEventAllComplete) > 0 {
			_ = c.hooks.Emit(ctx, ConcurrentEventAllComplete, ConcurrentEvent{ //nolint:errcheck
				Name:            c.name,
				TotalProcessors: len(processors),
				CompletedCount:  successCount + failureCount,
				SuccessCount:    successCount,
				FailureCount:    failureCount,
				TotalDuration:   totalDuration,
				Timestamp:       time.Now(),
			})
		}
		return input, nil
	case <-ctx.Done():
		// Context canceled - return without error as processors run independently
		return input, nil
	}
}

// Add appends a processor to the concurrent execution list.
func (c *Concurrent[T]) Add(processor Chainable[T]) *Concurrent[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processor)
	return c
}

// Remove removes the processor at the specified index.
func (c *Concurrent[T]) Remove(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.processors) {
		return ErrIndexOutOfBounds
	}

	c.processors = append(c.processors[:index], c.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (c *Concurrent[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Clear removes all processors from the concurrent execution list.
func (c *Concurrent[T]) Clear() *Concurrent[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = nil
	return c
}

// SetProcessors replaces all processors atomically.
func (c *Concurrent[T]) SetProcessors(processors ...Chainable[T]) *Concurrent[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = make([]Chainable[T], len(processors))
	copy(c.processors, processors)
	return c
}

// Name returns the name of this connector.
func (c *Concurrent[T]) Name() Name {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.name
}

// Metrics returns the metrics registry for this connector.
func (c *Concurrent[T]) Metrics() *metricz.Registry {
	return c.metrics
}

// Tracer returns the tracer for this connector.
func (c *Concurrent[T]) Tracer() *tracez.Tracer {
	return c.tracer
}

// Close gracefully shuts down observability components.
func (c *Concurrent[T]) Close() error {
	if c.tracer != nil {
		c.tracer.Close()
	}
	c.hooks.Close()
	return nil
}

// OnProcessorComplete registers a handler for when an individual processor completes.
// The handler is called asynchronously each time a processor finishes, whether it
// succeeds or fails. This allows monitoring individual processor outcomes in real-time.
func (c *Concurrent[T]) OnProcessorComplete(handler func(context.Context, ConcurrentEvent) error) error {
	_, err := c.hooks.Hook(ConcurrentEventProcessorComplete, handler)
	return err
}

// OnAllComplete registers a handler for when all processors have completed.
// The handler is called asynchronously after all processors finish executing.
// The event includes aggregate statistics about successes and failures.
func (c *Concurrent[T]) OnAllComplete(handler func(context.Context, ConcurrentEvent) error) error {
	_, err := c.hooks.Hook(ConcurrentEventAllComplete, handler)
	return err
}
