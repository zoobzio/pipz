# Adapters in pipz

Adapters are the bridge between your business logic and pipz's byte-based pipeline system. They let you write simple functions that work with your types, while pipz handles all the serialization magic behind the scenes.

## Key Types

Throughout this document, we'll use these key types for pipeline identification:

```go
type UserKey string
type OrderKey string
```

These key types help pipz identify and retrieve the correct pipeline for your data types.

## Why Adapters?

Under the hood, pipz pipelines work with bytes. This design enables:
- **Type-agnostic composition** - Any type can flow through the same pipeline infrastructure
- **Performance optimization** - Processors can skip serialization when data hasn't changed
- **Cross-team isolation** - Teams can work with different types without coordination

But working with bytes directly is cumbersome:

```go
// Without adapters - you handle serialization manually
func validateUser(u User) ([]byte, error) {
    if u.Age < 0 {
        return nil, errors.New("invalid age")
    }
    // Did we modify? No, so return nil
    return nil, nil
}

func normalizeEmail(u User) ([]byte, error) {
    u.Email = strings.ToLower(u.Email)
    // We modified, so we must encode
    return pipz.Encode(u)
}
```

Adapters eliminate this boilerplate, letting you focus on business logic:

```go
// With adapters - just write normal functions
const userKey UserKey = "validation-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    pipz.Validate(func(u User) error {
        if u.Age < 0 {
            return errors.New("invalid age")
        }
        return nil
    }),
    pipz.Transform(func(u User) User {
        u.Email = strings.ToLower(u.Email)
        return u
    }),
)
```

## Built-in Adapters

### Transform
Always modifies data. Use this for operations that change your data every time.

```go
// Signature: func(T) T

// Inline usage
const userKey UserKey = "transform-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    pipz.Transform(func(u User) User {
        u.UpdatedAt = time.Now()
        return u
    }),
)

// Or with named functions
func addTimestamp(u User) User {
    u.UpdatedAt = time.Now()
    return u
}

const userKey UserKey = "timestamp-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(pipz.Transform(addTimestamp))

// Your functions don't need to know about pipz!
// This can be in a completely different package:
package business

func NormalizeUser(u User) User {
    u.Name = strings.Title(u.Name)
    u.Email = strings.ToLower(u.Email)
    return u
}

// Then in your pipeline setup:
const userKey UserKey = "normalize-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(pipz.Transform(business.NormalizeUser))
```

### Apply
For transformations that might fail. The pipeline stops if an error is returned.

```go
// Signature: func(T) (T, error)

// Common pattern: validation with modification
const userKey UserKey = "apply-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    pipz.Apply(func(u User) (User, error) {
        if u.Email == "" {
            return u, errors.New("email required")
        }
        u.Email = strings.ToLower(u.Email)
        u.Validated = true
        return u, nil
    }),
)

// Great for API calls that might fail
func enrichUserFromAPI(u User) (User, error) {
    profile, err := api.GetProfile(u.ID)
    if err != nil {
        return u, fmt.Errorf("failed to get profile: %w", err)
    }
    u.Profile = profile
    return u, nil
}

const userKey UserKey = "enrich-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(pipz.Apply(enrichUserFromAPI))
```

### Validate
Check data without modifying it. No serialization happens, making this very efficient.

```go
// Signature: func(T) error

// Perfect for guard clauses
const orderKey OrderKey = "validate-v1"
contract := pipz.GetContract[Order](orderKey)
contract.Register(
    pipz.Validate(func(o Order) error {
        if o.Total <= 0 {
            return errors.New("order total must be positive")
        }
        return nil
    }),
    pipz.Validate(func(o Order) error {
        if len(o.Items) == 0 {
            return errors.New("order must have items")
        }
        return nil
    }),
)

// Reusable validators
package validators

func ValidateAge(u User) error {
    if u.Age < 0 || u.Age > 150 {
        return fmt.Errorf("invalid age: %d", u.Age)
    }
    return nil
}

func ValidateEmail(u User) error {
    if !strings.Contains(u.Email, "@") {
        return fmt.Errorf("invalid email: %s", u.Email)
    }
    return nil
}

// Use them across different pipelines
const userKey UserKey = "user-validation-v1"
userContract := pipz.GetContract[User](userKey)
userContract.Register(
    pipz.Validate(validators.ValidateAge),
    pipz.Validate(validators.ValidateEmail),
)
```

