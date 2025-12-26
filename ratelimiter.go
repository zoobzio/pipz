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
// RateLimiter wraps a processor and uses a token bucket algorithm to enforce
// rate limits, allowing controlled bursts while maintaining a steady average rate.
// This is essential for protecting external APIs, databases, and other rate-sensitive resources.
//
// CRITICAL: RateLimiter is a STATEFUL connector that maintains an internal token bucket.
// Create it once and reuse it - do NOT create a new RateLimiter for each request,
// as that would reset the token bucket and rate limiting would not work.
//
// ❌ WRONG - Creating per request (useless):
//
//	func handleRequest(req Request) Response {
//	    limiter := pipz.NewRateLimiter(LimiterID, 100, 10, apiCall)  // NEW limiter each time!
//	    return limiter.Process(ctx, req)                             // Always allows through
//	}
//
// ✅ RIGHT - Create once, reuse:
//
//	var apiLimiter = pipz.NewRateLimiter(LimiterID, 100, 10, apiCall)  // Shared instance
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
//   - Create RateLimiters once and reuse them (e.g., as struct fields or package variables)
//   - Configure limits based on actual downstream capacity
//   - Layer multiple limiters for complex scenarios (global → service → endpoint)
//
// Example:
//
//	var (
//	    APILimiterID = pipz.NewIdentity("api-limiter", "Rate limits Stripe API calls")
//	    ChargeID     = pipz.NewIdentity("charge", "Process payment charge")
//	)
//
//	// Create limiter wrapping the API call
//	var stripeLimiter = pipz.NewRateLimiter(APILimiterID, 100, 10,
//	    pipz.Apply(ChargeID, processStripeCharge),
//	)
//
//	// Use in pipeline
//	func createPaymentPipeline() pipz.Chainable[Payment] {
//	    return pipz.NewSequence(PipelineID,
//	        validatePayment,
//	        stripeLimiter,  // Rate-limited API call
//	        confirmPayment,
//	    )
//	}
type RateLimiter[T any] struct {
	processor  Chainable[T]  // wrapped processor
	lastRefill time.Time     // last refill time
	clock      clockz.Clock  // clock interface
	identity   Identity      // identity struct
	mode       string        // "wait" or "drop"
	rate       float64       // tokens per second
	tokens     float64       // current tokens
	mu         sync.Mutex    // mutex
	burst      int           // maximum tokens
	closeOnce  sync.Once     // ensures Close is idempotent
	closeErr   error         // cached close error
}

// NewRateLimiter creates a new RateLimiter connector wrapping the given processor.
// The ratePerSecond parameter sets the sustained rate limit.
// The burst parameter sets the maximum burst size.
func NewRateLimiter[T any](identity Identity, ratePerSecond float64, burst int, processor Chainable[T]) *RateLimiter[T] {
	now := clockz.RealClock.Now()

	return &RateLimiter[T]{
		identity:   identity,
		processor:  processor,
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

	// Calculate time needed for next token
	needed := 1.0 - r.tokens
	if needed <= 0 {
		return 0
	}

	return time.Duration(needed / r.rate * float64(time.Second))
}

// Process implements the Chainable interface.
func (r *RateLimiter[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, r.identity, data)

	for {
		r.mu.Lock()
		mode := r.mode
		if r.canTakeToken() {
			// Emit allowed signal
			capitan.Info(ctx, SignalRateLimiterAllowed,
				FieldName.Field(r.identity.Name()),
				FieldIdentityID.Field(r.identity.ID().String()),
				FieldTokens.Field(r.tokens),
				FieldRate.Field(r.rate),
				FieldBurst.Field(r.burst),
				FieldTimestamp.Field(float64(r.clock.Now().Unix())),
			)

			r.mu.Unlock()
			// Execute the wrapped processor
			result, err = r.processor.Process(ctx, data)
			if err != nil {
				var pipeErr *Error[T]
				if errors.As(err, &pipeErr) {
					pipeErr.Path = append([]Identity{r.identity}, pipeErr.Path...)
					return result, pipeErr
				}
				return result, &Error[T]{
					Timestamp: time.Now(),
					InputData: data,
					Err:       err,
					Path:      []Identity{r.identity},
				}
			}
			return result, nil
		}

		switch mode {
		case modeWait:
			waitTime := r.calculateWaitTime()

			// Emit throttled signal
			capitan.Warn(ctx, SignalRateLimiterThrottled,
				FieldName.Field(r.identity.Name()),
				FieldIdentityID.Field(r.identity.ID().String()),
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
					Path:      []Identity{r.identity},
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
					Path:      []Identity{r.identity},
					Timeout:   errors.Is(ctx.Err(), context.DeadlineExceeded),
					Canceled:  errors.Is(ctx.Err(), context.Canceled),
					Timestamp: r.clock.Now(),
				}
			}

		case modeDrop:
			// Emit dropped signal
			capitan.Error(ctx, SignalRateLimiterDropped,
				FieldName.Field(r.identity.Name()),
				FieldIdentityID.Field(r.identity.ID().String()),
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
				Path:      []Identity{r.identity},
				Timestamp: r.clock.Now(),
			}

		default:
			r.mu.Unlock()
			return data, &Error[T]{
				Err:       fmt.Errorf("invalid rate limiter mode: %s", mode),
				InputData: data,
				Path:      []Identity{r.identity},
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

// Identity returns the identity of this connector.
func (r *RateLimiter[T]) Identity() Identity {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.identity
}

// Schema returns a Node representing this connector in the pipeline schema.
func (r *RateLimiter[T]) Schema() Node {
	r.mu.Lock()
	defer r.mu.Unlock()

	return Node{
		Identity: r.identity,
		Type:     "ratelimiter",
		Flow:     RateLimiterFlow{Processor: r.processor.Schema()},
		Metadata: map[string]any{
			"rate":  r.rate,
			"burst": r.burst,
			"mode":  r.mode,
		},
	}
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

// Close gracefully shuts down the connector and its wrapped processor.
// Close is idempotent - multiple calls return the same result.
func (r *RateLimiter[T]) Close() error {
	r.closeOnce.Do(func() {
		r.closeErr = r.processor.Close()
	})
	return r.closeErr
}
