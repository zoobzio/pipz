# pipz Use Cases

This document showcases real-world use cases for pipz. Don't just read the code - **run the live demos** to see the magic happen! Each use case has an interactive demonstration:

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

## Data Validation Pipeline

Validate complex data structures with reusable validators:

```go
type ValidatorKey string

type Order struct {
    ID         string
    CustomerID string
    Items      []OrderItem
    Total      float64
}

type OrderItem struct {
    ProductID string
    Quantity  int
    Price     float64
}

// Validators
func validateOrderID(o Order) (Order, error) {
    if o.ID == "" {
        return o, fmt.Errorf("order ID required")
    }
    if !strings.HasPrefix(o.ID, "ORD-") {
        return o, fmt.Errorf("order ID must start with ORD-")
    }
    return o, nil
}

func validateItems(o Order) (Order, error) {
    if len(o.Items) == 0 {
        return o, fmt.Errorf("order must have at least one item")
    }
    for i, item := range o.Items {
        if item.Quantity <= 0 {
            return o, fmt.Errorf("item %d: quantity must be positive", i)
        }
        if item.Price < 0 {
            return o, fmt.Errorf("item %d: price cannot be negative", i)
        }
    }
    return o, nil
}

func validateTotal(o Order) (Order, error) {
    calculated := 0.0
    for _, item := range o.Items {
        calculated += float64(item.Quantity) * item.Price
    }
    if math.Abs(calculated-o.Total) > 0.01 {
        return o, fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
    }
    return o, nil
}

// Usage
validationContract := pipz.GetContract[ValidatorKey, Order](ValidatorKey("v1"))
validationContract.Register(validateOrderID, validateItems, validateTotal)
```

## Security Audit Pipeline

Track and audit data access with composable audit processors:

```go
type AuditKey string

type AuditableData struct {
    Data      interface{}
    UserID    string
    Timestamp time.Time
    Actions   []string
}

func logAccess(a AuditableData) (AuditableData, error) {
    a.Actions = append(a.Actions, fmt.Sprintf("accessed by %s at %s",
        a.UserID, a.Timestamp.Format(time.RFC3339)))
    return a, nil
}

func checkPermissions(a AuditableData) (AuditableData, error) {
    // Simulate permission check
    if a.UserID == "" {
        return a, fmt.Errorf("unauthorized: no user ID")
    }
    a.Actions = append(a.Actions, "permissions verified")
    return a, nil
}

func redactSensitive(a AuditableData) (AuditableData, error) {
    // Redact based on data type
    switch v := a.Data.(type) {
    case *User:
        if len(v.SSN) > 4 {
            v.SSN = "XXX-XX-" + v.SSN[len(v.SSN)-4:]
        }
    }
    a.Actions = append(a.Actions, "sensitive data redacted")
    return a, nil
}

// Usage
auditContract := pipz.GetContract[AuditKey, AuditableData](AuditKey("v1"))
auditContract.Register(checkPermissions, logAccess, redactSensitive)
```

## Data Transformation Pipeline

Transform data between different representations:

