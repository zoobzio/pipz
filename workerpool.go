package pipz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the WorkerPool connector.
const (
	// Metrics.
	WorkerPoolProcessedTotal = metricz.Key("workerpool.processed.total")
	WorkerPoolSuccessesTotal = metricz.Key("workerpool.successes.total")
	WorkerPoolTasksTotal     = metricz.Key("workerpool.tasks.total")
	WorkerPoolWorkersMax     = metricz.Key("workerpool.workers.max")
	WorkerPoolWorkersActive  = metricz.Key("workerpool.workers.active")
	WorkerPoolQueueWaitMs    = metricz.Key("workerpool.queue.wait.ms")
	WorkerPoolDurationMs     = metricz.Key("workerpool.duration.ms")

	// Spans.
	WorkerPoolProcessSpan = tracez.Key("workerpool.process")
	WorkerPoolTaskSpan    = tracez.Key("workerpool.task")

	// Tags.
	WorkerPoolTagProcessorCount = tracez.Tag("workerpool.processor_count")
	WorkerPoolTagWorkerCount    = tracez.Tag("workerpool.worker_count")
	WorkerPoolTagProcessorName  = tracez.Tag("workerpool.processor_name")
	WorkerPoolTagSuccess        = tracez.Tag("workerpool.success")
	WorkerPoolTagError          = tracez.Tag("workerpool.error")

	// Hook event keys.
	WorkerPoolEventTaskQueued   = hookz.Key("workerpool.task_queued")
	WorkerPoolEventTaskStarted  = hookz.Key("workerpool.task_started")
	WorkerPoolEventTaskComplete = hookz.Key("workerpool.task_complete")
	WorkerPoolEventAllComplete  = hookz.Key("workerpool.all_complete")
)

// WorkerPoolEvent represents a worker pool task event.
// This is emitted via hookz when tasks are queued, started, completed,
// or when all tasks finish, providing visibility into task lifecycle and queue management.
type WorkerPoolEvent struct {
	Name            Name          // Connector name
	ProcessorName   Name          // Name of the processor
	WorkerCount     int           // Maximum number of workers
	ActiveWorkers   int           // Number of active workers
	QueueWaitTime   time.Duration // Time spent waiting for worker (for task_started)
	Success         bool          // Whether the task succeeded
	Error           error         // Error if task failed
	Duration        time.Duration // How long the task took
	TotalTasks      int           // Total number of tasks (for all_complete)
	CompletedTasks  int           // Number of completed tasks (for all_complete)
	SuccessfulTasks int           // Number of successful tasks (for all_complete)
	FailedTasks     int           // Number of failed tasks (for all_complete)
	TotalDuration   time.Duration // Total time for all tasks (for all_complete)
	Timestamp       time.Time     // When the event occurred
}

// WorkerPool provides bounded parallel execution with a fixed number of workers.
// Uses semaphore pattern to limit concurrent processor execution while maintaining
// the same API and behavior as other connectors in the pipz ecosystem.
//
// The input type T must implement Cloner[T] to provide safe concurrent processing.
// Each processor receives an isolated copy of the input data.
//
// Example:
//
//	pool := pipz.NewWorkerPool("api-calls", 5,
//	    pipz.Apply("service-a", callServiceA),
//	    pipz.Apply("service-b", callServiceB),
//	    pipz.Apply("service-c", callServiceC),
//	)
//
// # Observability
//
// WorkerPool provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - workerpool.processed.total: Counter of workerpool operations
//   - workerpool.successes.total: Counter of successful completions
//   - workerpool.tasks.total: Counter of total tasks queued
//   - workerpool.workers.max: Gauge of maximum worker count
//   - workerpool.workers.active: Gauge of currently active workers
//   - workerpool.queue.wait.ms: Gauge of time tasks wait for workers
//   - workerpool.duration.ms: Gauge of total operation duration
//
// Traces:
//   - workerpool.process: Parent span for entire workerpool operation
//   - workerpool.task: Child span for each individual task
//
// Events (via hooks):
//   - workerpool.task_queued: Fired when a task is queued
//   - workerpool.task_started: Fired when a task acquires a worker
//   - workerpool.task_complete: Fired when a task finishes
//   - workerpool.all_complete: Fired when all tasks are done
//
// Example with hooks:
//
//	pool := pipz.NewWorkerPool("batch-processor", 10,
//	    processItem1,
//	    processItem2,
//	    processItem3,
//	)
//
//	// Monitor queue wait times
//	pool.OnTaskStarted(func(ctx context.Context, event WorkerPoolEvent) error {
//	    if event.QueueWaitTime > time.Second {
//	        log.Warn("Task %s waited %v for worker",
//	            event.ProcessorName, event.QueueWaitTime)
//	    }
//	    return nil
//	})
//
//	// Track completion statistics
//	pool.OnAllComplete(func(ctx context.Context, event WorkerPoolEvent) error {
//	    log.Info("Batch complete: %d/%d succeeded in %v",
//	        event.SuccessfulTasks, event.TotalTasks, event.TotalDuration)
//	    if event.FailedTasks > 0 {
//	        alert.Warn("%d tasks failed in batch", event.FailedTasks)
//	    }
//	    return nil
//	})
//
//nolint:govet // fieldalignment: 8-byte difference in struct size is negligible vs. readability
type WorkerPool[T Cloner[T]] struct {
	processors []Chainable[T]
	sem        chan struct{} // Semaphore for worker limit
	name       Name
	mu         sync.RWMutex  // Thread safety
	timeout    time.Duration // Optional per-task timeout
	queueSize  int           // Optional queue size (unused but kept for future)
	clock      clockz.Clock  // Clock for time operations
	metrics    *metricz.Registry
	tracer     *tracez.Tracer
	hooks      *hookz.Hooks[WorkerPoolEvent]
}

