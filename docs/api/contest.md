# Contest

`Contest` runs all processors in parallel and returns the first result that meets a specified condition. It combines the speed benefits of `Race` with conditional selection, allowing you to define what makes a "winner" beyond just being first to complete.

## Overview

Contest is ideal when you need the fastest result that also meets quality criteria:
- Finding the cheapest shipping rate under a time constraint
- Getting the first API response with required data completeness
- Querying multiple sources for the best quality result quickly
- Racing services where the "best" result matters more than just "first"

## Creating a Contest

```go
// Define the winning condition
condition := func(ctx context.Context, rate ShippingRate) bool {
    return rate.Cost < 50.00 && rate.DeliveryDays <= 3
}

// Create Contest with multiple processors
contest := pipz.NewContest("find-best-rate", condition,
    fedexRates,
    upsRates,
    uspsRates,
)
```

## Key Behaviors

1. **Parallel Execution**: All processors run concurrently
2. **Conditional Winner**: First result that meets the condition wins
3. **Early Termination**: Winner cancels remaining processors
4. **No Winner Handling**: Returns error if no results meet condition
5. **Clone Safety**: Each processor gets an isolated copy via `Clone()`

## Example: Rate Shopping

```go
package main

import (
    "context"
    "github.com/zoobzio/pipz"
)

// Find the cheapest acceptable shipping rate
func main() {
    // Condition: Must be under $30 and deliver within 5 days
    acceptableRate := func(_ context.Context, rate Rate) bool {
        return rate.Cost < 30.00 && rate.EstimatedDays <= 5
    }
    
    // Create processors for each provider
    fedex := pipz.Apply("fedex", fetchFedExRate)
    ups := pipz.Apply("ups", fetchUPSRate)
    usps := pipz.Apply("usps", fetchUSPSRate)
    
    // Contest to find first acceptable rate
    rateContest := pipz.NewContest("rate-shopping", acceptableRate,
        fedex, ups, usps,
    )
    
    shipment := Shipment{Weight: 5.0, Destination: "NYC"}
    result, err := rateContest.Process(context.Background(), shipment)
}
```

## Dynamic Conditions

You can update the winning condition at runtime:

```go
// Start with strict criteria
contest := pipz.NewContest("dynamic", strictCondition, processors...)

// Relax criteria based on circumstances
if timeIsRunningOut {
    contest.SetCondition(relaxedCondition)
}
```

## Complex Conditions

Conditions can use context for sophisticated logic:

```go
// Condition that adapts based on deadline
adaptiveCondition := func(ctx context.Context, result Result) bool {
    deadline, ok := ctx.Deadline()
    if !ok {
        // No deadline - use strict criteria
        return result.Quality > 90 && result.Cost < 100
    }
    
    // Relax criteria as deadline approaches
    timeLeft := time.Until(deadline)
    if timeLeft < 5*time.Second {
        return result.Quality > 70 // Accept lower quality if urgent
    }
    return result.Quality > 90 && result.Cost < 100
}
```

## Configuration Methods

Contest supports the same configuration methods as other connectors:

```go
contest := pipz.NewContest("configurable", condition)

// Add processors
contest.Add(newProcessor)

// Remove by index
contest.Remove(0)

// Replace all processors
contest.SetProcessors(p1, p2, p3)

// Clear all
contest.Clear()

// Get count
count := contest.Len()

// Update condition
contest.SetCondition(newCondition)
```

## Error Handling

Contest provides specific error messages for different scenarios:

```go
result, err := contest.Process(ctx, input)
if err != nil {
    var pipeErr *pipz.Error[T]
    if errors.As(err, &pipeErr) {
        if strings.Contains(pipeErr.Error(), "no processor results met") {
            // Some processors succeeded but none met condition
        } else if strings.Contains(pipeErr.Error(), "all processors failed") {
            // All processors returned errors
        }
    }
}
```

## Contest vs Race

| Aspect | Contest | Race |
|--------|---------|------|
| Winner Selection | First to meet condition | First to complete |
| Use Case | Quality + Speed | Pure speed |
| Condition Function | Required | Not applicable |
| Result Evaluation | Checks each result | Accepts any success |

## Best Practices

1. **Meaningful Conditions**: Write clear conditions that express business requirements
2. **Fail Fast**: Order processors by likelihood of meeting conditions
3. **Timeout Handling**: Consider deadline-aware conditions for time-sensitive operations
4. **Error Context**: Use the error path to understand which processors were tried
5. **Testing**: Test both successful and no-winner scenarios

## Common Patterns

### Fallback with Quality

```go
// Try premium services first, fall back to economy if needed
premiumCondition := func(_ context.Context, svc Service) bool {
    return svc.Type == "premium" && svc.Available
}

contest := pipz.NewContest("service-selection", premiumCondition,
    premiumService1,
    premiumService2,
    // These economy services won't win unless all premium fail
    economyService1,
    economyService2,
)
```

### Cost-Optimized Selection

```go
// Find cheapest option that meets SLA
budgetCondition := func(_ context.Context, opt Option) bool {
    return opt.MeetsSLA && opt.Cost < budget
}

contest := pipz.NewContest("cost-optimization", budgetCondition,
    vendors...,
)
```

### Progressive Relaxation

```go
// Try strict criteria first
strict := pipz.NewContest("strict", strictCondition, processors...)
result, err := strict.Process(ctx, input)

if err != nil {
    // Relax criteria and try again
    relaxed := pipz.NewContest("relaxed", relaxedCondition, processors...)
    result, err = relaxed.Process(ctx, input)
}
```

## See Also

- [Race](./race.md) - First successful result wins
- [Concurrent](./concurrent.md) - Run all in parallel
- [Fallback](./fallback.md) - Sequential fallback pattern
- [Switch](./switch.md) - Conditional routing