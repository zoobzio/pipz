# Switch

Routes data to different processors based on a condition function.

## Function Signature

```go
func NewSwitch[T any, K comparable](
    name Name,
    condition func(context.Context, T) K,
) *Switch[T, K]
```

## Type Parameters

- `T` - The data type being processed
- `K` - The route key type (must be comparable: string, int, etc.)

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `condition` - Function that examines data and returns a route key

## Returns

Returns a `*Switch[T, K]` that implements `Chainable[T]`.

## Methods

```go
// Add a route
AddRoute(key K, processor Chainable[T]) *Switch[T, K]

// Set default route (optional)
Default(processor Chainable[T]) *Switch[T, K]
```

## Behavior

- **Dynamic routing** - Routes determined at runtime based on data
- **Type-safe keys** - Route keys are strongly typed
- **Chainable API** - Routes can be added fluently
- **Default route** - Optional fallback for unmatched keys
- **Error on no match** - Fails if no route matches and no default set

## Example

```go
// Route by user type
userRouter := pipz.NewSwitch("user-router",
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

// Route by enum
type Priority int
const (
    Low Priority = iota
    Medium
    High
    Critical
)

priorityRouter := pipz.NewSwitch("priority-router",
    func(ctx context.Context, task Task) Priority {
        return task.Priority
    },
).
AddRoute(Critical, processCritical).
AddRoute(High, processHigh).
AddRoute(Medium, processMedium).
AddRoute(Low, processLow)

// Route with default
paymentRouter := pipz.NewSwitch("payment-router",
    func(ctx context.Context, payment Payment) string {
        return payment.Method
    },
).
AddRoute("credit_card", processCreditCard).
AddRoute("paypal", processPayPal).
AddRoute("crypto", processCrypto).
Default(processUnknownPayment) // Handles any other payment method
```

## When to Use

Use `Switch` when:
- You need conditional processing based on data
- Different types require different handling
- Implementing strategy pattern
- Building rule engines
- Creating workflow routers

## When NOT to Use

Don't use `Switch` when:
- Only two options exist (use `Fallback` or `Mutate`)
- All processors should run (use `Concurrent`)
- Conditions are complex/nested (consider multiple Switches)
- Simple boolean conditions (use `Mutate`)

## Error Handling

Switch fails if no route matches:

```go
router := pipz.NewSwitch("router",
    func(ctx context.Context, data Data) string {
        return data.Type
    },
).
AddRoute("typeA", processA).
AddRoute("typeB", processB)
// No default set!

data := Data{Type: "typeC"}
_, err := router.Process(ctx, data)
// err: "no route found for key: typeC"
```

## Common Patterns

```go
// Multi-level routing
mainRouter := pipz.NewSwitch("main-router",
    func(ctx context.Context, req Request) string {
        return req.Service
    },
).
AddRoute("auth", authPipeline).
AddRoute("payment", 
    pipz.NewSwitch("payment-sub-router",
        func(ctx context.Context, req Request) string {
            return req.PaymentType
        },
    ).
    AddRoute("card", cardProcessor).
    AddRoute("bank", bankProcessor),
).
AddRoute("shipping", shippingPipeline)

// Error-based routing
errorRouter := pipz.NewSwitch("error-router",
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
featureRouter := pipz.NewSwitch("feature-router",
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
router := pipz.NewSwitch[Order, string]("dynamic", getOrderType)

// Register routes from configuration
for _, route := range config.Routes {
    processor := createProcessor(route.Handler)
    router.AddRoute(route.Key, processor)
}

// Computed routing keys
complexRouter := pipz.NewSwitch("complex",
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
abRouter := pipz.NewSwitch("ab-test",
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

## Best Practices

```go
// Use meaningful route keys
// GOOD: Clear, self-documenting keys
goodRouter := pipz.NewSwitch("clear",
    func(ctx context.Context, order Order) string {
        if order.Total > 1000 {
            return "high-value"
        }
        if order.IsFirstOrder {
            return "first-time"
        }
        return "standard"
    },
)

// BAD: Opaque keys
badRouter := pipz.NewSwitch("opaque",
    func(ctx context.Context, order Order) int {
        // What do these numbers mean?
        if order.Total > 1000 {
            return 1
        }
        return 2
    },
)

// Consider using enums for type safety
type RouteKey string
const (
    RouteVIP      RouteKey = "vip"
    RouteStandard RouteKey = "standard"
    RouteNew      RouteKey = "new"
)

typedRouter := pipz.NewSwitch("typed",
    func(ctx context.Context, user User) RouteKey {
        // Compiler-checked route keys
        if user.IsVIP {
            return RouteVIP
        }
        return RouteStandard
    },
)
```

## See Also

- [Mutate](./mutate.md) - For simple conditional processing
- [Fallback](./fallback.md) - For two-option routing
- [Handle](./handle.md) - Often uses Switch for error routing
- [Concurrent](./concurrent.md) - When all routes should execute