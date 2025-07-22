# Payment Processing Example

This example demonstrates building a robust payment processing system with multiple providers, intelligent failover, and comprehensive error handling.

## Overview

The payment processing pipeline showcases:
- Multi-provider support (Stripe, PayPal, Square)
- Smart error categorization and recovery
- High-value payment special handling
- Provider health tracking
- Customer notifications

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Validate   │────▶│ Select Route │────▶│   Process   │
│   Payment   │     │ (High/Normal)│     │   Payment   │
└─────────────┘     └──────────────┘     └─────────────┘
                                               │
                                               ▼
                    ┌──────────────┐     ┌─────────────┐
                    │    Handle    │◀────│    Check    │
                    │    Errors    │     │   Result    │
                    └──────────────┘     └─────────────┘
```

## Implementation

### Data Models

```go
type Payment struct {
    ID         string
    CustomerID string
    Amount     float64
    Currency   string
    Method     string
    Provider   string
    Status     string
    Error      error
    Metadata   map[string]string
}

type PaymentResult struct {
    Payment       Payment
    TransactionID string
    Success       bool
    Message       string
}
```

### Error Categories

The system categorizes errors for appropriate handling:

```go
type ErrorCategory string

const (
    CategoryInsufficientFunds ErrorCategory = "insufficient_funds"
    CategoryNetworkError      ErrorCategory = "network_error"
    CategoryRateLimit         ErrorCategory = "rate_limit"
    CategoryProviderDown      ErrorCategory = "provider_unavailable"
    CategoryFraudSuspected    ErrorCategory = "fraud_suspected"
    CategoryUnknown           ErrorCategory = "unknown"
)

func categorizeError(err error) ErrorCategory {
    errStr := strings.ToLower(err.Error())
    
    switch {
    case strings.Contains(errStr, "insufficient funds"),
         strings.Contains(errStr, "card_declined"):
        return CategoryInsufficientFunds
    case strings.Contains(errStr, "timeout"),
         strings.Contains(errStr, "connection refused"):
        return CategoryNetworkError
    case strings.Contains(errStr, "rate limit"):
        return CategoryRateLimit
    case strings.Contains(errStr, "service unavailable"):
        return CategoryProviderDown
    case strings.Contains(errStr, "fraud"):
        return CategoryFraudSuspected
    default:
        return CategoryUnknown
    }
}
```

### Recovery Strategies

Different error types require different recovery approaches:

```go
type RecoveryStrategy string

const (
    StrategyRetry          RecoveryStrategy = "retry"
    StrategyAlternateProvider RecoveryStrategy = "alternate_provider"
    StrategyManualReview   RecoveryStrategy = "manual_review"
    StrategyNoRecovery     RecoveryStrategy = "no_recovery"
)

