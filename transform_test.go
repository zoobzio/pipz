package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTransform(t *testing.T) {
	t.Run("Basic Transform", func(t *testing.T) {
		// Create a simple string transformer
		toUpper := Transform(NewIdentity("to_upper", ""), func(_ context.Context, s string) string {
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
		divider := Transform(NewIdentity("divide", ""), func(_ context.Context, n int) int {
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
		transformer := Transform(NewIdentity("context_aware", ""), func(ctx context.Context, s string) string {
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

	t.Run("Transform panic recovery", func(t *testing.T) {
		// Create a Transform that panics
		panicTransform := Transform(NewIdentity("panic_transform", ""), func(_ context.Context, _ string) string {
			panic("test panic in transform")
		})

		result, err := panicTransform.Process(context.Background(), "test")

		// Should get empty string as result
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}

		// Should get Error[string] with sanitized panic message
		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if len(pipzErr.Path) != 1 || pipzErr.Path[0].Name() != "panic_transform" {
			t.Errorf("expected path [panic_transform], got %v", pipzErr.Path)
		}

		if pipzErr.InputData != "test" {
			t.Errorf("expected input data 'test', got %q", pipzErr.InputData)
		}

		// Check that panic message is properly wrapped
		var panicErr *panicError
		if !errors.As(pipzErr.Err, &panicErr) {
			t.Fatal("expected panicError")
		}

		expectedMsg := "panic occurred: test panic in transform"
		if panicErr.sanitized != expectedMsg {
			t.Errorf("expected %q, got %q", expectedMsg, panicErr.sanitized)
		}
	})

	t.Run("Processor Close", func(t *testing.T) {
		proc := Transform(NewIdentity("test", ""), func(_ context.Context, s string) string { return s })

		err := proc.Close()

		if err != nil {
			t.Errorf("Close() = %v, want nil", err)
		}
	})

	t.Run("Identity String", func(t *testing.T) {
		id := NewIdentity("my-processor", "A description")

		str := id.String()

		if str != "my-processor" {
			t.Errorf("String() = %v, want %v", str, "my-processor")
		}
	})
}
