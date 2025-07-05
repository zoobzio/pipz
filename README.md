# pipz

Build type-safe processing pipelines in Go that you can retrieve from anywhere in your codebase using just the types.

With pipz, you create a processing pipeline once and access it from any package without passing references around. Your pipelines are discoverable through Go's type system - if you know the types, you can find the pipeline.

```go
// Register a pipeline in one place...
contract := pipz.GetContract[SecurityKey, User](SecurityKey("v1"))
contract.Register(validateUser, sanitizeInput, auditAccess)

// ...retrieve and use it anywhere else
contract := pipz.GetContract[SecurityKey, User](SecurityKey("v1"))
user, err := contract.Process(userData)
```

No singletons to inject. No interfaces to implement. Just types.

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
premiumContract := pipz.GetContract[PremiumKey, Order](PremiumKey("v1"))
standardContract := pipz.GetContract[StandardKey, Order](StandardKey("v1"))
```

### Key Principles

1. **Isolation MEANS Isolation**: Different type universes cannot see or affect each other. Period.
2. **Types Are Configuration**: No YAML, no JSON, no environment variables. Just Go types.
3. **Zero Magic**: Everything is explicit. Want observability? Add it yourself:
   ```go
   func RegisterPaymentPipeline() {
       contract := pipz.GetContract[PaymentKey, Payment](PaymentKey("v1"))
       contract.Register(validate, charge, notify)
       
       // Your app's concern, not pipz's
       logger.Info("Payment pipeline v1 registered")
   }
   ```
4. **Ephemeral by Design**: pipz doesn't manage lifecycle. It's a pattern, not a framework.

## Installation

```bash
go get github.com/zoobzio/pipz
```

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

## Quick Start

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
    contract := pipz.GetContract[UserProcessorKey, User](UserProcessorKey("v1"))

    // Register processors
    contract.Register(
        normalizeEmail,
        validateAge,
        formatName,
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
| `GetContract[K, T](key K)`             | Gets or creates a contract with given key       |
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
| `Signature[K comparable, T any](key K) string` | Returns unique signature for contract K:key:T |

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
authContract := pipz.GetContract[AuthKey, User](AuthKey("v1"))
validationContract := pipz.GetContract[ValidationKey, User](ValidationKey("v1"))

// Anyone with AuthKey and User types can retrieve the same pipeline
contract := pipz.GetContract[AuthKey, User](AuthKey("v1"))
```

### Type Universes

The same string value combined with different key types creates isolated processing universes. This enables powerful multi-tenant architectures where each tenant's data flows through completely separate pipelines without any cross-contamination:

```go
// Multi-tenant payment processor example
type StandardMerchantKey string
type PremiumMerchantKey string
type HighRiskMerchantKey string

type Transaction struct {
    ID       string
    Amount   float64
    Currency string
    CardLast4 string
}

// Standard merchants: basic fraud checks
standardContract := pipz.GetContract[StandardMerchantKey, Transaction](StandardMerchantKey("v1"))
standardContract.Register(
    validateAmount,      // Max $10,000
    checkVelocity,      // 10 transactions/hour
    notifyMerchant,
)

// Premium merchants: relaxed limits, priority processing
premiumContract := pipz.GetContract[PremiumMerchantKey, Transaction](PremiumMerchantKey("v1"))
premiumContract.Register(
    validatePremiumAmount,  // Max $100,000
    checkPremiumVelocity,  // 100 transactions/hour
    prioritySettle,        // Same-day settlement
    notifyPremium,         // SMS + Email alerts
)

// High-risk merchants: enhanced security
highRiskContract := pipz.GetContract[HighRiskMerchantKey, Transaction](HighRiskMerchantKey("v1"))
highRiskContract.Register(
    validate3DS,           // Require 3D Secure
    checkBlocklist,        // Enhanced fraud database
    manualReview,          // Flag for human review
    holdFunds,            // 7-day settlement hold
    auditLog,             // Regulatory compliance
)

// The key value "v1" is the same, but the pipelines are completely isolated
// Each merchant type has its own processing rules without knowing about the others
```

This pattern is particularly powerful because:

- **Complete Isolation**: Pipelines cannot accidentally access each other
- **Type Safety**: Can't mistakenly process a high-risk transaction through standard pipeline
- **Extensibility**: New merchant types can be added without touching existing code
- **Discoverable**: Any code with the right key type can access the appropriate pipeline

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

contract := pipz.GetContract[SecurityKey, User](SecurityKey(SecurityContractV1))

// Avoid
contract := pipz.GetContract[SecurityKey, User](SecurityKey("v1"))
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
    contract := pipz.GetContract[PaymentKey, Payment](PaymentKey("v1"))
    contract.Register(validate, charge, notify)
    log.Printf("Registered payment pipeline v1 with %d processors", 3)
}
```

### "Is this just dependency injection?"
No. DI containers use reflection, configuration, and runtime resolution. pipz uses types - if you know the types, you have the pipeline. No container, no interfaces, no magic.

## Use Cases

For detailed use cases with live demonstrations, see [USE_CASES.md](USE_CASES.md).

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