```go
type TransformKey string

type CSVRecord struct {
    Fields []string
}

type DatabaseRecord struct {
    ID        int
    Name      string
    Email     string
    CreatedAt time.Time
}

type TransformContext struct {
    CSV    *CSVRecord
    DB     *DatabaseRecord
    Errors []string
}

func parseCSV(ctx TransformContext) (TransformContext, error) {
    if len(ctx.CSV.Fields) < 3 {
        return ctx, fmt.Errorf("insufficient fields")
    }

    id, err := strconv.Atoi(ctx.CSV.Fields[0])
    if err != nil {
        ctx.Errors = append(ctx.Errors, "invalid ID format")
        id = 0
    }

    ctx.DB = &DatabaseRecord{
        ID:        id,
        Name:      ctx.CSV.Fields[1],
        Email:     ctx.CSV.Fields[2],
        CreatedAt: time.Now(),
    }
    return ctx, nil
}

func validateEmail(ctx TransformContext) (TransformContext, error) {
    if ctx.DB == nil {
        return ctx, fmt.Errorf("no database record")
    }

    if !strings.Contains(ctx.DB.Email, "@") {
        ctx.Errors = append(ctx.Errors, "invalid email format")
    }
    return ctx, nil
}

func enrichData(ctx TransformContext) (TransformContext, error) {
    if ctx.DB != nil {
        // Title case the name
        words := strings.Fields(strings.ToLower(ctx.DB.Name))
        for i, word := range words {
            if len(word) > 0 {
                words[i] = strings.ToUpper(word[:1]) + word[1:]
            }
        }
        ctx.DB.Name = strings.Join(words, " ")
    }
    return ctx, nil
}

// Usage
transformContract := pipz.GetContract[TransformKey, TransformContext](TransformKey("csv-to-db"))
transformContract.Register(parseCSV, validateEmail, enrichData)
```

## Multi-Stage Processing Workflow

Combine multiple contracts for complex workflows:

```go
type WorkflowStage string

const (
    ValidationStage   WorkflowStage = "validation"
    EnrichmentStage   WorkflowStage = "enrichment"
    PersistenceStage  WorkflowStage = "persistence"
)

type WorkflowData struct {
    User      User
    Metadata  map[string]interface{}
    Processed []string
}

// Create separate contracts for each stage
func setupWorkflow() *pipz.Chain[WorkflowData] {
    // Validation stage
    validationContract := pipz.GetContract[WorkflowStage, WorkflowData](ValidationStage)
    validationContract.Register(
        func(w WorkflowData) (WorkflowData, error) {
            if w.User.Email == "" {
                return w, fmt.Errorf("email required")
            }
            w.Processed = append(w.Processed, "validated")
            return w, nil
        },
    )

    // Enrichment stage
    enrichmentContract := pipz.GetContract[WorkflowStage, WorkflowData](EnrichmentStage)
    enrichmentContract.Register(
        func(w WorkflowData) (WorkflowData, error) {
            w.Metadata["enriched_at"] = time.Now()
            w.Metadata["ip_country"] = "US" // Mock enrichment
            w.Processed = append(w.Processed, "enriched")
            return w, nil
        },
    )

    // Persistence stage
    persistenceContract := pipz.GetContract[WorkflowStage, WorkflowData](PersistenceStage)
    persistenceContract.Register(
        func(w WorkflowData) (WorkflowData, error) {
            // Simulate database save
            w.Metadata["persisted"] = true
            w.Metadata["id"] = "USR-12345"
            w.Processed = append(w.Processed, "persisted")
            return w, nil
        },
    )

    // Compose into workflow
    chain := pipz.NewChain[WorkflowData]()
    chain.Add(
        validationContract.Link(),
        enrichmentContract.Link(),
        persistenceContract.Link(),
    )
    return chain
}
```

## Request/Response Middleware

Build HTTP-like middleware chains:

```go
type MiddlewareKey string

type Request struct {
    Path    string
    Headers map[string]string
    Body    []byte
    Context map[string]interface{}
}

func authenticate(r Request) (Request, error) {
    token := r.Headers["Authorization"]
    if token == "" {
        return r, fmt.Errorf("no auth token")
    }

    // Validate token (mock)
    if !strings.HasPrefix(token, "Bearer ") {
        return r, fmt.Errorf("invalid token format")
    }

    r.Context["user_id"] = "user-123"
    r.Context["authenticated"] = true
    return r, nil
}

func rateLimit(r Request) (Request, error) {
    userID, ok := r.Context["user_id"].(string)
    if !ok {
        return r, fmt.Errorf("user not authenticated")
    }

    // Check rate limit (mock)
    limit := 100
    if strings.Contains(userID, "premium") {
        limit = 1000
    }

    r.Context["rate_limit"] = limit
    return r, nil
}

func logRequest(r Request) (Request, error) {
    fmt.Printf("[%s] %s - User: %v\n",
        time.Now().Format(time.RFC3339),
        r.Path,
        r.Context["user_id"])
    return r, nil
}

// Usage
middlewareContract := pipz.GetContract[MiddlewareKey, Request](MiddlewareKey("api-v1"))
middlewareContract.Register(authenticate, rateLimit, logRequest)
```

