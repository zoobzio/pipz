// Package pipz provides a lightweight, type-safe library for building composable data processing pipelines in Go.
//
// # Overview
//
// pipz enables developers to create clean, testable, and maintainable data processing workflows
// by composing small, focused functions into larger pipelines. It addresses common challenges
// in Go applications such as scattered business logic, repetitive error handling, and
// difficult-to-test code that mixes pure logic with external dependencies.
//
// # Installation
//
//	go get github.com/zoobzio/pipz
//
// Requires Go 1.21+ for generic type constraints.
//
// # Core Concepts
//
// The library is built around a single, uniform interface:
//
//	type Chainable[T any] interface {
//	    Process(context.Context, T) (T, error)
//	    Identity() Identity
//	    Schema() Node
//	    Close() error
//	}
//
// Key components:
//   - Processors: Individual processing steps created with adapter functions (Transform, Apply, etc.)
//   - Connectors: Compose multiple processors into complex flows (Sequence, Switch, Concurrent, etc.)
//   - Sequence: The primary way to build sequential pipelines with runtime modification support
//
// Design philosophy:
//   - Processors are immutable values (simple functions wrapped with metadata)
//   - Connectors are mutable pointers (configurable containers with state)
//
// Everything implements Chainable[T], enabling seamless composition while maintaining
// type safety through Go generics. Context support provides timeout control and cancellation.
// Execution follows a fail-fast pattern where processing stops at the first error.
//
// # Adapter Functions
//
// Adapters wrap your functions to implement the Chainable interface:
//
// Transform - Pure transformations that cannot fail:
//
//	var DoubleName = pipz.NewIdentity("double", "")
//	double := pipz.Transform(DoubleName, func(_ context.Context, n int) int {
//	    return n * 2
//	})
//
// Apply - Operations that can fail:
//
//	var ParseJSONName = pipz.NewIdentity("parse", "")
//	parseJSON := pipz.Apply(ParseJSONName, func(_ context.Context, s string) (Data, error) {
//	    var d Data
//	    return d, json.Unmarshal([]byte(s), &d)
//	})
//
// Effect - Side effects without modifying data:
//
//	var LoggerName = pipz.NewIdentity("log", "")
//	logger := pipz.Effect(LoggerName, func(_ context.Context, d Data) error {
//	    log.Printf("Processing: %+v", d)
//	    return nil
//	})
//
// Mutate - Conditional modifications:
//
//	var DiscountName = pipz.NewIdentity("discount", "")
//	discountPremium := pipz.Mutate(DiscountName,
//	    func(_ context.Context, u User) User { u.Discount = 0.2; return u },
//	    func(_ context.Context, u User) bool { return u.IsPremium },
//	)
//
// Enrich - Optional enhancements that log failures:
//
//	var GeoEnrichName = pipz.NewIdentity("geo", "")
//	addLocation := pipz.Enrich(GeoEnrichName, func(ctx context.Context, u User) (User, error) {
//	    u.Country = detectCountry(u.IP) // May fail, but won't stop pipeline
//	    return u, nil
//	})
//
// # Connectors
//
// Connectors compose multiple Chainables. Choose based on your needs:
//
// Sequential Processing:
//
//	var PipelineName = pipz.NewIdentity("pipeline", "")
//	pipeline := pipz.NewSequence(PipelineName, step1, step2, step3)
//	// Or build dynamically:
//	var DynamicName = pipz.NewIdentity("dynamic", "")
//	seq := pipz.NewSequence[T](DynamicName)
//	seq.Register(step1, step2)
//	seq.PushTail(step3)  // Add at runtime
//
// Parallel Processing (requires T implements Cloner[T]):
//
//	// Run all processors, return original data
//	var ParallelName = pipz.NewIdentity("parallel", "")
//	concurrent := pipz.NewConcurrent(ParallelName, proc1, proc2, proc3)
//
//	// Return first successful result
//	var FastestName = pipz.NewIdentity("fastest", "")
//	race := pipz.NewRace(FastestName, primary, secondary, tertiary)
//
//	// Return first result meeting a condition
//	var BestName = pipz.NewIdentity("best", "")
//	contest := pipz.NewContest(BestName, conditionFunc, option1, option2, option3)
//
// Error Handling:
//
//	// Try fallback on error
//	var SafeName = pipz.NewIdentity("safe", "")
//	fallback := pipz.NewFallback(SafeName, primary, backup)
//
//	// Retry with attempts
//	var ResilientName = pipz.NewIdentity("resilient", "")
//	retry := pipz.NewRetry(ResilientName, processor, 3)
//
//	// Retry with exponential backoff
//	var ApiCallName = pipz.NewIdentity("api-call", "")
//	backoff := pipz.NewBackoff(ApiCallName, processor, 5, time.Second)
//
//	// Handle errors without changing data flow
//	var ObservedName = pipz.NewIdentity("observed", "")
//	handle := pipz.NewHandle(ObservedName, processor, errorPipeline)
//
// Control Flow:
//
//	// Route based on conditions
//	var RouterName = pipz.NewIdentity("router", "")
//	router := pipz.NewSwitch(RouterName, func(ctx context.Context, d Data) string {
//	    if d.Type == "premium" { return "premium-flow" }
//	    return "standard-flow"
//	})
//	router.AddRoute("premium-flow", premiumProcessor)
//	router.AddRoute("standard-flow", standardProcessor)
//
//	// Enforce timeouts
//	var DeadlineName = pipz.NewIdentity("deadline", "")
//	timeout := pipz.NewTimeout(DeadlineName, processor, 30*time.Second)
//
//	// Conditional processing
//	var FeatureFlagName = pipz.NewIdentity("feature-flag", "")
//	filter := pipz.NewFilter(FeatureFlagName,
//	    func(ctx context.Context, u User) bool { return u.BetaEnabled },
//	    betaProcessor,
//	)
//
// Resource Protection:
//
//	// Rate limiting
//	var ApiLimitName = pipz.NewIdentity("api-limit", "")
//	rateLimiter := pipz.NewRateLimiter(ApiLimitName, 100, 10) // 100/sec, burst 10
//	rateLimiter.SetMode("drop") // Or "wait" (default)
//
//	// Circuit breaker
//	var ServiceBreakerName = pipz.NewIdentity("service-breaker", "")
//	breaker := pipz.NewCircuitBreaker(ServiceBreakerName, processor, 5, 30*time.Second)
//
// # Quick Start
//
// Simple example - transform strings through a pipeline:
//
//	package main
//
//	import (
//	    "context"
//	    "strings"
//	    "github.com/zoobzio/pipz"
//	)
//
//	func main() {
//	    // Define processor names as constants
//	    const (
//	        TrimName = pipz.NewIdentity("trim", "")
//	        UpperName = pipz.NewIdentity("uppercase", "")
//	        TextProcessorName = pipz.NewIdentity("text-processor", "")
//	    )
//
//	    // Create processors
//	    trim := pipz.Transform(TrimName, func(_ context.Context, s string) string {
//	        return strings.TrimSpace(s)
//	    })
//	    upper := pipz.Transform(UpperName, func(_ context.Context, s string) string {
//	        return strings.ToUpper(s)
//	    })
//
//	    // Method 1: Direct composition
//	    pipeline := pipz.NewSequence(TextProcessorName, trim, upper)
//
//	    // Method 2: Build dynamically
//	    sequence := pipz.NewSequence[string](TextProcessorName)
//	    sequence.Register(trim, upper)
//
//	    // Execute
//	    result, err := pipeline.Process(context.Background(), "  hello world  ")
//	    // result: "HELLO WORLD", err: nil
//	}
//
// # Implementing Cloner[T]
//
// For parallel processing with Concurrent or Race, types must implement Cloner[T]:
//
//	type Order struct {
//	    ID    string
//	    Items []Item        // Slice needs copying
//	    Meta  map[string]any // Map needs copying
//	}
//
//	func (o Order) Clone() Order {
//	    // Deep copy slice
//	    items := make([]Item, len(o.Items))
//	    for i, item := range o.Items {
//	        items[i] = item.Clone() // If Item also has references
//	    }
//
//	    // Deep copy map
//	    meta := make(map[string]any, len(o.Meta))
//	    for k, v := range o.Meta {
//	        meta[k] = v // Adjust based on value types
//	    }
//
//	    return Order{ID: o.ID, Items: items, Meta: meta}
//	}
//
// # Choosing the Right Connector
//
//   - NewSequence: Default choice for step-by-step processing
//   - Sequence: When you need to modify pipeline at runtime
//   - Switch: For conditional routing based on data
//   - Filter: For conditional processing (execute or skip)
//   - Concurrent: For parallel independent operations (requires Cloner[T])
//   - Race: When you need the fastest result
//   - Contest: When you need the fastest result that meets criteria
//   - Fallback: For primary/backup patterns
//   - Retry/Backoff: For handling transient failures
//   - Timeout: For operations that might hang
//   - Handle: For error monitoring without changing flow
//   - RateLimiter: For protecting rate-limited resources
//   - CircuitBreaker: For preventing cascade failures
//
// # Error Handling
//
// pipz provides rich error information through the Error[T] type:
//
//	type Error[T any] struct {
//	    Path      []string      // Full path: ["pipeline", "validate", "parse_json"]
//	    InputData T             // The input that caused the failure
//	    Err       error         // The underlying error
//	    Timestamp time.Time     // When the error occurred
//	    Duration  time.Duration // How long before failure
//	    Timeout   bool          // Was it a timeout?
//	    Canceled  bool          // Was it canceled?
//	}
//
// Error handling example:
//
//	result, err := pipeline.Process(ctx, data)
//	if err != nil {
//	    var pipeErr *pipz.Error[Data]
//	    if errors.As(err, &pipeErr) {
//	        log.Printf("Failed at: %s", strings.Join(pipeErr.Path, " → "))
//	        log.Printf("Input data: %+v", pipeErr.InputData)
//	        log.Printf("After: %v", pipeErr.Duration)
//
//	        if pipeErr.Timeout {
//	            // Handle timeout specifically
//	        }
//	    }
//	}
//
// # Performance
//
// pipz is designed for exceptional performance:
//
//   - Transform: 2.7ns per operation with zero allocations
//   - Apply/Effect (success): 46ns per operation with zero allocations
//   - Basic pipeline overhead: ~88 bytes, 3 allocations (constant regardless of length)
//   - Linear scaling: 5-step pipeline ~560ns, 50-step pipeline ~2.8μs
//   - No reflection or runtime type assertions
//   - Predictable performance characteristics
//
// See PERFORMANCE.md for detailed benchmarks.
//
// # Best Practices
//
//  1. Keep processors small and focused on a single responsibility
//  2. Use descriptive names for processors to aid debugging
//  3. Implement Cloner[T] correctly for types used with Concurrent/Race
//  4. Use NewSequence() for both static and dynamic pipelines
//  5. Check context.Err() in long-running processors
//  6. Let errors bubble up - handle at pipeline level
//  7. Use Effect for side effects to maintain purity
//  8. Test processors in isolation before composing
//  9. Prefer Transform over Apply when errors aren't possible
//
// 10. Use timeouts at the pipeline level, not individual processors
//
// # Common Patterns
//
// Validation Pipeline:
//
//	const (
//	    ValidationName = pipz.NewIdentity("validation", "")
//	    RequiredName = pipz.NewIdentity("required", "")
//	    FormatName = pipz.NewIdentity("format", "")
//	    SanitizeName = pipz.NewIdentity("sanitize", "")
//	)
//
//	validation := pipz.NewSequence(ValidationName,
//	    pipz.Effect(RequiredName, checkRequired),
//	    pipz.Effect(FormatName, checkFormat),
//	    pipz.Apply(SanitizeName, sanitizeInput),
//	)
//
// API with Retry and Timeout:
//
//	const (
//	    ApiTimeoutName = pipz.NewIdentity("api-timeout", "")
//	    ApiRetryName = pipz.NewIdentity("api-retry", "")
//	    FetchName = pipz.NewIdentity("fetch", "")
//	)
//
//	apiCall := pipz.NewTimeout(ApiTimeoutName,
//	    pipz.NewBackoff(ApiRetryName,
//	        pipz.Apply(FetchName, fetchFromAPI),
//	        3, time.Second,
//	    ),
//	    30*time.Second,
//	)
//
// Multi-path Processing:
//
//	var TypeRouterName = pipz.NewIdentity("type-router", "")
//	processor := pipz.NewSwitch(TypeRouterName, detectType)
//	processor.AddRoute("json", jsonProcessor)
//	processor.AddRoute("xml", xmlProcessor)
//	processor.AddRoute("csv", csvProcessor)
//
// For more examples, see the examples directory.
package pipz

