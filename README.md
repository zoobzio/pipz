# pipz

Build type-safe processing pipelines in Go that you can retrieve from anywhere in your codebase using just the types.

With pipz, you create a processing pipeline once and access it from any package without passing references around. Your pipelines are discoverable through Go's type system - if you know the types, you can find the pipeline.

```go
// Register a pipeline in one place...
const securityKey SecurityKey = "v1"
contract := pipz.GetContract[User](securityKey)
contract.Register(
    pipz.Apply(validateUser),
    pipz.Apply(sanitizeInput),
    pipz.Apply(auditAccess),
)

// ...retrieve and use it anywhere else
const securityKey SecurityKey = "v1"
contract := pipz.GetContract[User](securityKey)
user, err := contract.Process(userData)
```

No singletons to inject. No interfaces to implement. Just types.

## Installation

```bash
go get github.com/zoobzio/pipz
```

## Quick Start

### 5-Minute Guide to pipz

1. **Define your types** - A key type for discovery and a data type to process:

```go
type ValidationKey string    // Key type identifies the pipeline
type User struct {          // Data type to process
    Name  string
    Email string
    Age   int
}
```

2. **Write processor functions** - Simple functions that work with your types:

```go
// Validation - check without modifying
func checkEmail(u User) error {
    if !strings.Contains(u.Email, "@") {
        return fmt.Errorf("invalid email: %s", u.Email)
    }
    return nil
}

// Transformation - always modify
func normalizeEmail(u User) User {
    u.Email = strings.ToLower(u.Email)
    return u
}

// Conditional modification
func applyDiscount(u User) User {
    u.Discount = 0.10  // 10% off
    return u
}

func isVIP(u User) bool {
    return u.TotalPurchases > 1000
}
```

3. **Register using adapters** - Do this once at startup:

```go
const validationKey ValidationKey = "v1"
contract := pipz.GetContract[User](validationKey)
contract.Register(
    pipz.Validate(checkEmail),               // Check without modifying
    pipz.Transform(normalizeEmail),          // Always modify
    pipz.Mutate(applyDiscount, isVIP),      // Conditionally modify
    pipz.Effect(func(u User) error {         // Side effects
        log.Printf("Processed user: %s", u.Email)
        return nil
    }),
)
```

**Behind the scenes**: pipz handles all serialization. Your functions work with concrete types, not bytes!

4. **Use it anywhere** - No imports needed, just the types:

```go
// In a completely different package...
const validationKey ValidationKey = "v1"
contract := pipz.GetContract[User](validationKey)
user, err := contract.Process(User{
    Name:  "john doe",
    Email: "JOHN@EXAMPLE.COM",
    Age:   25,
})
// Result: {Name:"John Doe" Email:"john@example.com" Age:25}
```

That's it! The pipeline is globally discoverable through the type system.

### The Magic: Type-Based Discovery

```go
// team_a/auth.go
type AuthKey string
const authKey AuthKey = "v1"
contract := pipz.GetContract[User](authKey)
contract.Register(
    pipz.Apply(checkPassword),
    pipz.Apply(checkMFA),
    pipz.Apply(auditLogin),
)

// team_b/api.go - No import of team_a needed!
type AuthKey string  // Same type name
const authKey AuthKey = "v1"
contract := pipz.GetContract[User](authKey)
user, _ := contract.Process(loginRequest)  // Same pipeline!
```

If you know the types, you have the pipeline. No dependency injection, no singletons, no configuration.

## Philosophy: Types Instead of Logic

pipz represents a fundamental shift in how we organize code:

- **Traditional**: Business logic scattered in if/else statements
- **pipz**: Business logic encoded in the type system

```go
// ❌ Traditional: Runtime conditionals everywhere
if customer.Type == "premium" {
    processPremiumOrder(order)
} else if customer.Type == "standard" {
    processStandardOrder(order)
}

// ✅ pipz: Behavior determined by types
const premiumKey PremiumKey = "v1"
const standardKey StandardKey = "v1"
premiumContract := pipz.GetContract[Order](premiumKey)
standardContract := pipz.GetContract[Order](standardKey)
```

