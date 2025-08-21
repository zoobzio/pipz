# Filter

Filter provides conditional processing that either executes a processor or passes data through unchanged based on a predicate function.

## Overview

Filter creates a branch in your pipeline where processing is optional based on runtime conditions. Unlike Switch which routes to different processors, Filter either processes or skips. Unlike Mutate which only supports safe transformations, Filter can execute any Chainable including ones that may error.

```go
filter := pipz.NewFilter(name, condition, processor)
```

## When to Use

Use `Filter` when:
- **Conditional processing needed** (feature flags, A/B testing)
- Skip expensive operations based on data state
- Apply business rules to subset of data
- Different users need different processing paths
- You want clean separation of condition and logic
- Performance optimization through selective processing

## When NOT to Use

Don't use `Filter` when:
- All data needs the same processing (just use the processor directly)
- You need multiple branches (use `Switch` instead)
- The condition is better expressed in the processor itself
- You're just transforming conditionally (use `Mutate` for simpler cases)

## Basic Usage

```go
// Feature flag example
betaFeature := pipz.NewFilter("beta-feature",
    func(ctx context.Context, user User) bool {
        return user.BetaEnabled && isFeatureEnabled(ctx, "new-algorithm")
    },
    newAlgorithmProcessor,
)

// Conditional validation
validatePremium := pipz.NewFilter("premium-validation",
    func(ctx context.Context, order Order) bool {
        return order.CustomerTier == "premium"
    },
    pipz.NewSequence("premium-checks",
        validateCreditLimit,
        checkFraudScore,
        verifyIdentity,
    ),
)
```

## Condition Function

The condition function determines whether processing should occur:

```go
func(context.Context, T) bool
```

- **Returns true**: Execute the processor
- **Returns false**: Pass data through unchanged
- **Context aware**: Can use context for timeouts, values, cancellation
- **Pure function**: Should not have side effects

### Condition Examples

```go
// Simple data check
func(ctx context.Context, order Order) bool {
    return order.Amount > 1000
}

// Feature flag with context
func(ctx context.Context, user User) bool {
    return user.BetaEnabled && 
           featureFlags.IsEnabled(ctx, "experimental-feature")
}

// Time-based condition
func(ctx context.Context, data Data) bool {
    return time.Now().Hour() >= 9 && time.Now().Hour() < 17 // Business hours
}

// Complex business logic
func(ctx context.Context, payment Payment) bool {
    return payment.Method == "crypto" && 
           payment.Amount > 10000 &&
           payment.Customer.RiskScore < 0.3
}
```

## Processor

Any Chainable can be used as the processor:

```go
// Simple processor
processor := pipz.Transform("double", func(ctx context.Context, n int) int {
    return n * 2
})

// Error-prone processor
validator := pipz.Apply("validate", func(ctx context.Context, data Data) (Data, error) {
    return validateData(data)
})

// Complex pipeline
complexFlow := pipz.NewSequence("complex",
    validate,
    enrich,
    transform,
)

filter := pipz.NewFilter("conditional", condition, complexFlow)
```

## Dynamic Behavior

Filter supports runtime updates for dynamic behavior:

```go
filter := pipz.NewFilter("dynamic", initialCondition, initialProcessor)

// Update condition at runtime
filter.SetCondition(func(ctx context.Context, data Data) bool {
    // New condition logic
    return data.Version >= 2
})

// Update processor at runtime
filter.SetProcessor(newProcessor)

// Access current values
currentCondition := filter.Condition()
currentProcessor := filter.Processor()
```

## Error Handling

When the processor returns an error, Filter prepends its name to the error path:

```go
filter := pipz.NewFilter("payment-filter", 
    func(ctx context.Context, p Payment) bool { return p.Amount > 100 },
    pipz.Apply("validate", failingValidator),
)

result, err := filter.Process(ctx, payment)
if err != nil {
    // err.Path will be ["payment-filter", "validate"]
    fmt.Printf("Failed at: %v\n", err.Path)
}
```

## Thread Safety

Filter is thread-safe and can be safely used in concurrent scenarios:

```go
filter := pipz.NewFilter("concurrent-safe", condition, processor)

// Safe to call from multiple goroutines
go func() { filter.Process(ctx, data1) }()
go func() { filter.Process(ctx, data2) }()

// Safe to update from other goroutines  
go func() { filter.SetCondition(newCondition) }()
```

## Performance Characteristics

Filter has minimal overhead:

- **Condition false**: ~5ns with zero allocations
- **Condition true**: Processor overhead + ~10ns
- **No reflection**: Direct function calls
- **Memory efficient**: No intermediate allocations

## Common Patterns

### Feature Flag Processing

