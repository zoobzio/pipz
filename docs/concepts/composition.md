# Composition Patterns

This guide covers the core pattern for building composable systems with pipz.

## The Decompose-to-Compose Pattern

The key to building maintainable pipz systems is starting with **decomposed building blocks** that can be composed and recomposed as requirements change.

### Start with Atomic Business Logic

Break complex business processes into single-responsibility processors:

```go
// ❌ Wrong: Monolithic processor mixing concerns
orderProcessor := pipz.Apply("process_order", func(ctx context.Context, order Order) (Order, error) {
    // Validation logic
    if order.CustomerID == "" {
        return order, errors.New("missing customer")
    }
    
    // Pricing logic
    order.Total = calculatePrice(order)
    
    // Payment processing
    if err := chargeCard(order.PaymentToken, order.Total); err != nil {
        return order, err
    }
    
    // Notification logic
    sendConfirmationEmail(order.CustomerEmail)
    
    return order, nil
})

// ✅ Right: Atomic processors with single responsibilities
const (
    ProcessorValidate = "validate"
    ProcessorPrice    = "calculate_price"
    ProcessorPayment  = "process_payment"
    ProcessorNotify   = "send_notification"
)

var (
    ValidateOrder    = pipz.Apply(ProcessorValidate, validateOrderData)
    CalculatePrice   = pipz.Apply(ProcessorPrice, computePricing)
    ProcessPayment   = pipz.Apply(ProcessorPayment, chargeCustomer)
    SendNotification = pipz.Effect(ProcessorNotify, notifyCustomer)
)
```

### Compose Different Business Flows

With atomic building blocks, you can create multiple pipeline variants:

```go
// Different compositions for different business needs
var (
    StandardOrders = pipz.NewSequence[Order]("standard-orders")
    ExpressOrders  = pipz.NewSequence[Order]("express-orders")  
    TestOrders     = pipz.NewSequence[Order]("test-orders")
)

func init() {
    // Standard flow
    StandardOrders.Register(
        ValidateOrder,
        CalculatePrice,
        ProcessPayment,
        SendNotification,
    )
    
    // Express flow with different pricing
    ExpressOrders.Register(
        ValidateOrder,
        ExpressPrice,        // Different pricing logic
        PriorityPayment,     // Different payment processing
        SendNotification,
    )
    
    // Test flow with mocked payment
    TestOrders.Register(
        ValidateOrder,
        CalculatePrice,
        MockPayment,         // Mock for testing
        SendNotification,
    )
}
```

### Enable Runtime Recomposition

The same building blocks can be dynamically recombined:

```go
// Add fraud detection to existing pipeline
fraudDetection := pipz.Apply("fraud_check", detectFraud)
StandardOrders.After(ProcessorValidate, fraudDetection)

// Replace payment processor for maintenance
maintenancePayment := pipz.Apply("maintenance_payment", fallbackPayment)
StandardOrders.Replace(ProcessorPayment, maintenancePayment)

// Remove notifications for batch processing
BatchOrders := pipz.NewSequence[Order]("batch-orders")
BatchOrders.Register(
    ValidateOrder,
    CalculatePrice,
    ProcessPayment,
    // No notifications for batch
)
```

## Const Names + Vars Pattern

This pattern uses constants and variables together to enable composition:

### Constants: Names as System Keys

```go
const (
    ProcessorValidate = "validate"
    ProcessorPrice    = "calculate_price"
    ProcessorPayment  = "process_payment"
    
    PipelineStandard = "standard-orders"
    PipelineExpress  = "express-orders"
)
```

Constants serve as:
- **Identifiers** for dynamic pipeline modification
- **Keys** for finding and replacing processors
- **Documentation** of your system's building blocks
- **Contracts** that remain stable as implementation changes

### Variables: Living System Objects

```go
var (
    // Reusable processors
    ValidateOrder  = pipz.Apply(ProcessorValidate, validateOrderData)
    CalculatePrice = pipz.Apply(ProcessorPrice, computePricing)
    ProcessPayment = pipz.Apply(ProcessorPayment, chargeCustomer)
    
    // Mutable pipeline configurations
    StandardOrders = pipz.NewSequence[Order](PipelineStandard)
    ExpressOrders  = pipz.NewSequence[Order](PipelineExpress)
)
```

Variables hold:
- **Processor instances** that can be reused across pipelines
- **Pipeline configurations** that can be modified at runtime
- **System state** that evolves as your application runs

## Why This Pattern Works

### 1. **Reusability**
Atomic processors can be used in multiple contexts:
```go
// Same validation used in different flows
StandardOrders.Register(ValidateOrder, ...)
ExpressOrders.Register(ValidateOrder, ...)
RefundPipeline.Register(ValidateOrder, ...)
```

### 2. **Testability** 
Individual processors are easy to test in isolation:
```go
func TestValidateOrder(t *testing.T) {
    order := Order{CustomerID: ""}
    _, err := ValidateOrder.Process(context.Background(), order)
    assert.Error(t, err, "should reject empty customer ID")
}
```

### 3. **Flexibility**
Business requirements change, but building blocks remain stable:
```go
// Add new requirement: check inventory before pricing
InventoryCheck := pipz.Apply("inventory_check", checkStock)
StandardOrders.After(ProcessorValidate, InventoryCheck)

// Temporary A/B test: try different pricing
if feature.IsEnabled("new_pricing") {
    StandardOrders.Replace(ProcessorPrice, NewPricingAlgorithm)
}
```

### 4. **Composability**
Complex business logic emerges from simple building blocks:
```go
// Subscription renewal pipeline reuses existing pieces
SubscriptionRenewal := pipz.NewSequence[Subscription]("renewal")
SubscriptionRenewal.Register(
    ValidateSubscription,  // New processor
    CalculateRenewalPrice, // New processor  
    ProcessPayment,        // Reused processor
    SendRenewalEmail,      // New processor
)
```

## Best Practices

### Start Small, Compose Larger
Begin with the smallest useful processors:
```go
// Not: "ProcessOrderWithPaymentAndNotification" 
// But: "ValidateOrder", "ProcessPayment", "SendNotification"
```

### Name by Business Intent
Use names that reflect business value:
```go
const (
    ProcessorEnrichProfile = "enrich_customer_profile"  // Clear business purpose
    ProcessorCalcTax      = "calculate_sales_tax"       // Domain-specific
    ProcessorAuditTrail   = "record_audit_trail"        // Compliance requirement
)
```

### Design for Change
Assume requirements will evolve:
```go
// Design processors that can be easily replaced
var PricingEngine = pipz.Apply(ProcessorPrice, standardPricing)

// Later: switch to dynamic pricing
PricingEngine = pipz.Apply(ProcessorPrice, dynamicPricing)
```

This pattern enables you to build systems that grow and adapt with your business needs while maintaining clean, testable code.