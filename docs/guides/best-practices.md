# Best Practices

Guidelines for building production-ready pipelines with pipz.

## Design Principles

### 1. Single Responsibility

Each processor should do one thing well:

```go
// Good: Focused processors
validateEmail := pipz.Apply("validate_email", func(ctx context.Context, u User) (User, error) {
    if !isValidEmail(u.Email) {
        return u, errors.New("invalid email format")
    }
    return u, nil
})

normalizeEmail := pipz.Transform("normalize_email", func(ctx context.Context, u User) User {
    u.Email = strings.ToLower(strings.TrimSpace(u.Email))
    return u
})

// Bad: Doing too much
processEmail := pipz.Apply("process_email", func(ctx context.Context, u User) (User, error) {
    // Validating AND normalizing AND checking duplicates
    u.Email = strings.ToLower(strings.TrimSpace(u.Email))
    if !isValidEmail(u.Email) {
        return u, errors.New("invalid email")
    }
    if emailExists(u.Email) {
        return u, errors.New("email exists")
    }
    return u, nil
})
```

### 2. Const-Driven Naming

Use constants for all processor and connector names. **This isn't just good style - it's what makes your pipelines composable and modifiable at runtime.**

#### Why Const-Driven Naming is Critical

Constants serve as **keys** that enable dynamic pipeline modification:

```go
const (
    ProcessorValidate = "validate_order"
    ProcessorPrice    = "calculate_price"
    ProcessorPayment  = "process_payment"
)

// Later, you can modify pipelines using these keys:
orderPipeline.After(ProcessorValidate, fraudDetection)
orderPipeline.Replace(ProcessorPayment, testPaymentProcessor)
orderPipeline.Remove(ProcessorPrice) // For free orders
```

Without constants, you lose this composability - string literals make dynamic modification error-prone and brittle.

#### The Pattern

```go
// Define all names as constants - these become your pipeline's "keys"
const (
    // Processor names
    ProcessorValidateOrder     = "validate_order"
    ProcessorCheckInventory    = "check_inventory"
    ProcessorCalculatePrice    = "calculate_price"
    ProcessorApplyDiscounts    = "apply_discounts"
    ProcessorProcessPayment    = "process_payment"
    ProcessorUpdateInventory   = "update_inventory"
    ProcessorSendConfirmation  = "send_confirmation"
    ProcessorUpdateCRM         = "update_crm"
    ProcessorMarkPending       = "mark_pending"
    ProcessorApplyBulkDiscount = "apply_bulk_discount"
    
    // Connector names
    PipelineOrder          = "order-pipeline"
    ConnectorPaymentFlow   = "payment-flow"
    ConnectorNotifications = "notification-flow"
    PipelineExpressCheckout = "express-checkout"
    PipelineBulkOrders     = "bulk-orders"
)

// Use constants when creating processors
func createOrderProcessors() map[string]pipz.Chainable[Order] {
    return map[string]pipz.Chainable[Order]{
        ProcessorValidateOrder: pipz.Apply(ProcessorValidateOrder, func(ctx context.Context, o Order) (Order, error) {
            if o.Total <= 0 {
                return o, errors.New("invalid order total")
            }
            return o, nil
        }),
        
        ProcessorCheckInventory: pipz.Apply(ProcessorCheckInventory, func(ctx context.Context, o Order) (Order, error) {
            for _, item := range o.Items {
                if !inventory.Has(item.SKU, item.Quantity) {
                    return o, fmt.Errorf("insufficient inventory for %s", item.SKU)
                }
            }
            return o, nil
        }),
        
        ProcessorCalculatePrice: pipz.Transform(ProcessorCalculatePrice, func(ctx context.Context, o Order) Order {
            o.Subtotal = calculateSubtotal(o.Items)
            o.Tax = calculateTax(o.Subtotal, o.ShippingAddress)
            o.Total = o.Subtotal + o.Tax + o.ShippingCost
            return o
        }),
    }
}

// Build pipelines using constants
func createOrderPipeline() pipz.Chainable[Order] {
    seq := pipz.NewSequence[Order](PipelineOrder)
    seq.Register(
        pipz.Apply(ProcessorValidateOrder, validateOrder),
        pipz.Apply(ProcessorCheckInventory, checkInventory),
        pipz.Transform(ProcessorCalculatePrice, calculatePrice),
        pipz.Transform(ProcessorApplyDiscounts, applyDiscounts),
        
        // Payment with retry
        pipz.NewRetry(ConnectorPaymentFlow,
            pipz.Apply(ProcessorProcessPayment, processPayment),
            3,
        ),
        
        pipz.Apply(ProcessorUpdateInventory, updateInventory),
        
        // Notifications with timeout
        pipz.NewTimeout(ConnectorNotifications,
            pipz.Effect(ProcessorSendConfirmation, sendConfirmation),
            5*time.Second,
        ),
    )
    return seq
}

// Example: Dynamic pipeline composition
func createCustomPipeline(config Config) pipz.Chainable[Order] {
    seq := pipz.NewSequence[Order](config.Name)
    
    // Always validate
    seq.Register(pipz.Apply(ProcessorValidateOrder, validateOrder))
    
    // Conditionally add processors
    if config.CheckInventory {
        seq.Register(pipz.Apply(ProcessorCheckInventory, checkInventory))
    }
    
    seq.Register(pipz.Transform(ProcessorCalculatePrice, calculatePrice))
    
    if config.DiscountsEnabled {
        seq.Register(pipz.Transform(ProcessorApplyDiscounts, applyDiscounts))
    }
    
    return seq
}
```

