package benchmarks

import (
	"fmt"
	"strings"
	"testing"
	"time"
	
	"pipz"
)

// Security types
type AuditData struct {
	User       *UserInfo
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

// Security processors
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

func BenchmarkSecurityPipeline(b *testing.B) {
	// Setup pipeline
	pipeline := pipz.NewContract[AuditData]()
	pipeline.Register(
		pipz.Apply(checkAccess),
		pipz.Transform(logSecurityAccess),
		pipz.Transform(redactPII),
		pipz.Transform(trackCompliance),
	)

	user := &UserInfo{
		Name:  "John Doe",
		Email: "john.doe@example.com",
		SSN:   "123-45-6789",
		Role:  "user",
	}

	audit := AuditData{
		User:       user,
		AccessorID: "user123",
		Timestamp:  time.Now(),
		Actions:    []string{},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Create fresh audit data each iteration
		freshAudit := AuditData{
			User:       user,
			AccessorID: audit.AccessorID,
			Timestamp:  audit.Timestamp,
			Actions:    []string{},
		}
		
		_, err := pipeline.Process(freshAudit)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSecurityPipelineAdmin(b *testing.B) {
	// Admin pipeline with less redaction
	pipeline := pipz.NewContract[AuditData]()
	pipeline.Register(
		pipz.Apply(checkAccess),
		pipz.Transform(logSecurityAccess),
		pipz.Transform(func(a AuditData) AuditData {
			if a.User != nil && len(a.User.SSN) > 4 {
				redacted := *a.User
				a.User = &redacted
				a.User.SSN = "XXX-XX-" + a.User.SSN[len(a.User.SSN)-4:]
			}
			a.Actions = append(a.Actions, "admin_view")
			return a
		}),
		pipz.Transform(trackCompliance),
	)

	user := &UserInfo{
		Name:  "Admin User",
		Email: "admin@example.com",
		SSN:   "987-65-4321",
		Role:  "admin",
	}

	audit := AuditData{
		User:       user,
		AccessorID: "admin456",
		Timestamp:  time.Now(),
		Actions:    []string{},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		freshAudit := AuditData{
			User:       user,
			AccessorID: audit.AccessorID,
			Timestamp:  audit.Timestamp,
			Actions:    []string{},
		}
		
		_, err := pipeline.Process(freshAudit)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedaction(b *testing.B) {
	user := &UserInfo{
		Name:  "Test User",
		Email: "test.user@example.com",
		SSN:   "555-55-5555",
		Role:  "user",
	}

	audit := AuditData{
		User:       user,
		AccessorID: "user999",
		Timestamp:  time.Now(),
		Actions:    []string{},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		freshAudit := AuditData{
			User:       user,
			AccessorID: audit.AccessorID,
			Timestamp:  audit.Timestamp,
			Actions:    []string{},
		}
		
		_ = redactPII(freshAudit)
	}
}

func BenchmarkSecurityChain(b *testing.B) {
	// Permission check pipeline
	permissionCheck := pipz.NewContract[AuditData]()
	permissionCheck.Register(
		pipz.Apply(checkAccess),
	)

	// Redaction pipeline
	redactionPipeline := pipz.NewContract[AuditData]()
	redactionPipeline.Register(
		pipz.Transform(logSecurityAccess),
		pipz.Transform(redactPII),
		pipz.Transform(trackCompliance),
	)

	// Chain them
	chain := pipz.NewChain[AuditData]()
	chain.Add(permissionCheck, redactionPipeline)

	user := &UserInfo{
		Name:  "Chain User",
		Email: "chain@example.com",
		SSN:   "777-77-7777",
		Role:  "user",
	}

	audit := AuditData{
		User:       user,
		AccessorID: "chain123",
		Timestamp:  time.Now(),
		Actions:    []string{},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		freshAudit := AuditData{
			User:       user,
			AccessorID: audit.AccessorID,
			Timestamp:  audit.Timestamp,
			Actions:    []string{},
		}
		
		_, err := chain.Process(freshAudit)
		if err != nil {
			b.Fatal(err)
		}
	}
}