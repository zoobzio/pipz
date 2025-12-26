---
title: "Enrich"
description: "Creates a processor that attempts to enhance data but doesn't fail the pipeline on error"
author: zoobzio
published: 2025-12-13
updated: 2025-12-13
tags:
  - reference
  - processors
  - optional
  - graceful-degradation
---

# Enrich

Creates a processor that attempts to enhance data but doesn't fail the pipeline on error.

> **Note**: Enrich is a convenience wrapper. You can always implement `Chainable[T]` directly for more control or stateful processors.

## Function Signature

```go
func Enrich[T any](identity Identity, fn func(context.Context, T) (T, error)) Chainable[T]
```

## Parameters

- `identity` (`Identity`) - Identifier for the processor used in error messages and debugging
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
enrichLocation := pipz.Enrich(
    pipz.NewIdentity("geocode-address", "Geocodes user address to coordinates"),
    func(ctx context.Context, user User) (User, error) {
        coords, err := geocodeAPI.Lookup(ctx, user.Address)
        if err != nil {
            // Error is logged but pipeline continues
            return user, fmt.Errorf("geocoding failed: %w", err)
        }
        user.Latitude = coords.Lat
        user.Longitude = coords.Lng
        return user, nil
    },
)

// Optional external data
enrichProfile := pipz.Enrich(
    pipz.NewIdentity("social-profile", "Fetches social profile data"),
    func(ctx context.Context, user User) (User, error) {
        profile, err := socialAPI.GetProfile(ctx, user.Email)
        if err != nil {
            // User proceeds without social data
            return user, err
        }
        user.Avatar = profile.Avatar
        user.Bio = profile.Bio
        return user, nil
    },
)

// Optional scoring
enrichRisk := pipz.Enrich(
    pipz.NewIdentity("risk-score", "Calculates transaction risk score"),
    func(ctx context.Context, transaction Transaction) (Transaction, error) {
        score, err := riskEngine.Calculate(ctx, transaction)
        if err != nil {
            // Transaction proceeds with default risk
            transaction.RiskScore = 0.5 // default medium risk
            return transaction, err
        }
        transaction.RiskScore = score
        return transaction, nil
    },
)

// Optional caching
enrichFromCache := pipz.Enrich(
    pipz.NewIdentity("cache-lookup", "Enriches item with cached data"),
    func(ctx context.Context, item Item) (Item, error) {
        cached, err := cache.Get(ctx, item.ID)
        if err != nil {
            // Proceed without cached data
            return item, err
        }
        item.CachedPrice = cached.Price
        item.CachedAt = cached.Timestamp
        return item, nil
    },
)
```

## When to Use

Use `Enrich` when:
- The enhancement is **optional and can fail silently**
- You're calling unreliable external services (third-party APIs)
- You want graceful degradation
- The data is useful even without enrichment
- You're adding nice-to-have features (recommendations, social data)
- External data sources may be temporarily unavailable

## When NOT to Use

Don't use `Enrich` when:
- The operation is required (use `Apply` - fail fast)
- You need to handle errors explicitly (use `Apply` with error handling)
- The operation cannot fail (use `Transform` for better performance)
- You need to know if enrichment failed (use `Apply` with fallback)
- Validation or critical business logic (use `Apply`)

## Error Tracking

While Enrich doesn't fail the pipeline, errors are still tracked:

```go
// Define identities upfront
var (
    UserPipelineID   = pipz.NewIdentity("user-pipeline", "User processing pipeline")
    ValidateUserID   = pipz.NewIdentity("validate-user", "Validates user data")
    GeocodeAddressID = pipz.NewIdentity("geocode-address", "Geocodes user address")
    FetchSocialID    = pipz.NewIdentity("fetch-social", "Fetches social profile")
    SaveUserID       = pipz.NewIdentity("save-user", "Saves user to database")
)

// Errors are logged internally but not returned
pipeline := pipz.NewSequence[User](UserPipelineID,
    pipz.Apply(ValidateUserID, validateUser),           // Can fail
    pipz.Enrich(GeocodeAddressID, geocodeAddress),      // Won't fail
    pipz.Enrich(FetchSocialID, fetchSocialProfile),     // Won't fail
    pipz.Apply(SaveUserID, saveUser),                   // Can fail
)