// NewWorkerPool creates a WorkerPool with specified worker count.
// Workers parameter controls maximum concurrent processors (semaphore slots).
func NewWorkerPool[T Cloner[T]](name Name, workers int, processors ...Chainable[T]) *WorkerPool[T] {
	if workers <= 0 {
		workers = 1 // Sensible default
	}

	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(WorkerPoolProcessedTotal)
	metrics.Counter(WorkerPoolSuccessesTotal)
	metrics.Counter(WorkerPoolTasksTotal)
	metrics.Gauge(WorkerPoolWorkersMax)
	metrics.Gauge(WorkerPoolWorkersActive)
	metrics.Gauge(WorkerPoolQueueWaitMs)
	metrics.Gauge(WorkerPoolDurationMs)

	wp := &WorkerPool[T]{
		processors: make([]Chainable[T], len(processors)),
		sem:        make(chan struct{}, workers),
		name:       name,
		timeout:    0, // Default no timeout
		queueSize:  0, // Default no buffering
		clock:      clockz.RealClock,
		metrics:    metrics,
		tracer:     tracez.New(),
		hooks:      hookz.New[WorkerPoolEvent](),
	}
	copy(wp.processors, processors) // Defensive copy

	// Set initial worker count
	wp.metrics.Gauge(WorkerPoolWorkersMax).Set(float64(workers))

	return wp
}

// Name returns the name of this connector.
func (w *WorkerPool[T]) Name() Name {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.name
}

