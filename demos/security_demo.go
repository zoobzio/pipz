package main

import (
	"fmt"
	"strings"
	"time"

	"pipz"
)

// Security types for demo
type AuditData struct {
	User      *UserInfo
	AccessorID string
	Timestamp  time.Time
	Actions    []string
}

type UserInfo struct {
	Name  string
	Email string
	SSN   string
	Role  string
}

// Security functions
func checkAccess(a AuditData) (AuditData, error) {
	if a.AccessorID == "" {
		return AuditData{}, fmt.Errorf("unauthorized: no accessor ID")
	}
	a.Actions = append(a.Actions, "access_verified")
	return a, nil
}

func logSecurityAccess(a AuditData) AuditData {
	a.Actions = append(a.Actions, fmt.Sprintf("logged_at_%s", 
		a.Timestamp.Format("15:04:05")))
	return a
}

func redactPII(a AuditData) AuditData {
	if a.User == nil {
		return a
	}

	// Clone to avoid modifying original
	redacted := *a.User
	a.User = &redacted

	// Always redact SSN
	if len(a.User.SSN) > 4 {
		a.User.SSN = "XXX-XX-" + a.User.SSN[len(a.User.SSN)-4:]
	}

	// Redact email for non-admin
	if !strings.Contains(a.AccessorID, "admin") {
		if at := strings.Index(a.User.Email, "@"); at > 1 {
			a.User.Email = a.User.Email[:1] + "***" + a.User.Email[at:]
		}
	}

	a.Actions = append(a.Actions, "pii_redacted")
	return a
}

func trackCompliance(a AuditData) AuditData {
	a.Actions = append(a.Actions, "gdpr_compliant")
	return a
}

func runSecurityDemo() {
	section("SECURITY AUDIT PIPELINE")
	
	info("Use Case: Zero-Trust Data Access")
	info("• Verify accessor permissions")
	info("• Create audit trail")
	info("• Redact PII based on role")
	info("• Track regulatory compliance")

	// Create security pipeline
	securityPipeline := pipz.NewContract[AuditData]()
	securityPipeline.Register(
		pipz.Apply(checkAccess),
		pipz.Transform(logSecurityAccess),
		pipz.Transform(redactPII),
		pipz.Transform(trackCompliance),
	)

	code("go", `// Security pipeline with audit trail
security := pipz.NewContract[AuditData]()
security.Register(
    pipz.Apply(checkAccess),      // Zero-trust check
    pipz.Transform(logAccess),     // Audit logging
    pipz.Transform(redactPII),     // Data protection
    pipz.Transform(trackCompliance), // GDPR/CCPA
)`)

	// Test data
	sensitiveUser := &UserInfo{
		Name:  "Alice Johnson",
		Email: "alice.johnson@example.com",
		SSN:   "123-45-6789",
		Role:  "customer",
	}

	// Test Case 1: Regular user access
	fmt.Println("\n🔐 Test Case 1: Regular User Access")
	audit1 := AuditData{
		User:       sensitiveUser,
		AccessorID: "user456",
		Timestamp:  time.Now(),
	}

	result, err := securityPipeline.Process(audit1)
	if err != nil {
		showError(fmt.Sprintf("Access denied: %v", err))
	} else {
		success("Access granted with redaction")
		fmt.Printf("   Email: %s\n", result.User.Email)
		fmt.Printf("   SSN: %s\n", result.User.SSN)
		fmt.Printf("   Audit: %v\n", result.Actions)
	}

	// Test Case 2: Admin access
	fmt.Println("\n🔐 Test Case 2: Admin Access")
	audit2 := AuditData{
		User:       sensitiveUser,
		AccessorID: "admin789",
		Timestamp:  time.Now(),
	}

	result, err = securityPipeline.Process(audit2)
	if err != nil {
		showError(fmt.Sprintf("Access denied: %v", err))
	} else {
		success("Admin access granted")
		fmt.Printf("   Email: %s (full visibility)\n", result.User.Email)
		fmt.Printf("   SSN: %s\n", result.User.SSN)
		fmt.Printf("   Audit: %v\n", result.Actions)
	}

	// Test Case 3: Unauthorized access
	fmt.Println("\n🔐 Test Case 3: Unauthorized Access")
	audit3 := AuditData{
		User:       sensitiveUser,
		AccessorID: "", // No ID
		Timestamp:  time.Now(),
	}

	_, err = securityPipeline.Process(audit3)
	if err != nil {
		showError(fmt.Sprintf("Access denied as expected: %v", err))
	}

	fmt.Println("\n🔍 Security Features:")
	info("• Zero-trust: Every access requires verification")
	info("• Audit trail: Complete activity logging")
	info("• Role-based redaction: Different data visibility")
	info("• Compliance: Built-in regulatory tracking")
}