Benefits of const-driven naming:

1. **Consistency**: All names follow a predictable pattern
2. **No Magic Strings**: Constants prevent typos and runtime errors
3. **IDE Support**: Auto-completion and refactoring work perfectly
4. **Debugging**: Error messages show meaningful processor names
5. **Searchability**: Easy to find all usages of a processor
6. **Documentation**: Constants serve as self-documenting code

```go
// Example: Error messages are more helpful with const names
result, err := pipeline.Process(ctx, order)
if err != nil {
    // Error: "failed at [order-pipeline -> validate_order]: invalid order total"
    // Much clearer than: "failed at [pipeline1 -> proc1]: invalid order total"
}

// Example: Easy to find and modify pipelines
func addFraudCheck(seq *pipz.Sequence[Order]) {
    // Find where to insert fraud check
    for i, name := range seq.Names() {
        if name == ProcessorValidateOrder {
            // Insert after validation
            seq.InsertAt(i+1, pipz.Apply(ProcessorFraudCheck, checkFraud))
            break
        }
    }
}
```

### 3. Explicit Error Handling

Make error handling visible and intentional:

```go
// Good: Clear error strategy
payment := pipz.NewSequence("payment-flow",
    pipz.Apply("validate", validatePayment),
    
    // Critical operation with retry
    pipz.RetryWithBackoff(
        pipz.Apply("charge", chargeCard),
        3,
        time.Second,
    ),
    
    // Non-critical with error handler
    pipz.NewHandle("notify-with-logging",
        pipz.Effect("notify", sendNotification),
        pipz.Effect("log_notification_error", func(ctx context.Context, err *pipz.Error[Order]) error {
            log.Printf("Notification failed for order %s: %v", err.InputData.ID, err.Err)
            return nil
        }),
    ),
)

// Bad: Hidden error handling
payment := pipz.Apply("process", func(ctx context.Context, p Payment) (Payment, error) {
    // Validation, charging, and notification all mixed together
    // Error handling buried in function
})
```

### 4. Type-Safe Routes

Use custom types for routing decisions:

```go
// Good: Type-safe enum
type OrderPriority int
const (
    PriorityStandard OrderPriority = iota
    PriorityExpress
    PriorityOvernight
)

router := pipz.Switch(
    func(ctx context.Context, o Order) OrderPriority {
        if o.ShippingMethod == "overnight" {
            return PriorityOvernight
        }
        // ...
    },
    map[OrderPriority]pipz.Chainable[Order]{
        PriorityStandard:  standardFulfillment,
        PriorityExpress:   expressFulfillment,
        PriorityOvernight: overnightFulfillment,
    },
)

// Bad: Magic strings
router := pipz.Switch(
    func(ctx context.Context, o Order) string {
        return o.ShippingMethod // "standard", "express", etc.
    },
    map[string]pipz.Chainable[Order]{
        "standard": standardFulfillment,
        "express":  expressFulfillment,
        // Easy to typo, no compile-time checking
    },
)
```

