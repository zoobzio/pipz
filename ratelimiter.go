package pipz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"github.com/zoobzio/clockz"
)

// Mode constants.
const (
	modeWait = "wait"
	modeDrop = "drop"
)

// RateLimiter controls the rate of processing to protect downstream services.
// RateLimiter uses a token bucket algorithm to enforce rate limits, allowing
// controlled bursts while maintaining a steady average rate. This is essential
// for protecting external APIs, databases, and other rate-sensitive resources.
//
// CRITICAL: RateLimiter is a STATEFUL connector that maintains an internal token bucket.
// You MUST create it as a package-level variable (singleton) to share state across requests.
// Creating a new RateLimiter for each request defeats the purpose entirely!
//
// ❌ WRONG - Creating per request (useless):
//
//	func handleRequest(req Request) Response {
//	    limiter := pipz.NewRateLimiter("api", 100, 10)  // NEW limiter each time!
//	    return limiter.Process(ctx, req)                // Always allows through
//	}
//
// ✅ RIGHT - Package-level singleton:
//
//	var apiLimiter = pipz.NewRateLimiter("api", 100, 10)  // Shared instance
//
//	func handleRequest(req Request) Response {
//	    return apiLimiter.Process(ctx, req)  // Actually rate limits
//	}
//
// The limiter operates in two modes:
//   - "wait": Blocks until a token is available (default)
//   - "drop": Returns an error immediately if no tokens available
//
// RateLimiter is particularly useful for:
//   - API client implementations with rate limits
//   - Database connection throttling
//   - Preventing overwhelming downstream services
//   - Implementing fair resource sharing
//   - Meeting SLA requirements
//
// Best Practices:
//   - Use const names for all processors/connectors (see best-practices.md)
//   - Declare RateLimiters as package-level vars
//   - Configure limits based on actual downstream capacity
//   - Layer multiple limiters for complex scenarios (global → service → endpoint)
//
// Example:
//
//	// Define names as constants
//	const (
//	    ConnectorAPILimiter    = "api-limiter"
//	    ConnectorGlobalLimiter = "global-limiter"
//	)
//
//	// Create limiters as package-level singletons
//	var (
//	    // Global rate limit for entire system
//	    globalLimiter = pipz.NewRateLimiter(ConnectorGlobalLimiter, 10000, 1000)
//
//	    // Service-specific limit (e.g., Stripe API)
//	    apiLimiter = pipz.NewRateLimiter(ConnectorAPILimiter, 100, 10)
//	)
//
//	// Use in pipeline
//	func createPaymentPipeline() pipz.Chainable[Payment] {
//	    return pipz.NewSequence("payment-pipeline",
//	        globalLimiter,                           // System-wide limit
//	        apiLimiter,                              // Service-specific limit
//	        pipz.Apply("charge", processPayment),   // Actual operation
//	    )
//	}
type RateLimiter[T any] struct {
	lastRefill time.Time    // last refill time (24 bytes)
	clock      clockz.Clock // interface (16 bytes)
	name       Name         // string (16 bytes)
	mode       string       // "wait" or "drop" (16 bytes)
	rate       float64      // tokens per second (8 bytes)
	tokens     float64      // current tokens (8 bytes)
	mu         sync.Mutex   // mutex (8 bytes)
	burst      int          // maximum tokens (8 bytes)
}

// NewRateLimiter creates a new RateLimiter connector.
// The ratePerSecond parameter sets the sustained rate limit.
// The burst parameter sets the maximum burst size.
func NewRateLimiter[T any](name Name, ratePerSecond float64, burst int) *RateLimiter[T] {
	now := clockz.RealClock.Now()

	return &RateLimiter[T]{
		name:       name,
		rate:       ratePerSecond,
		burst:      burst,
		tokens:     float64(burst), // Start with full bucket
		lastRefill: now,
		mode:       modeWait, // Default to wait mode
		clock:      clockz.RealClock,
	}
}

// refillTokens updates the token bucket based on elapsed time since last refill.
// Formula: tokens = min(burst, tokens + elapsed_seconds * rate)
// Must be called with mutex held.
func (r *RateLimiter[T]) refillTokens() {
	now := r.clock.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.lastRefill = now

	// Handle infinite rate - bypass limits entirely
	if math.IsInf(r.rate, 1) {
		r.tokens = float64(r.burst)
		return
	}

	// Refill tokens based on elapsed time
	r.tokens = math.Min(float64(r.burst), r.tokens+elapsed*r.rate)
}

// canTakeToken checks if a token is available and takes it if so.
// Returns true if token was taken, false otherwise.
// Must be called with mutex held.
func (r *RateLimiter[T]) canTakeToken() bool {
	r.refillTokens()
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}
	return false
}