import (
	"context"

	"github.com/google/uuid"
)

// Identity provides rich metadata for processors and connectors.
// It replaces the simple Name type with structured identity information
// that supports debugging, visualization, and profiling.
//
// Each Identity has an auto-generated UUID that uniquely identifies the
// processor or connector instance, enabling correlation between schema
// definitions and runtime signal events.
//
// Example:
//
//	var (
//	    ValidateOrderID = pipz.NewIdentity("validate-order", "Validates order structure")
//	    EnrichCustomerID = pipz.NewIdentity("enrich-customer", "Adds customer details from CRM")
//	)
//
//	pipeline := pipz.NewSequence(PipelineID,
//	    pipz.Apply(ValidateOrderID, validateOrder),
//	    pipz.Enrich(EnrichCustomerID, enrichCustomer),
//	)
type Identity struct {
	id          uuid.UUID
	name        string
	description string
}

// NewIdentity creates a new Identity with an auto-generated UUID.
// The name should be a short, descriptive identifier (e.g., "validate-order").
// The description provides additional context for debugging and documentation.
func NewIdentity(name, description string) Identity {
	return Identity{
		id:          uuid.New(),
		name:        name,
		description: description,
	}
}

// ID returns the unique identifier for this processor or connector.
func (i Identity) ID() uuid.UUID {
	return i.id
}

