package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"github.com/zoobzio/clockz"
)

// State constants.
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
// CRITICAL: CircuitBreaker is a STATEFUL connector that tracks failure counts across requests.
// Create it once and reuse it - do NOT create a new CircuitBreaker for each request,
// as that would reset the failure count and the circuit would never open.
//
// ❌ WRONG - Creating per request (never opens):
//
//	func handleRequest(req Request) Response {
//	    breaker := pipz.NewCircuitBreaker("api", proc, 5, 30*time.Second)  // NEW breaker!
//	    return breaker.Process(ctx, req)  // Always closed, failure count always 0
//	}
//
// ✅ RIGHT - Create once, reuse:
//
//	var apiBreaker = pipz.NewCircuitBreaker("api", apiProcessor, 5, 30*time.Second)
//
//	func handleRequest(req Request) Response {
//	    return apiBreaker.Process(ctx, req)  // Tracks failures across requests
//	}
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
// Best Practices:
//   - Use const names for all processors/connectors (see best-practices.md)
//   - Create CircuitBreakers once and reuse them (e.g., as struct fields or package variables)
//   - Set thresholds based on service characteristics
//   - Combine with RateLimiter for comprehensive protection
//   - Monitor circuit state for operational awareness
//
// Example:
//
//	// Define names as constants
//	const (
//	    ConnectorAPIBreaker      = "api-breaker"
//	    ConnectorDatabaseBreaker = "db-breaker"
//	    ProcessorAPICall         = "api-call"
//	)
//
//	// Create breakers once and reuse
//	var (
//	    // External API - fail fast, longer recovery
//	    apiBreaker = pipz.NewCircuitBreaker(
//	        ConnectorAPIBreaker,
//	        pipz.Apply(ProcessorAPICall, callExternalAPI),
//	        5,                    // Open after 5 failures
//	        30 * time.Second,     // Try recovery after 30s
//	    )
//
//	    // Internal database - more tolerant
//	    dbBreaker = pipz.NewCircuitBreaker(
//	        ConnectorDatabaseBreaker,
//	        pipz.Apply("db-query", queryDatabase),
//	        10,                   // Open after 10 failures
//	        10 * time.Second,     // Try recovery after 10s
//	    )
//	)
//
//	// Combine with rate limiting for full protection
//	func createResilientPipeline() pipz.Chainable[Request] {
//	    return pipz.NewSequence("resilient-pipeline",
//	        rateLimiter,    // Protect downstream from overload
//	        apiBreaker,     // Fail fast if service is down
//	        pipz.NewRetry("retry", processor, 3),  // Retry transient failures
//	    )
//	}
type CircuitBreaker[T any] struct {
	lastFailTime     time.Time
	processor        Chainable[T]
	clock            clockz.Clock
	name             Name
	state            string
	mu               sync.Mutex
	resetTimeout     time.Duration
	generation       int
	failureThreshold int
	successThreshold int
	failures         int
	successes        int
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
func (cb *CircuitBreaker[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, cb.name, data)

	cb.mu.Lock()

	// Check if we should transition from open to half-open
	clock := cb.getClock()
	if cb.state == stateOpen && clock.Since(cb.lastFailTime) > cb.resetTimeout {
		cb.state = stateHalfOpen
		cb.failures = 0
		cb.successes = 0
		cb.generation++

		// Emit half-open signal
		capitan.Warn(ctx, SignalCircuitBreakerHalfOpen,
			FieldName.Field(string(cb.name)),
			FieldState.Field(cb.state),
			FieldGeneration.Field(cb.generation),
			FieldTimestamp.Field(float64(clock.Now().Unix())),
		)
	}

	state := cb.state
	generation := cb.generation
	processor := cb.processor

	// Fail fast if circuit is open
	if state == stateOpen {
		// Emit rejected signal
		capitan.Error(ctx, SignalCircuitBreakerRejected,
			FieldName.Field(string(cb.name)),
			FieldState.Field(state),
			FieldGeneration.Field(generation),
			FieldTimestamp.Field(float64(cb.getClock().Now().Unix())),
		)

		cb.mu.Unlock()
		return data, &Error[T]{
			Err:       fmt.Errorf("circuit breaker is open"),
			InputData: data,
			Path:      []Name{cb.name},
			Timestamp: cb.getClock().Now(),
		}
	}

	cb.mu.Unlock()

	// Try the operation
	result, err = processor.Process(ctx, data)

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
			Timestamp: cb.getClock().Now(),
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

			// Emit closed signal
			capitan.Info(context.Background(), SignalCircuitBreakerClosed,
				FieldName.Field(string(cb.name)),
				FieldState.Field(cb.state),
				FieldSuccesses.Field(cb.successes),
				FieldSuccessThreshold.Field(cb.successThreshold),
				FieldTimestamp.Field(float64(cb.getClock().Now().Unix())),
			)
		}
	}
}

// onFailure handles failed request.
func (cb *CircuitBreaker[T]) onFailure() {
	cb.lastFailTime = cb.getClock().Now()

	switch cb.state {
	case stateClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			// Too many failures, open the circuit
			cb.state = stateOpen

			// Emit opened signal
			capitan.Error(context.Background(), SignalCircuitBreakerOpened,
				FieldName.Field(string(cb.name)),
				FieldState.Field(cb.state),
				FieldFailures.Field(cb.failures),
				FieldFailureThreshold.Field(cb.failureThreshold),
				FieldTimestamp.Field(float64(cb.getClock().Now().Unix())),
			)
		}
	case stateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.state = stateOpen
		cb.failures = 0
		cb.successes = 0

		// Emit opened signal (reopened from half-open)
		capitan.Emit(context.Background(), SignalCircuitBreakerOpened,
			FieldName.Field(string(cb.name)),
			FieldState.Field(cb.state),
			FieldFailures.Field(cb.failures),
			FieldFailureThreshold.Field(cb.failureThreshold),
			FieldTimestamp.Field(float64(cb.getClock().Now().Unix())),
		)
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
	if cb.state == stateOpen && cb.getClock().Since(cb.lastFailTime) > cb.resetTimeout {
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

// WithClock sets a custom clock for testing.
func (cb *CircuitBreaker[T]) WithClock(clock clockz.Clock) *CircuitBreaker[T] {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.clock = clock
	return cb
}

// getClock returns the clock to use.
func (cb *CircuitBreaker[T]) getClock() clockz.Clock {
	if cb.clock == nil {
		return clockz.RealClock
	}
	return cb.clock
}

// Name returns the name of this connector.
func (cb *CircuitBreaker[T]) Name() Name {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.name
}

// Close gracefully shuts down the connector.
func (*CircuitBreaker[T]) Close() error {
	return nil
}
