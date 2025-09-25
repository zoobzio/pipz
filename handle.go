package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the Handle connector.
const (
	// Metrics.
	HandleProcessedTotal = metricz.Key("handle.processed.total")
	HandleErrorsTotal    = metricz.Key("handle.errors.total")
	HandleHandlerErrors  = metricz.Key("handle.handler.errors.total")

	// Spans.
	HandleProcessSpan = tracez.Key("handle.process")
	HandleErrorSpan   = tracez.Key("handle.error")

	// Tags.
	HandleTagHasError     = tracez.Tag("handle.has_error")
	HandleTagHandlerError = tracez.Tag("handle.handler_error")

	// Hook event keys.
	HandleEventError        = hookz.Key("handle.error")
	HandleEventHandled      = hookz.Key("handle.handled")
	HandleEventHandlerError = hookz.Key("handle.handler_error")
)

// HandleEvent represents an error handling event.
// This is emitted via hookz when errors occur, are handled, or when the handler itself fails,
// allowing external systems to monitor error patterns and handler effectiveness.
type HandleEvent struct {
	Name          Name          // Connector name
	ProcessorName Name          // Name of the processor that failed
	Error         error         // The original error
	HandlerName   Name          // Name of the error handler
	HandlerError  error         // Error from handler (if any)
	InputData     interface{}   // The input data that caused the error
	Duration      time.Duration // How long the error handling took
	Timestamp     time.Time     // When the event occurred
}

// Handle provides error observation and handling for processors.
// When the wrapped processor fails, Handle passes the error to an error handler
// for processing (e.g., logging, cleanup, notifications), then passes the
// original error through.
//
// Common patterns:
//   - Log errors with additional context
//   - Clean up resources on failure (e.g., release inventory)
//   - Send notifications or alerts
//   - Collect metrics about failures
//   - Implement compensation logic
//
// The error handler receives a Chainable[*Error[T]] with full error context,
// including the input data, error details, and processing path.
//
// Example:
//
//	// Log errors with context
//	logged := pipz.NewHandle(
//	    "with-logging",
//	    processOrder,
//	    pipz.Effect("log", func(ctx context.Context, err *Error[Order]) error {
//	        log.Printf("order %s failed: %v", err.InputData.ID, err.Err)
//	        return nil
//	    }),
//	)
//
//	// Clean up resources on failure
//	withCleanup := pipz.NewHandle(
//	    "inventory-cleanup",
//	    reserveAndCharge,
//	    pipz.Effect("release", func(ctx context.Context, err *Error[Order]) error {
//	        if err.InputData.ReservationID != "" {
//	            inventory.Release(err.InputData.ReservationID)
//	        }
//	        return nil
//	    }),
//	)
//
// # Observability
//
// Handle provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - handle.processed.total: Counter of handle operations
//   - handle.errors.total: Counter of processor errors
//   - handle.handler.errors.total: Counter of error handler failures
//
// Traces:
//   - handle.process: Parent span for handle operation
//   - handle.error: Child span for error handler execution
//
// Events (via hooks):
//   - handle.error: Fired when processor returns an error
//   - handle.handled: Fired when error handler succeeds
//   - handle.handler_error: Fired when error handler itself fails
//
// Example with hooks:
//
//	handle := pipz.NewHandle("order-errors",
//	    orderProcessor,
//	    errorHandler,
//	)
//
//	// Track error patterns
//	handle.OnError(func(ctx context.Context, event HandleEvent) error {
//	    metrics.Inc("order.errors", event.ProcessorName)
//	    log.Error("Order processing failed: %v", event.Error)
//	    return nil
//	})
//
//	// Alert on handler failures
//	handle.OnHandlerError(func(ctx context.Context, event HandleEvent) error {
//	    alert.Critical("Error handler failed: %v", event.HandlerError)
//	    return nil
//	})
type Handle[T any] struct {
	processor    Chainable[T]
	errorHandler Chainable[*Error[T]]
	name         Name
	mu           sync.RWMutex
	metrics      *metricz.Registry
	tracer       *tracez.Tracer
	hooks        *hookz.Hooks[HandleEvent]
}

// NewHandle creates a new Handle connector.
func NewHandle[T any](name Name, processor Chainable[T], errorHandler Chainable[*Error[T]]) *Handle[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(HandleProcessedTotal)
	metrics.Counter(HandleErrorsTotal)
	metrics.Counter(HandleHandlerErrors)

	return &Handle[T]{
		name:         name,
		processor:    processor,
		errorHandler: errorHandler,
		metrics:      metrics,
		tracer:       tracez.New(),
		hooks:        hookz.New[HandleEvent](),
	}
}

