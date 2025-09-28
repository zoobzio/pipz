package pipz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the Sequence connector.
const (
	// Metrics.
	SequenceProcessedTotal  = metricz.Key("sequence.processed.total")
	SequenceSuccessesTotal  = metricz.Key("sequence.successes.total")
	SequenceFailuresTotal   = metricz.Key("sequence.failures.total")
	SequenceStagesCompleted = metricz.Key("sequence.stages.completed")
	SequenceStagesTotal     = metricz.Key("sequence.stages.total")
	SequenceDurationMs      = metricz.Key("sequence.duration.ms")

	// Spans.
	SequenceProcessSpan = tracez.Key("sequence.process")
	SequenceStageSpan   = tracez.Key("sequence.stage")

	// Tags.
	SequenceTagStageCount    = tracez.Tag("sequence.stage_count")
	SequenceTagStageNumber   = tracez.Tag("sequence.stage_number")
	SequenceTagProcessorName = tracez.Tag("sequence.processor_name")
	SequenceTagSuccess       = tracez.Tag("sequence.success")
	SequenceTagError         = tracez.Tag("sequence.error")

	// Hook event keys.
	SequenceEventStageComplete = hookz.Key("sequence.stage_complete")
	SequenceEventAllComplete   = hookz.Key("sequence.all_complete")
)

// Sequence modification errors.
var (
	ErrIndexOutOfBounds = errors.New("index out of bounds")
	ErrEmptySequence    = errors.New("sequence is empty")
	ErrInvalidRange     = errors.New("invalid range")
)

// SequenceEvent represents a sequence processing event.
// This is emitted via hookz when individual stages complete or when
// all stages have finished, providing visibility into pipeline progress.
type SequenceEvent struct {
	Name            Name          // Connector name
	StageName       Name          // Name of the stage processor
	StageNumber     int           // Current stage number (1-based)
	TotalStages     int           // Total number of stages
	Success         bool          // Whether the stage succeeded
	Error           error         // Error if stage failed
	Duration        time.Duration // How long this stage took
	CompletedStages int           // Number of stages completed (for all_complete)
	TotalDuration   time.Duration // Total time for all stages (for all_complete)
	Timestamp       time.Time     // When the event occurred
}

// Sequence provides a type-safe sequence for processing values of type T.
// It maintains an ordered list of processors that are executed sequentially.
//
// Sequence offers a rich API with methods to dynamically modify the processor
// chain. This makes it ideal for scenarios where the processing steps need
// to be configured at runtime or modified based on conditions.
//
// Key features:
//   - Thread-safe for concurrent access
//   - Dynamic modification of processor chain
//   - Named processors for debugging
//   - Rich API for reordering and modification
//   - Fail-fast execution with detailed errors
//
// Sequence is the primary way to chain processors together.
//
// # Observability
//
// Sequence provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - sequence.processed.total: Counter of sequence operations
//   - sequence.successes.total: Counter of successful completions
//   - sequence.failures.total: Counter of failed sequences
//   - sequence.stages.completed: Gauge of stages completed
//   - sequence.stages.total: Gauge of total stages
//   - sequence.duration.ms: Gauge of total sequence duration
//
// Traces:
//   - sequence.process: Parent span for entire sequence
//   - sequence.stage: Child span for each individual stage
//
// Events (via hooks):
//   - sequence.stage_complete: Fired as each stage completes
//   - sequence.all_complete: Fired when all stages succeed
//
// Example with hooks:
//
//	const OrderPipelineName = pipz.Name("order-pipeline")
//	sequence := pipz.NewSequence(OrderPipelineName,
//	    validateOrder,
//	    calculatePricing,
//	    applyDiscounts,
//	    processPayment,
//	    sendConfirmation,
//	)
//
//	// Track pipeline progress
//	sequence.OnStageComplete(func(ctx context.Context, event SequenceEvent) error {
//	    log.Info("Stage %d/%d complete: %s (%.2fms)",
//	        event.StageNumber, event.TotalStages,
//	        event.StageName, event.Duration.Milliseconds())
//	    if !event.Success {
//	        alert.Warn("Stage %s failed: %v", event.StageName, event.Error)
//	    }
//	    return nil
//	})
//
//	// Monitor overall pipeline
//	sequence.OnAllComplete(func(ctx context.Context, event SequenceEvent) error {
//	    metrics.Record("pipeline.duration", event.TotalDuration)
//	    log.Success("Pipeline completed: %d stages in %v",
//	        event.CompletedStages, event.TotalDuration)
//	    return nil
//	})
type Sequence[T any] struct {
	name       Name
	processors []Chainable[T]
	mu         sync.RWMutex
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[SequenceEvent]
}