// calculateWaitTime returns the duration to wait for the next token.
// Formula: waitTime = (1 - currentTokens) / rate * time.Second
// Must be called with mutex held after refillTokens().
func (r *RateLimiter[T]) calculateWaitTime() time.Duration {
	// Handle zero rate - block forever
	if r.rate == 0 {
		return time.Duration(math.MaxInt64)
	}

	// Handle infinite rate - no wait
	if math.IsInf(r.rate, 1) {
		return 0
	}

	// Calculate time needed for next token
	needed := 1.0 - r.tokens
	if needed <= 0 {
		return 0
	}

	return time.Duration(needed / r.rate * float64(time.Second))
}

// Process implements the Chainable interface.
func (r *RateLimiter[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, r.name, data)

	for {
		r.mu.Lock()
		mode := r.mode
		if r.canTakeToken() {
			// Emit allowed signal
			capitan.Info(ctx, SignalRateLimiterAllowed,
				FieldName.Field(string(r.name)),
				FieldTokens.Field(r.tokens),
				FieldRate.Field(r.rate),
				FieldBurst.Field(r.burst),
				FieldTimestamp.Field(float64(r.clock.Now().Unix())),
			)

			r.mu.Unlock()
			return data, nil
		}

		switch mode {
		case modeWait:
			waitTime := r.calculateWaitTime()

			// Emit throttled signal
			capitan.Warn(ctx, SignalRateLimiterThrottled,
				FieldName.Field(string(r.name)),
				FieldWaitTime.Field(waitTime.Seconds()),
				FieldTokens.Field(r.tokens),
				FieldRate.Field(r.rate),
				FieldTimestamp.Field(float64(r.clock.Now().Unix())),
			)

			r.mu.Unlock() // Unlock before blocking

			// Handle zero rate (infinite wait)
			if waitTime == time.Duration(math.MaxInt64) {
				<-ctx.Done()
				return data, &Error[T]{
					Err:       ctx.Err(),
					InputData: data,
					Path:      []Name{r.name},
					Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
					Canceled:  errors.Is(ctx.Err(), context.Canceled),
					Timestamp: r.clock.Now(),
				}
			}

			// Wait for tokens or context cancellation
			select {
			case <-r.clock.After(waitTime):
				// Continue loop to check for tokens again
			case <-ctx.Done():
				return data, &Error[T]{
					Err:       ctx.Err(),
					InputData: data,
					Path:      []Name{r.name},
					Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
					Canceled:  errors.Is(ctx.Err(), context.Canceled),
					Timestamp: r.clock.Now(),
				}
			}

		case modeDrop:
			// Emit dropped signal
			capitan.Error(ctx, SignalRateLimiterDropped,
				FieldName.Field(string(r.name)),
				FieldTokens.Field(r.tokens),
				FieldRate.Field(r.rate),
				FieldBurst.Field(r.burst),
				FieldMode.Field(mode),
				FieldTimestamp.Field(float64(r.clock.Now().Unix())),
			)

			r.mu.Unlock()
			return data, &Error[T]{
				Err:       fmt.Errorf("rate limit exceeded"),
				InputData: data,
				Path:      []Name{r.name},
				Timestamp: r.clock.Now(),
			}

		default:
			r.mu.Unlock()
			return data, &Error[T]{
				Err:       fmt.Errorf("invalid rate limiter mode: %s", mode),
				InputData: data,
				Path:      []Name{r.name},
				Timestamp: r.clock.Now(),
			}
		}
	}
}

// SetRate updates the rate limit (requests per second).
func (r *RateLimiter[T]) SetRate(ratePerSecond float64) *RateLimiter[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Refill tokens before changing rate to maintain accuracy
	r.refillTokens()
	r.rate = ratePerSecond
	return r
}

// SetBurst updates the burst capacity.
func (r *RateLimiter[T]) SetBurst(burst int) *RateLimiter[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Refill tokens before changing burst to maintain accuracy
	r.refillTokens()
	r.burst = burst
	// Cap current tokens to new burst limit
	if r.tokens > float64(burst) {
		r.tokens = float64(burst)
	}
	return r
}

// SetMode sets the rate limiting mode ("wait" or "drop").
func (r *RateLimiter[T]) SetMode(mode string) *RateLimiter[T] {
	if mode != modeWait && mode != modeDrop {
		// Invalid mode, ignore
		return r
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mode = mode
	return r
}

// GetRate returns the current rate limit.
func (r *RateLimiter[T]) GetRate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rate
}

// GetBurst returns the current burst capacity.
func (r *RateLimiter[T]) GetBurst() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.burst
}

// GetMode returns the current mode ("wait" or "drop").
func (r *RateLimiter[T]) GetMode() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mode
}

// Name returns the name of this connector.
func (r *RateLimiter[T]) Name() Name {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name
}

// WithClock sets the clock implementation for testing purposes.
// This method is primarily intended for testing with FakeClock.
func (r *RateLimiter[T]) WithClock(clock clockz.Clock) *RateLimiter[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clock = clock
	r.lastRefill = clock.Now()
	return r
}

// GetAvailableTokens returns the current number of available tokens.
// This method is primarily intended for testing and debugging.
func (r *RateLimiter[T]) GetAvailableTokens() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refillTokens()
	return r.tokens
}

// Close gracefully shuts down the connector.
func (*RateLimiter[T]) Close() error {
	return nil
}
