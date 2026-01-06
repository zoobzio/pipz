# pipz

[![CI Status](https://github.com/zoobzio/pipz/workflows/CI/badge.svg)](https://github.com/zoobzio/pipz/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/pipz/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/pipz)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/pipz)](https://goreportcard.com/report/github.com/zoobzio/pipz)
[![CodeQL](https://github.com/zoobzio/pipz/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/pipz/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/pipz.svg)](https://pkg.go.dev/github.com/zoobzio/pipz)
[![License](https://img.shields.io/github/license/zoobzio/pipz)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/pipz)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/pipz)](https://github.com/zoobzio/pipz/releases)

Type-safe, composable data pipelines for Go.

Build processing pipelines from simple parts, compose them into complex flows, and get rich error context when things fail.

## One Interface To Rule Them All

Every primitive in pipz implements `Chainable[T]`:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
    Identity() Identity
    Schema() Node
    Close() error
}
```

**Processors** wrap your functions — the callback signature is the only difference:

```go
// Transform: pure function, no errors
enrich := pipz.Transform(EnrichID, func(ctx context.Context, o Order) Order {
    o.ProcessedAt = time.Now()
    return o
})

// Apply: fallible function
validate := pipz.Apply(ValidateID, func(ctx context.Context, o Order) (Order, error) {
    if o.Total <= 0 {
        return o, errors.New("invalid total")
    }
    return o, nil
})

// Effect: side effect, data passes through unchanged
notify := pipz.Effect(NotifyID, func(ctx context.Context, o Order) error {
    return sendNotification(o.ID)
})
```

**Connectors** compose processors — and each other:

```go
// Compose processors into a sequence
flow := pipz.NewSequence(FlowID, validate, enrich, notify)

// Wrap with resilience patterns
resilient := pipz.NewRetry(RetryID, flow, 3)
protected := pipz.NewTimeout(TimeoutID, resilient, 5*time.Second)

// Connectors nest freely — it's Chainable[T] all the way down
pipeline := pipz.NewCircuitBreaker(BreakerID, protected, 5, 30*time.Second)
```

## Install

```bash
go get github.com/zoobzio/pipz
```

Requires Go 1.24+.

## Quick Start

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "strings"
    "time"

    "github.com/zoobzio/pipz"
)

// Identities for debugging and observability
var (
    ValidateID = pipz.NewIdentity("validate", "Validates order totals")
    EnrichID   = pipz.NewIdentity("enrich", "Adds processing timestamp")
    FormatID   = pipz.NewIdentity("format", "Formats order ID")
    PipelineID = pipz.NewIdentity("order-flow", "Main order pipeline")
)

type Order struct {
    ID          string
    Total       float64
    ProcessedAt time.Time
}

func main() {
    ctx := context.Background()

    // Processors wrap functions
    validate := pipz.Apply(ValidateID, func(_ context.Context, o Order) (Order, error) {
        if o.Total <= 0 {
            return o, errors.New("invalid total")
        }
        return o, nil
    })

    enrich := pipz.Transform(EnrichID, func(_ context.Context, o Order) Order {
        o.ProcessedAt = time.Now()
        return o
    })

    format := pipz.Transform(FormatID, func(_ context.Context, o Order) Order {
        o.ID = strings.ToUpper(o.ID)
        return o
    })

    // Connectors compose processors
    pipeline := pipz.NewSequence(PipelineID, validate, enrich, format)

    // Process
    result, err := pipeline.Process(ctx, Order{ID: "order-123", Total: 99.99})
    if err != nil {
        var pipeErr *pipz.Error[Order]
        if errors.As(err, &pipeErr) {
            fmt.Printf("Failed at %s: %v\n", strings.Join(pipeErr.Path, "->"), pipeErr.Err)
        }
        return
    }

    fmt.Printf("Processed: %s at %v\n", result.ID, result.ProcessedAt)
}
```

## Capabilities

| Feature              | Description                                                                       | Docs                                                          |
| -------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| Uniform Interface    | Everything implements `Chainable[T]` for seamless composition                     | [Core Concepts](docs/2.learn/3.core-concepts.md)              |
| Type-Safe Generics   | Full compile-time checking with zero reflection                                   | [Architecture](docs/2.learn/4.architecture.md)                |
| Rich Error Context   | Path tracking, timestamps, and input capture on failure                           | [Safety & Reliability](docs/3.guides/6.safety-reliability.md) |
| Panic Recovery       | Automatic recovery with security-conscious sanitization                           | [Safety & Reliability](docs/3.guides/6.safety-reliability.md) |
| Signal Observability | State change events via [capitan](https://github.com/zoobzio/capitan) integration | [Hooks](docs/2.learn/5.hooks.md)                              |
| Pipeline Schemas     | `Schema()` exports structure for visualization and debugging                      | [Cheatsheet](docs/5.reference/1.cheatsheet.md)                |

## Why pipz?

- **Type-safe** — Full compile-time checking with generics
- **Composable** — Complex pipelines from simple parts
- **Minimal dependencies** — Standard library plus [clockz](https://github.com/zoobzio/clockz)
- **Observable** — Typed signals for state changes via [capitan](https://github.com/zoobzio/capitan)
- **Rich errors** — Full path tracking shows exactly where failures occur
- **Panic-safe** — Automatic recovery with security sanitization

## Composable Reliability

Use pipz directly to build secure, observable reliability patterns over your types:

```go
// Your domain type
type Order struct { ... }

// Wrap any operation with resilience
fetch := pipz.Apply(FetchID, fetchOrder)

reliable := pipz.NewSequence(ReliableID,
    pipz.NewRateLimiter[Order](LimiterID, 100, 10),      // throttle
    pipz.NewRetry(RetryID, fetch, 3),                    // retry on failure
    pipz.NewTimeout(TimeoutID, fetch, 5*time.Second),    // enforce deadline
    pipz.NewCircuitBreaker(BreakerID, fetch, 5, 30*time.Second), // prevent cascade
)

// Full error context when things fail
result, err := reliable.Process(ctx, order)
```

Every connector emits [capitan](https://github.com/zoobzio/capitan) signals — circuit breaker state changes, retry attempts, rate limit hits — observable without instrumentation code.

## Extensible Application Vocabulary

Fix T to a domain type and `Chainable[T]` becomes your API surface:

```go
// Library fixes T to a domain type
type File struct {
    Name     string
    Size     int64
    Data     []byte
    Metadata map[string]string
}

// Library provides domain-specific primitives
func Scan(scanner VirusScanner) pipz.Chainable[*File] { ... }
func Thumbnail(width, height int) pipz.Chainable[*File] { ... }
func Compress(quality int) pipz.Chainable[*File] { ... }
func Upload(storage Storage) pipz.Chainable[*File] { ... }

// Users extend with their own — same interface, first-class citizen
type Watermark struct {
    identity pipz.Identity
    logo     []byte
}
func (w *Watermark) Process(ctx context.Context, f *File) (*File, error) {
    f.Data = applyWatermark(f.Data, w.logo)
    return f, nil
}
func (w *Watermark) Identity() pipz.Identity { return w.identity }
func (w *Watermark) Schema() pipz.Node       { return pipz.Node{Identity: w.identity, Type: "processor"} }
func (w *Watermark) Close() error            { return nil }

// Everything composes — library primitives and user code, indistinguishable
pipeline := pipz.NewSequence(PipelineID,
    Scan(clamav),
    Thumbnail(800, 600),
    &Watermark{logo},  // user's primitive slots right in
    Compress(85),
    Upload(s3),
)
```

The built-in primitives are the base vocabulary. Users add their own words following the same grammar. The interface IS the API — implement it and express whatever you want.

## Documentation

- [Overview](docs/1.overview.md) — Design philosophy and architecture

### Learn

- [Quickstart](docs/2.learn/1.quickstart.md) — Build your first pipeline
- [Introduction](docs/2.learn/2.introduction.md) — What pipz is and why it exists
- [Core Concepts](docs/2.learn/3.core-concepts.md) — Processors, connectors, identity
- [Architecture](docs/2.learn/4.architecture.md) — Internal design and components
- [Hooks](docs/2.learn/5.hooks.md) — Signal-based observability

### Guides

- [Connector Selection](docs/3.guides/1.connector-selection.md) — Choosing the right connector
- [Cloning](docs/3.guides/2.cloning.md) — Data isolation for parallel processing
- [Best Practices](docs/3.guides/3.best-practices.md) — Patterns and recommendations
- [Testing](docs/3.guides/4.testing.md) — Testing pipelines
- [Performance](docs/3.guides/5.performance.md) — Optimization and benchmarking
- [Safety & Reliability](docs/3.guides/6.safety-reliability.md) — Error handling, panics, timeouts
- [Troubleshooting](docs/3.guides/7.troubleshooting.md) — Common issues and solutions

### Cookbook

- [Building Pipelines](docs/4.cookbook/1.building-pipelines.md) — Complete pipeline with validation, resilience, observability
- [Library Resilience](docs/4.cookbook/2.library-resilience.md) — Expose resilience patterns via functional options
- [Extensible Vocabulary](docs/4.cookbook/3.extensible-vocabulary.md) — Domain-specific APIs with composable primitives

### Reference

- [Cheatsheet](docs/5.reference/1.cheatsheet.md) — Quick reference for all primitives
- [Types](docs/5.reference/2.types/) — Error, Identity, Node, Schema

#### Processors

| Processor                                               | Purpose                                  |
| ------------------------------------------------------- | ---------------------------------------- |
| [Transform](docs/5.reference/3.processors/transform.md) | Pure transformation (no errors)          |
| [Apply](docs/5.reference/3.processors/apply.md)         | Transformation that may fail             |
| [Effect](docs/5.reference/3.processors/effect.md)       | Side effect, passes data through         |
| [Mutate](docs/5.reference/3.processors/mutate.md)       | Conditional modification                 |
| [Enrich](docs/5.reference/3.processors/enrich.md)       | Best-effort enhancement (errors ignored) |

#### Connectors

| Connector                                                         | Purpose                                     |
| ----------------------------------------------------------------- | ------------------------------------------- |
| [Sequence](docs/5.reference/4.connectors/sequence.md)             | Run in order                                |
| [Concurrent](docs/5.reference/4.connectors/concurrent.md)         | Run in parallel, collect all results        |
| [WorkerPool](docs/5.reference/4.connectors/workerpool.md)         | Bounded parallelism with fixed worker count |
| [Scaffold](docs/5.reference/4.connectors/scaffold.md)             | Fire-and-forget parallel execution          |
| [Fallback](docs/5.reference/4.connectors/fallback.md)             | Try primary, fall back on error             |
| [Race](docs/5.reference/4.connectors/race.md)                     | First success wins                          |
| [Contest](docs/5.reference/4.connectors/contest.md)               | First result meeting condition wins         |
| [Switch](docs/5.reference/4.connectors/switch.md)                 | Route based on conditions                   |
| [Filter](docs/5.reference/4.connectors/filter.md)                 | Conditional execution                       |
| [Retry](docs/5.reference/4.connectors/retry.md)                   | Retry on failure                            |
| [Backoff](docs/5.reference/4.connectors/backoff.md)               | Retry with exponential delays               |
| [Timeout](docs/5.reference/4.connectors/timeout.md)               | Enforce time limits                         |
| [Handle](docs/5.reference/4.connectors/handle.md)                 | Error recovery pipeline                     |
| [RateLimiter](docs/5.reference/4.connectors/ratelimiter.md)       | Token bucket rate limiting                  |
| [CircuitBreaker](docs/5.reference/4.connectors/circuitbreaker.md) | Prevent cascading failures                  |
| [Pipeline](docs/5.reference/4.connectors/pipeline.md)             | Execution context for tracing               |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. Run `make help` for available commands.

## License

MIT License — see [LICENSE](LICENSE) for details.
