# Type Safety

pipz leverages Go generics to provide compile-time type safety throughout your pipelines.

## Generic Foundations

Every pipz component is generic over the data type it processes:

```go
// The core interface
type Chainable[T any] interface {
    Process(context.Context, T) (T, *Error[T])
    Name() Name
}

// Processors maintain type T throughout
func Transform[T any](name Name, fn func(context.Context, T) T) Chainable[T]
func Apply[T any](name Name, fn func(context.Context, T) (T, error)) Chainable[T]
```

## Type Consistency

The pipeline enforces type consistency at compile time:

```go
// This compiles - all processors work with User type
userPipeline := pipz.NewSequence[User]("user-pipeline")
userPipeline.Register(
    pipz.Apply("validate", func(ctx context.Context, u User) (User, error) {
        return u, nil
    }),
    pipz.Transform("normalize", func(ctx context.Context, u User) User {
        return u
    }),
)

// This won't compile - type mismatch
userPipeline := pipz.NewSequence[User]("user-pipeline")
userPipeline.Register(
    pipz.Apply("validate", validateUser),    // Processes User
    pipz.Apply("calculate", calculateOrder),  // Processes Order - ERROR!
)
```

## Working with Different Types

To transform between types, create explicit conversion processors:

```go
// Convert User to Account
func userToAccount(ctx context.Context, user User) (Account, error) {
    return Account{
        ID:       user.ID,
        Email:    user.Email,
        Balance:  0,
        Created:  time.Now(),
    }, nil
}

// Now you need separate pipelines
userPipeline := pipz.Apply("validate", validateUser)
accountPipeline := pipz.Apply("setup", setupAccount)

// Compose them manually
user, err := userPipeline.Process(ctx, inputUser)
if err != nil {
    return err
}
account, err := userToAccount(ctx, user)
if err != nil {
    return err
}
result, err := accountPipeline.Process(ctx, account)
```

## Generic Constraints

Some connectors require additional constraints:

### Comparable for Switch

Switch keys must be comparable:

```go
// Works - string is comparable
router := pipz.NewSwitch("type-router",
    func(ctx context.Context, data Data) string { 
        return data.Type 
    },
)
router.AddRoute("type1", processor1)
router.AddRoute("type2", processor2)

// Works - custom type based on comparable type
type RouteKey int
priorityRouter := pipz.NewSwitch("priority-router",
    func(ctx context.Context, data Data) RouteKey { 
        return data.Priority 
    },
)
priorityRouter.AddRoute(RouteKey(1), highPriorityProcessor)
priorityRouter.AddRoute(RouteKey(2), normalProcessor)

// Won't compile - slices aren't comparable
// This would fail at compile time when trying to use []string as map key
```

### Cloner for Concurrent Processing

Concurrent and Race require the Cloner interface:

```go
// Types must implement Cloner for concurrent processing
type Order struct {
    ID    string
    Items []Item
}

func (o Order) Clone() Order {
    items := make([]Item, len(o.Items))
    copy(items, o.Items)
    return Order{
        ID:    o.ID,
        Items: items,
    }
}

// Now it can be used with Concurrent
pipeline := pipz.NewConcurrent[Order]("order-processing",
    notifyWarehouse,
    updateInventory,
    sendConfirmation,
)
```

Without Cloner, the type system prevents data races:

```go
type UnsafeOrder struct {
    ID    string
    Items []Item
}

// This won't compile - UnsafeOrder doesn't implement Cloner
pipeline := pipz.NewConcurrent[UnsafeOrder]("unsafe-processing",
    processor1,
    processor2,
) // ERROR: UnsafeOrder does not implement Cloner[UnsafeOrder]
```

## Type-Safe Routing

Use custom types for routing keys:

```go
// Define semantic route types
type CustomerSegment string
const (
    SegmentVIP      CustomerSegment = "vip"
    SegmentRegular  CustomerSegment = "regular"
    SegmentInactive CustomerSegment = "inactive"
)

// Type-safe routing
router := pipz.NewSwitch("customer-router",
    func(ctx context.Context, c Customer) CustomerSegment {
        if c.TotalSpent > 10000 {
            return SegmentVIP
        }
        if c.LastOrderDate.Before(time.Now().AddDate(0, -6, 0)) {
            return SegmentInactive
        }
        return SegmentRegular
    },
)

// Add routes
router.AddRoute(SegmentVIP, vipPipeline)
router.AddRoute(SegmentRegular, regularPipeline)
router.AddRoute(SegmentInactive, reactivationPipeline)
```

## Generic Pipeline Builders

Create reusable pipeline patterns:

```go
// Generic validation pipeline builder
func ValidationPipeline[T any](
    validator func(context.Context, T) error,
    enricher func(context.Context, T) T,
) pipz.Chainable[T] {
    seq := pipz.NewSequence[T]("validation")
    seq.Register(
        pipz.Effect("validate", validator),
        pipz.Transform("enrich", enricher),
        pipz.Effect("audit", func(ctx context.Context, data T) error {
            log.Printf("Validated: %+v", data)
            return nil
        }),
    )
    return seq
}

// Use with any type
userValidation := ValidationPipeline(validateUser, enrichUser)
orderValidation := ValidationPipeline(validateOrder, enrichOrder)
```

## Interface Constraints

Use interface constraints for common operations:

```go
// Identifiable constraint
type Identifiable interface {
    GetID() string
}

// Generic deduplication processor
func Deduplicate[T Identifiable]() pipz.Chainable[T] {
    seen := make(map[string]bool)
    var mu sync.Mutex
    
    return pipz.Apply("deduplicate", func(ctx context.Context, item T) (T, error) {
        mu.Lock()
        defer mu.Unlock()
        
        id := item.GetID()
        if seen[id] {
            return item, errors.New("duplicate item")
        }
        seen[id] = true
        return item, nil
    })
}

// Works with any type that has GetID()
userDedup := Deduplicate[User]()
orderDedup := Deduplicate[Order]()
```

## Zero Runtime Type Assertions

Unlike interface{}-based pipelines, pipz never requires runtime type assertions:

```go
// Old way - runtime type assertions needed
func oldProcessor(data interface{}) (interface{}, error) {
    user, ok := data.(User) // Runtime check
    if !ok {
        return nil, errors.New("expected User type")
    }
    // Process user...
}

// pipz way - compile-time type safety
func newProcessor(ctx context.Context, user User) (User, error) {
    // user is guaranteed to be User type
    // No runtime checks needed
}
```

## Benefits

1. **Compile-Time Safety**: Type errors caught during development
2. **Better IDE Support**: Full autocomplete and type information
3. **No Runtime Overhead**: No reflection or type assertions
4. **Self-Documenting**: Types clearly show what data flows through
5. **Refactoring Safety**: Change types and compiler shows all affected code

## Limitations

1. **Single Type Per Pipeline**: Can't mix types within a pipeline
2. **Explicit Conversions**: Must write converters between types
3. **Generic Syntax**: Some developers find generics syntax complex

## Next Steps

- [Best Practices](../guides/best-practices.md) - Type design patterns
- [Testing](../guides/testing.md) - Testing generic code
- [Performance](../guides/performance.md) - Generic performance considerations