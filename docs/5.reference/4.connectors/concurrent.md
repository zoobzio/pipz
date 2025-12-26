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
    identity Identity,
    reducer func(original T, results map[Identity]T, errors map[Identity]error) T,
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

- `identity` (`Identity`) - Identifier with name and description for debugging
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

// Define identities upfront
var (
    NotifyAllID = pipz.NewIdentity("notify-all", "Send notifications to all channels in parallel")
    EmailID     = pipz.NewIdentity("email", "Send email notification")
    SMSID       = pipz.NewIdentity("sms", "Send SMS notification")
    PushID      = pipz.NewIdentity("push", "Send push notification")
    AuditID     = pipz.NewIdentity("audit", "Log to audit trail")
)

// Create concurrent processor without reducer
notifications := pipz.NewConcurrent(
    NotifyAllID,
    nil, // No reducer - just run side effects
    pipz.Effect(EmailID, sendEmailNotification),
    pipz.Effect(SMSID, sendSMSNotification),
    pipz.Effect(PushID, sendPushNotification),
    pipz.Effect(AuditID, logToAuditTrail),
)

// Define pipeline identities
var (
    UserUpdateID = pipz.NewIdentity("user-update", "Process user update with notifications")
    ValidateID   = pipz.NewIdentity("validate", "Validate user data")
    UpdateID     = pipz.NewIdentity("update", "Update user in database")
)

