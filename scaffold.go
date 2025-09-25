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

// Observability constants for the Scaffold connector.
const (
	// Metrics.
	ScaffoldProcessedTotal = metricz.Key("scaffold.processed.total")
	ScaffoldLaunchedTotal  = metricz.Key("scaffold.launched.total")

	// Spans.
	ScaffoldProcessSpan = tracez.Key("scaffold.process")

	// Tags.
	ScaffoldTagProcessorCount = tracez.Tag("scaffold.processor_count")

	// Hook event keys.
	ScaffoldEventLaunched    = hookz.Key("scaffold.launched")
	ScaffoldEventAllLaunched = hookz.Key("scaffold.all_launched")
)

// ScaffoldEvent represents a scaffold launch event.
// This is emitted via hookz when background processors are launched,
// providing visibility into fire-and-forget operations.
type ScaffoldEvent struct {
	Name           Name      // Connector name
	ProcessorName  Name      // Name of the processor being launched
	ProcessorCount int       // Total number of processors
	ProcessorIndex int       // Index of this processor (for launched event)
	Timestamp      time.Time // When the event occurred
}

// Scaffold runs all processors in parallel with context isolation for true fire-and-forget behavior.
// Unlike Concurrent, Scaffold uses context.WithoutCancel to ensure processors continue
// running even if the parent context is canceled. This is ideal for operations that
// must complete regardless of the main pipeline's state.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// Use Scaffold when you need:
//   - True fire-and-forget operations that outlive the request
//   - Background tasks that shouldn't be canceled with the main flow
//   - Cleanup or logging operations that must complete
//   - Non-critical side effects that shouldn't block the pipeline
//
// Common use cases:
//   - Asynchronous audit logging
//   - Background cache warming
//   - Non-critical notifications
//   - Metrics collection
//   - Cleanup tasks that should complete independently
//
// Important characteristics:
//   - Input type must implement Cloner[T] interface
//   - Processors continue even after parent context cancellation
//   - Returns immediately without waiting for completion
//   - Original input always returned unchanged
//   - No error reporting from background processors
//   - Trace context is preserved (but cancellation is not)
//
// Example:
//
//	scaffold := pipz.NewScaffold(
//	    "async-operations",
//	    asyncAuditLog,
//	    warmCache,
//	    collectMetrics,
//	)
//
//	// Returns immediately, processors run in background
//	result, err := scaffold.Process(ctx, order)
//
// # Observability
//
// Scaffold provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - scaffold.processed.total: Counter of scaffold operations
//   - scaffold.launched.total: Counter of processors launched
//
// Traces:
//   - scaffold.process: Span for scaffold operation (completes immediately)
//
// Events (via hooks):
//   - scaffold.launched: Fired as each processor is launched
//   - scaffold.all_launched: Fired after all processors are launched
//
// Example with hooks:
//
//	scaffold := pipz.NewScaffold("background",
//	    auditLogger,
//	    metricsCollector,
//	    cacheWarmer,
//	)
//
//	// Track background operations
//	scaffold.OnLaunched(func(ctx context.Context, event ScaffoldEvent) error {
//	    log.Debug("Launched background task: %s", event.ProcessorName)
//	    metrics.Inc("background.tasks.launched", event.ProcessorName)
//	    return nil
//	})
//
//	// Monitor total launches
//	scaffold.OnAllLaunched(func(ctx context.Context, event ScaffoldEvent) error {
//	    log.Info("Launched %d background tasks", event.ProcessorCount)
//	    return nil
//	})
type Scaffold[T Cloner[T]] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[ScaffoldEvent]
}

// NewScaffold creates a new Scaffold connector.
func NewScaffold[T Cloner[T]](name Name, processors ...Chainable[T]) *Scaffold[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(ScaffoldProcessedTotal)
	metrics.Counter(ScaffoldLaunchedTotal)

	return &Scaffold[T]{
		name:       name,
		processors: processors,
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[ScaffoldEvent](),
	}
}

