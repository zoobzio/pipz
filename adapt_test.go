package pipz

import (
	"context"
	"errors"
	"testing"
)

func TestTransform(t *testing.T) {
	t.Run("Basic Transform", func(t *testing.T) {
		double := Transform("double", func(_ context.Context, n int) int {
			return n * 2
		})

		if double.Name != "double" {
			t.Errorf("expected name 'double', got %q", double.Name)
		}

		result, err := double.Fn(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Transform Never Returns Error", func(t *testing.T) {
		transform := Transform("identity", func(_ context.Context, s string) string {
			return s
		})

		_, err := transform.Fn(context.Background(), "test")
		if err != nil {
			t.Errorf("Transform should never return error, got: %v", err)
		}
	})

	t.Run("Transform With Context Check", func(t *testing.T) {
		transform := Transform("context_aware", func(_ context.Context, s string) string {
			// Even if context is canceled, Transform doesn't check
			return s + "_processed"
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := transform.Fn(ctx, "test")
		if err != nil {
			t.Errorf("Transform should not fail on canceled context")
		}
		if result != "test_processed" {
			t.Errorf("Transform should still process despite canceled context")
		}
	})
}

func TestApply(t *testing.T) {
	t.Run("Apply Success", func(t *testing.T) {
		parser := Apply("parse_int", func(_ context.Context, s string) (string, error) {
			if s == "" {
				return "", errors.New("empty string")
			}
			return s + "_parsed", nil
		})

		if parser.Name != "parse_int" {
			t.Errorf("expected name 'parse_int', got %q", parser.Name)
		}

		result, err := parser.Fn(context.Background(), "123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "123_parsed" {
			t.Errorf("expected '123_parsed', got %q", result)
		}
	})

	t.Run("Apply Error", func(t *testing.T) {
		parser := Apply("parse", func(_ context.Context, s string) (string, error) {
			if s == "" {
				return "", errors.New("empty string")
			}
			return s, nil
		})

		_, err := parser.Fn(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
		if err.Error() != "empty string" {
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

		processor := Apply("increment", fn)
		result, err := processor.Fn(context.Background(), 5)

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
}

func TestValidate(t *testing.T) {
	t.Run("Validate Pass", func(t *testing.T) {
		validator := Validate("check_positive", func(_ context.Context, n int) error {
			if n <= 0 {
				return errors.New("must be positive")
			}
			return nil
		})

		if validator.Name != "check_positive" {
			t.Errorf("expected name 'check_positive', got %q", validator.Name)
		}

		result, err := validator.Fn(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 5 {
			t.Errorf("Validate should return input unchanged, got %d", result)
		}
	})

	t.Run("Validate Fail", func(t *testing.T) {
		validator := Validate("check_positive", func(_ context.Context, n int) error {
			if n <= 0 {
				return errors.New("must be positive")
			}
			return nil
		})

		result, err := validator.Fn(context.Background(), -5)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if err.Error() != "must be positive" {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 0 {
			t.Errorf("Validate should return zero value on error, got %d", result)
		}
	})

	t.Run("Validate Does Not Modify", func(t *testing.T) {
		type User struct {
			Name string
			Age  int
		}

		validator := Validate("check_age", func(_ context.Context, u User) error {
			if u.Age < 18 {
				return errors.New("must be 18+")
			}
			return nil
		})

		user := User{Name: "Alice", Age: 25}
		result, err := validator.Fn(context.Background(), user)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != user {
			t.Errorf("Validate should not modify input")
		}
	})
}

func TestMutate(t *testing.T) {
	t.Run("Mutate When Condition True", func(t *testing.T) {
		mutator := Mutate("double_if_even",
			func(_ context.Context, n int) int {
				return n * 2
			},
			func(_ context.Context, n int) bool {
				return n%2 == 0
			},
		)

		if mutator.Name != "double_if_even" {
			t.Errorf("expected name 'double_if_even', got %q", mutator.Name)
		}

		// Test with even number (condition true)
		result, err := mutator.Fn(context.Background(), 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 8 {
			t.Errorf("expected 8, got %d", result)
		}
	})

	t.Run("Mutate When Condition False", func(t *testing.T) {
		mutator := Mutate("double_if_even",
			func(_ context.Context, n int) int {
				return n * 2
			},
			func(_ context.Context, n int) bool {
				return n%2 == 0
			},
		)

		// Test with odd number (condition false)
		result, err := mutator.Fn(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 5 {
			t.Errorf("expected 5 (unchanged), got %d", result)
		}
	})

	t.Run("Mutate Never Returns Error", func(t *testing.T) {
		mutator := Mutate("always_mutate",
			func(_ context.Context, s string) string {
				return s + "_mutated"
			},
			func(_ context.Context, _ string) bool {
				return true
			},
		)

		_, err := mutator.Fn(context.Background(), "test")
		if err != nil {
			t.Errorf("Mutate should never return error, got: %v", err)
		}
	})

	t.Run("Mutate Complex Condition", func(t *testing.T) {
		type User struct {
			Name  string
			Role  string
			Score int
		}

		mutator := Mutate("boost_admin_score",
			func(_ context.Context, u User) User {
				u.Score *= 2
				return u
			},
			func(_ context.Context, u User) bool {
				return u.Role == "admin" && u.Score > 50
			},
		)

		// Admin with high score - should mutate
		admin := User{Name: "Alice", Role: "admin", Score: 75}
		result, err := mutator.Fn(context.Background(), admin)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Score != 150 {
			t.Errorf("expected score 150, got %d", result.Score)
		}

		// Regular user - should not mutate
		user := User{Name: "Bob", Role: "user", Score: 75}
		result, err = mutator.Fn(context.Background(), user)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Score != 75 {
			t.Errorf("expected score unchanged (75), got %d", result.Score)
		}
	})
}

func TestEffect(t *testing.T) {
	t.Run("Effect Success", func(t *testing.T) {
		var logged []string
		logger := Effect("log", func(_ context.Context, s string) error {
			logged = append(logged, s)
			return nil
		})

		if logger.Name != "log" {
			t.Errorf("expected name 'log', got %q", logger.Name)
		}

		result, err := logger.Fn(context.Background(), "test message")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "test message" {
			t.Errorf("Effect should return input unchanged, got %q", result)
		}
		if len(logged) != 1 || logged[0] != "test message" {
			t.Errorf("Effect should have logged the message")
		}
	})

	t.Run("Effect Error", func(t *testing.T) {
		effect := Effect("validate_length", func(_ context.Context, s string) error {
			if len(s) > 10 {
				return errors.New("string too long")
			}
			return nil
		})

		result, err := effect.Fn(context.Background(), "this is a very long string")
		if err == nil {
			t.Fatal("expected error for long string")
		}
		if err.Error() != "string too long" {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "" {
			t.Errorf("Effect should return zero value on error, got %q", result)
		}
	})

	t.Run("Effect Does Not Modify", func(t *testing.T) {
		type Event struct {
			ID   string
			Type string
		}

		var events []Event
		tracker := Effect("track_event", func(_ context.Context, e Event) error {
			events = append(events, e)
			return nil
		})

		event := Event{ID: "123", Type: "click"}
		result, err := tracker.Fn(context.Background(), event)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != event {
			t.Errorf("Effect should not modify input")
		}
		if len(events) != 1 {
			t.Errorf("Effect should have tracked the event")
		}
	})
}

func TestEnrich(t *testing.T) {
	t.Run("Enrich Success", func(t *testing.T) {
		enricher := Enrich("add_timestamp", func(_ context.Context, m map[string]string) (map[string]string, error) {
			enriched := make(map[string]string)
			for k, v := range m {
				enriched[k] = v
			}
			enriched["timestamp"] = "2024-01-01"
			return enriched, nil
		})

		if enricher.Name != "add_timestamp" {
			t.Errorf("expected name 'add_timestamp', got %q", enricher.Name)
		}

		input := map[string]string{"user": "alice"}
		result, err := enricher.Fn(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["user"] != "alice" {
			t.Errorf("original data should be preserved")
		}
		if result["timestamp"] != "2024-01-01" {
			t.Errorf("enrichment should add timestamp")
		}
	})

	t.Run("Enrich Failure Returns Original", func(t *testing.T) {
		enricher := Enrich("fetch_user_data", func(_ context.Context, userID string) (string, error) {
			if userID == "" {
				return "", errors.New("empty user ID")
			}
			return userID + "_enriched", nil
		})

		// Should return original on error
		result, err := enricher.Fn(context.Background(), "")
		if err != nil {
			t.Fatal("Enrich should not return error, even on enrichment failure")
		}
		if result != "" {
			t.Errorf("Enrich should return original value on failure, got %q", result)
		}
	})

	t.Run("Enrich Best Effort", func(t *testing.T) {
		type User struct {
			ID    string
			Name  string
			Email string
		}

		callCount := 0
		enricher := Enrich("fetch_email", func(_ context.Context, u User) (User, error) {
			callCount++
			if u.ID == "404" {
				return User{}, errors.New("user not found")
			}
			u.Email = u.ID + "@example.com"
			return u, nil
		})

		// Success case
		user1 := User{ID: "123", Name: "Alice"}
		result, err := enricher.Fn(context.Background(), user1)
		if err != nil {
			t.Fatal("unexpected error")
		}
		if result.Email != "123@example.com" {
			t.Errorf("expected enriched email")
		}

		// Failure case - should return original
		user2 := User{ID: "404", Name: "Bob"}
		result, err = enricher.Fn(context.Background(), user2)
		if err != nil {
			t.Fatal("Enrich should not propagate enrichment errors")
		}
		if result != user2 {
			t.Errorf("should return original user on enrichment failure")
		}

		if callCount != 2 {
			t.Errorf("enrichment function should be called for each invocation")
		}
	})
}

type contextKey string

func TestContextUsage(t *testing.T) {
	// Test that all adapters properly pass context to their functions
	t.Run("All Adapters Pass Context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), contextKey("test-key"), "test-value")

		// Transform
		transform := Transform("test", func(ctx context.Context, s string) string {
			if ctx.Value(contextKey("test-key")) != "test-value" {
				t.Error("Transform: context not passed correctly")
			}
			return s
		})
		_, err := transform.Fn(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Apply
		apply := Apply("test", func(ctx context.Context, s string) (string, error) {
			if ctx.Value(contextKey("test-key")) != "test-value" {
				t.Error("Apply: context not passed correctly")
			}
			return s, nil
		})
		_, err = apply.Fn(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Validate
		validate := Validate("test", func(ctx context.Context, _ string) error {
			if ctx.Value(contextKey("test-key")) != "test-value" {
				t.Error("Validate: context not passed correctly")
			}
			return nil
		})
		_, err = validate.Fn(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Mutate
		mutate := Mutate("test",
			func(ctx context.Context, s string) string {
				if ctx.Value(contextKey("test-key")) != "test-value" {
					t.Error("Mutate transformer: context not passed correctly")
				}
				return s
			},
			func(ctx context.Context, _ string) bool {
				if ctx.Value(contextKey("test-key")) != "test-value" {
					t.Error("Mutate condition: context not passed correctly")
				}
				return true
			},
		)
		_, err = mutate.Fn(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Effect
		effect := Effect("test", func(ctx context.Context, _ string) error {
			if ctx.Value(contextKey("test-key")) != "test-value" {
				t.Error("Effect: context not passed correctly")
			}
			return nil
		})
		_, err = effect.Fn(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Enrich
		enrich := Enrich("test", func(ctx context.Context, s string) (string, error) {
			if ctx.Value(contextKey("test-key")) != "test-value" {
				t.Error("Enrich: context not passed correctly")
			}
			return s, nil
		})
		_, err = enrich.Fn(ctx, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
