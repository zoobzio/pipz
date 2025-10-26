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

## Condition Type

The `Condition` type determines routing in Switch connectors:

```go
type Condition[T any, K comparable] func(context.Context, T) K
```

### Type Parameters
- `T` - The input data type to be examined
- `K` - The route key type that will be returned (must be comparable)

### Function Signature
- **Input**: Takes a context and data of type T
- **Output**: Returns a route key of type K
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

// Enum-based routing
func priorityCondition(ctx context.Context, task Task) Priority {
    return task.Priority // Priority is an int-based enum
}

// Computed routing key
func loadBalanceCondition(ctx context.Context, req Request) int {
    // Route based on request ID hash for load distribution
    return int(req.ID % 3) // Routes to 0, 1, or 2
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

## Gotchas

### ❌ Don't forget to handle all cases
```go
// WRONG - Missing routes will cause runtime errors
switch := pipz.NewSwitch("incomplete",
    func(ctx context.Context, data Data) string {
        return data.Type // Could be "A", "B", "C", or "D"
    },
).
AddRoute("A", processA).
AddRoute("B", processB)
// Missing C and D - will fail at runtime!
```

### ✅ Add all routes or use Default
```go
// RIGHT - Handle all cases
switch := pipz.NewSwitch("complete",
    func(ctx context.Context, data Data) string {
        return data.Type
    },
).
AddRoute("A", processA).
AddRoute("B", processB).
AddRoute("C", processC).
AddRoute("D", processD).
Default(processUnknown) // Safety net
```

### ❌ Don't use Switch for simple boolean logic
```go
// WRONG - Overkill for boolean
switch := pipz.NewSwitch("overkill",
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
filter := pipz.NewFilter("simple",
    func(ctx context.Context, user User) bool {
        return user.IsActive
    },
    processActive,
)
```

### ❌ Don't use opaque route keys
```go
// WRONG - What do these numbers mean?
switch := pipz.NewSwitch("opaque",
    func(ctx context.Context, order Order) int {
        if order.Total > 1000 {
            return 1 // Magic number!
        }
        return 2 // Another magic number!
    },
)
```

### ✅ Use meaningful, self-documenting keys
```go
// RIGHT - Clear intent
switch := pipz.NewSwitch("clear",
    func(ctx context.Context, order Order) string {
        if order.Total > 1000 {
            return "high-value"
        }
        return "standard"
    },
)
```

## Best Practices

1. **Use enums or constants** for route keys to avoid typos
2. **Always consider a Default** route for safety
3. **Keep routing logic simple** - complex conditions make debugging hard
4. **Document route keys** if they're not self-evident
5. **Test all routes** including the default case

## See Also

- [Mutate](../processors/mutate.md) - For simple conditional processing
- [Fallback](./fallback.md) - For two-option routing
- [Handle](../processors/handle.md) - Often uses Switch for error routing
- [Concurrent](./concurrent.md) - When all routes should execute