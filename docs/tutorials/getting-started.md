# Getting Started with pipz

Build your first data pipeline in 10 minutes.

## What is pipz?

pipz is a Go library for building type-safe, composable data pipelines. Think of it as LEGO blocks for data processing - small, focused pieces that connect together to solve complex problems.

## Installation

```bash
go get github.com/zoobzio/pipz
```

Requires Go 1.21+ for generics support.

## Your First Pipeline

Let's build a simple pipeline that processes user registration:

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

// Our data structure
type User struct {
    Email    string
    Username string
    Age      int
    Verified bool
}

func main() {
    // Create a pipeline with 3 steps
    pipeline := pipz.NewSequence[User]("registration",
        // Step 1: Validate
        pipz.Apply("validate", validateUser),
        
        // Step 2: Normalize
        pipz.Transform("normalize", normalizeUser),
        
        // Step 3: Log
        pipz.Effect("log", logUser),
    )
    
    // Process a user
    user := User{
        Email:    "JOHN.DOE@EXAMPLE.COM",
        Username: "johndoe123",
        Age:      25,
    }
    
    result, err := pipeline.Process(context.Background(), user)
    if err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
        return
    }
    
    fmt.Printf("Processed user: %+v\n", result)
}

// Validation can fail
func validateUser(ctx context.Context, u User) (User, error) {
    if u.Email == "" {
        return u, errors.New("email required")
    }
    if u.Age < 18 {
        return u, errors.New("must be 18 or older")
    }
    return u, nil
}

// Transformation can't fail
func normalizeUser(ctx context.Context, u User) User {
    u.Email = strings.ToLower(u.Email)
    u.Verified = true
    return u
}

// Side effect (logging)
func logUser(ctx context.Context, u User) error {
    fmt.Printf("User registered: %s\n", u.Email)
    return nil
}
```

## Understanding the Building Blocks

### Processors: Transform Your Data

pipz provides different processor types for different needs:

```go
// Transform: Pure functions that can't fail
upperCase := pipz.Transform("upper", func(ctx context.Context, s string) string {
    return strings.ToUpper(s)
})

// Apply: Operations that can fail
parse := pipz.Apply("parse", func(ctx context.Context, s string) (Data, error) {
    var data Data
    err := json.Unmarshal([]byte(s), &data)
    return data, err
})

// Effect: Side effects without modifying data
log := pipz.Effect("log", func(ctx context.Context, d Data) error {
    fmt.Printf("Processing: %+v\n", d)
    return nil
})
```

### Connectors: Control the Flow

Connectors determine how processors are executed:

```go
// Sequence: Run steps in order
sequential := pipz.NewSequence[Data]("steps", step1, step2, step3)

// Concurrent: Run in parallel (requires Clone())
parallel := pipz.NewConcurrent[Data]("parallel", task1, task2, task3)

// Fallback: Try primary, use backup on failure
resilient := pipz.NewFallback[Data]("resilient", primary, backup)

// Switch: Route based on conditions
router := pipz.NewSwitch[Data, string]("router", getType).
    AddRoute("typeA", processTypeA).
    AddRoute("typeB", processTypeB)
