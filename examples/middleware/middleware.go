// Package middleware demonstrates how to use pipz for HTTP request processing
// with authentication, rate limiting, and logging middleware.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// Request represents an HTTP request context that flows through middleware
type Request struct {
	Method    string
	Path      string
	Headers   map[string]string
	Body      []byte
	ClientIP  string
	UserAgent string
	RequestID string
	Timestamp time.Time

	// Authentication context
	UserID     string
	Roles      []string
	IsAdmin    bool
	Authorized bool

	// Rate limiting context
	RateLimit *RateLimit

	// Logging context
	StartTime    time.Time
	Duration     time.Duration
	StatusCode   int
	BytesRead    int64
	BytesWritten int64

	// Request metadata
	Metadata map[string]interface{}
	Tags     []string

	// Response context
	Response *Response
}

// Response represents the HTTP response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Cached     bool
}

// RateLimit tracks rate limiting information
type RateLimit struct {
	UserID    string
	Limit     int
	Remaining int
	Reset     time.Time
	Window    time.Duration
}

// User represents a user in the system
type User struct {
	ID       string
	Username string
	Roles    []string
	IsAdmin  bool
	Active   bool
}

// Simple in-memory stores for demonstration
var (
	userStore = map[string]*User{
		"user-123":     {ID: "user-123", Username: "john", Roles: []string{"user"}, IsAdmin: false, Active: true},
		"admin-456":    {ID: "admin-456", Username: "admin", Roles: []string{"admin"}, IsAdmin: true, Active: true},
		"user-789":     {ID: "user-789", Username: "jane", Roles: []string{"user", "premium"}, IsAdmin: false, Active: true},
		"inactive-999": {ID: "inactive-999", Username: "inactive", Roles: []string{"user"}, IsAdmin: false, Active: false},
	}

	tokenStore = map[string]string{
		"valid-token-123":    "user-123",
		"admin-token-456":    "admin-456",
		"premium-token-789":  "user-789",
		"inactive-token-999": "inactive-999",
	}

	rateLimitStore = sync.Map{}
)

// AddMetadata adds metadata to the request
func (r *Request) AddMetadata(key string, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
}

// AddTag adds a tag to the request
func (r *Request) AddTag(tag string) {
	r.Tags = append(r.Tags, tag)
}

// HasRole checks if the user has a specific role
func (r *Request) HasRole(role string) bool {
	for _, userRole := range r.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID(_ context.Context, req Request) (Request, error) {
	// Simple request ID generation - in production, use UUID or similar
	req.RequestID = fmt.Sprintf("req-%d-%s", time.Now().UnixNano(), req.ClientIP)
	req.Timestamp = time.Now()
	req.StartTime = time.Now()
	req.AddMetadata("request_id", req.RequestID)
	return req, nil
}

// ParseHeaders extracts and validates important headers
func ParseHeaders(_ context.Context, req Request) (Request, error) {
	if req.Headers == nil {
		return req, fmt.Errorf("no headers provided")
	}

	// Extract user agent
	if userAgent, exists := req.Headers["User-Agent"]; exists {
		req.UserAgent = userAgent
	}

	// Extract content length
	if contentLen, exists := req.Headers["Content-Length"]; exists {
		if length, err := strconv.ParseInt(contentLen, 10, 64); err == nil {
			req.BytesRead = length
		}
	}

	// Extract client IP from X-Forwarded-For or X-Real-IP
	if forwardedFor, exists := req.Headers["X-Forwarded-For"]; exists {
		// Take the first IP from the list
		if ips := strings.Split(forwardedFor, ","); len(ips) > 0 {
			req.ClientIP = strings.TrimSpace(ips[0])
		}
	} else if realIP, exists := req.Headers["X-Real-IP"]; exists {
		req.ClientIP = realIP
	}

	req.AddMetadata("user_agent", req.UserAgent)
	req.AddMetadata("content_length", req.BytesRead)

	return req, nil
}

// AuthenticateToken validates the authentication token
func AuthenticateToken(_ context.Context, req Request) (Request, error) {
	authHeader, exists := req.Headers["Authorization"]
	if !exists {
		return req, fmt.Errorf("missing authorization header")
	}

	// Extract token from "Bearer <token>" format
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return req, fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]
	if token == "" {
		return req, fmt.Errorf("empty auth token")
	}

	// Look up user ID from token
	userID, exists := tokenStore[token]
	if !exists {
		return req, fmt.Errorf("invalid auth token")
	}

	// Get user details
	user, exists := userStore[userID]
	if !exists {
		return req, fmt.Errorf("user not found")
	}

	if !user.Active {
		return req, fmt.Errorf("user account is inactive")
	}

	// Set authentication context
	req.UserID = user.ID
	req.Roles = user.Roles
	req.IsAdmin = user.IsAdmin
	req.Authorized = true

	req.AddMetadata("user_id", req.UserID)
	req.AddMetadata("roles", req.Roles)
	req.AddMetadata("is_admin", req.IsAdmin)
	req.AddTag("authenticated")

	return req, nil
}

