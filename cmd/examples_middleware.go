package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/middleware"
)

// MiddlewareExample implements the Example interface for HTTP middleware
type MiddlewareExample struct{}

func (m *MiddlewareExample) Name() string {
	return "middleware"
}

func (m *MiddlewareExample) Description() string {
	return "HTTP middleware patterns for request/response processing"
}

func (m *MiddlewareExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ HTTP MIDDLEWARE EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Authentication and authorization")
	fmt.Println("• Request validation and sanitization")
	fmt.Println("• Rate limiting per user/IP")
	fmt.Println("• Request/response logging")
	fmt.Println("• Security headers and audit trails")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("HTTP middleware quickly becomes tangled:")
	fmt.Println(colorGray + `
func handler(w http.ResponseWriter, r *http.Request) {
    // Logging
    start := time.Now()
    log.Printf("Request: %s %s", r.Method, r.URL.Path)
    
    // Authentication
    token := r.Header.Get("Authorization")
    if token == "" {
        http.Error(w, "Unauthorized", 401)
        return
    }
    
    user, err := validateToken(token)
    if err != nil {
        http.Error(w, "Invalid token", 401)
        return
    }
    
    // Rate limiting
    key := fmt.Sprintf("rl:%s", user.ID)
    if !rateLimiter.Allow(key) {
        http.Error(w, "Rate limit exceeded", 429)
        return
    }
    
    // Request validation
    if r.ContentLength > maxBodySize {
        http.Error(w, "Request too large", 413)
        return
    }
    
    // Authorization
    if !user.HasPermission(r.URL.Path) {
        http.Error(w, "Forbidden", 403)
        return
    }
    
    // Process request...
    // Error handling...
    // Response logging...
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Use the real middleware example with proper types:")
	fmt.Println(colorGray + `
// Import the real middleware package
import "github.com/zoobzio/pipz/examples/middleware"

// Use the actual Request type and processors
standardMiddleware := middleware.CreateStandardMiddleware()
adminMiddleware := middleware.CreateAdminMiddleware()
publicMiddleware := middleware.CreatePublicMiddleware()

// Process HTTP requests with full middleware stack
result, err := standardMiddleware.Process(ctx, request)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's process some HTTP requests!" + colorReset)
	return m.runInteractive(ctx)
}

func (m *MiddlewareExample) runInteractive(ctx context.Context) error {
	// Create different middleware pipelines using the real middleware example
	publicPipeline := middleware.CreatePublicMiddleware()
	standardPipeline := middleware.CreateStandardMiddleware()
	adminPipeline := middleware.CreateAdminMiddleware()

	// Test scenarios
	scenarios := []struct {
		name     string
		pipeline *pipz.Pipeline[middleware.Request]
		request  middleware.Request
	}{
		{
			name:     "Public Endpoint - Health Check",
			pipeline: publicPipeline,
			request: middleware.Request{
				Method:    "GET",
				Path:      "/health",
				Headers:   map[string]string{"User-Agent": "test-client/1.0"},
				ClientIP:  "192.168.1.100",
				UserAgent: "test-client/1.0",
				RequestID: "req_001",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Protected Endpoint - No Auth",
			pipeline: standardPipeline,
			request: middleware.Request{
				Method:    "GET",
				Path:      "/api/profile",
				Headers:   map[string]string{"User-Agent": "test-client/1.0"},
				ClientIP:  "192.168.1.100",
				UserAgent: "test-client/1.0",
				RequestID: "req_002",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Protected Endpoint - Valid Auth",
			pipeline: standardPipeline,
			request: middleware.Request{
				Method:    "GET",
				Path:      "/api/profile",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "test-client/1.0",
				},
				ClientIP:  "192.168.1.100",
				UserAgent: "test-client/1.0",
				RequestID: "req_003",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Admin Endpoint - User Token",
			pipeline: adminPipeline,
			request: middleware.Request{
				Method:    "DELETE",
				Path:      "/admin/users/123",
				Headers:   map[string]string{"Authorization": "Bearer valid-token-123"},
				ClientIP:  "192.168.1.100",
				UserAgent: "admin-panel/1.0",
				RequestID: "req_004",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Admin Endpoint - Admin Token",
			pipeline: adminPipeline,
			request: middleware.Request{
				Method:    "DELETE",
				Path:      "/admin/users/123",
				Headers:   map[string]string{"Authorization": "Bearer admin-token-456"},
				ClientIP:  "192.168.1.100",
				UserAgent: "admin-panel/1.0",
				RequestID: "req_005",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Rate Limit Test - Burst Requests",
			pipeline: standardPipeline,
			request: middleware.Request{
				Method:    "POST",
				Path:      "/api/action",
				Headers:   map[string]string{"Authorization": "Bearer valid-token-123"},
				Body:      []byte(`{"action":"test"}`),
				ClientIP:  "192.168.1.100",
				UserAgent: "test-client/1.0",
				RequestID: "req_burst",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Invalid Request - XSS Attempt",
			pipeline: standardPipeline,
			request: middleware.Request{
				Method:    "POST",
				Path:      "/api/comment",
				Headers:   map[string]string{"Authorization": "Bearer valid-token-123"},
				Body:      []byte(`{"comment":"<script>alert('xss')</script>"}`),
				ClientIP:  "192.168.1.100",
				UserAgent: "malicious-client/1.0",
				RequestID: "req_006",
				Timestamp: time.Now(),
			},
		},
		{
			name:     "Premium User Request",
			pipeline: standardPipeline,
			request: middleware.Request{
				Method:    "GET",
				Path:      "/api/premium/feature",
				Headers:   map[string]string{"Authorization": "Bearer premium-token-789"},
				ClientIP:  "192.168.1.100",
				UserAgent: "premium-app/2.0",
				RequestID: "req_007",
				Timestamp: time.Now(),
			},
		},
	}

	// Process scenarios
	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		// Show request details
		fmt.Printf("\nRequest:\n")
		fmt.Printf("  %s %s\n", scenario.request.Method, scenario.request.Path)
		fmt.Printf("  Client IP: %s\n", scenario.request.ClientIP)
		if auth := scenario.request.Headers["Authorization"]; auth != "" {
			fmt.Printf("  Auth: %s\n", auth)
		}
		if len(scenario.request.Body) > 0 {
			fmt.Printf("  Body: %s\n", string(scenario.request.Body))
		}

		// Special handling for rate limit test
		if scenario.name == "Rate Limit Test - Burst Requests" {
			fmt.Printf("\n%sSending burst requests to test rate limiting...%s\n", 
				colorYellow, colorReset)

			successCount := 0
			rateLimitedCount := 0

			for j := 0; j < 8; j++ {
				req := scenario.request
				req.RequestID = fmt.Sprintf("burst_%d", j)
				req.Timestamp = time.Now()

				result, err := scenario.pipeline.Process(ctx, req)

				if err != nil {
					if strings.Contains(err.Error(), "rate limit") {
						rateLimitedCount++
						fmt.Printf("  %s⚠️  Request %d: Rate limited!%s\n",
							colorYellow, j+1, colorReset)
					} else {
						fmt.Printf("  %s❌ Request %d: %s%s\n",
							colorRed, j+1, err.Error(), colorReset)
					}
				} else {
					successCount++
					status := result.StatusCode
					if status == 0 {
						status = 200 // Default success
					}
					fmt.Printf("  %s✅ Request %d: %d%s\n",
						colorGreen, j+1, status, colorReset)
				}

				time.Sleep(100 * time.Millisecond)
			}

			fmt.Printf("\nBurst Results: %d successful, %d rate limited\n",
				successCount, rateLimitedCount)
		} else {
			// Normal request processing
			fmt.Printf("\n%sProcessing...%s\n", colorYellow, colorReset)
			start := time.Now()

			result, err := scenario.pipeline.Process(ctx, scenario.request)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Request Failed%s\n", colorRed, colorReset)
				
				var pipelineErr *pipz.PipelineError[middleware.Request]
				if errors.As(err, &pipelineErr) {
					fmt.Printf("  Failed at: %s%s%s (stage %d)\n",
						colorYellow, pipelineErr.ProcessorName, colorReset,
						pipelineErr.StageIndex)
				}
				
				fmt.Printf("  Error: %s\n", err.Error())
			} else {
				fmt.Printf("\n%s✅ Request Successful%s\n", colorGreen, colorReset)
				
				status := result.StatusCode
				if status == 0 {
					status = 200 // Default success
				}
				fmt.Printf("  Status: %d\n", status)
				
				if result.UserID != "" {
					fmt.Printf("  User: %s\n", result.UserID)
					if len(result.Roles) > 0 {
						fmt.Printf("  Roles: %v\n", result.Roles)
					}
					if result.IsAdmin {
						fmt.Printf("  Admin: %v\n", result.IsAdmin)
					}
				}
				
				if result.RateLimit != nil {
					fmt.Printf("  Rate Limit: %d/%d remaining\n", 
						result.RateLimit.Remaining, result.RateLimit.Limit)
				}
				
				fmt.Printf("  Processing Time: %v\n", duration)
				
				// Show metadata if present
				if len(result.Metadata) > 0 {
					fmt.Printf("  Metadata: %+v\n", result.Metadata)
				}
			}
		}

		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show best practices
	fmt.Printf("\n%s═══ MIDDLEWARE BEST PRACTICES ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\n• Order middleware from least to most expensive\n")
	fmt.Printf("• Fail fast with validation and auth checks\n")
	fmt.Printf("• Use context for request-scoped values\n")
	fmt.Printf("• Implement proper error types for HTTP status codes\n")
	fmt.Printf("• Log requests asynchronously to avoid blocking\n")
	fmt.Printf("• Use rate limiting to protect against abuse\n")
	fmt.Printf("• Always validate and sanitize user input\n")

	return nil
}

func (m *MiddlewareExample) Benchmark(b *testing.B) error {
	return nil
}