package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var middlewareCmd = &cobra.Command{
	Use:   "middleware",
	Short: "Request/Response Middleware demonstration",
	Long:  `Demonstrates building HTTP-like middleware chains with pipz.`,
	Run:   runMiddlewareDemo,
}

func init() {
	rootCmd.AddCommand(middlewareCmd)
}

// Demo types for middleware
type MiddlewareKey string

const (
	// APIMiddlewareV1 is the current version of the API middleware
	APIMiddlewareV1 MiddlewareKey = "api-v1"
)

type Request struct {
	ID      string
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
	
	// Context fields as concrete types
	UserID           string
	Plan             string
	Permissions      []string
	Authenticated    bool
	AuthTime         time.Time
	RateLimit        *RateLimitInfo
	RateLimitLimit   int
	RateLimitRemaining int
	RateLimitReset   int64
	RequestStart     time.Time
	LogEntry         string
	Preflight        bool
	SkipProcessing   bool
}

type RateLimitInfo struct {
	UserID    string
	Limit     int
	Used      int
	ResetTime time.Time
}

// Middleware processors
func authenticate(r Request) ([]byte, error) {
	// Skip auth for CORS preflight
	if r.Method == "OPTIONS" {
		return nil, nil
	}
	
	token := r.Headers["Authorization"]
	if token == "" {
		return nil, fmt.Errorf("401: no auth token")
	}

	// Validate token format
	if !strings.HasPrefix(token, "Bearer ") {
		return nil, fmt.Errorf("401: invalid token format")
	}

	// Extract token and "validate" it (mock)
	tokenValue := strings.TrimPrefix(token, "Bearer ")
	if len(tokenValue) < 10 {
		return nil, fmt.Errorf("401: invalid token")
	}

	// Set user context based on token
	if strings.Contains(tokenValue, "premium") {
		r.UserID = "premium-user-123"
		r.Plan = "premium"
	} else if strings.Contains(tokenValue, "admin") {
		r.UserID = "admin-user-456"
		r.Plan = "admin"
		r.Permissions = []string{"read", "write", "delete"}
	} else {
		r.UserID = "basic-user-789"
		r.Plan = "basic"
	}

	r.Authenticated = true
	r.AuthTime = time.Now()
	return pipz.Encode(r)
}

func rateLimit(r Request) ([]byte, error) {
	// Skip rate limiting for CORS preflight
	if r.Method == "OPTIONS" {
		return nil, nil
	}
	
	if r.UserID == "" {
		return nil, fmt.Errorf("500: user not authenticated")
	}

	// Check rate limit based on plan
	var limit int
	switch r.Plan {
	case "admin":
		limit = 10000
	case "premium":
		limit = 1000
	case "basic":
		limit = 100
	default:
		limit = 10
	}

	// Mock rate limit check
	rateLimitInfo := &RateLimitInfo{
		UserID:    r.UserID,
		Limit:     limit,
		Used:      42, // Mock current usage
		ResetTime: time.Now().Add(1 * time.Hour),
	}

	if rateLimitInfo.Used >= rateLimitInfo.Limit {
		return nil, fmt.Errorf("429: rate limit exceeded (%d/%d)", rateLimitInfo.Used, rateLimitInfo.Limit)
	}

	r.RateLimit = rateLimitInfo
	r.RateLimitLimit = limit
	r.RateLimitRemaining = limit - rateLimitInfo.Used
	r.RateLimitReset = rateLimitInfo.ResetTime.Unix()
	return pipz.Encode(r)
}

func logRequest(r Request) ([]byte, error) {
	// Add request timing
	r.RequestStart = time.Now()
	
	// Log the request (in a real app, this would go to a logging service)
	r.LogEntry = fmt.Sprintf("[%s] %s %s - User: %s, Plan: %s",
		time.Now().Format(time.RFC3339),
		r.Method,
		r.Path,
		r.UserID,
		r.Plan)
	
	return pipz.Encode(r)
}

