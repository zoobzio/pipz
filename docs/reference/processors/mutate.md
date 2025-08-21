# Mutate

Creates a processor that conditionally modifies data based on a predicate.

> **Note**: Mutate is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Mutate[T any](
    name Name,
    condition func(context.Context, T) bool,
    mutation func(context.Context, T) T,
) Chainable[T]
```

## Parameters

- `name` (`Name`) - Identifier for the processor used in error messages and debugging
- `condition` - Predicate function that determines if mutation should occur
- `mutation` - Function that performs the transformation when condition is true

## Returns

Returns a `Chainable[T]` that applies the mutation only when the condition is met.

## Behavior

- **Conditional execution** - Mutation only runs if condition returns true
- **Pass-through on false** - Original data returned when condition is false
- **Cannot fail** - Neither condition nor mutation can return errors
- **Context aware** - Both functions receive context

## Example

```go
// Auto-verify trusted domains
autoVerify := pipz.Mutate("auto-verify",
    func(ctx context.Context, user User) bool {
        return strings.HasSuffix(user.Email, "@company.com")
    },
    func(ctx context.Context, user User) User {
        user.Verified = true
        user.VerifiedAt = time.Now()
        return user
    },
)

// Apply discounts
applyDiscount := pipz.Mutate("vip-discount",
    func(ctx context.Context, order Order) bool {
        return order.Customer.Tier == "VIP" && order.Total > 100
    },
    func(ctx context.Context, order Order) Order {
        order.Discount = order.Total * 0.2
        order.Total = order.Total - order.Discount
        return order
    },
)

// Feature flags
betaFeature := pipz.Mutate("beta-enrichment",
    func(ctx context.Context, data Data) bool {
        return featureFlags.IsEnabled(ctx, "beta-enrichment")
    },
    func(ctx context.Context, data Data) Data {
        data.BetaScore = calculateBetaScore(data)
        return data
    },
)

// Conditional formatting
formatPhone := pipz.Mutate("format-phone",
    func(ctx context.Context, contact Contact) bool {
        return contact.Country == "US" && len(contact.Phone) == 10
    },
    func(ctx context.Context, contact Contact) Contact {
        // Format as (XXX) XXX-XXXX
        contact.Phone = fmt.Sprintf("(%s) %s-%s",
            contact.Phone[0:3],
            contact.Phone[3:6],
            contact.Phone[6:10],
        )
        return contact
    },
)
```

## When to Use

Use `Mutate` when:
- You need **conditional transformations that can't fail**
- Different data needs different processing based on simple conditions
- You're implementing business rules with pure functions
- You want feature flags or A/B testing for transformations
- You need data normalization based on conditions
- Applying defaults or enrichments conditionally

## When NOT to Use

Don't use `Mutate` when:
- The operation can fail (use `Apply` with conditions)
- You always transform (use `Transform` - no condition needed)
- You need complex routing (use `Switch` for multiple branches)
- The condition needs error handling (use `Filter` with `Apply`)
- You need side effects (use `Filter` with `Effect`)

## Performance

Mutate has minimal overhead:
- Condition check is fast
- No allocations if condition is false
- Similar to Transform when condition is true

## Common Patterns

```go
// Chain multiple conditional mutations
pipeline := pipz.NewSequence[User]("user-processing",
    pipz.Mutate("verify-trusted", isTrustedDomain, markVerified),
    pipz.Mutate("apply-regional", isEuropean, applyGDPR),
    pipz.Mutate("premium-features", isPremium, addPremiumFeatures),
)

// Combine with validation
processOrder := pipz.NewSequence[Order]("order",
    pipz.Apply("validate", validateOrder),
    pipz.Mutate("loyalty-discount", isLoyaltyMember, applyLoyaltyDiscount),
    pipz.Mutate("bulk-discount", isBulkOrder, applyBulkDiscount),
    pipz.Apply("calculate-tax", calculateTax),
)

// Environment-based behavior
debugEnrichment := pipz.Mutate("debug-data",
    func(ctx context.Context, data Data) bool {
        return os.Getenv("ENV") == "development"
    },
    func(ctx context.Context, data Data) Data {
        data.DebugInfo = generateDebugInfo(data)
        return data
    },
)

// Default values
applyDefaults := pipz.Mutate("defaults",
    func(ctx context.Context, cfg Config) bool {
        return cfg.Timeout == 0 // No timeout set
    },
    func(ctx context.Context, cfg Config) Config {
        cfg.Timeout = 30 * time.Second
        return cfg
    },
)
```

## Gotchas

### ❌ Don't use for operations that can fail
```go
// WRONG - Parse can fail but Mutate can't handle errors
mutate := pipz.Mutate("parse",
    func(ctx context.Context, s string) bool { return s != "" },
    func(ctx context.Context, s string) Data {
        data, _ := json.Unmarshal([]byte(s), &Data{}) // Error ignored!
        return data
    },
)
```

### ✅ Use Apply for fallible operations
```go
// RIGHT - Proper error handling
apply := pipz.Apply("parse", func(ctx context.Context, s string) (Data, error) {
    if s == "" {
        return Data{}, nil // Skip parsing
    }
    var data Data
    err := json.Unmarshal([]byte(s), &data)
    return data, err
})
```

### ❌ Don't use when you always transform
```go
// WRONG - Condition always true
mutate := pipz.Mutate("always",
    func(ctx context.Context, data Data) bool { return true }, // Always!
    transform,
)
```

### ✅ Use Transform directly
```go
// RIGHT - No condition needed
transform := pipz.Transform("always", transform)
```

## Advanced Usage

```go
// Complex conditions
smartRouting := pipz.Mutate("smart-route",
    func(ctx context.Context, req Request) bool {
        // Multiple conditions
        return req.Priority == High &&
               time.Now().Hour() >= 9 &&
               time.Now().Hour() <= 17 &&
               !isHoliday(time.Now())
    },
    func(ctx context.Context, req Request) Request {
        req.Route = "express"
        req.SLA = time.Hour
        return req
    },
)

// Stateful conditions (be careful with concurrency)
rateLimiter := pipz.Mutate("rate-limit",
    func(ctx context.Context, req Request) bool {
        return limiter.Allow(req.UserID)
    },
    func(ctx context.Context, req Request) Request {
        req.RateLimited = false
        return req
    },
)
```

## See Also

- [Transform](./transform.md) - For unconditional transformations
- [Switch](./switch.md) - For routing to different processors
- [Apply](./apply.md) - For conditional operations that can fail