## Pipeline Patterns

### Pattern: Concurrent vs Sequential

Understanding when to use Concurrent versus Sequential is critical for performance:

#### Use Concurrent for I/O Operations

Concurrent is designed for operations with real latency where parallel execution provides benefit:

```go
// GOOD: Parallel API calls with actual network latency
notifications := pipz.NewConcurrent("send-notifications",
    sendEmailNotification,      // API call to email service
    sendSMSNotification,       // API call to SMS gateway
    updateCRMSystem,           // API call to CRM
    pushToAnalytics,           // API call to analytics service
)

// GOOD: Multiple database queries that don't depend on each other
enrichment := pipz.NewConcurrent("enrich-data",
    fetchUserProfile,          // Database query
    fetchAccountHistory,       // Database query
    fetchPreferences,          // Database query
)
```

#### Use Sequential for Fast Operations

Sequential is better for CPU-bound operations or fast validations:

```go
// BAD: Using Concurrent for simple validations
validations := pipz.NewConcurrent("validate",  // ❌ Don't do this!
    validateEmail,     // Simple regex check
    validateAge,       // Number comparison
    validateCountry,   // String check
)

// GOOD: Sequential for fast operations
validations := pipz.NewSequence("validate",     // ✅ Better!
    validateEmail,
    validateAge,
    validateCountry,
)
```

#### Performance Implications

Concurrent creates copies of your data (using the Cloner interface) for goroutine isolation:

```go
// This happens internally in Concurrent:
// 1. data.Clone() called for each processor
// 2. Goroutine spawned for each
// 3. Original input returned (modifications discarded)

// For simple operations, the overhead of:
// - Cloning data
// - Spawning goroutines
// - Channel communication
// ...exceeds the benefit of parallelism
```

Rule of thumb: If an operation takes less than 10ms, use Sequential.

### Pattern: Validation First

Always validate early to fail fast:

```go
pipeline := pipz.NewSequence("validation-pipeline",
    // Validate first - cheap and catches errors early
    pipz.Apply("validate_structure", validateStructure),
    pipz.Apply("validate_business_rules", validateBusinessRules),
    
    // Then expensive operations
    pipz.Apply("enrich_from_api", enrichFromAPI),
    pipz.Apply("calculate_pricing", calculatePricing),
    pipz.Apply("save_to_database", saveToDatabase),
)
```

### Pattern: Graceful Degradation

Continue processing even when non-critical operations fail:

```go
pipeline := pipz.NewSequence("processing-pipeline",
    // Critical path
    pipz.Apply("validate", validate),
    pipz.Apply("process", process),
    pipz.Apply("save", save),
    
    // Best-effort enrichments
    pipz.Enrich("add_recommendations", func(ctx context.Context, order Order) (Order, error) {
        recs, err := recommendationService.Get(ctx, order.UserID)
        if err != nil {
            return order, err // Enrich will ignore this
        }
        order.Recommendations = recs
        return order, nil
    }),
    
    // Non-blocking notifications
    pipz.Concurrent(
        pipz.WithErrorHandler(
            pipz.Effect("email", sendEmail),
            pipz.Effect("log_email_error", logError),
        ),
        pipz.Effect("analytics", trackAnalytics),
    ),
)
```

### Pattern: Bulkhead Isolation

Isolate failures to prevent cascade. The key insight is that bulkheads are simple to connect when operating on the same type:

```go
// Separate pipelines for different concerns
orderPipeline := pipz.NewSequence("order-pipeline",
    validateOrder,
    processPayment,
    updateInventory,
)

// Isolated notification pipeline
notificationPipeline := pipz.Timeout(
    pipz.Concurrent(
        sendEmail,
        sendSMS,
        updateCRM,
    ),
    5*time.Second, // Don't let notifications block orders
)

// Compose with isolation - easy because both operate on Order type
fullPipeline := pipz.NewSequence("full-pipeline",
    orderPipeline,
    pipz.WithErrorHandler(
        notificationPipeline,
        pipz.Effect("log_notification_failure", logError),
    ),
)
```

