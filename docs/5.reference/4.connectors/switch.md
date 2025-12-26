---
title: "Switch"
description: "Routes data to different processors based on a condition function for dynamic workflow routing"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - connectors
  - routing
  - conditional
  - strategy-pattern
---

# Switch

Routes data to different processors based on a condition function.

## Function Signature

```go
func NewSwitch[T any](
    identity Identity,
    condition func(context.Context, T) string,
) *Switch[T]
```

## Type Parameters

- `T` - The data type being processed

## Parameters

- `identity` (`Identity`) - Identifier with name and description for the connector used in debugging and observability
- `condition` - Function that examines data and returns a route key string

## Condition Type

The `Condition` type determines routing in Switch connectors:

```go
type Condition[T any] func(context.Context, T) string
```

### Type Parameters
- `T` - The input data type to be examined

### Function Signature
- **Input**: Takes a context and data of type T
- **Output**: Returns a route key string
- **Purpose**: Examines the input data and determines which route to take

### How It Works

The condition function is called for each piece of data processed:
1. Receives the current context and input data
2. Examines the data to determine the appropriate route
3. Returns a key that maps to a specific processor
4. The Switch connector uses this key to route the data

### Example Conditions

```go
// Simple string-based routing
func userTypeCondition(ctx context.Context, user User) string {
    if user.IsVIP {
        return "vip"
    }
    return "regular"
}

// Priority routing with string keys
func priorityCondition(ctx context.Context, task Task) string {
    switch task.Priority {
    case 3:
        return "critical"
    case 2:
        return "high"
    case 1:
        return "medium"
    default:
        return "low"
    }
}

// Computed routing key
func loadBalanceCondition(ctx context.Context, req Request) string {
    // Route based on request ID hash for load distribution
    bucket := req.ID % 3
    return fmt.Sprintf("bucket-%d", bucket) // Routes to "bucket-0", "bucket-1", "bucket-2"
}

// Context-aware routing
func featureCondition(ctx context.Context, data Data) string {
    // Use context values for routing decisions
    if feature, ok := ctx.Value("feature").(string); ok && feature == "beta" {
        return "experimental"
    }
    return "stable"
}
```

### Best Practices for Conditions

1. **Keep conditions pure** - Avoid side effects in condition functions
2. **Make them fast** - Conditions are called for every data item
3. **Use meaningful keys** - Return descriptive strings or enums
4. **Handle all cases** - Ensure all possible return values have routes
5. **Leverage context** - Use context for feature flags or configuration

## Returns

Returns a `*Switch[T]` that implements `Chainable[T]`.

## Methods

```go
// Add a route
AddRoute(key string, processor Chainable[T]) *Switch[T]

// Remove a route
RemoveRoute(key string) *Switch[T]

// Check if route exists
HasRoute(key string) bool

// Get all routes (copy)
Routes() map[string]Chainable[T]

// Clear all routes
ClearRoutes() *Switch[T]

// Replace all routes atomically
SetRoutes(routes map[string]Chainable[T]) *Switch[T]

// Update condition function
SetCondition(condition Condition[T]) *Switch[T]
```

## Behavior

- **Dynamic routing** - Routes determined at runtime based on data
- **String keys** - Route keys are strings for simplicity and serialization
- **Chainable API** - Routes can be added fluently
- **Pass-through on no match** - Returns input unchanged if no route matches
- **Thread-safe** - Routes can be modified during operation

## Example

