package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/security"
	"github.com/zoobzio/pipz/examples/validation"
)

// ValidationExample implements the Example interface for validation
type ValidationExample struct{}

func (v *ValidationExample) Name() string {
	return "validation"
}

func (v *ValidationExample) Description() string {
	return "Composable validators with clear errors"
}

func (v *ValidationExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ DATA VALIDATION EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Composable validators with single responsibility")
	fmt.Println("• Clear error messages with context")
	fmt.Println("• Pipeline modification for different validation levels")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Complex business objects need multi-step validation.")
	fmt.Println("Traditional approaches lead to nested if-statements and")
	fmt.Println("validation logic scattered throughout the codebase.")

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("```go")
	fmt.Println("pipeline := pipz.NewPipeline[Order]()")
	fmt.Println("pipeline.Register(")
	fmt.Println("    pipz.Apply(\"validate_order_id\", ValidateOrderID),")
	fmt.Println("    pipz.Apply(\"validate_customer\", ValidateCustomerID),")
	fmt.Println("    pipz.Apply(\"validate_items\", ValidateItems),")
	fmt.Println("    pipz.Apply(\"validate_total\", ValidateTotal),")
	fmt.Println(")")
	fmt.Println("```")

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's try it!" + colorReset)
	return v.runInteractive(ctx)
}

func (v *ValidationExample) runInteractive(ctx context.Context) error {
	pipeline := validation.CreateValidationPipeline()

	// Test cases
	testCases := []struct {
		name  string
		order validation.Order
		valid bool
	}{
		{
			name: "Valid Order",
			order: validation.Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items: []validation.OrderItem{
					{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
					{ProductID: "PROD-002", Quantity: 1, Price: 49.99},
				},
				Total: 109.97,
			},
			valid: true,
		},
		{
			name: "Invalid Order ID",
			order: validation.Order{
				ID:         "12345", // Missing ORD- prefix
				CustomerID: "CUST-789",
				Items: []validation.OrderItem{
					{ProductID: "PROD-001", Quantity: 1, Price: 10.00},
				},
				Total: 10.00,
			},
			valid: false,
		},
		{
			name: "Incorrect Total",
			order: validation.Order{
				ID:         "ORD-12345",
				CustomerID: "CUST-789",
				Items: []validation.OrderItem{
					{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
				},
				Total: 100.00, // Wrong!
			},
			valid: false,
		},
	}

	for i, tc := range testCases {
		fmt.Printf("\n%sTest %d: %s%s\n", colorWhite, i+1, tc.name, colorReset)
		fmt.Printf("Order: %+v\n", tc.order)

		start := time.Now()
		result, err := pipeline.Process(ctx, tc.order)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("%sResult: ❌ FAILED%s\n", colorRed, colorReset)

			// Show detailed error information
			var pipelineErr *pipz.PipelineError[validation.Order]
			if errors.As(err, &pipelineErr) {
				fmt.Printf("  Failed at: %s%s%s (stage %d)\n",
					colorYellow, pipelineErr.ProcessorName, colorReset, pipelineErr.StageIndex)
			}
			fmt.Printf("  Error: %s\n", err.Error())
		} else {
			fmt.Printf("%sResult: ✅ PASSED%s\n", colorGreen, colorReset)
			fmt.Printf("  Validated order: %s\n", result.ID)
		}

		fmt.Printf("  Time: %s%v%s\n", colorCyan, duration, colorReset)

		if i < len(testCases)-1 {
			waitForEnter()
		}
	}

	return nil
}

func (v *ValidationExample) Benchmark(b *testing.B) error {
	// This would be implemented if we needed in-process benchmarking
	// For now, we use go test -bench
	return nil
}

// SecurityExample implements the Example interface for security
type SecurityExample struct{}

func (s *SecurityExample) Name() string {
	return "security"
}

func (s *SecurityExample) Description() string {
	return "Permission checks and data redaction"
}

func (s *SecurityExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ SECURITY AUDIT EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Permission-based access control")
	fmt.Println("• Automatic data redaction based on roles")
	fmt.Println("• Comprehensive audit logging")
	fmt.Println("• Rate limiting and replay attack prevention")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Applications need to track data access, redact sensitive")
	fmt.Println("information based on permissions, and maintain audit logs.")
	fmt.Println("This logic often gets tangled with business logic.")

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("```go")
	fmt.Println("securityPipeline := pipz.NewPipeline[AuditableData]()")
	fmt.Println("securityPipeline.Register(")
	fmt.Println("    pipz.Apply(\"check_permissions\", CheckPermissions),")
	fmt.Println("    pipz.Apply(\"log_access\", LogAccess),")
	fmt.Println("    pipz.Apply(\"classify_data\", CheckDataClassification),")
	fmt.Println("    pipz.Validate(\"rate_limit\", EnforceRateLimit),")
	fmt.Println("    pipz.Apply(\"redact_sensitive\", RedactSensitiveData),")
	fmt.Println(")")
	fmt.Println("```")

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's try it!" + colorReset)
	return s.runInteractive(ctx)
}