// The pipeline only fails if validate or save fail
// Enrich failures are logged but don't stop processing
```

## Common Patterns

```go
// Define identities upfront
var (
    EnrichmentID        = pipz.NewIdentity("enrichment", "Product enrichment pipeline")
    FetchReviewsID      = pipz.NewIdentity("fetch-reviews", "Fetches product reviews")
    CheckInventoryID    = pipz.NewIdentity("check-inventory", "Checks inventory levels")
    GetRecommendationsID = pipz.NewIdentity("get-recommendations", "Fetches product recommendations")
    DynamicPricingID    = pipz.NewIdentity("dynamic-pricing", "Calculates dynamic pricing")
    ParallelEnrichID    = pipz.NewIdentity("parallel-enrich", "Parallel user enrichment")
    GeocodeUserID       = pipz.NewIdentity("geocode-user", "Geocodes user location")
    FetchSocialID       = pipz.NewIdentity("fetch-social", "Fetches social data")
    LoadPreferencesID   = pipz.NewIdentity("load-preferences", "Loads user preferences")
    SmartEnrichID       = pipz.NewIdentity("smart-enrich", "Smart order enrichment")
    NeedsEnrichmentID   = pipz.NewIdentity("needs-enrichment", "Enriches high-value orders")
    PremiumDataID       = pipz.NewIdentity("premium-data", "Fetches premium data")
)

// Multiple optional enrichments
enrichmentPipeline := pipz.NewSequence[Product](EnrichmentID,
    pipz.Enrich(FetchReviewsID, fetchReviews),
    pipz.Enrich(CheckInventoryID, checkInventory),
    pipz.Enrich(GetRecommendationsID, getRecommendations),
    pipz.Enrich(DynamicPricingID, getDynamicPricing),
)

// Parallel optional enrichments
parallelEnrich := pipz.NewConcurrent[User](ParallelEnrichID,
    pipz.Enrich(GeocodeUserID, geocodeUser),
    pipz.Enrich(FetchSocialID, fetchSocialData),
    pipz.Enrich(LoadPreferencesID, loadPreferences),
)

// Conditional enrichment
smartEnrich := pipz.NewSequence[Order](SmartEnrichID,
    pipz.Mutate(NeedsEnrichmentID,
        func(ctx context.Context, order Order) Order {
            enriched, _ := pipz.Enrich(PremiumDataID,
                fetchPremiumData,
            ).Process(ctx, order)
            return enriched
        },
        func(ctx context.Context, order Order) bool {
            return order.Total > 1000 // Only enrich high-value orders
        },
    ),
)
```

## Gotchas

### ❌ Don't use for required operations
```go
// WRONG - Validation should fail the pipeline
enrich := pipz.Enrich(
    pipz.NewIdentity("validate-user", "Validates user"),
    func(ctx context.Context, user User) (User, error) {
        if !isValid(user) {
            return user, errors.New("invalid user") // Error is swallowed!
        }
        return user, nil
    },
)
```

### ✅ Use Apply for required operations
```go
// RIGHT - Validation fails the pipeline
apply := pipz.Apply(
    pipz.NewIdentity("validate-user", "Validates user"),
    func(ctx context.Context, user User) (User, error) {
        if !isValid(user) {
            return user, errors.New("invalid user")
        }
        return user, nil
    },
)
```

### ❌ Don't use when you need error details
```go
// Define identities upfront
var ProcessPaymentID = pipz.NewIdentity("process-payment", "Processes payment")

// WRONG - Can't handle specific errors
enrich := pipz.Enrich(ProcessPaymentID, processPayment) // Errors hidden
```

### ✅ Use Apply with explicit error handling
```go
// Define identities upfront
var (
    PaymentID       = pipz.NewIdentity("payment", "Payment processing with fallback")
    PrimaryPayID    = pipz.NewIdentity("primary-payment", "Processes payment via primary provider")
    BackupPayID     = pipz.NewIdentity("backup-payment", "Processes payment via backup provider")
)

// RIGHT - Handle errors explicitly
withFallback := pipz.NewFallback(PaymentID,
    pipz.Apply(PrimaryPayID, processPayment),
    pipz.Apply(BackupPayID, processBackupPayment),
)
```

## Best Practices

```go
// Provide defaults when enrichment fails
enrichWithDefault := pipz.Enrich(
    pipz.NewIdentity("weather-data", "Enriches event with weather information"),
    func(ctx context.Context, event Event) (Event, error) {
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
    },
)

// Log enrichment failures for monitoring
monitoredEnrich := pipz.Enrich(
    pipz.NewIdentity("external-api", "Enhances data via external API"),
    func(ctx context.Context, data Data) (Data, error) {
        result, err := externalAPI.Enhance(ctx, data.ID)
        if err != nil {
            // Log for monitoring but don't fail
            log.Printf("Enrichment failed for %s: %v", data.ID, err)
            metrics.Increment("enrichment.failures", "api", "external")
            return data, err
        }
        data.Enhanced = result
        return data, nil
    },
)
```

## See Also

- [Apply](./apply.md) - For required operations that can fail
- [Transform](./transform.md) - For operations that cannot fail
- [Effect](./effect.md) - For optional side effects