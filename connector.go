package pipz

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Chainable defines the interface for any component that can process
// values of type T. This interface enables composition of different
// processing components that operate on the same type.
//
// Chainable is the foundation of pipz - every processor, pipeline,
// and connector implements this interface. The uniform interface
// enables seamless composition while maintaining type safety through
// Go generics.
//
// Key design principles:
//   - Single method interface for maximum flexibility
//   - Context support for timeout and cancellation
//   - Type safety through generics (no interface{})
//   - Error propagation for fail-fast behavior
//   - Immutable by convention (return modified copies)
//
// Any function matching func(context.Context, T) (T, error) can
// become Chainable through ProcessorFunc or adapter functions.
type Chainable[T any] interface {
	Process(context.Context, T) (T, error)
}

// ProcessorFunc is a function adapter that implements Chainable.
// It allows any function with the signature func(context.Context, T) (T, error)
// to be used directly as a Chainable without creating a wrapper type.
//
// This adapter is useful for:
//   - Quick inline processors in tests
//   - Simple transformations that don't need naming
//   - Wrapping existing functions as Chainables
//   - Creating anonymous processors in connectors
type ProcessorFunc[T any] func(context.Context, T) (T, error)

// Process implements the Chainable interface.
func (f ProcessorFunc[T]) Process(ctx context.Context, data T) (T, error) {
	return f(ctx, data)
}

// Condition determines routing based on input data.
// Returns a route key of any comparable type for multi-way branching.
//
// Using generic keys instead of strings enables type-safe routing
// beyond simple string matching. Define custom types for your routes:
//
//	type PaymentRoute string
//	const (
//	    RouteStandard   PaymentRoute = "standard"
//	    RouteHighValue  PaymentRoute = "high_value"
//	    RouteCrypto     PaymentRoute = "crypto"
//	    RouteDefault    PaymentRoute = "default"
//	)
//
// Common patterns include routing by:
//   - Typed enums for business states
//   - Integer codes for priority levels
//   - Custom types for domain concepts
type Condition[T any, K comparable] func(context.Context, T) K

// Sequential runs chainables in order, passing output to input.
// Sequential is the most fundamental connector - it executes each step
// in order, with each step receiving the output of the previous step.
// If any step fails, processing stops immediately and returns the error.
//
// Sequential is ideal for:
//   - Multi-step data transformations
//   - Workflows with dependent steps
//   - Validation followed by processing
//   - Any linear sequence of operations
//
// The fail-fast behavior ensures data integrity - partial processing
// is prevented when any step encounters an error.
//
// Example:
//
//	processOrder := pipz.Sequential(
//	    validateOrder,      // First: validate the order
//	    calculateTax,       // Then: add tax
//	    applyDiscount,      // Then: apply any discounts
//	    chargePayment,      // Finally: charge the customer
//	)
func Sequential[T any](chainables ...Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		var err error
		for _, chainable := range chainables {
			data, err = chainable.Process(ctx, data)
			if err != nil {
				return data, err
			}
		}
		return data, nil
	})
}

// Switch routes to different chainables based on condition result.
// Switch enables conditional processing where the path taken depends
// on the input data. The condition function examines the data and
// returns a route key that determines which processor to use.
//
// The key type K must be comparable (can be used as map key). This enables
// type-safe routing with custom types, avoiding magic strings. If no route
// exists for the returned key, an error is returned.
//
// IMPORTANT: The routes map is captured by reference, not copied.
// This means you can modify the map after creating the Switch to dynamically
// add, remove, or update routes. This is an intentional feature that enables
// runtime reconfiguration without rebuilding the Switch.
//
// Thread Safety: The routes map is NOT synchronized. If you modify routes
// concurrently with Process calls, you MUST handle synchronization yourself
// using sync.Map, a mutex, or other synchronization primitives. Concurrent
// map access without synchronization will cause a panic.
//
// Switch is perfect for:
//   - Type-based processing with enum safety
//   - Status-based workflows with defined states
//   - Region-specific logic with typed regions
//   - Priority handling with numeric levels
//   - A/B testing with experiment types
//   - Dynamic routing tables that change at runtime
//   - Feature flag controlled processing paths
//
// Example with type-safe keys:
//
//	type PaymentRoute string
//	const (
//	    RouteStandard   PaymentRoute = "standard"
//	    RouteHighValue  PaymentRoute = "high_value"
//	    RouteCrypto     PaymentRoute = "crypto"
//	)
//
//	routes := map[PaymentRoute]pipz.Chainable[Payment]{
//	    RouteStandard:   standardProcessor,
//	    RouteHighValue:  highValueProcessor,
//	    RouteCrypto:     cryptoProcessor,
//	}
//
//	processPayment := pipz.Switch(
//	    func(ctx context.Context, p Payment) PaymentRoute {
//	        if p.Amount > 10000 {
//	            return RouteHighValue
//	        } else if p.Method == "crypto" {
//	            return RouteCrypto
//	        }
//	        return RouteStandard  // Explicit default
//	    },
//	    routes,
//	)
//
//	// Later: Add new route without rebuilding
//	const RouteExpress PaymentRoute = "express"
//	routes[RouteExpress] = expressProcessor
func Switch[T any, K comparable](condition Condition[T, K], routes map[K]Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		route := condition(ctx, data)
		chainable, exists := routes[route]
		if !exists {
			return data, fmt.Errorf("no route for condition result: %v", route)
		}
		return chainable.Process(ctx, data)
	})
}