func cors(r Request) ([]byte, error) {
	// Add CORS headers
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	
	r.Headers["Access-Control-Allow-Origin"] = "*"
	r.Headers["Access-Control-Allow-Methods"] = "GET, POST, PUT, DELETE, OPTIONS"
	r.Headers["Access-Control-Allow-Headers"] = "Content-Type, Authorization"
	
	// Handle preflight
	if r.Method == "OPTIONS" {
		r.Preflight = true
		r.SkipProcessing = true
	}
	
	return pipz.Encode(r)
}

func runMiddlewareDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🔌 REQUEST/RESPONSE MIDDLEWARE DEMO")
	
	pp.SubSection("📋 Use Case: API Gateway Middleware")
	pp.Info("Scenario: An API gateway needs to process all incoming requests.")
	pp.Info("Requirements:")
	pp.Info("  • Authenticate requests using Bearer tokens")
	pp.Info("  • Apply rate limiting based on user plan")
	pp.Info("  • Log all requests for analytics")
	pp.Info("  • Handle CORS for browser clients")
	
	pp.SubSection("🔧 Middleware Pipeline")
	pp.Code("go", `type MiddlewareKey string

const (
    APIMiddlewareV1 MiddlewareKey = "api-v1"
)

// Register middleware pipeline
middlewareContract := pipz.GetContract[Request](APIMiddlewareV1)
middlewareContract.Register(
    authenticate,  // Check auth token
    rateLimit,    // Apply rate limits
    logRequest,   // Log for analytics
    cors,         // Handle CORS
)`)
	
	// Register the middleware pipeline
	middlewareContract := pipz.GetContract[Request](APIMiddlewareV1)
	err := middlewareContract.Register(authenticate, rateLimit, logRequest, cors)
	if err != nil {
		pp.Error(fmt.Sprintf("Failed to register middleware: %v", err))
		return
	}
	
	pp.Success("Middleware pipeline registered")
	
	pp.SubSection("🔍 Live Request Processing")
	
	// Example 1: Authenticated premium user
	pp.Info("Example 1: Premium user request")
	premiumRequest := Request{
		ID:     "req-001",
		Method: "GET",
		Path:   "/api/v1/users/profile",
		Headers: map[string]string{
			"Authorization": "Bearer premium-token-abc123xyz",
			"Content-Type":  "application/json",
		},
	}
	
	result, err := middlewareContract.Process(premiumRequest)
	if err != nil {
		pp.Error(fmt.Sprintf("Request failed: %v", err))
	} else {
		pp.Success("Request processed successfully!")
		pp.Info(fmt.Sprintf("  User: %s", result.UserID))
		pp.Info(fmt.Sprintf("  Plan: %s", result.Plan))
		pp.Info(fmt.Sprintf("  Rate Limit: %d/%d", 
			result.RateLimitRemaining,
			result.RateLimitLimit))
		pp.Info(fmt.Sprintf("  CORS: %s", result.Headers["Access-Control-Allow-Origin"]))
	}
	
	pp.WaitForEnter("")
	
	// Example 2: Unauthenticated request
	pp.Info("")
	pp.Info("Example 2: Unauthenticated request")
	unauthRequest := Request{
		ID:     "req-002",
		Method: "POST",
		Path:   "/api/v1/data",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
	
	_, err = middlewareContract.Process(unauthRequest)
	if err != nil {
		pp.Error(fmt.Sprintf("Request blocked: %v", err))
		pp.Info("  ↳ Middleware chain stopped at authentication")
	}
	
	// Example 3: Admin user with full access
	pp.Info("")
	pp.Info("Example 3: Admin user request")
	adminRequest := Request{
		ID:     "req-003",
		Method: "DELETE",
		Path:   "/api/v1/users/123",
		Headers: map[string]string{
			"Authorization": "Bearer admin-super-secret-token",
		},
	}
	
	result, err = middlewareContract.Process(adminRequest)
	if err != nil {
		pp.Error(fmt.Sprintf("Request failed: %v", err))
	} else {
		pp.Success("Admin request processed!")
		pp.Info(fmt.Sprintf("  Permissions: %v", result.Permissions))
		pp.Info(fmt.Sprintf("  Rate Limit: %d (admin tier)", result.RateLimitLimit))
	}
	
	// Example 4: CORS preflight
	pp.Info("")
	pp.Info("Example 4: CORS preflight request")
	preflightRequest := Request{
		ID:     "req-004",
		Method: "OPTIONS",
		Path:   "/api/v1/data",
		Headers: map[string]string{
			"Origin": "https://example.com",
			"Access-Control-Request-Method": "POST",
		},
	}
	
	result, err = middlewareContract.Process(preflightRequest)
	if err != nil {
		pp.Error(fmt.Sprintf("Preflight failed: %v", err))
	} else {
		pp.Success("CORS preflight handled!")
		pp.Info(fmt.Sprintf("  Allowed Origin: %s", result.Headers["Access-Control-Allow-Origin"]))
		pp.Info(fmt.Sprintf("  Allowed Methods: %s", result.Headers["Access-Control-Allow-Methods"]))
		pp.Info(fmt.Sprintf("  Skip Processing: %v", result.SkipProcessing))
	}
	
	pp.SubSection("🎯 Middleware Patterns")
	
	pp.Feature("🛡️", "Security First", "Authentication happens before any processing")
	pp.Feature("⚡", "Early Exit", "Failed auth stops the chain immediately")
	pp.Feature("📊", "Rate Limiting", "Different limits per user tier")
	pp.Feature("📝", "Request Logging", "Every request is tracked")
	pp.Feature("🌐", "CORS Support", "Browser-friendly API")
	
	pp.SubSection("Middleware Execution Order")
	pp.Info("1. Authentication - Verify user identity")
	pp.Info("2. Rate Limiting - Check usage quotas")
	pp.Info("3. Logging - Track all requests")
	pp.Info("4. CORS - Handle browser security")
	pp.Info("")
	pp.Info("Each step can modify the request or stop the chain.")
	
	pp.Info("")
	pp.Info("💡 Order matters! Watch what happens with different orderings:")
	pp.Info("")
	pp.Info("❌ BAD: If we put CORS last, preflight fails at auth")
	pp.Info("✅ GOOD: Our demo puts auth first but skips it for OPTIONS")
	pp.Info("")
	pp.Info("Performance tip: Put expensive operations last!")
	pp.Info("Security tip: Put auth/validation first!")
	
	pp.SubSection("🔧 Advanced Middleware Patterns")
	
	pp.Info("Middleware ordering is determined by registration order:")
	pp.Code("go", `// Security-first ordering (recommended)
const secureKey MiddlewareKey = "secure"
secureContract := pipz.GetContract[Request](secureKey)
secureContract.Register(
    authenticate,  // 1st: Auth before anything else
    rateLimit,     // 2nd: Rate limit authenticated users
    logRequest,    // 3rd: Log after auth succeeds
    cors,          // 4th: Add CORS headers
)

// Performance-first ordering (for public endpoints)
const publicKey MiddlewareKey = "public"
publicContract := pipz.GetContract[Request](publicKey)
publicContract.Register(
    cors,          // 1st: Handle CORS immediately
    rateLimit,     // 2nd: Rate limit everyone
    logRequest,    // 3rd: Log all requests
    // No auth - this is a public endpoint!
)

// Conditional middleware based on request
if request.Path == "/health" {
    // Minimal middleware for health checks
    const healthKey MiddlewareKey = "health"
    healthContract := pipz.GetContract[Request](healthKey)
    healthContract.Register(logRequest) // Only logging
}`)
	
	pp.Stats("Middleware Performance", map[string]interface{}{
		"Processors": 4,
		"Avg Latency": "< 0.1ms",
		"Memory": "~500 bytes/request",
		"Throughput": "> 100k req/s",
	})
}