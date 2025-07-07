# Validation Example

This example demonstrates using pipz for data validation pipelines in an e-commerce context.

## Overview

The validation pipeline shows how to:
- Chain multiple validation functions
- Handle errors appropriately (returning zero values on error)
- Use the `Apply` adapter for validation functions
- Compose validation logic in a clean, maintainable way

## Structure

```go
type Order struct {
    ID         string
    CustomerID string
    Items      []OrderItem
    Total      float64
}
```

## Validation Steps

1. **ValidateOrderID**: Ensures order IDs follow the required format (must start with "ORD-")
2. **ValidateItems**: Validates that items have positive quantities, valid prices, and product IDs
3. **ValidateTotal**: Verifies the order total matches the sum of item prices

## Usage

```go
// Create validation pipeline
validator := CreateValidationPipeline()

// Process an order
result, err := validator.Process(order)
if err != nil {
    // Validation failed - result will be zero value
    log.Printf("Validation failed: %v", err)
}
```

## Running

```bash
# Run the example
go run validation.go

# Run tests
go test
```

## Key Features

- **Early Exit**: Pipeline stops on first validation error
- **Zero Values on Error**: Failed validation returns zero value (Go convention)
- **Composable**: Individual validators can be mixed and matched
- **Type Safe**: Full compile-time type checking

## Example Output

```
Validating valid order...
✓ Order ORD-12345 validated successfully

Validating invalid order...
✗ Validation failed: total mismatch: expected 59.97, got 50.00

Validating order with bad ID...
✗ Validation failed: order ID must start with ORD-
```