// CheckPermissions validates that the user has permission for the request
func CheckPermissions(_ context.Context, req Request) error {
	if !req.Authorized {
		return fmt.Errorf("user not authenticated")
	}

	// Admin paths require admin role
	if strings.HasPrefix(req.Path, "/admin") && !req.IsAdmin {
		return fmt.Errorf("admin access required for path: %s", req.Path)
	}

	// Premium paths require premium role
	if strings.HasPrefix(req.Path, "/premium") && !req.HasRole("premium") && !req.IsAdmin {
		return fmt.Errorf("premium access required for path: %s", req.Path)
	}

	// DELETE operations require admin or ownership
	if req.Method == "DELETE" && !req.IsAdmin {
		// Check if user is trying to delete their own resource
		if !strings.Contains(req.Path, req.UserID) {
			return fmt.Errorf("insufficient permissions for DELETE operation")
		}
	}

	return nil
}

// EnforceRateLimit checks and enforces rate limiting
func EnforceRateLimit(_ context.Context, req Request) (Request, error) {
	if !req.Authorized {
		return req, fmt.Errorf("rate limiting requires authentication")
	}

	// Get or create rate limit for user
	var limit *RateLimit
	if stored, exists := rateLimitStore.Load(req.UserID); exists {
		limit = stored.(*RateLimit)
	} else {
		// Default limits: 1000 requests per hour for regular users, 5000 for admins
		maxRequests := 1000
		if req.IsAdmin {
			maxRequests = 5000
		}

		limit = &RateLimit{
			UserID:    req.UserID,
			Limit:     maxRequests,
			Remaining: maxRequests,
			Reset:     time.Now().Add(time.Hour),
			Window:    time.Hour,
		}
		rateLimitStore.Store(req.UserID, limit)
	}

	// Check if rate limit window has reset
	if time.Now().After(limit.Reset) {
		limit.Remaining = limit.Limit
		limit.Reset = time.Now().Add(limit.Window)
	}

	// Check if user has exceeded rate limit
	if limit.Remaining <= 0 {
		return req, fmt.Errorf("rate limit exceeded for user %s. Try again at %s",
			req.UserID, limit.Reset.Format(time.RFC3339))
	}

	// Consume one request
	limit.Remaining--
	req.RateLimit = limit

	req.AddMetadata("rate_limit", map[string]interface{}{
		"limit":     limit.Limit,
		"remaining": limit.Remaining,
		"reset":     limit.Reset,
	})

	return req, nil
}

// LogRequest logs the incoming request
func LogRequest(_ context.Context, req Request) error {
	logLevel := "INFO"
	if req.IsAdmin {
		logLevel = "ADMIN"
	}

	fmt.Printf("[%s] %s %s %s - User: %s, IP: %s, Agent: %s\n",
		logLevel,
		req.Timestamp.Format(time.RFC3339),
		req.Method,
		req.Path,
		req.UserID,
		req.ClientIP,
		req.UserAgent,
	)

	req.AddTag("logged")
	return nil
}

// ValidateRequest performs basic request validation
func ValidateRequest(_ context.Context, req Request) error {
	if req.Method == "" {
		return fmt.Errorf("HTTP method is required")
	}

	if req.Path == "" {
		return fmt.Errorf("request path is required")
	}

	// Validate path format
	if !strings.HasPrefix(req.Path, "/") {
		return fmt.Errorf("request path must start with /")
	}

	// Check for suspicious patterns
	if strings.Contains(req.Path, "..") {
		return fmt.Errorf("path traversal attempt detected")
	}

	if strings.Contains(req.Path, "<script>") {
		return fmt.Errorf("XSS attempt detected in path")
	}

	// Validate method
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	methodValid := false
	for _, method := range validMethods {
		if req.Method == method {
			methodValid = true
			break
		}
	}

	if !methodValid {
		return fmt.Errorf("unsupported HTTP method: %s", req.Method)
	}

	return nil
}