```go
type FeatureFlags struct {
    flags map[string]bool
    mu    sync.RWMutex
}

func (f *FeatureFlags) IsEnabled(flag string) bool {
    f.mu.RLock()
    defer f.mu.RUnlock()
    return f.flags[flag]
}

// Create feature flag filter
featureFilter := pipz.NewFilter("feature-gate",
    func(ctx context.Context, user User) bool {
        return user.BetaEnabled && flags.IsEnabled("new-feature")
    },
    newFeatureProcessor,
)
```

### Conditional Enrichment

```go
// Only enrich premium customers
enrichPremium := pipz.NewFilter("enrich-premium",
    func(ctx context.Context, customer Customer) bool {
        return customer.Tier == "premium" || customer.Tier == "enterprise"
    },
    pipz.NewSequence("premium-enrichment",
        addPersonalizedOffers,
        calculateLoyaltyPoints,
        addPrioritySupport,
    ),
)
```

### Performance Optimization

```go
// Skip expensive processing for cached data
skipIfCached := pipz.NewFilter("cache-check",
    func(ctx context.Context, request Request) bool {
        _, exists := cache.Get(request.CacheKey())
        return !exists // Only process if not cached
    },
    expensiveProcessor,
)
```

### Time-Based Processing

```go
// Only process during business hours
businessHours := pipz.NewFilter("business-hours",
    func(ctx context.Context, task Task) bool {
        now := time.Now()
        hour := now.Hour()
        weekday := now.Weekday()
        
        return weekday >= time.Monday && 
               weekday <= time.Friday && 
               hour >= 9 && 
               hour < 17
    },
    businessProcessor,
)
```

## Filter vs Other Connectors

### Filter vs Switch

- **Filter**: Execute or skip (binary choice)
- **Switch**: Route to different processors (multiple choices)

```go
// Filter: Optional processing
filter := pipz.NewFilter("optional", condition, processor)

// Switch: Alternative processing
router := pipz.NewSwitch("router", routingFunction)
router.AddRoute("path-a", processorA)
router.AddRoute("path-b", processorB)
```

### Filter vs Mutate

- **Filter**: Can use any Chainable, including error-prone ones
- **Mutate**: Only safe transformations (no errors)

```go
// Filter: Can fail
filter := pipz.NewFilter("validate-if-needed", condition, validator)

// Mutate: Cannot fail
mutate := pipz.Mutate("modify-if-needed", transformer, condition)
```

### Filter vs Conditional Logic

```go
// Instead of embedding conditions
processor := pipz.Apply("mixed-logic", func(ctx context.Context, data Data) (Data, error) {
    if shouldProcess(data) {
        return expensiveOperation(ctx, data)
    }
    return data, nil
})

// Use Filter for cleaner separation
filter := pipz.NewFilter("clean-separation",
    shouldProcess,
    pipz.Apply("expensive", expensiveOperation),
)
```

## Testing

Test Filter by verifying both condition paths:

```go
func TestFilter(t *testing.T) {
    processor := pipz.Transform("double", func(ctx context.Context, n int) int {
        return n * 2
    })
    
    filter := pipz.NewFilter("even-only",
        func(ctx context.Context, n int) bool { return n%2 == 0 },
        processor,
    )
    
    // Test condition true
    result, err := filter.Process(context.Background(), 4)
    assert.NoError(t, err)
    assert.Equal(t, 8, result) // 4 * 2
    
    // Test condition false  
    result, err = filter.Process(context.Background(), 3)
    assert.NoError(t, err)
    assert.Equal(t, 3, result) // unchanged
}
```

## Gotchas

### ❌ Don't have side effects in conditions
```go
// WRONG - Condition modifies state
filter := pipz.NewFilter("bad",
    func(ctx context.Context, data Data) bool {
        counter++ // Side effect!
        log.Println("Checking...") // Side effect!
        return data.Important
    },
    processor,
)
```

### ✅ Keep conditions pure
```go
// RIGHT - Pure condition function
filter := pipz.NewFilter("good",
    func(ctx context.Context, data Data) bool {
        return data.Important
    },
    processor,
)
```

### ❌ Don't use for simple true/false transforms
```go
// WRONG - Overkill for simple conditional
filter := pipz.NewFilter("overkill",
    func(ctx context.Context, n int) bool { return n > 0 },
    pipz.Transform("abs", math.Abs),
)
```

### ✅ Use Mutate for simple conditional transforms
```go
// RIGHT - Simpler with Mutate
mutate := pipz.Mutate("abs-if-negative",
    func(ctx context.Context, n int) bool { return n < 0 },
    func(ctx context.Context, n int) int { return -n },
)
```

## Best Practices

1. **Keep conditions simple**: Complex logic makes debugging difficult
2. **Avoid side effects in conditions**: Conditions should be pure functions
3. **Use descriptive names**: Names appear in error paths
4. **Test both paths**: Verify condition true and false scenarios
5. **Consider caching**: For expensive condition calculations
6. **Use context**: Leverage context for timeouts and values
7. **Document behavior**: Make condition logic clear to other developers