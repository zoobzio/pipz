package pipz

import (
	"context"
	"errors"
	"time"
)

// Apply creates a Processor from a function that transforms data and may return an error.
// Apply is the workhorse processor - use it when your transformation might fail due to
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
func Apply[T any](name Name, fn func(context.Context, T) (T, error)) Processor[T] {
	return Processor[T]{
		name: name,
		fn: func(ctx context.Context, value T) (T, *Error[T]) {
			start := time.Now()
			result, err := fn(ctx, value)
			if err != nil {
				var zero T
				return zero, &Error[T]{
					Path:      []Name{name},
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