// AddSecurityHeaders adds security-related headers to the response
func AddSecurityHeaders(_ context.Context, req Request) (Request, error) {
	if req.Response == nil {
		req.Response = &Response{
			StatusCode: 200,
			Headers:    make(map[string]string),
		}
	}

	// Add security headers
	req.Response.Headers["X-Content-Type-Options"] = "nosniff"
	req.Response.Headers["X-Frame-Options"] = "DENY"
	req.Response.Headers["X-XSS-Protection"] = "1; mode=block"
	req.Response.Headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
	req.Response.Headers["Content-Security-Policy"] = "default-src 'self'"
	req.Response.Headers["Referrer-Policy"] = "strict-origin-when-cross-origin"

	// Add rate limit headers
	if req.RateLimit != nil {
		req.Response.Headers["X-RateLimit-Limit"] = fmt.Sprintf("%d", req.RateLimit.Limit)
		req.Response.Headers["X-RateLimit-Remaining"] = fmt.Sprintf("%d", req.RateLimit.Remaining)
		req.Response.Headers["X-RateLimit-Reset"] = fmt.Sprintf("%d", req.RateLimit.Reset.Unix())
	}

	// Add request ID header
	req.Response.Headers["X-Request-ID"] = req.RequestID

	req.AddTag("security_headers_added")
	return req, nil
}

// MeasurePerformance measures request processing performance
func MeasurePerformance(_ context.Context, req Request) error {
	req.Duration = time.Since(req.StartTime)

	// Log slow requests
	if req.Duration > 500*time.Millisecond {
		fmt.Printf("[SLOW] Request %s took %v to process\n", req.RequestID, req.Duration)
		req.AddTag("slow_request")
	}

	req.AddMetadata("duration_ms", req.Duration.Milliseconds())
	return nil
}

// AuditRequest audits the request for security and compliance
func AuditRequest(_ context.Context, req Request) error {
	auditData := map[string]interface{}{
		"request_id":  req.RequestID,
		"timestamp":   req.Timestamp,
		"user_id":     req.UserID,
		"method":      req.Method,
		"path":        req.Path,
		"client_ip":   req.ClientIP,
		"user_agent":  req.UserAgent,
		"duration":    req.Duration,
		"status_code": req.StatusCode,
		"tags":        req.Tags,
	}

	// In production, this would write to an audit log system
	fmt.Printf("[AUDIT] %+v\n", auditData)

	return nil
}

// CreateStandardMiddleware creates a standard HTTP middleware pipeline
func CreateStandardMiddleware() *pipz.Pipeline[Request] {
	pipeline := pipz.NewPipeline[Request]()
	pipeline.Register(
		pipz.Apply("generate_request_id", GenerateRequestID),
		pipz.Apply("parse_headers", ParseHeaders),
		pipz.Effect("validate_request", ValidateRequest),
		pipz.Apply("authenticate", AuthenticateToken),
		pipz.Effect("check_permissions", CheckPermissions),
		pipz.Apply("enforce_rate_limit", EnforceRateLimit),
		pipz.Effect("log_request", LogRequest),
		pipz.Apply("add_security_headers", AddSecurityHeaders),
		pipz.Effect("measure_performance", MeasurePerformance),
		pipz.Effect("audit_request", AuditRequest),
	)
	return pipeline
}

// CreatePublicMiddleware creates middleware for public endpoints (no auth required)
func CreatePublicMiddleware() *pipz.Pipeline[Request] {
	pipeline := pipz.NewPipeline[Request]()
	pipeline.Register(
		pipz.Apply("generate_request_id", GenerateRequestID),
		pipz.Apply("parse_headers", ParseHeaders),
		pipz.Effect("validate_request", ValidateRequest),
		pipz.Apply("add_security_headers", AddSecurityHeaders),
		pipz.Effect("log_request", func(_ context.Context, req Request) error {
			fmt.Printf("[PUBLIC] %s %s %s - IP: %s\n",
				req.Timestamp.Format(time.RFC3339),
				req.Method,
				req.Path,
				req.ClientIP,
			)
			return nil
		}),
		pipz.Effect("measure_performance", MeasurePerformance),
	)
	return pipeline
}