**The deeper insight**: Most if/else statements in business logic are just routing decisions. With pipz, the type system becomes your router. No more switch statements checking user roles, customer tiers, regions, or versions - just pass the right type and get the right behavior.

### Key Principles

1. **Isolation MEANS Isolation**: Different type universes cannot see or affect each other. Period.
2. **Types Are Configuration**: No YAML, no JSON, no environment variables. Just Go types.
3. **Zero Magic**: Everything is explicit. Want observability? Add it yourself:
   ```go
   func RegisterPaymentPipeline() {
       const paymentKey PaymentKey = "v1"
       contract := pipz.GetContract[Payment](paymentKey)
       contract.Register(
           pipz.Apply(validate),
           pipz.Apply(charge),
           pipz.Apply(notify),
       )

       // Your app's concern, not pipz's
       logger.Info("Payment pipeline v1 registered")
   }
   ```
4. **Ephemeral by Design**: pipz doesn't manage lifecycle. It's a pattern, not a framework.

## See It In Action

Want to see pipz in action before diving into code? Run the interactive demos:

```bash
go run ./demo all         # Run all demos
go run ./demo security    # Security audit pipeline
go run ./demo transform   # Data transformation
go run ./demo universes   # Multi-tenant isolation
go run ./demo validation  # Data validation
go run ./demo workflow    # Multi-stage workflows
go run ./demo middleware  # Request middleware
go run ./demo errors      # Error handling pipelines
go run ./demo versioning  # Pipeline versioning & A/B/C testing
go run ./demo testing     # Testing without mocks

# For a more engaging experience with typewriter effects:
go run ./demo all --interactive
```

## Complete Example

```go
package main

import (
    "fmt"
    "strings"
    "github.com/zoobzio/pipz"
)

// Define a contract key type
type UserProcessorKey string

// Define your data type
type User struct {
    Name  string
    Email string
    Age   int
}

func main() {
    // Get a contract
    const userProcessorKey UserProcessorKey = "v1"
    contract := pipz.GetContract[User](userProcessorKey)

    // Register processors
    contract.Register(
        pipz.Apply(normalizeEmail),
        pipz.Apply(validateAge),
        pipz.Apply(formatName),
    )

    // Process data
    user := User{Name: "john doe", Email: "JOHN@EXAMPLE.COM", Age: 25}
    result, err := contract.Process(user)
    if err != nil {
        panic(err)
    }

    fmt.Printf("%+v\n", result)
    // Output: {Name:John Doe Email:john@example.com Age:25}
}

func normalizeEmail(u User) (User, error) {
    u.Email = strings.ToLower(u.Email)
    return u, nil
}

func validateAge(u User) (User, error) {
    if u.Age < 0 || u.Age > 150 {
        return u, fmt.Errorf("invalid age: %d", u.Age)
    }
    return u, nil
}

func formatName(u User) (User, error) {
    // Title case each word in the name
    words := strings.Fields(strings.ToLower(u.Name))
    for i, word := range words {
        if len(word) > 0 {
            words[i] = strings.ToUpper(word[:1]) + word[1:]
        }
    }
    u.Name = strings.Join(words, " ")
    return u, nil
}
```

## API Reference

### Core Types

| Type                            | Description                                                  |
| ------------------------------- | ------------------------------------------------------------ |
| `Processor[T any]`              | Function type that transforms `T` to `T` with possible error |
| `Contract[K comparable, T any]` | Type-safe pipeline bound to key type `K` and data type `T`   |
| `Chainable[T any]`              | Interface for components that can process type `T`           |
| `Chain[T any]`                  | Sequential executor for multiple `Chainable[T]` components   |
| `ByteProcessor`                 | Low-level function type for byte transformations             |

### Contract Methods

