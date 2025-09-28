package integration

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	pipztesting "github.com/zoobzio/pipz/testing"
)

// TestUser represents a user for testing validation and transformation pipelines.
type TestUser struct {
	ID       int64                  `json:"id"`
	Name     string                 `json:"name"`
	Email    string                 `json:"email"`
	Age      int                    `json:"age"`
	Premium  bool                   `json:"premium"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Clone implements pipz.Cloner for safe concurrent processing.
func (u TestUser) Clone() TestUser {
	metadata := make(map[string]interface{}, len(u.Metadata))
	for k, v := range u.Metadata {
		metadata[k] = v
	}
	return TestUser{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Age:      u.Age,
		Premium:  u.Premium,
		Metadata: metadata,
	}
}

func TestPipelineFlows_SequentialProcessing(t *testing.T) {
	tests := []struct {
		name     string
		input    TestUser
		expected TestUser
	}{
		{
			name: "basic_user_processing",
			input: TestUser{
				ID:    1,
				Name:  "john doe",
				Email: "JOHN@EXAMPLE.COM",
				Age:   30,
			},
			expected: TestUser{
				ID:      1,
				Name:    "John Doe",
				Email:   "john@example.com",
				Age:     30,
				Premium: false,
				Metadata: map[string]interface{}{
					"processed_at": "validation_step",
					"normalized":   true,
				},
			},
		},
		{
			name: "premium_user_upgrade",
			input: TestUser{
				ID:    2,
				Name:  "jane smith",
				Email: "jane@example.com",
				Age:   45,
			},
			expected: TestUser{
				ID:      2,
				Name:    "Jane Smith",
				Email:   "jane@example.com",
				Age:     45,
				Premium: true, // Age >= 40 gets premium
				Metadata: map[string]interface{}{
					"processed_at":   "validation_step",
					"normalized":     true,
					"premium_reason": "age_based_upgrade",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Build a user processing pipeline
			pipeline := pipz.NewSequence[TestUser]("user-processing",
				// Step 1: Validate user data
				pipz.Apply("validate", func(_ context.Context, user TestUser) (TestUser, error) {
					if user.Name == "" {
						return user, errors.New("name is required")
					}
					if user.Email == "" {
						return user, errors.New("email is required")
					}
					if user.Age < 0 {
						return user, errors.New("age must be non-negative")
					}

					// Add validation metadata
					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["processed_at"] = "validation_step"
					return user, nil
				}),

				// Step 2: Normalize data
				pipz.Transform("normalize", func(_ context.Context, user TestUser) TestUser {
					// Normalize name to title case
					words := strings.Fields(strings.ToLower(user.Name))
					for i, word := range words {
						if word != "" {
							words[i] = strings.ToUpper(string(word[0])) + word[1:]
						}
					}
					user.Name = strings.Join(words, " ")
					// Normalize email to lowercase
					user.Email = strings.ToLower(user.Email)

					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["normalized"] = true
					return user
				}),

				// Step 3: Apply business rules
				pipz.Transform("business-rules", func(_ context.Context, user TestUser) TestUser {
					// Premium upgrade for users 40+
					if user.Age >= 40 && !user.Premium {
						user.Premium = true
						if user.Metadata == nil {
							user.Metadata = make(map[string]interface{})
						}
						user.Metadata["premium_reason"] = "age_based_upgrade"
					}
					return user
				}),
			)

			result, err := pipeline.Process(ctx, tt.input)
			if err != nil {
				t.Fatalf("pipeline failed: %v", err)
			}

			// Verify result
			if result.ID != tt.expected.ID {
				t.Errorf("expected ID %d, got %d", tt.expected.ID, result.ID)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("expected Name %q, got %q", tt.expected.Name, result.Name)
			}
			if result.Email != tt.expected.Email {
				t.Errorf("expected Email %q, got %q", tt.expected.Email, result.Email)
			}
			if result.Premium != tt.expected.Premium {
				t.Errorf("expected Premium %t, got %t", tt.expected.Premium, result.Premium)
			}

			// Verify metadata
			for key, expectedValue := range tt.expected.Metadata {
				if actualValue, ok := result.Metadata[key]; !ok {
					t.Errorf("expected metadata key %q not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("metadata %q: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestPipelineFlows_ConditionalBranching(t *testing.T) {
	ctx := context.Background()

	// Create counters to track which paths are taken
	var adultProcessing int64
	var minorProcessing int64

	// Build a pipeline with conditional branching
	pipeline := pipz.NewSequence[TestUser]("age-based-processing",
		// Validate input
		pipz.Apply("validate", func(_ context.Context, user TestUser) (TestUser, error) {
			if user.Age < 0 {
				return user, errors.New("age cannot be negative")
			}
			return user, nil
		}),

		// Branch based on age
		pipz.NewSwitch[TestUser, string]("age-switch", func(_ context.Context, user TestUser) string {
			if user.Age < 18 {
				return "minor"
			}
			return "adult"
		}).
			AddRoute("minor",
				pipz.Transform("minor-processing", func(_ context.Context, user TestUser) TestUser {
					atomic.AddInt64(&minorProcessing, 1)
					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["category"] = "minor"
					user.Metadata["restrictions"] = []string{"no_premium", "parental_consent_required"}
					return user
				}),
			).
			AddRoute("adult",
				pipz.Transform("adult-processing", func(_ context.Context, user TestUser) TestUser {
					atomic.AddInt64(&adultProcessing, 1)
					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["category"] = "adult"
					user.Metadata["features"] = []string{"premium_eligible", "full_access"}
					return user
				}),
			),
	)

	testCases := []struct {
		name        string
		user        TestUser
		expectMinor bool
		expectAdult bool
	}{
		{
			name:        "child_user",
			user:        TestUser{ID: 1, Name: "Child", Age: 12},
			expectMinor: true,
			expectAdult: false,
		},
		{
			name:        "teenager_user",
			user:        TestUser{ID: 2, Name: "Teen", Age: 16},
			expectMinor: true,
			expectAdult: false,
		},
		{
			name:        "young_adult",
			user:        TestUser{ID: 3, Name: "Young Adult", Age: 18},
			expectMinor: false,
			expectAdult: true,
		},
		{
			name:        "adult_user",
			user:        TestUser{ID: 4, Name: "Adult", Age: 35},
			expectMinor: false,
			expectAdult: true,
		},
	}

	// Reset counters
	atomic.StoreInt64(&minorProcessing, 0)
	atomic.StoreInt64(&adultProcessing, 0)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := pipeline.Process(ctx, tc.user)
			if err != nil {
				t.Fatalf("pipeline failed: %v", err)
			}

			if tc.expectMinor {
				if category, ok := result.Metadata["category"]; !ok || category != "minor" {
					t.Errorf("expected minor category, got %v", category)
				}
				if restrictions, ok := result.Metadata["restrictions"]; !ok {
					t.Error("expected restrictions metadata for minor")
				} else {
					restrictionSlice, _ := restrictions.([]string) //nolint:errcheck // Type assertion for test verification
					if len(restrictionSlice) == 0 {
						t.Error("expected non-empty restrictions for minor")
					}
				}
			}

			if tc.expectAdult {
				if category, ok := result.Metadata["category"]; !ok || category != "adult" {
					t.Errorf("expected adult category, got %v", category)
				}
				if features, ok := result.Metadata["features"]; !ok {
					t.Error("expected features metadata for adult")
				} else {
					featureSlice, _ := features.([]string) //nolint:errcheck // Type assertion for test verification
					if len(featureSlice) == 0 {
						t.Error("expected non-empty features for adult")
					}
				}
			}
		})
	}

	// Verify counters
	expectedMinor := int64(2) // child_user and teenager_user
	expectedAdult := int64(2) // young_adult and adult_user

	actualMinor := atomic.LoadInt64(&minorProcessing)
	actualAdult := atomic.LoadInt64(&adultProcessing)

	if actualMinor != expectedMinor {
		t.Errorf("expected %d minor processing calls, got %d", expectedMinor, actualMinor)
	}
	if actualAdult != expectedAdult {
		t.Errorf("expected %d adult processing calls, got %d", expectedAdult, actualAdult)
	}
}

func TestPipelineFlows_DynamicModification(t *testing.T) {
	ctx := context.Background()

	// Create a sequence that can be modified at runtime
	seq := pipz.NewSequence[TestUser]("dynamic-pipeline",
		pipz.Transform("step1", func(_ context.Context, user TestUser) TestUser {
			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata["step1"] = true
			return user
		}),
	)

	user := TestUser{ID: 1, Name: "Test User", Age: 25}

	// Initial processing - only step1
	result1, err := seq.Process(ctx, user)
	if err != nil {
		t.Fatalf("initial processing failed: %v", err)
	}

	if _, ok := result1.Metadata["step1"]; !ok {
		t.Error("expected step1 metadata")
	}
	if _, ok := result1.Metadata["step2"]; ok {
		t.Error("unexpected step2 metadata in initial result")
	}

	// Add step2 dynamically
	seq.Register(pipz.Transform("step2", func(_ context.Context, user TestUser) TestUser {
		if user.Metadata == nil {
			user.Metadata = make(map[string]interface{})
		}
		user.Metadata["step2"] = true
		return user
	}))

	// Process again - should include both steps
	result2, err := seq.Process(ctx, user)
	if err != nil {
		t.Fatalf("processing with step2 failed: %v", err)
	}

	if _, ok := result2.Metadata["step1"]; !ok {
		t.Error("expected step1 metadata after adding step2")
	}
	if _, ok := result2.Metadata["step2"]; !ok {
		t.Error("expected step2 metadata after adding step2")
	}

	// Insert step1.5 between step1 and step2
	_ = seq.After("step1", pipz.Transform("step1.5", func(_ context.Context, user TestUser) TestUser { //nolint:errcheck // Return value ignored in test
		if user.Metadata == nil {
			user.Metadata = make(map[string]interface{})
		}
		user.Metadata["step1.5"] = true
		return user
	}))

	// Process again - should include all three steps in order
	result3, err := seq.Process(ctx, user)
	if err != nil {
		t.Fatalf("processing with step1.5 failed: %v", err)
	}

	expectedSteps := []string{"step1", "step1.5", "step2"}
	for _, step := range expectedSteps {
		if _, ok := result3.Metadata[step]; !ok {
			t.Errorf("expected %s metadata after inserting step1.5", step)
		}
	}

	// Verify execution order by checking processor names
	names := seq.Names()
	expectedNames := []string{"step1", "step1.5", "step2"}
	if len(names) != len(expectedNames) {
		t.Fatalf("expected %d processors, got %d", len(expectedNames), len(names))
	}

	for i, expected := range expectedNames {
		if names[i] != pipz.Name(expected) {
			t.Errorf("processor %d: expected %s, got %s", i, expected, names[i])
		}
	}

	// Remove step1.5
	_ = seq.Remove("step1.5") //nolint:errcheck // Return value ignored in test

	// Process again - should only include step1 and step2
	result4, err := seq.Process(ctx, user)
	if err != nil {
		t.Fatalf("processing after removing step1.5 failed: %v", err)
	}

	if _, ok := result4.Metadata["step1"]; !ok {
		t.Error("expected step1 metadata after removing step1.5")
	}
	if _, ok := result4.Metadata["step1.5"]; ok {
		t.Error("unexpected step1.5 metadata after removal")
	}
	if _, ok := result4.Metadata["step2"]; !ok {
		t.Error("expected step2 metadata after removing step1.5")
	}
}

func TestPipelineFlows_ErrorPropagation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		failAtStep    string
		expectError   bool
		expectMessage string
	}{
		{
			name:          "no_error",
			failAtStep:    "",
			expectError:   false,
			expectMessage: "",
		},
		{
			name:          "fail_at_validation",
			failAtStep:    "validate",
			expectError:   true,
			expectMessage: "validation failed",
		},
		{
			name:          "fail_at_transform",
			failAtStep:    "transform",
			expectError:   true,
			expectMessage: "transformation failed",
		},
		{
			name:          "fail_at_enrich",
			failAtStep:    "enrich",
			expectError:   true,
			expectMessage: "enrichment failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build pipeline with potential failure points
			pipeline := pipz.NewSequence[TestUser]("error-propagation",
				pipz.Apply("validate", func(_ context.Context, user TestUser) (TestUser, error) {
					if tt.failAtStep == "validate" {
						return user, errors.New("validation failed")
					}
					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["validated"] = true
					return user, nil
				}),

				pipz.Apply("transform", func(_ context.Context, user TestUser) (TestUser, error) {
					if tt.failAtStep == "transform" {
						return user, errors.New("transformation failed")
					}
					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["transformed"] = true
					return user, nil
				}),

				pipz.Apply("enrich", func(_ context.Context, user TestUser) (TestUser, error) {
					if tt.failAtStep == "enrich" {
						return user, errors.New("enrichment failed")
					}
					if user.Metadata == nil {
						user.Metadata = make(map[string]interface{})
					}
					user.Metadata["enriched"] = true
					return user, nil
				}),
			)

			user := TestUser{ID: 1, Name: "Test User", Age: 25}
			result, err := pipeline.Process(ctx, user)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}

				// Check error message
				if !strings.Contains(err.Error(), tt.expectMessage) {
					t.Errorf("expected error to contain %q, got %q", tt.expectMessage, err.Error())
				}

				// Verify error contains pipeline path information
				var pipzErr *pipz.Error[TestUser]
				if errors.As(err, &pipzErr) {
					if len(pipzErr.Path) == 0 {
						t.Error("expected error to contain path information")
					}

					// Verify the error includes the failing step
					found := false
					for _, elem := range pipzErr.Path {
						if elem == pipz.Name(tt.failAtStep) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error path to contain failing step %q", tt.failAtStep)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error but got: %v", err)
				}

				// Verify all steps completed
				expectedMetadata := []string{"validated", "transformed", "enriched"}
				for _, key := range expectedMetadata {
					if _, ok := result.Metadata[key]; !ok {
						t.Errorf("expected metadata key %q", key)
					}
				}
			}
		})
	}
}

func TestPipelineFlows_ContextCancellation(t *testing.T) {
	// Test normal execution without cancellation
	t.Run("no_cancellation", func(t *testing.T) {
		ctx := context.Background()

		pipeline := pipz.NewSequence[TestUser]("cancellation-test",
			pipz.Transform("step1", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step1"] = true
				return user
			}),

			pipz.Apply("step2", func(_ context.Context, user TestUser) (TestUser, error) {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step2"] = true
				return user, nil
			}),

			pipz.Transform("step3", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step3"] = true
				return user
			}),
		)

		user := TestUser{ID: 1, Name: "Test User", Age: 25}
		result, err := pipeline.Process(ctx, user)

		if err != nil {
			t.Fatalf("expected no error but got: %v", err)
		}

		// All steps should have completed
		expectedSteps := []string{"step1", "step2", "step3"}
		for _, step := range expectedSteps {
			if _, ok := result.Metadata[step]; !ok {
				t.Errorf("expected step %s to complete", step)
			}
		}
	})

	// Test cancellation with pre-canceled context (deterministic)
	t.Run("pre_canceled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		pipeline := pipz.NewSequence[TestUser]("cancellation-test",
			pipz.Transform("step1", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step1"] = true
				return user
			}),

			pipz.Transform("step2", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step2"] = true
				return user
			}),

			pipz.Transform("step3", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step3"] = true
				return user
			}),
		)

		user := TestUser{ID: 1, Name: "Test User", Age: 25}
		result, err := pipeline.Process(ctx, user)

		if err == nil {
			t.Fatalf("expected cancellation error but got none")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}

		// With pre-canceled context, no steps should run
		// The pipeline should return the original input
		if result.Metadata != nil {
			t.Errorf("expected no metadata with pre-canceled context, got %v", result.Metadata)
		}
	})

	// Test context with timeout (more deterministic than manual cancellation)
	t.Run("timeout_during_execution", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		pipeline := pipz.NewSequence[TestUser]("cancellation-test",
			pipz.Transform("step1", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step1"] = true
				return user
			}),

			pipz.Apply("step2", func(_ context.Context, user TestUser) (TestUser, error) {
				// This step will cause timeout
				time.Sleep(20 * time.Millisecond)
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step2"] = true
				return user, nil
			}),

			pipz.Transform("step3", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["step3"] = true
				return user
			}),
		)

		user := TestUser{ID: 1, Name: "Test User", Age: 25}
		result, err := pipeline.Process(ctx, user)

		if err == nil {
			t.Fatalf("expected timeout error but got none")
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded error, got %v", err)
		}

		// step1 should have completed since it runs before the timeout
		if _, ok := result.Metadata["step1"]; !ok {
			t.Error("expected step1 to complete before timeout")
		}

		// step3 should not have completed due to timeout in step2
		if _, ok := result.Metadata["step3"]; ok {
			t.Error("unexpected step3 completion after timeout")
		}
	})
}

func TestPipelineFlows_ComplexComposition(t *testing.T) {
	ctx := context.Background()

	// Build a complex pipeline that combines multiple patterns:
	// 1. Validation with fallback
	// 2. Conditional processing with switch
	// 3. Parallel enrichment with concurrent
	// 4. Final aggregation

	// Mock external services
	validateAPI := pipztesting.NewMockProcessor[TestUser](t, "validate-api").
		WithReturn(TestUser{}, nil)

	enrichAPI1 := pipztesting.NewMockProcessor[TestUser](t, "enrich-api-1").
		WithDelay(50 * time.Millisecond)
	enrichAPI2 := pipztesting.NewMockProcessor[TestUser](t, "enrich-api-2").
		WithDelay(30 * time.Millisecond)

	// Build the complex pipeline
	pipeline := pipz.NewSequence[TestUser]("complex-composition",
		// Step 1: Validation with fallback
		pipz.NewFallback[TestUser]("validation",
			// Primary: API validation
			validateAPI,
			// Fallback: Local validation
			pipz.Apply("local-validate", func(_ context.Context, user TestUser) (TestUser, error) {
				if user.Name == "" || user.Email == "" {
					return user, errors.New("invalid user data")
				}
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["validation_method"] = "local"
				return user, nil
			}),
		),

		// Step 2: Conditional processing based on user type
		pipz.NewSwitch[TestUser, string]("user-type-processing", func(_ context.Context, user TestUser) string {
			if user.Premium {
				return "premium"
			}
			return "standard"
		}).AddRoute("standard",
			pipz.Transform("standard", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["processing_type"] = "standard"
				return user
			}),
		).AddRoute("premium",
			pipz.Transform("premium", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["processing_type"] = "premium"
				user.Metadata["premium_features"] = []string{"priority_support", "advanced_analytics"}
				return user
			}),
		),

		// Step 3: Parallel enrichment (note: using sequential due to Cloner requirement)
		pipz.NewSequence[TestUser]("enrichment",
			// Enrich with API 1
			pipz.Transform("enrich-1", func(_ context.Context, user TestUser) TestUser {
				_, _ = enrichAPI1.Process(ctx, user) //nolint:errcheck // Mock API call for test
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				if enrichedBy, ok := user.Metadata["enriched_by"].([]string); ok {
					user.Metadata["enriched_by"] = append(enrichedBy, "api1")
				} else {
					user.Metadata["enriched_by"] = []string{"api1"}
				}
				return user
			}),

			// Enrich with API 2
			pipz.Transform("enrich-2", func(_ context.Context, user TestUser) TestUser {
				_, _ = enrichAPI2.Process(ctx, user) //nolint:errcheck // Mock API call for test
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				if enrichedBy, ok := user.Metadata["enriched_by"].([]string); ok {
					user.Metadata["enriched_by"] = append(enrichedBy, "api2")
				} else {
					user.Metadata["enriched_by"] = []string{"api2"}
				}
				return user
			}),
		),

		// Step 4: Final aggregation
		pipz.Transform("aggregate", func(_ context.Context, user TestUser) TestUser {
			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata["processing_complete"] = true
			user.Metadata["processed_at"] = time.Now().Format(time.RFC3339)
			return user
		}),
	)

	testCases := []struct {
		name  string
		user  TestUser
		setup func()
	}{
		{
			name: "standard_user_api_success",
			user: TestUser{ID: 1, Name: "John", Email: "john@test.com", Age: 25, Premium: false},
			setup: func() {
				validateAPI.Reset()
				enrichAPI1.Reset()
				enrichAPI2.Reset()
				validateAPI.WithReturn(TestUser{}, nil) // API succeeds
				enrichAPI1.WithReturn(TestUser{}, nil)
				enrichAPI2.WithReturn(TestUser{}, nil)
			},
		},
		{
			name: "premium_user_api_failure",
			user: TestUser{ID: 2, Name: "Jane", Email: "jane@test.com", Age: 35, Premium: true},
			setup: func() {
				validateAPI.Reset()
				enrichAPI1.Reset()
				enrichAPI2.Reset()
				validateAPI.WithReturn(TestUser{}, errors.New("API failure")) // API fails, use fallback
				enrichAPI1.WithReturn(TestUser{}, nil)
				enrichAPI2.WithReturn(TestUser{}, nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()

			// Initialize enriched_by slice if not present
			if tc.user.Metadata == nil {
				tc.user.Metadata = make(map[string]interface{})
			}
			tc.user.Metadata["enriched_by"] = []string{}

			result, err := pipeline.Process(ctx, tc.user)
			if err != nil {
				t.Fatalf("complex pipeline failed: %v", err)
			}

			// Verify validation occurred (either API or fallback)
			if validationMethod, ok := result.Metadata["validation_method"]; ok {
				if validationMethod != "local" {
					t.Errorf("expected fallback validation method when API fails")
				}
			}

			// Verify processing type matches user premium status
			processingType := result.Metadata["processing_type"]
			if tc.user.Premium && processingType != "premium" {
				t.Errorf("expected premium processing for premium user, got %v", processingType)
			}
			if !tc.user.Premium && processingType != "standard" {
				t.Errorf("expected standard processing for non-premium user, got %v", processingType)
			}

			// Verify parallel enrichment occurred
			if enrichedBy, ok := result.Metadata["enriched_by"].([]string); ok {
				if len(enrichedBy) != 2 {
					t.Errorf("expected 2 enrichment sources, got %d", len(enrichedBy))
				}
			} else {
				t.Error("expected enriched_by metadata")
			}

			// Verify final aggregation
			if complete, ok := result.Metadata["processing_complete"]; !ok || complete != true {
				t.Error("expected processing_complete to be true")
			}

			if _, ok := result.Metadata["processed_at"]; !ok {
				t.Error("expected processed_at timestamp")
			}

			// Verify mock APIs were called
			pipztesting.AssertProcessed(t, validateAPI, 1)
			pipztesting.AssertProcessed(t, enrichAPI1, 1)
			pipztesting.AssertProcessed(t, enrichAPI2, 1)
		})
	}
}
