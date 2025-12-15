package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"github.com/zoobzio/clockz"
)

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
//nolint:govet // fieldalignment: 8-byte difference in struct size is negligible vs. readability
type WorkerPool[T Cloner[T]] struct {
	processors []Chainable[T]
	sem        chan struct{} // Semaphore for worker limit
	name       Name
	mu         sync.RWMutex  // Thread safety
	timeout    time.Duration // Optional per-task timeout
	queueSize  int           // Optional queue size (unused but kept for future)
	clock      clockz.Clock  // Clock for time operations
	closeOnce  sync.Once
	closeErr   error
}

// NewWorkerPool creates a WorkerPool with specified worker count.
// Workers parameter controls maximum concurrent processors (semaphore slots).
func NewWorkerPool[T Cloner[T]](name Name, workers int, processors ...Chainable[T]) *WorkerPool[T] {
	if workers <= 0 {
		workers = 1 // Sensible default
	}

	wp := &WorkerPool[T]{
		processors: make([]Chainable[T], len(processors)),
		sem:        make(chan struct{}, workers),
		name:       name,
		timeout:    0, // Default no timeout
		queueSize:  0, // Default no buffering
		clock:      clockz.RealClock,
	}
	copy(wp.processors, processors) // Defensive copy

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

	if len(processors) == 0 {
		return input, nil
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(processors))

	for _, processor := range processors {
		wg.Add(1)

		go func(p Chainable[T]) {
			defer wg.Done()

			// Check if pool is saturated before acquiring
			workerCount := cap(w.sem)
			activeWorkers := len(w.sem)
			if activeWorkers >= workerCount {
				// Emit saturated signal
				capitan.Warn(ctx, SignalWorkerPoolSaturated,
					FieldName.Field(string(w.name)),
					FieldWorkerCount.Field(workerCount),
					FieldActiveWorkers.Field(activeWorkers),
					FieldTimestamp.Field(float64(clock.Now().Unix())),
				)
			}

			// Acquire semaphore slot (blocks if all workers busy)
			select {
			case w.sem <- struct{}{}:
				// Emit acquired signal
				capitan.Info(ctx, SignalWorkerPoolAcquired,
					FieldName.Field(string(w.name)),
					FieldWorkerCount.Field(workerCount),
					FieldActiveWorkers.Field(len(w.sem)),
					FieldTimestamp.Field(float64(clock.Now().Unix())),
				)

				defer func() {
					<-w.sem // Release slot when done

					// Emit released signal
					capitan.Info(ctx, SignalWorkerPoolReleased,
						FieldName.Field(string(w.name)),
						FieldWorkerCount.Field(workerCount),
						FieldActiveWorkers.Field(len(w.sem)),
						FieldTimestamp.Field(float64(clock.Now().Unix())),
					)
				}()
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}

			// Create task context with optional timeout
			taskCtx := ctx
			if timeout > 0 {
				var cancel context.CancelFunc
				taskCtx, cancel = clock.WithTimeout(taskCtx, timeout)
				defer cancel()
			}

			// Process with cloned input
			inputCopy := input.Clone()
			_, taskErr := p.Process(taskCtx, inputCopy)

			if taskErr != nil {
				errors <- taskErr
			}
		}(processor)
	}

	// Wait for all processors to complete
	wg.Wait()
	close(errors)

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

// Close gracefully shuts down the worker pool and all its child processors.
// Close is idempotent - multiple calls return the same result.
func (w *WorkerPool[T]) Close() error {
	w.closeOnce.Do(func() {
		w.mu.RLock()
		defer w.mu.RUnlock()

		var errs []error
		for i := len(w.processors) - 1; i >= 0; i-- {
			if err := w.processors[i].Close(); err != nil {
				errs = append(errs, err)
			}
		}
		w.closeErr = errors.Join(errs...)
	})
	return w.closeErr
}