| Method                                 | Description                                     |
| -------------------------------------- | ----------------------------------------------- |
| `GetContract[T](key K)`                | Gets or creates a contract with given key       |
| `Register(processors ...Processor[T])` | Registers processors to the contract's pipeline |
| `Process(value T) (T, error)`          | Executes the pipeline on input value            |
| `Link() Chainable[T]`                  | Returns contract as chainable for composition   |
| `String() string`                      | Returns string representation of contract       |

### Chain Methods

| Method                            | Description                                   |
| --------------------------------- | --------------------------------------------- |
| `NewChain[T]()`                   | Creates a new empty chain                     |
| `Add(processors ...Chainable[T])` | Adds chainable processors to the chain        |
| `Process(value T) (T, error)`     | Executes all processors sequentially in order |

### Utility Functions

| Function                                       | Description                                   |
| ---------------------------------------------- | --------------------------------------------- |
| `Signature[T any](key K) string`               | Returns unique signature for contract K:key:T |

## Concepts

### Contracts

Contracts provide type-safe access to processing pipelines. They are identified by two generic parameters:

- `K`: A comparable type serving as the contract's domain identifier
- `T`: The data type being processed

The combination of `K`, its value, and `T` creates a globally unique pipeline identifier.

### Type-Based Discovery

By using concrete types as keys, contracts become discoverable through type information alone:

```go
type AuthKey string
type ValidationKey string

// Different contracts for different purposes
const authKey AuthKey = "v1"
const validationKey ValidationKey = "v1"
authContract := pipz.GetContract[User](authKey)
validationContract := pipz.GetContract[User](validationKey)

// Anyone with AuthKey and User types can retrieve the same pipeline
const authKey AuthKey = "v1"
contract := pipz.GetContract[User](authKey)
```

### Type Universes

**Type Universes are the killer feature of pipz.** They let you create completely isolated processing pipelines for different contexts using the same data types.

#### The Concept

The KEY TYPE determines which pipeline you get. Even if two pipelines use the same key value and process the same data type, different key types create different universes:

```go
type StandardKey string
type PremiumKey string

// Same key value "v1", same data type Transaction
// But COMPLETELY DIFFERENT pipelines!
const standardKey StandardKey = "v1"
const premiumKey PremiumKey = "v1"
standard := pipz.GetContract[Transaction](standardKey)
premium := pipz.GetContract[Transaction](premiumKey)
```

#### Real-World Example: Multi-Tenant Payment Processing

```go
// Define separate key types for each merchant tier
type StandardMerchantKey string
type PremiumMerchantKey string
type HighRiskMerchantKey string

// All process the same Transaction type
type Transaction struct {
    ID       string
    Amount   float64
    Currency string
    CardLast4 string
}

// Standard merchants: basic fraud checks
const standardMerchantKey StandardMerchantKey = "v1"
standardContract := pipz.GetContract[Transaction](standardMerchantKey)
standardContract.Register(
    pipz.Apply(validateAmount),      // Max $10,000
    pipz.Apply(checkVelocity),      // 10 transactions/hour
    pipz.Apply(notifyMerchant),
)

// Premium merchants: relaxed limits, priority processing
const premiumMerchantKey PremiumMerchantKey = "v1"
premiumContract := pipz.GetContract[Transaction](premiumMerchantKey)
premiumContract.Register(
    pipz.Apply(validatePremiumAmount),  // Max $100,000
    pipz.Apply(checkPremiumVelocity),  // 100 transactions/hour
    pipz.Apply(prioritySettle),        // Same-day settlement
    pipz.Apply(notifyPremium),         // SMS + Email alerts
)

// High-risk merchants: enhanced security
const highRiskMerchantKey HighRiskMerchantKey = "v1"
highRiskContract := pipz.GetContract[Transaction](highRiskMerchantKey)
highRiskContract.Register(
    pipz.Apply(validate3DS),           // Require 3D Secure
    pipz.Apply(checkBlocklist),        // Enhanced fraud database
    pipz.Apply(manualReview),          // Flag for human review
    pipz.Apply(holdFunds),            // 7-day settlement hold
    pipz.Apply(auditLog),             // Regulatory compliance
)

// Usage: Generic function - the KEY TYPE determines the pipeline!
func processPayment[K comparable](key K, txn Transaction) (Transaction, error) {
    contract := pipz.GetContract[Transaction](key)
    return contract.Process(txn)
}

// The caller chooses the universe by passing the right key type:
const standardKey StandardMerchantKey = "v1"
const premiumKey PremiumMerchantKey = "v1"
const highRiskKey HighRiskMerchantKey = "v1"
result, err := processPayment(standardKey, txn)  // Standard pipeline
result, err := processPayment(premiumKey, txn)   // Premium pipeline
result, err := processPayment(highRiskKey, txn)  // High-risk pipeline

// Even cleaner - let the merchant's key type drive the behavior:
func (m Merchant) ProcessTransaction(txn Transaction) (Transaction, error) {
    // Merchant has a key field that determines their universe
    return processPayment(m.Key, txn)
}
```

