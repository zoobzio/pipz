package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var composabilityCmd = &cobra.Command{
	Use:   "composability",
	Short: "Pipeline Composability demonstration",
	Long:  `Demonstrates how multiple teams can independently contribute to the same pipeline.`,
	Run:   runComposabilityDemo,
}

func init() {
	rootCmd.AddCommand(composabilityCmd)
}

// SecurityRequest represents a request that needs multi-team security processing
type SecurityRequest struct {
	UserID      string
	Action      string
	Resource    string
	IPAddress   string
	Timestamp   time.Time
	Data        map[string]interface{}
	
	// Added by different teams
	Authenticated bool
	Authorized    bool
	RateLimited   bool
	Compliant     bool
	Logged        bool
	Redacted      bool
}

// Team 1: Authentication Team
func checkAuthentication(req SecurityRequest) ([]byte, error) {
	// Simulate token validation
	if req.UserID == "" {
		return nil, fmt.Errorf("401: unauthenticated request")
	}
	req.Authenticated = true
	return pipz.Encode(req)
}

func validateSession(req SecurityRequest) ([]byte, error) {
	// Ensure session is valid
	if !req.Authenticated {
		return nil, fmt.Errorf("invalid session")
	}
	// Read-only check
	return nil, nil
}

// Team 2: Authorization Team  
func checkAuthorization(req SecurityRequest) ([]byte, error) {
	// Check if user can perform action on resource
	if !req.Authenticated {
		return nil, fmt.Errorf("must authenticate first")
	}
	
	// Simple RBAC check
	if req.Action == "DELETE" && !strings.Contains(req.UserID, "admin") {
		return nil, fmt.Errorf("403: forbidden - admins only")
	}
	
	req.Authorized = true
	return pipz.Encode(req)
}

func enforceResourceLimits(req SecurityRequest) ([]byte, error) {
	// Check resource-specific limits
	if req.Resource == "/api/expensive-operation" && !req.Authorized {
		return nil, fmt.Errorf("unauthorized for expensive operations")
	}
	return nil, nil // Read-only
}

// Team 3: Platform/Infrastructure Team
func rateLimitSecurity(req SecurityRequest) ([]byte, error) {
	// Simple rate limiting by IP
	if req.IPAddress == "10.0.0.99" { // Simulate rate limited IP
		return nil, fmt.Errorf("429: rate limit exceeded")
	}
	req.RateLimited = false
	return pipz.Encode(req)
}

func logSecurityRequest(req SecurityRequest) ([]byte, error) {
	// Log all authenticated requests
	if req.Authenticated {
		req.Logged = true
		// In real app: write to log aggregator
		return pipz.Encode(req)
	}
	return nil, nil
}

// Team 4: Compliance Team
func checkCompliance(req SecurityRequest) ([]byte, error) {
	// GDPR/HIPAA compliance checks
	if req.Resource == "/api/health-records" && req.UserID != "doctor-123" {
		return nil, fmt.Errorf("compliance violation: health data access restricted")
	}
	req.Compliant = true
	return pipz.Encode(req)
}

func redactSensitiveData(req SecurityRequest) ([]byte, error) {
	// Redact PII from response data
	if data, ok := req.Data["ssn"]; ok && data != "" {
		req.Data["ssn"] = "XXX-XX-" + data.(string)[7:]
		req.Redacted = true
		return pipz.Encode(req)
	}
	return nil, nil // No changes needed
}

func runComposabilityDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🔗 PIPELINE COMPOSABILITY DEMO")
	
	pp.SubSection("📋 Scenario: Multi-Team Security Pipeline")
	pp.Info("A security audit pipeline where different teams own different aspects:")
	pp.Info("  • Authentication Team: Token validation, session management")
	pp.Info("  • Authorization Team: RBAC, resource permissions")
	pp.Info("  • Platform Team: Rate limiting, request logging")  
	pp.Info("  • Compliance Team: GDPR/HIPAA, PII redaction")
	
	pp.SubSection("🔧 The Problem with Traditional Approaches")
	pp.Info("Traditional approach: One team owns the entire pipeline")
	pp.Code("go", `// Monolithic security pipeline (hard to maintain)
func ProcessSecurityRequest(req Request) (Response, error) {
    // Team A's code
    if !checkAuth(req) { return nil, errors.New("401") }
    
    // Team B's code mixed in
    if !checkPermissions(req) { return nil, errors.New("403") }
    
    // Team C's code mixed in  
    if rateLimited(req) { return nil, errors.New("429") }
    
    // Team D's code mixed in
    redactPII(&req)
    
    return process(req)
}`)
	
	pp.Info("")
	pp.Info("Problems:")
	pp.Info("  ❌ Teams step on each other's code")
	pp.Info("  ❌ Merge conflicts and coordination overhead")
	pp.Info("  ❌ Can't deploy team changes independently")
	pp.Info("  ❌ Testing requires full pipeline")
	
	pp.WaitForEnter("")
	
	pp.SubSection("✨ The Composable Pipeline Solution")
	pp.Info("Each team independently registers their processors:")
	
	// Define the security pipeline key
	type SecurityKey string
	const securityPipeline SecurityKey = "security-audit-v1"
	
	pp.Info("")
	pp.Info("1️⃣ Authentication Team registers their processors:")
	pp.Code("go", `// auth_team.go
contract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
contract.Register(
    checkAuthentication,
    validateSession,
)`)
	
	// Register auth processors
	authContract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
	authContract.Register(
		checkAuthentication,
		validateSession,
	)
	pp.Success("✓ Auth processors registered")
	
	pp.Info("")
	pp.Info("2️⃣ Authorization Team adds their processors:")
	pp.Code("go", `// authz_team.go  
contract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
contract.Register(
    checkAuthorization,
    enforceResourceLimits,
)`)
	
	// Register authz processors
	authzContract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
	authzContract.Register(
		checkAuthorization,
		enforceResourceLimits,
	)
	pp.Success("✓ AuthZ processors appended to pipeline")
	
	pp.Info("")
	pp.Info("3️⃣ Platform Team adds infrastructure concerns:")
	pp.Code("go", `// platform_team.go
contract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
contract.Register(
    rateLimitSecurity,
    logSecurityRequest,
)`)
	
	// Register platform processors
	platformContract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
	platformContract.Register(
		rateLimitSecurity,
		logSecurityRequest,
	)
	pp.Success("✓ Platform processors appended to pipeline")
	
	pp.Info("")
	pp.Info("4️⃣ Compliance Team adds their processors:")
	pp.Code("go", `// compliance_team.go
contract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
contract.Register(
    checkCompliance,
    redactSensitiveData,
)`)
	
	// Register compliance processors
	complianceContract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
	complianceContract.Register(
		checkCompliance,
		redactSensitiveData,
	)
	pp.Success("✓ Compliance processors appended to pipeline")
	
	pp.WaitForEnter("")
	
	pp.SubSection("🔍 Testing the Composed Pipeline")
	pp.Info("Any team can use the complete pipeline:")
	
	// Test different scenarios
	scenarios := []struct {
		name string
		req  SecurityRequest
	}{
		{
			name: "Valid Admin Request",
			req: SecurityRequest{
				UserID:    "admin-123",
				Action:    "DELETE",
				Resource:  "/api/users/456",
				IPAddress: "10.0.0.1",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"ssn": "123-45-6789"},
			},
		},
		{
			name: "Non-Admin DELETE (Should Fail)",
			req: SecurityRequest{
				UserID:    "user-789",
				Action:    "DELETE", 
				Resource:  "/api/users/456",
				IPAddress: "10.0.0.2",
				Timestamp: time.Now(),
			},
		},
		{
			name: "Rate Limited Request",
			req: SecurityRequest{
				UserID:    "user-123",
				Action:    "GET",
				Resource:  "/api/data",
				IPAddress: "10.0.0.99", // Rate limited IP
				Timestamp: time.Now(),
			},
		},
		{
			name: "Compliance Violation",
			req: SecurityRequest{
				UserID:    "user-456",
				Action:    "GET",
				Resource:  "/api/health-records",
				IPAddress: "10.0.0.3",
				Timestamp: time.Now(),
			},
		},
	}
	
	// Process each scenario
	anyContract := pipz.GetContract[SecurityKey, SecurityRequest](securityPipeline)
	
	for i, scenario := range scenarios {
		pp.Info(fmt.Sprintf("\nScenario %d: %s", i+1, scenario.name))
		pp.Info(fmt.Sprintf("  User: %s, Action: %s %s", 
			scenario.req.UserID, scenario.req.Action, scenario.req.Resource))
		
		result, err := anyContract.Process(scenario.req)
		if err != nil {
			pp.Error(fmt.Sprintf("  ❌ Blocked: %v", err))
		} else {
			pp.Success("  ✅ Allowed")
			pp.Info(fmt.Sprintf("  Auth: %v, AuthZ: %v, Logged: %v, Compliant: %v",
				result.Authenticated, result.Authorized, result.Logged, result.Compliant))
			if result.Redacted {
				pp.Info(fmt.Sprintf("  🔒 PII Redacted: ssn=%v", result.Data["ssn"]))
			}
		}
	}
	
	pp.WaitForEnter("")
	
	pp.SubSection("🎯 Benefits of Composability")
	
	pp.Feature("🔧", "Team Independence", "Each team owns their processors")
	pp.Feature("🔄", "Dynamic Composition", "Pipeline assembles from all registered processors")
	pp.Feature("📦", "Modular Deployment", "Teams can deploy independently")
	pp.Feature("🧪", "Isolated Testing", "Test your processors without the full pipeline")
	pp.Feature("🚀", "Performance Optimized", "Each Register creates one combined processor")
	
	pp.SubSection("Key Insights")
	pp.Info("1. Order matters - processors execute in registration order")
	pp.Info("2. Each team's processors are wrapped in a single byte processor")
	pp.Info("3. No code conflicts - teams work in separate files")
	pp.Info("4. Pipeline behavior emerges from composition")
	pp.Info("5. Easy to add/remove/reorder functionality")
	
	pp.SubSection("🔬 Performance Characteristics")
	pp.Info("For our 4-team pipeline with 8 processors:")
	pp.Info("  • Traditional: 8 encode/decode operations")
	pp.Info("  • Composable: 4 encode/decode operations (one per team)")
	pp.Info("  • 50% reduction in serialization overhead!")
	
	pp.Stats("Pipeline Composition", map[string]interface{}{
		"Teams Contributing": 4,
		"Total Processors": 8,
		"Byte Processors": 4,
		"Serialization Ops": "4 vs 8",
		"Performance Gain": "~50%",
	})
}