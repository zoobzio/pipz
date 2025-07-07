package main

import (
	"strings"
	"testing"
	"time"

	"pipz"
)

func TestCheckPermissions(t *testing.T) {
	tests := []struct {
		name    string
		audit   AuditableData
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid user ID",
			audit: AuditableData{
				UserID: "user123",
			},
			wantErr: false,
		},
		{
			name: "empty user ID",
			audit: AuditableData{
				UserID: "",
			},
			wantErr: true,
			errMsg:  "unauthorized: no user ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CheckPermissions(tt.audit)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("CheckPermissions() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err == nil && !containsAction(result.Actions, "permissions verified") {
				t.Error("CheckPermissions() should add 'permissions verified' action")
			}
			if err != nil && result.Data != nil {
				t.Error("CheckPermissions() should return zero value on error")
			}
		})
	}
}

func TestLogAccess(t *testing.T) {
	audit := AuditableData{
		UserID:    "user123",
		Timestamp: time.Now(),
		Actions:   []string{},
	}

	result := LogAccess(audit)
	
	if len(result.Actions) != 1 {
		t.Errorf("LogAccess() should add one action, got %d", len(result.Actions))
	}
	
	if !strings.Contains(result.Actions[0], "accessed by user123") {
		t.Error("LogAccess() should record user ID")
	}
	
	if !strings.Contains(result.Actions[0], "at 20") { // Year starts with 20
		t.Error("LogAccess() should record timestamp")
	}
}

func TestRedactSensitive(t *testing.T) {
	tests := []struct {
		name           string
		audit          AuditableData
		expectedSSN    string
		expectedEmail  string
		isAdmin        bool
	}{
		{
			name: "regular user redaction",
			audit: AuditableData{
				Data: &User{
					Name:  "John Doe",
					Email: "john.doe@example.com",
					SSN:   "123-45-6789",
				},
				UserID: "user123",
			},
			expectedSSN:   "XXX-XX-6789",
			expectedEmail: "j***@example.com",
			isAdmin:       false,
		},
		{
			name: "admin user redaction",
			audit: AuditableData{
				Data: &User{
					Name:  "Jane Admin",
					Email: "jane.admin@example.com",
					SSN:   "987-65-4321",
				},
				UserID: "admin456",
			},
			expectedSSN:   "XXX-XX-4321",
			expectedEmail: "jane.admin@example.com", // Admin keeps full email
			isAdmin:       true,
		},
		{
			name: "short SSN",
			audit: AuditableData{
				Data: &User{
					SSN: "123",
				},
				UserID: "user123",
			},
			expectedSSN:   "123", // Too short to redact
			expectedEmail: "",
		},
		{
			name: "nil data",
			audit: AuditableData{
				Data:   nil,
				UserID: "user123",
			},
			expectedSSN:   "",
			expectedEmail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitive(tt.audit)
			
			if tt.audit.Data == nil {
				if result.Data != nil {
					t.Error("RedactSensitive() should return nil data for nil input")
				}
				return
			}
			
			if result.Data.SSN != tt.expectedSSN {
				t.Errorf("RedactSensitive() SSN = %v, want %v", result.Data.SSN, tt.expectedSSN)
			}
			
			if result.Data.Email != tt.expectedEmail {
				t.Errorf("RedactSensitive() Email = %v, want %v", result.Data.Email, tt.expectedEmail)
			}
			
			if !containsAction(result.Actions, "sensitive data redacted") {
				t.Error("RedactSensitive() should add redaction action")
			}
		})
	}
}

