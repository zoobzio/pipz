package pipz

import "context"

// Transform creates a Processor that applies a pure transformation function to data.
// Transform is the simplest adapter - use it when your operation always succeeds
// and always modifies the data in a predictable way.
//
// The transformation function cannot fail, making Transform ideal for:
//   - Data formatting (uppercase, trimming, parsing that can't fail)
//   - Mathematical calculations that can't error
//   - Field mapping or restructuring
//   - Adding computed fields
//
// If your transformation might fail (e.g., parsing, validation), use Apply instead.
// If you need conditional transformation, use Mutate.
//
// Example:
//
//	uppercase := pipz.Transform("uppercase", func(ctx context.Context, s string) string {
//	    return strings.ToUpper(s)
//	})
func Transform[T any](name string, fn func(context.Context, T) T) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			return fn(ctx, value), nil
		},
	}
}

// Apply creates a Processor from a function that transforms data and may return an error.
// Apply is the workhorse adapter - use it when your transformation might fail due to
// validation, parsing, external API calls, or business rule violations.
//
// The function receives a context for timeout/cancellation support. Long-running
// operations should check ctx.Err() periodically. On error, the pipeline stops
// immediately and returns the error wrapped with debugging context.
//
// Apply is ideal for:
//   - Data validation with transformation
//   - API calls that return modified data
//   - Database lookups that enhance data
//   - Parsing operations that might fail
//   - Business rule enforcement
//
// For pure transformations that can't fail, use Transform for better performance.
// For operations that should continue on failure, use Enrich.
//
// Example:
//
//	parseJSON := pipz.Apply("parse_json", func(ctx context.Context, raw string) (Data, error) {
//	    var data Data
//	    if err := json.Unmarshal([]byte(raw), &data); err != nil {
//	        return Data{}, fmt.Errorf("invalid JSON: %w", err)
//	    }
//	    return data, nil
//	})
func Apply[T any](name string, fn func(context.Context, T) (T, error)) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn:   fn,
	}
}

// Mutate creates a Processor that conditionally transforms data based on a predicate.
// Mutate combines a condition check with a transformation, applying the transformer
// only when the condition returns true. When false, data passes through unchanged.
//
// This pattern is cleaner than embedding if-statements in Transform functions and
// makes the condition explicit and testable. Use Mutate for:
//   - Feature flags (transform only for enabled users)
//   - A/B testing (apply changes to test group)
//   - Conditional formatting based on data values
//   - Environment-specific transformations
//   - Business rules that apply to subset of data
//
// The condition and transformer are separate functions for better testability
// and reusability. The transformer cannot fail - use Apply with conditional
// logic if you need error handling.
//
// Example:
//
//	discountPremium := pipz.Mutate("premium_discount",
//	    func(ctx context.Context, order Order) Order {
//	        order.Total *= 0.9  // 10% discount
//	        return order
//	    },
//	    func(ctx context.Context, order Order) bool {
//	        return order.CustomerTier == "premium" && order.Total > 100
//	    },
//	)
func Mutate[T any](name string, transformer func(context.Context, T) T, condition func(context.Context, T) bool) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			if condition(ctx, value) {
				return transformer(ctx, value), nil
			}
			return value, nil
		},
	}
}

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
//	auditLog := pipz.Effect("audit_payment", func(ctx context.Context, payment Payment) error {
//	    return auditLogger.Log(ctx, "payment_processed", map[string]any{
//	        "amount": payment.Amount,
//	        "user_id": payment.UserID,
//	        "timestamp": time.Now(),
//	    })
//	})
func Effect[T any](name string, fn func(context.Context, T) error) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			if err := fn(ctx, value); err != nil {
				var zero T
				return zero, err
			}
			return value, nil
		},
	}
}

// Enrich creates a Processor that attempts to enhance data with additional information.
// Enrich is unique among adapters - if the enrichment fails, it returns the original
// data unchanged rather than stopping the pipeline. This makes it ideal for optional
// enhancements that improve data quality but aren't critical for processing.
//
// The enrichment function should fetch additional data and return an enhanced version.
// Common enrichment patterns include:
//   - Adding user details from a cache or database
//   - Geocoding addresses to add coordinates
//   - Fetching current prices or exchange rates
//   - Looking up metadata from external services
//   - Adding computed fields from external data
//
// Use Enrich when the additional data is "nice to have" but not required.
// If the enrichment is mandatory, use Apply instead. Enrich swallows errors
// to ensure pipeline continuity, so consider logging failures within the
// enrichment function for observability.
//
// Example:
//
//	addCustomerName := pipz.Enrich("add_customer_name", func(ctx context.Context, order Order) (Order, error) {
//	    customer, err := customerService.Get(ctx, order.CustomerID)
//	    if err != nil {
//	        // Log but don't fail - order processing continues without name
//	        log.Printf("failed to enrich customer data: %v", err)
//	        return order, err
//	    }
//	    order.CustomerName = customer.Name
//	    return order, nil
//	})
func Enrich[T any](name string, fn func(context.Context, T) (T, error)) Processor[T] {
	return Processor[T]{
		Name: name,
		Fn: func(ctx context.Context, value T) (T, error) {
			enriched, err := fn(ctx, value)
			if err != nil {
				// Continue with original data - enrichment is best-effort
				return value, nil
			}
			return enriched, nil
		},
	}
}