// NewSequence creates a new Sequence with optional initial processors.
// The sequence is ready to use immediately and can be safely
// accessed concurrently. Additional processors can be added using Register
// or the various modification methods.
//
// Example:
//
//	// Single line declaration
//	const (
//	    UserProcessingName = pipz.Name("user-processing")
//	    ValidateName = pipz.Name("validate")
//	    EnrichName = pipz.Name("enrich")
//	    AuditName = pipz.Name("audit")
//	)
//	sequence := pipz.NewSequence(UserProcessingName,
//	    pipz.Effect(ValidateName, validateUser),
//	    pipz.Apply(EnrichName, enrichUser),
//	    pipz.Effect(AuditName, auditUser),
//	)
//
//	// Or create empty and add later
//	sequence := pipz.NewSequence[User](UserProcessingName)
//	sequence.Register(validateUser, enrichUser)
func NewSequence[T any](name Name, processors ...Chainable[T]) *Sequence[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(SequenceProcessedTotal)
	metrics.Counter(SequenceSuccessesTotal)
	metrics.Counter(SequenceFailuresTotal)
	metrics.Gauge(SequenceStagesCompleted)
	metrics.Gauge(SequenceStagesTotal)
	metrics.Gauge(SequenceDurationMs)

	return &Sequence[T]{
		name:       name,
		processors: slices.Clone(processors),
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[SequenceEvent](),
	}
}

// Register adds processors to this Sequence.
// Processors are executed in the order they are registered.
//
// This method is thread-safe and can be called concurrently.
// New processors are appended to the existing chain, making
// Register ideal for building sequences incrementally:
//
//	sequence := pipz.NewSequence[Order]("order-processing")
//	sequence.Register(validateOrder)
//	sequence.Register(calculateTax, applyDiscount)
//	if config.RequiresApproval {
//	    sequence.Register(requireApproval)
//	}
func (c *Sequence[T]) Register(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// Process executes all registered processors on the input value.
// Each processor receives the output of the previous processor.
// The context is checked before each processor execution - if the context
// is canceled or expired, processing stops immediately.
// If any processor returns an error, execution stops and a Error
// is returned with rich debugging information.
//
// Process is thread-safe and can be called concurrently. The sequence's
// processor list is locked during execution to prevent modifications.
//
// Error handling includes:
//   - Processor name and stage index for debugging
//   - Original input data that caused the failure
//   - Execution duration for performance analysis
//   - Timeout/cancellation detection
//
// Context best practices:
//   - Always use context with timeout for production
//   - Check ctx.Err() in long-running processors
//   - Pass context through to external calls
func (c *Sequence[T]) Process(ctx context.Context, value T) (result T, err error) {
	defer recoverFromPanic(&result, &err, c.name, value)

	c.mu.RLock()
	processors := make([]Chainable[T], len(c.processors))
	copy(processors, c.processors)
	c.mu.RUnlock()

	// Handle nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// Track metrics
	c.metrics.Counter(SequenceProcessedTotal).Inc()
	c.metrics.Gauge(SequenceStagesTotal).Set(float64(len(processors)))
	start := time.Now()

	// Start main span
	ctx, span := c.tracer.StartSpan(ctx, SequenceProcessSpan)
	span.SetTag(SequenceTagStageCount, fmt.Sprintf("%d", len(processors)))
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		c.metrics.Gauge(SequenceDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(SequenceTagSuccess, "true")
			c.metrics.Counter(SequenceSuccessesTotal).Inc()
		} else {
			span.SetTag(SequenceTagSuccess, "false")
			c.metrics.Counter(SequenceFailuresTotal).Inc()
			if err != nil {
				span.SetTag(SequenceTagError, err.Error())
			}
		}
		span.Finish()
	}()

	result = value
	stagesCompleted := 0

	for i, proc := range processors {
		// Check context before starting processor
		select {
		case <-ctx.Done():
			// Context canceled/timed out - create appropriate error
			return result, &Error[T]{
				Err:       ctx.Err(),
				InputData: value,
				Path:      []Name{c.name},
				Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
				Canceled:  errors.Is(ctx.Err(), context.Canceled),
				Timestamp: time.Now(),
			}
		default:
			// Start span for this stage
			stageCtx, stageSpan := c.tracer.StartSpan(ctx, SequenceStageSpan)
			stageSpan.SetTag(SequenceTagStageNumber, fmt.Sprintf("%d", i+1))
			stageSpan.SetTag(SequenceTagProcessorName, string(proc.Name()))

			stageStart := time.Now()
			result, err = proc.Process(stageCtx, result)
			stageDuration := time.Since(stageStart)
			stageSpan.Finish()

			if err == nil {
				stagesCompleted++
				c.metrics.Gauge(SequenceStagesCompleted).Set(float64(stagesCompleted))

				// Emit stage complete event for successful stage
				_ = c.hooks.Emit(ctx, SequenceEventStageComplete, SequenceEvent{ //nolint:errcheck
					Name:        c.name,
					StageName:   proc.Name(),
					StageNumber: i + 1,
					TotalStages: len(processors),
					Success:     true,
					Error:       nil,
					Duration:    stageDuration,
					Timestamp:   time.Now(),
				})
			}
			if err != nil {
				// Emit stage complete event for failed stage
				_ = c.hooks.Emit(ctx, SequenceEventStageComplete, SequenceEvent{ //nolint:errcheck
					Name:        c.name,
					StageName:   proc.Name(),
					StageNumber: i + 1,
					TotalStages: len(processors),
					Success:     false,
					Error:       err,
					Duration:    stageDuration,
					Timestamp:   time.Now(),
				})

				var pipeErr *Error[T]
				if errors.As(err, &pipeErr) {
					// Prepend this sequence's name to the path
					pipeErr.Path = append([]Name{c.name}, pipeErr.Path...)
					return result, pipeErr
				}
				// Handle non-pipeline errors by wrapping them
				return result, &Error[T]{
					Timestamp: time.Now(),
					InputData: value,
					Err:       err,
					Path:      []Name{c.name},
				}
			}
		}
	}

	// All stages completed successfully - emit all_complete event
	totalDuration := time.Since(start)
	_ = c.hooks.Emit(ctx, SequenceEventAllComplete, SequenceEvent{ //nolint:errcheck
		Name:            c.name,
		TotalStages:     len(processors),
		CompletedStages: stagesCompleted,
		TotalDuration:   totalDuration,
		Success:         true,
		Timestamp:       time.Now(),
	})

	return result, nil
}