func TestValidateDataIntegrity(t *testing.T) {
	tests := []struct {
		name    string
		audit   AuditableData
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid data",
			audit: AuditableData{
				Data:      &User{Name: "Test"},
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "nil data",
			audit: AuditableData{
				Data:      nil,
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "no data to audit",
		},
		{
			name: "zero timestamp",
			audit: AuditableData{
				Data:      &User{Name: "Test"},
				Timestamp: time.Time{},
			},
			wantErr: true,
			errMsg:  "missing timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDataIntegrity(tt.audit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDataIntegrity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateDataIntegrity() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestSecurityPipeline(t *testing.T) {
	pipeline := CreateSecurityPipeline()

	tests := []struct {
		name    string
		audit   AuditableData
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid access",
			audit: AuditableData{
				Data: &User{
					Name:  "John Doe",
					Email: "john@example.com",
					SSN:   "123-45-6789",
				},
				UserID:    "user123",
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "unauthorized access",
			audit: AuditableData{
				Data: &User{
					Name: "John Doe",
				},
				UserID:    "",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "unauthorized",
		},
		{
			name: "invalid data",
			audit: AuditableData{
				UserID:    "user123",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "no data to audit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(tt.audit)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Process() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err == nil {
				// Verify all actions were recorded
				expectedActions := []string{
					"permissions verified",
					"accessed by",
					"sensitive data redacted",
					"GDPR/CCPA compliant",
				}
				for _, action := range expectedActions {
					found := false
					for _, a := range result.Actions {
						if strings.Contains(a, action) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Process() missing expected action: %s", action)
					}
				}
			}
		})
	}
}

func TestAdminPipeline(t *testing.T) {
	pipeline := CreateAdminPipeline()

	audit := AuditableData{
		Data: &User{
			Name:  "Admin User",
			Email: "admin@example.com",
			SSN:   "987-65-4321",
		},
		UserID:    "admin123",
		Timestamp: time.Now(),
	}

	result, err := pipeline.Process(audit)
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	// Admin should have SSN redacted but full email
	if result.Data.SSN != "XXX-XX-4321" {
		t.Errorf("Admin SSN should be redacted, got %v", result.Data.SSN)
	}
	if result.Data.Email != "admin@example.com" {
		t.Errorf("Admin email should not be redacted, got %v", result.Data.Email)
	}
	if !containsAction(result.Actions, "admin view - minimal redaction") {
		t.Error("Admin pipeline should record minimal redaction")
	}
}

func TestSecurityChaining(t *testing.T) {
	// Create permission check pipeline
	permissionCheck := pipz.NewContract[AuditableData]()
	permissionCheck.Register(
		pipz.Apply(CheckPermissions),
		pipz.Validate(ValidateDataIntegrity),
	)

	// Create redaction pipeline
	redactionPipeline := pipz.NewContract[AuditableData]()
	redactionPipeline.Register(
		pipz.Transform(LogAccess),
		pipz.Transform(RedactSensitive),
		pipz.Transform(TrackCompliance),
	)

	// Chain them together
	chain := pipz.NewChain[AuditableData]()
	chain.Add(permissionCheck, redactionPipeline)

	// Test valid audit
	audit := AuditableData{
		Data: &User{
			Name:  "Test User",
			Email: "test@example.com",
			SSN:   "555-55-5555",
		},
		UserID:    "user999",
		Timestamp: time.Now(),
	}

	result, err := chain.Process(audit)
	if err != nil {
		t.Errorf("Chain.Process() unexpected error: %v", err)
	}

	// Verify data was redacted
	if !strings.HasPrefix(result.Data.SSN, "XXX-XX-") {
		t.Error("Chain.Process() should redact SSN")
	}
	if !strings.Contains(result.Data.Email, "***") {
		t.Error("Chain.Process() should redact email for non-admin")
	}
}

func TestSecurityEffect(t *testing.T) {
	// Pipeline with audit logging effect
	pipeline := pipz.NewContract[AuditableData]()
	
	var loggedAccess []string
	
	pipeline.Register(
		pipz.Apply(CheckPermissions),
		pipz.Effect(func(a AuditableData) error {
			// Simulate audit logging
			loggedAccess = append(loggedAccess, 
				"AUDIT: User "+a.UserID+" accessed data at "+a.Timestamp.Format(time.RFC3339))
			return nil
		}),
		pipz.Transform(RedactSensitive),
	)

	audit := AuditableData{
		Data: &User{
			Name:  "Test User",
			Email: "test@example.com",
			SSN:   "111-11-1111",
		},
		UserID:    "testuser",
		Timestamp: time.Now(),
	}

	_, err := pipeline.Process(audit)
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}

	if len(loggedAccess) != 1 {
		t.Errorf("Effect should log access, got %d logs", len(loggedAccess))
	}
	if !strings.Contains(loggedAccess[0], "AUDIT: User testuser") {
		t.Error("Effect should log user ID")
	}
}

// TestSecurityErrorPropagation verifies zero values are returned on error
func TestSecurityErrorPropagation(t *testing.T) {
	pipeline := CreateSecurityPipeline()
	
	// Audit that will fail permissions check
	audit := AuditableData{
		Data: &User{
			Name: "Test",
		},
		UserID:    "", // No user ID
		Timestamp: time.Now(),
	}
	
	result, err := pipeline.Process(audit)
	if err == nil {
		t.Error("Expected error for unauthorized access")
	}
	
	// Verify zero value is returned
	if result.Data != nil || result.UserID != "" || len(result.Actions) != 0 {
		t.Error("Expected zero value AuditableData on error")
	}
}

// Helper function
func containsAction(actions []string, substr string) bool {
	for _, action := range actions {
		if strings.Contains(action, substr) {
			return true
		}
	}
	return false
}