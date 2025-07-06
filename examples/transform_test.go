package examples_test

import (
	"strings"
	"testing"

	"pipz"
	"pipz/examples"
)

func TestTransformPipeline(t *testing.T) {
	// Register transform pipeline
	const testKey examples.TransformKey = "test"
	contract := pipz.GetContract[examples.TransformKey, examples.TransformContext](testKey)
	
	err := contract.Register(
		pipz.Apply(examples.ParseCSV),
		pipz.Apply(examples.ValidateEmail),
		pipz.Apply(examples.NormalizePhone),
		pipz.Apply(examples.EnrichData),
	)
	if err != nil {
		t.Fatalf("Failed to register transform pipeline: %v", err)
	}

	t.Run("ValidTransformation", func(t *testing.T) {
		ctx := examples.TransformContext{
			CSV: &examples.CSVRecord{
				Fields: []string{"123", "JOHN DOE", "john.doe@example.com", "(555) 123-4567"},
			},
		}

		result, err := contract.Process(ctx)
		if err != nil {
			t.Fatalf("Transformation failed: %v", err)
		}

		// Verify parsing
		if result.DB == nil {
			t.Fatal("Database record not created")
		}

		// Verify name formatting
		if result.DB.Name != "John Doe" {
			t.Errorf("Name not properly formatted: %s", result.DB.Name)
		}

		// Verify email normalization
		if result.DB.Email != "john.doe@example.com" {
			t.Errorf("Email not normalized: %s", result.DB.Email)
		}

		// Verify phone formatting
		if result.DB.Phone != "+1-555-123-4567" {
			t.Errorf("Phone not properly formatted: %s", result.DB.Phone)
		}

		// Should have no errors
		if len(result.Errors) > 0 {
			t.Errorf("Unexpected errors: %v", result.Errors)
		}
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		ctx := examples.TransformContext{
			CSV: &examples.CSVRecord{
				Fields: []string{"456", "Jane Smith", "invalid-email", "555-987-6543"},
			},
		}

		result, err := contract.Process(ctx)
		if err != nil {
			t.Fatalf("Transformation should not fail on invalid email: %v", err)
		}

		// Should have email error
		if len(result.Errors) == 0 {
			t.Fatal("Expected email validation error")
		}

		foundEmailError := false
		for _, e := range result.Errors {
			if strings.Contains(e, "email") {
				foundEmailError = true
				break
			}
		}
		if !foundEmailError {
			t.Errorf("No email error found: %v", result.Errors)
		}
	})

	t.Run("PhoneNormalization", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"(555) 123-4567", "+1-555-123-4567"},
			{"5551234567", "+1-555-123-4567"},
			{"1-555-123-4567", "+1-555-123-4567"},
			{"555.123.4567", "+1-555-123-4567"},
		}

		for _, tc := range testCases {
			ctx := examples.TransformContext{
				CSV: &examples.CSVRecord{
					Fields: []string{"1", "Test", "test@example.com", tc.input},
				},
			}

			result, err := contract.Process(ctx)
			if err != nil {
				t.Fatalf("Failed to process %s: %v", tc.input, err)
			}

			if result.DB.Phone != tc.expected {
				t.Errorf("Phone %s normalized to %s, expected %s",
					tc.input, result.DB.Phone, tc.expected)
			}
		}
	})

	t.Run("InsufficientFields", func(t *testing.T) {
		ctx := examples.TransformContext{
			CSV: &examples.CSVRecord{
				Fields: []string{"123", "Name"}, // Missing email and phone
			},
		}

		_, err := contract.Process(ctx)
		if err == nil {
			t.Fatal("Expected error for insufficient fields")
		}

		if !strings.Contains(err.Error(), "insufficient") {
			t.Errorf("Wrong error message: %v", err)
		}
	})
}