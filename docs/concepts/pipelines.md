# Pipelines

Pipelines are the heart of pipz - they represent chains of processors working together to transform data through multiple stages. A pipeline is not a specific type, but rather the conceptual pattern of composing small, focused processors into larger workflows.

## What is a Pipeline?

A pipeline is any composition of processors that:
- Takes input data of type T
- Processes it through one or more stages
- Returns transformed data (still type T) or an error
- Maintains type safety throughout

In pipz, everything that implements `Chainable[T]` can be part of a pipeline:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, *Error[T])
    Name() Name
}
```

## Building Pipelines

There are several ways to build pipelines in pipz:

### Simple Composition

For fixed pipelines, compose processors directly:

```go
// Individual processors
validate := pipz.Effect("validate", validateFunc)
enrich := pipz.Apply("enrich", enrichFunc)
save := pipz.Apply("save", saveFunc)

// Compose into a pipeline using Sequence (single line)
pipeline := pipz.NewSequence("user-pipeline", validate, enrich, save)

// Or build dynamically
pipeline := pipz.NewSequence[User]("user-pipeline")
pipeline.Register(validate, enrich, save)
```

### Complex Flows

Use connectors to create sophisticated processing patterns:

```go
// Parallel processing
parallel := pipz.NewConcurrent("parallel-enrichment",
    fetchUserProfile,
    fetchUserPreferences,
    fetchUserHistory,
)

// Conditional routing
router := pipz.NewSwitch("payment-router", 
    detectPaymentType,
).AddRoute("credit", processCreditCard).
  AddRoute("crypto", processCrypto)

// Error handling
reliable := pipz.NewRetry("reliable-save",
    pipz.NewTimeout("timeout-save", saveToDatabase, 5*time.Second),
    3,
)
```

## Pipeline Patterns

### Linear Pipeline

The most common pattern - data flows through each stage sequentially:

```go
// Order processing pipeline
orderPipeline := pipz.NewSequence("order-flow",
    validateOrder,
    checkInventory,
    calculatePricing,
    applyDiscounts,
    chargePayment,
    createShipment,
    sendConfirmation,
)
```

### Branching Pipeline

Different paths based on data characteristics:

```go
// User registration with different flows
userRouter := pipz.NewSwitch("user-type", detectUserType)
userRouter.AddRoute("individual", processIndividual)
userRouter.AddRoute("business", processBusiness)
userRouter.AddRoute("enterprise", processEnterprise)

registration := pipz.NewSequence("registration",
    validateInput,
    userRouter,
    createAccount,
    sendWelcomeEmail,
    )
```

### Resilient Pipeline

Built-in error handling and recovery:

```go
// API integration with fallbacks
apiPipeline := pipz.NewSequence[Request]("api-handler").
    Register(
        authenticate,
        validateRequest,
        pipz.NewFallback("data-fetch",
            pipz.NewTimeout("primary-api", fetchFromPrimary, 2*time.Second),
            pipz.NewRetry("backup-api", fetchFromBackup, 3),
        ),
        transformResponse,
        cacheResult,
    )
```

### Parallel Pipeline

Process independent operations concurrently:

```go
// Document processing with parallel analysis
// Note: Document must implement Cloner[Document]
docPipeline := pipz.NewSequence[Document]("doc-processor").
    Register(
        parseDocument,
        pipz.NewConcurrent("analysis",
            extractText,
            analyzeSentiment,
            detectLanguage,
            extractEntities,
        ),
        generateSummary,
        storeResults,
    )
```

## Pipeline Philosophy

### Composition Over Configuration

Instead of complex configuration files, pipelines are built by composing simple functions:

```go
// Not this:
// config.yaml with 100 lines of pipeline configuration

// But this:
pipeline := pipz.NewSequence[Data]("my-pipeline").
    Register(step1, step2, step3)
```

### Type Safety Throughout

The pipeline maintains type safety from input to output:

```go
// This won't compile - type mismatch
userPipeline := pipz.NewSequence[User]("users")
userPipeline.Register(
    validateUser,           // Chainable[User] ✓
    processOrder,          // Chainable[Order] ✗ Compile error!
)
```

### Fail-Fast with Rich Errors

Pipelines stop at the first error, providing detailed context:

```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[MyType]
    if errors.As(err, &pipeErr) {
        // Full path: ["pipeline", "validate", "parse_json"]
        log.Printf("Failed at: %v", pipeErr.Path)
        log.Printf("Input data: %+v", pipeErr.InputData)
    }
}
```

### Context-Aware

Every pipeline stage receives a context for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := pipeline.Process(ctx, data)
```

## Testing Pipelines

Pipelines are inherently testable because they're built from small, focused units:

```go
// Test individual processors
func TestValidateUser(t *testing.T) {
    validator := pipz.Effect("validate", validateUser)
    err := validator.Process(context.Background(), testUser)
    // assertions...
}

// Test complete pipeline
func TestUserPipeline(t *testing.T) {
    pipeline := createUserPipeline()
    result, err := pipeline.Process(context.Background(), testUser)
    // assertions...
}

// Test with mocks
func TestPipelineWithMocks(t *testing.T) {
    pipeline := pipz.NewSequence[User]("test").
        Register(
            validateUser,           // Real validator
            mockEnricher,          // Mock for external service
            pipz.Effect("assert", func(ctx context.Context, u User) error {
                // Assertions on transformed data
                assert.Equal(t, "enriched", u.Status)
                return nil
            }),
        )
}
```

## Best Practices

1. **Keep processors focused** - Each should do one thing well
2. **Name everything** - Names appear in errors for easy debugging
3. **Use the right connector** - Sequence for simple flows, specialized connectors for complex patterns
4. **Handle errors at pipeline level** - Let errors bubble up with context
5. **Test at multiple levels** - Unit test processors, integration test pipelines
6. **Consider performance** - Parallel processing requires Cloner[T] implementation
7. **Document intent** - Pipeline structure should tell the story of your data flow

## Common Use Cases

- **API Middleware** - Request validation → Authentication → Rate limiting → Processing → Response formatting
- **ETL Pipelines** - Extract → Validate → Transform → Enrich → Load
- **Event Processing** - Parse → Filter → Transform → Route → Persist
- **Data Validation** - Schema check → Business rules → Sanitization → Normalization
- **Payment Processing** - Validate → Fraud check → Process → Notify → Audit

## Next Steps

- [Processors](./processors.md) - Building blocks of pipelines
- [Connectors](./connectors.md) - Ways to compose processors
- [Error Handling](./error-handling.md) - Handling failures in pipelines
- [Type Safety](./type-safety.md) - How pipz maintains type safety