func determineRecoveryStrategy(category ErrorCategory) RecoveryStrategy {
    switch category {
    case CategoryNetworkError, CategoryRateLimit:
        return StrategyRetry
    case CategoryProviderDown:
        return StrategyAlternateProvider
    case CategoryFraudSuspected:
        return StrategyManualReview
    default:
        return StrategyNoRecovery
    }
}
```

### Provider Implementation

Each payment provider is wrapped in a processor:

```go
func createStripeProcessor() pipz.Chainable[Payment] {
    return pipz.Apply("stripe", func(ctx context.Context, payment Payment) (Payment, error) {
        // Simulate Stripe API call
        if payment.Amount > 50000 {
            return payment, fmt.Errorf("stripe: amount exceeds limit")
        }
        
        // Random failures for demo
        if rand.Float32() < 0.1 {
            return payment, fmt.Errorf("stripe: connection timeout")
        }
        
        payment.Status = "completed"
        payment.Metadata["transaction_id"] = fmt.Sprintf("stripe_%s_%d", 
            payment.ID, time.Now().Unix())
        return payment, nil
    })
}
```

### The Main Pipeline

```go
func CreatePaymentPipeline() pipz.Chainable[Payment] {
    return pipz.Sequential(
        // Step 1: Validate payment
        pipz.Apply("validate", validatePayment),
        
        // Step 2: Route based on amount
        pipz.Switch(
            func(ctx context.Context, p Payment) string {
                if p.Amount > 10000 {
                    return "high_value"
                }
                return "normal"
            },
            map[string]pipz.Chainable[Payment]{
                "high_value": createHighValuePipeline(),
                "normal":     createNormalPipeline(),
            },
        ),
        
        // Step 3: Always log results
        pipz.Effect("log_result", logPaymentResult),
    )
}
```

### High-Value Payment Handling

High-value payments get special treatment:

```go
func createHighValuePipeline() pipz.Chainable[Payment] {
    return pipz.Sequential(
        // Additional fraud checks
        pipz.Apply("enhanced_fraud_check", enhancedFraudCheck),
        
        // Manual approval if needed
        pipz.Mutate("require_approval",
            func(ctx context.Context, p Payment) bool {
                return p.Amount > 25000
            },
            func(ctx context.Context, p Payment) Payment {
                p.Status = "pending_approval"
                p.Metadata["approval_required"] = "true"
                return p
            },
        ),
        
        // Process with primary provider only (no automatic failover)
        pipz.WithErrorHandler(
            createStripeProcessor(),
            pipz.Effect("alert_ops", func(ctx context.Context, err error) error {
                alertOps(fmt.Sprintf("High-value payment failed: %v", err))
                return nil
            }),
        ),
    )
}
```

### Normal Payment Processing with Failover

```go
func createNormalPipeline() pipz.Chainable[Payment] {
    return pipz.Sequential(
        // Try providers in order of preference
        pipz.Fallback(
            pipz.RetryWithBackoff(
                createStripeProcessor(),
                3,
                100*time.Millisecond,
            ),
            pipz.Fallback(
                createPayPalProcessor(),
                createSquareProcessor(),
            ),
        ),
        
        // Handle any failures
        pipz.ProcessorFunc[Payment](func(ctx context.Context, p Payment) (Payment, error) {
            if p.Status != "completed" {
                return handlePaymentFailure(ctx, p)
            }
            return p, nil
        }),
    )
}
```

### Error Recovery Pipeline

```go
func CreateErrorRecoveryPipeline() pipz.Chainable[Payment] {
    return pipz.Sequential(
        // Categorize the error
        pipz.Transform("categorize_error", func(ctx context.Context, p Payment) Payment {
            if p.Error != nil {
                category := categorizeError(p.Error)
                p.Metadata["error_category"] = string(category)
                p.Metadata["recovery_strategy"] = string(determineRecoveryStrategy(category))
            }
            return p
        }),
        
        // Notify customer if needed
        pipz.Effect("notify_customer", notifyCustomerOfFailure),
        
        // Attempt recovery based on strategy
        pipz.Switch(
            func(ctx context.Context, p Payment) RecoveryStrategy {
                return RecoveryStrategy(p.Metadata["recovery_strategy"])
            },
            map[RecoveryStrategy]pipz.Chainable[Payment]{
                StrategyRetry:             createRetryPipeline(),
                StrategyAlternateProvider: createAlternateProviderPipeline(),
                StrategyManualReview:      createManualReviewPipeline(),
                StrategyNoRecovery:        pipz.Transform("mark_failed", markAsFailed),
            },
        ),
    )
}
```

## Usage

```go
func ProcessPayment(ctx context.Context, payment Payment) (*PaymentResult, error) {
    pipeline := CreatePaymentPipeline()
    
    processed, err := pipeline.Process(ctx, payment)
    if err != nil {
        // Pipeline itself failed (vs payment failure)
        return nil, fmt.Errorf("pipeline error: %w", err)
    }
    
    result := &PaymentResult{
        Payment:       processed,
        Success:       processed.Status == "completed",
        TransactionID: processed.Metadata["transaction_id"],
    }
    
    if !result.Success {
        result.Message = processed.Metadata["failure_reason"]
    }
    
    return result, nil
}
```

## Testing

The pipeline is highly testable:

```go
func TestPaymentPipeline(t *testing.T) {
    t.Run("successful payment", func(t *testing.T) {
        payment := Payment{
            ID:       "TEST-001",
            Amount:   99.99,
            Currency: "USD",
            Method:   "card",
        }
        
        result, err := ProcessPayment(context.Background(), payment)
        assert.NoError(t, err)
        assert.True(t, result.Success)
        assert.NotEmpty(t, result.TransactionID)
    })
    
    t.Run("high value payment requires approval", func(t *testing.T) {
        payment := Payment{
            ID:     "TEST-002",
            Amount: 50000,
        }
        
        result, err := ProcessPayment(context.Background(), payment)
        assert.NoError(t, err)
        assert.Equal(t, "pending_approval", result.Payment.Status)
    })
}
```

## Key Patterns

1. **Error Categorization**: Different errors need different handling
2. **Provider Health Tracking**: Monitor success rates per provider
3. **Graceful Degradation**: Fall back to alternate providers
4. **High-Value Special Handling**: Extra checks and manual approval
5. **Customer Communication**: Keep users informed of issues

## Running the Example

```bash
cd examples/payment
go test -v
go run .
```

## Next Steps

- See the [ETL Pipeline](./etl-pipeline.md) for batch processing patterns
- Check [Error Handling](../concepts/error-handling.md) for more recovery strategies
- Explore [Testing Guide](../guides/testing.md) for testing best practices