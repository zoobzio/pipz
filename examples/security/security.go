// Package security demonstrates how to use pipz for security auditing,
// permission checks, and data redaction pipelines.
package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zoobzio/pipz"
)

// AuditableData represents data that needs security processing
type AuditableData struct {
	Data        *User
	RequestorID string
	Timestamp   time.Time
	Actions     []string
	Context     map[string]interface{}
}

// User represents a user with sensitive information
type User struct {
	ID        string
	Name      string
	Email     string
	SSN       string
	Phone     string
	Role      string
	IsAdmin   bool
	CreatedAt time.Time
}

// AuditLog represents where audit events would be recorded
type AuditLog struct {
	Events []AuditEvent
}

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	UserID    string
	Action    string
	Target    interface{}
	Timestamp time.Time
}

// Global audit log for demo purposes
var globalAuditLog = &AuditLog{
	Events: make([]AuditEvent, 0),
}

// CheckPermissions verifies the requestor has permission to access the data
func CheckPermissions(_ context.Context, a AuditableData) (AuditableData, error) {
	if a.RequestorID == "" {
		return AuditableData{}, fmt.Errorf("unauthorized: no requestor ID provided")
	}

	// Simulate permission check
	if !strings.HasPrefix(a.RequestorID, "USER-") && !strings.HasPrefix(a.RequestorID, "ADMIN-") {
		return AuditableData{}, fmt.Errorf("unauthorized: invalid requestor ID format")
	}

	// Check if requestor is admin
	if strings.HasPrefix(a.RequestorID, "ADMIN-") {
		a.Context["is_admin_request"] = true
	}

	a.Actions = append(a.Actions, fmt.Sprintf("permissions verified for %s", a.RequestorID))
	return a, nil
}

// LogAccess records the data access in the audit log
func LogAccess(_ context.Context, a AuditableData) (AuditableData, error) {
	event := AuditEvent{
		UserID:    a.RequestorID,
		Action:    "data_access",
		Target:    a.Data.ID,
		Timestamp: a.Timestamp,
	}

	// In a real system, this would persist to a database or audit service
	globalAuditLog.Events = append(globalAuditLog.Events, event)

	a.Actions = append(a.Actions, fmt.Sprintf("access logged at %s",
		a.Timestamp.Format(time.RFC3339)))

	return a, nil
}

// RedactSensitiveData removes or masks sensitive information based on permissions
func RedactSensitiveData(_ context.Context, a AuditableData) (AuditableData, error) {
	if a.Data == nil {
		return a, nil
	}

	// Check if this is an admin request
	isAdmin, ok := a.Context["is_admin_request"].(bool)
	if !ok {
		isAdmin = false
	}

	// Non-admins get redacted data
	if !isAdmin {
		// Redact SSN - show only last 4 digits
		if len(a.Data.SSN) > 4 {
			a.Data.SSN = "***-**-" + a.Data.SSN[len(a.Data.SSN)-4:]
		}

		// Mask email - show only first 2 chars and domain
		if idx := strings.Index(a.Data.Email, "@"); idx > 2 {
			a.Data.Email = a.Data.Email[:2] + "****" + a.Data.Email[idx:]
		}

		// Redact phone - show only area code
		if len(a.Data.Phone) >= 10 {
			a.Data.Phone = a.Data.Phone[:3] + "-***-****"
		}

		a.Actions = append(a.Actions, "sensitive data redacted")
	} else {
		a.Actions = append(a.Actions, "full data access granted (admin)")
	}

	return a, nil
}

// CheckDataClassification adds data classification metadata
func CheckDataClassification(_ context.Context, a AuditableData) (AuditableData, error) {
	if a.Data == nil {
		return a, nil
	}

	// Classify data sensitivity
	sensitivity := "low"
	if a.Data.SSN != "" {
		sensitivity = "high"
	} else if a.Data.Email != "" || a.Data.Phone != "" {
		sensitivity = "medium"
	}

	a.Context["data_sensitivity"] = sensitivity
	a.Actions = append(a.Actions, fmt.Sprintf("data classified as %s sensitivity", sensitivity))

	return a, nil
}

