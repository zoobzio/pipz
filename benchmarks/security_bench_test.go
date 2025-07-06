package benchmarks

import (
	"testing"
	"time"
	
	"pipz"
	"pipz/examples"
)

// BenchmarkSecurityPipeline tests the performance of a security audit pipeline
func BenchmarkSecurityPipeline(b *testing.B) {
	// Setup
	const securityKey examples.SecurityKey = "bench-security"
	contract := pipz.GetContract[examples.AuditableData](securityKey)
	contract.Register(
		pipz.Apply(examples.CheckPermissions),
		pipz.Apply(examples.LogAccess),
		pipz.Apply(examples.RedactSensitive),
		pipz.Apply(examples.TrackCompliance),
	)
	
	// Test data - non-admin user
	auditData := examples.AuditableData{
		UserID:    "user-123",
		Timestamp: time.Now(),
		Actions:   []string{"read_patient_record"},
		Data: &examples.User{
			Name:    "John Doe",
			Email:   "john.doe@example.com",
			SSN:     "123-45-6789",
			IsAdmin: false,
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(auditData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSecurityPipelineAdmin tests performance with admin user (less redaction)
func BenchmarkSecurityPipelineAdmin(b *testing.B) {
	// Setup
	const securityKey examples.SecurityKey = "bench-security-admin"
	contract := pipz.GetContract[examples.AuditableData](securityKey)
	contract.Register(
		pipz.Apply(examples.CheckPermissions),
		pipz.Apply(examples.LogAccess),
		pipz.Apply(examples.RedactSensitive),
		pipz.Apply(examples.TrackCompliance),
	)
	
	// Test data - admin user
	auditData := examples.AuditableData{
		UserID:    "admin-999",
		Timestamp: time.Now(),
		Actions:   []string{"read_patient_record"},
		Data: &examples.User{
			Name:    "Jane Admin",
			Email:   "jane.admin@hospital.com",
			SSN:     "987-65-4321",
			IsAdmin: true,
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(auditData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSecurityPipelineMinimal tests performance with minimal security checks
func BenchmarkSecurityPipelineMinimal(b *testing.B) {
	// Setup - only permission check and logging
	const securityKey examples.SecurityKey = "bench-security-minimal"
	contract := pipz.GetContract[examples.AuditableData](securityKey)
	contract.Register(
		pipz.Apply(examples.CheckPermissions),
		pipz.Apply(examples.LogAccess),
	)
	
	// Test data
	auditData := examples.AuditableData{
		UserID:    "user-789",
		Timestamp: time.Now(),
		Actions:   []string{"list_patients"},
		Data: &examples.User{
			Name:    "API User",
			Email:   "api@example.com",
			SSN:     "000-00-0000",
			IsAdmin: false,
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(auditData)
		if err != nil {
			b.Fatal(err)
		}
	}
}