```go
// Route by user type
userRouter := pipz.NewSwitch(
    pipz.NewIdentity("user-router", "Routes users to appropriate handlers based on VIP/new/regular status"),
    func(ctx context.Context, user User) string {
        if user.IsVIP {
            return "vip"
        }
        if user.IsNew {
            return "new"
        }
        return "regular"
    },
).
AddRoute("vip", processVIPUser).
AddRoute("new", processNewUser).
AddRoute("regular", processRegularUser)

// Route by priority level
priorityRouter := pipz.NewSwitch(
    pipz.NewIdentity("priority-router", "Routes tasks by priority level for appropriate processing"),
    func(ctx context.Context, task Task) string {
        switch task.Priority {
        case 3:
            return "critical"
        case 2:
            return "high"
        case 1:
            return "medium"
        default:
            return "low"
        }
    },
).
AddRoute("critical", processCritical).
AddRoute("high", processHigh).
AddRoute("medium", processMedium).
AddRoute("low", processLow)

// Route by payment method
paymentRouter := pipz.NewSwitch(
    pipz.NewIdentity("payment-router", "Routes payment processing based on payment method type"),
    func(ctx context.Context, payment Payment) string {
        return payment.Method
    },
).
AddRoute("credit_card", processCreditCard).
AddRoute("paypal", processPayPal).
AddRoute("crypto", processCrypto)
// Unmatched methods pass through unchanged
```

## When to Use

Use `Switch` when:
- You need **conditional routing with 3+ branches**
- Different types require different handling
- Implementing strategy pattern
- Building rule engines
- Creating workflow routers
- A/B testing with multiple variants
- Processing varies by category/type/status

## When NOT to Use

Don't use `Switch` when:
- Only two options exist (use `Fallback` or `Filter`)
- All processors should run (use `Concurrent`)
- Conditions are complex/nested (consider multiple Switches)
- Simple boolean conditions (use `Filter` or `Mutate`)
- You just need if/else logic (use `Filter`)

## Pass-Through Behavior

Switch passes through unchanged if no route matches:

```go
router := pipz.NewSwitch(
    pipz.NewIdentity("router", "Routes data by type to appropriate processors"),
    func(ctx context.Context, data Data) string {
        return data.Type
    },
).
AddRoute("typeA", processA).
AddRoute("typeB", processB)

data := Data{Type: "typeC"}
result, err := router.Process(ctx, data)
// err: nil
// result: data (unchanged, passed through)
```

This design allows Switch to be safely added to pipelines without requiring exhaustive route coverage.

## Common Patterns

```go
// Multi-level routing
mainRouter := pipz.NewSwitch(
    pipz.NewIdentity("main-router", "Primary service router for auth, payment, and shipping requests"),
    func(ctx context.Context, req Request) string {
        return req.Service
    },
).
AddRoute("auth", authPipeline).
AddRoute("payment",
    pipz.NewSwitch(
        pipz.NewIdentity("payment-sub-router", "Sub-router for different payment method types"),
        func(ctx context.Context, req Request) string {
            return req.PaymentType
        },
    ).
    AddRoute("card", cardProcessor).
    AddRoute("bank", bankProcessor),
).
AddRoute("shipping", shippingPipeline)

// Error-based routing
errorRouter := pipz.NewSwitch(
    pipz.NewIdentity("error-router", "Routes errors to appropriate handlers based on error type"),
    func(ctx context.Context, err *pipz.Error[Data]) string {
        switch {
        case err.Timeout:
            return "timeout"
        case err.Canceled:
            return "canceled"
        case strings.Contains(err.Err.Error(), "validation"):
            return "validation"
        default:
            return "other"
        }
    },
).
AddRoute("timeout", handleTimeout).
AddRoute("canceled", handleCancellation).
AddRoute("validation", handleValidation).
AddRoute("other", handleGenericError)

// Feature flag routing
featureRouter := pipz.NewSwitch(
    pipz.NewIdentity("feature-router", "Routes between new and legacy algorithms based on feature flags"),
    func(ctx context.Context, data Data) string {
        if featureFlags.IsEnabled(ctx, "new_algorithm") {
            return "new"
        }
        return "old"
    },
).
AddRoute("new", newAlgorithm).
AddRoute("old", oldAlgorithm)
```

## Advanced Patterns

