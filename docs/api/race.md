# Race

Runs processors in parallel and returns the first successful result.

## Function Signature

```go
func NewRace[T Cloner[T]](name Name, processors ...Chainable[T]) *Race[T]
```

## Type Constraints

- `T` must implement the `Cloner[T]` interface:
  ```go
  type Cloner[T any] interface {
      Clone() T
  }
  ```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `processors` - Variable number of processors to race

## Returns

Returns a `*Race[T]` that implements `Chainable[T]`.

## Behavior

- **Parallel execution** - All processors start simultaneously
- **First wins** - Returns the first successful result
- **Cancellation** - Cancels remaining processors when one succeeds
- **All fail = error** - Only fails if all processors fail
- **Returns clone** - Winner's result is returned
- **Context preservation** - Uses `context.WithCancel(ctx)` to preserve trace context while enabling cancellation of losing processors

## Example

```go
// Race multiple data sources
fetchData := pipz.NewRace("fetch-fastest",
    pipz.Apply("cache", fetchFromCache),
    pipz.Apply("primary-db", fetchFromPrimary),
    pipz.Apply("replica-db", fetchFromReplica),
    pipz.Apply("api-fallback", fetchFromAPI),
)

// Race different processing strategies
processImage := pipz.NewRace("image-processor",
    pipz.Apply("gpu", processWithGPU),
    pipz.Apply("cpu-optimized", processWithSIMD),
    pipz.Apply("cpu-standard", processWithCPU),
)

// Race external services
translateText := pipz.NewRace("translate",
    pipz.Apply("google", translateWithGoogle),
    pipz.Apply("deepl", translateWithDeepL),
    pipz.Apply("azure", translateWithAzure),
)
```

## When to Use

Use `Race` when:
- You have multiple ways to get the same result
- You want the fastest response
- You have primary/backup services
- Latency is critical
- Any successful result is acceptable

## When NOT to Use

Don't use `Race` when:
- You need all operations to complete (use `Concurrent`)
- Results might differ between processors
- Order matters (use `Sequence`)
- You need the specific source of the result

## Error Handling

Race only fails if all processors fail:

```go
race := pipz.NewRace("multi-fetch",
    pipz.Apply("fast", fastButFlaky),     // Fails 50% of time
    pipz.Apply("slow", slowButReliable),  // Takes 5 seconds
    pipz.Apply("backup", backupService),  // Last resort
)

// Returns first success or error if all fail
result, err := race.Process(ctx, input)
if err != nil {
    // All three processors failed
    var raceErr *pipz.Error[Data]
    if errors.As(err, &raceErr) {
        // Error from the last processor to fail
        fmt.Printf("All processors failed: %v", raceErr)
    }
}
```

## Performance Considerations

- Creates one goroutine per processor
- Requires data cloning (allocation cost)
- Cancels losers (saves resources)
- Winner's speed determines total time

## Common Patterns

```go
// Multi-region API calls
multiRegion := pipz.NewRace("geo-fetch",
    pipz.Apply("us-east", fetchFromUSEast),
    pipz.Apply("eu-west", fetchFromEUWest),
    pipz.Apply("ap-south", fetchFromAPSouth),
)

// Cache with fallbacks
cachedFetch := pipz.NewRace("cached-data",
    pipz.Apply("memory", fetchFromMemory),    // Fastest
    pipz.Apply("redis", fetchFromRedis),      // Fast
    pipz.Apply("database", fetchFromDB),      // Slower
    pipz.Apply("compute", computeFromScratch), // Slowest
)

// Service degradation
resilientService := pipz.NewSequence[Request]("resilient",
    pipz.Apply("validate", validateRequest),
    pipz.NewRace("process",
        pipz.NewTimeout("fast-primary", 
            pipz.Apply("primary", usePrimaryService),
            1*time.Second,
        ),
        pipz.NewTimeout("slow-secondary",
            pipz.Apply("secondary", useSecondaryService),
            5*time.Second,
        ),
    ),
)
```

## Advanced Usage

```go
// Combine with retry for resilience
resilientRace := pipz.NewRace("resilient-fetch",
    pipz.NewRetry("cache-retry", fetchFromCache, 2),
    pipz.NewRetry("db-retry", fetchFromDB, 3),
    pipz.Apply("api", fetchFromAPI),
)

// Monitor race results
monitoredRace := pipz.NewRace("monitored",
    pipz.Apply("option-a", func(ctx context.Context, data Data) (Data, error) {
        defer metrics.Increment("race.winner", "option", "a")
        return processOptionA(ctx, data)
    }),
    pipz.Apply("option-b", func(ctx context.Context, data Data) (Data, error) {
        defer metrics.Increment("race.winner", "option", "b")
        return processOptionB(ctx, data)
    }),
)

// Conditional racing based on context
smartRace := pipz.NewSequence[Query]("smart-query",
    pipz.Mutate("add-cache",
        func(ctx context.Context, q Query) bool {
            return !q.NoCache
        },
        func(ctx context.Context, q Query) Query {
            // Add cache processor to race if caching is enabled
            q.Processors = append([]Chainable[Query]{cacheProcessor}, q.Processors...)
            return q
        },
    ),
    pipz.Apply("race", func(ctx context.Context, q Query) (Query, error) {
        return pipz.NewRace("dynamic", q.Processors...).Process(ctx, q)
    }),
)
```

## See Also

- [Concurrent](./concurrent.md) - For running all processors
- [Fallback](./fallback.md) - For simple primary/backup pattern
- [Timeout](./timeout.md) - Often used with Race