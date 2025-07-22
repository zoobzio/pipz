# Quick Start Guide

Get up and running with pipz in 5 minutes!

## Installation

```bash
go get github.com/zoobzio/pipz
```

## Your First Pipeline

Let's build a simple data processing pipeline that validates, transforms, and enriches user data.

### Step 1: Define Your Data Type

```go
type User struct {
    ID       string
    Email    string
    Name     string
    Age      int
    Verified bool
}
```

### Step 2: Create Processors

```go
package main

import (
    "context"
    "fmt"
    "strings"
    "github.com/zoobzio/pipz"
)

// Validate ensures user data is correct
func validateUser(ctx context.Context, user User) (User, error) {
    if user.Email == "" {
        return user, fmt.Errorf("email is required")
    }
    if !strings.Contains(user.Email, "@") {
        return user, fmt.Errorf("invalid email format")
    }
    if user.Age < 0 || user.Age > 150 {
        return user, fmt.Errorf("invalid age: %d", user.Age)
    }
    return user, nil
}

// Normalize cleans up the data
func normalizeUser(ctx context.Context, user User) (User, error) {
    user.Email = strings.ToLower(strings.TrimSpace(user.Email))
    user.Name = strings.TrimSpace(user.Name)
    return user, nil
}

// Enrich adds computed fields
func enrichUser(ctx context.Context, user User) (User, error) {
    // Auto-verify users with company emails
    if strings.HasSuffix(user.Email, "@company.com") {
        user.Verified = true
    }
    return user, nil
}
```

### Step 3: Build the Pipeline

```go
func main() {
    // Create a pipeline using Sequential connector
    pipeline := pipz.Sequential(
        pipz.Apply("validate", validateUser),
        pipz.Apply("normalize", normalizeUser),
        pipz.Apply("enrich", enrichUser),
    )

    // Process a user
    user := User{
        ID:    "123",
        Email: "  John.Doe@Company.COM  ",
        Name:  "John Doe",
        Age:   30,
    }

    result, err := pipeline.Process(context.Background(), user)
    if err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
        return
    }

    fmt.Printf("Processed user: %+v\n", result)
    // Output: Processed user: {ID:123 Email:john.doe@company.com Name:John Doe Age:30 Verified:true}
}
```

## Adding Error Handling

Let's make our pipeline more robust with retry and fallback:

```go
// Simulate a flaky database save
func saveToDatabase(ctx context.Context, user User) (User, error) {
    // Randomly fail 50% of the time (for demo purposes)
    if time.Now().UnixNano()%2 == 0 {
        return user, fmt.Errorf("database connection failed")
    }
    fmt.Println("User saved to database")
    return user, nil
}

// Fallback to file storage
func saveToFile(ctx context.Context, user User) (User, error) {
    fmt.Println("User saved to file (fallback)")
    return user, nil
}

func main() {
    // Create a robust pipeline
    pipeline := pipz.Sequential(
        pipz.Apply("validate", validateUser),
        pipz.Apply("normalize", normalizeUser),
        pipz.Apply("enrich", enrichUser),
        // Try database 3 times, then fall back to file
        pipz.Fallback(
            pipz.Retry(
                pipz.Apply("save_db", saveToDatabase),
                3,
            ),
            pipz.Apply("save_file", saveToFile),
        ),
    )

    user := User{
        ID:    "123",
        Email: "john@example.com",
        Name:  "John",
        Age:   25,
    }

    result, err := pipeline.Process(context.Background(), user)
    if err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
        return
    }

    fmt.Printf("Successfully processed: %s\n", result.Email)
}
```

## Conditional Processing

Use the Switch connector for conditional logic:

```go
// Define route types for type safety
type UserCategory string

const (
    CategoryVIP      UserCategory = "vip"
    CategoryStandard UserCategory = "standard"
    CategoryInactive UserCategory = "inactive"
)

// Categorize users
func categorizeUser(ctx context.Context, user User) UserCategory {
    if !user.Verified {
        return CategoryInactive
    }
    if strings.HasSuffix(user.Email, "@vip.com") {
        return CategoryVIP
    }
    return CategoryStandard
}

// Create category-specific processors
func processVIP(ctx context.Context, user User) (User, error) {
    fmt.Println("VIP processing: priority support enabled")
    // Add VIP benefits...
    return user, nil
}

func processStandard(ctx context.Context, user User) (User, error) {
    fmt.Println("Standard processing")
    return user, nil
}

func processInactive(ctx context.Context, user User) (User, error) {
    fmt.Println("Inactive user: sending verification email")
    return user, nil
}

func main() {
    // Create a routing pipeline
    pipeline := pipz.Sequential(
        pipz.Apply("validate", validateUser),
        pipz.Apply("normalize", normalizeUser),
        
        // Route based on user category
        pipz.Switch(
            categorizeUser,
            map[UserCategory]pipz.Chainable[User]{
                CategoryVIP:      pipz.Apply("vip", processVIP),
                CategoryStandard: pipz.Apply("standard", processStandard),
                CategoryInactive: pipz.Apply("inactive", processInactive),
            },
        ),
    )

    // Process different types of users
    users := []User{
        {Email: "ceo@vip.com", Verified: true},
        {Email: "user@example.com", Verified: true},
        {Email: "new@example.com", Verified: false},
    }

    for _, user := range users {
        _, err := pipeline.Process(context.Background(), user)
        if err != nil {
            fmt.Printf("Failed to process %s: %v\n", user.Email, err)
        }
    }
}
```

## What's Next?

You've just built your first pipelines with pipz! Here's where to go next:

- [Processors](./concepts/processors.md) - Learn about all processor types
- [Connectors](./concepts/connectors.md) - Explore advanced composition patterns
- [Error Handling](./concepts/error-handling.md) - Build resilient pipelines
- [Examples](./examples/payment-processing.md) - See real-world implementations

## Key Takeaways

1. **Everything is composable** - Processors combine with connectors
2. **Type safety throughout** - No runtime type assertions needed
3. **Errors are explicit** - Handle failures at the right level
4. **Context flows through** - Cancellation and timeouts work automatically