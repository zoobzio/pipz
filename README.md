# pipz

Build fast, type-safe processing pipelines in Go with zero dependencies and dynamic runtime modification.

pipz provides a simple way to build composable data pipelines that transform, validate, and process your data through a series of operations. Pipelines can be modified at runtime for conditional processing, A/B testing, and performance optimization. No global state, no magic, just clean functional composition with dynamic adaptability.

```go
// Create a pipeline
userPipeline := pipz.NewContract[User]()
userPipeline.Register(
    pipz.Validate(checkEmail),        // Validation
    pipz.Transform(normalizeData),    // Transformation  
    pipz.Mutate(applyDiscount, isVIP), // Conditional logic
    pipz.Effect(logAccess),           // Side effects
)

// Process data through the pipeline
user, err := userPipeline.Process(userData)

// Dynamic modification - add fraud detection for high-value users
if user.AccountValue > 10000 {
    userPipeline.PushHead(pipz.Apply(fraudCheck))
}
```

## Why pipz?

### The Problem: Scattered Business Logic

Every codebase eventually looks like this:

```go
// Middleware hell - repeated in every handler
func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    // Auth check scattered everywhere
    token := r.Header.Get("Authorization")
    if token == "" {
        http.Error(w, "Unauthorized", 401)
        return
    }
    
    // Rate limiting logic duplicated
    user := getUserFromToken(token)
    if isRateLimited(user) {
        http.Error(w, "Rate limited", 429)
        return
    }
    
    // Logging scattered across handlers
    log.Printf("API Request: %s %s", r.Method, r.URL.Path)
    
    // Finally... actual business logic
    processRequest(w, r)
}
// Copy-pasted everywhere with slight variations 😢
```

### The Solution: Pipeline-Driven Architecture

```go
// Create a reusable pipeline
apiPipeline := pipz.NewContract[Request]()
apiPipeline.Register(
    pipz.Apply(authenticate),  // Check auth token
    pipz.Apply(rateLimit),     // Apply rate limits
    pipz.Effect(logRequest),   // Log the request
)

// Use it cleanly in your handlers
func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    request, err := apiPipeline.Process(buildRequest(r))
    if err != nil {
        http.Error(w, err.Error(), getStatusCode(err))
        return
    }
    // Clean business logic with authenticated, rate-limited request
    processRequest(w, request)
}
```

**Result**: No more scattered conditionals. No more copy-paste middleware. Just clean, reusable, type-safe pipelines.

### The Problem: Error Handling Hell

Error handling logic gets duplicated everywhere:

```go
// Same error handling copied in every function 😫
func chargeCard(p Payment) (Payment, error) {
    err := processCharge(p)
    if err != nil {
        // Scattered error logic in every function
        if strings.Contains(err.Error(), "insufficient funds") {
            sendEmail(p.CustomerEmail, "Payment declined")
            logToAudit("insufficient_funds", p)
            return p, err
        }
        if strings.Contains(err.Error(), "network") {
            alertOpsTeam("payment_network_down")
            tryBackupProvider(p)
            return p, err
        }
        // More scattered error handling...
    }
    return p, nil
}
```

### The Solution: Error Handling as a Pipeline

```go
// Create a reusable error handling pipeline
errorPipeline := pipz.NewContract[PaymentError]()
errorPipeline.Register(
    pipz.Apply(categorizeError),     // Determine error type
    pipz.Apply(notifyCustomer),      // Send appropriate notifications  
    pipz.Apply(attemptRecovery),     // Try backup strategies
    pipz.Effect(alertOpsTeam),       // Alert operations if needed
    pipz.Effect(auditLog),           // Compliance logging
)

// Use it everywhere - no more duplication!
func chargeCard(p Payment) (Payment, error) {
    err := processCharge(p)
    if err != nil {
        result, _ := errorPipeline.Process(PaymentError{
            Payment: p,
            OriginalError: err,
        })
        return p, result.FinalError  // Might be nil if recovered!
    }
    return p, nil
}
```

**Result**: Error handling logic is centralized, testable, and reusable. No more copy-paste error handling!

### The Problem: Untestable Code

Business logic gets tangled with external dependencies:

```go
// Impossible to test without external services 😫
func processOrder(orderID string) error {
    // Database call mixed with business logic
    order, err := db.GetOrder(orderID)
    if err != nil {
        return err
    }
    
    // Validation mixed with external calls
    if order.Total < 0 {
        return errors.New("invalid total")
    }
    
    // Payment processing mixed with notifications
    if err := stripe.ChargeCard(order.CardToken, order.Total); err != nil {
        return err
    }
    
    // Email service mixed with business rules
    if err := sendGrid.SendConfirmation(order.Email, order.ID); err != nil {
        return err
    }
    
    return db.MarkOrderComplete(orderID)
}
```

### The Solution: Testable Pipeline Components