// Process implements the Chainable interface.
func (w *WorkerPool[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, w.name, input)
	w.mu.RLock()
	processors := make([]Chainable[T], len(w.processors))
	copy(processors, w.processors)
	timeout := w.timeout
	clock := w.getClock()
	w.mu.RUnlock()

	// Track metrics
	w.metrics.Counter(WorkerPoolProcessedTotal).Inc()
	start := time.Now()

	// Start main span
	ctx, span := w.tracer.StartSpan(ctx, WorkerPoolProcessSpan)
	span.SetTag(WorkerPoolTagProcessorCount, fmt.Sprintf("%d", len(processors)))
	span.SetTag(WorkerPoolTagWorkerCount, fmt.Sprintf("%d", cap(w.sem)))
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		w.metrics.Gauge(WorkerPoolDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(WorkerPoolTagSuccess, "true")
			w.metrics.Counter(WorkerPoolSuccessesTotal).Inc()
		} else {
			span.SetTag(WorkerPoolTagSuccess, "false")
			if err != nil {
				span.SetTag(WorkerPoolTagError, err.Error())
			}
		}
		span.Finish()
	}()

	if len(processors) == 0 {
		// Emit all complete event for empty processor list
		_ = w.hooks.Emit(ctx, WorkerPoolEventAllComplete, WorkerPoolEvent{ //nolint:errcheck
			Name:            w.name,
			WorkerCount:     cap(w.sem),
			TotalTasks:      0,
			CompletedTasks:  0,
			SuccessfulTasks: 0,
			FailedTasks:     0,
			TotalDuration:   0,
			Timestamp:       time.Now(),
		})
		return input, nil
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(processors))

	// Track completion statistics for all_complete event
	var completedMu sync.Mutex
	var successfulTasks, failedTasks int
	allTasksStart := time.Now()

	for _, processor := range processors {
		wg.Add(1)
		w.metrics.Counter(WorkerPoolTasksTotal).Inc()

		// Emit task queued event
		_ = w.hooks.Emit(ctx, WorkerPoolEventTaskQueued, WorkerPoolEvent{ //nolint:errcheck
			Name:          w.name,
			ProcessorName: processor.Name(),
			WorkerCount:   cap(w.sem),
			ActiveWorkers: len(w.sem),
			Timestamp:     time.Now(),
		})

		go func(p Chainable[T]) {
			defer wg.Done()

			// Start span for this task
			taskCtx, taskSpan := w.tracer.StartSpan(ctx, WorkerPoolTaskSpan)
			taskSpan.SetTag(WorkerPoolTagProcessorName, string(p.Name()))
			defer taskSpan.Finish()

			// Track queue wait time
			queueStart := time.Now()

			// Acquire semaphore slot (blocks if all workers busy)
			select {
			case w.sem <- struct{}{}:
				// Record queue wait time
				queueWait := time.Since(queueStart)
				w.metrics.Gauge(WorkerPoolQueueWaitMs).Set(float64(queueWait.Milliseconds()))

				// Track active workers
				w.metrics.Gauge(WorkerPoolWorkersActive).Set(float64(len(w.sem)))

				// Emit task started event
				_ = w.hooks.Emit(ctx, WorkerPoolEventTaskStarted, WorkerPoolEvent{ //nolint:errcheck
					Name:          w.name,
					ProcessorName: p.Name(),
					WorkerCount:   cap(w.sem),
					ActiveWorkers: len(w.sem),
					QueueWaitTime: queueWait,
					Timestamp:     time.Now(),
				})

				defer func() {
					<-w.sem // Release slot when done
					w.metrics.Gauge(WorkerPoolWorkersActive).Set(float64(len(w.sem)))
				}()
			case <-ctx.Done():
				taskSpan.SetTag(WorkerPoolTagError, "context canceled")
				errors <- ctx.Err()

				// Track as failed task
				completedMu.Lock()
				failedTasks++
				completedMu.Unlock()

				// Emit task complete event for canceled task
				_ = w.hooks.Emit(ctx, WorkerPoolEventTaskComplete, WorkerPoolEvent{ //nolint:errcheck
					Name:          w.name,
					ProcessorName: p.Name(),
					WorkerCount:   cap(w.sem),
					Success:       false,
					Error:         ctx.Err(),
					Timestamp:     time.Now(),
				})
				return
			}

			// Create task context with optional timeout
			if timeout > 0 {
				var cancel context.CancelFunc
				taskCtx, cancel = clock.WithTimeout(taskCtx, timeout)
				defer cancel()
			}

			// Process with cloned input
			inputCopy := input.Clone()
			taskStart := time.Now()
			_, taskErr := p.Process(taskCtx, inputCopy)
			taskDuration := time.Since(taskStart)

			// Update completion stats
			completedMu.Lock()
			if taskErr == nil {
				successfulTasks++
			} else {
				failedTasks++
				taskSpan.SetTag(WorkerPoolTagError, taskErr.Error())
				errors <- taskErr
			}
			completedMu.Unlock()

			// Emit task complete event
			_ = w.hooks.Emit(ctx, WorkerPoolEventTaskComplete, WorkerPoolEvent{ //nolint:errcheck
				Name:          w.name,
				ProcessorName: p.Name(),
				WorkerCount:   cap(w.sem),
				ActiveWorkers: len(w.sem) - 1, // One less after release
				Success:       taskErr == nil,
				Error:         taskErr,
				Duration:      taskDuration,
				Timestamp:     time.Now(),
			})
		}(processor)
	}

	// Wait for all processors to complete
	wg.Wait()
	close(errors)

	// Emit all complete event
	allTasksDuration := time.Since(allTasksStart)
	_ = w.hooks.Emit(ctx, WorkerPoolEventAllComplete, WorkerPoolEvent{ //nolint:errcheck
		Name:            w.name,
		WorkerCount:     cap(w.sem),
		TotalTasks:      len(processors),
		CompletedTasks:  successfulTasks + failedTasks,
		SuccessfulTasks: successfulTasks,
		FailedTasks:     failedTasks,
		TotalDuration:   allTasksDuration,
		Timestamp:       time.Now(),
	})

	// Handle errors (first error wins)
	for err := range errors {
		if err != nil {
			return input, &Error[T]{
				Err:       err,
				InputData: input,
				Path:      []Name{w.name},
				Timestamp: clock.Now(),
				Duration:  0, // Duration not tracked at connector level
			}
		}
	}

	return input, nil
}

