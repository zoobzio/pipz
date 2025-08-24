package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

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
	name    Name
	limiter *rate.Limiter
	mode    string // "wait" or "drop"
	mu      sync.RWMutex
}

// NewRateLimiter creates a new RateLimiter connector.
// The ratePerSecond parameter sets the sustained rate limit.
// The burst parameter sets the maximum burst size.
func NewRateLimiter[T any](name Name, ratePerSecond float64, burst int) *RateLimiter[T] {
	return &RateLimiter[T]{
		name:    name,
		limiter: rate.NewLimiter(rate.Limit(ratePerSecond), burst),
		mode:    modeWait, // Default to wait mode
	}
}

// Process implements the Chainable interface.
func (r *RateLimiter[T]) Process(ctx context.Context, data T) (T, error) {
	r.mu.RLock()
	limiter := r.limiter
	mode := r.mode
	r.mu.RUnlock()

	switch mode {
	case modeWait:
		// Wait for permission
		err := limiter.Wait(ctx)
		if err != nil {
			// Context was canceled while waiting
			return data, &Error[T]{
				Err:       err,
				InputData: data,
				Path:      []Name{r.name},
				Timeout:   errors.Is(err, context.DeadlineExceeded),
				Canceled:  errors.Is(err, context.Canceled),
				Timestamp: time.Now(),
			}
		}
		return data, nil

	case modeDrop:
		// Try to get permission without waiting
		if !limiter.Allow() {
			return data, &Error[T]{
				Err:       fmt.Errorf("rate limit exceeded"),
				InputData: data,
				Path:      []Name{r.name},
				Timestamp: time.Now(),
			}
		}
		return data, nil

	default:
		// Should not happen, but handle gracefully
		return data, &Error[T]{
			Err:       fmt.Errorf("invalid rate limiter mode: %s", mode),
			InputData: data,
			Path:      []Name{r.name},
			Timestamp: time.Now(),
		}
	}
}

// SetRate updates the rate limit (requests per second).
func (r *RateLimiter[T]) SetRate(ratePerSecond float64) *RateLimiter[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiter.SetLimit(rate.Limit(ratePerSecond))
	return r
}

// SetBurst updates the burst capacity.
func (r *RateLimiter[T]) SetBurst(burst int) *RateLimiter[T] {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiter.SetBurst(burst)
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	return float64(r.limiter.Limit())
}

// GetBurst returns the current burst capacity.
func (r *RateLimiter[T]) GetBurst() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.limiter.Burst()
}

// GetMode returns the current mode ("wait" or "drop").
func (r *RateLimiter[T]) GetMode() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mode
}

// Name returns the name of this connector.
func (r *RateLimiter[T]) Name() Name {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.name
}