### Mutate
Conditionally modify data based on business rules.

```go
// Signature: func(T) T, func(T) bool

// Apply discounts conditionally
const orderKey OrderKey = "discount-v1"
contract := pipz.GetContract[Order](orderKey)
contract.Register(
    pipz.Mutate(
        func(o Order) Order {
            o.Discount = 0.10
            o.DiscountReason = "VIP customer"
            return o
        },
        func(o Order) bool {
            return o.Customer.Tier == "VIP"
        },
    ),
)

// Multiple conditional modifications
func applyBulkDiscount(o Order) Order {
    o.Discount = 0.15
    o.DiscountReason = "Bulk order"
    return o
}

func isBulkOrder(o Order) bool {
    return o.Total > 1000
}

func applyHolidayPricing(o Order) Order {
    o.Discount = 0.20
    o.DiscountReason = "Holiday sale"
    return o
}

func isHolidaySeason(o Order) bool {
    month := o.Date.Month()
    return month == time.November || month == time.December
}

const orderKey OrderKey = "pricing-v1"
contract := pipz.GetContract[Order](orderKey)
contract.Register(
    pipz.Mutate(applyBulkDiscount, isBulkOrder),
    pipz.Mutate(applyHolidayPricing, isHolidaySeason),
)
```

### Effect
Run side effects without modifying data. Perfect for logging, metrics, and notifications.

```go
// Signature: func(T) error

// Logging and metrics
const userKey UserKey = "logging-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    pipz.Effect(func(u User) error {
        log.Printf("Processing user: %s", u.ID)
        return nil
    }),
    pipz.Effect(func(u User) error {
        metrics.Increment("users.processed", 1)
        metrics.Gauge("user.age", u.Age)
        return nil
    }),
)

// External notifications
func notifyUserService(u User) error {
    return userService.Notify(u.ID, "profile.updated")
}

func sendWelcomeEmail(u User) error {
    if u.IsNew {
        return emailService.Send(u.Email, "welcome")
    }
    return nil
}

const userKey UserKey = "notifications-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    pipz.Effect(notifyUserService),
    pipz.Effect(sendWelcomeEmail),
)

// Audit trails
const orderKey OrderKey = "audit-v1"
contract := pipz.GetContract[Order](orderKey)
contract.Register(
    pipz.Effect(func(o Order) error {
        audit.Log(AuditEntry{
            Action: "order.processed",
            OrderID: o.ID,
            Amount: o.Total,
            Time: time.Now(),
        })
        return nil
    }),
)
```

### Enrich
Fetch additional data with graceful degradation. If enrichment fails, processing continues with original data.

```go
// Signature: func(T) (T, error)

// Optional API enrichment
const userKey UserKey = "social-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    pipz.Enrich(func(u User) (User, error) {
        // Try to get social profile
        social, err := socialAPI.GetProfile(u.Email)
        if err != nil {
            // This error is logged but doesn't stop processing
            return u, err
        }
        u.SocialProfile = social
        return u, nil
    }),
)

// Database lookups that shouldn't break the flow
func enrichWithPurchaseHistory(u User) (User, error) {
    history, err := db.GetPurchaseHistory(u.ID)
    if err != nil {
        // Maybe DB is slow - don't fail the whole pipeline
        return u, fmt.Errorf("couldn't get purchase history: %w", err)
    }
    u.TotalPurchases = history.Total
    u.LastPurchase = history.LastDate
    return u, nil
}

const userKey UserKey = "purchase-history-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(pipz.Enrich(enrichWithPurchaseHistory))
```

## Creating Custom Adapters

Adapters are just functions that return `Processor[T]`. Here's the pattern:

```go
func MyAdapter[T any](/* your parameters */) Processor[T] {
    return func(value T) ([]byte, error) {
        // Your logic here
        
        // If data changed, encode it:
        return Encode(modifiedValue)
        
        // If no changes, return nil:
        return nil, nil
        
        // If error, return it:
        return nil, err
    }
}
```

### Example: Retry Adapter

Wrap flaky operations with automatic retry:

