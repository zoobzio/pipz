# Enrich

Creates a processor that attempts to enhance data but doesn't fail the pipeline on error.

> **Note**: Enrich is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Enrich[T any](name Name, fn func(context.Context, T) (T, error)) Chainable[T]
```

## Parameters

- `name` (`Name`) - Identifier for the processor used in error messages and debugging
- `fn` - Enrichment function that attempts to enhance the data

## Returns

Returns a `Chainable[T]` that enhances data when possible, passes through original on failure.

## Behavior

- **Best effort** - Tries to enhance data but continues on failure
- **Non-failing** - Errors are logged but don't stop the pipeline
- **Graceful degradation** - Returns original input if enrichment fails
- **Error visibility** - Failures are included in error tracking but not propagated

## Example

```go
// Optional geolocation
enrichLocation := pipz.Enrich("geocode", func(ctx context.Context, user User) (User, error) {
    coords, err := geocodeAPI.Lookup(ctx, user.Address)
    if err != nil {
        // Error is logged but pipeline continues
        return user, fmt.Errorf("geocoding failed: %w", err)
    }
    user.Latitude = coords.Lat
    user.Longitude = coords.Lng
    return user, nil
})

// Optional external data
enrichProfile := pipz.Enrich("social-profile", func(ctx context.Context, user User) (User, error) {
    profile, err := socialAPI.GetProfile(ctx, user.Email)
    if err != nil {
        // User proceeds without social data
        return user, err
    }
    user.Avatar = profile.Avatar
    user.Bio = profile.Bio
    return user, nil
})

// Optional scoring
enrichRisk := pipz.Enrich("risk-score", func(ctx context.Context, transaction Transaction) (Transaction, error) {
    score, err := riskEngine.Calculate(ctx, transaction)
    if err != nil {
        // Transaction proceeds with default risk
        transaction.RiskScore = 0.5 // default medium risk
        return transaction, err
    }
    transaction.RiskScore = score
    return transaction, nil
})

// Optional caching
enrichFromCache := pipz.Enrich("cache-lookup", func(ctx context.Context, item Item) (Item, error) {
    cached, err := cache.Get(ctx, item.ID)
    if err != nil {
        // Proceed without cached data
        return item, err
    }
    item.CachedPrice = cached.Price
    item.CachedAt = cached.Timestamp
    return item, nil
})
```

## When to Use

Use `Enrich` when:
- The enhancement is optional
- You're calling unreliable external services
- You want graceful degradation
- The data is useful even without enrichment
- You're adding nice-to-have features

## When NOT to Use

Don't use `Enrich` when:
- The operation is required (use `Apply`)
- You need to handle errors explicitly (use `Apply` with error handling)
- The operation cannot fail (use `Transform`)
- You need to know if enrichment failed (use `Apply` with fallback)

## Error Tracking

While Enrich doesn't fail the pipeline, errors are still tracked:

```go
// Errors are logged internally but not returned
pipeline := pipz.NewSequence[User]("user-pipeline",
    pipz.Apply("validate", validateUser),        // Can fail
    pipz.Enrich("geocode", geocodeAddress),      // Won't fail
    pipz.Enrich("social", fetchSocialProfile),   // Won't fail
    pipz.Apply("save", saveUser),                // Can fail
)

// The pipeline only fails if validate or save fail
// Enrich failures are logged but don't stop processing
```

## Common Patterns

```go
// Multiple optional enrichments
enrichmentPipeline := pipz.NewSequence[Product]("enrichment",
    pipz.Enrich("reviews", fetchReviews),
    pipz.Enrich("inventory", checkInventory),
    pipz.Enrich("recommendations", getRecommendations),
    pipz.Enrich("pricing", getDynamicPricing),
)

// Parallel optional enrichments
parallelEnrich := pipz.NewConcurrent[User](
    pipz.Enrich("location", geocodeUser),
    pipz.Enrich("social", fetchSocialData),
    pipz.Enrich("preferences", loadPreferences),
)

// Conditional enrichment
smartEnrich := pipz.NewSequence[Order]("smart-enrich",
    pipz.Mutate("needs-enrichment",
        func(ctx context.Context, order Order) bool {
            return order.Total > 1000 // Only enrich high-value orders
        },
        func(ctx context.Context, order Order) Order {
            enriched, _ := pipz.Enrich("premium-data", fetchPremiumData).Process(ctx, order)
            return enriched
        },
    ),
)
```

## Best Practices

```go
// Provide defaults when enrichment fails
enrichWithDefault := pipz.Enrich("weather", func(ctx context.Context, event Event) (Event, error) {
    weather, err := weatherAPI.GetCurrent(ctx, event.Location)
    if err != nil {
        // Provide sensible default
        event.Weather = "unknown"
        event.Temperature = 20.0 // room temperature
        return event, err
    }
    event.Weather = weather.Condition
    event.Temperature = weather.Temp
    return event, nil
})

// Log enrichment failures for monitoring
monitoredEnrich := pipz.Enrich("external-api", func(ctx context.Context, data Data) (Data, error) {
    result, err := externalAPI.Enhance(ctx, data.ID)
    if err != nil {
        // Log for monitoring but don't fail
        log.Printf("Enrichment failed for %s: %v", data.ID, err)
        metrics.Increment("enrichment.failures", "api", "external")
        return data, err
    }
    data.Enhanced = result
    return data, nil
})
```

## See Also

- [Apply](./apply.md) - For required operations that can fail
- [Transform](./transform.md) - For operations that cannot fail
- [Effect](./effect.md) - For optional side effects