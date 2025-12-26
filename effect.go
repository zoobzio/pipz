package pipz

import (
	"context"
	"errors"
	"time"
)

// Effect creates a Processor that performs side effects without modifying the data.
// Effect is for operations that need to happen alongside your main processing flow,
// such as logging, metrics collection, notifications, or audit trails.
//
// The function receives the data for inspection but must not modify it. Any returned
// error stops the pipeline immediately. The original data always passes through
// unchanged, making Effect perfect for:
//   - Logging important events or data states
//   - Recording metrics (counts, latencies, values)
//   - Sending notifications or alerts
//   - Writing audit logs for compliance
//   - Triggering external systems
//   - Validating without transformation
//
// Unlike Apply, Effect cannot transform data. Unlike Transform, it can fail.
// This separation ensures side effects are explicit and testable.
//
// Example:
//
//	var AuditPaymentID = pipz.NewIdentity("audit-payment", "Logs payment to audit trail")
//	auditLog := pipz.Effect(AuditPaymentID, func(ctx context.Context, payment Payment) error {
//	    return auditLogger.Log(ctx, "payment_processed", map[string]any{
//	        "amount": payment.Amount,
//	        "user_id": payment.UserID,
//	        "timestamp": time.Now(),
//	    })
//	})
func Effect[T any](identity Identity, fn func(context.Context, T) error) Processor[T] {
	return Processor[T]{
		identity: identity,
		fn: func(ctx context.Context, value T) (result T, err error) {
			defer recoverFromPanic(&result, &err, identity, value)
			start := time.Now()
			result = value // Effect always returns original value
			if err = fn(ctx, value); err != nil {
				var zero T
				return zero, &Error[T]{
					Path:      []Identity{identity},
					InputData: value,
					Err:       err,
					Timestamp: time.Now(),
					Duration:  time.Since(start),
					Timeout:   errors.Is(err, context.DeadlineExceeded),
					Canceled:  errors.Is(err, context.Canceled),
				}
			}
			return result, nil
		},
	}
}
