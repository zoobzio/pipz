---
title: "Concurrent"
description: "Runs multiple processors in parallel with isolated data copies and optional result aggregation"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - parallel
  - concurrency
  - aggregation
---

# Concurrent

Runs multiple processors in parallel with isolated data copies.

## Function Signature

```go
func NewConcurrent[T Cloner[T]](
    name Name,
    reducer func(original T, results map[Name]T, errors map[Name]error) T,
    processors ...Chainable[T],
) *Concurrent[T]
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
- `reducer` - Optional function to aggregate results; if `nil`, returns original input unchanged
- `processors` - Variable number of processors to run concurrently

## Returns

Returns a `*Concurrent[T]` that implements `Chainable[T]`.

## Behavior

- **Parallel execution** - All processors run simultaneously
- **Data isolation** - Each processor receives a clone of the input
- **Non-failing** - Individual failures don't stop other processors
- **Wait for all** - Waits for all processors to complete
- **Two modes**:
  - **Without reducer** (`nil`) - Returns the original input unchanged
  - **With reducer** - Collects all results and errors, then calls reducer to produce final output
- **Context preservation** - Passes original context to all processors, preserving distributed tracing and context values
- **Cancellation support** - Parent context cancellation affects all child processors

## Examples

### Without Reducer (Side Effects)

```go
// Define a type that implements Cloner
type User struct {
    ID      string
    Name    string
    Email   string
    Tags    []string
}

func (u User) Clone() User {
    tags := make([]string, len(u.Tags))
    copy(tags, u.Tags)
    return User{
        ID:    u.ID,
        Name:  u.Name,
        Email: u.Email,
        Tags:  tags,
    }
}

// Create concurrent processor without reducer
notifications := pipz.NewConcurrent("notify-all",
    nil, // No reducer - just run side effects
    pipz.Effect("email", sendEmailNotification),
    pipz.Effect("sms", sendSMSNotification),
    pipz.Effect("push", sendPushNotification),
    pipz.Effect("audit", logToAuditTrail),
)

// Use in a pipeline
pipeline := pipz.NewSequence[User]("user-update",
    pipz.Apply("validate", validateUser),
    pipz.Apply("update", updateDatabase),
    notifications, // All notifications sent in parallel
)
```

### With Reducer (Aggregate Results)

```go
type PriceCheck struct {
    ProductID string
    BestPrice float64
}

func (p PriceCheck) Clone() PriceCheck {
    return p
}

// Reducer function to find the best price
reducer := func(original PriceCheck, results map[Name]PriceCheck, errors map[Name]error) PriceCheck {
    bestPrice := original.BestPrice
    for _, result := range results {
        if result.BestPrice > 0 && result.BestPrice < bestPrice {
            bestPrice = result.BestPrice
        }
    }
    return PriceCheck{
        ProductID: original.ProductID,
        BestPrice: bestPrice,
    }
}

// Check prices from multiple vendors concurrently
priceChecker := pipz.NewConcurrent("check-prices",
    reducer,
    pipz.Transform("amazon", checkAmazonPrice),
    pipz.Transform("walmart", checkWalmartPrice),
    pipz.Transform("target", checkTargetPrice),
)

// Returns PriceCheck with the lowest price found
result, _ := priceChecker.Process(ctx, PriceCheck{ProductID: "abc123", BestPrice: 999.99})
```

## When to Use

Use `Concurrent` when:
- Operations are **independent and can run in parallel**
- You want to fire multiple actions simultaneously
- Side effects can run in parallel (notifications, logging)
- You need to aggregate results from multiple parallel sources
- Individual failures shouldn't affect others
- You need to notify multiple systems
- Performance benefit from parallelization
- You want to collect data from multiple APIs concurrently

## When NOT to Use

Don't use `Concurrent` when:
- Operations must run in order (use `Sequence`)
- Type doesn't implement `Cloner[T]` (compilation error)
- You need to stop on first error (all run to completion)
- Operations share state or resources (race conditions)
- You need fastest result only (use `Race`)

## Error Handling

### Without Reducer

Concurrent continues even if some processors fail:

```go
concurrent := pipz.NewConcurrent("multi-save",
    nil, // No reducer
    pipz.Apply("primary", saveToPrimary),    // Might fail
    pipz.Apply("backup", saveToBackup),      // Still runs
    pipz.Effect("cache", updateCache),       // Still runs
)

// The original data is returned regardless of individual failures
result, err := concurrent.Process(ctx, data)
// err is nil even if some processors failed
// result is the original data
```

### With Reducer

Errors are collected in the `errors` map passed to the reducer:

```go
reducer := func(original Data, results map[Name]Data, errors map[Name]error) Data {
    if len(errors) > 0 {
        // Handle errors - maybe log them or set a flag
        for name, err := range errors {
            log.Printf("processor %s failed: %v", name, err)
        }
    }
    // Merge successful results
    merged := original
    for _, result := range results {
        merged = mergeData(merged, result)
    }
    return merged
}
```

## Performance Considerations

- Creates one goroutine per processor
- Requires data cloning (allocation cost)
- All processors run even if some finish early
- Context cancellation stops waiting processors

## Common Patterns

### Side Effects Pattern (No Reducer)

```go
// Parallel notifications
userNotifications := pipz.NewConcurrent("notifications",
    nil, // No reducer needed
    pipz.Effect("email", sendWelcomeEmail),
    pipz.Effect("sms", sendWelcomeSMS),
    pipz.Effect("crm", updateCRM),
    pipz.Effect("analytics", trackSignup),
)

