// Package processors contains the business logic implementations used in pipz demos.
// These processors demonstrate real-world use cases and best practices for building
// pipelines with pipz.
package processors

import (
	"fmt"
	"strings"
	"time"
)

// SecurityKey is the contract key type for security audit pipelines.
type SecurityKey string

// AuditableData represents data that needs security auditing.
// It wraps user data with audit metadata for tracking access and modifications.
type AuditableData struct {
	Data      *User
	UserID    string
	Timestamp time.Time
	Actions   []string
}

// User represents a user record with sensitive information.
type User struct {
	Name      string
	Email     string
	SSN       string
	IsAdmin   bool
}

// CheckPermissions validates that the accessing user has proper authorization.
// It returns an error if no user ID is provided, implementing zero-trust security.
func CheckPermissions(a AuditableData) (AuditableData, error) {
	if a.UserID == "" {
		return a, fmt.Errorf("unauthorized: no user ID")
	}
	a.Actions = append(a.Actions, "permissions verified")
	return a, nil
}

// LogAccess records who accessed the data and when.
// This creates an audit trail for compliance and security monitoring.
func LogAccess(a AuditableData) (AuditableData, error) {
	a.Actions = append(a.Actions, fmt.Sprintf("accessed by %s at %s",
		a.UserID, a.Timestamp.Format(time.RFC3339)))
	return a, nil
}

// RedactSensitive masks personally identifiable information (PII) based on user permissions.
// Non-admin users see redacted SSN and partially masked email addresses.
func RedactSensitive(a AuditableData) (AuditableData, error) {
	if a.Data == nil {
		return a, nil
	}

	// Redact SSN for everyone (show only last 4 digits)
	if len(a.Data.SSN) > 4 {
		a.Data.SSN = "XXX-XX-" + a.Data.SSN[len(a.Data.SSN)-4:]
	}

	// Additional redaction for non-admin users
	if !strings.Contains(a.UserID, "admin") {
		// Partially mask email
		if at := strings.Index(a.Data.Email, "@"); at > 1 {
			a.Data.Email = a.Data.Email[:1] + "***" + a.Data.Email[at:]
		}
	}

	a.Actions = append(a.Actions, "sensitive data redacted")
	return a, nil
}

// TrackCompliance records that the access was compliant with regulations.
// This processor would typically integrate with compliance monitoring systems.
func TrackCompliance(a AuditableData) (AuditableData, error) {
	// In production, this would send to compliance monitoring system
	a.Actions = append(a.Actions, "GDPR/CCPA compliant access logged")
	return a, nil
}