```go
// Dynamic route registration
router := pipz.NewSwitch[Order](
    pipz.NewIdentity("dynamic", "Dynamically configured router for order processing"),
    getOrderType,
)

// Register routes from configuration
for _, route := range config.Routes {
    processor := createProcessor(route.Handler)
    router.AddRoute(route.Key, processor)
}

// Computed routing keys
complexRouter := pipz.NewSwitch(
    pipz.NewIdentity("complex", "Routes events by region and calculated score for distributed processing"),
    func(ctx context.Context, event Event) string {
        // Complex routing logic
        score := calculateScore(event)
        region := detectRegion(event.IP)

        return fmt.Sprintf("%s:%d", region, score/10)
    },
).
AddRoute("us:0", lowPriorityUS).
AddRoute("us:1", mediumPriorityUS).
AddRoute("eu:0", lowPriorityEU).
AddRoute("eu:1", mediumPriorityEU).
Default(genericProcessor)

// Percentage-based routing (A/B testing)
abRouter := pipz.NewSwitch(
    pipz.NewIdentity("ab-test", "A/B test router directing 10% of users to experimental flow"),
    func(ctx context.Context, user User) string {
        hash := hashUserID(user.ID)
        if hash%100 < 10 { // 10% of users
            return "experiment"
        }
        return "control"
    },
).
AddRoute("experiment", experimentalFlow).
AddRoute("control", standardFlow)
```

## Gotchas

### ❌ Don't use Switch for simple boolean logic
```go
// WRONG - Overkill for boolean
switch := pipz.NewSwitch(
    pipz.NewIdentity("overkill", "Over-engineered router for simple boolean condition"),
    func(ctx context.Context, user User) string {
        if user.IsActive {
            return "active"
        }
        return "inactive"
    },
).
AddRoute("active", processActive).
AddRoute("inactive", processInactive)
```

### ✅ Use Filter for simple conditions
```go
// RIGHT - Simpler with Filter
filter := pipz.NewFilter(
    pipz.NewIdentity("simple", "Processes active users"),
    func(ctx context.Context, user User) bool {
        return user.IsActive
    },
    processActive,
)
```

### ❌ Don't use opaque route keys
```go
// WRONG - What do these mean?
switch := pipz.NewSwitch(
    pipz.NewIdentity("opaque", "Routes orders by value threshold with unclear keys"),
    func(ctx context.Context, order Order) string {
        if order.Total > 1000 {
            return "1" // Magic value!
        }
        return "2" // Another magic value!
    },
)
```

### ✅ Use meaningful, self-documenting keys
```go
// RIGHT - Clear intent
switch := pipz.NewSwitch(
    pipz.NewIdentity("clear", "Routes orders based on total value using descriptive keys"),
    func(ctx context.Context, order Order) string {
        if order.Total > 1000 {
            return "high-value"
        }
        return "standard"
    },
)
```

### ❌ Don't assume unmatched routes fail
```go
// WRONG - Expecting an error
router := pipz.NewSwitch(
    pipz.NewIdentity("router", "Routes by type"),
    func(ctx context.Context, data Data) string {
        return data.Type
    },
).AddRoute("known", processKnown)

_, err := router.Process(ctx, Data{Type: "unknown"})
// err is nil! Data passes through unchanged
```

### ✅ Add explicit handling if needed
```go
// RIGHT - Validate in condition or use a catch-all route
router := pipz.NewSwitch(
    pipz.NewIdentity("router", "Routes by type with unknown handling"),
    func(ctx context.Context, data Data) string {
        switch data.Type {
        case "typeA", "typeB":
            return data.Type
        default:
            return "unknown"
        }
    },
).
AddRoute("typeA", processA).
AddRoute("typeB", processB).
AddRoute("unknown", handleUnknown)
```

## Best Practices

1. **Use constants** for route keys to avoid typos
2. **Add a catch-all route** if you need to handle unknown cases explicitly
3. **Keep routing logic simple** - complex conditions make debugging hard
4. **Document route keys** if they're not self-evident
5. **Test all routes** including pass-through behavior for unmatched keys

## See Also

- [Mutate](../3.processors/mutate.md) - For simple conditional processing
- [Fallback](./fallback.md) - For two-option routing
- [Handle](../3.processors/handle.md) - Often uses Switch for error routing
- [Concurrent](./concurrent.md) - When all routes should execute