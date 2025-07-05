package processors_test

import (
	"strings"
	"testing"
	"time"

	"pipz"
	"pipz/demo/processors"
)

func TestSecurityPipeline(t *testing.T) {
	// Register the security pipeline
	const testKey processors.SecurityKey = "test"
	contract := pipz.GetContract[processors.SecurityKey, processors.AuditableData](testKey)
	
	err := contract.Register(
		processors.Adapt(processors.CheckPermissions),
		processors.Adapt(processors.LogAccess),
		processors.Adapt(processors.RedactSensitive),
		processors.Adapt(processors.TrackCompliance),
	)
	if err != nil {
		t.Fatalf("Failed to register pipeline: %v", err)
	}

	t.Run("ValidAccess", func(t *testing.T) {
		data := processors.AuditableData{
			Data: &processors.User{
				Name:  "John Doe",
				Email: "john@example.com",
				SSN:   "123-45-6789",
			},
			UserID:    "doctor-123",
			Timestamp: time.Now(),
		}

		result, err := contract.Process(data)
		if err != nil {
			t.Fatalf("Expected successful processing, got error: %v", err)
		}

		// Verify redaction occurred
		if !strings.HasPrefix(result.Data.SSN, "XXX-XX-") {
			t.Errorf("SSN not redacted: %s", result.Data.SSN)
		}

		// Verify email masking for non-admin
		if !strings.Contains(result.Data.Email, "***") {
			t.Errorf("Email not masked for non-admin: %s", result.Data.Email)
		}

		// Verify audit trail
		if len(result.Actions) != 4 {
			t.Errorf("Expected 4 audit actions, got %d", len(result.Actions))
		}
	})

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		data := processors.AuditableData{
			Data: &processors.User{
				Name: "Jane Doe",
				SSN:  "987-65-4321",
			},
			UserID:    "", // No user ID
			Timestamp: time.Now(),
		}

		_, err := contract.Process(data)
		if err == nil {
			t.Fatal("Expected authorization error, got nil")
		}

		if !strings.Contains(err.Error(), "unauthorized") {
			t.Errorf("Expected unauthorized error, got: %v", err)
		}
	})

	t.Run("AdminAccess", func(t *testing.T) {
		data := processors.AuditableData{
			Data: &processors.User{
				Name:  "Admin User",
				Email: "admin@example.com",
				SSN:   "555-55-5555",
			},
			UserID:    "admin-001",
			Timestamp: time.Now(),
		}

		result, err := contract.Process(data)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Admin should see unmasked email
		if strings.Contains(result.Data.Email, "***") {
			t.Errorf("Email should not be masked for admin: %s", result.Data.Email)
		}

		// SSN should still be redacted
		if !strings.HasPrefix(result.Data.SSN, "XXX-XX-") {
			t.Errorf("SSN not redacted even for admin: %s", result.Data.SSN)
		}
	})
}

func TestPipelineDiscovery(t *testing.T) {
	// Register a pipeline with one key
	const key1 processors.SecurityKey = "discovery-test"
	contract1 := pipz.GetContract[processors.SecurityKey, processors.AuditableData](key1)
	
	err := contract1.Register(
		processors.Adapt(processors.CheckPermissions),
	)
	if err != nil {
		t.Fatalf("Failed to register first contract: %v", err)
	}

	// Discover the same pipeline with the same key
	contract2 := pipz.GetContract[processors.SecurityKey, processors.AuditableData](key1)
	
	// Process through discovered contract
	data := processors.AuditableData{
		UserID:    "test-user",
		Timestamp: time.Now(),
		Data:      &processors.User{Name: "Test"},
	}

	result, err := contract2.Process(data)
	if err != nil {
		t.Fatalf("Failed to process through discovered contract: %v", err)
	}

	if len(result.Actions) != 1 {
		t.Errorf("Expected 1 action from discovered pipeline, got %d", len(result.Actions))
	}
}