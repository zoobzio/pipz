package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFallback(t *testing.T) {
	t.Run("Primary Success", func(t *testing.T) {
		primary := Transform("primary", func(_ context.Context, n int) int {
			return n * 2
		})
		fallback := Transform("fallback", func(_ context.Context, n int) int {
			return n * 3
		})

		fb := NewFallback("test-fallback", primary, fallback)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 { // Should use primary (5 * 2)
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Primary Fails Uses Fallback", func(t *testing.T) {
		primary := Apply("primary", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := Transform("fallback", func(_ context.Context, n int) int {
			return n * 3
		})

		fb := NewFallback("test-fallback", primary, fallback)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 15 { // Should use fallback (5 * 3)
			t.Errorf("expected 15, got %d", result)
		}
	})

	t.Run("Both Fail", func(t *testing.T) {
		primary := Apply("primary", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("primary failed")
		})
		fallback := Apply("fallback", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("fallback failed")
		})

		fb := NewFallback("test-fallback", primary, fallback)

		_, err := fb.Process(context.Background(), 5)
		if err == nil {
			t.Fatal("expected error when both fail")
		}
		if !strings.Contains(err.Error(), "fallback failed") {
			t.Errorf("expected fallback error, got: %v", err)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		primary := Transform("noop", func(_ context.Context, n int) int { return n })
		fallback := Transform("noop", func(_ context.Context, n int) int { return n })
		fb := NewFallback("my-fallback", primary, fallback)
		if fb.Name() != "my-fallback" {
			t.Errorf("expected 'my-fallback', got %q", fb.Name())
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		primary1 := Transform("p1", func(_ context.Context, n int) int { return n })
		primary2 := Transform("p2", func(_ context.Context, n int) int { return n * 2 })
		fallback1 := Transform("f1", func(_ context.Context, n int) int { return n })
		fallback2 := Transform("f2", func(_ context.Context, n int) int { return n * 3 })

		fb := NewFallback("test", primary1, fallback1)

		// Test getters - just verify they return something (can't compare functions)
		if fb.GetPrimary() == nil {
			t.Error("GetPrimary returned nil")
		}
		if fb.GetFallback() == nil {
			t.Error("GetFallback returned nil")
		}

		// Test setters
		fb.SetPrimary(primary2)
		fb.SetFallback(fallback2)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 { // primary2: 5 * 2
			t.Errorf("expected 10 after SetPrimary, got %d", result)
		}
	})

	t.Run("Multiple Fallbacks", func(t *testing.T) {
		first := Apply("first", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("first failed")
		})
		second := Apply("second", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("second failed")
		})
		third := Transform("third", func(_ context.Context, n int) int {
			return n * 10 // This should succeed
		})
		fourth := Transform("fourth", func(_ context.Context, n int) int {
			return n * 100 // Should not be reached
		})

		fb := NewFallback("multi-fallback", first, second, third, fourth)

		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 50 { // Should use third (5 * 10)
			t.Errorf("expected 50, got %d", result)
		}
	})

	t.Run("All Multiple Fallbacks Fail", func(t *testing.T) {
		first := Apply("first", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("first failed")
		})
		second := Apply("second", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("second failed")
		})
		third := Apply("third", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("third failed")
		})

		fb := NewFallback("all-fail", first, second, third)

		result, err := fb.Process(context.Background(), 5)
		if err == nil {
			t.Fatal("expected error when all fallbacks fail")
		}

		// Should return last error with path
		if !strings.Contains(err.Error(), "third failed") {
			t.Errorf("expected error to contain 'third failed', got: %v", err)
		}
		if !strings.Contains(err.Error(), "all-fail") {
			t.Errorf("expected error path to contain 'all-fail', got: %v", err)
		}
		if result != 5 { // Should return original input
			t.Errorf("expected original input 5, got %d", result)
		}
	})

	t.Run("New API Methods", func(t *testing.T) {
		first := Transform("first", func(_ context.Context, n int) int { return n * 2 })
		second := Transform("second", func(_ context.Context, n int) int { return n * 3 })

		fb := NewFallback("test", first, second)

		// Test Len
		if fb.Len() != 2 {
			t.Errorf("expected length 2, got %d", fb.Len())
		}

		// Test GetProcessors
		processors := fb.GetProcessors()
		if len(processors) != 2 {
			t.Errorf("expected 2 processors, got %d", len(processors))
		}

		// Test AddFallback
		third := Transform("third", func(_ context.Context, n int) int { return n * 4 })
		fb.AddFallback(third)
		if fb.Len() != 3 {
			t.Errorf("expected length 3 after AddFallback, got %d", fb.Len())
		}
	})
}
