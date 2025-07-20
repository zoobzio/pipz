# pipz

Type-safe, composable data pipelines for Go with zero dependencies.

Build robust data processing pipelines that are easy to test, reason about, and maintain. Perfect for ETL, API middleware, event processing, and any multi-stage data transformation.

**Quick Start:** `make try` - Run interactive demos to see pipz in action!

```go
// Build a payment processing pipeline
pipeline := pipz.Sequential(
    pipz.Apply("validate", validateOrder),
    pipz.Apply("check_inventory", checkInventory),
    pipz.Apply("check_fraud", checkFraud),
    pipz.Fallback(
        pipz.Apply("charge_primary", chargeCreditCard),
        pipz.Apply("charge_backup", chargePayPal),
    ),
    pipz.Effect("send_receipt", emailReceipt),
)

// Process with full context support
order, err := pipeline.Process(ctx, orderRequest)
```

## 📋 Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Interactive Demo](#interactive-demo)
- [Core Concepts](#core-concepts)
- [Why pipz?](#why-pipz)
- [API Reference](#api-reference)
- [Building Workflows](#building-workflows)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Performance](#performance)
- [Contributing](#contributing)


## Installation

```bash
go get github.com/zoobzio/pipz
```

## Interactive Demo

Try our interactive demos to see pipz in action:

```bash
# Run interactive demo menu
make demo

# List available demos
make demo-list

# Run specific demos
make demo-run DEMO=validation  # Business rule validation
make demo-run DEMO=payment     # Payment processing with failover
make demo-run DEMO=etl         # Data transformation pipeline
make demo-run DEMO=webhook     # Multi-provider webhook handling
make demo-run DEMO=security    # Security audit pipeline
make demo-run DEMO=events      # Event routing and deduplication
make demo-run DEMO=ai          # LLM integration with caching
make demo-run DEMO=middleware  # HTTP middleware patterns
make demo-run DEMO=workflow    # Complex order processing workflows
make demo-run DEMO=moderation  # Content moderation pipelines

# Run all demos sequentially
make demo-all

# Or install the CLI globally
make install
pipz demo
```

## Quick Start

### 1. Define Your Data Types

```go
type Order struct {
    ID       string
    Customer string
    Items    []Item
    Total    float64
}
```

### 2. Create Processors

```go
// Validation
validateOrder := pipz.Effect("validate", func(ctx context.Context, o Order) error {
    if len(o.Items) == 0 {
        return errors.New("order must have items")
    }
    return nil
})

// Transformation
applyTax := pipz.Transform("tax", func(ctx context.Context, o Order) Order {
    o.Total = o.Total * 1.08 // 8% tax
    return o
})

// Side effects
logOrder := pipz.Effect("log", func(ctx context.Context, o Order) error {
    log.Printf("Processing order %s", o.ID)
    return nil
})
```

### 3. Build Your Pipeline

```go
orderPipeline := pipz.Sequential(
    validateOrder,
    applyTax,
    logOrder,
)

// Process orders
order, err := orderPipeline.Process(ctx, newOrder)
```

## Core Concepts

pipz follows a simple pattern that scales from basic operations to complex workflows:

```
Processors → Pipelines → Workflows
```

- **Processors**: Individual operations (validate, transform, enrich)
- **Pipelines**: Sequences of processors working together  
- **Workflows**: Composed pipelines with control flow (fallbacks, retries, routing)

This composable pattern lets you build complex business logic from simple, testable pieces.

## Why pipz?

pipz transforms scattered business logic into clean, testable pipelines. Here are three real-world examples showing how pipz solves increasingly complex business problems:

### 1. Order Validation (Simple Business Rules)

**The Problem:** Validating orders requires checking multiple business rules, and the code becomes a nested mess of if-statements and error handling.

**Without pipz:**

```go
func validateOrder(order Order) error {
    // Check order ID format
    if !strings.HasPrefix(order.ID, "ORD-") {
        return fmt.Errorf("invalid order ID format: %s", order.ID)
    }

    // Validate customer
    if order.CustomerID == "" {
        return errors.New("customer ID is required")
    }
    if !isValidUUID(order.CustomerID) {
        return fmt.Errorf("invalid customer ID format: %s", order.CustomerID)
    }

    // Check items
    if len(order.Items) == 0 {
        return errors.New("order must contain at least one item")
    }

    // Validate each item
    for i, item := range order.Items {
        if item.SKU == "" {
            return fmt.Errorf("item %d: SKU is required", i)
        }
        if item.Quantity <= 0 {
            return fmt.Errorf("item %d: quantity must be positive", i)
        }
        if item.Price < 0 {
            return fmt.Errorf("item %d: price cannot be negative", i)
        }
    }

    // Business rules
    if order.Total > 10000 {
        return errors.New("order exceeds maximum allowed amount")
    }

    // Calculate expected total
    var expectedTotal float64
    for _, item := range order.Items {
        expectedTotal += item.Price * float64(item.Quantity)
    }
    if math.Abs(expectedTotal-order.Total) > 0.01 {
        return fmt.Errorf("order total mismatch: expected %.2f, got %.2f", expectedTotal, order.Total)
    }

    return nil
}
```

**The pipz Way:**

```go
// Define focused validators
validateOrderID := pipz.Effect("order_id", func(_ context.Context, o Order) error {
    if !strings.HasPrefix(o.ID, "ORD-") {
        return fmt.Errorf("invalid order ID format")
    }
    return nil
})

validateCustomer := pipz.Effect("customer", func(_ context.Context, o Order) error {
    if o.CustomerID == "" {
        return errors.New("customer ID required")
    }
    return nil
})

validateItems := pipz.Effect("items", func(_ context.Context, o Order) error {
    if len(o.Items) == 0 {
        return errors.New("order must have items")
    }
    for _, item := range o.Items {
        if item.Quantity <= 0 {
            return errors.New("invalid item quantity")
        }
    }
    return nil
})

// Compose into a pipeline
validationPipeline := pipz.Sequential(
    validateOrderID,
    validateCustomer,
    validateItems,
    pipz.Effect("total", validateOrderTotal),
)

// Clean, testable, reusable
err := validationPipeline.Process(ctx, order)
```

### 2. Event Processing (Routing and Deduplication)

**The Problem:** Processing events from multiple sources requires routing, deduplication, and handling failures gracefully.

**Without pipz:**

```go
func processEvent(ctx context.Context, event Event) error {
    // Check if we've seen this event
    dedupKey := fmt.Sprintf("%s:%s", event.Source, event.ID)
    if seen, _ := cache.Get(dedupKey); seen != nil {
        log.Printf("Duplicate event: %s", event.ID)
        metrics.Inc("events.duplicate")
        return nil // Skip duplicates
    }

    // Store in dedup cache
    if err := cache.Set(dedupKey, true, 24*time.Hour); err != nil {
        log.Printf("Failed to update dedup cache: %v", err)
        // Don't fail on cache errors
    }

    // Validate event schema
    if event.Type == "" {
        metrics.Inc("events.invalid")
        return errors.New("event type is required")
    }
    if event.Timestamp.IsZero() {
        metrics.Inc("events.invalid")
        return errors.New("event timestamp is required")
    }

    // Route based on event type
    switch event.Type {
    case "user.created", "user.updated", "user.deleted":
        metrics.Inc("events.user")
        if err := handleUserEvent(ctx, event); err != nil {
            metrics.Inc("events.user.failed")
            // Check if retryable
            if isRetryable(err) {
                return fmt.Errorf("retryable error: %w", err)
            }
            // Send to DLQ
            if dlqErr := sendToDeadLetter(event, err); dlqErr != nil {
                log.Printf("Failed to send to DLQ: %v", dlqErr)
            }
            return err
        }

    case "order.placed", "order.shipped", "order.delivered":
        metrics.Inc("events.order")
        if err := handleOrderEvent(ctx, event); err != nil {
            metrics.Inc("events.order.failed")
            // Similar error handling...
            return err
        }

    case "payment.processed", "payment.failed":
        metrics.Inc("events.payment")
        // Payment events need special handling
        for attempt := 0; attempt < 3; attempt++ {
            err := handlePaymentEvent(ctx, event)
            if err == nil {
                break
            }
            if attempt < 2 {
                time.Sleep(time.Duration(attempt+1) * time.Second)
                continue
            }
            metrics.Inc("events.payment.failed")
            return err
        }

    default:
        metrics.Inc("events.unknown")
        log.Printf("Unknown event type: %s", event.Type)
        return fmt.Errorf("unknown event type: %s", event.Type)
    }

    // Update metrics
    metrics.Inc("events.processed")
    processingTime := time.Since(event.Timestamp)
    metrics.Histogram("events.processing_time", processingTime.Seconds())

    return nil
}
```

**The pipz Way:**

```go
// Event routing function
routeByType := func(_ context.Context, e Event) string {
    switch {
    case strings.HasPrefix(e.Type, "user."):
        return "user"
    case strings.HasPrefix(e.Type, "order."):
        return "order"
    case strings.HasPrefix(e.Type, "payment."):
        return "payment"
    default:
        return "unknown"
    }
}

// Build the pipeline
eventPipeline := pipz.Sequential(
    // Validation and deduplication
    pipz.Effect("validate", validateEventSchema),
    pipz.Apply("deduplicate", deduplicateEvent),
    pipz.Transform("enrich", enrichEventMetadata),

    // Route to appropriate handler
    pipz.Switch(routeByType, map[string]pipz.Chainable[Event]{
        "user":    handleUserEvents,
        "order":   handleOrderEvents,
        "payment": pipz.RetryWithBackoff(handlePaymentEvents, 3, time.Second),
        "unknown": pipz.Apply("dlq", sendToDeadLetter),
    }),

    // Always track metrics
    pipz.Effect("metrics", recordEventMetrics),
)

// Clean processing with all the same features
result, err := eventPipeline.Process(ctx, event)
```

### 3. Payment Processing (Complex Error Recovery)

**The Problem:** Payment processing requires sophisticated error handling, provider failover, and transaction recovery strategies.

**Without pipz:**

```go
func processPayment(ctx context.Context, payment Payment) (*TransactionResult, error) {
    // Validate payment details
    if err := validatePaymentDetails(payment); err != nil {
        log.Printf("Payment validation failed: %v", err)
        recordFailure("validation", payment, err)
        return nil, fmt.Errorf("validation: %w", err)
    }

    // Check fraud score
    fraudScore, err := checkFraudScore(ctx, payment)
    if err != nil {
        log.Printf("Fraud check failed: %v", err)
        // Don't fail on fraud check errors, just log
    } else if fraudScore > 0.8 {
        recordFailure("fraud", payment, nil)
        return nil, errors.New("payment flagged as fraudulent")
    }

    // Try primary provider (Stripe)
    result, err := chargeViaStripe(ctx, payment)
    if err == nil {
        recordSuccess("stripe", payment, result)
        return result, nil
    }

    // Log primary failure
    log.Printf("Stripe payment failed: %v", err)
    recordFailure("stripe", payment, err)

    // Check if error is retryable
    if isRetryableError(err) {
        // Try with exponential backoff
        for attempt := 1; attempt <= 3; attempt++ {
            time.Sleep(time.Duration(attempt*attempt) * time.Second)
            result, retryErr := chargeViaStripe(ctx, payment)
            if retryErr == nil {
                recordRecovery("stripe", payment, result, attempt)
                return result, nil
            }
            err = retryErr
        }
    }

    // Check if we should try alternate provider
    if !isTerminalError(err) {
        log.Printf("Attempting PayPal fallback")

        // Try PayPal
        result, paypalErr := chargeViaPayPal(ctx, payment)
        if paypalErr == nil {
            recordSuccess("paypal_fallback", payment, result)
            return result, nil
        }

        log.Printf("PayPal payment failed: %v", paypalErr)
        recordFailure("paypal", payment, paypalErr)

        // Both failed - try recovery based on error types
        if isInsufficientFunds(err) || isInsufficientFunds(paypalErr) {
            // Try smaller amount
            if payment.Amount > 100 {
                splitPayment := payment
                splitPayment.Amount = payment.Amount / 2

                result, splitErr := processPayment(ctx, splitPayment)
                if splitErr == nil {
                    // Process remainder
                    remainderPayment := payment
                    remainderPayment.Amount = payment.Amount - splitPayment.Amount
                    remainderResult, remainderErr := processPayment(ctx, remainderPayment)
                    if remainderErr == nil {
                        recordRecovery("split_payment", payment, result, 2)
                        return combineResults(result, remainderResult), nil
                    }
                }
            }
        }
    }

    // All attempts failed
    recordTotalFailure(payment, err)
    return nil, fmt.Errorf("all payment attempts failed: %w", err)
}
```

**The pipz Way:**

```go
// Define payment processors
validatePayment := pipz.Effect("validate", func(ctx context.Context, p Payment) error {
    return validatePaymentDetails(p)
})

checkFraud := pipz.Apply("fraud_check", func(ctx context.Context, p Payment) (Payment, error) {
    score, err := getFraudScore(ctx, p)
    if err != nil {
        return p, nil // Continue on fraud check errors
    }
    if score > 0.8 {
        return p, errors.New("high fraud risk")
    }
    p.FraudScore = score
    return p, nil
})

// Provider processors with built-in error categorization
stripeProcessor := pipz.Apply("stripe", func(ctx context.Context, p Payment) (Payment, error) {
    result, err := chargeViaStripe(ctx, p)
    if err != nil {
        return p, categorizeError("stripe", err)
    }
    p.Result = result
    return p, nil
})

paypalProcessor := pipz.Apply("paypal", func(ctx context.Context, p Payment) (Payment, error) {
    result, err := chargeViaPayPal(ctx, p)
    if err != nil {
        return p, categorizeError("paypal", err)
    }
    p.Result = result
    return p, nil
})

// Advanced recovery strategies
splitPaymentRecovery := pipz.Apply("split_payment", func(ctx context.Context, p Payment) (Payment, error) {
    if p.Amount <= 100 {
        return p, errors.New("amount too small to split")
    }
    // Process in two parts
    return processSplitPayment(ctx, p)
})

// Build sophisticated payment pipeline
paymentPipeline := pipz.Sequential(
    validatePayment,
    checkFraud,

    // Try primary with retries, fallback to PayPal, then try split payment
    pipz.Fallback(
        pipz.RetryWithBackoff(stripeProcessor, 3, time.Second),
        paypalProcessor,
        splitPaymentRecovery,
    ),

    // Always record metrics
    pipz.Effect("metrics", recordPaymentMetrics),
)

// All the complexity handled cleanly
payment, err := paymentPipeline.Process(ctx, paymentRequest)
```

**Benefits Across All Examples:**

- ✅ **Readable** - Business logic is clear and sequential
- ✅ **Testable** - Each processor can be tested in isolation
- ✅ **Reusable** - Processors can be shared across pipelines
- ✅ **Maintainable** - Easy to add, remove, or reorder steps
- ✅ **Debuggable** - Errors include processor names and context

## API Reference

### Core Types

| Type           | Description                                                  |
| -------------- | ------------------------------------------------------------ |
| `Chainable[T]` | Interface for any pipeline component that can process type T |
| `Processor[T]` | Basic unit of work that implements Chainable                 |
| `Pipeline[T]`  | Stateful pipeline that can be modified after creation        |

### Adapters

Adapters convert your functions into processors:

| Adapter     | Function Signature                                                                    | Purpose                  | Error Handling                   |
| ----------- | ------------------------------------------------------------------------------------- | ------------------------ | -------------------------------- |
| `Transform` | `func(context.Context, T) T`                                                          | Always modifies data     | Cannot fail                      |
| `Apply`     | `func(context.Context, T) (T, error)`                                                 | Can modify data          | Can return error                 |
| `Effect`    | `func(context.Context, T) error`                                                      | Side effects only        | Can return error                 |
| `Mutate`    | `transformer func(context.Context, T) T`<br>`condition func(context.Context, T) bool` | Conditional modification | Cannot fail                      |
| `Enrich`    | `func(context.Context, T) (T, error)`                                                 | Best-effort enhancement  | Continues with original on error |

### Connectors

Connectors compose processors into complex flows:

| Connector                                    | Purpose                        | Behavior                          |
| -------------------------------------------- | ------------------------------ | --------------------------------- |
| `Sequential(...Chainable[T])`                | Run processors in order        | Stops on first error              |
| `Pipeline[T]`                                | Mutable sequential pipeline    | Can add/remove/insert processors  |
| `Switch(router, map[string]Chainable[T])`    | Route to different processors  | Routes based on data content      |
| `Fallback(...Chainable[T])`                  | Try alternatives on failure    | Returns first success             |
| `Retry(Chainable[T], maxAttempts)`           | Retry on failure               | Fixed number of attempts          |
| `RetryWithBackoff(Chainable[T], max, delay)` | Retry with exponential backoff | Increases delay between attempts  |
| `Timeout(Chainable[T], duration)`            | Enforce time limits            | Returns timeout error if exceeded |
| `Concurrent(...Chainable[T])`                | Run processors in parallel     | Ignores errors, returns input     |
| `Race(...Chainable[T])`                      | Run in parallel, first wins    | Returns first success             |
| `WithErrorHandler(Chainable[T], Chainable[error])` | Handle processor errors   | Processes errors without failing   |

### Pipeline Methods

The `Pipeline[T]` type provides additional methods for dynamic pipeline modification:

| Method                                   | Description                                    |
| ---------------------------------------- | ---------------------------------------------- |
| `Register(...Chainable[T])`              | Add processors to the end                      |
| `PushHead(...Chainable[T])`              | Add processors to the beginning                |
| `PushTail(...Chainable[T])`              | Add processors to the end (alias for Register) |
| `InsertAt(index, ...Chainable[T])`       | Insert processors at specific position         |
| `RemoveAt(index) error`                  | Remove processor at index                      |
| `Clear()`                                | Remove all processors                          |
| `Len() int`                              | Get number of processors                       |
| `Process(context.Context, T) (T, error)` | Execute the pipeline                           |

## Building Workflows

Now that you understand the building blocks, let's walk through creating a complete workflow step-by-step. We'll build an order processing system that demonstrates how processors, pipelines, and workflows compose together with real-world patterns like routing, fallbacks, and parallel processing.

### Step 1: Define Your Data Types

```go
type Order struct {
    ID         string
    CustomerID string
    Items      []Item
    Total      float64
    Priority   string // "standard", "express", "overnight"
    Status     string
}

type Item struct {
    SKU      string
    Quantity int
    Price    float64
}

type PaymentResult struct {
    Order         Order
    TransactionID string
    Provider      string // "stripe", "paypal", "invoice"
}
```

### Step 2: Create Individual Processors

```go
// Validation processors - each handles one concern
validateOrder := pipz.Effect("validate_order", func(ctx context.Context, order Order) error {
    if len(order.Items) == 0 {
        return errors.New("order must have items")
    }
    if order.Total <= 0 {
        return errors.New("order total must be positive")
    }
    return nil
})

validateCustomer := pipz.Effect("validate_customer", func(ctx context.Context, order Order) error {
    active, err := db.IsCustomerActive(ctx, order.CustomerID)
    if err != nil {
        return fmt.Errorf("customer check failed: %w", err)
    }
    if !active {
        return errors.New("customer account is not active")
    }
    return nil
})

// Business logic processors
checkInventory := pipz.Apply("check_inventory", func(ctx context.Context, order Order) (Order, error) {
    for _, item := range order.Items {
        available, err := inventory.CheckStock(ctx, item.SKU, item.Quantity)
        if err != nil {
            return order, fmt.Errorf("inventory check failed: %w", err)
        }
        if !available {
            return order, fmt.Errorf("insufficient stock for %s", item.SKU)
        }
    }
    return order, nil
})

calculateTax := pipz.Transform("calculate_tax", func(ctx context.Context, order Order) Order {
    order.Total = order.Total * 1.08 // 8% tax
    return order
})

applyDiscounts := pipz.Mutate(
    "apply_discount",
    func(ctx context.Context, order Order) Order {
        order.Total = order.Total * 0.9 // 10% discount
        return order
    },
    func(ctx context.Context, order Order) bool {
        return order.Total > 100 // Only for orders over $100
    },
)

// Payment processors for different providers
chargeStripe := pipz.Apply("stripe_payment", func(ctx context.Context, order Order) (PaymentResult, error) {
    txID, err := stripe.Charge(ctx, order.CustomerID, order.Total)
    if err != nil {
        return PaymentResult{}, err
    }
    return PaymentResult{
        Order:         order,
        TransactionID: txID,
        Provider:      "stripe",
    }, nil
})

chargePayPal := pipz.Apply("paypal_payment", func(ctx context.Context, order Order) (PaymentResult, error) {
    txID, err := paypal.Charge(ctx, order.CustomerID, order.Total)
    if err != nil {
        return PaymentResult{}, err
    }
    return PaymentResult{
        Order:         order,
        TransactionID: txID,
        Provider:      "paypal",
    }, nil
})

createInvoice := pipz.Apply("invoice_payment", func(ctx context.Context, order Order) (PaymentResult, error) {
    // For B2B customers, create invoice instead of charging
    invoiceID, err := billing.CreateInvoice(ctx, order)
    if err != nil {
        return PaymentResult{}, err
    }
    return PaymentResult{
        Order:         order,
        TransactionID: invoiceID,
        Provider:      "invoice",
    }, nil
})

// Side effect processors
updateInventory := pipz.Effect("update_inventory", func(ctx context.Context, result PaymentResult) error {
    for _, item := range result.Order.Items {
        if err := inventory.Reserve(ctx, item.SKU, item.Quantity); err != nil {
            return err
        }
    }
    return nil
})

sendConfirmation := pipz.Effect("send_confirmation", func(ctx context.Context, result PaymentResult) error {
    return email.SendOrderConfirmation(ctx, result.Order.CustomerID, result.Order.ID)
})

logOrder := pipz.Effect("log_order", func(ctx context.Context, result PaymentResult) error {
    log.Printf("Order %s processed via %s (tx: %s)", 
        result.Order.ID, result.Provider, result.TransactionID)
    return nil
})
```

### Step 3: Combine Processors into Pipelines

```go
// Validation pipeline - all must pass
validationPipeline := pipz.Sequential(
    validateOrder,
    pipz.Concurrent(  // Check customer and inventory in parallel
        validateCustomer,
        checkInventory,
    ),
)

// Pricing pipeline - apply business rules
pricingPipeline := pipz.Sequential(
    applyDiscounts,   // Conditional discount
    calculateTax,     // Always apply tax
)

// Payment pipeline with fallback strategies
paymentPipeline := pipz.Fallback(
    pipz.RetryWithBackoff(chargeStripe, 3, time.Second),  // Try Stripe first with retries
    chargePayPal,                                          // Fallback to PayPal
    createInvoice,                                         // Last resort: invoice
)

// Post-payment pipeline - handle side effects
fulfillmentPipeline := pipz.Sequential(
    updateInventory,
    pipz.Concurrent(  // Send notifications in parallel
        pipz.WithErrorHandler(
            sendConfirmation,
            pipz.Sequential(
                pipz.Effect("increment_email_failures", incrementEmailFailureMetric),
                pipz.Apply("fallback_sms", sendSMSNotification),
            ),
        ),
        pipz.WithErrorHandler(
            updateSearchIndex,
            pipz.Effect("requeue_index", requeueForIndexing),
        ),
        logOrder,
    ),
)
```

### Step 4: Build the Complete Workflow

```go
// Route orders based on priority
routeByPriority := func(_ context.Context, order Order) string {
    return order.Priority
}

// Create specialized workflows for each priority level
standardWorkflow := pipz.Sequential(
    validationPipeline,
    pricingPipeline,
    pipz.Timeout(paymentPipeline, 30*time.Second),
    fulfillmentPipeline,
)

expressWorkflow := pipz.Sequential(
    validationPipeline,
    pricingPipeline,
    pipz.Timeout(paymentPipeline, 15*time.Second), // Shorter timeout
    pipz.Apply("expedite", func(ctx context.Context, result PaymentResult) (PaymentResult, error) {
        // Add express shipping
        return shipping.Expedite(ctx, result)
    }),
    fulfillmentPipeline,
)

// Main order processing workflow with routing
orderWorkflow := pipz.Switch(routeByPriority, map[string]pipz.Chainable[Order]{
    "standard":  standardWorkflow,
    "express":   expressWorkflow,
    "overnight": pipz.Sequential(
        validationPipeline,
        // Skip discounts for overnight orders
        calculateTax,
        pipz.Race( // Use fastest payment method
            chargeStripe,
            chargePayPal,
        ),
        fulfillmentPipeline,
    ),
})
```

### Step 5: Use Your Workflow

```go
func ProcessOrder(ctx context.Context, order Order) (*PaymentResult, error) {
    // The workflow handles all complexity internally
    result, err := orderWorkflow.Process(ctx, order)
    if err != nil {
        // Errors include processor names for debugging
        // e.g., "check_inventory: insufficient stock for SKU-123"
        return nil, fmt.Errorf("order processing failed: %w", err)
    }
    
    return &result, nil
}

// HTTP handler example
func HandleOrder(w http.ResponseWriter, r *http.Request) {
    var order Order
    if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    result, err := ProcessOrder(r.Context(), order)
    if err != nil {
        // Error messages include the failing processor for easy debugging
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}
```

### The Power of Composition

Now you can easily extend or modify your workflow:

```go
// Add fraud detection to the payment flow
fraudCheck := pipz.Apply("fraud_check", func(ctx context.Context, order Order) (Order, error) {
    score, err := fraud.CalculateRiskScore(ctx, order)
    if err != nil {
        return order, err
    }
    if score > 0.8 {
        return order, errors.New("high fraud risk")
    }
    return order, nil
})

// Create enhanced workflow with fraud detection
enhancedWorkflow := pipz.Sequential(
    validationPipeline,
    fraudCheck,          // New step inserted
    pricingPipeline,
    paymentPipeline,
    fulfillmentPipeline,
)

// Or create a B2B-specific workflow
b2bWorkflow := pipz.Sequential(
    validationPipeline,
    pipz.Apply("apply_contract_pricing", applyContractPricing),
    calculateTax,
    createInvoice,  // B2B always gets invoiced
    pipz.Sequential(
        updateInventory,
        pipz.Apply("notify_account_manager", notifyAccountManager),
        logOrder,
    ),
)

// Dynamic pipeline modification
pipeline := pipz.NewPipeline[Order]()
pipeline.Register(validationPipeline, pricingPipeline)

if config.FraudCheckEnabled {
    pipeline.InsertAt(1, fraudCheck)  // Insert after validation
}

if customer.IsVIP {
    pipeline.PushHead(pipz.Apply("vip_priority", prioritizeVIPOrder))
}
```

### Key Takeaways

1. **Start Small**: Build individual processors that do one thing well
2. **Compose Gradually**: Group related processors into pipelines
3. **Add Control Flow**: Use connectors like Fallback, Retry, Switch, Concurrent, and Race
4. **Stay Flexible**: Pipelines are just processors - compose them freely
5. **Type Safety**: Each pipeline works with a single type throughout
6. **Error Handling**: Use Fallback for alternatives, Enrich for non-critical operations
7. **Performance**: Use Concurrent/Race for parallel processing, Timeout for SLA enforcement

## Examples

For more production-ready examples, see the [examples directory](examples/):

- **[validation](examples/validation/)** - Business rule validation
- **[events](examples/events/)** - Event routing and deduplication
- **[payment](examples/payment/)** - Payment processing with failover
- **[webhook](examples/webhook/)** - Multi-provider webhook handling
- **[etl](examples/etl/)** - Data transformation pipelines
- **[ai](examples/ai/)** - LLM integration with caching
- **[middleware](examples/middleware/)** - HTTP middleware patterns
- **[workflow](examples/workflow/)** - Complex multi-stage business workflows
- **[moderation](examples/moderation/)** - Content moderation pipelines

## Best Practices

### 1. Name Your Processors

Always provide meaningful names for better debugging:

```go
// Good - errors will show "validate_age: age must be positive"
pipz.Effect("validate_age", checkAge)

// Bad - errors show function pointer
pipz.Effect("", checkAge)
```

### 2. Keep Processors Focused

Each processor should have a single responsibility:

```go
// Good - focused processors
validateEmail := pipz.Effect("email", checkEmail)
validateAge := pipz.Effect("age", checkAge)

// Bad - doing too much
validateEverything := pipz.Effect("validate_all", checkEmailAndAgeAndName)
```

### 3. Use Context Appropriately

Pass configuration and cancellation through context:

```go
ctx = context.WithValue(ctx, configKey, config)
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

result, err := pipeline.Process(ctx, data)
```

### 4. Choose the Right Adapter

- Use `Transform` when you always modify data and can't fail
- Use `Apply` when you might modify data or might fail
- Use `Effect` for validation, logging, metrics (side effects)
- Use `Mutate` for conditional modifications
- Use `Enrich` for optional enhancements that shouldn't fail the pipeline

### 5. Error Handling Strategy

- Return errors from processors to stop the pipeline
- Use `Fallback` for operations with alternatives
- Use `Retry` or `RetryWithBackoff` for transient failures
- Use `Concurrent` for independent I/O operations that can run in parallel
- Use `WithErrorHandler` for side-effect error processing (metrics, fallbacks)

## Performance

pipz is designed for production use with minimal overhead:

- **Zero allocations** in happy path for most processors
- **No reflection** at runtime (except for `Concurrent` deep copying)
- **No global state** or synchronization
- **Negligible overhead** - typically 10-50ns per processor

Example benchmarks:

```
BenchmarkValidation_Pipeline-12       2,915,101    432.1 ns/op    0 B/op    0 allocs/op
BenchmarkETL_Pipeline-12                763,939    1,564 ns/op    112 B/op    2 allocs/op
BenchmarkPayment_Retry-12             1,000,000    1,053 ns/op    0 B/op    0 allocs/op

# Concurrent processing benchmarks
BenchmarkConcurrent_Sequential-12           355    3,320,651 ns/op    0 B/op    0 allocs/op
BenchmarkConcurrent_Concurrent-12         1,029    1,148,231 ns/op    1,576 B/op    29 allocs/op
BenchmarkConcurrent_SmallStruct-12       92,572       11,101 ns/op    1,264 B/op    26 allocs/op
```

**Note:** `Concurrent` provides 3x speedup for I/O-bound operations despite copy overhead.

## Development

The Makefile provides all common development tasks:

```bash
# Run tests
make test              # Test core library
make test-examples     # Test all examples

# Run benchmarks
make bench             # Core benchmarks only
make bench-all         # All benchmarks including examples

# Development workflow
make check             # Run tests and lint
make lint              # Run linters
make lint-fix          # Auto-fix linting issues
make coverage          # Generate coverage report

# Build and install
make build             # Build CLI to ./pipz
make install           # Install CLI to $GOPATH/bin

# See all available commands
make help
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT - See [LICENSE](LICENSE) for details.