func (s *SecurityExample) runInteractive(ctx context.Context) error {
	// Clear audit log for demo
	security.ClearAuditLog()

	fmt.Println("\n" + colorWhite + "Scenario 1: Regular User Access" + colorReset)
	fmt.Println("A regular user (USER-123) is accessing user data...")

	userData, err := security.GetUserData(ctx, "USER-001", "USER-123")
	if err != nil {
		fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
	} else {
		fmt.Printf("\n%sData received (notice redaction):%s\n", colorGreen, colorReset)
		fmt.Printf("  Name:  %s\n", userData.Name)
		fmt.Printf("  Email: %s%s%s (masked)\n", colorYellow, userData.Email, colorReset)
		fmt.Printf("  SSN:   %s%s%s (redacted)\n", colorYellow, userData.SSN, colorReset)
		fmt.Printf("  Phone: %s%s%s (redacted)\n", colorYellow, userData.Phone, colorReset)
	}

	waitForEnter()

	fmt.Println("\n" + colorWhite + "Scenario 2: Admin Access" + colorReset)
	fmt.Println("An admin (ADMIN-456) is accessing the same user data...")

	adminData, err := security.GetUserData(ctx, "USER-001", "ADMIN-456")
	if err != nil {
		fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
	} else {
		fmt.Printf("\n%sData received (full access):%s\n", colorGreen, colorReset)
		fmt.Printf("  Name:  %s\n", adminData.Name)
		fmt.Printf("  Email: %s\n", adminData.Email)
		fmt.Printf("  SSN:   %s\n", adminData.SSN)
		fmt.Printf("  Phone: %s\n", adminData.Phone)
	}

	waitForEnter()

	fmt.Println("\n" + colorWhite + "Scenario 3: Unauthorized Access Attempt" + colorReset)
	fmt.Println("Someone without proper credentials tries to access data...")

	_, err = security.GetUserData(ctx, "USER-001", "HACKER-999")
	if err != nil {
		fmt.Printf("\n%sAccess Denied: %v%s\n", colorRed, err, colorReset)

		// Show error context
		var pipelineErr *pipz.PipelineError[security.AuditableData]
		if errors.As(err, &pipelineErr) {
			fmt.Printf("  Failed at: %s%s%s (stage %d)\n",
				colorYellow, pipelineErr.ProcessorName, colorReset, pipelineErr.StageIndex)
		}
	}

	waitForEnter()

	fmt.Println("\n" + colorWhite + "Scenario 4: Rate Limiting" + colorReset)
	fmt.Println("Simulating too many requests from the same user...")

	// Make multiple requests to trigger rate limit
	for i := 0; i < 11; i++ {
		_, err := security.GetUserData(ctx, "USER-001", "USER-789")
		if err != nil {
			fmt.Printf("\n%sRequest %d failed: %v%s\n", colorRed, i+1, err, colorReset)
			break
		} else if i < 10 {
			fmt.Printf("Request %d: %sSuccess%s\n", i+1, colorGreen, colorReset)
		}
	}

	waitForEnter()

	// Show audit log
	fmt.Println("\n" + colorCyan + "═══ AUDIT LOG ═══" + colorReset)
	fmt.Println("All access attempts have been logged:")

	events := security.GetAuditLog()
	// Show last 5 events
	start := 0
	if len(events) > 5 {
		start = len(events) - 5
	}

	for i := start; i < len(events); i++ {
		event := events[i]
		fmt.Printf("\n%s[%s]%s\n", colorGray, event.Timestamp.Format("15:04:05"), colorReset)
		fmt.Printf("  User: %s%s%s\n", colorWhite, event.UserID, colorReset)
		fmt.Printf("  Action: %s\n", event.Action)
		fmt.Printf("  Target: %v\n", event.Target)
	}

	if len(events) > 5 {
		fmt.Printf("\n%s... and %d more events%s\n", colorGray, len(events)-5, colorReset)
	}

	return nil
}

func (s *SecurityExample) Benchmark(b *testing.B) error {
	// This would be implemented if we needed in-process benchmarking
	// For now, we use go test -bench
	return nil
}

// Helper function for demos
func waitForEnter() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(colorGray + "\nPress Enter to continue..." + colorReset)
	reader.ReadString('\n')
}
