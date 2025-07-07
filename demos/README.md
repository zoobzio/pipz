# Pipz Demonstrations

Interactive demonstrations showcasing pipz capabilities and best practices.

## Overview

These demos illustrate real-world use cases for building type-safe, composable processing pipelines in Go.

## Available Demos

### 1. Validation Demo
```bash
go run . validation
```
Demonstrates complex data validation with reusable validators:
- Order validation pipeline
- Early exit on errors
- Clear error messages
- Zero value returns

### 2. Payment Demo
```bash
go run . payment
```
Shows payment processing with fallback strategies:
- Multi-provider payment processing
- Fallback to backup providers
- Fraud detection
- Retry logic

### 3. Security Demo
```bash
go run . security
```
Illustrates security audit pipelines:
- Zero-trust access control
- Audit trail creation
- Role-based data redaction
- Compliance tracking

### 4. Transform Demo
```bash
go run . transform
```
ETL pipeline for data transformation:
- CSV to database conversion
- Data normalization
- Error collection
- Batch processing

### 5. Composability Demo
```bash
go run . composability
```
Building complex pipelines from simple parts:
- Modular pipeline design
- Pipeline chaining
- Conditional processing
- Reusable components

### 6. Error Demo
```bash
go run . errors
```
Error handling and recovery strategies:
- Validation errors
- Transient failures
- Retry mechanisms
- Fallback processing

### Run All Demos
```bash
go run . all
```

## Key Concepts Demonstrated

1. **Type Safety**: All pipelines maintain compile-time type checking
2. **Composability**: Build complex workflows from simple processors
3. **Error Handling**: Go-idiomatic error handling with zero values
4. **Performance**: Zero serialization between processors
5. **Flexibility**: Multiple strategies for different use cases

## Pipeline Patterns

### Basic Pipeline
```go
pipeline := pipz.NewContract[T]()
pipeline.Register(
    pipz.Apply(processor1),
    pipz.Transform(processor2),
    pipz.Validate(processor3),
)
```

### Chained Pipelines
```go
chain := pipz.NewChain[T]()
chain.Add(pipeline1, pipeline2, pipeline3)
```

### Conditional Processing
```go
pipz.Mutate(
    transformFunc,
    conditionFunc,
)
```

## Best Practices

1. **Small, Focused Processors**: Each processor should do one thing well
2. **Error First**: Validate early in the pipeline
3. **Immutability**: Clone data when modifying to avoid side effects
4. **Zero Values**: Return zero values on error (Go convention)
5. **Composability**: Design pipelines to be reusable

## Running the Demos

```bash
# Install dependencies
go mod tidy

# Run a specific demo
go run . <demo-name>

# Run all demos
go run . all
```