// EnforceRateLimit checks if the requestor has exceeded access limits
func EnforceRateLimit(_ context.Context, a AuditableData) error {
	// Count recent accesses by this user
	recentCount := 0
	cutoff := time.Now().Add(-1 * time.Hour)

	for _, event := range globalAuditLog.Events {
		if event.UserID == a.RequestorID && event.Timestamp.After(cutoff) {
			recentCount++
		}
	}

	// Non-admins have stricter limits
	limit := 100
	if isAdmin, ok := a.Context["is_admin_request"].(bool); !ok || !isAdmin {
		limit = 10
	}

	if recentCount >= limit {
		return fmt.Errorf("rate limit exceeded: %d accesses in the last hour", recentCount)
	}

	return nil
}

// CreateSecurityPipeline creates a reusable security audit pipeline
func CreateSecurityPipeline() *pipz.Pipeline[AuditableData] {
	pipeline := pipz.NewPipeline[AuditableData]()
	pipeline.Register(
		pipz.Apply("check_permissions", CheckPermissions),
		pipz.Apply("log_access", LogAccess),
		pipz.Apply("classify_data", CheckDataClassification),
		pipz.Effect("rate_limit", EnforceRateLimit),
		pipz.Apply("redact_sensitive", RedactSensitiveData),
	)
	return pipeline
}

// CreateStrictSecurityPipeline creates a pipeline with additional security checks
func CreateStrictSecurityPipeline() *pipz.Pipeline[AuditableData] {
	pipeline := CreateSecurityPipeline()

	// Add additional security measures
	pipeline.PushHead(
		pipz.Effect("verify_timestamp", func(_ context.Context, a AuditableData) error {
			// Ensure timestamp is recent to prevent replay attacks
			if time.Since(a.Timestamp) > 5*time.Minute {
				return fmt.Errorf("request timestamp too old, possible replay attack")
			}
			return nil
		}),
	)

	pipeline.PushTail(
		pipz.Effect("security_metrics", func(_ context.Context, a AuditableData) error {
			// In a real system, this would update security metrics
			// Commenting out the print for benchmarks
			// sensitivity := a.Context["data_sensitivity"]
			// fmt.Printf("[METRICS] Data access: sensitivity=%v, requestor=%s\n",
			//	sensitivity, a.RequestorID)
			return nil
		}),
	)

	return pipeline
}

// GetUserData simulates fetching user data with security controls
func GetUserData(ctx context.Context, userID string, requestorID string) (*User, error) {
	// Simulate database fetch
	user := &User{
		ID:        userID,
		Name:      "John Doe",
		Email:     "john.doe@example.com",
		SSN:       "123-45-6789",
		Phone:     "555-123-4567",
		Role:      "user",
		IsAdmin:   false,
		CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
	}

	// Create auditable data
	auditData := AuditableData{
		Data:        user,
		RequestorID: requestorID,
		Timestamp:   time.Now(),
		Actions:     []string{},
		Context:     make(map[string]interface{}),
	}

	// Process through security pipeline
	pipeline := CreateSecurityPipeline()
	result, err := pipeline.Process(ctx, auditData)
	if err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	return result.Data, nil
}

// GetAuditLog returns recent audit events (for demo purposes)
func GetAuditLog() []AuditEvent {
	return globalAuditLog.Events
}

// ClearAuditLog clears the audit log (for testing)
func ClearAuditLog() {
	globalAuditLog.Events = make([]AuditEvent, 0)
}

// Example demonstrates the security pipeline in action
func Example() {
	ctx := context.Background()

	// Regular user accessing data
	fmt.Println("=== Regular User Access ===")
	user1, err := GetUserData(ctx, "USER-001", "USER-123")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("User data (redacted): %+v\n", user1)

	// Admin accessing data
	fmt.Println("\n=== Admin Access ===")
	user2, err := GetUserData(ctx, "USER-001", "ADMIN-456")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("User data (full): %+v\n", user2)

	// Show audit log
	fmt.Println("\n=== Audit Log ===")
	for _, event := range GetAuditLog() {
		fmt.Printf("%s - User %s performed %s on %v\n",
			event.Timestamp.Format(time.RFC3339),
			event.UserID,
			event.Action,
			event.Target)
	}
}
