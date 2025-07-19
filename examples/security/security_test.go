package security

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestCheckPermissions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		data    AuditableData
		wantErr string
	}{
		{
			name: "valid user ID",
			data: AuditableData{
				RequestorID: "USER-123",
				Context:     make(map[string]interface{}),
			},
			wantErr: "",
		},
		{
			name: "valid admin ID",
			data: AuditableData{
				RequestorID: "ADMIN-456",
				Context:     make(map[string]interface{}),
			},
			wantErr: "",
		},
		{
			name: "empty requestor ID",
			data: AuditableData{
				RequestorID: "",
				Context:     make(map[string]interface{}),
			},
			wantErr: "unauthorized: no requestor ID provided",
		},
		{
			name: "invalid ID format",
			data: AuditableData{
				RequestorID: "INVALID-123",
				Context:     make(map[string]interface{}),
			},
			wantErr: "unauthorized: invalid requestor ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CheckPermissions(ctx, tt.data)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				// Check if admin flag is set correctly
				if strings.HasPrefix(tt.data.RequestorID, "ADMIN-") {
					if !result.Context["is_admin_request"].(bool) {
						t.Error("expected is_admin_request to be true for admin user")
					}
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestRedactSensitiveData(t *testing.T) {
	ctx := context.Background()

	user := &User{
		ID:    "USER-001",
		Name:  "John Doe",
		Email: "john.doe@example.com",
		SSN:   "123-45-6789",
		Phone: "555-123-4567",
	}

	tests := []struct {
		name           string
		isAdmin        bool
		expectRedacted bool
	}{
		{
			name:           "admin sees full data",
			isAdmin:        true,
			expectRedacted: false,
		},
		{
			name:           "non-admin gets redacted data",
			isAdmin:        false,
			expectRedacted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the user for each test
			testUser := &User{
				ID:    user.ID,
				Name:  user.Name,
				Email: user.Email,
				SSN:   user.SSN,
				Phone: user.Phone,
			}

			data := AuditableData{
				Data:    testUser,
				Context: make(map[string]interface{}),
			}

			if tt.isAdmin {
				data.Context["is_admin_request"] = true
			}

			result, err := RedactSensitiveData(ctx, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectRedacted {
				// Check SSN is redacted
				if !strings.HasPrefix(result.Data.SSN, "***-**-") {
					t.Errorf("expected SSN to be redacted, got %s", result.Data.SSN)
				}
				// Check email is masked
				if !strings.Contains(result.Data.Email, "****@") {
					t.Errorf("expected email to be masked, got %s", result.Data.Email)
				}
				// Check phone is redacted
				if !strings.HasSuffix(result.Data.Phone, "-***-****") {
					t.Errorf("expected phone to be redacted, got %s", result.Data.Phone)
				}
			} else {
				// Admin should see full data
				if result.Data.SSN != user.SSN {
					t.Errorf("expected full SSN for admin, got %s", result.Data.SSN)
				}
				if result.Data.Email != user.Email {
					t.Errorf("expected full email for admin, got %s", result.Data.Email)
				}
				if result.Data.Phone != user.Phone {
					t.Errorf("expected full phone for admin, got %s", result.Data.Phone)
				}
			}
		})
	}
}

func TestEnforceRateLimit(t *testing.T) {
	ctx := context.Background()

	// Clear audit log for clean test
	ClearAuditLog()

	// Add some events to the audit log
	now := time.Now()
	for i := 0; i < 9; i++ {
		globalAuditLog.Events = append(globalAuditLog.Events, AuditEvent{
			UserID:    "USER-123",
			Action:    "data_access",
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	tests := []struct {
		name        string
		requestorID string
		isAdmin     bool
		wantErr     bool
	}{
		{
			name:        "user under limit",
			requestorID: "USER-123",
			isAdmin:     false,
			wantErr:     false,
		},
		{
			name:        "user at limit",
			requestorID: "USER-123",
			isAdmin:     false,
			wantErr:     true,
		},
		{
			name:        "admin under limit",
			requestorID: "ADMIN-456",
			isAdmin:     true,
			wantErr:     false,
		},
		{
			name:        "different user not limited",
			requestorID: "USER-789",
			isAdmin:     false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := AuditableData{
				RequestorID: tt.requestorID,
				Context:     make(map[string]interface{}),
			}

			if tt.isAdmin {
				data.Context["is_admin_request"] = true
			}

			// Add one more event if we're testing the limit
			if tt.name == "user at limit" {
				globalAuditLog.Events = append(globalAuditLog.Events, AuditEvent{
					UserID:    "USER-123",
					Action:    "data_access",
					Timestamp: now,
				})
			}

			err := EnforceRateLimit(ctx, data)
			if tt.wantErr && err == nil {
				t.Error("expected rate limit error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSecurityPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateSecurityPipeline()

	// Clear audit log
	ClearAuditLog()

	tests := []struct {
		name    string
		data    AuditableData
		wantErr string
	}{
		{
			name: "successful user access",
			data: AuditableData{
				Data: &User{
					ID:    "USER-001",
					Name:  "Test User",
					Email: "test@example.com",
					SSN:   "987-65-4321",
				},
				RequestorID: "USER-123",
				Timestamp:   time.Now(),
				Context:     make(map[string]interface{}),
			},
			wantErr: "",
		},
		{
			name: "unauthorized access",
			data: AuditableData{
				Data: &User{
					ID: "USER-001",
				},
				RequestorID: "",
				Timestamp:   time.Now(),
				Context:     make(map[string]interface{}),
			},
			wantErr: "unauthorized: no requestor ID provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(ctx, tt.data)

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Check that actions were logged
				if len(result.Actions) == 0 {
					t.Error("expected actions to be logged")
				}

				// Check that audit log was updated
				if len(GetAuditLog()) == 0 {
					t.Error("expected audit log to have entries")
				}

				// For non-admin users, check data was redacted
				if !strings.HasPrefix(tt.data.RequestorID, "ADMIN-") && result.Data != nil {
					if strings.Contains(result.Data.SSN, "987-65-4321") {
						t.Error("expected SSN to be redacted for non-admin user")
					}
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestPipelineErrorContext(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateSecurityPipeline()

	// Create data that will fail at rate limit
	ClearAuditLog()

	// Fill up the rate limit
	for i := 0; i < 10; i++ {
		globalAuditLog.Events = append(globalAuditLog.Events, AuditEvent{
			UserID:    "USER-123",
			Action:    "data_access",
			Timestamp: time.Now(),
		})
	}

	data := AuditableData{
		Data:        &User{ID: "USER-001"},
		RequestorID: "USER-123",
		Timestamp:   time.Now(),
		Context:     make(map[string]interface{}),
	}

	_, err := pipeline.Process(ctx, data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check that we get a PipelineError with context
	var pipelineErr *pipz.PipelineError[AuditableData]
	if !errors.As(err, &pipelineErr) {
		t.Fatalf("expected PipelineError, got %T", err)
	}

	// Verify error context
	if pipelineErr.ProcessorName != "rate_limit" {
		t.Errorf("expected processor name 'rate_limit', got %q", pipelineErr.ProcessorName)
	}

	if pipelineErr.StageIndex != 3 { // 4th processor (0-indexed)
		t.Errorf("expected stage index 3, got %d", pipelineErr.StageIndex)
	}
}

func TestStrictSecurityPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateStrictSecurityPipeline()

	// Clear audit log to avoid rate limit issues from previous tests
	ClearAuditLog()

	tests := []struct {
		name    string
		data    AuditableData
		wantErr string
	}{
		{
			name: "old timestamp rejected",
			data: AuditableData{
				Data:        &User{ID: "USER-001"},
				RequestorID: "USER-123",
				Timestamp:   time.Now().Add(-10 * time.Minute), // Too old
				Context:     make(map[string]interface{}),
			},
			wantErr: "request timestamp too old",
		},
		{
			name: "recent timestamp accepted",
			data: AuditableData{
				Data:        &User{ID: "USER-001"},
				RequestorID: "USER-123",
				Timestamp:   time.Now(),
				Context:     make(map[string]interface{}),
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pipeline.Process(ctx, tt.data)

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestGetUserData(t *testing.T) {
	ctx := context.Background()

	// Clear audit log
	ClearAuditLog()

	// Test regular user access
	user, err := GetUserData(ctx, "USER-001", "USER-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify data was redacted
	if !strings.HasPrefix(user.SSN, "***-**-") {
		t.Errorf("expected SSN to be redacted, got %s", user.SSN)
	}

	// Test admin access
	adminUser, err := GetUserData(ctx, "USER-001", "ADMIN-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify admin sees full data
	if strings.HasPrefix(adminUser.SSN, "***-**-") {
		t.Error("expected admin to see full SSN")
	}

	// Verify audit log has entries
	events := GetAuditLog()
	if len(events) != 2 {
		t.Errorf("expected 2 audit log entries, got %d", len(events))
	}
}

func TestDataClassification(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                string
		user                *User
		expectedSensitivity string
	}{
		{
			name: "high sensitivity with SSN",
			user: &User{
				SSN:   "123-45-6789",
				Email: "test@example.com",
			},
			expectedSensitivity: "high",
		},
		{
			name: "medium sensitivity with email/phone",
			user: &User{
				Email: "test@example.com",
				Phone: "555-1234",
			},
			expectedSensitivity: "medium",
		},
		{
			name: "low sensitivity with basic info",
			user: &User{
				Name: "John Doe",
			},
			expectedSensitivity: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := AuditableData{
				Data:    tt.user,
				Context: make(map[string]interface{}),
			}

			result, err := CheckDataClassification(ctx, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sensitivity, ok := result.Context["data_sensitivity"].(string)
			if !ok {
				t.Fatal("data_sensitivity not set in context")
			}

			if sensitivity != tt.expectedSensitivity {
				t.Errorf("expected sensitivity %q, got %q", tt.expectedSensitivity, sensitivity)
			}
		})
	}
}

// Example test to ensure the example compiles
func TestExample(t *testing.T) {
	// Capture output
	ClearAuditLog()

	// Run example (normally prints to stdout)
	Example()

	// Verify audit log was populated
	if len(GetAuditLog()) == 0 {
		t.Error("expected audit log to have entries after example")
	}
}
