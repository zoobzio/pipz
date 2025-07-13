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
// The library is built around three main types:
//
//   - Processor: A function that transforms data of type T, returning (T, error)
//   - Contract: A single processor that can be executed or composed with others
//   - Chain: A sequence of processors executed in order
//
// All processors follow a uniform signature, enabling seamless composition while maintaining
// type safety through Go generics. Execution follows a fail-fast pattern where processing
// stops at the first error.
//
// # Adapter Functions
//
// pipz provides several adapter functions to wrap common patterns:
//
//   - Transform: Pure data transformations that cannot fail
//   - Validate: Data validation without modification
//   - Apply: Operations that might fail (parsing, network calls)
//   - Mutate: Conditional transformations based on data state
//   - Effect: Side effects like logging or metrics
//   - Enrich: Best-effort data enhancement
//
// # Usage Example
//
// Here's a simple example of building a user registration pipeline:
//
//	type User struct {
//	    Email    string
//	    Password string
//	    ID       string
//	}
//
//	// Create individual processors
//	validateEmail := pipz.Validate(func(u User) error {
//	    if !strings.Contains(u.Email, "@") {
//	        return errors.New("invalid email")
//	    }
//	    return nil
//	})
//
//	hashPassword := pipz.Transform(func(u User) User {
//	    u.Password = hash(u.Password)
//	    return u
//	})
//
//	generateID := pipz.Transform(func(u User) User {
//	    u.ID = uuid.New().String()
//	    return u
//	})
//
//	// Compose into a pipeline
//	pipeline := validateEmail.
//	    Then(hashPassword).
//	    Then(generateID)
//
//	// Execute the pipeline
//	user := User{Email: "user@example.com", Password: "secret"}
//	result, err := pipeline.Process(user)
//
// # Benefits
//
// Using pipz provides several advantages:
//
//   - Testability: Each processor can be tested in isolation
//   - Reusability: Common processors can be shared across pipelines
//   - Clarity: Business logic is clearly expressed as a sequence of steps
//   - Type Safety: Compile-time type checking prevents runtime errors
//   - Performance: Minimal overhead with zero allocations in hot paths
//
// # Pipeline Patterns
//
// pipz supports several common patterns:
//
//   - Sequential Processing: Chain operations that depend on previous results
//   - Conditional Execution: Use Mutate for conditional transformations
//   - Error Recovery: Implement fallback logic in Apply functions
//   - Parallel Composition: Build and compose independent pipelines
//   - Side Effect Management: Use Effect for logging, metrics, or external calls
//
// # Performance
//
// The library is designed for minimal overhead:
//
//   - ~20-30ns per processor in the pipeline
//   - Zero allocations for processor execution
//   - No reflection, locks, or global state
//   - Predictable performance characteristics
//
// # Best Practices
//
// When using pipz:
//
//  1. Keep processors small and focused on a single responsibility
//  2. Use the appropriate adapter for your use case
//  3. Compose pipelines from reusable processors
//  4. Test processors independently before composing
//  5. Handle errors at the pipeline level, not within processors
//  6. Use Effect for side effects to maintain processor purity
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
// For more examples and use cases, see the examples directory and USE_CASES.md.
package pipz