// Process implements the Chainable interface.
func (s *Scaffold[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, s.name, input)

	// Track metrics
	s.metrics.Counter(ScaffoldProcessedTotal).Inc()

	// Start span
	ctx, span := s.tracer.StartSpan(ctx, ScaffoldProcessSpan)
	defer span.Finish()

	s.mu.RLock()
	processors := make([]Chainable[T], len(s.processors))
	copy(processors, s.processors)
	s.mu.RUnlock()

	span.SetTag(ScaffoldTagProcessorCount, fmt.Sprintf("%d", len(processors)))

	if len(processors) == 0 {
		return input, nil
	}

	// Create context that won't be canceled when parent is
	// This preserves values like trace IDs while removing cancellation
	bgCtx := context.WithoutCancel(ctx)

	// Launch all processors in background without waiting
	for i, processor := range processors {
		s.metrics.Counter(ScaffoldLaunchedTotal).Inc()

		// Emit launched event for each processor
		_ = s.hooks.Emit(ctx, ScaffoldEventLaunched, ScaffoldEvent{ //nolint:errcheck
			Name:           s.name,
			ProcessorName:  processor.Name(),
			ProcessorCount: len(processors),
			ProcessorIndex: i,
			Timestamp:      time.Now(),
		})

		go func(p Chainable[T]) {
			// Create an isolated copy using the Clone method
			inputCopy := input.Clone()

			// Process with isolated context - continues even if parent canceled
			if _, err := p.Process(bgCtx, inputCopy); err != nil {
				// Fire-and-forget: errors are not reported back
				_ = err
			}
		}(processor)
	}

	// Emit all launched event
	_ = s.hooks.Emit(ctx, ScaffoldEventAllLaunched, ScaffoldEvent{ //nolint:errcheck
		Name:           s.name,
		ProcessorCount: len(processors),
		Timestamp:      time.Now(),
	})

	// Return immediately without waiting
	return input, nil
}

// Add appends a processor to the scaffold execution list.
func (s *Scaffold[T]) Add(processor Chainable[T]) *Scaffold[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processors = append(s.processors, processor)
	return s
}

// Remove removes the processor at the specified index.
func (s *Scaffold[T]) Remove(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.processors) {
		return ErrIndexOutOfBounds
	}

	s.processors = append(s.processors[:index], s.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (s *Scaffold[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.processors)
}

// Clear removes all processors from the scaffold execution list.
func (s *Scaffold[T]) Clear() *Scaffold[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processors = nil
	return s
}

// SetProcessors replaces all processors atomically.
func (s *Scaffold[T]) SetProcessors(processors ...Chainable[T]) *Scaffold[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processors = make([]Chainable[T], len(processors))
	copy(s.processors, processors)
	return s
}

// Name returns the name of this connector.
func (s *Scaffold[T]) Name() Name {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}

// Metrics returns the metrics registry for this connector.
func (s *Scaffold[T]) Metrics() *metricz.Registry {
	return s.metrics
}

// Tracer returns the tracer for this connector.
func (s *Scaffold[T]) Tracer() *tracez.Tracer {
	return s.tracer
}

// Close gracefully shuts down observability components.
func (s *Scaffold[T]) Close() error {
	if s.tracer != nil {
		s.tracer.Close()
	}
	s.hooks.Close()
	return nil
}

// OnLaunched registers a handler for when a processor is launched.
// The handler is called synchronously as each processor is launched in the background.
// This provides visibility into which background operations have been started.
func (s *Scaffold[T]) OnLaunched(handler func(context.Context, ScaffoldEvent) error) error {
	_, err := s.hooks.Hook(ScaffoldEventLaunched, handler)
	return err
}

// OnAllLaunched registers a handler for when all processors have been launched.
// The handler is called synchronously after all background processors have been started.
// Note: This does not mean the processors have completed, only that they've been launched.
func (s *Scaffold[T]) OnAllLaunched(handler func(context.Context, ScaffoldEvent) error) error {
	_, err := s.hooks.Hook(ScaffoldEventAllLaunched, handler)
	return err
}
