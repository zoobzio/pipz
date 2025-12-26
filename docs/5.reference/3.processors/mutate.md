---
title: "Mutate"
description: "Creates a processor that conditionally modifies data based on a predicate function"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - processors
  - conditional
  - transformation
---

# Mutate

Creates a processor that conditionally modifies data based on a predicate.

> **Note**: Mutate is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Mutate[T any](
    identity Identity,
    transformer func(context.Context, T) T,
    condition func(context.Context, T) bool,
) Processor[T]
```

## Parameters

- `identity` (`Identity`) - Identifier for the processor used in error messages and debugging
- `transformer` - Function that performs the transformation when condition is true
- `condition` - Predicate function that determines if transformation should occur

## Returns

Returns a `Processor[T]` that applies the transformation only when the condition is met.

## Behavior

- **Conditional execution** - Mutation only runs if condition returns true
- **Pass-through on false** - Original data returned when condition is false
- **Cannot fail** - Neither condition nor mutation can return errors
- **Context aware** - Both functions receive context

## Example

```go
// Auto-verify trusted domains
autoVerify := pipz.Mutate(
    pipz.NewIdentity("auto-verify", "Auto-verifies company email addresses"),
    func(ctx context.Context, user User) User {
        user.Verified = true
        user.VerifiedAt = time.Now()
        return user
    },
    func(ctx context.Context, user User) bool {
        return strings.HasSuffix(user.Email, "@company.com")
    },
)

// Apply discounts
applyDiscount := pipz.Mutate(
    pipz.NewIdentity("vip-discount", "Applies VIP discount to qualifying orders"),
    func(ctx context.Context, order Order) Order {
        order.Discount = order.Total * 0.2
        order.Total = order.Total - order.Discount
        return order
    },
    func(ctx context.Context, order Order) bool {
        return order.Customer.Tier == "VIP" && order.Total > 100
    },
)

// Feature flags
betaFeature := pipz.Mutate(
    pipz.NewIdentity("beta-enrichment", "Adds beta score when feature enabled"),
    func(ctx context.Context, data Data) Data {
        data.BetaScore = calculateBetaScore(data)
        return data
    },
    func(ctx context.Context, data Data) bool {
        return featureFlags.IsEnabled(ctx, "beta-enrichment")
    },
)

// Conditional formatting
formatPhone := pipz.Mutate(
    pipz.NewIdentity("format-phone", "Formats US phone numbers"),
    func(ctx context.Context, contact Contact) Contact {
        // Format as (XXX) XXX-XXXX
        contact.Phone = fmt.Sprintf("(%s) %s-%s",
            contact.Phone[0:3],
            contact.Phone[3:6],
            contact.Phone[6:10],
        )
        return contact
    },
    func(ctx context.Context, contact Contact) bool {
        return contact.Country == "US" && len(contact.Phone) == 10
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
// Define identities upfront
var (
    UserProcessingID = pipz.NewIdentity("user-processing", "User processing pipeline")
    VerifyTrustedID  = pipz.NewIdentity("verify-trusted", "Verifies trusted domains")
    ApplyRegionalID  = pipz.NewIdentity("apply-regional", "Applies regional settings")
    PremiumFeaturesID = pipz.NewIdentity("premium-features", "Adds premium features")
    OrderID          = pipz.NewIdentity("order", "Order processing pipeline")
    ValidateID       = pipz.NewIdentity("validate", "Validates order")
    LoyaltyDiscountID = pipz.NewIdentity("loyalty-discount", "Applies loyalty discount")
    BulkDiscountID   = pipz.NewIdentity("bulk-discount", "Applies bulk discount")
    CalculateTaxID   = pipz.NewIdentity("calculate-tax", "Calculates tax")
)

// Chain multiple conditional mutations
pipeline := pipz.NewSequence[User](UserProcessingID,
    pipz.Mutate(VerifyTrustedID, markVerified, isTrustedDomain),
    pipz.Mutate(ApplyRegionalID, applyGDPR, isEuropean),
    pipz.Mutate(PremiumFeaturesID, addPremiumFeatures, isPremium),
)

// Combine with validation
processOrder := pipz.NewSequence[Order](OrderID,
    pipz.Apply(ValidateID, validateOrder),
    pipz.Mutate(LoyaltyDiscountID, applyLoyaltyDiscount, isLoyaltyMember),
    pipz.Mutate(BulkDiscountID, applyBulkDiscount, isBulkOrder),
    pipz.Apply(CalculateTaxID, calculateTax),
)

// Environment-based behavior
debugEnrichment := pipz.Mutate(
    pipz.NewIdentity("debug-data", "Adds debug information in development"),
    func(ctx context.Context, data Data) Data {
        data.DebugInfo = generateDebugInfo(data)
        return data
    },
    func(ctx context.Context, data Data) bool {
        return os.Getenv("ENV") == "development"
    },
)

// Default values
applyDefaults := pipz.Mutate(
    pipz.NewIdentity("defaults", "Applies default timeout"),
    func(ctx context.Context, cfg Config) Config {
        cfg.Timeout = 30 * time.Second
        return cfg
    },
    func(ctx context.Context, cfg Config) bool {
        return cfg.Timeout == 0 // No timeout set
    },
)
```

## Gotchas

### ❌ Don't use for operations that can fail
```go
// WRONG - Parse can fail but Mutate can't handle errors
mutate := pipz.Mutate(
    pipz.NewIdentity("parse", "Parses JSON"),
    func(ctx context.Context, s string) Data {
        data, _ := json.Unmarshal([]byte(s), &Data{}) // Error ignored!
        return data
    },
    func(ctx context.Context, s string) bool { return s != "" },
)
```

### ✅ Use Apply for fallible operations
```go
// RIGHT - Proper error handling
apply := pipz.Apply(
    pipz.NewIdentity("parse", "Parses JSON with error handling"),
    func(ctx context.Context, s string) (Data, error) {
        if s == "" {
            return Data{}, nil // Skip parsing
        }
        var data Data
        err := json.Unmarshal([]byte(s), &data)
        return data, err
    },
)
```

### ❌ Don't use when you always transform
```go
// WRONG - Condition always true
mutate := pipz.Mutate(
    pipz.NewIdentity("always", "Always transforms"),
    transform,
    func(ctx context.Context, data Data) bool { return true }, // Always!
)
```

### ✅ Use Transform directly
```go
// RIGHT - No condition needed
transform := pipz.Transform(
    pipz.NewIdentity("always", "Always transforms"),
    transform,
)
```

## Advanced Usage

```go
// Complex conditions
smartRouting := pipz.Mutate(
    pipz.NewIdentity("smart-route", "Routes high priority requests to express during business hours"),
    func(ctx context.Context, req Request) Request {
        req.Route = "express"
        req.SLA = time.Hour
        return req
    },
    func(ctx context.Context, req Request) bool {
        // Multiple conditions
        return req.Priority == High &&
               time.Now().Hour() >= 9 &&
               time.Now().Hour() <= 17 &&
               !isHoliday(time.Now())
    },
)

// Stateful conditions (be careful with concurrency)
rateLimiter := pipz.Mutate(
    pipz.NewIdentity("rate-limit", "Applies rate limiting based on user quota"),
    func(ctx context.Context, req Request) Request {
        req.RateLimited = false
        return req
    },
    func(ctx context.Context, req Request) bool {
        return limiter.Allow(req.UserID)
    },
)
```

## See Also

- [Transform](./transform.md) - For unconditional transformations
- [Switch](./switch.md) - For routing to different processors
- [Apply](./apply.md) - For conditional operations that can fail