// Name returns the human-readable name.
func (i Identity) Name() string {
	return i.name
}

// Description returns the optional description.
func (i Identity) Description() string {
	return i.description
}

// String implements fmt.Stringer, returning the name for convenient logging.
func (i Identity) String() string {
	return i.name
}

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
//   - Context support for timeout and cancellation
//   - Type safety through generics (no interface{})
//   - Error propagation for fail-fast behavior
//   - Immutable by convention (return modified copies)
//   - Identity-based components for debugging, monitoring, and visualization
type Chainable[T any] interface {
	Process(context.Context, T) (T, error)
	Identity() Identity
	Schema() Node
	Close() error
}

// Processor defines an identified processing stage that transforms a value of type T.
// It contains an Identity for debugging and visualization, and a private function
// that processes the value. The function receives a context for cancellation and
// timeout control.
//
// Processor is the basic building block created by adapter functions like
// Apply, Transform, Effect, Mutate, and Enrich. The identity field is crucial for
// debugging, appearing in error messages and the Error[T].Path to identify exactly
// where failures occur. The identity's UUID enables correlation between schema
// definitions and runtime signal events.
//
// The fn field is intentionally private to ensure processors are only created through
// the provided adapter functions, maintaining consistent error handling and path tracking.
//
// Best practices for processor identities:
//   - Use descriptive, action-oriented names ("validate-email", not "email")
//   - Include the operation type ("parse-json", "fetch-user", "log-event")
//   - Keep names concise but meaningful
//   - Use descriptions to document the processor's purpose
//   - Identities appear in Error[T].Path for debugging
type Processor[T any] struct {
	fn       func(context.Context, T) (T, error)
	identity Identity
}

