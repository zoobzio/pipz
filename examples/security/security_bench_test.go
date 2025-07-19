package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/pipz/examples/security"
)

func BenchmarkSecurity_SingleProcessor(b *testing.B) {
	ctx := context.Background()

	b.Run("CheckPermissions", func(b *testing.B) {
		data := security.AuditableData{
			RequestorID: "USER-123",
			Context:     make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = security.CheckPermissions(ctx, data)
		}
	})

	b.Run("RedactSensitiveData", func(b *testing.B) {
		user := &security.User{
			ID:    "USER-001",
			Name:  "John Doe",
			Email: "john.doe@example.com",
			SSN:   "123-45-6789",
			Phone: "555-123-4567",
		}

		data := security.AuditableData{
			Data:    user,
			Context: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create a copy to avoid modifying the original
			userCopy := *user
			data.Data = &userCopy
			_, _ = security.RedactSensitiveData(ctx, data)
		}
	})
}

func BenchmarkSecurity_Pipeline(b *testing.B) {
	ctx := context.Background()
	pipeline := security.CreateSecurityPipeline()

	b.Run("SuccessfulAccess", func(b *testing.B) {
		user := &security.User{
			ID:        "USER-001",
			Name:      "Test User",
			Email:     "test@example.com",
			SSN:       "987-65-4321",
			Phone:     "555-987-6543",
			CreatedAt: time.Now(),
		}

		// Clear audit log periodically
		security.ClearAuditLog()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create fresh data for each iteration
			data := security.AuditableData{
				Data:        user,
				RequestorID: "USER-123",
				Timestamp:   time.Now(),
				Actions:     []string{},
				Context:     make(map[string]interface{}),
			}

			_, _ = pipeline.Process(ctx, data)

			// Clear every 100 iterations to prevent rate limit issues
			if i%100 == 0 {
				security.ClearAuditLog()
			}
		}
	})

	b.Run("UnauthorizedAccess_FailsFast", func(b *testing.B) {
		data := security.AuditableData{
			Data:        &security.User{ID: "USER-001"},
			RequestorID: "", // Will fail immediately
			Timestamp:   time.Now(),
			Context:     make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, data)
		}
	})
}