// Fallback attempts the primary chainable, falling back to secondary on error.
// Fallback provides automatic failover to an alternative processor when
// the primary fails. This creates resilient pipelines that can recover
// from failures gracefully.
//
// Unlike Retry which attempts the same operation multiple times,
// Fallback switches to a completely different implementation. This is
// valuable when you have multiple ways to accomplish the same goal.
//
// Common use cases:
//   - Primary/backup service failover
//   - Graceful degradation strategies
//   - Multiple payment provider support
//   - Cache miss handling (try cache, then database)
//   - API version compatibility
//
// Fallback can be nested for multiple alternatives:
//
//	Fallback(primary, Fallback(secondary, tertiary))
//
// Example:
//
//	processPayment := pipz.Fallback(
//	    stripeProcessor,       // Try Stripe first
//	    paypalProcessor,       // Fall back to PayPal on error
//	)
func Fallback[T any](primary, fallback Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		result, err := primary.Process(ctx, data)
		if err != nil {
			return fallback.Process(ctx, data)
		}
		return result, nil
	})
}

// Retry attempts the chainable up to maxAttempts times.
// Retry provides simple retry logic for operations that may fail
// transiently. It immediately retries on failure without delay,
// making it suitable for quick operations or when failures are
// expected to clear immediately.
//
// Each retry uses the same input data. Context cancellation is
// checked between attempts to allow for early termination.
// If all attempts fail, the last error is returned with attempt
// count information for debugging.
//
// Use Retry for:
//   - Network calls with transient failures
//   - Database operations during brief contentions
//   - File operations with temporary locks
//   - Any operation with intermittent failures
//
// For operations needing delay between retries, use RetryWithBackoff.
// For trying different approaches, use Fallback instead.
//
// Example:
//
//	saveToDatabase := pipz.Retry(
//	    databaseWriter,
//	    3,  // Try up to 3 times
//	)
func Retry[T any](chainable Chainable[T], maxAttempts int) Chainable[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		var lastErr error
		for i := 0; i < maxAttempts; i++ {
			result, err := chainable.Process(ctx, data)
			if err == nil {
				return result, nil
			}
			lastErr = err
			// Check if context is canceled between attempts
			if ctx.Err() != nil {
				return data, ctx.Err()
			}
		}
		return data, fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
	})
}

// RetryWithBackoff attempts the chainable with exponential backoff between attempts.
// RetryWithBackoff adds intelligent spacing between retry attempts, starting with
// baseDelay and doubling after each failure. This prevents overwhelming failed
// services and allows time for transient issues to resolve.
//
// The exponential backoff pattern (delay, 2*delay, 4*delay, ...) is widely
// used for its effectiveness in handling various failure scenarios without
// overwhelming systems. The operation can be canceled via context during waits.
//
// Ideal for:
//   - API calls to rate-limited services
//   - Database operations during high load
//   - Distributed system interactions
//   - Any operation where immediate retry is counterproductive
//
// The total time spent can be significant with multiple retries.
// For example, with baseDelay=1s and maxAttempts=5:
//
//	Delays: 1s, 2s, 4s, 8s (total wait: 15s plus processing time)
//
// Example:
//
//	callExternalAPI := pipz.RetryWithBackoff(
//	    apiProcessor,
//	    5,                    // Max 5 attempts
//	    100*time.Millisecond, // Start with 100ms delay
//	)
func RetryWithBackoff[T any](chainable Chainable[T], maxAttempts int, baseDelay time.Duration) Chainable[T] {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		var lastErr error
		delay := baseDelay

		for i := 0; i < maxAttempts; i++ {
			result, err := chainable.Process(ctx, data)
			if err == nil {
				return result, nil
			}
			lastErr = err

			// Don't sleep after the last attempt
			if i < maxAttempts-1 {
				select {
				case <-time.After(delay):
					delay *= 2 // Exponential backoff
				case <-ctx.Done():
					return data, ctx.Err()
				}
			}
		}
		return data, fmt.Errorf("failed after %d attempts with backoff: %w", maxAttempts, lastErr)
	})
}