// Len returns the number of processors in the Sequence.
func (c *Sequence[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.processors)
}

// Clear removes all processors from the Sequence.
func (c *Sequence[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = c.processors[:0]
}

// Unshift adds processors to the front of the Sequence (runs first).
func (c *Sequence[T]) Unshift(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = slices.Insert(c.processors, 0, processors...)
}

// Push adds processors to the back of the Sequence (runs last).
func (c *Sequence[T]) Push(processors ...Chainable[T]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processors = append(c.processors, processors...)
}

// Shift removes and returns the first processor.
func (c *Sequence[T]) Shift() (Chainable[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Chainable[T]
		return zero, ErrEmptySequence
	}

	processor := c.processors[0]
	c.processors = c.processors[1:]
	return processor, nil
}

// Pop removes and returns the last processor.
func (c *Sequence[T]) Pop() (Chainable[T], error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.processors) == 0 {
		var zero Chainable[T]
		return zero, ErrEmptySequence
	}

	lastIndex := len(c.processors) - 1
	processor := c.processors[lastIndex]
	c.processors = c.processors[:lastIndex]
	return processor, nil
}

// Names returns the names of all processors in order.
func (c *Sequence[T]) Names() []Name {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]Name, len(c.processors))
	for i, proc := range c.processors {
		names[i] = proc.Name()
	}
	return names
}

// Remove removes the first processor with the specified name.
func (c *Sequence[T]) Remove(name Name) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == name {
			c.processors = slices.Delete(c.processors, i, i+1)
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", name)
}

// Replace replaces the first processor with the specified name.
func (c *Sequence[T]) Replace(name Name, processor Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == name {
			c.processors[i] = processor
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", name)
}

// After inserts processors after the first processor with the specified name.
func (c *Sequence[T]) After(afterName Name, processors ...Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == afterName {
			c.processors = slices.Insert(c.processors, i+1, processors...)
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", afterName)
}

// Before inserts processors before the first processor with the specified name.
func (c *Sequence[T]) Before(beforeName Name, processors ...Chainable[T]) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, proc := range c.processors {
		if proc.Name() == beforeName {
			c.processors = slices.Insert(c.processors, i, processors...)
			return nil
		}
	}

	return fmt.Errorf("processor %q not found", beforeName)
}

// Name returns the name of this sequence.
func (c *Sequence[T]) Name() Name {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.name
}

// Metrics returns the metrics registry for this connector.
func (c *Sequence[T]) Metrics() *metricz.Registry {
	return c.metrics
}

// Tracer returns the tracer for this connector.
func (c *Sequence[T]) Tracer() *tracez.Tracer {
	return c.tracer
}

// Close gracefully shuts down observability components.
func (c *Sequence[T]) Close() error {
	if c.tracer != nil {
		c.tracer.Close()
	}
	c.hooks.Close()
	return nil
}

// OnStageComplete registers a handler for when an individual stage completes.
// The handler is called asynchronously each time a stage finishes, whether it
// succeeds or fails. This provides visibility into pipeline progress in real-time.
func (c *Sequence[T]) OnStageComplete(handler func(context.Context, SequenceEvent) error) error {
	_, err := c.hooks.Hook(SequenceEventStageComplete, handler)
	return err
}

// OnAllComplete registers a handler for when all stages have completed successfully.
// The handler is called asynchronously after the entire sequence finishes without errors.
// This event includes aggregate statistics about the pipeline execution.
func (c *Sequence[T]) OnAllComplete(handler func(context.Context, SequenceEvent) error) error {
	_, err := c.hooks.Hook(SequenceEventAllComplete, handler)
	return err
}
