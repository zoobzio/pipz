package pipz

import (
	"context"
	"strings"
	"testing"
)

func TestTransform(t *testing.T) {
	t.Run("Basic Transform", func(t *testing.T) {
		// Create a simple string transformer
		toUpper := Transform("to_upper", func(_ context.Context, s string) string {
			return strings.ToUpper(s)
		})

		result, err := toUpper.Process(context.Background(), "hello")
		if err != nil {
			t.Fatalf("transform should not return error: %v", err)
		}
		if result != "HELLO" {
			t.Errorf("expected HELLO, got %s", result)
		}
	})

	t.Run("Transform Never Returns Error", func(t *testing.T) {
		// Create a transformer that would panic in a normal function
		divider := Transform("divide", func(_ context.Context, n int) int {
			if n == 0 {
				return 0 // Transform can't return error, must handle internally
			}
			return 100 / n
		})

		// Even with problematic input, no error is returned
		result, err := divider.Process(context.Background(), 0)
		if err != nil {
			t.Fatalf("transform should never return error: %v", err)
		}
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("Transform With Context Check", func(t *testing.T) {
		// Transform can check context but can't return context errors
		transformer := Transform("context_aware", func(ctx context.Context, s string) string {
			select {
			case <-ctx.Done():
				return "canceled"
			default:
				return s + "_processed"
			}
		})

		result, err := transformer.Process(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "test_processed" {
			t.Errorf("expected test_processed, got %s", result)
		}
	})
}
