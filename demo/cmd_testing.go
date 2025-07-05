package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var testingCmd = &cobra.Command{
	Use:   "testing",
	Short: "Testing patterns demonstration",
	Long:  `Demonstrates how type universes make testing elegant without mocks.`,
	Run:   runTestingDemo,
}

func init() {
	rootCmd.AddCommand(testingCmd)
}

// Demo types
type EmailServiceKey string

type EmailMessage struct {
	ID        string
	To        string
	From      string
	Subject   string
	Body      string
	Timestamp time.Time
	
	// Processing results
	Validated     bool
	Sanitized     bool
	TemplateUsed  string
	Provider      string
	SentAt        time.Time
	TestMode      bool
}

// Test tracking
var (
	testEmails = sync.Map{} // Track emails sent in test mode
	prodEmails = sync.Map{} // Track emails sent in prod mode
)

// Production processors
func validateEmail(msg EmailMessage) ([]byte, error) {
	// Real validation
	if msg.To == "" {
		return nil, fmt.Errorf("recipient required")
	}
	if !strings.Contains(msg.To, "@") {
		return nil, fmt.Errorf("invalid email format")
	}
	msg.Validated = true
	return pipz.Encode(msg)
}

func sanitizeContent(msg EmailMessage) ([]byte, error) {
	// Remove potential XSS
	msg.Body = strings.ReplaceAll(msg.Body, "<script>", "&lt;script&gt;")
	msg.Body = strings.ReplaceAll(msg.Body, "</script>", "&lt;/script&gt;")
	msg.Sanitized = true
	return pipz.Encode(msg)
}

func applyTemplate(msg EmailMessage) ([]byte, error) {
	// Apply branding template
	if strings.Contains(msg.To, "vip") {
		msg.TemplateUsed = "premium"
		msg.Body = "🌟 VIP MESSAGE 🌟\n\n" + msg.Body
	} else {
		msg.TemplateUsed = "standard"
		msg.Body = "Thank you for using our service!\n\n" + msg.Body
	}
	return pipz.Encode(msg)
}

func sendViaProvider(msg EmailMessage) ([]byte, error) {
	// Real email sending
	time.Sleep(50 * time.Millisecond) // Simulate API call
	
	// Would normally call SendGrid, SES, etc.
	msg.Provider = "sendgrid"
	msg.SentAt = time.Now()
	
	// Track for demo
	prodEmails.Store(msg.ID, msg)
	
	return pipz.Encode(msg)
}

// Test processors (same signatures, different behavior)
func mockSend(msg EmailMessage) ([]byte, error) {
	// Don't actually send, just record
	msg.Provider = "mock"
	msg.SentAt = time.Now()
	msg.TestMode = true
	
	// Track test emails
	testEmails.Store(msg.ID, msg)
	
	return pipz.Encode(msg)
}

func testTemplate(msg EmailMessage) ([]byte, error) {
	// Simplified template for tests
	msg.TemplateUsed = "test"
	msg.Body = "[TEST] " + msg.Body
	return pipz.Encode(msg)
}

// Assertion helper for tests
func assertEmailSent(id string) (EmailMessage, bool) {
	if val, ok := testEmails.Load(id); ok {
		return val.(EmailMessage), true
	}
	return EmailMessage{}, false
}

func runTestingDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🧪 TESTING PATTERNS DEMO")
	
	pp.SubSection("📋 Use Case: Email Service Testing")
	pp.Info("Challenge: How do you test email sending without:")
	pp.Info("  ❌ Actually sending emails")
	pp.Info("  ❌ Complex mocking frameworks")
	pp.Info("  ❌ Dependency injection")
	pp.Info("  ❌ Test doubles")
	pp.Info("")
	pp.Info("Solution: Use type universes! 🌌")
	
	pp.SubSection("🔧 The Pattern: Test vs Production Universes")
	pp.Code("go", `// Production pipeline
prodContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("prod"))
prodContract.Register(
    validateEmail,    // Real validation
    sanitizeContent,  // Real sanitization
    applyTemplate,    // Real templates
    sendViaProvider,  // Real sending (SendGrid, SES, etc)
)

// Test pipeline - same types, different universe!
testContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("test"))
testContract.Register(
    validateEmail,    // Reuse real validation
    sanitizeContent,  // Reuse real sanitization
    testTemplate,     // Simplified template for tests
    mockSend,         // Don't actually send!
)

// In your tests, just use the test contract
// No mocking frameworks needed!`)
	
	pp.Info("")
	pp.Info("💡 Key insight: Tests and production are just different universes!")
	pp.Info("   Same types, different behavior, perfect isolation.")
	
	// Register pipelines
	pp.SubSection("Step 1: Register Both Pipelines")
	
	// Production pipeline
	prodContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("prod"))
	prodContract.Register(validateEmail, sanitizeContent, applyTemplate, sendViaProvider)
	pp.Success("✓ Production pipeline registered")
	
	// Test pipeline
	testContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("test"))
	testContract.Register(validateEmail, sanitizeContent, testTemplate, mockSend)
	pp.Success("✓ Test pipeline registered")
	
	pp.Info("")
	pp.Info("Notice: We reuse real processors (validate, sanitize)")
	pp.Info("        But swap out external calls (template, send)")
	
	pp.SubSection("🔍 Example 1: Unit Testing")
	
	pp.Info("Test Case: Validate email rejection")
	testMsg1 := EmailMessage{
		ID:      "test-001",
		To:      "not-an-email",
		Subject: "Test",
		Body:    "Hello",
	}
	
	_, err := testContract.Process(testMsg1)
	if err != nil {
		pp.Success(fmt.Sprintf("✓ Test passed: %v", err))
	} else {
		pp.Error("✗ Test failed: Should have rejected invalid email")
	}
	
	pp.Info("")
	pp.Info("Test Case: Valid email processing")
	testMsg2 := EmailMessage{
		ID:      "test-002",
		To:      "user@example.com",
		Subject: "Welcome",
		Body:    "Thanks for <script>alert('xss')</script> signing up!",
	}
	
	result, err := testContract.Process(testMsg2)
	if err != nil {
		pp.Error(fmt.Sprintf("✗ Test failed: %v", err))
	} else {
		pp.Success("✓ Email processed successfully")
		pp.Info(fmt.Sprintf("  Validated: %v", result.Validated))
		pp.Info(fmt.Sprintf("  Sanitized: %v", result.Sanitized))
		pp.Info(fmt.Sprintf("  XSS removed: %v", !strings.Contains(result.Body, "<script>")))
		pp.Info(fmt.Sprintf("  Template: %s", result.TemplateUsed))
		pp.Info(fmt.Sprintf("  Actually sent: %v", result.Provider != "mock"))
		
		// Verify it was recorded in test mode
		if stored, ok := assertEmailSent("test-002"); ok {
			pp.Success("✓ Email recorded in test tracker")
			pp.Info(fmt.Sprintf("  Test mode: %v", stored.TestMode))
		}
	}
	
	pp.WaitForEnter("")
	
	pp.SubSection("🔍 Example 2: Integration Testing")
	
	pp.Info("Test multiple emails through the pipeline:")
	pp.Info("")
	
	testBatch := []EmailMessage{
		{ID: "batch-001", To: "regular@example.com", Subject: "Order Confirmation"},
		{ID: "batch-002", To: "vip@example.com", Subject: "VIP Welcome"},
		{ID: "batch-003", To: "admin@example.com", Subject: "System Alert"},
	}
	
	for _, msg := range testBatch {
		result, _ := testContract.Process(msg)
		pp.Info(fmt.Sprintf("Email %s:", msg.ID))
		pp.Info(fmt.Sprintf("  To: %s", result.To))
		pp.Info(fmt.Sprintf("  Template: %s", result.TemplateUsed))
		pp.Info(fmt.Sprintf("  Body preview: %s", strings.Split(result.Body, "\n")[0]))
	}
	
	// Count test emails
	testCount := 0
	testEmails.Range(func(_, _ interface{}) bool {
		testCount++
		return true
	})
	
	pp.Info("")
	pp.Success(fmt.Sprintf("✓ %d emails processed in test mode", testCount))
	pp.Info("  Zero emails actually sent!")
	
	pp.SubSection("🔍 Example 3: A/B Testing Pattern")
	
	pp.Info("Compare test vs production behavior:")
	pp.Info("")
	
	compareMsg := EmailMessage{
		ID:      "compare-001",
		To:      "customer@example.com",
		Subject: "Special Offer",
		Body:    "Check out our deals!",
	}
	
	// Process through both pipelines
	start := time.Now()
	testResult, _ := testContract.Process(compareMsg)
	testTime := time.Since(start)
	
	compareMsg.ID = "compare-002" // Different ID for tracking
	start = time.Now()
	prodResult, _ := prodContract.Process(compareMsg)
	prodTime := time.Since(start)
	
	pp.Stats("Pipeline Comparison", map[string]interface{}{
		"Test Time":     testTime,
		"Prod Time":     prodTime,
		"Test Provider": testResult.Provider,
		"Prod Provider": prodResult.Provider,
		"Test Template": testResult.TemplateUsed,
		"Prod Template": prodResult.TemplateUsed,
	})
	
	pp.Info("")
	pp.Info("Notice: Test pipeline is much faster (no API calls)")
	
	pp.SubSection("🔧 Advanced Testing Patterns")
	
	pp.Code("go", `// Pattern 1: Partial mocking
hybridContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("hybrid"))
hybridContract.Register(
    validateEmail,    // Real
    sanitizeContent,  // Real
    applyTemplate,    // Real
    mockSend,         // Mocked - test templates without sending
)

// Pattern 2: Failure injection
errorContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("error-test"))
errorContract.Register(
    validateEmail,
    alwaysFail,  // Inject failures to test error handling
)

// Pattern 3: Performance testing
perfContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("perf"))
perfContract.Register(
    skipValidation,  // Remove slow steps
    mockSend,        // No external calls
)

// Pattern 4: Snapshot testing
snapshotContract := pipz.GetContract[EmailServiceKey, EmailMessage](EmailServiceKey("snapshot"))
snapshotContract.Register(
    validateEmail,
    sanitizeContent,
    applyTemplate,
    saveSnapshot,  // Save output for comparison
)`)
	
	pp.SubSection("🎯 Testing Benefits with pipz")
	
	pp.Feature("🔒", "True Isolation", "Tests can't affect production")
	pp.Feature("🚀", "No Mocks", "Just different processor implementations")
	pp.Feature("♻️", "Reuse Code", "Mix real and test processors freely")
	pp.Feature("🎭", "Multiple Modes", "Test, staging, production - all isolated")
	pp.Feature("📊", "Easy Assertions", "Track calls without frameworks")
	
	pp.SubSection("The Testing Philosophy")
	pp.Info("Traditional: Mock everything, inject dependencies")
	pp.Info("pipz: Create a test universe")
	pp.Info("")
	pp.Info("Traditional: Complex mocking frameworks")
	pp.Info("pipz: Just write different processors")
	pp.Info("")
	pp.Info("Traditional: Test doubles, stubs, spies")
	pp.Info("pipz: Just different contract keys")
	pp.Info("")
	pp.Info("The same pattern that enables A/B testing in production")
	pp.Info("enables elegant testing in development!")
	
	pp.SubSection("💡 Real-World Testing Scenarios")
	
	pp.Info("This pattern works for any external dependency:")
	pp.Info("  • Database calls → In-memory test processors")
	pp.Info("  • API calls → Return canned responses")
	pp.Info("  • File I/O → Use temp files or memory")
	pp.Info("  • Time-based → Control time in tests")
	pp.Info("  • Random → Use deterministic seeds")
	pp.Info("")
	pp.Info("Each test scenario is just another universe!")
	
	// Final stats
	prodCount := 0
	prodEmails.Range(func(_, _ interface{}) bool {
		prodCount++
		return true
	})
	
	pp.Stats("Demo Summary", map[string]interface{}{
		"Test Emails":    testCount,
		"Prod Emails":    prodCount,
		"Mocking Libs":   0,
		"DI Containers":  0,
		"Type Safety":    "100%",
	})
}