#### Why This Matters

1. **Complete Isolation**: A bug in the high-risk pipeline CANNOT affect standard merchants
2. **Type Safety**: You cannot accidentally process a high-risk transaction through the standard pipeline
3. **Independent Development**: Teams can work on different pipelines without coordination
4. **A/B Testing**: Run experiments without feature flags
5. **Multi-tenancy**: Each customer gets their own processing universe

#### Common Use Cases for Type Universes

```go
// A/B Testing
type StrategyAKey string
type StrategyBKey string

// Multi-region processing
type USRegionKey string
type EURegionKey string
type APACRegionKey string

// Environment separation
type DevKey string
type StagingKey string
type ProductionKey string

// API versioning
type APIv1Key string
type APIv2Key string
type APIv3Key string

// Customer tiers
type FreeUserKey string
type ProUserKey string
type EnterpriseKey string
```

Each key type creates its own universe. The same code can behave completely differently based on which universe it's in.

### Processing Model

1. Processors are pure functions that transform data
2. Each processor receives the output of the previous processor
3. Processing stops on first error
4. Original data is preserved if an error occurs

### Composition

Contracts can be composed into larger workflows using chains:

```go
chain := pipz.NewChain[User]()
chain.Add(
    validationContract.Link(),
    enrichmentContract.Link(),
    auditContract.Link(),
)

result, err := chain.Process(user)
```

## How It Works

### The Architecture

pipz uses several techniques to provide its type-safe, discoverable pipeline system:

#### 1. Global Registry Pattern

A singleton service initialized at package import manages all pipelines:

- Pipelines are stored as chains of `ByteProcessor` functions
- Thread-safe with read/write mutex protection
- Allows pipeline discovery from anywhere in your codebase

#### 2. Smart Serialization

Contracts intelligently handle data serialization:

- Serialization only happens when absolutely necessary
- Direct memory passing when possible
- Automatic optimization for common patterns
- Zero overhead for most use cases

#### 3. Smart Type Caching

Type reflection happens only once per type:

- First use: `reflect.TypeOf()` captures type information
- Subsequent uses: Returns cached string instantly
- Thread-safe double-checked locking for performance

### Thread Safety

All operations are thread-safe:

- Multiple goroutines can register pipelines concurrently
- Multiple goroutines can process data through pipelines concurrently
- Type cache is protected with read/write mutexes

### Important Behaviors

- **Contract Registration**: Registering processors to an existing contract **overwrites** the previous pipeline
- **Error Handling**: If any processor fails, the chain stops and returns the original input value with the error
- **Isolation**: Each contract's processors are completely isolated via gob copying

## Best Practices

### Use Constants for Contract Keys

Instead of using string literals throughout your code, define constants:

```go
// Good
const (
    SecurityContractV1 = "v1"
    SecurityContractV2 = "v2"
)

const securityKey SecurityKey = SecurityContractV1
contract := pipz.GetContract[User](securityKey)

// Avoid
contract := pipz.GetContract[User](SecurityKey("v1"))
```

### Use Meaningful Key Types

Your key types should describe the domain they represent:

```go
// Good - clearly indicates purpose
type AuthenticationKey string
type ValidationKey string
type PersistenceKey string

// Avoid - too generic
type Key string
type ProcessorKey string
```

### Embrace Type Proliferation

Many key types is not a problem - it's the solution:

```go
// Each type represents different business logic
type FreeUserKey string
type PremiumUserKey string
type EnterpriseUserKey string

type USRegionKey string
type EURegionKey string
type APACRegionKey string

type TestPaymentKey string
type LivePaymentKey string
```

These types replace runtime conditionals with compile-time guarantees. You're moving business logic from if/else statements into the type system where it belongs.

### Compose with Chains

When combining multiple contracts, use a single `Add` call:

```go
// Good
chain := pipz.NewChain[User]()
chain.Add(
    authContract.Link(),
    validationContract.Link(),
    auditContract.Link(),
)

// Avoid
chain := pipz.NewChain[User]()
chain.Add(authContract.Link())
chain.Add(validationContract.Link())
chain.Add(auditContract.Link())
```

### Keep Processors Pure

Processors should be pure functions without side effects:

```go
// Good - pure function
func validateAge(u User) (User, error) {
    if u.Age < 0 || u.Age > 150 {
        return u, fmt.Errorf("invalid age: %d", u.Age)
    }
    return u, nil
}

// Avoid - has side effects
func validateAge(u User) (User, error) {
    log.Printf("Validating user %s", u.Name) // Side effect!
    if u.Age < 0 || u.Age > 150 {
        return u, fmt.Errorf("invalid age: %d", u.Age)
    }
    return u, nil
}
```

### Error Messages Should Be Descriptive

Include context in your error messages:

```go
// Good
return u, fmt.Errorf("age validation failed: %d is outside valid range (0-150)", u.Age)

// Avoid
return u, fmt.Errorf("invalid age")
```

## Common Questions

### "What about the global state?"

Isolation MEANS isolation. `TestKey("test-1")` and `TestKey("test-2")` create completely separate universes that cannot see each other. The global registry is an implementation detail - what matters is that universes are isolated.

### "Won't I have too many key types?"

That's the point! Each key type represents different business logic. Instead of runtime if/else statements, you encode behavior in types. Type proliferation is a feature, not a bug - it's compile-time business logic.

### "How do I debug which processors are registered?"

Add logging to your registration functions - pipz is ephemeral by design:

```go
func RegisterPaymentPipeline() {
    const paymentKey PaymentKey = "v1"
    contract := pipz.GetContract[Payment](paymentKey)
    contract.Register(
        pipz.Apply(validate),
        pipz.Apply(charge),
        pipz.Apply(notify),
    )
    log.Printf("Registered payment pipeline v1 with %d processors", 3)
)
```

### "Is this just dependency injection?"

No. DI containers use reflection, configuration, and runtime resolution. pipz uses types - if you know the types, you have the pipeline. No container, no interfaces, no magic.

## Documentation

- [ADAPTERS.md](ADAPTERS.md) - Complete guide to using and creating adapters
- [USE_CASES.md](USE_CASES.md) - Detailed use cases with live demonstrations

Run the interactive demos to see pipz in action:

```bash
go run ./demo all         # Run all demos
go run ./demo security    # Security audit pipeline
go run ./demo transform   # Data transformation
go run ./demo universes   # Multi-tenant isolation
go run ./demo validation  # Data validation
go run ./demo workflow    # Multi-stage workflows
go run ./demo middleware  # Request middleware
go run ./demo errors      # Error handling pipelines
go run ./demo versioning  # Pipeline versioning & A/B/C testing
go run ./demo testing     # Testing without mocks
```

## License

MIT
