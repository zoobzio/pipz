package pipz

import "context"

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
func Enrich[T any](name Name, fn func(context.Context, T) (T, error)) Processor[T] {
	return Processor[T]{
		name: name,
		fn: func(ctx context.Context, value T) (T, error) {
			enriched, err := fn(ctx, value)
			if err != nil {
				// Continue with original data - enrichment is best-effort
				return value, nil
			}
			return enriched, nil
		},
	}
}
