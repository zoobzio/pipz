---
title: "Pipeline"
description: "Wraps a Chainable with execution context for distributed tracing and observability"
author: zoobzio
published: 2025-12-26
updated: 2025-12-26
tags:
  - reference
  - connectors
  - pipeline
  - tracing
  - observability
  - correlation
---

# Pipeline

Wraps a Chainable with a semantic execution context for distributed tracing and observability.

## Function Signature

```go
func NewPipeline[T any](
    identity Identity,
    root Chainable[T],
) *Pipeline[T]
```

## Parameters

- `identity` (`Identity`) - Semantic identity for the pipeline, used for correlation across executions
- `root` (`Chainable[T]`) - The chainable to wrap with execution context

## Returns

Returns a `*Pipeline[T]` that implements `Chainable[T]`.

## Behavior

- **Execution ID injection** - Each `Process()` call generates a unique execution UUID
- **Pipeline ID injection** - The pipeline's identity ID is injected into context
- **Context propagation** - Both IDs flow through to all nested chainables
- **Transparent delegation** - Processing is delegated to the root chainable

## Context Extraction

Extract IDs from context in signal handlers or custom processors:

```go
// Extract execution ID (unique per Process() call)
if execID, ok := pipz.ExecutionIDFromContext(ctx); ok {
    // Use for tracing, logging, metrics...
}

// Extract pipeline ID (stable across executions)
if pipeID, ok := pipz.PipelineIDFromContext(ctx); ok {
    // Use for correlation, grouping...
}
```

## Example

```go
// Define identities
var (
    OrderPipelineID = pipz.NewIdentity("order-processing", "Main order processing flow")
    ValidateID      = pipz.NewIdentity("validate", "Validates order data")
    EnrichID        = pipz.NewIdentity("enrich", "Enriches order with customer data")
    SaveID          = pipz.NewIdentity("save", "Persists order to database")
    InternalSeqID   = pipz.NewIdentity("order-steps", "Internal processing sequence")
)

// Build the processing logic
sequence := pipz.NewSequence(InternalSeqID,
    pipz.Apply(ValidateID, validateOrder),
    pipz.Apply(EnrichID, enrichOrder),
    pipz.Apply(SaveID, saveOrder),
)

// Wrap with Pipeline for execution context
pipeline := pipz.NewPipeline(OrderPipelineID, sequence)

// Process - execution ID generated automatically
result, err := pipeline.Process(ctx, order)
```

## When to Use

Use `Pipeline` when:
- **Distributed tracing** - Correlating signals across pipeline execution
- **Observability** - Tracking execution runs in monitoring systems
- **Debugging** - Associating logs with specific pipeline invocations
- **Metrics** - Grouping performance data by pipeline and execution

## When NOT to Use

Don't use `Pipeline` when:
- Simple pipelines without tracing needs
- Performance-critical paths where context overhead matters
- You're not consuming execution/pipeline IDs anywhere

## Integration with Signals

Connectors emit signals with context. Use signal handlers to extract IDs:

```go
capitan.Hook(pipz.SignalCircuitBreakerOpened, func(ctx context.Context, e *capitan.Event) {
    execID, _ := pipz.ExecutionIDFromContext(ctx)
    pipeID, _ := pipz.PipelineIDFromContext(ctx)

    // Log with correlation
    log.Printf("Circuit opened in pipeline %s, execution %s", pipeID, execID)

    // Send to tracing system
    span.SetAttribute("pipz.execution_id", execID.String())
    span.SetAttribute("pipz.pipeline_id", pipeID.String())
})
```

## Schema

Pipeline appears in schema with type `"pipeline"` and a `PipelineFlow`:

```go
schema := pipeline.Schema()
// schema.Type == "pipeline"
// schema.Identity.Name() == "order-processing"

if flow, ok := pipz.PipelineKey.From(schema); ok {
    // flow.Root contains the wrapped chainable's schema
}
```

## Common Patterns

```go
// Multiple pipelines sharing processors
var (
    SyncPipelineID  = pipz.NewIdentity("sync-orders", "Synchronous order processing")
    AsyncPipelineID = pipz.NewIdentity("async-orders", "Async order processing")
)

// Same processing, different execution contexts
syncPipeline := pipz.NewPipeline(SyncPipelineID, orderSequence)
asyncPipeline := pipz.NewPipeline(AsyncPipelineID, orderSequence)

// Nested pipelines (outer wins)
var (
    OuterID = pipz.NewIdentity("outer", "Outer pipeline")
    InnerID = pipz.NewIdentity("inner", "Inner pipeline")
)

// Inner pipeline's context injection is overwritten by outer
outer := pipz.NewPipeline(OuterID,
    pipz.NewPipeline(InnerID, processor), // Inner IDs ignored
)
// All signals will have OuterID as pipeline ID
```

## See Also

- [Sequence](./sequence.md) - Common root for pipelines
- [Hooks](../../2.learn/5.hooks.md) - Signal-based observability
