package pipz

import (
	"context"
	"errors"
	"sync"
	"time"
)

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
type Handle[T any] struct {
	processor    Chainable[T]
	errorHandler Chainable[*Error[T]]
	name         Name
	mu           sync.RWMutex
}

// NewHandle creates a new Handle connector.
func NewHandle[T any](name Name, processor Chainable[T], errorHandler Chainable[*Error[T]]) *Handle[T] {
	return &Handle[T]{
		name:         name,
		processor:    processor,
		errorHandler: errorHandler,
	}
}

// Process implements the Chainable interface.
func (h *Handle[T]) Process(ctx context.Context, input T) (result T, err error) {
	defer recoverFromPanic(&result, &err, h.name, input)

	h.mu.RLock()
	processor := h.processor
	errorHandler := h.errorHandler
	h.mu.RUnlock()

	result, err = processor.Process(ctx, input)
	if err != nil {
		var pipeErr *Error[T]
		if errors.As(err, &pipeErr) {
			// Prepend this handle's name to the path
			pipeErr.Path = append([]Name{h.name}, pipeErr.Path...)
			// Process the error through the error handler
			_, _ = errorHandler.Process(ctx, pipeErr) //nolint:errcheck // Handler errors are intentionally ignored
			// Always pass through the original error
			return result, err
		}
		// Handle non-pipeline errors by wrapping them
		wrappedErr := &Error[T]{
			Timestamp: time.Now(),
			InputData: input,
			Err:       err,
			Path:      []Name{h.name, processor.Name()},
		}
		_, _ = errorHandler.Process(ctx, wrappedErr) //nolint:errcheck // Handler errors are intentionally ignored
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
