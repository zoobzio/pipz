# Quick Start Guide

Build your first pipeline in 5 minutes!

## Installation

```bash
go get github.com/zoobzio/pipz
```

Requires Go 1.21 or later.

## Your First Pipeline

Let's build a simple pipeline that processes user data through validation, normalization, and enrichment steps.

```go
package main

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/zoobzio/pipz"
)

// Define your data type
type User struct {
    Email    string
    Name     string
    Verified bool
}

// Create processing functions
func validateUser(ctx context.Context, user User) (User, error) {
    if user.Email == "" {
        return user, fmt.Errorf("email is required")
    }
    if !strings.Contains(user.Email, "@") {
        return user, fmt.Errorf("invalid email format")
    }
    return user, nil
}

func normalizeUser(ctx context.Context, user User) User {
    user.Email = strings.ToLower(strings.TrimSpace(user.Email))
    user.Name = strings.TrimSpace(user.Name)
    return user
}

func enrichUser(ctx context.Context, user User) User {
    // Mark company emails as verified
    if strings.HasSuffix(user.Email, "@company.com") {
        user.Verified = true
    }
    return user
}

func main() {
    // Create processors from your functions
    validate := pipz.Apply("validate", validateUser)
    normalize := pipz.Transform("normalize", normalizeUser)
    enrich := pipz.Transform("enrich", enrichUser)
    
    // Compose them into a pipeline
    pipeline := pipz.NewSequence[User]("user-processing",
        validate,
        normalize,
        enrich,
    )
    
    // Process some data
    user := User{
        Email: "  John.Doe@Company.COM  ",
        Name:  "John Doe",
    }
    
    result, err := pipeline.Process(context.Background(), user)
    if err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
        return
    }
    
    fmt.Printf("Result: %+v\n", result)
    // Output: Result: {Email:john.doe@company.com Name:John Doe Verified:true}
}
```

## Key Concepts

### Processors
Transform your data using adapter functions:
- `Transform` - Pure transformations that cannot fail
- `Apply` - Operations that can return errors
- `Effect` - Side effects without modifying data
- `Mutate` - Conditional modifications
- `Enrich` - Optional enhancements

### Sequences
Compose processors into pipelines:
```go
pipeline := pipz.NewSequence[T]("name",
    processor1,
    processor2,
    processor3,
)
```

### Error Handling
pipz provides rich error context:
```go
result, err := pipeline.Process(ctx, data)
if err != nil {
    var pipeErr *pipz.Error[User]
    if errors.As(err, &pipeErr) {
        fmt.Printf("Failed at: %v\n", pipeErr.Path)
        fmt.Printf("Input: %+v\n", pipeErr.InputData)
    }
}
```

## What's Next?

Now that you've built your first pipeline:

- **[Concepts](./concepts/processors.md)** - Deep dive into processors and connectors
- **[Building Pipelines](./guides/first-pipeline.md)** - Learn composition patterns
- **[Error Handling](./concepts/error-handling.md)** - Build resilient pipelines
- **[Examples](./examples/payment-processing.md)** - See real-world use cases

## Need Help?

- Check the [API Reference](./api/transform.md) for detailed documentation
- Browse [examples](https://github.com/zoobzio/pipz/tree/main/examples) for more patterns
- Read the [best practices guide](./guides/best-practices.md)