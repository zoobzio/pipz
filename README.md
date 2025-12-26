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

## One Interface

Everything in pipz implements `Chainable[T]`:

```go
type Chainable[T any] interface {
    Process(context.Context, T) (T, error)
    Identity() Identity
    Schema() Node
    Close() error
}
```

**Processors** wrap your functions:

```go
validate := pipz.Apply(ValidateID, validateOrder)    // can fail
enrich := pipz.Transform(EnrichID, addTimestamp)     // pure transform
notify := pipz.Effect(NotifyID, sendAlert)           // side effect
```

**Connectors** compose them:

```go
pipeline := pipz.NewSequence(PipelineID, validate, enrich, notify)
resilient := pipz.NewFallback(FetchID, primaryDB, replicaDB)
protected := pipz.NewCircuitBreaker(BreakerID, externalAPI, 5, 30*time.Second)
```

Both implement `Chainable[T]`, so connectors nest freely—wrap a sequence in a timeout, add retry around a circuit breaker, compose without limits.

## Install

```bash
go get github.com/zoobzio/pipz
```

Requires Go 1.21+.

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

## Processors and Connectors

**Processors** wrap functions into `Chainable[T]`:

| Processor | Purpose |
|-----------|---------|
| `Transform` | Pure transformation (no errors) |
| `Apply` | Transformation that may fail |
| `Effect` | Side effect, passes data through |
| `Mutate` | Conditional modification |
| `Enrich` | Best-effort enhancement (errors ignored) |

**Connectors** compose any `Chainable[T]`:

| Connector | Purpose |
|-----------|---------|
| `Sequence` | Run in order |
| `Concurrent` | Run in parallel, collect all results |
| `WorkerPool` | Bounded parallelism with fixed worker count |
| `Scaffold` | Fire-and-forget parallel execution |
| `Fallback` | Try primary, fall back on error |
| `Race` | First success wins |
| `Contest` | First result meeting condition wins |
| `Switch` | Route based on conditions |
| `Filter` | Conditional execution |
| `Retry` / `Backoff` | Retry with optional delays |
| `Timeout` | Enforce time limits |
| `Handle` | Error recovery pipeline |
| `RateLimiter` | Token bucket rate limiting |
| `CircuitBreaker` | Prevent cascading failures |
| `Pipeline` | Execution context for tracing |

## Custom Implementations

Implement `Chainable[T]` directly for full control:

```go
type RateLimiter[T any] struct {
    identity pipz.Identity
    limiter  *rate.Limiter
}

func (r *RateLimiter[T]) Process(ctx context.Context, data T) (T, error) {
    if err := r.limiter.Wait(ctx); err != nil {
        return data, fmt.Errorf("rate limit: %w", err)
    }
    return data, nil
}

func (r *RateLimiter[T]) Identity() pipz.Identity { return r.identity }
func (r *RateLimiter[T]) Schema() pipz.Node       { return pipz.Node{Identity: r.identity, Type: "processor"} }
func (r *RateLimiter[T]) Close() error            { return nil }

// Use alongside built-in processors
pipeline := pipz.NewSequence(FlowID,
    pipz.Apply(ValidateID, validateFn),
    &RateLimiter[Order]{identity: LimiterID, limiter: limiter},
    pipz.Transform(FormatID, formatFn),
)
```

## Why pipz?

- **Type-safe** — Full compile-time checking with generics
- **Composable** — Complex pipelines from simple parts
- **Minimal dependencies** — Standard library plus [clockz](https://github.com/zoobzio/clockz)
- **Observable** — Typed signals for state changes via [capitan](https://github.com/zoobzio/capitan)
- **Rich errors** — Full path tracking shows exactly where failures occur
- **Panic-safe** — Automatic recovery with security sanitization

## Documentation

- [Overview](docs/1.overview.md) — Design philosophy and architecture
- **Learn**
  - [Quickstart](docs/2.learn/1.quickstart.md) — Build your first pipeline
  - [Core Concepts](docs/2.learn/3.core-concepts.md) — Processors, connectors, identity
  - [Hooks](docs/2.learn/5.hooks.md) — Signal-based observability
- **Guides**
  - [Connector Selection](docs/3.guides/3.connector-selection.md) — Choosing the right connector
  - [Testing](docs/3.guides/6.testing.md) — Testing pipelines
  - [Safety & Reliability](docs/3.guides/8.safety-reliability.md) — Error handling, panics, timeouts
- **Cookbook**
  - [Resilient API Calls](docs/4.cookbook/1.resilient-api-calls.md)
  - [ETL Pipelines](docs/4.cookbook/4.etl-pipelines.md)
  - [Patterns](docs/4.cookbook/6.patterns.md)
- **Reference**
  - [Cheatsheet](docs/5.reference/1.cheatsheet.md)
  - [Processors](docs/5.reference/3.processors/)
  - [Connectors](docs/5.reference/4.connectors/)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
make test    # Run tests
make lint    # Run linter
make bench   # Run benchmarks
```

## License

MIT License — see [LICENSE](LICENSE) for details.