```go
// Pure business logic - easy to test!
orderPipeline := pipz.NewContract[Order]()
orderPipeline.Register(
    pipz.Validate(validateOrder),     // Pure validation
    pipz.Apply(calculateTax),         // Pure calculation
    pipz.Apply(applyDiscounts),       // Pure business rules
)

// Test with mock external services
func TestOrderProcessing(t *testing.T) {
    // Test pure business logic without external dependencies
    order := Order{ID: "123", Total: 100.00}
    
    result, err := orderPipeline.Process(order)
    
    // Fast, reliable tests
    assert.NoError(t, err)
    assert.Equal(t, 108.00, result.Total) // With tax
}

// Production: compose with external services
func processOrder(orderID string) error {
    order := fetchFromDB(orderID)
    
    // Business logic pipeline (tested)
    processedOrder, err := orderPipeline.Process(order)
    if err != nil {
        return err
    }
    
    // External services (mocked in tests)
    return saveAndNotify(processedOrder)
}
```

**Result**: Business logic is isolated and easily testable. External dependencies are cleanly separated!

## Installation

```bash
go get github.com/zoobzio/pipz
```

## Quick Start

### 1. Define Your Data Types

```go
type User struct {
    Name  string
    Email string
    Age   int
}
```

### 2. Write Processing Functions

Write normal Go functions - no special interfaces needed:

```go
// Validation function
func checkEmail(u User) error {
    if !strings.Contains(u.Email, "@") {
        return fmt.Errorf("invalid email: %s", u.Email)
    }
    return nil
}

// Transform function
func normalizeEmail(u User) User {
    u.Email = strings.ToLower(u.Email)
    return u
}

// Conditional logic
func applyDiscount(u User) User {
    u.Discount = 0.10  // 10% off
    return u
}

func isVIP(u User) bool {
    return u.TotalPurchases > 1000
}
```

### 3. Create and Use a Pipeline

```go
// Create a pipeline
userPipeline := pipz.NewContract[User]()
userPipeline.Register(
    pipz.Validate(checkEmail),           // Check without modifying
    pipz.Transform(normalizeEmail),      // Always modify
    pipz.Mutate(applyDiscount, isVIP),  // Conditionally modify
    pipz.Effect(func(u User) error {     // Side effects
        log.Printf("Processed user: %s", u.Email)
        return nil
    }),
)

// Process data
user, err := userPipeline.Process(User{
    Name:  "john doe",
    Email: "JOHN@EXAMPLE.COM",
    Age:   25,
})
// Result: {Name:"john doe" Email:"john@example.com" Age:25}
```

## Core Concept: Processors

pipz expects all processors to have the same signature:
```go
func(T) (T, error)
```

This uniform signature enables type-safe composition, but your functions might have different signatures. That's where adapters come in:

```go
// Your function signatures might vary:
func normalize(u User) User { }           // No error return
func validate(u User) error { }           // No data return
func enrich(u User) (User, error) { }    // Already matches!

// Adapters make them compatible:
pipeline.Register(
    pipz.Transform(normalize),  // Wraps to add error return
    pipz.Validate(validate),    // Wraps to add data passthrough
    pipz.Apply(enrich),        // Already matches, but makes intent clear
)
```

## Adapters

pipz provides adapters to wrap your functions based on their behavior:

### Transform
Always modifies the data:
```go
pipz.Transform(func(u User) User {
    u.Name = strings.Title(u.Name)
    return u
})
```

### Apply
Can fail with an error:
```go
pipz.Apply(func(u User) (User, error) {
    if u.Age < 0 {
        return u, errors.New("invalid age")
    }
    u.Verified = true
    return u, nil
})
```

### Validate
Checks without modifying:
```go
pipz.Validate(func(u User) error {
    if u.Email == "" {
        return errors.New("email required")
    }
    return nil
})
```

### Mutate
Conditionally modifies:
```go
pipz.Mutate(
    func(u User) User { u.Score *= 2; return u },  // transformer
    func(u User) bool { return u.IsVIP },          // condition
)
```

### Effect
Side effects without modification:
```go
pipz.Effect(func(u User) error {
    metrics.RecordUser(u)
    return nil
})
```

### Enrich
Best-effort data enhancement:
```go
pipz.Enrich(func(u User) (User, error) {
    profile, err := fetchProfile(u.ID)
    if err != nil {
        return u, err  // Original data continues
    }
    u.Profile = profile
    return u, nil
})
```

## Dynamic Pipeline Modification

pipz supports runtime pipeline modification for adaptive processing:

### Queue/Stack Operations

```go
contract := pipz.NewContract[Transaction]()

// Queue workflow (FIFO)
contract.PushTail(validate, normalize)    // Add to back
contract.PushHead(priorityCheck)          // Add to front
processor, _ := contract.PopHead()        // Remove from front

// Stack workflow (LIFO) 
contract.PushTail(undoValidation)         // Add to back
processor, _ := contract.PopTail()        // Remove from back (undo)
```

### Conditional Processing

```go
contract := pipz.NewContract[Payment]()
contract.PushTail(basicValidation, processPayment)

// Add fraud detection for high-value transactions
if payment.Amount > 1000 {
    contract.PushHead(fraudDetection)
}

// Remove expensive checks during high load
if serverLoad > 80 {
    contract.RemoveAt(0) // Remove fraud detection
}
```

### A/B Testing