### Pattern: Orchestration Pipeline

The ultimate pattern is to build workflows as individual pipelines for discrete types, then create an orchestration pipeline that coordinates them:

```go
// Step 1: Build discrete pipelines for each type
type OrderWorkflow struct {
    Order    Order
    Customer Customer
    Invoice  Invoice
    Config   WorkflowConfig
}

// Individual pipelines for each domain object
orderPipeline := pipz.NewSequence[Order]("order-workflow",
    validateOrder,
    calculatePricing,
    applyDiscounts,
)

customerPipeline := pipz.NewSequence[Customer]("customer-workflow",
    validateCustomer,
    checkCreditLimit,
    updateLoyaltyPoints,
)

invoicePipeline := pipz.NewSequence[Invoice]("invoice-workflow",
    generateInvoice,
    applyTaxes,
    formatForDisplay,
)

// Step 2: Create orchestration pipeline that coordinates execution
orchestrator := pipz.NewSequence[OrderWorkflow]("orchestrator",
    // Process order first
    pipz.Apply("process_order", func(ctx context.Context, wf OrderWorkflow) (OrderWorkflow, error) {
        processed, err := orderPipeline.Process(ctx, wf.Order)
        if err != nil {
            return wf, fmt.Errorf("order processing failed: %w", err)
        }
        wf.Order = processed
        return wf, nil
    }),
    
    // Process customer in parallel with invoice if order succeeded
    pipz.Apply("process_customer_invoice", func(ctx context.Context, wf OrderWorkflow) (OrderWorkflow, error) {
        var wg sync.WaitGroup
        var custErr, invErr error
        
        wg.Add(2)
        
        // Process customer
        go func() {
            defer wg.Done()
            processed, err := customerPipeline.Process(ctx, wf.Customer)
            if err != nil {
                custErr = err
                return
            }
            wf.Customer = processed
        }()
        
        // Process invoice
        go func() {
            defer wg.Done()
            // Invoice needs order data
            wf.Invoice.OrderID = wf.Order.ID
            wf.Invoice.Amount = wf.Order.Total
            
            processed, err := invoicePipeline.Process(ctx, wf.Invoice)
            if err != nil {
                invErr = err
                return
            }
            wf.Invoice = processed
        }()
        
        wg.Wait()
        
        // Handle errors based on config
        if custErr != nil && wf.Config.CustomerRequired {
            return wf, fmt.Errorf("customer processing failed: %w", custErr)
        }
        if invErr != nil && wf.Config.InvoiceRequired {
            return wf, fmt.Errorf("invoice processing failed: %w", invErr)
        }
        
        return wf, nil
    }),
    
    // Final coordination
    pipz.Apply("finalize", func(ctx context.Context, wf OrderWorkflow) (OrderWorkflow, error) {
        // Any final coordination logic
        if wf.Config.SendConfirmation {
            // Send order confirmation with all processed data
            sendOrderConfirmation(wf.Order, wf.Customer, wf.Invoice)
        }
        return wf, nil
    }),
)

// Usage
workflow := OrderWorkflow{
    Order:    order,
    Customer: customer,
    Invoice:  Invoice{},
    Config: WorkflowConfig{
        CustomerRequired: true,
        InvoiceRequired:  false,
        SendConfirmation: true,
    },
}

result, err := orchestrator.Process(ctx, workflow)
```

This pattern provides:
- **Type Safety**: Each pipeline operates on its specific type
- **Reusability**: Individual pipelines can be reused in different workflows
- **Testability**: Each pipeline can be tested independently
- **Flexibility**: Orchestration logic can change without modifying domain pipelines
- **Clear Dependencies**: The orchestrator explicitly manages data flow between pipelines

### Pattern: Feature Flags

Support gradual rollouts and A/B testing:

```go
func createPipeline(features FeatureFlags) pipz.Chainable[Order] {
    processors := []pipz.Chainable[Order]{
        validateOrder,
        calculatePricing,
    }
    
    // Conditionally add processors
    if features.IsEnabled("fraud-detection-v2") {
        processors = append(processors, fraudDetectionV2)
    } else {
        processors = append(processors, fraudDetectionV1)
    }
    
    if features.IsEnabled("loyalty-points") {
        processors = append(processors, calculateLoyaltyPoints)
    }
    
    processors = append(processors, 
        chargePayment,
        fulfillOrder,
    )
    
    seq := pipz.NewSequence("pipeline", processors...)
    return seq
}
```

## Error Handling Strategies

### Strategy: Categorize and Route

```go
type ErrorCategory string

const (
    CategoryValidation ErrorCategory = "validation"
    CategoryTransient  ErrorCategory = "transient"
    CategoryBusiness   ErrorCategory = "business"
    CategorySystem     ErrorCategory = "system"
)

func handleError(ctx context.Context, failure FailedOrder) (FailedOrder, error) {
    category := categorizeError(failure.Error)
    
    switch category {
    case CategoryValidation:
        // Don't retry bad input
        return sendToDeadLetter(ctx, failure)
        
    case CategoryTransient:
        // Retry with backoff
        return pipz.RetryWithBackoff(
            reprocessOrder,
            5,
            time.Second,
        ).Process(ctx, failure)
        
    case CategoryBusiness:
        // Needs human intervention
        return sendToManualReview(ctx, failure)
        
    case CategorySystem:
        // Alert ops team
        alertOps(failure.Error)
        return sendToDeadLetter(ctx, failure)
    }
    
    return failure, failure.Error
}
```

### Strategy: Circuit Breaker per Dependency

```go
var (
    stripeBreaker = NewCircuitBreaker("stripe", 5, 30*time.Second)
    dbBreaker     = NewCircuitBreaker("database", 10, 1*time.Minute)
    cacheBreaker  = NewCircuitBreaker("cache", 20, 10*time.Second)
)

pipeline := pipz.NewSequence("circuit-breaker-pipeline",
    // Wrap external calls with circuit breakers
    pipz.Apply("load_from_cache", func(ctx context.Context, id string) (Order, error) {
        return cacheBreaker.Do(ctx, func() (Order, error) {
            return cache.Get(ctx, id)
        })
    }),
    
    pipz.Apply("charge_payment", func(ctx context.Context, order Order) (Order, error) {
        return stripeBreaker.Do(ctx, func() (Order, error) {
            return stripe.Charge(ctx, order)
        })
    }),
    
    pipz.Apply("save_order", func(ctx context.Context, order Order) (Order, error) {
        return dbBreaker.Do(ctx, func() (Order, error) {
            return db.Save(ctx, order)
        })
    }),
)
```


## Production Checklist

### Design
- [ ] Each processor has single responsibility
- [ ] Error handling is explicit and appropriate
- [ ] Uses type-safe routing (no magic strings)
- [ ] Validates input early
- [ ] Non-critical operations don't block critical path

### Resilience
- [ ] Timeouts on all external calls
- [ ] Retry logic for transient failures
- [ ] Circuit breakers for dependencies
- [ ] Graceful degradation for features
- [ ] Bulkhead isolation between components

### Observability
- [ ] Metrics on all processors
- [ ] Distributed tracing enabled
- [ ] Structured logging throughout
- [ ] Error categorization and alerting
- [ ] Performance benchmarks

### Operations
- [ ] Feature flags for gradual rollout
- [ ] Dead letter queue for failed items
- [ ] Manual intervention workflow
- [ ] Runbooks for common issues
- [ ] Load testing completed

## Anti-Patterns to Avoid

1. **God Processor**: One processor doing everything
2. **Silent Failures**: Swallowing errors without logging
3. **Unbounded Retries**: Retrying forever without backoff
4. **Missing Timeouts**: No time bounds on operations
5. **Shared Mutable State**: Processors modifying shared data
6. **Magic Strings**: Using strings instead of typed constants
7. **Hidden Dependencies**: Processors with side effects on external state

## Next Steps

- [Performance Guide](./performance.md) - Optimize for production
- [Error Recovery](./error-recovery.md) - Advanced error patterns