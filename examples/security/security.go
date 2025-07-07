package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"pipz"
)

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
	Name    string
	Email   string
	SSN     string
	IsAdmin bool
}

// CheckPermissions validates that the accessing user has proper authorization.
// It returns an error if no user ID is provided, implementing zero-trust security.
func CheckPermissions(a AuditableData) (AuditableData, error) {
	if a.UserID == "" {
		return AuditableData{}, fmt.Errorf("unauthorized: no user ID")
	}
	a.Actions = append(a.Actions, "permissions verified")
	return a, nil
}

// LogAccess records who accessed the data and when.
// This creates an audit trail for compliance and security monitoring.
func LogAccess(a AuditableData) AuditableData {
	a.Actions = append(a.Actions, fmt.Sprintf("accessed by %s at %s",
		a.UserID, a.Timestamp.Format(time.RFC3339)))
	return a
}

// RedactSensitive masks personally identifiable information (PII) based on user permissions.
// Non-admin users see redacted SSN and partially masked email addresses.
func RedactSensitive(a AuditableData) AuditableData {
	if a.Data == nil {
		return a
	}

	// Clone the user data to avoid mutating the original
	redactedUser := *a.Data
	a.Data = &redactedUser

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
	return a
}

// TrackCompliance records that the access was compliant with regulations.
// This processor would typically integrate with compliance monitoring systems.
func TrackCompliance(a AuditableData) AuditableData {
	// In production, this would send to compliance monitoring system
	a.Actions = append(a.Actions, "GDPR/CCPA compliant access logged")
	return a
}

// ValidateDataIntegrity ensures the audit data is properly formed
func ValidateDataIntegrity(a AuditableData) error {
	if a.Data == nil {
		return fmt.Errorf("no data to audit")
	}
	if a.Timestamp.IsZero() {
		return fmt.Errorf("missing timestamp")
	}
	return nil
}

// CreateSecurityPipeline creates a security audit pipeline
func CreateSecurityPipeline() *pipz.Contract[AuditableData] {
	pipeline := pipz.NewContract[AuditableData]()
	pipeline.Register(
		pipz.Apply(CheckPermissions),
		pipz.Validate(ValidateDataIntegrity),
		pipz.Transform(LogAccess),
		pipz.Transform(RedactSensitive),
		pipz.Transform(TrackCompliance),
	)
	return pipeline
}

// CreateAdminPipeline creates a pipeline for admin users with less redaction
func CreateAdminPipeline() *pipz.Contract[AuditableData] {
	pipeline := pipz.NewContract[AuditableData]()
	pipeline.Register(
		pipz.Apply(CheckPermissions),
		pipz.Validate(ValidateDataIntegrity),
		pipz.Transform(LogAccess),
		// Admin users still get SSN redacted but keep full email
		pipz.Transform(func(a AuditableData) AuditableData {
			if a.Data != nil && len(a.Data.SSN) > 4 {
				redactedUser := *a.Data
				a.Data = &redactedUser
				a.Data.SSN = "XXX-XX-" + a.Data.SSN[len(a.Data.SSN)-4:]
			}
			a.Actions = append(a.Actions, "admin view - minimal redaction")
			return a
		}),
		pipz.Transform(TrackCompliance),
	)
	return pipeline
}

func main() {
	// Create security pipeline
	securityPipeline := CreateSecurityPipeline()

	// Test data
	user := &User{
		Name:    "John Doe",
		Email:   "john.doe@example.com",
		SSN:     "123-45-6789",
		IsAdmin: false,
	}

	// Test 1: Regular user access
	fmt.Println("Test 1: Regular user access")
	audit1 := AuditableData{
		Data:      user,
		UserID:    "user123",
		Timestamp: time.Now(),
		Actions:   []string{},
	}

	result, err := securityPipeline.Process(audit1)
	if err != nil {
		log.Printf("Security check failed: %v", err)
	} else {
		fmt.Printf("✓ Access granted to %s\n", result.UserID)
		fmt.Printf("  Email: %s\n", result.Data.Email)
		fmt.Printf("  SSN: %s\n", result.Data.SSN)
		fmt.Printf("  Audit trail: %v\n", result.Actions)
	}

	// Test 2: Admin user access
	fmt.Println("\nTest 2: Admin user access")
	adminPipeline := CreateAdminPipeline()
	
	admin := &User{
		Name:    "Jane Admin",
		Email:   "jane.admin@example.com",
		SSN:     "987-65-4321",
		IsAdmin: true,
	}

	audit2 := AuditableData{
		Data:      admin,
		UserID:    "admin456",
		Timestamp: time.Now(),
		Actions:   []string{},
	}

	result, err = adminPipeline.Process(audit2)
	if err != nil {
		log.Printf("Security check failed: %v", err)
	} else {
		fmt.Printf("✓ Admin access granted to %s\n", result.UserID)
		fmt.Printf("  Email: %s\n", result.Data.Email)
		fmt.Printf("  SSN: %s\n", result.Data.SSN)
		fmt.Printf("  Audit trail: %v\n", result.Actions)
	}

	// Test 3: Unauthorized access attempt
	fmt.Println("\nTest 3: Unauthorized access attempt")
	audit3 := AuditableData{
		Data:      user,
		UserID:    "", // No user ID
		Timestamp: time.Now(),
		Actions:   []string{},
	}

	_, err = securityPipeline.Process(audit3)
	if err != nil {
		fmt.Printf("✗ Access denied: %v\n", err)
	}

	// Test 4: Invalid data
	fmt.Println("\nTest 4: Invalid data integrity")
	audit4 := AuditableData{
		Data:      nil, // No data
		UserID:    "user789",
		Timestamp: time.Now(),
		Actions:   []string{},
	}

	_, err = securityPipeline.Process(audit4)
	if err != nil {
		fmt.Printf("✗ Integrity check failed: %v\n", err)
	}
}