## Error Handling Pipelines

Use pipelines for sophisticated error handling and recovery:

```go
type PaymentKey string
type PaymentErrorKey string

type Payment struct {
    ID        string
    Amount    float64
    Provider  string
}

type PaymentError struct {
    Payment       Payment
    OriginalError error
    ErrorType     string
    RecoveryAttempted bool
    RecoverySuccess   bool
    FinalError    error
}

// Payment processor that can trigger error pipeline
func chargeCard(p Payment) ([]byte, error) {
    err := processCharge(p)
    if err != nil {
        // Discover and use error pipeline - NO IMPORTS NEEDED!
        errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorKey("v1"))
        result, _ := errorContract.Process(PaymentError{
            Payment:       p,
            OriginalError: err,
        })
        
        // Check if recovery succeeded
        if result.RecoverySuccess {
            p.Provider = "backup"
            return pipz.Encode(p)
        }
        
        return nil, result.FinalError
    }
    return pipz.Encode(p)
}

// Error handling processors
func categorizeError(e PaymentError) ([]byte, error) {
    // Analyze error type
    if strings.Contains(e.OriginalError.Error(), "network") {
        e.ErrorType = "network"
    }
    return pipz.Encode(e)
}

func attemptRecovery(e PaymentError) ([]byte, error) {
    if e.ErrorType == "network" {
        // Try backup provider
        e.RecoveryAttempted = true
        e.RecoverySuccess = tryBackupProvider()
    }
    return pipz.Encode(e)
}

// Register pipelines
paymentContract := pipz.GetContract[PaymentKey, Payment](PaymentKey("v1"))
paymentContract.Register(validatePayment, chargeCard, updateStatus)

errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorKey("v1"))
errorContract.Register(categorizeError, notifyCustomer, attemptRecovery, auditLog)
```

### Key Insight: Type-Based Discovery

The error handling use case demonstrates the most powerful aspect of pipz: **complete decoupling through type-based discovery**. 

The payment processor can discover and use the error handling pipeline without any imports or dependencies. This means:

- **Team Independence**: Different teams can own different pipelines
- **No Coupling**: Payment code doesn't import error handling code
- **Type Safety**: Still get compile-time guarantees
- **Dynamic Discovery**: Pipelines are discovered at runtime using only type information

This enables a microservice-like architecture within a monolith, where different domains can evolve independently while still working together seamlessly.

## Pipeline Versioning and A/B/C Testing

Use type universes to run multiple pipeline versions simultaneously:

```go
type PaymentKey string

type Payment struct {
    ID         string
    Amount     float64
    CustomerID string
    Timestamp  time.Time
}

// Version A: MVP - Just charge the card
vA := pipz.GetContract[PaymentKey, Payment](PaymentKey("A"))
vA.Register(chargeCard)

// Version B: Add validation
vB := pipz.GetContract[PaymentKey, Payment](PaymentKey("B"))
vB.Register(validateAmount, chargeCard, logPayment)

// Version C: Enterprise features
vC := pipz.GetContract[PaymentKey, Payment](PaymentKey("C"))
vC.Register(
    validateAmount,
    checkVelocity,
    fraudScore,
    routeProvider,
    chargeCard,
    notify,
    analytics,
)

// A/B/C testing with zero interference
func processPayment(payment Payment) (Payment, error) {
    switch getTestGroup(payment.CustomerID) {
    case "A":
        return vA.Process(payment)  // Simple customers
    case "B":
        return vB.Process(payment)  // Standard customers
    case "C":
        return vC.Process(payment)  // Premium customers
    default:
        return vA.Process(payment)  // Fallback to MVP
    }
}
```

