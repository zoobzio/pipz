# Handle

Provides error observation and handling capabilities for processors.

## Function Signature

```go
func NewHandle[T any](
    name Name,
    processor Chainable[T],
    errorHandler Chainable[*Error[T]],
) *Handle[T]
```

## Parameters

- `name` (`Name`) - Identifier for the connector used in debugging
- `processor` - Main processor that might fail
- `errorHandler` - Pipeline that processes errors (receives `*Error[T]`)

## Returns

Returns a `*Handle[T]` that implements `Chainable[T]`.

## Behavior

- **Error observation** - Handler processes errors for side effects (logging, cleanup)
- **Error pass-through** - Original errors always propagate after handling
- **Error as data** - Errors flow through the error handler pipeline
- **Handler errors ignored** - Handler failures don't affect error propagation
- **Success pass-through** - Successful results bypass error handler

## Key Insight

Handle provides error observation and cleanup. By wrapping a processor with Handle, you're saying "when this fails, I need to do something about it" - whether that's logging, cleanup, notifications, or compensation. The error always propagates after handling.

## Example

```go
// Log errors with context
logged := pipz.NewHandle("order-logging",
    processOrder,
    pipz.Effect("log", func(ctx context.Context, err *pipz.Error[Order]) error {
        log.Printf("Order %s failed at %s: %v", 
            err.InputData.ID, err.Path, err.Err)
        metrics.Increment("order.failures")
        return nil
    }),
)

// Clean up resources on failure
withCleanup := pipz.NewHandle("inventory-management",
    pipz.NewSequence[Order](
        reserveInventory,
        chargePayment,
        confirmOrder,
    ),
    pipz.Effect("cleanup", func(ctx context.Context, err *pipz.Error[Order]) error {
        if err.InputData.ReservationID != "" {
            log.Printf("Releasing inventory for failed order %s", err.InputData.ID)
            inventory.Release(err.InputData.ReservationID)
        }
        return nil
    }),
)

// Send notifications on failure
notifying := pipz.NewHandle("payment-alerts",
    processPayment,
    pipz.Effect("notify", func(ctx context.Context, err *pipz.Error[Payment]) error {
        if err.InputData.Amount > 10000 {
            // Alert on large payment failures
            alerting.SendHighValuePaymentFailure(err.InputData, err.Err)
        }
        return nil
    }),
)
```

## When to Use

Use `Handle` when:
- You need to perform cleanup on failure (release resources)
- Errors require logging with additional context
- You want to send notifications or alerts on failure
- You need to implement compensation logic
- You want to collect metrics about failures
- Different errors require different side effects

## When NOT to Use

Don't use `Handle` when:
- You just need to suppress errors (use `Fallback` with identity function)
- Simple retry is sufficient (use `Retry`)
- You want to transform errors into values (use `Fallback`)
- No cleanup or side effects are needed on failure

## Error Handler Access

The error handler receives `*Error[T]` with full context:

```go
type Error[T any] struct {
    Path      []Name        // Full path through processors
    Err       error         // Original error
    InputData T             // Input when error occurred
    Timeout   bool          // Was this a timeout?
    Canceled  bool          // Was this cancelled?
    Timestamp time.Time     // When the error occurred
    Duration  time.Duration // Processing time before error
}
```

## Common Patterns