// CreateAdminMiddleware creates stricter middleware for admin endpoints
func CreateAdminMiddleware() *pipz.Pipeline[Request] {
	pipeline := CreateStandardMiddleware()

	// Add additional admin-specific middleware
	pipeline.InsertAt(6, // After rate limiting
		pipz.Effect("require_admin", func(_ context.Context, req Request) error {
			if !req.IsAdmin {
				return fmt.Errorf("admin access required")
			}
			return nil
		}),
		pipz.Effect("log_admin_access", func(_ context.Context, req Request) error {
			fmt.Printf("[ADMIN_ACCESS] User %s accessed %s %s from %s\n",
				req.UserID, req.Method, req.Path, req.ClientIP)
			return nil
		}),
	)

	return pipeline
}

// HTTPHandler converts an HTTP request to our Request type
func HTTPHandler(pipeline *pipz.Pipeline[Request], handler func(Request) *Response) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Convert HTTP request to our Request type
		headers := make(map[string]string)
		for key, values := range r.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		req := Request{
			Method:    r.Method,
			Path:      r.URL.Path,
			Headers:   headers,
			ClientIP:  r.RemoteAddr,
			Timestamp: time.Now(),
			StartTime: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		// Process through middleware pipeline
		result, err := pipeline.Process(r.Context(), req)
		if err != nil {
			// Handle middleware errors
			http.Error(w, err.Error(), getHTTPStatusFromError(err))
			return
		}

		// Call the actual handler
		response := handler(result)
		if response == nil {
			response = &Response{
				StatusCode: 200,
				Headers:    make(map[string]string),
				Body:       []byte("OK"),
			}
		}

		// Set response headers
		for key, value := range response.Headers {
			w.Header().Set(key, value)
		}

		// Write response
		w.WriteHeader(response.StatusCode)
		if response.Body != nil {
			w.Write(response.Body)
		}
	}
}

// getHTTPStatusFromError maps errors to HTTP status codes
func getHTTPStatusFromError(err error) int {
	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "missing authorization") ||
		strings.Contains(errMsg, "invalid auth token") ||
		strings.Contains(errMsg, "user not authenticated"):
		return http.StatusUnauthorized

	case strings.Contains(errMsg, "insufficient permissions") ||
		strings.Contains(errMsg, "admin access required") ||
		strings.Contains(errMsg, "premium access required"):
		return http.StatusForbidden

	case strings.Contains(errMsg, "rate limit exceeded"):
		return http.StatusTooManyRequests

	case strings.Contains(errMsg, "validation") ||
		strings.Contains(errMsg, "invalid") ||
		strings.Contains(errMsg, "required"):
		return http.StatusBadRequest

	case strings.Contains(errMsg, "user not found"):
		return http.StatusNotFound

	case strings.Contains(errMsg, "inactive"):
		return http.StatusForbidden

	default:
		return http.StatusInternalServerError
	}
}

// Example demonstrates using the middleware pipeline
func Example() {
	// Create different pipelines for different endpoint types
	standardPipeline := CreateStandardMiddleware()
	publicPipeline := CreatePublicMiddleware()
	adminPipeline := CreateAdminMiddleware()

	// Example handler
	handler := func(req Request) *Response {
		return &Response{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"message": "Hello, World!", "user": "` + req.UserID + `"}`),
		}
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/public", HTTPHandler(publicPipeline, handler))
	mux.HandleFunc("/api/", HTTPHandler(standardPipeline, handler))
	mux.HandleFunc("/admin/", HTTPHandler(adminPipeline, handler))

	// Example requests
	fmt.Println("=== Public Request ===")
	req := Request{
		Method:    "GET",
		Path:      "/public",
		Headers:   map[string]string{"User-Agent": "Test-Client/1.0"},
		ClientIP:  "192.168.1.100",
		Timestamp: time.Now(),
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	result, err := publicPipeline.Process(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Request processed successfully: %s\n", result.RequestID)
	}

	fmt.Println("\n=== Authenticated Request ===")
	req = Request{
		Method: "GET",
		Path:   "/api/users",
		Headers: map[string]string{
			"Authorization": "Bearer valid-token-123",
			"User-Agent":    "Test-Client/1.0",
		},
		ClientIP:  "192.168.1.100",
		Timestamp: time.Now(),
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	result, err = standardPipeline.Process(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Request processed successfully: %s\n", result.RequestID)
	}

	fmt.Println("\n=== Admin Request ===")
	req = Request{
		Method: "DELETE",
		Path:   "/admin/users/user-123",
		Headers: map[string]string{
			"Authorization": "Bearer admin-token-456",
			"User-Agent":    "Admin-Client/1.0",
		},
		ClientIP:  "192.168.1.200",
		Timestamp: time.Now(),
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	result, err = adminPipeline.Process(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Request processed successfully: %s\n", result.RequestID)
	}
}
