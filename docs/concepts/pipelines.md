# Pipelines

Pipelines are managed sequences of processors that provide introspection, modification, and advanced control over processor execution.

## Pipeline vs Sequential

While `Sequential` creates a fixed chain of processors, `Pipeline` offers dynamic management:

```go
// Sequential - fixed at creation
seq := pipz.Sequential(validate, transform, save)

// Pipeline - dynamic and introspectable  
pipeline := pipz.NewPipeline[Order]()
pipeline.Register("validate", validate)
pipeline.Register("transform", transform)
pipeline.Register("save", save)
```

## Creating Pipelines

```go
// Create an empty pipeline
pipeline := pipz.NewPipeline[User]()

// Register processors
pipeline.Register("validate", validateUser)
pipeline.Register("enrich", enrichUser)
pipeline.Register("save", saveUser)

// Process data
result, err := pipeline.Process(ctx, user)
```

## Introspection

Pipelines provide visibility into their structure:

```go
// Get processor names in order
names := pipeline.Names()
// ["validate", "enrich", "save"]

// Check if processor exists
if pipeline.Find("validate") != nil {
    fmt.Println("Pipeline has validation")
}

// Get pipeline length
count := pipeline.Len()

// Check if empty
if pipeline.IsEmpty() {
    fmt.Println("No processors registered")
}
```

## Dynamic Modification

Pipelines can be modified at runtime:

### Adding Processors

```go
// Add to end
pipeline.PushTail("audit", auditProcessor)

// Add to beginning  
pipeline.PushHead("authenticate", authProcessor)

// Insert at specific position
pipeline.InsertAt(2, "authorize", authzProcessor)
```

### Removing Processors

```go
// Remove from end
removed := pipeline.PopTail()

// Remove from beginning
removed := pipeline.PopHead()

// Remove at index
pipeline.RemoveAt(1)

// Clear all
pipeline.Clear()
```

### Reordering

```go
// Move to head
pipeline.MoveToHead(2)

// Move to tail
pipeline.MoveToTail(1)

// Move to specific position
pipeline.MoveTo(1, 3)

// Swap positions
pipeline.Swap(0, 2)

// Reverse order
pipeline.Reverse()
```

### Replacing

```go
// Replace processor at index
pipeline.ReplaceAt(1, "transform_v2", newTransform)
```

## Linking Pipelines

Pipelines can be composed:

```go
// Create sub-pipelines
validation := pipz.NewPipeline[Order]()
validation.Register("check_items", checkItems)
validation.Register("check_payment", checkPayment)

processing := pipz.NewPipeline[Order]()
processing.Register("calculate_tax", calculateTax)
processing.Register("apply_discount", applyDiscount)

// Link them together
main := pipz.NewPipeline[Order]()
main.Link(validation)
main.Link(processing)
main.Register("save", saveOrder)
```

## Use Cases

### Feature Flags

```go
pipeline := pipz.NewPipeline[Request]()
pipeline.Register("auth", authenticate)
pipeline.Register("validate", validate)

if featureFlags.IsEnabled("new-enrichment") {
    pipeline.Register("enrich", enrichDataV2)
} else {
    pipeline.Register("enrich", enrichDataV1)
}

pipeline.Register("process", process)
```

### A/B Testing

```go
func createPipeline(variant string) *pipz.Pipeline[Order] {
    p := pipz.NewPipeline[Order]()
    p.Register("validate", validateOrder)
    
    switch variant {
    case "A":
        p.Register("price", standardPricing)
    case "B":
        p.Register("price", dynamicPricing)
    }
    
    p.Register("fulfill", fulfillOrder)
    return p
}
```

### Debug Mode

```go
pipeline := pipz.NewPipeline[Data]()
pipeline.Register("transform", transform)

if debugMode {
    // Insert logging between each step
    pipeline.InsertAt(1, "log_after_transform", debugLogger)
    pipeline.PushTail("final_log", debugLogger)
}
```

### Plugin Systems

```go
// Core pipeline
pipeline := pipz.NewPipeline[Event]()
pipeline.Register("parse", parseEvent)
pipeline.Register("validate", validateEvent)

// Load plugins
for _, plugin := range loadPlugins() {
    pipeline.Register(plugin.Name(), plugin.Processor())
}

pipeline.Register("dispatch", dispatchEvent)
```

## Thread Safety

Pipelines are NOT thread-safe for modifications. If you need concurrent access:

```go
type SafePipeline[T any] struct {
    pipeline *pipz.Pipeline[T]
    mu       sync.RWMutex
}

func (sp *SafePipeline[T]) Process(ctx context.Context, data T) (T, error) {
    sp.mu.RLock()
    defer sp.mu.RUnlock()
    return sp.pipeline.Process(ctx, data)
}

func (sp *SafePipeline[T]) Register(name string, proc pipz.Chainable[T]) {
    sp.mu.Lock()
    defer sp.mu.Unlock()
    sp.pipeline.Register(name, proc)
}
```

## Pipeline vs Connectors

Use Pipeline when you need:
- Runtime modification
- Introspection capabilities
- Plugin architectures
- Feature flag integration
- Debug instrumentation

Use Connectors when you need:
- Maximum performance
- Compile-time guarantees
- Simple, fixed workflows
- Functional composition

## Example: Dynamic ETL Pipeline

```go
type ETLPipeline struct {
    pipeline *pipz.Pipeline[Record]
    config   Config
}

func (etl *ETLPipeline) Configure(cfg Config) {
    etl.pipeline.Clear()
    
    // Always start with validation
    etl.pipeline.Register("validate", validateRecord)
    
    // Add transformations based on config
    for _, transform := range cfg.Transformations {
        etl.pipeline.Register(
            transform.Name,
            createTransform(transform),
        )
    }
    
    // Conditional enrichment
    if cfg.EnableEnrichment {
        etl.pipeline.Register("enrich", enrichRecord)
    }
    
    // Output varies by destination
    switch cfg.Destination {
    case "database":
        etl.pipeline.Register("output", writeToDatabase)
    case "file":
        etl.pipeline.Register("output", writeToFile)
    case "api":
        etl.pipeline.Register("output", postToAPI)
    }
}
```

## Next Steps

- [Error Handling](./error-handling.md) - Handle failures in pipelines
- [Best Practices](../guides/best-practices.md) - Pipeline design patterns
- [Testing](../guides/testing.md) - Test dynamic pipelines