// Add appends a processor to the worker pool execution list.
func (w *WorkerPool[T]) Add(processor Chainable[T]) *WorkerPool[T] {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.processors = append(w.processors, processor)
	return w
}

// Remove removes the processor at the specified index.
func (w *WorkerPool[T]) Remove(index int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if index < 0 || index >= len(w.processors) {
		return ErrIndexOutOfBounds
	}

	w.processors = append(w.processors[:index], w.processors[index+1:]...)
	return nil
}

// Len returns the number of processors.
func (w *WorkerPool[T]) Len() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.processors)
}

// Clear removes all processors from the worker pool execution list.
func (w *WorkerPool[T]) Clear() *WorkerPool[T] {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.processors = nil
	return w
}

// SetProcessors replaces all processors atomically.
func (w *WorkerPool[T]) SetProcessors(processors ...Chainable[T]) *WorkerPool[T] {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.processors = make([]Chainable[T], len(processors))
	copy(w.processors, processors)
	return w
}

// WithTimeout sets per-task timeout. Each processor must complete within this duration.
func (w *WorkerPool[T]) WithTimeout(timeout time.Duration) *WorkerPool[T] {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.timeout = timeout
	return w
}

// SetWorkerCount adjusts the worker pool size by recreating the semaphore.
func (w *WorkerPool[T]) SetWorkerCount(workers int) *WorkerPool[T] {
	if workers <= 0 {
		return w // Invalid count, ignore
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Create new semaphore with updated size
	w.sem = make(chan struct{}, workers)
	// Update metric
	w.metrics.Gauge(WorkerPoolWorkersMax).Set(float64(workers))
	return w
}

// GetWorkerCount returns the maximum number of concurrent workers.
func (w *WorkerPool[T]) GetWorkerCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return cap(w.sem)
}

// GetActiveWorkers returns the number of currently active workers.
func (w *WorkerPool[T]) GetActiveWorkers() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.sem)
}

// WithClock sets a custom clock for testing.
func (w *WorkerPool[T]) WithClock(clock clockz.Clock) *WorkerPool[T] {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.clock = clock
	return w
}

// getClock returns the clock to use.
func (w *WorkerPool[T]) getClock() clockz.Clock {
	if w.clock == nil {
		return clockz.RealClock
	}
	return w.clock
}

// Metrics returns the metrics registry for this connector.
func (w *WorkerPool[T]) Metrics() *metricz.Registry {
	return w.metrics
}

// Tracer returns the tracer for this connector.
func (w *WorkerPool[T]) Tracer() *tracez.Tracer {
	return w.tracer
}

// Close gracefully shuts down observability components.
func (w *WorkerPool[T]) Close() error {
	if w.tracer != nil {
		w.tracer.Close()
	}
	w.hooks.Close()
	return nil
}

// OnTaskQueued registers a handler for when a task is queued.
// The handler is called asynchronously when a task is added to the queue,
// before it starts waiting for a worker.
func (w *WorkerPool[T]) OnTaskQueued(handler func(context.Context, WorkerPoolEvent) error) error {
	_, err := w.hooks.Hook(WorkerPoolEventTaskQueued, handler)
	return err
}

// OnTaskStarted registers a handler for when a task starts executing.
// The handler is called asynchronously after a task acquires a worker slot
// and begins execution. The event includes queue wait time information.
func (w *WorkerPool[T]) OnTaskStarted(handler func(context.Context, WorkerPoolEvent) error) error {
	_, err := w.hooks.Hook(WorkerPoolEventTaskStarted, handler)
	return err
}

// OnTaskComplete registers a handler for when a task completes.
// The handler is called asynchronously each time a task finishes, whether it
// succeeds or fails. This provides visibility into individual task outcomes.
func (w *WorkerPool[T]) OnTaskComplete(handler func(context.Context, WorkerPoolEvent) error) error {
	_, err := w.hooks.Hook(WorkerPoolEventTaskComplete, handler)
	return err
}

// OnAllComplete registers a handler for when all tasks have completed.
// The handler is called asynchronously after all tasks finish executing.
// The event includes aggregate statistics about the batch execution.
func (w *WorkerPool[T]) OnAllComplete(handler func(context.Context, WorkerPoolEvent) error) error {
	_, err := w.hooks.Hook(WorkerPoolEventAllComplete, handler)
	return err
}
