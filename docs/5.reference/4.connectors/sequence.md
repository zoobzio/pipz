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
// Create a sequence
seq := pipz.NewSequence[Order]("order-processing")

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
// Create an empty sequence with a descriptive name
sequence := pipz.NewSequence[User]("user-registration")

// Register processors (they must already be created)
validateUser := pipz.Effect("validate", validateFunc)
enrichUser := pipz.Apply("enrich", enrichFunc)
saveUser := pipz.Apply("save", saveFunc)

sequence.Register(validateUser, enrichUser, saveUser)

// Or chain the calls
sequence = pipz.NewSequence[User]("user-pipeline").
    Register(validateUser, enrichUser, saveUser)
```

## Introspection

Sequences provide visibility into their structure:

```go
// Get processor names in order
names := sequence.Names()
// ["validate", "enrich", "save"]

// Find a specific processor
processor, err := sequence.Find("validate")
if err == nil {
    fmt.Printf("Found processor: %s\n", processor.Name())
}

// Get sequence length
count := sequence.Len()

// Check if empty
if sequence.IsEmpty() {
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
sequence.PushHead(authProcessor)

// Add multiple to beginning
sequence.PushHead(preprocess, authenticate)

// Add to end explicitly
sequence.PushTail(postprocess)

// Insert at specific position
err := sequence.InsertAt(2, authzProcessor)
```

### Removing Processors

```go
// Remove from end
processor, err := sequence.PopTail()

// Remove from beginning
processor, err := sequence.PopHead()

// Remove at index
err := sequence.RemoveAt(1)

// Clear all
sequence.Clear()
```

### Reordering

```go
// Move to head
err := sequence.MoveToHead(2)

// Move to tail
err := sequence.MoveToTail(1)

// Move to specific position
err := sequence.MoveTo(1, 3)

// Swap positions
err := sequence.Swap(0, 2)

// Reverse order
sequence.Reverse()
```

### Replacing

```go
// Replace processor at index
newTransform := pipz.Transform("transform_v2", transformFunc)
err := sequence.ReplaceAt(1, newTransform)
```

## Using Sequences with Other Connectors

Sequences implement `Chainable[T]`, so they can be used anywhere a processor is expected:

```go
// Create sub-sequences
validation := pipz.NewSequence[Order]("validation", checkItems, checkPayment)
processing := pipz.NewSequence[Order]("processing", calculateTax, applyDiscount)

// Combine in a parent sequence
main := pipz.NewSequence[Order]("main")
main.Register(
    validation,   // Already implements Chainable[T]
    processing,   // Already implements Chainable[T]
    saveOrder,
)

// Or use in other connectors
withRetry := pipz.NewRetry("reliable-processing", main, 3)
```

## Use Cases

### Feature Flags

```go
sequence := pipz.NewSequence[Request]("api-handler")
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
    seq := pipz.NewSequence[Order]("order-flow-" + variant)
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
sequence := pipz.NewSequence[Data]("data-processor")
sequence.Register(transform)

if debugMode {
    // Insert logging after transform
    debugLog := pipz.Effect("debug", logFunc)
    sequence.InsertAt(1, debugLog)
    sequence.PushTail(debugLog) // Also log at end
}
```

### Plugin Systems

```go
// Core sequence
sequence := pipz.NewSequence[Event]("event-handler")
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
- Introspection capabilities (Names, Find, Len)
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
2. **Check errors** - Modification methods return errors for invalid indices
3. **Use Link() carefully** - It returns the sequence as a Chainable, hiding modification methods
4. **Don't over-modify** - If you're constantly changing the sequence, consider using Switch instead
5. **Test modifications** - Ensure your dynamic changes work as expected

## Next Steps

- [Error Handling](../../3.guides/8.safety-reliability.md) - Handle failures in sequences
- [Connector Selection](../../3.guides/3.connector-selection.md) - Choose the right connector for your use case
- [Testing](../../3.guides/6.testing.md) - Test dynamic sequences