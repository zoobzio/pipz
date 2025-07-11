package pipz

import (
	"errors"
	"strings"
	"testing"
)

func TestAdapters(t *testing.T) {
	type TestUser struct {
		ID    string
		Name  string
		Email string
		Age   int
		Score float64
	}

	t.Run("Transform", func(t *testing.T) {
		processor := Transform(func(u TestUser) TestUser {
			u.Name = strings.ToUpper(u.Name)
			u.Email = strings.ToLower(u.Email)
			return u
		})

		input := TestUser{Name: "john doe", Email: "JOHN@EXAMPLE.COM"}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "JOHN DOE" {
			t.Errorf("expected Name 'JOHN DOE', got '%s'", result.Name)
		}
		if result.Email != "john@example.com" {
			t.Errorf("expected Email 'john@example.com', got '%s'", result.Email)
		}
	})

	t.Run("Apply_Success", func(t *testing.T) {
		processor := Apply(func(u TestUser) (TestUser, error) {
			if u.Age < 0 {
				return u, errors.New("age cannot be negative")
			}
			u.Score = float64(u.Age) * 1.5
			return u, nil
		})

		input := TestUser{Age: 20}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Score != 30.0 {
			t.Errorf("expected Score 30.0, got %f", result.Score)
		}
	})

	t.Run("Apply_Error", func(t *testing.T) {
		processor := Apply(func(u TestUser) (TestUser, error) {
			if u.Age < 0 {
				return u, errors.New("age cannot be negative")
			}
			u.Score = float64(u.Age) * 1.5
			return u, nil
		})

		input := TestUser{Age: -5}
		_, err := processor(input)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "age cannot be negative" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Validate_Success", func(t *testing.T) {
		processor := Validate(func(u TestUser) error {
			if u.Email == "" {
				return errors.New("email required")
			}
			if !strings.Contains(u.Email, "@") {
				return errors.New("invalid email")
			}
			return nil
		})

		input := TestUser{Email: "test@example.com"}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Value should be unchanged
		if result != input {
			t.Error("Validate should not modify the value")
		}
	})

	t.Run("Validate_Error", func(t *testing.T) {
		processor := Validate(func(u TestUser) error {
			if u.Email == "" {
				return errors.New("email required")
			}
			return nil
		})

		input := TestUser{Name: "test"}
		result, err := processor(input)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "email required" {
			t.Errorf("unexpected error: %v", err)
		}
		// Result should be zero value on error
		if result.Name != "" {
			t.Error("expected zero value on error")
		}
	})

	t.Run("Mutate_ConditionTrue", func(t *testing.T) {
		processor := Mutate(
			func(u TestUser) TestUser {
				u.Score *= 2
				return u
			},
			func(u TestUser) bool {
				return u.Name == "VIP"
			},
		)

		input := TestUser{Name: "VIP", Score: 100}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Score != 200 {
			t.Errorf("expected Score 200, got %f", result.Score)
		}
	})

	t.Run("Mutate_ConditionFalse", func(t *testing.T) {
		processor := Mutate(
			func(u TestUser) TestUser {
				u.Score *= 2
				return u
			},
			func(u TestUser) bool {
				return u.Name == "VIP"
			},
		)

		input := TestUser{Name: "regular", Score: 100}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Value should be unchanged
		if result.Score != 100 {
			t.Error("Mutate should not modify when condition is false")
		}
	})

	t.Run("Effect_Success", func(t *testing.T) {
		callCount := 0
		processor := Effect(func(u TestUser) error {
			callCount++
			if u.ID == "" {
				return errors.New("ID required for logging")
			}
			// Simulate logging
			return nil
		})

		input := TestUser{ID: "123", Name: "test"}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount != 1 {
			t.Errorf("expected effect to be called once, got %d", callCount)
		}
		// Value should be unchanged
		if result != input {
			t.Error("Effect should not modify the value")
		}
	})

	t.Run("Effect_Error", func(t *testing.T) {
		processor := Effect(func(u TestUser) error {
			if u.ID == "" {
				return errors.New("ID required for logging")
			}
			return nil
		})

		input := TestUser{Name: "test"}
		result, err := processor(input)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "ID required for logging" {
			t.Errorf("unexpected error: %v", err)
		}
		// Result should be zero value on error
		if result.Name != "" {
			t.Error("expected zero value on error")
		}
	})

	t.Run("Enrich_Success", func(t *testing.T) {
		processor := Enrich(func(u TestUser) (TestUser, error) {
			// Simulate fetching additional data
			u.Email = u.Name + "@example.com"
			u.Score = 100.0
			return u, nil
		})

		input := TestUser{ID: "123", Name: "john"}
		result, err := processor(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Email != "john@example.com" {
			t.Errorf("expected Email 'john@example.com', got '%s'", result.Email)
		}
		if result.Score != 100.0 {
			t.Errorf("expected Score 100.0, got %f", result.Score)
		}
	})

	t.Run("Enrich_Failure", func(t *testing.T) {
		processor := Enrich(func(u TestUser) (TestUser, error) {
			return u, errors.New("enrichment service unavailable")
		})

		input := TestUser{ID: "123", Name: "john", Score: 50}
		result, err := processor(input)

		if err != nil {
			t.Fatal("Enrich should not fail the pipeline")
		}
		// Original value should be returned on enrichment failure
		if result != input {
			t.Error("expected original value on enrichment failure")
		}
	})
}
