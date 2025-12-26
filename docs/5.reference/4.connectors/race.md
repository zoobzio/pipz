---
title: "Race"
description: "Runs processors in parallel and returns the first successful result for optimizing latency"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - parallel
  - performance
  - latency
---

# Race

Runs processors in parallel and returns the first successful result.

## Function Signature

```go
func NewRace[T Cloner[T]](identity Identity, processors ...Chainable[T]) *Race[T]
```

## Type Constraints

- `T` must implement the `Cloner[T]` interface:
  ```go
  type Cloner[T any] interface {
      Clone() T
  }
  ```

## Parameters

- `identity` (`Identity`) - Identity containing name and description for debugging and documentation
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
var (
    FetchFastestID = pipz.NewIdentity("fetch-fastest", "race to fetch data from first available source")
    CacheID        = pipz.NewIdentity("cache", "try cache first")
    PrimaryDBID    = pipz.NewIdentity("primary-db", "fetch from primary database")
    ReplicaDBID    = pipz.NewIdentity("replica-db", "fetch from replica database")
    APIFallbackID  = pipz.NewIdentity("api-fallback", "fallback to external API")
)

fetchData := pipz.NewRace(
    FetchFastestID,
    pipz.Apply(CacheID, fetchFromCache),
    pipz.Apply(PrimaryDBID, fetchFromPrimary),
    pipz.Apply(ReplicaDBID, fetchFromReplica),
    pipz.Apply(APIFallbackID, fetchFromAPI),
)

// Race different processing strategies
var (
    ImageProcessorID = pipz.NewIdentity("image-processor", "race different image processing strategies")
    GPUID            = pipz.NewIdentity("gpu", "process image using GPU acceleration")
    CPUOptimizedID   = pipz.NewIdentity("cpu-optimized", "process using SIMD CPU optimization")
    CPUStandardID    = pipz.NewIdentity("cpu-standard", "process using standard CPU")
)

processImage := pipz.NewRace(
    ImageProcessorID,
    pipz.Apply(GPUID, processWithGPU),
    pipz.Apply(CPUOptimizedID, processWithSIMD),
    pipz.Apply(CPUStandardID, processWithCPU),
)

// Race external services
var (
    TranslateID = pipz.NewIdentity("translate", "race translation services for fastest response")
    GoogleID    = pipz.NewIdentity("google", "translate using Google Translate API")
    DeepLID     = pipz.NewIdentity("deepl", "translate using DeepL API")
    AzureID     = pipz.NewIdentity("azure", "translate using Azure Translator")
)

translateText := pipz.NewRace(
    TranslateID,
    pipz.Apply(GoogleID, translateWithGoogle),
    pipz.Apply(DeepLID, translateWithDeepL),
    pipz.Apply(AzureID, translateWithAzure),
)
```

## When to Use

Use `Race` when:
- You have **multiple ways to get the same result** (cache, database, API)
- You want the fastest response time
- You have geographically distributed services
- Latency is critical
- Any successful result is acceptable
- Trading resource usage for speed is acceptable

## When NOT to Use

Don't use `Race` when:
- You need all operations to complete (use `Concurrent`)
- Results might differ between processors (not equivalent)
- Order matters (use `Sequence`)
- You need to know which source succeeded
- Resource usage needs to be minimized
- Operations have side effects (all will run)

## Error Handling

Race only fails if all processors fail:

```go
var (
    MultiFetchID = pipz.NewIdentity("multi-fetch", "race multiple fetch strategies for first success")
    FastID       = pipz.NewIdentity("fast", "fast but flaky service")
    SlowID       = pipz.NewIdentity("slow", "slow but reliable service")
    BackupID     = pipz.NewIdentity("backup", "backup service as last resort")
)