// Timeout enforces a timeout on the chainable's execution.
// Timeout wraps any chainable with a hard time limit, ensuring operations
// complete within acceptable bounds. If the timeout expires, the operation
// is canceled via context and a timeout error is returned.
//
// This connector is critical for:
//   - Preventing hung operations
//   - Meeting SLA requirements
//   - Protecting against slow external services
//   - Ensuring predictable system behavior
//   - Resource management in concurrent systems
//
// The wrapped operation should respect context cancellation for
// immediate termination. Operations that ignore context may continue
// running in the background even after timeout.
//
// Timeout is often combined with Retry for robust error handling:
//
//	Retry(Timeout(operation, 5*time.Second), 3)
//
// Example:
//
//	fetchUserData := pipz.Timeout(
//	    userServiceCall,
//	    2*time.Second,  // Must complete within 2 seconds
//	)
func Timeout[T any](chainable Chainable[T], duration time.Duration) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, data T) (T, error) {
		ctx, cancel := context.WithTimeout(ctx, duration)
		defer cancel()

		done := make(chan struct{})
		var result T
		var err error

		go func() {
			result, err = chainable.Process(ctx, data)
			close(done)
		}()

		select {
		case <-done:
			return result, err
		case <-ctx.Done():
			return data, fmt.Errorf("timeout after %v: %w", duration, ctx.Err())
		}
	})
}

// Cloner is an interface for types that can create deep copies of themselves.
// Implementing this interface allows types to be used with ConcurrentClone,
// providing a type-safe and performant alternative to reflection-based copying.
//
// The Clone method should return a deep copy where modifications to the clone
// do not affect the original value. For types containing pointers, slices, or maps,
// ensure these are also copied to achieve true isolation.
//
// Example implementation:
//
//	type Order struct {
//	    ID     string
//	    Items  []Item
//	    Status string
//	}
//
//	func (o Order) Clone() Order {
//	    items := make([]Item, len(o.Items))
//	    copy(items, o.Items)
//	    return Order{
//	        ID:     o.ID,
//	        Items:  items,
//	        Status: o.Status,
//	    }
//	}
type Cloner[T any] interface {
	Clone() T
}

// Concurrent runs all processors in parallel, each with an isolated copy of the input.
// Concurrent enables parallel execution of independent operations that don't need
// to coordinate or share results. Each processor receives a deep copy of the input,
// ensuring complete isolation. The original input is always returned unchanged.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// This pattern is powerful for "fire and forget" operations where you need multiple
// side effects to happen simultaneously:
//   - Sending notifications to multiple channels
//   - Updating multiple external systems
//   - Parallel logging to different destinations
//   - Triggering independent workflows
//   - Warming multiple caches
//
// Important characteristics:
//   - Input type must implement Cloner[T] interface
//   - All processors run regardless of individual failures
//   - Original input always returned (processors can't modify it)
//   - Context cancellation stops all processors
//   - No result aggregation (use custom logic if needed)
//
// Example:
//
//	type Order struct {
//	    ID     string
//	    Items  []Item
//	    Status string
//	}
//
//	func (o Order) Clone() Order {
//	    items := make([]Item, len(o.Items))
//	    copy(items, o.Items)
//	    return Order{
//	        ID:     o.ID,
//	        Items:  items,
//	        Status: o.Status,
//	    }
//	}
//
//	notifyOrder := pipz.Concurrent(
//	    sendEmailNotification,
//	    sendSMSNotification,
//	    updateInventorySystem,
//	    logToAnalytics,
//	)
func Concurrent[T Cloner[T]](processors ...Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, input T) (T, error) {
		if len(processors) == 0 {
			return input, nil
		}

		var wg sync.WaitGroup
		wg.Add(len(processors))

		// Create a cancellable context for all goroutines
		concCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		for _, processor := range processors {
			go func(p Chainable[T]) {
				defer wg.Done()

				// Create an isolated copy using the Clone method
				inputCopy := input.Clone()

				// Process with the copy, ignoring any returns
				if _, err := p.Process(concCtx, inputCopy); err != nil {
					// Log or handle error if needed in the future
					_ = err
				}
			}(processor)
		}

		// Wait for completion or context cancellation
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return input, nil
		case <-ctx.Done():
			return input, ctx.Err()
		}
	})
}