```go
contract := pipz.NewContract[User]()
contract.PushTail(authenticate, authorize)

// Dynamically switch algorithms
if user.ID%2 == 0 {
    contract.ReplaceAt(1, newAuthAlgorithm)
} else {
    contract.ReplaceAt(1, legacyAuthAlgorithm)
}
```

### Pipeline Reordering

```go
contract := pipz.NewContract[Order]()
contract.PushTail(validate, enrich, audit)

// Optimize for performance - move audit to end
contract.MoveToTail(2)

// Reverse entire pipeline for testing
contract.Reverse()

// Swap processors
contract.Swap(0, 1)
```

### Runtime Optimization

```go
contract := pipz.NewContract[Request]()
contract.PushTail(expensiveValidation, process)

// Switch to lightweight validation under load
if cpuUsage > 75 {
    contract.ReplaceAt(0, lightValidation)
}

// Remove optional processors when overloaded
if queueDepth > 1000 {
    contract.Clear()
    contract.PushTail(essentialProcessor)
}
```

### Thread-Safe Modifications

All modification operations are thread-safe and can be performed concurrently with pipeline processing:

```go
contract := pipz.NewContract[Data]()

// Goroutine 1: Processing data
go func() {
    for data := range dataChannel {
        result, _ := contract.Process(data)
        // Process result...
    }
}()

// Goroutine 2: Dynamic optimization
go func() {
    for {
        if shouldOptimize() {
            contract.ReplaceAt(1, optimizedProcessor)
        }
        time.Sleep(time.Minute)
    }
}()
```

### Available Operations

- **Queue/Stack**: `PushHead()`, `PushTail()`, `PopHead()`, `PopTail()`
- **Precise**: `InsertAt()`, `RemoveAt()`, `ReplaceAt()`
- **Movement**: `MoveToHead()`, `MoveToTail()`, `MoveTo()`, `Swap()`, `Reverse()`
- **Utility**: `Len()`, `IsEmpty()`, `Clear()`

## Composing Pipelines

Use chains to combine multiple pipelines:

```go
// Create separate pipelines
validationPipeline := pipz.NewContract[Order]()
validationPipeline.Register(/* validators */)

enrichmentPipeline := pipz.NewContract[Order]()
enrichmentPipeline.Register(/* enrichers */)

auditPipeline := pipz.NewContract[Order]()
auditPipeline.Register(/* auditors */)

// Compose them
orderChain := pipz.NewChain[Order]()
orderChain.Add(
    validationPipeline.Link(),
    enrichmentPipeline.Link(),
    auditPipeline.Link(),
)

// Process through the complete chain
order, err := orderChain.Process(orderData)
```

## Error Handling

When a processor returns an error:
- Pipeline execution stops immediately
- The zero value of the type is returned
- The error is propagated to the caller

```go
pipeline := pipz.NewContract[User]()
pipeline.Register(
    pipz.Validate(checkAge),      // Returns error if age < 0
    pipz.Transform(normalizeData), // Won't execute if validation fails
)

user, err := pipeline.Process(invalidUser)
if err != nil {
    // Handle error - user will be zero value
}
```

## Performance

pipz is designed for speed:

- **Zero serialization overhead** - Direct function calls
- **No reflection** at runtime
- **No global locks** or state
- **Minimal allocations**

Benchmarks show pipz adds only ~20-30ns overhead per processor compared to direct function calls. See [benchmarks/](benchmarks/) for detailed performance analysis.

## Best Practices

### 1. Keep Processors Pure
```go
// Good - pure function
func normalizeEmail(u User) User {
    u.Email = strings.ToLower(u.Email)
    return u
}

// Avoid - has side effects
func normalizeEmail(u User) User {
    log.Printf("Normalizing %s", u.Email) // Side effect!
    u.Email = strings.ToLower(u.Email)
    return u
}
```

### 2. Use the Right Adapter
- `Validate` for checks that don't modify data
- `Transform` when you always modify
- `Apply` when the operation might fail
- `Effect` for logging, metrics, etc.

### 3. Compose Small Pipelines
Instead of one large pipeline, create focused pipelines and compose them:

```go
// Good - focused, reusable pipelines
validationPipeline := createValidationPipeline()
transformPipeline := createTransformPipeline()
auditPipeline := createAuditPipeline()

// Bad - everything in one pipeline
megaPipeline := pipz.NewContract[Data]()
megaPipeline.Register(validate1, validate2, transform1, transform2, audit1, audit2)
```

## Examples

See the [examples](examples/) directory for complete examples:
- **validation** - Order validation pipeline
- **security** - Security audit and data redaction
- **transform** - CSV to database transformation
- **payment** - Payment processing with error handling

Run interactive demos:
```bash
cd demos
go run . dynamic    # Dynamic pipeline modification
go run . all        # All capability demonstrations
```

## When to Use pipz

pipz is perfect when you have:
- ✅ Multi-step validation or transformation
- ✅ Reusable middleware patterns
- ✅ Complex error handling flows
- ✅ ETL or data processing pipelines
- ✅ Conditional processing logic
- ✅ A/B testing requirements
- ✅ Performance optimization needs
- ✅ Queue/stack processing patterns

## License

MIT