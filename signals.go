package pipz

import "github.com/zoobzio/capitan"

// Signal constants for pipz connector events.
// Signals follow the pattern: <connector-type>.<event>.
const (
	// CircuitBreaker signals.
	SignalCircuitBreakerOpened   capitan.Signal = "circuitbreaker.opened"
	SignalCircuitBreakerClosed   capitan.Signal = "circuitbreaker.closed"
	SignalCircuitBreakerHalfOpen capitan.Signal = "circuitbreaker.half-open"
	SignalCircuitBreakerRejected capitan.Signal = "circuitbreaker.rejected"

	// RateLimiter signals.
	SignalRateLimiterThrottled capitan.Signal = "ratelimiter.throttled"
	SignalRateLimiterDropped   capitan.Signal = "ratelimiter.dropped"
	SignalRateLimiterAllowed   capitan.Signal = "ratelimiter.allowed"

	// WorkerPool signals.
	SignalWorkerPoolSaturated capitan.Signal = "workerpool.saturated"
	SignalWorkerPoolAcquired  capitan.Signal = "workerpool.acquired"
	SignalWorkerPoolReleased  capitan.Signal = "workerpool.released"

	// Retry signals.
	SignalRetryAttemptStart capitan.Signal = "retry.attempt-start"
	SignalRetryAttemptFail  capitan.Signal = "retry.attempt-fail"
	SignalRetryExhausted    capitan.Signal = "retry.exhausted"

	// Fallback signals.
	SignalFallbackAttempt capitan.Signal = "fallback.attempt"
	SignalFallbackFailed  capitan.Signal = "fallback.failed"

	// Timeout signals.
	SignalTimeoutTriggered capitan.Signal = "timeout.triggered"

	// Backoff signals.
	SignalBackoffWaiting capitan.Signal = "backoff.waiting"
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
