package pipz

import (
	"context"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Enrich creates a Processor that attempts to enhance data with additional information.
// Enrich is unique among processors - if the enrichment fails, it returns the original
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
//	const AddCustomerName = pipz.Name("add_customer_name")
//	addCustomerName := pipz.Enrich(AddCustomerName, func(ctx context.Context, order Order) (Order, error) {
//	    customer, err := customerService.Get(ctx, order.CustomerID)
//	    if err != nil {
//	        // Log but don't fail - order processing continues without name
//	        log.Printf("failed to enrich customer data: %v", err)
//	        return order, err
//	    }
//	    order.CustomerName = customer.Name
//	    return order, nil
//	})
func Enrich[T any](name Name, fn func(context.Context, T) (T, error)) Processor[T] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(ProcessorCallsTotal)
	metrics.Counter(ProcessorErrorsTotal)

	return Processor[T]{
		name:    name,
		metrics: metrics,
		tracer:  tracez.New(),
		fn: func(ctx context.Context, value T) (result T, err error) {
			defer recoverFromPanic(&result, &err, name, value)
			enriched, enrichErr := fn(ctx, value)
			if enrichErr != nil {
				// Continue with original data - enrichment is best-effort
				return value, nil
			}
			return enriched, nil
		},
	}
}