```go
func Retry[T any](fn func(T) (T, error), maxAttempts int) pipz.Processor[T] {
    return func(value T) ([]byte, error) {
        var lastErr error
        
        for i := 0; i < maxAttempts; i++ {
            result, err := fn(value)
            if err == nil {
                return pipz.Encode(result)
            }
            lastErr = err
            
            // Exponential backoff
            if i < maxAttempts-1 {
                time.Sleep(time.Millisecond * time.Duration(100 * (i+1)))
            }
        }
        
        return nil, fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
    }
}

// Usage
const userKey UserKey = "retry-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    Retry(enrichFromFlakeyAPI, 3),
    Retry(saveToDatabase, 5),
)
```

### Example: Cache Adapter

Add caching to expensive operations:

```go
func Cache[T any](fn func(T) (T, error), keyFn func(T) string, ttl time.Duration) pipz.Processor[T] {
    return func(value T) ([]byte, error) {
        key := keyFn(value)
        
        // Check cache
        if cached, found := cache.Get(key); found {
            return cached.([]byte), nil
        }
        
        // Call function
        result, err := fn(value)
        if err != nil {
            return nil, err
        }
        
        // Cache result
        encoded := pipz.Encode(result)
        cache.Set(key, encoded, ttl)
        
        return encoded, nil
    }
}

// Usage
const userKey UserKey = "cache-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    Cache(
        expensiveEnrichment,
        func(u User) string { return u.ID },
        5*time.Minute,
    ),
)
```

### Example: Timeout Adapter

Prevent slow operations from blocking:

```go
func Timeout[T any](fn func(T) (T, error), duration time.Duration) pipz.Processor[T] {
    return func(value T) ([]byte, error) {
        ctx, cancel := context.WithTimeout(context.Background(), duration)
        defer cancel()
        
        resultCh := make(chan struct{result T; err error}, 1)
        
        go func() {
            result, err := fn(value)
            resultCh <- struct{result T; err error}{result, err}
        }()
        
        select {
        case <-ctx.Done():
            return nil, fmt.Errorf("operation timed out after %v", duration)
        case r := <-resultCh:
            if r.err != nil {
                return nil, r.err
            }
            return pipz.Encode(r.result)
        }
    }
}

// Usage
const userKey UserKey = "timeout-v1"
contract := pipz.GetContract[User](userKey)
contract.Register(
    Timeout(callSlowAPI, 5*time.Second),
)
```

### Example: Batch Adapter

Process items in batches for efficiency:

```go
type Batchable interface {
    GetID() string
}

func Batch[T Batchable](fn func([]T) error, size int, timeout time.Duration) pipz.Processor[T] {
    var (
        batch []T
        mu    sync.Mutex
        timer *time.Timer
    )
    
    flush := func() error {
        if len(batch) == 0 {
            return nil
        }
        
        err := fn(batch)
        batch = nil
        return err
    }
    
    return func(value T) ([]byte, error) {
        mu.Lock()
        defer mu.Unlock()
        
        batch = append(batch, value)
        
        if timer == nil {
            timer = time.AfterFunc(timeout, func() {
                mu.Lock()
                defer mu.Unlock()
                flush()
            })
        }
        
        if len(batch) >= size {
            if err := flush(); err != nil {
                return nil, err
            }
            timer.Stop()
            timer = nil
        }
        
        // Batch processor doesn't modify individual items
        return nil, nil
    }
}
```

## Best Practices

1. **Keep functions pure** - Adapters work best with pure functions that don't have side effects
2. **Use the right adapter** - Don't use `Transform` for validation or `Effect` for modifications
3. **Name your functions** - While inline functions are convenient, named functions are more testable
4. **Compose small functions** - Many small, focused processors are better than few large ones
5. **Handle errors appropriately** - Use `Apply` when errors should stop processing, `Enrich` when they shouldn't

## Summary

Adapters are pipz's way of making byte-based pipelines feel natural. They handle all the serialization complexity so you can focus on writing clean, testable business logic. The built-in adapters cover most use cases, but creating custom adapters is straightforward when you need specialized behavior.

Remember: your business logic functions don't need to import pipz or know anything about pipelines. Keep them pure, simple, and focused on their specific task.