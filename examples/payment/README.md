# Payment Processing Example

This example demonstrates using pipz for payment processing pipelines with validation, fraud detection, and fallback providers.

## Overview

The payment pipeline shows how to:
- Validate payment data
- Perform fraud detection
- Process payments with multiple providers
- Handle failures gracefully
- Chain multiple processing stages

## Structure

```go
type Payment struct {
    ID             string
    Amount         float64
    Currency       string
    CardNumber     string  // Last 4 digits only
    CardLimit      float64
    CustomerEmail  string
    CustomerPhone  string
    Provider       string  // primary, backup, tertiary
    Attempts       int
    Status         string
    ProcessedAt    time.Time
}
```

## Processing Steps

1. **ValidatePayment**: Ensures amount is positive, card info and currency are present
2. **CheckFraud**: Basic fraud detection for large transactions and card limits
3. **UpdatePaymentStatus**: Tracks processing attempts
4. **ProcessWithPrimary**: Attempts processing with primary provider
5. **ProcessWithBackup**: Fallback provider for failed transactions

## Usage

```go
// Create payment pipeline
processor := CreatePaymentPipeline()

// Process a payment
result, err := processor.Process(payment)
if err != nil {
    // Payment failed - result is zero value
    // Could retry with backup provider
}
```

## Fallback Pattern

Since pipz returns zero values on error, fallback logic is implemented at the application level:

```go
result, err := primaryPipeline.Process(payment)
if err != nil {
    // Try backup provider
    result, err = backupPipeline.Process(payment)
}
```

## Running

```bash
# Run the example
go run payment.go

# Run tests
go test
```

## Key Features

- **Validation**: Multi-stage validation before processing
- **Fraud Detection**: Configurable rules for suspicious transactions
- **Provider Fallback**: Multiple payment providers for reliability
- **Error Handling**: Zero values on error (Go convention)
- **Audit Trail**: Tracks attempts and processing time

## Example Output

```
Processing valid payment...
✓ Payment PAY-001 processed successfully by primary provider

Processing large payment...
✗ Primary processing failed: primary provider: amount too large
✓ Payment PAY-002 processed successfully by backup provider

Processing suspicious payment...
✗ Payment blocked: FRAUD: amount exceeds card limit
```