// WithErrorHandler wraps a processor with error handling.
// WithErrorHandler allows you to add error handling logic to any processor
// without changing its behavior. When the wrapped processor fails, the error
// is passed to the errorHandler for processing (logging, metrics, alerts),
// but the original error is still returned.
//
// This separation of concerns keeps error handling logic out of business logic
// while ensuring errors are properly observed. It's especially valuable with
// Concurrent where individual failures shouldn't stop other processors.
//
// Common error handling patterns:
//   - Logging errors with context
//   - Incrementing error metrics
//   - Sending alerts for critical failures
//   - Recording errors for retry analysis
//   - Triggering compensating actions
//
// The error handler itself is a Chainable[error], allowing complex
// error processing pipelines with the same tools used for data.
//
// Example:
//
//	processWithLogging := pipz.WithErrorHandler(
//	    riskyProcessor,
//	    pipz.Effect("log_error", func(ctx context.Context, err error) error {
//	        log.Printf("processor failed: %v", err)
//	        metrics.Increment("processor.errors")
//	        return nil  // Error handler errors are ignored
//	    }),
//	)
func WithErrorHandler[T any](
	processor Chainable[T],
	errorHandler Chainable[error],
) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, input T) (T, error) {
		result, err := processor.Process(ctx, input)
		if err != nil {
			// Process the error through the error handler
			// We ignore the error handler's return values and any errors it produces
			// nolint:errcheck
			_, _ = errorHandler.Process(ctx, err)
		}
		return result, err
	})
}

// Race runs all processors in parallel and returns the result of the first
// to complete successfully. Race implements competitive processing where speed
// matters more than which specific processor succeeds. The first successful
// result wins and cancels all other processors.
//
// The input type T must implement the Cloner[T] interface to provide efficient,
// type-safe copying without reflection. This ensures predictable performance and
// allows types to control their own copying semantics.
//
// This pattern excels when you have multiple ways to get the same result
// and want the fastest one:
//   - Querying multiple replicas or regions
//   - Trying different algorithms with varying performance
//   - Fetching from multiple caches
//   - Calling primary and backup services simultaneously
//   - Any scenario where latency matters more than specific source
//
// Key behaviors:
//   - First success wins and cancels others
//   - All failures returns the last error
//   - Each processor gets an isolated copy via Clone()
//   - Useful for reducing p99 latencies
//   - Can increase load (all processors run)
//
// Example:
//
//	// UserQuery must implement Cloner[UserQuery]
//	fetchUserData := pipz.Race(
//	    fetchFromLocalCache,
//	    fetchFromRegionalCache,
//	    fetchFromDatabase,
//	)
func Race[T Cloner[T]](processors ...Chainable[T]) Chainable[T] {
	return ProcessorFunc[T](func(ctx context.Context, input T) (T, error) {
		if len(processors) == 0 {
			return input, fmt.Errorf("no processors provided to Race")
		}

		// Create channels for results and errors
		type result struct {
			data T
			err  error
			idx  int
		}

		resultCh := make(chan result, len(processors))
		raceCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Launch all processors
		for i, processor := range processors {
			go func(idx int, p Chainable[T]) {
				// Create an isolated copy using the Clone method
				inputCopy := input.Clone()

				data, err := p.Process(raceCtx, inputCopy)
				select {
				case resultCh <- result{data: data, err: err, idx: idx}:
				case <-raceCtx.Done():
				}
			}(i, processor)
		}

		// Collect results
		var lastErr error
		for i := 0; i < len(processors); i++ {
			select {
			case res := <-resultCh:
				if res.err == nil {
					// First success wins
					cancel() // Cancel other goroutines
					return res.data, nil
				}
				lastErr = res.err
			case <-ctx.Done():
				return input, ctx.Err()
			}
		}

		// All failed
		return input, fmt.Errorf("all processors failed, last error: %w", lastErr)
	})
}
