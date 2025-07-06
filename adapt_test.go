package pipz

import (
	"errors"
	"strings"
	"testing"
)

// Test data types
type TestUser struct {
	ID       string
	Name     string
	Email    string
	Age      int
	Verified bool
	Score    float64
}

func TestTransformAdapter(t *testing.T) {
	t.Run("AlwaysModifies", func(t *testing.T) {
		user := TestUser{Name: "john", Email: "JOHN@EXAMPLE.COM"}
		
		processor := Transform(func(u TestUser) TestUser {
			u.Name = strings.Title(u.Name)
			u.Email = strings.ToLower(u.Email)
			return u
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Transform should always return bytes")
		}
		
		// Decode and verify
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		if decoded.Name != "John" {
			t.Errorf("expected Name 'John', got '%s'", decoded.Name)
		}
		if decoded.Email != "john@example.com" {
			t.Errorf("expected Email 'john@example.com', got '%s'", decoded.Email)
		}
	})
	
	t.Run("MultipleTransformations", func(t *testing.T) {
		user := TestUser{Age: 25, Score: 85.5}
		
		processor := Transform(func(u TestUser) TestUser {
			u.Age = u.Age + 1
			u.Score = u.Score * 1.1
			u.Verified = true
			return u
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
		
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		if decoded.Age != 26 {
			t.Errorf("expected Age 26, got %d", decoded.Age)
		}
		if !decoded.Verified {
			t.Error("expected Verified to be true")
		}
	})
}

func TestApplyAdapter(t *testing.T) {
	t.Run("SuccessfulApplication", func(t *testing.T) {
		user := TestUser{Email: "test@example.com", Age: 25}
		
		processor := Apply(func(u TestUser) (TestUser, error) {
			if u.Age < 18 {
				return u, errors.New("must be 18 or older")
			}
			u.Verified = true
			return u, nil
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Apply should return bytes on success")
		}
		
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		if !decoded.Verified {
			t.Error("expected Verified to be true")
		}
	})
	
	t.Run("FailedApplication", func(t *testing.T) {
		user := TestUser{Age: 16}
		
		processor := Apply(func(u TestUser) (TestUser, error) {
			if u.Age < 18 {
				return u, errors.New("must be 18 or older")
			}
			u.Verified = true
			return u, nil
		})
		
		result, err := processor(user)
		if err == nil {
			t.Fatal("Apply should have failed for underage user")
		}
		
		if result != nil {
			t.Error("Apply should return nil bytes on error")
		}
		
		expectedErr := "must be 18 or older"
		if err.Error() != expectedErr {
			t.Errorf("expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})
	
	t.Run("NoModification", func(t *testing.T) {
		user := TestUser{Name: "Alice", Age: 30}
		
		processor := Apply(func(u TestUser) (TestUser, error) {
			// Don't modify anything, just validate
			if u.Name == "" {
				return u, errors.New("name required")
			}
			return u, nil
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
		
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		if decoded.Name != "Alice" {
			t.Errorf("Name should be unchanged: %s", decoded.Name)
		}
	})
}

func TestValidateAdapter(t *testing.T) {
	t.Run("SuccessfulValidation", func(t *testing.T) {
		user := TestUser{Email: "test@example.com", Age: 25}
		
		processor := Validate(func(u TestUser) error {
			if u.Age < 18 {
				return errors.New("must be 18 or older")
			}
			if !strings.Contains(u.Email, "@") {
				return errors.New("invalid email")
			}
			return nil
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		
		if result != nil {
			t.Error("Validate should return nil bytes (no modification)")
		}
	})
	
	t.Run("FailedValidation", func(t *testing.T) {
		user := TestUser{Email: "invalid-email", Age: 25}
		
		processor := Validate(func(u TestUser) error {
			if !strings.Contains(u.Email, "@") {
				return errors.New("invalid email")
			}
			return nil
		})
		
		result, err := processor(user)
		if err == nil {
			t.Fatal("Validate should have failed for invalid email")
		}
		
		if result != nil {
			t.Error("Validate should return nil bytes on error")
		}
		
		expectedErr := "invalid email"
		if err.Error() != expectedErr {
			t.Errorf("expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})
	
	t.Run("MultipleValidationRules", func(t *testing.T) {
		tests := []struct {
			name    string
			user    TestUser
			wantErr string
		}{
			{
				name: "ValidUser",
				user: TestUser{Name: "Alice", Email: "alice@example.com", Age: 25},
			},
			{
				name:    "MissingName",
				user:    TestUser{Email: "alice@example.com", Age: 25},
				wantErr: "name required",
			},
			{
				name:    "InvalidEmail",
				user:    TestUser{Name: "Alice", Email: "invalid", Age: 25},
				wantErr: "invalid email",
			},
			{
				name:    "TooYoung",
				user:    TestUser{Name: "Alice", Email: "alice@example.com", Age: 16},
				wantErr: "must be 18 or older",
			},
		}
		
		processor := Validate(func(u TestUser) error {
			if u.Name == "" {
				return errors.New("name required")
			}
			if !strings.Contains(u.Email, "@") {
				return errors.New("invalid email")
			}
			if u.Age < 18 {
				return errors.New("must be 18 or older")
			}
			return nil
		})
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := processor(tt.user)
				
				if tt.wantErr != "" {
					if err == nil {
						t.Fatalf("expected error '%s', got nil", tt.wantErr)
					}
					if err.Error() != tt.wantErr {
						t.Errorf("expected error '%s', got '%s'", tt.wantErr, err.Error())
					}
					if result != nil {
						t.Error("should return nil bytes on error")
					}
				} else {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if result != nil {
						t.Error("should return nil bytes (no modification)")
					}
				}
			})
		}
	})
}

func TestMutateAdapter(t *testing.T) {
	t.Run("ConditionTrue", func(t *testing.T) {
		user := TestUser{Age: 65}
		
		processor := Mutate(
			func(u TestUser) TestUser {
				u.Score = 100.0
				return u
			},
			func(u TestUser) bool {
				return u.Age >= 65 // Senior discount condition
			},
		)
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Mutate failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Mutate should return bytes when condition is true")
		}
		
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		if decoded.Score != 100.0 {
			t.Errorf("expected Score 100.0, got %f", decoded.Score)
		}
	})
	
	t.Run("ConditionFalse", func(t *testing.T) {
		user := TestUser{Age: 25}
		
		processor := Mutate(
			func(u TestUser) TestUser {
				u.Score = 100.0
				return u
			},
			func(u TestUser) bool {
				return u.Age >= 65 // Senior discount condition
			},
		)
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Mutate failed: %v", err)
		}
		
		if result != nil {
			t.Error("Mutate should return nil bytes when condition is false")
		}
	})
	
	t.Run("ComplexConditions", func(t *testing.T) {
		tests := []struct {
			name      string
			user      TestUser
			shouldMod bool
		}{
			{
				name:      "VIPUser",
				user:      TestUser{Name: "Alice", Score: 95.0, Verified: true},
				shouldMod: true,
			},
			{
				name:      "HighScoreNotVerified",
				user:      TestUser{Name: "Bob", Score: 95.0, Verified: false},
				shouldMod: false,
			},
			{
				name:      "LowScoreVerified",
				user:      TestUser{Name: "Charlie", Score: 50.0, Verified: true},
				shouldMod: false,
			},
		}
		
		processor := Mutate(
			func(u TestUser) TestUser {
				u.ID = "VIP-" + u.Name
				return u
			},
			func(u TestUser) bool {
				return u.Score > 90.0 && u.Verified // VIP condition
			},
		)
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := processor(tt.user)
				if err != nil {
					t.Fatalf("Mutate failed: %v", err)
				}
				
				if tt.shouldMod {
					if result == nil {
						t.Fatal("expected modification, got nil bytes")
					}
					decoded, err := Decode[TestUser](result)
					if err != nil {
						t.Fatalf("Failed to decode result: %v", err)
					}
					expectedID := "VIP-" + tt.user.Name
					if decoded.ID != expectedID {
						t.Errorf("expected ID '%s', got '%s'", expectedID, decoded.ID)
					}
				} else {
					if result != nil {
						t.Error("expected no modification, got bytes")
					}
				}
			})
		}
	})
}

func TestEffectAdapter(t *testing.T) {
	t.Run("SuccessfulEffect", func(t *testing.T) {
		user := TestUser{Name: "Alice"}
		effectCalled := false
		
		processor := Effect(func(u TestUser) error {
			effectCalled = true
			// Simulate logging or metrics
			if u.Name == "" {
				return errors.New("cannot log empty name")
			}
			return nil
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Effect failed: %v", err)
		}
		
		if result != nil {
			t.Error("Effect should never return bytes")
		}
		
		if !effectCalled {
			t.Error("effect function should have been called")
		}
	})
	
	t.Run("FailedEffect", func(t *testing.T) {
		user := TestUser{Name: ""}
		
		processor := Effect(func(u TestUser) error {
			if u.Name == "" {
				return errors.New("cannot process empty name")
			}
			return nil
		})
		
		result, err := processor(user)
		if err == nil {
			t.Fatal("Effect should have failed for empty name")
		}
		
		if result != nil {
			t.Error("Effect should return nil bytes on error")
		}
		
		expectedErr := "cannot process empty name"
		if err.Error() != expectedErr {
			t.Errorf("expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})
	
	t.Run("MultipleEffects", func(t *testing.T) {
		user := TestUser{ID: "123", Name: "Alice"}
		var calls []string
		
		// Simulate multiple side effects
		processor1 := Effect(func(u TestUser) error {
			calls = append(calls, "log")
			return nil
		})
		
		processor2 := Effect(func(u TestUser) error {
			calls = append(calls, "metrics")
			return nil
		})
		
		processor3 := Effect(func(u TestUser) error {
			calls = append(calls, "notify")
			return nil
		})
		
		// Run all effects
		_, err1 := processor1(user)
		_, err2 := processor2(user)
		_, err3 := processor3(user)
		
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatal("effects should not fail")
		}
		
		expectedCalls := []string{"log", "metrics", "notify"}
		if len(calls) != len(expectedCalls) {
			t.Fatalf("expected %d calls, got %d", len(expectedCalls), len(calls))
		}
		
		for i, expected := range expectedCalls {
			if calls[i] != expected {
				t.Errorf("call %d: expected '%s', got '%s'", i, expected, calls[i])
			}
		}
	})
}

func TestEnrichAdapter(t *testing.T) {
	t.Run("SuccessfulEnrichment", func(t *testing.T) {
		user := TestUser{Name: "Alice", Email: "alice@example.com"}
		
		processor := Enrich(func(u TestUser) (TestUser, error) {
			// Simulate external API call
			u.ID = "USR-" + strings.ToUpper(u.Name)
			u.Verified = true
			return u, nil
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Enrich failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Enrich should return bytes on success")
		}
		
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		if decoded.ID != "USR-ALICE" {
			t.Errorf("expected ID 'USR-ALICE', got '%s'", decoded.ID)
		}
		if !decoded.Verified {
			t.Error("expected Verified to be true")
		}
	})
	
	t.Run("FailedEnrichment", func(t *testing.T) {
		user := TestUser{Name: "Alice"}
		
		processor := Enrich(func(u TestUser) (TestUser, error) {
			// Simulate API failure
			return u, errors.New("external API unavailable")
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Enrich should not fail when enrichment fails: %v", err)
		}
		
		if result != nil {
			t.Error("Enrich should return nil bytes when enrichment fails")
		}
	})
	
	t.Run("PartialEnrichment", func(t *testing.T) {
		user := TestUser{Name: "Alice", Age: 25}
		
		processor := Enrich(func(u TestUser) (TestUser, error) {
			// Some enrichments succeed, others fail
			if u.Age > 0 {
				u.Score = float64(u.Age) * 2.5
				return u, nil
			}
			return u, errors.New("cannot calculate score")
		})
		
		result, err := processor(user)
		if err != nil {
			t.Fatalf("Enrich failed: %v", err)
		}
		
		decoded, err := Decode[TestUser](result)
		if err != nil {
			t.Fatalf("Failed to decode result: %v", err)
		}
		
		expectedScore := 25 * 2.5
		if decoded.Score != expectedScore {
			t.Errorf("expected Score %f, got %f", expectedScore, decoded.Score)
		}
	})
	
	t.Run("BestEffortPattern", func(t *testing.T) {
		// Test multiple enrichments where some might fail
		user := TestUser{Name: "Alice", Email: "alice@example.com"}
		
		// Enrichment that always succeeds
		enrichBasic := Enrich(func(u TestUser) (TestUser, error) {
			u.ID = "basic-id"
			return u, nil
		})
		
		// Enrichment that fails
		enrichFlakey := Enrich(func(u TestUser) (TestUser, error) {
			return u, errors.New("service unavailable")
		})
		
		// Enrichment that succeeds
		enrichStable := Enrich(func(u TestUser) (TestUser, error) {
			u.Verified = true
			return u, nil
		})
		
		// Run enrichments in sequence
		result1, err1 := enrichBasic(user)
		if err1 != nil {
			t.Fatalf("basic enrichment failed: %v", err1)
		}
		
		// Continue with enriched data
		enriched1, _ := Decode[TestUser](result1)
		
		result2, err2 := enrichFlakey(enriched1)
		if err2 != nil {
			t.Fatalf("flakey enrichment should not fail: %v", err2)
		}
		if result2 != nil {
			t.Error("flakey enrichment should return nil bytes")
		}
		
		// Use original enriched data since flakey failed
		result3, err3 := enrichStable(enriched1)
		if err3 != nil {
			t.Fatalf("stable enrichment failed: %v", err3)
		}
		
		final, err := Decode[TestUser](result3)
		if err != nil {
			t.Fatalf("Failed to decode final result: %v", err)
		}
		
		// Should have basic and stable enrichments
		if final.ID != "basic-id" {
			t.Errorf("expected ID 'basic-id', got '%s'", final.ID)
		}
		if !final.Verified {
			t.Error("expected Verified to be true")
		}
	})
}

func TestAdapterChaining(t *testing.T) {
	t.Run("ValidateTransformEffect", func(t *testing.T) {
		user := TestUser{Name: "alice", Email: "ALICE@EXAMPLE.COM", Age: 25}
		
		// Chain: Validate -> Transform -> Effect
		validate := Validate(func(u TestUser) error {
			if u.Age < 18 {
				return errors.New("too young")
			}
			return nil
		})
		
		transform := Transform(func(u TestUser) TestUser {
			u.Name = strings.Title(u.Name)
			u.Email = strings.ToLower(u.Email)
			return u
		})
		
		var effectCalled bool
		effect := Effect(func(u TestUser) error {
			effectCalled = true
			return nil
		})
		
		// Step 1: Validate (no modification)
		result1, err1 := validate(user)
		if err1 != nil {
			t.Fatalf("validation failed: %v", err1)
		}
		if result1 != nil {
			t.Error("validate should not modify data")
		}
		
		// Step 2: Transform (modifies data)
		result2, err2 := transform(user)
		if err2 != nil {
			t.Fatalf("transform failed: %v", err2)
		}
		if result2 == nil {
			t.Fatal("transform should return bytes")
		}
		
		// Step 3: Effect on transformed data
		transformed, _ := Decode[TestUser](result2)
		result3, err3 := effect(transformed)
		if err3 != nil {
			t.Fatalf("effect failed: %v", err3)
		}
		if result3 != nil {
			t.Error("effect should not return bytes")
		}
		
		// Verify transformations
		if transformed.Name != "Alice" {
			t.Errorf("expected Name 'Alice', got '%s'", transformed.Name)
		}
		if transformed.Email != "alice@example.com" {
			t.Errorf("expected Email 'alice@example.com', got '%s'", transformed.Email)
		}
		if !effectCalled {
			t.Error("effect should have been called")
		}
	})
}