```

## Building a Real-World Pipeline

Let's create a more realistic example - an order processing pipeline with validation, enrichment, and error handling:

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"
    
    "github.com/zoobzio/pipz"
)

type Order struct {
    ID         string
    CustomerID string
    Items      []Item
    Total      float64
    Status     string
    CreatedAt  time.Time
}

type Item struct {
    ProductID string
    Quantity  int
    Price     float64
}

// Implement Clone for concurrent processing
func (o Order) Clone() Order {
    items := make([]Item, len(o.Items))
    copy(items, o.Items)
    return Order{
        ID:         o.ID,
        CustomerID: o.CustomerID,
        Items:      items,
        Total:      o.Total,
        Status:     o.Status,
        CreatedAt:  o.CreatedAt,
    }
}

func createOrderPipeline() pipz.Chainable[Order] {
    return pipz.NewSequence[Order]("order-processing",
        // Validation phase
        pipz.Apply("validate", validateOrder),
        
        // Enrichment phase
        pipz.Transform("calculate-total", calculateTotal),
        pipz.Enrich("add-customer-data", enrichWithCustomerData),
        
        // Processing phase
        pipz.Apply("check-inventory", checkInventory),
        pipz.Apply("process-payment", processPayment),
        
        // Parallel notifications
        pipz.NewConcurrent[Order]("notifications",
            pipz.Effect("email", sendEmailConfirmation),
            pipz.Effect("sms", sendSMSNotification),
            pipz.Effect("analytics", trackOrderMetrics),
        ),
        
        // Final status update
        pipz.Transform("complete", func(ctx context.Context, o Order) Order {
            o.Status = "completed"
            return o
        }),
    )
}

func validateOrder(ctx context.Context, o Order) (Order, error) {
    if len(o.Items) == 0 {
        return o, errors.New("order must have items")
    }
    if o.CustomerID == "" {
        return o, errors.New("customer ID required")
    }
    return o, nil
}

func calculateTotal(ctx context.Context, o Order) Order {
    total := 0.0
    for _, item := range o.Items {
        total += item.Price * float64(item.Quantity)
    }
    o.Total = total
    return o
}

func enrichWithCustomerData(ctx context.Context, o Order) (Order, error) {
    // This is optional - won't fail the pipeline
    customer, err := fetchCustomer(o.CustomerID)
    if err != nil {
        // Log but continue
        fmt.Printf("Could not enrich with customer data: %v\n", err)
        return o, err // Enrich logs but doesn't fail
    }
    // Add customer tier for discounts, etc.
    o.Status = fmt.Sprintf("processing-%s", customer.Tier)
    return o, nil
}

func checkInventory(ctx context.Context, o Order) (Order, error) {
    for _, item := range o.Items {
        available, err := getInventoryCount(item.ProductID)
        if err != nil {
            return o, fmt.Errorf("inventory check failed: %w", err)
        }
        if available < item.Quantity {
            return o, fmt.Errorf("insufficient inventory for %s", item.ProductID)
        }
    }
    return o, nil
}

func processPayment(ctx context.Context, o Order) (Order, error) {
    // Process payment...
    fmt.Printf("Processing payment of $%.2f\n", o.Total)
    return o, nil
}

func sendEmailConfirmation(ctx context.Context, o Order) error {
    fmt.Printf("Sending email for order %s\n", o.ID)
    return nil
}

func sendSMSNotification(ctx context.Context, o Order) error {
    fmt.Printf("Sending SMS for order %s\n", o.ID)
    return nil
}

func trackOrderMetrics(ctx context.Context, o Order) error {
    fmt.Printf("Tracking metrics for order %s: $%.2f\n", o.ID, o.Total)
    return nil
}
```

## Adding Resilience

Real-world systems need error handling, retries, and timeouts:

```go
func createResilientPipeline() pipz.Chainable[Order] {
    // Basic pipeline
    basicPipeline := createOrderPipeline()
    
    // Add resilience layers
    return pipz.NewTimeout("timeout-protection",
        pipz.NewRetry("retry-on-failure",
            pipz.NewFallback("with-fallback",
                basicPipeline,
                pipz.Apply("fallback", processFallbackOrder),
            ),
            3, // Retry up to 3 times
        ),
        30*time.Second, // Overall timeout
    )
}

func processFallbackOrder(ctx context.Context, o Order) (Order, error) {
    // Simplified processing for fallback
    o.Status = "pending-manual-review"
    fmt.Printf("Order %s sent for manual review\n", o.ID)
    return o, nil
}
```

## Error Handling

pipz provides rich error information:

```go
func handlePipelineError(err error) {
    var pipeErr *pipz.Error[Order]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Pipeline failed at stage: %s\n", pipeErr.Stage)
        fmt.Printf("Error: %v\n", pipeErr.Cause)
        fmt.Printf("Order state at failure: %+v\n", pipeErr.State)
        
        if pipeErr.Timeout {
            fmt.Println("Failure was due to timeout")
        }
    }
}

func main() {
    pipeline := createResilientPipeline()
    
    order := Order{
        ID:         "ORD-123",
        CustomerID: "CUST-456",
        Items: []Item{
            {ProductID: "PROD-1", Quantity: 2, Price: 29.99},
            {ProductID: "PROD-2", Quantity: 1, Price: 49.99},
        },
        CreatedAt: time.Now(),
    }
    
    result, err := pipeline.Process(context.Background(), order)
    if err != nil {
        handlePipelineError(err)
        return
    }
    
    fmt.Printf("Order processed successfully: %+v\n", result)
}
```

