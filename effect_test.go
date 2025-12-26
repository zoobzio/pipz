package pipz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Effect test identity variables.
var (
	logEffect       = NewIdentity("log", "")
	validateEffect  = NewIdentity("validate", "")
	badEffectID     = NewIdentity("bad_effect", "")
	auditLogEffect  = NewIdentity("audit_log", "")
	strictLogger    = NewIdentity("strict_logger", "")
	captureIDEffect = NewIdentity("capture_id", "")
)

func TestEffect(t *testing.T) {
	t.Run("Effect Pass", func(t *testing.T) {
		// Track side effect execution
		var executed bool
		logger := Effect(logEffect, func(_ context.Context, _ string) error {
			executed = true
			return nil
		})

		result, err := logger.Process(context.Background(), "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "test" {
			t.Errorf("effect should not modify data")
		}
		if !executed {
			t.Error("effect should have executed")
		}
	})

	t.Run("Effect Fail", func(t *testing.T) {
		// Effect that fails
		validator := Effect(validateEffect, func(_ context.Context, s string) error {
			if s == "" {
				return errors.New("empty string not allowed")
			}
			return nil
		})

		_, err := validator.Process(context.Background(), "")
		if err == nil {
			t.Fatal("expected validation error")
		}

		// Check that error is wrapped
		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.InputData != "" {
			t.Errorf("expected input data to be preserved")
		}
		if len(pipzErr.Path) == 0 || pipzErr.Path[len(pipzErr.Path)-1].Name() != "validate" {
			t.Errorf("expected processor name in path")
		}
	})

	t.Run("Effect Does Not Modify", func(t *testing.T) {
		// Even if the effect tries to modify, it can't
		type User struct {
			Name string
			Age  int
		}

		badEffect := Effect(badEffectID, func(_ context.Context, u User) error {
			u.Name = "modified" // This won't affect the result
			u.Age = 100         // This won't affect the result
			return nil
		})

		original := User{Name: "alice", Age: 30}
		result, err := badEffect.Process(context.Background(), original)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "alice" || result.Age != 30 {
			t.Error("effect should not modify data")
		}
	})
}

func TestEffectLogging(t *testing.T) {
	// Example of using Effect for logging
	type LogEntry struct {
		Level   string
		Message string
	}

	var logs []LogEntry

	t.Run("Effect Success", func(t *testing.T) {
		logger := Effect(auditLogEffect, func(_ context.Context, data string) error {
			logs = append(logs, LogEntry{
				Level:   "INFO",
				Message: fmt.Sprintf("Processing: %s", data),
			})
			return nil
		})

		result, err := logger.Process(context.Background(), "important-data")
		if err != nil {
			t.Fatalf("logging should not fail: %v", err)
		}
		if result != "important-data" {
			t.Errorf("data should pass through unchanged")
		}
		if len(logs) != 1 {
			t.Errorf("expected 1 log entry, got %d", len(logs))
		}
	})

	t.Run("Effect Error", func(t *testing.T) {
		logs = nil // Reset logs

		logger := Effect(strictLogger, func(_ context.Context, data string) error {
			if strings.Contains(data, "error") {
				return errors.New("cannot log error data")
			}
			logs = append(logs, LogEntry{
				Level:   "INFO",
				Message: data,
			})
			return nil
		})

		// Success case
		_, err := logger.Process(context.Background(), "normal-data")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Error case
		_, err = logger.Process(context.Background(), "error-data")
		if err == nil {
			t.Fatal("expected error for error data")
		}
		if len(logs) != 1 {
			t.Errorf("only successful logs should be recorded")
		}
	})

	t.Run("Effect Does Not Modify", func(t *testing.T) {
		type Request struct {
			ID   string
			Data string
		}

		var capturedID string
		capturer := Effect(captureIDEffect, func(_ context.Context, req Request) error {
			capturedID = req.ID
			return nil
		})

		original := Request{ID: "123", Data: "test"}
		result, err := capturer.Process(context.Background(), original)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != original.ID || result.Data != original.Data {
			t.Error("effect should not modify request")
		}
		if capturedID != "123" {
			t.Error("effect should have captured ID")
		}
	})

	t.Run("Effect panic recovery", func(t *testing.T) {
		panicEffect := Effect(NewIdentity("panic_effect", ""), func(_ context.Context, _ string) error {
			panic("effect panic")
		})

		result, err := panicEffect.Process(context.Background(), "original")

		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.InputData != "original" {
			t.Errorf("expected input data 'original', got %q", pipzErr.InputData)
		}

		// Check that panic message is properly wrapped
		var panicErr *panicError
		if !errors.As(pipzErr.Err, &panicErr) {
			t.Fatal("expected panicError")
		}

		expectedMsg := "panic occurred: effect panic"
		if panicErr.sanitized != expectedMsg {
			t.Errorf("expected %q, got %q", expectedMsg, panicErr.sanitized)
		}
	})
}