// Process implements the Chainable interface, allowing individual processors
// to be used directly or composed in connectors.
//
// This means a single Processor can be used anywhere a Chainable is expected:
//
//	validator := pipz.Effect(ValidateID, validateFunc)
//	// Can be used directly
//	result, err := validator.Process(ctx, data)
//	// Or in connectors
//	pipeline := pipz.NewSequence(PipelineID, validator, transformer)
func (p Processor[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, p.identity, data)
	return p.fn(ctx, data)
}

// Identity returns the identity of the processor for debugging and error reporting.
func (p Processor[T]) Identity() Identity {
	return p.identity
}

// Schema returns a Node representing this processor in the pipeline schema.
// Processors are leaf nodes with type "processor".
func (p Processor[T]) Schema() Node {
	return Node{
		Identity: p.identity,
		Type:     "processor",
	}
}

// Close gracefully shuts down any resources.
func (Processor[T]) Close() error {
	return nil
}

// Cloner is an interface for types that can create deep copies of themselves.
// Implementing this interface is required to use types with Concurrent and Race connectors,
// providing a type-safe and performant alternative to reflection-based copying.
//
// The Clone method must return a deep copy where modifications to the clone
// do not affect the original value. For types containing pointers, slices, or maps,
// ensure these are also copied to achieve true isolation between concurrent processors.
//
// Example implementation:
//
//	type Order struct {
//	    ID       string
//	    Items    []Item
//	    Status   string
//	    Metadata map[string]string
//	}
//
//	func (o Order) Clone() Order {
//	    // Deep copy slice
//	    items := make([]Item, len(o.Items))
//	    copy(items, o.Items)
//
//	    // Deep copy map
//	    metadata := make(map[string]string, len(o.Metadata))
//	    for k, v := range o.Metadata {
//	        metadata[k] = v
//	    }
//
//	    return Order{
//	        ID:       o.ID,
//	        Items:    items,
//	        Status:   o.Status,
//	        Metadata: metadata,
//	    }
//	}
type Cloner[T any] interface {
	Clone() T
}
