package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	stateClosed   = "closed"
	stateOpen     = "open"
	stateHalfOpen = "half-open"
)

// CircuitBreaker prevents cascading failures by stopping requests to failing services.
// CircuitBreaker implements the circuit breaker pattern with three states:
//   - Closed: Normal operation, requests pass through
//   - Open: Requests fail immediately without calling the wrapped processor
//   - Half-Open: Testing state, limited requests to check if service recovered
//
// The circuit opens after consecutive failures reach the threshold. After a
// timeout period, it transitions to half-open to test recovery. Successful
// requests in half-open state close the circuit, while failures reopen it.
//
// CircuitBreaker is essential for:
//   - Preventing cascade failures in distributed systems
//   - Giving failing services time to recover
//   - Failing fast when services are down
//   - Reducing unnecessary load on struggling services
//   - Improving overall system resilience
//
// Example:
//
//	breaker := pipz.NewCircuitBreaker(
//	    "api-breaker",
//	    5,                    // Open after 5 consecutive failures
//	    30 * time.Second,     // Try recovery after 30 seconds
//	)
//
//	// Use in a pipeline with retry
//	resilient := pipz.NewSequence("resilient-api",
//	    breaker,
//	    pipz.NewRetry("retry", apiCall, 3),
//	)
type CircuitBreaker[T any] struct {
	lastFailTime     time.Time
	processor        Chainable[T]
	name             Name
	state            string
	resetTimeout     time.Duration
	generation       uint64
	failureThreshold int
	successThreshold int
	failures         int
	successes        int
	mu               sync.Mutex
}

// NewCircuitBreaker creates a new CircuitBreaker connector.
// The failureThreshold sets how many consecutive failures trigger opening.
// The resetTimeout sets how long to wait before attempting recovery.
func NewCircuitBreaker[T any](name Name, processor Chainable[T], failureThreshold int, resetTimeout time.Duration) *CircuitBreaker[T] {
	if failureThreshold < 1 {
		failureThreshold = 1
	}
	return &CircuitBreaker[T]{
		name:             name,
		processor:        processor,
		failureThreshold: failureThreshold,
		successThreshold: 1, // Default: 1 success to close from half-open
		resetTimeout:     resetTimeout,
		state:            stateClosed,
	}
}

// Process implements the Chainable interface.
func (cb *CircuitBreaker[T]) Process(ctx context.Context, data T) (T, error) {
	cb.mu.Lock()

	// Check if we should transition from open to half-open
	if cb.state == stateOpen && time.Since(cb.lastFailTime) > cb.resetTimeout {
		cb.state = stateHalfOpen
		cb.failures = 0
		cb.successes = 0
		cb.generation++
	}

	state := cb.state
	generation := cb.generation
	processor := cb.processor

	// Fail fast if circuit is open
	if state == stateOpen {
		cb.mu.Unlock()
		return data, &Error[T]{
			Err:       fmt.Errorf("circuit breaker is open"),
			InputData: data,
			Path:      []Name{cb.name},
			Timestamp: time.Now(),
		}
	}

	cb.mu.Unlock()

	// Try the operation
	result, err := processor.Process(ctx, data)

	// Record the result
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Only update state if we're still in the same generation
	// This prevents race conditions in half-open state
	if cb.generation != generation {
		return result, err
	}

	if err != nil {
		cb.onFailure()
		// Wrap the error with circuit breaker context
		var pipeErr *Error[T]
		if errors.As(err, &pipeErr) {
			pipeErr.Path = append([]Name{cb.name}, pipeErr.Path...)
			return result, pipeErr
		}
		return result, &Error[T]{
			Err:       err,
			InputData: data,
			Path:      []Name{cb.name},
			Timestamp: time.Now(),
		}
	}

	cb.onSuccess()
	return result, nil
}

// onSuccess handles successful request.
func (cb *CircuitBreaker[T]) onSuccess() {
	switch cb.state {
	case stateClosed:
		// Reset failure count on success
		cb.failures = 0
	case stateHalfOpen:
		cb.successes++
		if cb.successes >= cb.successThreshold {
			// Enough successes, close the circuit
			cb.state = stateClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// onFailure handles failed request.
func (cb *CircuitBreaker[T]) onFailure() {
	cb.lastFailTime = time.Now()

	switch cb.state {
	case stateClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			// Too many failures, open the circuit
			cb.state = stateOpen
		}
	case stateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.state = stateOpen
		cb.failures = 0
		cb.successes = 0
	}
}

// SetFailureThreshold updates the consecutive failures needed to open the circuit.
func (cb *CircuitBreaker[T]) SetFailureThreshold(n int) *CircuitBreaker[T] {
	if n < 1 {
		n = 1
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureThreshold = n
	return cb
}

// SetSuccessThreshold updates the successes needed to close from half-open state.
func (cb *CircuitBreaker[T]) SetSuccessThreshold(n int) *CircuitBreaker[T] {
	if n < 1 {
		n = 1
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.successThreshold = n
	return cb
}

// SetResetTimeout updates the time to wait before attempting recovery.
func (cb *CircuitBreaker[T]) SetResetTimeout(d time.Duration) *CircuitBreaker[T] {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.resetTimeout = d
	return cb
}

// GetState returns the current circuit state.
func (cb *CircuitBreaker[T]) GetState() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check for automatic transition to half-open
	if cb.state == stateOpen && time.Since(cb.lastFailTime) > cb.resetTimeout {
		return stateHalfOpen
	}

	return cb.state
}

// GetFailureThreshold returns the current failure threshold.
func (cb *CircuitBreaker[T]) GetFailureThreshold() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failureThreshold
}

// GetSuccessThreshold returns the current success threshold.
func (cb *CircuitBreaker[T]) GetSuccessThreshold() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.successThreshold
}

// GetResetTimeout returns the current reset timeout.
func (cb *CircuitBreaker[T]) GetResetTimeout() time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.resetTimeout
}

// Reset manually resets the circuit to closed state.
func (cb *CircuitBreaker[T]) Reset() *CircuitBreaker[T] {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = stateClosed
	cb.failures = 0
	cb.successes = 0
	cb.generation++
	return cb
}

// Name returns the name of this connector.
func (cb *CircuitBreaker[T]) Name() Name {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.name
}