race := pipz.NewRace(
    MultiFetchID,
    pipz.Apply(FastID, fastButFlaky),     // Fails 50% of time
    pipz.Apply(SlowID, slowButReliable),  // Takes 5 seconds
    pipz.Apply(BackupID, backupService),  // Last resort
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
var (
    GeoFetchID = pipz.NewIdentity("geo-fetch", "race geo-distributed API calls")
    USEastID   = pipz.NewIdentity("us-east", "fetch from US East region")
    EUWestID   = pipz.NewIdentity("eu-west", "fetch from EU West region")
    APSouthID  = pipz.NewIdentity("ap-south", "fetch from Asia Pacific South region")
)

multiRegion := pipz.NewRace(
    GeoFetchID,
    pipz.Apply(USEastID, fetchFromUSEast),
    pipz.Apply(EUWestID, fetchFromEUWest),
    pipz.Apply(APSouthID, fetchFromAPSouth),
)

// Cache with fallbacks
var (
    CachedDataID = pipz.NewIdentity("cached-data", "race cache layers for fastest data access")
    MemoryID     = pipz.NewIdentity("memory", "fetch from in-memory cache")
    RedisID      = pipz.NewIdentity("redis", "fetch from Redis cache")
    DatabaseID   = pipz.NewIdentity("database", "fetch from database")
    ComputeID    = pipz.NewIdentity("compute", "compute data from scratch")
)

cachedFetch := pipz.NewRace(
    CachedDataID,
    pipz.Apply(MemoryID, fetchFromMemory),    // Fastest
    pipz.Apply(RedisID, fetchFromRedis),      // Fast
    pipz.Apply(DatabaseID, fetchFromDB),      // Slower
    pipz.Apply(ComputeID, computeFromScratch), // Slowest
)

// Service degradation
var (
    ResilientID     = pipz.NewIdentity("resilient", "resilient service with timeout-based degradation")
    ValidateID      = pipz.NewIdentity("validate", "validate incoming request")
    ProcessID       = pipz.NewIdentity("process", "race primary and secondary services")
    FastPrimaryID   = pipz.NewIdentity("fast-primary", "primary service with tight timeout")
    PrimaryID       = pipz.NewIdentity("primary", "call primary service")
    SlowSecondaryID = pipz.NewIdentity("slow-secondary", "secondary service with relaxed timeout")
    SecondaryID     = pipz.NewIdentity("secondary", "call secondary service")
)

resilientService := pipz.NewSequence(
    ResilientID,
    pipz.Apply(ValidateID, validateRequest),
    pipz.NewRace(
        ProcessID,
        pipz.NewTimeout(
            FastPrimaryID,
            pipz.Apply(PrimaryID, usePrimaryService),
            1*time.Second,
        ),
        pipz.NewTimeout(
            SlowSecondaryID,
            pipz.Apply(SecondaryID, useSecondaryService),
            5*time.Second,
        ),
    ),
)
```

## Gotchas

### ❌ Don't use Race for different results
```go
// WRONG - These return different data!
var (
    DifferentID = pipz.NewIdentity("different", "race different data sources")
    SummaryID   = pipz.NewIdentity("summary", "get summary data")
    DetailedID  = pipz.NewIdentity("detailed", "get detailed data")
    MetadataID  = pipz.NewIdentity("metadata", "get metadata only")
)

race := pipz.NewRace(
    DifferentID,
    pipz.Apply(SummaryID, getSummaryData),     // Returns summary
    pipz.Apply(DetailedID, getDetailedData),   // Returns full data
    pipz.Apply(MetadataID, getMetadata),       // Returns only metadata
)
// You'll get random incomplete data!
```

### ✅ Use Race for equivalent results
```go
// RIGHT - All return the same data
var (
    EquivalentID   = pipz.NewIdentity("equivalent", "race equivalent data sources")
    CacheID        = pipz.NewIdentity("cache", "get from cache")
    PrimaryID      = pipz.NewIdentity("primary", "get from primary database")
    ReplicaID      = pipz.NewIdentity("replica", "get from replica database")
)

race := pipz.NewRace(
    EquivalentID,
    pipz.Apply(CacheID, getFromCache),
    pipz.Apply(PrimaryID, getFromPrimary),
    pipz.Apply(ReplicaID, getFromReplica),
)
```

### ❌ Don't use Race with side effects
```go
// WRONG - All processors run until one succeeds!
var (
    SideEffectsID = pipz.NewIdentity("side-effects", "race payment methods")
    Charge1ID     = pipz.NewIdentity("charge1", "charge first payment method")
    Charge2ID     = pipz.NewIdentity("charge2", "charge second payment method")
    Charge3ID     = pipz.NewIdentity("charge3", "charge third payment method")
)

race := pipz.NewRace(
    SideEffectsID,
    pipz.Apply(Charge1ID, chargePaymentMethod1), // Charges!
    pipz.Apply(Charge2ID, chargePaymentMethod2), // Also charges!
    pipz.Apply(Charge3ID, chargePaymentMethod3), // Triple charge!
)
```

### ✅ Use Race for read operations
```go
// RIGHT - Safe read operations
var (
    ReadsID = pipz.NewIdentity("reads", "race read operations for fastest response")
    Get1ID  = pipz.NewIdentity("get1", "fetch data from first source")
    Get2ID  = pipz.NewIdentity("get2", "fetch data from second source")
    Get3ID  = pipz.NewIdentity("get3", "fetch data from third source")
)

race := pipz.NewRace(
    ReadsID,
    pipz.Apply(Get1ID, fetchData1),
    pipz.Apply(Get2ID, fetchData2),
    pipz.Apply(Get3ID, fetchData3),
)
```

### ❌ Don't forget about resource usage
```go
// WRONG - Wastes resources
var (
    WastefulID   = pipz.NewIdentity("wasteful", "race expensive operations")
    Expensive1ID = pipz.NewIdentity("expensive1", "very expensive GPU operation")
    Expensive2ID = pipz.NewIdentity("expensive2", "expensive RAM operation")
    Expensive3ID = pipz.NewIdentity("expensive3", "expensive CPU operation")
)

race := pipz.NewRace(
    WastefulID,
    pipz.Apply(Expensive1ID, veryExpensiveOperation), // Uses GPU
    pipz.Apply(Expensive2ID, anotherExpensiveOp),     // Uses lots of RAM
    pipz.Apply(Expensive3ID, yetAnotherExpensive),    // Heavy CPU
)
// All three run even though only one result is needed!
```

### ✅ Put cheap operations first
```go
// RIGHT - Try cheap operations first
var (
    EfficientID = pipz.NewIdentity("efficient", "race operations from cheapest to most expensive")
    CacheID     = pipz.NewIdentity("cache", "check cache for data")
    DatabaseID  = pipz.NewIdentity("database", "query database for data")
    ComputeID   = pipz.NewIdentity("compute", "compute data from scratch")
)

race := pipz.NewRace(
    EfficientID,
    pipz.Apply(CacheID, checkCache),           // Microseconds
    pipz.Apply(DatabaseID, queryDatabase),     // Milliseconds
    pipz.Apply(ComputeID, computeFromScratch), // Seconds
)
```

## Advanced Usage

```go
// Combine with retry for resilience
var (
    ResilientFetchID = pipz.NewIdentity("resilient-fetch", "race fetch operations with retry support")
    CacheRetryID     = pipz.NewIdentity("cache-retry", "retry cache fetch")
    DBRetryID        = pipz.NewIdentity("db-retry", "retry database fetch")
    APIID            = pipz.NewIdentity("api", "fetch from API without retry")
)

resilientRace := pipz.NewRace(
    ResilientFetchID,
    pipz.NewRetry(CacheRetryID, fetchFromCache, 2),
    pipz.NewRetry(DBRetryID, fetchFromDB, 3),
    pipz.Apply(APIID, fetchFromAPI),
)

// Monitor race results
var (
    MonitoredID = pipz.NewIdentity("monitored", "race processing options with metrics")
    OptionAID   = pipz.NewIdentity("option-a", "process with option A")
    OptionBID   = pipz.NewIdentity("option-b", "process with option B")
)

monitoredRace := pipz.NewRace(
    MonitoredID,
    pipz.Apply(OptionAID, func(ctx context.Context, data Data) (Data, error) {
        defer metrics.Increment("race.winner", "option", "a")
        return processOptionA(ctx, data)
    }),
    pipz.Apply(OptionBID, func(ctx context.Context, data Data) (Data, error) {
        defer metrics.Increment("race.winner", "option", "b")
        return processOptionB(ctx, data)
    }),
)

// Conditional racing based on context
var (
    SmartQueryID = pipz.NewIdentity("smart-query", "conditionally add cache to race based on query settings")
    AddCacheID   = pipz.NewIdentity("add-cache", "add cache processor if caching enabled")
    RaceID       = pipz.NewIdentity("race", "race dynamically configured processors")
    DynamicID    = pipz.NewIdentity("dynamic", "dynamically created race")
)

smartRace := pipz.NewSequence(
    SmartQueryID,
    pipz.Mutate(
        AddCacheID,
        func(ctx context.Context, q Query) Query {
            // Add cache processor to race if caching is enabled
            q.Processors = append([]Chainable[Query]{cacheProcessor}, q.Processors...)
            return q
        },
        func(ctx context.Context, q Query) bool {
            return !q.NoCache
        },
    ),
    pipz.Apply(RaceID, func(ctx context.Context, q Query) (Query, error) {
        return pipz.NewRace(DynamicID, q.Processors...).Process(ctx, q)
    }),
)
```

## See Also

- [Concurrent](./concurrent.md) - For running all processors
- [Fallback](./fallback.md) - For simple primary/backup pattern
- [Timeout](./timeout.md) - Often used with Race