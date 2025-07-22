// Package pipz provides a lightweight, type-safe library for building composable data processing pipelines in Go.
//
// # Overview
//
// pipz enables developers to create clean, testable, and maintainable data processing workflows
// by composing small, focused functions into larger pipelines. It addresses common challenges
// in Go applications such as scattered business logic, repetitive error handling, and
// difficult-to-test code that mixes pure logic with external dependencies.
//
// # Core Concepts
//
// The library is built around a simple, uniform interface:
//
//   - Chainable[T]: The core interface with Process(context.Context, T) (T, error)
//   - Processors: Functions wrapped as Chainables using adapter functions
//   - Connectors: Functions that compose multiple Chainables into complex flows
//
// Everything implements the Chainable interface, enabling seamless composition while maintaining
// type safety through Go generics. Context support allows for timeout control and cancellation.
// Execution follows a fail-fast pattern where processing stops at the first error.
//
// # Adapter Functions
//
// pipz provides several adapter functions to wrap common patterns:
//
//   - Transform: Pure transformations that cannot fail
//   - Apply: Operations that transform data and might fail (parsing, API calls)
//   - Effect: Side effects like logging or metrics (doesn't modify data)
//   - Mutate: Conditional modifications based on predicates
//   - Enrich: Best-effort enhancements that don't fail the pipeline
//
// # Connectors
//
// Connectors compose Chainables into complex processing flows:
//
//   - Sequential: Process steps in order, stopping on first error
//   - Switch: Route to different processors based on data
//   - Concurrent: Run processors in parallel with isolated data (requires Cloner interface)
//   - Race: Return first successful result from parallel processors
//   - Fallback: Try alternatives if the primary fails
//   - Retry: Retry operations with configurable attempts
//   - RetryWithBackoff: Retry with exponential backoff
//   - Timeout: Enforce time limits on operations
//   - WithErrorHandler: Add error observation without changing behavior
//
// # Usage Example
//
// Here's a simple example of building a user registration pipeline:
//
//	import (
//	    "context"
//	    "errors"
//	    "strings"
//	    "time"
//	)
//
//	type User struct {
//	    Email    string
//	    Password string
//	    ID       string
//	}
//
//	// Create individual processors
//	validateEmail := pipz.Effect("validate_email", func(_ context.Context, u User) error {
//	    if !strings.Contains(u.Email, "@") {
//	        return errors.New("invalid email")
//	    }
//	    return nil
//	})
//
//	hashPassword := pipz.Apply("hash_password", func(_ context.Context, u User) (User, error) {
//	    u.Password = hash(u.Password)
//	    return u, nil
//	})
//
//	generateID := pipz.Apply("generate_id", func(_ context.Context, u User) (User, error) {
//	    u.ID = uuid.New().String()
//	    return u, nil
//	})
//
//	// Compose into a pipeline
//	pipeline := pipz.Sequential(
//	    validateEmail,
//	    hashPassword,
//	    generateID,
//	)
//
//	// Execute with timeout context
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	user := User{Email: "user@example.com", Password: "secret"}
//	result, err := pipeline.Process(ctx, user)
//
// # Benefits
//
// Using pipz provides several advantages:
//
//   - Testability: Each processor can be tested in isolation
//   - Reusability: Common processors can be shared across pipelines
//   - Clarity: Business logic is clearly expressed as a sequence of steps
//   - Type Safety: Compile-time type checking prevents runtime errors
//   - Timeout Control: Context support enables reliable timeout handling
//   - Cancellation: Processors can be canceled mid-execution for security
//   - Performance: Minimal overhead with predictable execution patterns
//
// # Common Patterns
//
// pipz supports several powerful composition patterns:
//
//   - Sequential Processing: Chain operations that depend on previous results
//   - Conditional Routing: Use Switch to route based on data attributes
//   - Error Recovery: Use Fallback for alternative processing paths
//   - Resilience: Add Retry and Timeout for unreliable operations
//   - Side Effect Management: Use Effect for logging, metrics, or external calls
//
// # Performance
//
// The library is designed for minimal overhead:
//
//   - Minimal per-processor overhead
//   - Context passing adds negligible cost
//   - No reflection or runtime type assertions
//   - Predictable performance characteristics
//   - Zero allocations in core operations
//
// # Best Practices
//
// When using pipz:
//
//  1. Keep processors small and focused on a single responsibility
//  2. Use descriptive names for processors to aid debugging
//  3. Check context.Err() in long-running processors for cancellation
//  4. Use appropriate timeouts with the Timeout connector
//  5. Use the appropriate adapter for your use case (Apply, Validate, Effect)
//  6. Compose pipelines from reusable processors using connectors
//  7. Test processors independently before composing
//  8. Handle errors at the pipeline level, not within processors
//  9. Use Effect for side effects to maintain processor purity
//
// # Integration
//
// pipz integrates well with existing Go code:
//
//   - Works with any data type through generics
//   - Compatible with standard Go error handling
//   - No external dependencies required
//   - Can wrap existing functions with adapters
//   - Supports gradual adoption in existing codebases
//
// For more examples and documentation, see the examples directory and docs directory.
package pipz
