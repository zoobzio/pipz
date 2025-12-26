---
title: "Contest"
description: "Runs processors in parallel and returns the first result that meets a specified condition"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - parallel
  - conditional
  - quality
---

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
// Define identities
var FindBestRateID = pipz.NewIdentity("find-best-rate", "Find first shipping rate under $50 with 3-day delivery")

// Define the winning condition
condition := func(ctx context.Context, rate ShippingRate) bool {
    return rate.Cost < 50.00 && rate.DeliveryDays <= 3
}

// Create Contest with multiple processors
contest := pipz.NewContest(
    FindBestRateID,
    condition,
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
6. **Context Preservation**: Uses `context.WithCancel(ctx)` to preserve trace context while enabling cancellation when winner is found

## Example: Rate Shopping

```go
package main

import (
    "context"
    "github.com/zoobzio/pipz"
)

// Define identities
var (
    FedExID = pipz.NewIdentity("fedex", "Fetch FedEx shipping rate")
    UPSID = pipz.NewIdentity("ups", "Fetch UPS shipping rate")
    USPSID = pipz.NewIdentity("usps", "Fetch USPS shipping rate")
    RateShoppingID = pipz.NewIdentity("rate-shopping", "Find first acceptable shipping rate under $30")
)

// Find the cheapest acceptable shipping rate
func main() {
    // Condition: Must be under $30 and deliver within 5 days
    acceptableRate := func(_ context.Context, rate Rate) bool {
        return rate.Cost < 30.00 && rate.EstimatedDays <= 5
    }

    // Create processors for each provider
    fedex := pipz.Apply(FedExID, fetchFedExRate)
    ups := pipz.Apply(UPSID, fetchUPSRate)
    usps := pipz.Apply(USPSID, fetchUSPSRate)

    // Contest to find first acceptable rate
    rateContest := pipz.NewContest(
        RateShoppingID,
        acceptableRate,
        fedex, ups, usps,
    )

    shipment := Shipment{Weight: 5.0, Destination: "NYC"}
    result, err := rateContest.Process(context.Background(), shipment)
}
```

## Dynamic Conditions

You can update the winning condition at runtime:

```go
// Define identity
var DynamicContestID = pipz.NewIdentity("dynamic", "Contest with dynamic quality criteria")

// Start with strict criteria
contest := pipz.NewContest(
    DynamicContestID,
    strictCondition,
    processors...,
)

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
// Define identity
var ConfigurableContestID = pipz.NewIdentity("configurable", "Contest with configurable processors")

contest := pipz.NewContest(
    ConfigurableContestID,
    condition,
)

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

## When to Use

Use `Contest` when:
- You need the **fastest result that meets quality criteria**
- Multiple sources can provide acceptable results
- Quality matters more than just speed
- You're comparing prices, rates, or scores
- You want to optimize for both speed and quality
- Different processors have different quality/speed tradeoffs

## When NOT to Use

Don't use `Contest` when:
- Any successful result is fine (use `Race`)
- You need all results (use `Concurrent`)
- Results aren't comparable (different data types)
- Order of execution matters (use `Sequence`)
- You always need the highest quality regardless of time (process all, then select)

## Contest vs Race

| Aspect | Contest | Race |
|--------|---------|------|
| Winner Selection | First to meet condition | First to complete |
| Use Case | Quality + Speed | Pure speed |
| Condition Function | Required | Not applicable |
| Result Evaluation | Checks each result | Accepts any success |

## Gotchas

### ❌ Don't use vague conditions
```go
// WRONG - What does "good" mean?
var VagueContestID = pipz.NewIdentity("vague", "Contest with unclear criteria")

contest := pipz.NewContest(
    VagueContestID,
    func(ctx context.Context, result Result) bool {
        return result.IsGood // Unclear criteria
    },
    processors...,
)
```

### ✅ Use specific, measurable conditions
```go
// RIGHT - Clear, measurable criteria
var SpecificContestID = pipz.NewIdentity("specific", "Find result with >95% accuracy, <100ms latency, and <$10 cost")

contest := pipz.NewContest(
    SpecificContestID,
    func(ctx context.Context, result Result) bool {
        return result.Accuracy > 0.95 &&
               result.Latency < 100*time.Millisecond &&
               result.Cost < 10.00
    },
    processors...,
)
```

### ❌ Don't ignore "no winner" scenarios
```go
// WRONG - Assumes someone always wins
result, _ := contest.Process(ctx, input) // Ignoring error!
processResult(result) // May be zero value!
```

### ✅ Handle no winner gracefully
```go
// RIGHT - Handle no winner case
result, err := contest.Process(ctx, input)
if err != nil {
    if strings.Contains(err.Error(), "no processor results met") {
        // Use fallback or relax criteria
        result = getDefaultResult()
    } else {
        return err // Real error
    }
}
```

### ❌ Don't use Contest for side effects
```go
// WRONG - All run until one meets condition!
var (
    SideEffectsContestID = pipz.NewIdentity("side-effects", "Contest with side effects (dangerous)")
    Update1ID = pipz.NewIdentity("update1", "Update database 1")
    Update2ID = pipz.NewIdentity("update2", "Update database 2")
)

contest := pipz.NewContest(
    SideEffectsContestID,
    func(ctx context.Context, r Result) bool {
        return r.Success
    },
    pipz.Apply(Update1ID, updateDatabase1), // Updates!
    pipz.Apply(Update2ID, updateDatabase2), // Also updates!
)
```

### ✅ Use Contest for queries only
```go
// RIGHT - Safe read operations
var (
    QueriesContestID = pipz.NewIdentity("queries", "Find first complete and fresh query result")
    Query1ID = pipz.NewIdentity("query1", "Query database 1")
    Query2ID = pipz.NewIdentity("query2", "Query database 2")
)

contest := pipz.NewContest(
    QueriesContestID,
    func(ctx context.Context, r Result) bool {
        return r.Complete && r.Fresh
    },
    pipz.Apply(Query1ID, queryDatabase1),
    pipz.Apply(Query2ID, queryDatabase2),
)
```

## Best Practices

1. **Meaningful Conditions**: Write clear conditions that express business requirements
2. **Fail Fast**: Order processors by likelihood of meeting conditions
3. **Timeout Handling**: Consider deadline-aware conditions for time-sensitive operations
4. **Error Context**: Use the error path to understand which processors were tried
5. **Testing**: Test both successful and no-winner scenarios

## Common Patterns

### Fallback with Quality

```go
// Define identity
var ServiceSelectionID = pipz.NewIdentity("service-selection", "Select first available premium service, fallback to economy")

// Try premium services first, fall back to economy if needed
premiumCondition := func(_ context.Context, svc Service) bool {
    return svc.Type == "premium" && svc.Available
}

contest := pipz.NewContest(
    ServiceSelectionID,
    premiumCondition,
    premiumService1,
    premiumService2,
    // These economy services won't win unless all premium fail
    economyService1,
    economyService2,
)
```

### Cost-Optimized Selection

```go
// Define identity
var CostOptimizationID = pipz.NewIdentity("cost-optimization", "Find first vendor meeting SLA within budget")

// Find cheapest option that meets SLA
budgetCondition := func(_ context.Context, opt Option) bool {
    return opt.MeetsSLA && opt.Cost < budget
}

contest := pipz.NewContest(
    CostOptimizationID,
    budgetCondition,
    vendors...,
)
```

### Progressive Relaxation

```go
// Define identities
var (
    StrictContestID = pipz.NewIdentity("strict", "Contest with strict quality criteria")
    RelaxedContestID = pipz.NewIdentity("relaxed", "Contest with relaxed criteria")
)

// Try strict criteria first
strict := pipz.NewContest(
    StrictContestID,
    strictCondition,
    processors...,
)
result, err := strict.Process(ctx, input)

if err != nil {
    // Relax criteria and try again
    relaxed := pipz.NewContest(
        RelaxedContestID,
        relaxedCondition,
        processors...,
    )
    result, err = relaxed.Process(ctx, input)
}
```

## See Also

- [Race](./race.md) - First successful result wins
- [Concurrent](./concurrent.md) - Run all in parallel
- [Fallback](./fallback.md) - Sequential fallback pattern
- [Switch](./switch.md) - Conditional routing