## Advanced Patterns

### Conditional Processing

```go
// Route orders based on value
valueRouter := pipz.NewSwitch[Order, string]("value-router",
    func(ctx context.Context, o Order) string {
        if o.Total > 1000 {
            return "high-value"
        }
        if o.Total > 100 {
            return "standard"
        }
        return "low-value"
    },
).
AddRoute("high-value", highValuePipeline).
AddRoute("standard", standardPipeline).
AddRoute("low-value", lowValuePipeline)
```

### Rate Limiting

```go
// Protect external APIs
var apiLimiter = pipz.NewRateLimiter[Order]("api-limit", 
    100,              // 100 requests per second
    10,               // Burst of 10
)

protectedAPI := pipz.NewSequence[Order]("protected",
    apiLimiter,
    pipz.Apply("api-call", callExternalAPI),
)
```

### Circuit Breaking

```go
// Prevent cascading failures
circuitBreaker := pipz.NewCircuitBreaker[Order]("breaker",
    externalService,
    5,                  // Open after 5 failures
    30*time.Second,     // Try recovery after 30s
)
```

## Testing Your Pipelines

Pipelines are easy to test:

```go
func TestOrderPipeline(t *testing.T) {
    pipeline := createOrderPipeline()
    
    // Test valid order
    validOrder := Order{
        ID:         "TEST-1",
        CustomerID: "CUST-1",
        Items:      []Item{{ProductID: "P1", Quantity: 1, Price: 10.00}},
    }
    
    result, err := pipeline.Process(context.Background(), validOrder)
    assert.NoError(t, err)
    assert.Equal(t, "completed", result.Status)
    assert.Equal(t, 10.00, result.Total)
    
    // Test invalid order
    invalidOrder := Order{ID: "TEST-2"} // Missing customer ID
    
    _, err = pipeline.Process(context.Background(), invalidOrder)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "customer ID required")
}
```

## Best Practices

1. **Use constants for processor names**
```go
const (
    StageValidate = "validate"
    StageEnrich   = "enrich"
    StageProcess  = "process"
)
```

2. **Implement Clone() properly for concurrent processing**
```go
func (d Data) Clone() Data {
    // Deep copy all reference types
    newSlice := make([]Item, len(d.Items))
    copy(newSlice, d.Items)
    return Data{Items: newSlice}
}
```

3. **Respect context cancellation**
```go
func slowOperation(ctx context.Context, data Data) (Data, error) {
    select {
    case <-ctx.Done():
        return data, ctx.Err()
    case result := <-doWork(data):
        return result, nil
    }
}
```

4. **Create singletons for stateful connectors**
```go
// RIGHT - Shared instance
var rateLimiter = pipz.NewRateLimiter[Data]("api", 100, 10)

// WRONG - New instance each time
func process(data Data) {
    limiter := pipz.NewRateLimiter[Data]("api", 100, 10) // Don't do this!
}
```

## Next Steps

Now that you understand the basics:

1. Explore the [Core Concepts](../learn/core-concepts.md) for deeper understanding
2. Check the [Cookbook](../cookbook/) for real-world recipes
3. Browse the [API Reference](../reference/) for detailed documentation
4. Review the [Cheatsheet](../reference/cheatsheet.md) for quick decisions

## Common Questions

**Q: When should I use pipz instead of regular Go code?**  
A: Use pipz when you need composable, reusable data processing with good error handling and built-in patterns like retry, circuit breaking, and rate limiting.

**Q: How does pipz compare to other pipeline libraries?**  
A: pipz focuses on type safety, simplicity, and composability. It's lighter than workflow engines like Temporal but more structured than basic function chaining.

**Q: Can I use pipz for streaming data?**  
A: pipz processes one item at a time. For streaming, wrap your stream processing in pipz processors or check the [Event Processing](../cookbook/event-processing.md) recipe.

**Q: How do I handle errors in the middle of a pipeline?**  
A: Use `Fallback` for recovery, `Handle` for cleanup, or check the [Safety and Reliability](../guides/safety-reliability.md) guide.

## Getting Help

- Check the [Troubleshooting](../troubleshooting.md) guide
- Browse [Examples](https://github.com/zoobzio/pipz/tree/main/examples)
- Open an [Issue](https://github.com/zoobzio/pipz/issues) on GitHub

Happy pipelining! 🚀