package pipz

import "github.com/zoobzio/capitan"

// Signal definitions for pipz connector events.
// Signals follow the pattern: <connector-type>.<event>.
var (
	// CircuitBreaker signals.
	SignalCircuitBreakerOpened = capitan.NewSignal(
		"circuitbreaker.opened",
		"Circuit breaker has transitioned to open state due to exceeding failure threshold",
	)
	SignalCircuitBreakerClosed = capitan.NewSignal(
		"circuitbreaker.closed",
		"Circuit breaker has transitioned to closed state after successful recovery",
	)
	SignalCircuitBreakerHalfOpen = capitan.NewSignal(
		"circuitbreaker.half-open",
		"Circuit breaker has transitioned to half-open state to test if the issue has resolved",
	)
	SignalCircuitBreakerRejected = capitan.NewSignal(
		"circuitbreaker.rejected",
		"Circuit breaker rejected a request because it is in open state",
	)

	// RateLimiter signals.
	SignalRateLimiterThrottled = capitan.NewSignal(
		"ratelimiter.throttled",
		"Rate limiter delayed a request to comply with rate limits",
	)
	SignalRateLimiterDropped = capitan.NewSignal(
		"ratelimiter.dropped",
		"Rate limiter rejected a request because rate limit was exceeded and drop mode is enabled",
	)
	SignalRateLimiterAllowed = capitan.NewSignal(
		"ratelimiter.allowed",
		"Rate limiter allowed a request to proceed",
	)

	// WorkerPool signals.
	SignalWorkerPoolSaturated = capitan.NewSignal(
		"workerpool.saturated",
		"Worker pool has reached maximum capacity and is waiting for available workers",
	)
	SignalWorkerPoolAcquired = capitan.NewSignal(
		"workerpool.acquired",
		"Worker pool acquired a worker slot for processing",
	)
	SignalWorkerPoolReleased = capitan.NewSignal(
		"workerpool.released",
		"Worker pool released a worker slot after processing completed",
	)

	// Retry signals.
	SignalRetryAttemptStart = capitan.NewSignal(
		"retry.attempt-start",
		"Retry connector is starting an execution attempt",
	)
	SignalRetryAttemptFail = capitan.NewSignal(
		"retry.attempt-fail",
		"Retry connector attempt failed and will be retried if attempts remain",
	)
	SignalRetryExhausted = capitan.NewSignal(
		"retry.exhausted",
		"Retry connector has exhausted all retry attempts and is failing",
	)

	// Fallback signals.
	SignalFallbackAttempt = capitan.NewSignal(
		"fallback.attempt",
		"Fallback connector is attempting to execute a processor in the fallback chain",
	)
	SignalFallbackFailed = capitan.NewSignal(
		"fallback.failed",
		"Fallback connector exhausted all processors without success",
	)

	// Timeout signals.
	SignalTimeoutTriggered = capitan.NewSignal(
		"timeout.triggered",
		"Timeout connector canceled execution because the deadline was exceeded",
	)

	// Backoff signals.
	SignalBackoffWaiting = capitan.NewSignal(
		"backoff.waiting",
		"Backoff connector is delaying before the next execution attempt",
	)
)

// Common field keys using capitan primitive types.
// All keys use primitive types to avoid custom struct serialization.
var (
	// Common fields.
	FieldName      = capitan.NewStringKey("name")       // Connector instance name
	FieldError     = capitan.NewStringKey("error")      // Error message
	FieldTimestamp = capitan.NewFloat64Key("timestamp") // Unix timestamp

	// CircuitBreaker fields.
	FieldState            = capitan.NewStringKey("state")           // Circuit state: closed/open/half-open
	FieldFailures         = capitan.NewIntKey("failures")           // Current failure count
	FieldSuccesses        = capitan.NewIntKey("successes")          // Current success count
	FieldFailureThreshold = capitan.NewIntKey("failure_threshold")  // Threshold to open
	FieldSuccessThreshold = capitan.NewIntKey("success_threshold")  // Threshold to close from half-open
	FieldResetTimeout     = capitan.NewFloat64Key("reset_timeout")  // Reset timeout in seconds
	FieldGeneration       = capitan.NewIntKey("generation")         // Circuit generation number
	FieldLastFailTime     = capitan.NewFloat64Key("last_fail_time") // Last failure timestamp

	// RateLimiter fields.
	FieldRate     = capitan.NewFloat64Key("rate")      // Requests per second
	FieldBurst    = capitan.NewIntKey("burst")         // Burst capacity
	FieldTokens   = capitan.NewFloat64Key("tokens")    // Current tokens
	FieldMode     = capitan.NewStringKey("mode")       // Mode: wait/drop
	FieldWaitTime = capitan.NewFloat64Key("wait_time") // Wait time in seconds

	// WorkerPool fields.
	FieldWorkerCount   = capitan.NewIntKey("worker_count")   // Total worker slots
	FieldActiveWorkers = capitan.NewIntKey("active_workers") // Currently active workers

	// Retry fields.
	FieldAttempt     = capitan.NewIntKey("attempt")      // Current attempt number
	FieldMaxAttempts = capitan.NewIntKey("max_attempts") // Maximum attempts

	// Fallback fields.
	FieldProcessorIndex = capitan.NewIntKey("processor_index")   // Index of processor being tried
	FieldProcessorName  = capitan.NewStringKey("processor_name") // Name of processor being tried

	// Timeout fields.
	FieldDuration = capitan.NewFloat64Key("duration") // Timeout duration in seconds

	// Backoff fields.
	FieldDelay     = capitan.NewFloat64Key("delay")      // Current backoff delay in seconds
	FieldNextDelay = capitan.NewFloat64Key("next_delay") // Next delay if this attempt fails in seconds
)