### Key Insight: Natural Versioning

Pipeline versioning isn't a framework feature - it's a natural consequence of type universes:

- **Same Types, Different Universes**: `PaymentKey("A")`, `PaymentKey("B")`, and `PaymentKey("C")` create completely isolated pipelines
- **Zero Coupling**: Modify version C without touching A or B
- **Simultaneous Operation**: All versions run concurrently with no interference
- **Gradual Complexity**: Each version can build on lessons learned without risk
- **Infinite Scaling**: Could have D, E, F... each team owns their version

This pattern shows how pipz enables:
1. **Safe experimentation**: Try new features in isolation
2. **Customer segmentation**: Different features for different tiers
3. **Gradual rollout**: Start with 1%, scale to 100%
4. **Instant rollback**: Just change the version string
5. **A/B/C/D... testing**: Unlimited variations

The beauty is in the simplicity - versioning requires no special APIs, just different strings passed to the same `GetContract` function.

## Testing Without Mocks

Use type universes to create isolated test environments:

```go
type EmailServiceKey string

type EmailMessage struct {
    ID      string
    To      string
    Subject string
    Body    string
    // ... processing fields
}

// Production pipeline
prodContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("prod"))
prodContract.Register(
    validateEmail,
    sanitizeContent,
    applyTemplate,
    sendViaProvider,  // Real email sending
)

// Test pipeline - same types, different universe!
testContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("test"))
testContract.Register(
    validateEmail,    // Reuse real validation
    sanitizeContent,  // Reuse real sanitization
    testTemplate,     // Simplified for tests
    mockSend,         // Don't actually send!
)

// In tests, just use the test contract
func TestEmailValidation(t *testing.T) {
    msg := EmailMessage{To: "invalid-email"}
    _, err := testContract.Process(msg)
    assert.Error(t, err, "Should reject invalid email")
}

// Advanced patterns
hybridContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("hybrid"))
hybridContract.Register(
    validateEmail,    // Real validation
    sanitizeContent,  // Real sanitization
    applyTemplate,    // Real templates
    mockSend,         // But don't send
)
```

### Key Insight: Tests Are Just Another Universe

Testing with pipz doesn't require:
- **No mocking frameworks**: Just different processor implementations
- **No dependency injection**: Contracts are globally discoverable
- **No test doubles**: Create test-specific processors
- **No complex setup**: Just register a different pipeline

Benefits:
1. **True isolation**: Test and production pipelines can't interfere
2. **Partial mocking**: Mix real and mock processors freely
3. **Multiple test modes**: Create different pipelines for different test scenarios
4. **Type safety**: Same compile-time guarantees in tests
5. **Simplicity**: Testing is just another pipeline configuration

Common testing patterns:
- **Unit tests**: Test individual processors or subsets
- **Integration tests**: Use hybrid pipelines with some real processors
- **Failure injection**: Create pipelines that always fail
- **Performance tests**: Strip out slow processors
- **Snapshot tests**: Save outputs for comparison

The same type universe concept that enables A/B testing in production makes testing elegant in development.

---

## 🚀 See It In Action!

Reading code is one thing, but seeing pipz in action is transformative. **Run the demos** - they're interactive, informative, and will help you truly understand the power of type-based pipelines:

```bash
# Start with all demos for the full experience
go run ./demo all -i  # Interactive mode with typewriter effects

# Or explore specific patterns
go run ./demo universes   # See type isolation in action
go run ./demo versioning  # Watch A/B/C testing with zero coupling
go run ./demo testing     # Experience testing without mocks
```

Each demo progressively builds your understanding, from basic pipelines to advanced architectural patterns. The demos aren't just examples - they're a journey through a new way of thinking about code organization.

**Trust us: 5 minutes with the demos will teach you more than an hour of documentation.**