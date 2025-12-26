package pipz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	t.Run("Apply Success", func(t *testing.T) {
		// Create a parser that can fail
		parser := Apply(NewIdentity("parse_int", ""), func(_ context.Context, s string) (string, error) {
			if s == "" {
				return "", errors.New("empty string")
			}
			return s + "_parsed", nil
		})

		if parser.Identity().Name() != "parse_int" {
			t.Errorf("expected name 'parse_int', got %q", parser.Identity().Name())
		}

		result, err := parser.Process(context.Background(), "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "123_parsed" {
			t.Errorf("expected '123_parsed', got %q", result)
		}
	})

	t.Run("Apply Error", func(t *testing.T) {
		// Create a parser that will fail
		parser := Apply(NewIdentity("parse", ""), func(_ context.Context, s string) (string, error) {
			if s == "" {
				return "", errors.New("empty string")
			}
			return s, nil
		})

		_, err := parser.Process(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty string")
		}

		// Check that it's wrapped in pipz.Error
		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}
		if !strings.Contains(pipzErr.Err.Error(), "empty string") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Apply Direct Pass-Through", func(t *testing.T) {
		// Apply should directly use the provided function
		callCount := 0
		fn := func(_ context.Context, n int) (int, error) {
			callCount++
			return n + 1, nil
		}

		processor := Apply(NewIdentity("increment", ""), fn)
		result, err := processor.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 6 {
			t.Errorf("expected 6, got %d", result)
		}
		if callCount != 1 {
			t.Errorf("expected function to be called once, called %d times", callCount)
		}
	})

	t.Run("Apply With Numbers", func(t *testing.T) {
		// Apply with integer type
		doubleIfPositive := Apply(NewIdentity("double_if_positive", ""), func(_ context.Context, n int) (int, error) {
			if n < 0 {
				return 0, errors.New("negative number")
			}
			return n * 2, nil
		})

		result, err := doubleIfPositive.Process(context.Background(), 21)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}

		// Test error case
		_, err = doubleIfPositive.Process(context.Background(), -5)
		if err == nil {
			t.Fatal("expected error for negative input")
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}
		if pipzErr.InputData != -5 {
			t.Error("expected input data to be preserved")
		}
	})

	t.Run("Apply With Validation", func(t *testing.T) {
		validator := Apply(NewIdentity("validate_length", ""), func(_ context.Context, s string) (string, error) {
			if len(s) < 3 {
				return "", fmt.Errorf("string too short: %d chars", len(s))
			}
			return s, nil
		})

		// Valid input passes through unchanged
		result, err := validator.Process(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "hello" {
			t.Errorf("expected unchanged value")
		}

		// Invalid input fails
		_, err = validator.Process(context.Background(), "hi")
		if err == nil {
			t.Fatal("expected validation error")
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}
		if !strings.Contains(pipzErr.Err.Error(), "string too short") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Apply panic recovery", func(t *testing.T) {
		// Create an Apply that panics
		panicApply := Apply(NewIdentity("panic_apply", ""), func(_ context.Context, n int) (int, error) {
			if n == 42 {
				panic("the answer panics")
			}
			return n * 2, nil
		})

		// Test normal operation first
		result, err := panicApply.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// Test panic recovery
		result, err = panicApply.Process(context.Background(), 42)
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}

		// Check that panic message is properly wrapped
		var panicErr *panicError
		if !errors.As(pipzErr.Err, &panicErr) {
			t.Fatal("expected panicError")
		}

		expectedMsg := "panic occurred: the answer panics"
		if panicErr.sanitized != expectedMsg {
			t.Errorf("expected %q, got %q", expectedMsg, panicErr.sanitized)
		}
	})
}