// Process implements the Chainable interface.
func (h *Handle[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, h.name, input)

	// Track metrics
	h.metrics.Counter(HandleProcessedTotal).Inc()

	// Start span
	ctx, span := h.tracer.StartSpan(ctx, HandleProcessSpan)
	defer func() {
		if err != nil {
			span.SetTag(HandleTagHasError, "true")
		} else {
			span.SetTag(HandleTagHasError, "false")
		}
		span.Finish()
	}()

	// Take a snapshot of processor and errorHandler to prevent race conditions
	h.mu.RLock()
	processor := h.processor
	errorHandler := h.errorHandler
	h.mu.RUnlock()

	// Use the snapshots instead of accessing fields directly
	result, err = processor.Process(ctx, input)
	if err != nil {
		h.metrics.Counter(HandleErrorsTotal).Inc()

		// Emit error event
		_ = h.hooks.Emit(ctx, HandleEventError, HandleEvent{ //nolint:errcheck
			Name:          h.name,
			ProcessorName: processor.Name(),
			Error:         err,
			HandlerName:   errorHandler.Name(),
			InputData:     input,
			Timestamp:     time.Now(),
		})

		var pipeErr *Error[T]
		if errors.As(err, &pipeErr) {
			// Prepend this handle's name to the path
			pipeErr.Path = append([]Name{h.name}, pipeErr.Path...)
			// Process the error through the error handler
			errorCtx, errorSpan := h.tracer.StartSpan(ctx, HandleErrorSpan)
			handlerStart := time.Now()
			_, handlerErr := errorHandler.Process(errorCtx, pipeErr) //nolint:errcheck // Handler errors are recorded but don't affect flow
			handlerDuration := time.Since(handlerStart)
			if handlerErr != nil {
				h.metrics.Counter(HandleHandlerErrors).Inc()
				errorSpan.SetTag(HandleTagHandlerError, handlerErr.Error())

				// Emit handler error event
				_ = h.hooks.Emit(ctx, HandleEventHandlerError, HandleEvent{ //nolint:errcheck
					Name:          h.name,
					ProcessorName: processor.Name(),
					Error:         err,
					HandlerName:   errorHandler.Name(),
					HandlerError:  handlerErr,
					InputData:     input,
					Duration:      handlerDuration,
					Timestamp:     time.Now(),
				})
			} else {
				// Emit handled event
				_ = h.hooks.Emit(ctx, HandleEventHandled, HandleEvent{ //nolint:errcheck
					Name:          h.name,
					ProcessorName: processor.Name(),
					Error:         err,
					HandlerName:   errorHandler.Name(),
					InputData:     input,
					Duration:      handlerDuration,
					Timestamp:     time.Now(),
				})
			}
			errorSpan.Finish()
			// Always pass through the original error
			return result, err
		}
		// Handle non-pipeline errors by wrapping them
		// Use the processor snapshot name to avoid race condition
		processorName := processor.Name()
		wrappedErr := &Error[T]{
			Timestamp: time.Now(),
			InputData: input,
			Err:       err,
			Path:      []Name{h.name, processorName},
		}
		errorCtx, errorSpan := h.tracer.StartSpan(ctx, HandleErrorSpan)
		handlerStart := time.Now()
		_, handlerErr := errorHandler.Process(errorCtx, wrappedErr) //nolint:errcheck // Handler errors are recorded but don't affect flow
		handlerDuration := time.Since(handlerStart)
		if handlerErr != nil {
			h.metrics.Counter(HandleHandlerErrors).Inc()
			errorSpan.SetTag(HandleTagHandlerError, handlerErr.Error())

			// Emit handler error event
			_ = h.hooks.Emit(ctx, HandleEventHandlerError, HandleEvent{ //nolint:errcheck
				Name:          h.name,
				ProcessorName: processorName,
				Error:         err,
				HandlerName:   errorHandler.Name(),
				HandlerError:  handlerErr,
				InputData:     input,
				Duration:      handlerDuration,
				Timestamp:     time.Now(),
			})
		} else {
			// Emit handled event
			_ = h.hooks.Emit(ctx, HandleEventHandled, HandleEvent{ //nolint:errcheck
				Name:          h.name,
				ProcessorName: processorName,
				Error:         err,
				HandlerName:   errorHandler.Name(),
				InputData:     input,
				Duration:      handlerDuration,
				Timestamp:     time.Now(),
			})
		}
		errorSpan.Finish()
		// Always pass through the original error
		return result, err
	}
	return result, nil
}

// SetProcessor updates the main processor.
func (h *Handle[T]) SetProcessor(processor Chainable[T]) *Handle[T] {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.processor = processor
	return h
}

// SetErrorHandler updates the error handler.
func (h *Handle[T]) SetErrorHandler(handler Chainable[*Error[T]]) *Handle[T] {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errorHandler = handler
	return h
}

// Name returns the name of this connector.
func (h *Handle[T]) Name() Name {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.name
}

// GetProcessor returns the current main processor.
func (h *Handle[T]) GetProcessor() Chainable[T] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.processor
}

// GetErrorHandler returns the current error handler.
func (h *Handle[T]) GetErrorHandler() Chainable[*Error[T]] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.errorHandler
}

// Metrics returns the metrics registry for this connector.
func (h *Handle[T]) Metrics() *metricz.Registry {
	return h.metrics
}

// Tracer returns the tracer for this connector.
func (h *Handle[T]) Tracer() *tracez.Tracer {
	return h.tracer
}

// Close gracefully shuts down observability components.
func (h *Handle[T]) Close() error {
	if h.tracer != nil {
		h.tracer.Close()
	}
	h.hooks.Close()
	return nil
}

// OnError registers a handler for when an error occurs.
// The handler is called asynchronously when the processor returns an error,
// before the error handler processes it.
func (h *Handle[T]) OnError(handler func(context.Context, HandleEvent) error) error {
	_, err := h.hooks.Hook(HandleEventError, handler)
	return err
}

// OnHandled registers a handler for when an error is successfully handled.
// The handler is called asynchronously after the error handler processes the error
// without returning an error itself.
func (h *Handle[T]) OnHandled(handler func(context.Context, HandleEvent) error) error {
	_, err := h.hooks.Hook(HandleEventHandled, handler)
	return err
}

// OnHandlerError registers a handler for when the error handler itself fails.
// The handler is called asynchronously when the error handler returns an error
// while processing the original error.
func (h *Handle[T]) OnHandlerError(handler func(context.Context, HandleEvent) error) error {
	_, err := h.hooks.Hook(HandleEventHandlerError, handler)
	return err
}