```go
// Resource cleanup pattern
inventoryCleanup := pipz.NewHandle("order-with-cleanup",
    pipz.NewSequence[Order](
        validateOrder,
        reserveInventory,
        chargePayment,
        shipOrder,
    ),
    pipz.Effect("release-inventory", func(ctx context.Context, err *pipz.Error[Order]) error {
        if reservation := err.InputData.ReservationID; reservation != "" {
            log.Printf("Releasing inventory reservation %s after failure", reservation)
            if releaseErr := inventory.Release(reservation); releaseErr != nil {
                log.Printf("Failed to release inventory: %v", releaseErr)
            }
        }
        return nil
    }),
)

// Monitoring and alerting
monitoredPayment := pipz.NewHandle("payment-monitoring",
    processPayment,
    pipz.Effect("monitor", func(ctx context.Context, err *pipz.Error[Payment]) error {
        metrics.RecordPaymentFailure(err.InputData.Method, err.Err)
        
        if err.InputData.Amount > alertThreshold {
            alerting.NotifyHighValueFailure(err.InputData, err.Err)
        }
        
        if err.Timeout {
            log.Printf("Payment timeout after %v", err.Duration)
            metrics.RecordTimeout("payment", err.Duration)
        }
        
        return nil
    }),
)

// Compensation pattern
compensatingTransaction := pipz.NewHandle("transfer-with-compensation",
    pipz.NewSequence[Transfer](
        debitSource,
        creditDestination,
        recordTransaction,
    ),
    pipz.Effect("compensate", func(ctx context.Context, err *pipz.Error[Transfer]) error {
        // Determine how far we got
        failedAt := err.Path[len(err.Path)-1]
        
        switch failedAt {
        case "recordTransaction":
            // Both debit and credit succeeded, just logging failed
            log.Printf("Transaction completed but not recorded: %v", err.InputData)
            // Try to record in backup system
            backupLog.Record(err.InputData)
            
        case "creditDestination":
            // Debit succeeded but credit failed - must reverse
            log.Printf("Reversing debit due to credit failure")
            if reverseErr := reverseDebit(err.InputData); reverseErr != nil {
                // Critical - manual intervention needed
                alerting.CriticalAlert("Failed to reverse debit", err.InputData, reverseErr)
            }
        }
        
        return nil
    }),
)
```

## Advanced Error Flows

```go
// Parallel error handling
parallelRecovery := pipz.NewHandle("parallel-recovery",
    mainProcess,
    pipz.NewConcurrent[*pipz.Error[Data]](
        pipz.Effect("log", logToCentralSystem),
        pipz.Effect("metrics", updateDashboard),
        pipz.Effect("backup", saveFailedRequest),
        pipz.Apply("notify", notifyOnCallTeam),
    ),
)

// Nested error handling
nestedHandling := pipz.NewHandle("outer",
    pipz.NewHandle("inner",
        riskyOperation,
        pipz.Effect("inner-log", logInnerError),
    ),
    pipz.Effect("outer-log", func(ctx context.Context, err *pipz.Error[Data]) error {
        // This catches errors from both riskyOperation and inner-log
        log.Printf("Outer handler: %v", err)
        return nil
    }),
)

// Error aggregation for batch processing
batchErrors := pipz.NewHandle("batch",
    batchProcessor,
    pipz.Apply("collect", func(ctx context.Context, err *pipz.Error[Batch]) (*pipz.Error[Batch], error) {
        errorCollector.Add(err)
        if errorCollector.Count() > errorThreshold {
            // Trigger batch error recovery
            triggerBatchRecovery(errorCollector.GetAll())
        }
        return err, nil
    }),
)
```

## Best Practices

```go
// Clear separation of concerns
// GOOD: Handle for cleanup, Fallback for recovery
goodPattern := pipz.NewFallback("with-recovery",
    pipz.NewHandle("with-cleanup",
        riskyOperation,
        pipz.Effect("cleanup", func(ctx context.Context, err *pipz.Error[Data]) error {
            // Clean up resources
            cleanup(err.InputData)
            return nil
        }),
    ),
    fallbackOperation,  // This provides the recovery
)

// Resource management
// GOOD: Always clean up acquired resources
fileProcessor := pipz.NewHandle("file-processing",
    pipz.Apply("process", func(ctx context.Context, path string) (Result, error) {
        file, err := os.Open(path)
        if err != nil {
            return Result{}, err
        }
        defer file.Close()
        // ... processing ...
    }),
    pipz.Effect("cleanup-temp", func(ctx context.Context, err *pipz.Error[string]) error {
        // Clean up any temporary files created
        tempPath := filepath.Join(os.TempDir(), filepath.Base(err.InputData))
        os.Remove(tempPath)
        return nil
    }),
)

// Comprehensive monitoring
// GOOD: Collect all relevant metrics
monitoredService := pipz.NewHandle("monitored",
    externalService,
    pipz.Effect("metrics", func(ctx context.Context, err *pipz.Error[Request]) error {
        labels := map[string]string{
            "service": "external",
            "method":  err.InputData.Method,
            "error":   errorType(err.Err),
        }
        
        metrics.RecordError(labels)
        metrics.RecordLatency(err.Duration, labels)
        
        if err.Timeout {
            metrics.RecordTimeout(labels)
        }
        
        return nil
    }),
)
```

## See Also

- [Fallback](./fallback.md) - For simple primary/backup patterns
- [Retry](./retry.md) - For retry logic
- [Switch](./switch.md) - Often used within error handlers
- [Concurrent](./concurrent.md) - For parallel error handling