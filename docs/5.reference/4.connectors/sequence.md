---
title: "Sequences"
description: "Mutable, managed chains of processors with introspection and dynamic modification capabilities"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - sequential
  - dynamic
  - pipeline
---

# Sequences

Sequences are mutable, managed chains of processors that provide introspection, modification, and advanced control over processor execution.

## Understanding Sequence

`Sequence` is the primary way to build sequential pipelines in pipz. Unlike simple processor composition, Sequence offers dynamic management:

```go
// Define identities
var (
    OrderProcessingID = pipz.NewIdentity("order-processing", "sequential order processing pipeline")
)

// Create a sequence
seq := pipz.NewSequence(OrderProcessingID)

// Register processors
seq.Register(
    validateOrder,
    calculateTax,
    applyDiscount,
    saveOrder,
)

// Process data
result, err := seq.Process(ctx, order)
```

## Creating Sequences

```go
// Define identities upfront
var (
    UserRegistrationID = pipz.NewIdentity("user-registration", "user registration workflow")
    ValidateUserID     = pipz.NewIdentity("validate", "validate user data")
    EnrichUserID       = pipz.NewIdentity("enrich", "enrich user profile")
    SaveUserID         = pipz.NewIdentity("save", "save user to database")
    UserPipelineID     = pipz.NewIdentity("user-pipeline", "complete user processing pipeline")
)

// Create an empty sequence with a descriptive name
sequence := pipz.NewSequence(UserRegistrationID)

// Register processors (they must already be created)
validateUser := pipz.Effect(ValidateUserID, validateFunc)
enrichUser := pipz.Apply(EnrichUserID, enrichFunc)
saveUser := pipz.Apply(SaveUserID, saveFunc)

sequence.Register(validateUser, enrichUser, saveUser)

// Or chain the calls
sequence = pipz.NewSequence(UserPipelineID).Register(validateUser, enrichUser, saveUser)
```

## Introspection

Sequences provide visibility into their structure:

```go
// Get processor names in order
names := sequence.Names()
// ["validate", "enrich", "save"]

// Get sequence length
count := sequence.Len()

// Check if empty
if sequence.Len() == 0 {
    fmt.Println("No processors registered")
}
```

## Dynamic Modification

Sequences can be modified at runtime - this is their key advantage:

### Adding Processors

```go
// Add to end (most common)
sequence.Register(auditProcessor)

// Add to beginning
sequence.Unshift(authProcessor)

// Add multiple to beginning
sequence.Unshift(preprocess, authenticate)

// Add to end explicitly
sequence.Push(postprocess)

// Insert after a specific processor (by Identity)
err := sequence.After(validateID, authzProcessor)

// Insert before a specific processor (by Identity)
err := sequence.Before(saveID, cacheProcessor)
```

### Removing Processors

```go
// Remove from end
processor, err := sequence.Pop()

// Remove from beginning
processor, err := sequence.Shift()

// Remove by Identity
err := sequence.Remove(validateID)

// Clear all
sequence.Clear()
```

### Replacing

```go
// Define identities upfront
var (
    TransformV2ID = pipz.NewIdentity("transform_v2", "Updated transform logic")
)

// Replace processor by Identity
newTransform := pipz.Transform(TransformV2ID, transformFunc)
err := sequence.Replace(transformID, newTransform)
```

## Using Sequences with Other Connectors

Sequences implement `Chainable[T]`, so they can be used anywhere a processor is expected:

```go
// Define identities upfront
var (
    ValidationID         = pipz.NewIdentity("validation", "validate order items and payment")
    ProcessingID         = pipz.NewIdentity("processing", "calculate tax and apply discounts")
    MainID               = pipz.NewIdentity("main", "main order processing pipeline")
    ReliableProcessingID = pipz.NewIdentity("reliable-processing", "retry main pipeline on failure")
)

// Create sub-sequences
validation := pipz.NewSequence(ValidationID, checkItems, checkPayment)
processing := pipz.NewSequence(ProcessingID, calculateTax, applyDiscount)

// Combine in a parent sequence
main := pipz.NewSequence(MainID)
main.Register(
    validation,   // Already implements Chainable[T]
    processing,   // Already implements Chainable[T]
    saveOrder,
)

// Or use in other connectors
withRetry := pipz.NewRetry(ReliableProcessingID, main, 3)
```

## Use Cases

### Feature Flags

