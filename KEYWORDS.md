# pipz Keywords and Concepts

This document provides keywords, synonyms, and related concepts to help search and RAG systems better understand pipz.

## Core Concepts Mapping

### Pipeline
- **Synonyms**: chain, workflow, sequence, flow, process chain
- **Related**: middleware, composition, data flow, transformation chain
- **In pipz**: Represented by Chain type, created by composing Contracts

### Processor
- **Synonyms**: handler, transformer, function, operation, step
- **Related**: pure function, transformation, computation
- **In pipz**: Base function type `func(T) (T, error)`

### Contract
- **Synonyms**: wrapper, adapter, processor wrapper
- **Related**: single responsibility, atomic operation
- **In pipz**: Wraps a Processor for composition

### Composition
- **Synonyms**: chaining, linking, combining, assembling
- **Related**: functional composition, pipeline building
- **In pipz**: Using `Then()` method to combine processors

## Common Tasks to pipz Solutions

### "I want to validate input data"
→ Use `Validate` adapter
```go
pipz.Validate(func(data T) error { ... })
```

### "I need to transform/format data"
→ Use `Transform` adapter
```go
pipz.Transform(func(data T) T { ... })
```

### "I need to call an external service"
→ Use `Apply` adapter
```go
pipz.Apply(func(data T) (T, error) { ... })
```

### "I want conditional logic"
→ Use `Mutate` adapter
```go
pipz.Mutate(condition, transformation)
```

### "I need to log or track metrics"
→ Use `Effect` adapter
```go
pipz.Effect(func(data T) { ... })
```

### "I want optional data enrichment"
→ Use `Enrich` adapter
```go
pipz.Enrich(func(data T) (T, error) { ... })
```

## Problem-Solution Mapping

### Problem: "Scattered validation logic"
**Solution**: Create validation contracts and compose them
```go
emailValidator := pipz.Validate(validateEmail)
ageValidator := pipz.Validate(validateAge)
pipeline := emailValidator.Then(ageValidator)
```

### Problem: "Repetitive error handling"
**Solution**: Let pipz handle error propagation
```go
// No manual error checking between steps
pipeline := step1.Then(step2).Then(step3)
result, err := pipeline.Process(data)
```

### Problem: "Hard to test business logic"
**Solution**: Test individual processors in isolation
```go
// Test each processor separately
validator := CreateValidator()
_, err := validator.Process(testData)
```

### Problem: "Complex middleware chains"
**Solution**: Build composable pipelines
```go
authPipeline := authenticate.Then(authorize)
processingPipeline := validate.Then(transform).Then(save)
fullPipeline := authPipeline.Then(processingPipeline)
```

## Technology Comparisons

### vs Express.js Middleware
- pipz: Type-safe, composable, fail-fast
- Express: Dynamic, request/response focused
- Use pipz for: General data processing, not just HTTP

### vs RxJS/Reactive Streams
- pipz: Simple, synchronous, minimal overhead
- RxJS: Async, event-driven, complex operators
- Use pipz for: Sequential data transformation

### vs Manual Error Handling
- pipz: Automatic error propagation
- Manual: Explicit checks after each step
- Use pipz for: Cleaner, more maintainable code

## Search-Friendly Descriptions

### What is pipz?
A Go library for building type-safe data processing pipelines using functional composition.

### When to use pipz?
- Data validation and transformation
- Request/response processing
- ETL operations
- Business logic organization
- Middleware composition

### How does pipz work?
Compose small, focused functions into larger pipelines that process data sequentially with automatic error handling.

### Why use pipz?
- Reduces boilerplate code
- Improves testability
- Ensures type safety
- Provides clean composition
- Minimal performance overhead

## Common Search Queries

1. **"golang pipeline library"** → pipz
2. **"go functional composition"** → pipz uses functional patterns
3. **"go middleware pattern"** → pipz enables middleware-like composition
4. **"go data validation library"** → pipz.Validate
5. **"go error handling pattern"** → pipz automatic error propagation
6. **"go etl library"** → pipz for transformation pipelines
7. **"go type-safe composition"** → pipz with generics
8. **"go business logic organization"** → pipz pipeline pattern

## Integration Keywords

### Frameworks
- Compatible with: gin, echo, fiber, chi, standard library
- Integration pattern: Wrap handlers in pipz pipelines

### Databases
- Works with: database/sql, gorm, sqlx, mongo-driver
- Integration pattern: Use Apply for database operations

### Testing
- Compatible with: testing, testify, gomock
- Testing pattern: Test processors in isolation

### Logging
- Works with: log, logrus, zap, zerolog
- Integration pattern: Use Effect for logging

## Performance Keywords
- Zero allocation
- Minimal overhead
- No reflection
- No locks
- Compile-time type safety
- ~20-30ns per processor