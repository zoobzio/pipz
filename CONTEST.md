# Contest Connector Implementation

## Summary

Successfully implemented the Contest connector for pipz, which combines Race's speed with conditional winner selection.

## What was added:

### 1. Core Implementation (`contest.go`)
- Contest type that runs processors in parallel
- Returns first result meeting a specified condition
- Cancels remaining processors once winner found
- Thread-safe with proper mutex handling
- Full configuration methods (Add, Remove, SetProcessors, SetCondition)

### 2. Comprehensive Tests (`contest_test.go`)
- 13 test cases covering all scenarios
- Tests for winner selection, no-winner handling, timeouts
- Configuration method tests
- Complex condition tests using context
- All tests passing

### 3. Benchmarks (`contest_bench_test.go`)  
- Performance benchmarks vs Race
- Tests with different condition complexities
- No-winner scenario benchmarks

### 4. Documentation (`docs/api/contest.md`)
- Complete API documentation
- Usage examples and patterns
- Best practices
- Comparison with Race

### 5. Integration
- Updated `docs/concepts/connectors.md` to include Contest
- Updated main `api.go` documentation
- Added Contest to the "Choosing the Right Connector" guide

### 6. Shipping Example (`examples/shipping-fulfillment/`)
- Complete example showcasing Contest for rate shopping
- Demonstrates adapter pattern with simple provider interface
- Shows Contest finding "first acceptable rate" vs Race's "first any rate"
- Includes tests and documentation

## Key Features:

1. **Conditional Winners**: Define what makes a result acceptable
2. **Speed + Quality**: Get fastest result that meets criteria  
3. **Early Termination**: Cancels slower processors once winner found
4. **Flexible Conditions**: Can use context for deadline-aware logic
5. **Runtime Updates**: Condition can be changed dynamically

## Example Usage:

```go
// Find cheapest shipping rate under $50 that delivers in 3 days
contest := pipz.NewContest("rate-shopping",
    func(_ context.Context, rate Rate) bool {
        return rate.Cost < 50.00 && rate.DeliveryDays <= 3
    },
    fedexRates,
    upsRates, 
    uspsRates,
)
```

## Performance:

Contest adds minimal overhead compared to Race when the condition is simple. The main benefit is avoiding the need to wait for all results when you can define acceptance criteria upfront.

## When to Use:

- **Contest**: When you need the fastest result that meets quality criteria
- **Race**: When any successful result is acceptable
- **Concurrent**: When you need all results for comparison

The shipping example demonstrates real-world usage where Contest shines - finding the first acceptable shipping rate without waiting for all providers to respond.