```go
// Define identities upfront
var (
    APIHandlerID = pipz.NewIdentity("api-handler", "API request handler with feature flag support")
)

sequence := pipz.NewSequence(APIHandlerID)
sequence.Register(authenticate, validate)

if featureFlags.IsEnabled("new-enrichment") {
    sequence.Register(enrichDataV2)
} else {
    sequence.Register(enrichDataV1)
}

sequence.Register(process)
```

### A/B Testing

```go
func createSequence(variant string) *Sequence[Order] {
    // Define identity with variant
    orderFlowID := pipz.NewIdentity("order-flow-"+variant, "order flow for A/B test variant "+variant)

    seq := pipz.NewSequence(orderFlowID)
    seq.Register(validateOrder)

    switch variant {
    case "A":
        seq.Register(standardPricing)
    case "B":
        seq.Register(dynamicPricing)
    }

    seq.Register(fulfillOrder)
    return seq
}
```

### Debug Mode

```go
// Define identities upfront
var (
    TransformID     = pipz.NewIdentity("transform", "transform data")
    DataProcessorID = pipz.NewIdentity("data-processor", "data processor with optional debug logging")
    DebugLogID      = pipz.NewIdentity("debug", "log debug information")
)

transform := pipz.Transform(TransformID, transformFunc)

sequence := pipz.NewSequence(DataProcessorID)
sequence.Register(transform)

if debugMode {
    // Insert logging after transform
    debugLog := pipz.Effect(DebugLogID, logFunc)
    sequence.After(TransformID, debugLog)
    sequence.Push(debugLog) // Also log at end
}
```

### Plugin Systems

```go
// Define identities upfront
var (
    EventHandlerID = pipz.NewIdentity("event-handler", "event handler with dynamic plugin support")
)

// Core sequence
sequence := pipz.NewSequence(EventHandlerID)
sequence.Register(parseEvent, validateEvent)

// Load plugins
for _, plugin := range loadPlugins() {
    sequence.Register(plugin.Processor())
}

sequence.Register(dispatchEvent)
```

## Thread Safety

Sequences ARE thread-safe. All modification methods use internal locking:

```go
// Safe to use concurrently
go func() {
    sequence.Register(newProcessor)
}()

go func() {
    result, err := sequence.Process(ctx, data)
}()
```

The internal `sync.RWMutex` ensures:
- Multiple concurrent Process calls can execute
- Modifications lock out all access temporarily
- No race conditions or data corruption

## Sequence vs Direct Composition

Use Sequence when you need:
- Runtime modification of processing steps
- Introspection capabilities (Names, Len)
- Plugin architectures
- Feature flag integration
- Debug instrumentation
- A/B testing flows

Use direct processor composition when:
- Pipeline is fixed at compile time
- Maximum performance is critical
- Simplicity is preferred

## Example: Dynamic ETL Pipeline

```go
type ETLProcessor struct {
    sequence *pipz.Sequence[Record]
}

func (etl *ETLProcessor) Configure(cfg Config) {
    etl.sequence.Clear()
    
    // Always start with validation
    etl.sequence.Register(validateRecord)
    
    // Add transformations based on config
    for _, transform := range cfg.Transformations {
        etl.sequence.Register(createTransform(transform))
    }
    
    // Conditional enrichment
    if cfg.EnableEnrichment {
        etl.sequence.Register(enrichRecord)
    }
    
    // Output varies by destination
    switch cfg.Destination {
    case "database":
        etl.sequence.Register(writeToDatabase)
    case "file":
        etl.sequence.Register(writeToFile)
    case "api":
        etl.sequence.Register(postToAPI)
    }
}

func (etl *ETLProcessor) Process(ctx context.Context, record Record) (Record, error) {
    return etl.sequence.Process(ctx, record)
}
```

## Best Practices

1. **Name your sequences** - The name appears in error paths for debugging
2. **Store Identity references** - Keep references to processor Identities so you can use Remove, Replace, After, and Before
3. **Check errors** - Modification methods return errors when processors aren't found
4. **Don't over-modify** - If you're constantly changing the sequence, consider using Switch instead
5. **Test modifications** - Ensure your dynamic changes work as expected

## Next Steps

- [Error Handling](../../3.guides/6.safety-reliability.md) - Handle failures in sequences
- [Connector Selection](../../3.guides/1.connector-selection.md) - Choose the right connector for your use case
- [Testing](../../3.guides/4.testing.md) - Test dynamic sequences