// Parallel data distribution
distribute := pipz.NewConcurrent("distribute",
    nil,
    pipz.Apply("elasticsearch", indexInElastic),
    pipz.Apply("redis", cacheInRedis),
    pipz.Apply("s3", uploadToS3),
    pipz.Effect("metrics", recordMetrics),
)

// Multi-channel processing
processOrder := pipz.NewSequence[Order]("order-flow",
    pipz.Apply("validate", validateOrder),
    pipz.Apply("payment", processPayment),
    pipz.NewConcurrent("post-payment",
        nil,
        pipz.Effect("inventory", updateInventory),
        pipz.Effect("shipping", createShippingLabel),
        pipz.Effect("email", sendConfirmation),
        pipz.Effect("analytics", trackRevenue),
    ),
)
```

### Aggregation Pattern (With Reducer)

```go
// Merge enrichment data from multiple sources
type Product struct {
    ID          string
    Name        string
    Description string
    Reviews     []Review
    Inventory   int
    Price       float64
}

enrichReducer := func(original Product, results map[Name]Product, errors map[Name]error) Product {
    enriched := original

    // Merge reviews from review service
    if r, ok := results["reviews"]; ok {
        enriched.Reviews = r.Reviews
    }

    // Merge inventory from warehouse service
    if inv, ok := results["inventory"]; ok {
        enriched.Inventory = inv.Inventory
    }

    // Merge pricing from pricing service
    if price, ok := results["pricing"]; ok {
        enriched.Price = price.Price
    }

    return enriched
}

enrichProduct := pipz.NewConcurrent("enrich-product",
    enrichReducer,
    pipz.Transform("reviews", fetchReviews),
    pipz.Transform("inventory", fetchInventory),
    pipz.Transform("pricing", fetchPricing),
)
```

## Gotchas

### ❌ Don't forget to use reducer if you need results
```go
// WRONG - Expecting modified data without reducer
concurrent := pipz.NewConcurrent("modify",
    nil, // No reducer!
    pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2 // Result is discarded!
    }),
)
result, _ := concurrent.Process(ctx, 5)
// result is still 5, not 10!
```

### ✅ Use reducer when you need results
```go
// RIGHT - Reducer aggregates results
reducer := func(original int, results map[Name]int, errors map[Name]error) int {
    sum := original
    for _, v := range results {
        sum += v
    }
    return sum
}

concurrent := pipz.NewConcurrent("sum",
    reducer,
    pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2
    }),
)
result, _ := concurrent.Process(ctx, 5)
// result is now 15 (5 + 10)
```

### ✅ Or use nil reducer for side effects only
```go
// RIGHT - Side effects, not transformations
concurrent := pipz.NewConcurrent("effects",
    nil,
    pipz.Effect("log", logData),
    pipz.Effect("metrics", updateMetrics),
)
```

### ❌ Don't share state between processors
```go
// WRONG - Race condition!
var counter int
concurrent := pipz.NewConcurrent("racy",
    pipz.Effect("inc1", func(ctx context.Context, _ Data) error {
        counter++ // Race!
        return nil
    }),
    pipz.Effect("inc2", func(ctx context.Context, _ Data) error {
        counter++ // Race!
        return nil
    }),
)
```

### ✅ Use proper synchronization or avoid shared state
```go
// RIGHT - No shared state
concurrent := pipz.NewConcurrent("safe",
    pipz.Effect("db1", saveToDatabase1),
    pipz.Effect("db2", saveToDatabase2),
)
```

## Implementation Requirements

Your type must implement `Clone()` correctly:

```go
// Simple struct
type Event struct {
    ID        string
    Type      string
    Timestamp time.Time
}

func (e Event) Clone() Event {
    return e // Struct with only value types can be copied directly
}

// Struct with slices/maps
type Document struct {
    ID       string
    Sections []Section
    Metadata map[string]string
}

func (d Document) Clone() Document {
    sections := make([]Section, len(d.Sections))
    copy(sections, d.Sections)
    
    metadata := make(map[string]string, len(d.Metadata))
    for k, v := range d.Metadata {
        metadata[k] = v
    }
    
    return Document{
        ID:       d.ID,
        Sections: sections,
        Metadata: metadata,
    }
}
```

## See Also

- [Race](./race.md) - For getting the first successful result
- [Sequence](./sequence.md) - For sequential execution
- [Effect](./effect.md) - Common processor for concurrent operations