// Use in a pipeline
pipeline := pipz.NewSequence[User](
    UserUpdateID,
    pipz.Apply(ValidateID, validateUser),
    pipz.Apply(UpdateID, updateDatabase),
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

// Define identities upfront
var (
    CheckPricesID = pipz.NewIdentity("check-prices", "Check prices across multiple vendors for best price")
    AmazonID      = pipz.NewIdentity("amazon", "Check Amazon price")
    WalmartID     = pipz.NewIdentity("walmart", "Check Walmart price")
    TargetID      = pipz.NewIdentity("target", "Check Target price")
)

// Reducer function to find the best price
reducer := func(original PriceCheck, results map[pipz.Identity]PriceCheck, errors map[pipz.Identity]error) PriceCheck {
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
priceChecker := pipz.NewConcurrent(
    CheckPricesID,
    reducer,
    pipz.Transform(AmazonID, checkAmazonPrice),
    pipz.Transform(WalmartID, checkWalmartPrice),
    pipz.Transform(TargetID, checkTargetPrice),
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
// Define identities upfront
var (
    MultiSaveID = pipz.NewIdentity("multi-save", "Save data to multiple destinations")
    PrimaryID   = pipz.NewIdentity("primary", "Save to primary storage")
    BackupID    = pipz.NewIdentity("backup", "Save to backup storage")
    CacheID     = pipz.NewIdentity("cache", "Update cache")
)

concurrent := pipz.NewConcurrent(
    MultiSaveID,
    nil, // No reducer
    pipz.Apply(PrimaryID, saveToPrimary),   // Might fail
    pipz.Apply(BackupID, saveToBackup),     // Still runs
    pipz.Effect(CacheID, updateCache),      // Still runs
)

// The original data is returned regardless of individual failures
result, err := concurrent.Process(ctx, data)
// err is nil even if some processors failed
// result is the original data
```

### With Reducer

Errors are collected in the `errors` map passed to the reducer:

```go
reducer := func(original Data, results map[pipz.Identity]Data, errors map[pipz.Identity]error) Data {
    if len(errors) > 0 {
        // Handle errors - maybe log them or set a flag
        for id, err := range errors {
            log.Printf("processor %s failed: %v", id.Name(), err)
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
// Define identities for parallel notifications
var (
    NotificationsID = pipz.NewIdentity("notifications", "Send welcome notifications across all channels")
    WelcomeEmailID  = pipz.NewIdentity("email", "Send welcome email")
    WelcomeSMSID    = pipz.NewIdentity("sms", "Send welcome SMS")
    CRMID           = pipz.NewIdentity("crm", "Update CRM system")
    SignupAnalyticsID = pipz.NewIdentity("analytics", "Track signup event")
)

// Parallel notifications
userNotifications := pipz.NewConcurrent(
    NotificationsID,
    nil, // No reducer needed
    pipz.Effect(WelcomeEmailID, sendWelcomeEmail),
    pipz.Effect(WelcomeSMSID, sendWelcomeSMS),
    pipz.Effect(CRMID, updateCRM),
    pipz.Effect(SignupAnalyticsID, trackSignup),
)

// Define identities for parallel data distribution
var (
    DistributeID      = pipz.NewIdentity("distribute", "Distribute data to multiple systems")
    ElasticsearchID   = pipz.NewIdentity("elasticsearch", "Index in Elasticsearch")
    RedisID           = pipz.NewIdentity("redis", "Cache in Redis")
    S3ID              = pipz.NewIdentity("s3", "Upload to S3")
    MetricsID         = pipz.NewIdentity("metrics", "Record metrics")
)

// Parallel data distribution
distribute := pipz.NewConcurrent(
    DistributeID,
    nil,
    pipz.Apply(ElasticsearchID, indexInElastic),
    pipz.Apply(RedisID, cacheInRedis),
    pipz.Apply(S3ID, uploadToS3),
    pipz.Effect(MetricsID, recordMetrics),
)

// Define identities for order processing
var (
    OrderFlowID       = pipz.NewIdentity("order-flow", "Process order from validation to post-payment")
    ValidateOrderID   = pipz.NewIdentity("validate", "Validate order details")
    PaymentID         = pipz.NewIdentity("payment", "Process payment")
    PostPaymentID     = pipz.NewIdentity("post-payment", "Execute post-payment operations in parallel")
    InventoryID       = pipz.NewIdentity("inventory", "Update inventory")
    ShippingID        = pipz.NewIdentity("shipping", "Create shipping label")
    ConfirmationID    = pipz.NewIdentity("email", "Send confirmation email")
    RevenueAnalyticsID = pipz.NewIdentity("analytics", "Track revenue")
)

// Multi-channel processing
processOrder := pipz.NewSequence[Order](
    OrderFlowID,
    pipz.Apply(ValidateOrderID, validateOrder),
    pipz.Apply(PaymentID, processPayment),
    pipz.NewConcurrent(
        PostPaymentID,
        nil,
        pipz.Effect(InventoryID, updateInventory),
        pipz.Effect(ShippingID, createShippingLabel),
        pipz.Effect(ConfirmationID, sendConfirmation),
        pipz.Effect(RevenueAnalyticsID, trackRevenue),
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

// Define identities upfront so we can reference them in the reducer
var (
    EnrichProductID = pipz.NewIdentity("enrich-product", "Enrich product with data from multiple services")
    ReviewsID       = pipz.NewIdentity("reviews", "Fetch product reviews")
    ProductInventoryID = pipz.NewIdentity("inventory", "Fetch inventory levels")
    PricingID       = pipz.NewIdentity("pricing", "Fetch current pricing")
)

enrichReducer := func(original Product, results map[pipz.Identity]Product, errors map[pipz.Identity]error) Product {
    enriched := original

    // Merge reviews from review service
    if r, ok := results[ReviewsID]; ok {
        enriched.Reviews = r.Reviews
    }

    // Merge inventory from warehouse service
    if inv, ok := results[ProductInventoryID]; ok {
        enriched.Inventory = inv.Inventory
    }

    // Merge pricing from pricing service
    if price, ok := results[PricingID]; ok {
        enriched.Price = price.Price
    }

    return enriched
}

enrichProduct := pipz.NewConcurrent(
    EnrichProductID,
    enrichReducer,
    pipz.Transform(ReviewsID, fetchReviews),
    pipz.Transform(ProductInventoryID, fetchInventory),
    pipz.Transform(PricingID, fetchPricing),
)
```

## Gotchas

### ❌ Don't forget to use reducer if you need results
```go
// WRONG - Expecting modified data without reducer
var (
    ModifyID = pipz.NewIdentity("modify", "Modify values")
    DoubleID = pipz.NewIdentity("double", "Double the value")
)

concurrent := pipz.NewConcurrent(
    ModifyID,
    nil, // No reducer!
    pipz.Transform(DoubleID, func(ctx context.Context, n int) int {
        return n * 2 // Result is discarded!
    }),
)
result, _ := concurrent.Process(ctx, 5)
// result is still 5, not 10!
```

### ✅ Use reducer when you need results
```go
// Define identities upfront
var (
    SumID    = pipz.NewIdentity("sum", "Sum original value with results")
    DoubleID = pipz.NewIdentity("double", "Double the value")
)

// RIGHT - Reducer aggregates results
reducer := func(original int, results map[pipz.Identity]int, errors map[pipz.Identity]error) int {
    sum := original
    for _, v := range results {
        sum += v
    }
    return sum
}

concurrent := pipz.NewConcurrent(
    SumID,
    reducer,
    pipz.Transform(DoubleID, func(ctx context.Context, n int) int {
        return n * 2
    }),
)
result, _ := concurrent.Process(ctx, 5)
// result is now 15 (5 + 10)
```

### ✅ Or use nil reducer for side effects only
```go
// Define identities upfront
var (
    EffectsID = pipz.NewIdentity("effects", "Execute side effects in parallel")
    LogID     = pipz.NewIdentity("log", "Log data")
    MetricsID = pipz.NewIdentity("metrics", "Update metrics")
)

// RIGHT - Side effects, not transformations
concurrent := pipz.NewConcurrent(
    EffectsID,
    nil,
    pipz.Effect(LogID, logData),
    pipz.Effect(MetricsID, updateMetrics),
)
```

### ❌ Don't share state between processors
```go
// WRONG - Race condition!
var counter int

var (
    RacyID = pipz.NewIdentity("racy", "Increment counter (has race condition)")
    Inc1ID = pipz.NewIdentity("inc1", "Increment counter")
    Inc2ID = pipz.NewIdentity("inc2", "Increment counter again")
)

concurrent := pipz.NewConcurrent(
    RacyID,
    pipz.Effect(Inc1ID, func(ctx context.Context, _ Data) error {
        counter++ // Race!
        return nil
    }),
    pipz.Effect(Inc2ID, func(ctx context.Context, _ Data) error {
        counter++ // Race!
        return nil
    }),
)
```

### ✅ Use proper synchronization or avoid shared state
```go
// Define identities upfront
var (
    SafeID = pipz.NewIdentity("safe", "Save to multiple databases independently")
    DB1ID  = pipz.NewIdentity("db1", "Save to database 1")
    DB2ID  = pipz.NewIdentity("db2", "Save to database 2")
)

// RIGHT - No shared state
concurrent := pipz.NewConcurrent(
    SafeID,
    pipz.Effect(DB1ID, saveToDatabase1),
    pipz.Effect(DB2ID, saveToDatabase2),
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