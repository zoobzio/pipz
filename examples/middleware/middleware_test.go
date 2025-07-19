package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestGenerateRequestID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     Request
		checkFn func(t *testing.T, result Request)
	}{
		{
			name: "generates request ID",
			req: Request{
				ClientIP: "192.168.1.100",
			},
			checkFn: func(t *testing.T, result Request) {
				if result.RequestID == "" {
					t.Error("expected request ID to be generated")
				}
				if !strings.Contains(result.RequestID, "192.168.1.100") {
					t.Error("expected request ID to contain client IP")
				}
			},
		},
		{
			name: "sets timestamp",
			req: Request{
				ClientIP: "192.168.1.100",
			},
			checkFn: func(t *testing.T, result Request) {
				if result.Timestamp.IsZero() {
					t.Error("expected timestamp to be set")
				}
				if result.StartTime.IsZero() {
					t.Error("expected start time to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateRequestID(ctx, tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.checkFn(t, result)
		})
	}
}

func TestParseHeaders(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     Request
		wantErr bool
		checkFn func(t *testing.T, result Request)
	}{
		{
			name: "parses user agent",
			req: Request{
				Headers: map[string]string{
					"User-Agent": "Test-Client/1.0",
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, result Request) {
				if result.UserAgent != "Test-Client/1.0" {
					t.Errorf("expected user agent 'Test-Client/1.0', got %s", result.UserAgent)
				}
			},
		},
		{
			name: "parses content length",
			req: Request{
				Headers: map[string]string{
					"Content-Length": "1024",
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, result Request) {
				if result.BytesRead != 1024 {
					t.Errorf("expected bytes read 1024, got %d", result.BytesRead)
				}
			},
		},
		{
			name: "parses X-Forwarded-For",
			req: Request{
				Headers: map[string]string{
					"X-Forwarded-For": "203.0.113.1, 192.168.1.100",
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, result Request) {
				if result.ClientIP != "203.0.113.1" {
					t.Errorf("expected client IP '203.0.113.1', got %s", result.ClientIP)
				}
			},
		},
		{
			name: "no headers",
			req: Request{
				Headers: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseHeaders(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHeaders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestAuthenticateToken(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     Request
		wantErr string
		checkFn func(t *testing.T, result Request)
	}{
		{
			name: "valid token",
			req: Request{
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "",
			checkFn: func(t *testing.T, result Request) {
				if result.UserID != "user-123" {
					t.Errorf("expected user ID 'user-123', got %s", result.UserID)
				}
				if !result.Authorized {
					t.Error("expected user to be authorized")
				}
			},
		},
		{
			name: "admin token",
			req: Request{
				Headers: map[string]string{
					"Authorization": "Bearer admin-token-456",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "",
			checkFn: func(t *testing.T, result Request) {
				if !result.IsAdmin {
					t.Error("expected user to be admin")
				}
			},
		},
		{
			name: "missing authorization header",
			req: Request{
				Headers: map[string]string{},
			},
			wantErr: "missing authorization header",
		},
		{
			name: "invalid token format",
			req: Request{
				Headers: map[string]string{
					"Authorization": "InvalidFormat",
				},
			},
			wantErr: "invalid authorization header format",
		},
		{
			name: "invalid token",
			req: Request{
				Headers: map[string]string{
					"Authorization": "Bearer invalid-token",
				},
			},
			wantErr: "invalid auth token",
		},
		{
			name: "inactive user",
			req: Request{
				Headers: map[string]string{
					"Authorization": "Bearer inactive-token-999",
				},
			},
			wantErr: "user account is inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AuthenticateToken(ctx, tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkFn != nil {
					tt.checkFn(t, result)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestCheckPermissions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     Request
		wantErr string
	}{
		{
			name: "admin access to admin path",
			req: Request{
				Path:       "/admin/users",
				Method:     "GET",
				Authorized: true,
				IsAdmin:    true,
			},
			wantErr: "",
		},
		{
			name: "non-admin access to admin path",
			req: Request{
				Path:       "/admin/users",
				Method:     "GET",
				Authorized: true,
				IsAdmin:    false,
			},
			wantErr: "admin access required",
		},
		{
			name: "premium user access to premium path",
			req: Request{
				Path:       "/premium/features",
				Method:     "GET",
				Authorized: true,
				IsAdmin:    false,
				Roles:      []string{"user", "premium"},
			},
			wantErr: "",
		},
		{
			name: "regular user access to premium path",
			req: Request{
				Path:       "/premium/features",
				Method:     "GET",
				Authorized: true,
				IsAdmin:    false,
				Roles:      []string{"user"},
			},
			wantErr: "premium access required",
		},
		{
			name: "delete own resource",
			req: Request{
				Path:       "/users/user-123",
				Method:     "DELETE",
				Authorized: true,
				IsAdmin:    false,
				UserID:     "user-123",
			},
			wantErr: "",
		},
		{
			name: "delete other user's resource",
			req: Request{
				Path:       "/users/user-456",
				Method:     "DELETE",
				Authorized: true,
				IsAdmin:    false,
				UserID:     "user-123",
			},
			wantErr: "insufficient permissions for DELETE",
		},
		{
			name: "unauthenticated request",
			req: Request{
				Path:       "/api/users",
				Method:     "GET",
				Authorized: false,
			},
			wantErr: "user not authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPermissions(ctx, tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestEnforceRateLimit(t *testing.T) {
	ctx := context.Background()

	// Clear rate limit store
	rateLimitStore = sync.Map{}

	tests := []struct {
		name    string
		req     Request
		wantErr bool
		checkFn func(t *testing.T, result Request)
	}{
		{
			name: "first request within limit",
			req: Request{
				UserID:     "test-user-1",
				Authorized: true,
				IsAdmin:    false,
				Metadata:   make(map[string]interface{}),
			},
			wantErr: false,
			checkFn: func(t *testing.T, result Request) {
				if result.RateLimit == nil {
					t.Error("expected rate limit to be set")
				}
				if result.RateLimit.Remaining != 999 {
					t.Errorf("expected remaining 999, got %d", result.RateLimit.Remaining)
				}
			},
		},
		{
			name: "admin gets higher limit",
			req: Request{
				UserID:     "admin-user-1",
				Authorized: true,
				IsAdmin:    true,
				Metadata:   make(map[string]interface{}),
			},
			wantErr: false,
			checkFn: func(t *testing.T, result Request) {
				if result.RateLimit.Limit != 5000 {
					t.Errorf("expected admin limit 5000, got %d", result.RateLimit.Limit)
				}
			},
		},
		{
			name: "unauthenticated request",
			req: Request{
				UserID:     "",
				Authorized: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EnforceRateLimit(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnforceRateLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestValidateRequest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     Request
		wantErr string
	}{
		{
			name: "valid request",
			req: Request{
				Method: "GET",
				Path:   "/api/users",
			},
			wantErr: "",
		},
		{
			name: "missing method",
			req: Request{
				Path: "/api/users",
			},
			wantErr: "HTTP method is required",
		},
		{
			name: "missing path",
			req: Request{
				Method: "GET",
			},
			wantErr: "request path is required",
		},
		{
			name: "invalid path format",
			req: Request{
				Method: "GET",
				Path:   "api/users",
			},
			wantErr: "request path must start with /",
		},
		{
			name: "path traversal attempt",
			req: Request{
				Method: "GET",
				Path:   "/api/../admin/users",
			},
			wantErr: "path traversal attempt detected",
		},
		{
			name: "XSS attempt",
			req: Request{
				Method: "GET",
				Path:   "/search?q=<script>alert('xss')</script>",
			},
			wantErr: "XSS attempt detected",
		},
		{
			name: "invalid HTTP method",
			req: Request{
				Method: "INVALID",
				Path:   "/api/users",
			},
			wantErr: "unsupported HTTP method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(ctx, tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestAddSecurityHeaders(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		req     Request
		checkFn func(t *testing.T, result Request)
	}{
		{
			name: "adds security headers",
			req: Request{
				RequestID: "test-123",
			},
			checkFn: func(t *testing.T, result Request) {
				if result.Response == nil {
					t.Fatal("expected response to be created")
				}

				expectedHeaders := []string{
					"X-Content-Type-Options",
					"X-Frame-Options",
					"X-XSS-Protection",
					"Strict-Transport-Security",
					"Content-Security-Policy",
					"Referrer-Policy",
					"X-Request-ID",
				}

				for _, header := range expectedHeaders {
					if _, exists := result.Response.Headers[header]; !exists {
						t.Errorf("expected header %s to be set", header)
					}
				}
			},
		},
		{
			name: "adds rate limit headers",
			req: Request{
				RequestID: "test-123",
				RateLimit: &RateLimit{
					Limit:     1000,
					Remaining: 999,
					Reset:     time.Now().Add(time.Hour),
				},
			},
			checkFn: func(t *testing.T, result Request) {
				if result.Response.Headers["X-RateLimit-Limit"] != "1000" {
					t.Error("expected X-RateLimit-Limit header to be set")
				}
				if result.Response.Headers["X-RateLimit-Remaining"] != "999" {
					t.Error("expected X-RateLimit-Remaining header to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AddSecurityHeaders(ctx, tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.checkFn(t, result)
		})
	}
}

func TestStandardMiddleware(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateStandardMiddleware()

	tests := []struct {
		name    string
		req     Request
		wantErr string
		checkFn func(t *testing.T, result Request)
	}{
		{
			name: "successful request",
			req: Request{
				Method:   "GET",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "",
			checkFn: func(t *testing.T, result Request) {
				if result.UserID != "user-123" {
					t.Error("expected user to be authenticated")
				}
				if result.RequestID == "" {
					t.Error("expected request ID to be generated")
				}
				if result.Response == nil {
					t.Error("expected response to be created")
				}
			},
		},
		{
			name: "unauthorized request",
			req: Request{
				Method:   "GET",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"User-Agent": "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "missing authorization header",
		},
		{
			name: "admin access to admin path",
			req: Request{
				Method:   "GET",
				Path:     "/admin/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer admin-token-456",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "",
			checkFn: func(t *testing.T, result Request) {
				if !result.IsAdmin {
					t.Error("expected user to be admin")
				}
			},
		},
		{
			name: "non-admin access to admin path",
			req: Request{
				Method:   "GET",
				Path:     "/admin/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "admin access required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(ctx, tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkFn != nil {
					tt.checkFn(t, result)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestPublicMiddleware(t *testing.T) {
	ctx := context.Background()
	pipeline := CreatePublicMiddleware()

	req := Request{
		Method:   "GET",
		Path:     "/public/health",
		ClientIP: "192.168.1.100",
		Headers: map[string]string{
			"User-Agent": "Test-Client/1.0",
		},
		Metadata: make(map[string]interface{}),
	}

	result, err := pipeline.Process(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RequestID == "" {
		t.Error("expected request ID to be generated")
	}

	if result.Response == nil {
		t.Error("expected response to be created")
	}

	// Public middleware should not authenticate
	if result.Authorized {
		t.Error("public middleware should not authenticate")
	}
}

func TestAdminMiddleware(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateAdminMiddleware()

	tests := []struct {
		name    string
		req     Request
		wantErr string
	}{
		{
			name: "admin access succeeds",
			req: Request{
				Method:   "GET",
				Path:     "/admin/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer admin-token-456",
					"User-Agent":    "Admin-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "",
		},
		{
			name: "non-admin access fails",
			req: Request{
				Method:   "GET",
				Path:     "/admin/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
			wantErr: "admin access required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pipeline.Process(ctx, tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestGetHTTPStatusFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "unauthorized error",
			err:  fmt.Errorf("missing authorization header"),
			want: http.StatusUnauthorized,
		},
		{
			name: "forbidden error",
			err:  fmt.Errorf("insufficient permissions"),
			want: http.StatusForbidden,
		},
		{
			name: "rate limit error",
			err:  fmt.Errorf("rate limit exceeded"),
			want: http.StatusTooManyRequests,
		},
		{
			name: "validation error",
			err:  fmt.Errorf("invalid request"),
			want: http.StatusBadRequest,
		},
		{
			name: "not found error",
			err:  fmt.Errorf("user not found"),
			want: http.StatusNotFound,
		},
		{
			name: "unknown error",
			err:  fmt.Errorf("something went wrong"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getHTTPStatusFromError(tt.err)
			if got != tt.want {
				t.Errorf("getHTTPStatusFromError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestHelpers(t *testing.T) {
	t.Run("AddMetadata", func(t *testing.T) {
		req := Request{}
		req.AddMetadata("key1", "value1")
		req.AddMetadata("key2", 42)

		if req.Metadata["key1"] != "value1" {
			t.Error("expected metadata key1 to be set")
		}
		if req.Metadata["key2"] != 42 {
			t.Error("expected metadata key2 to be set")
		}
	})

	t.Run("AddTag", func(t *testing.T) {
		req := Request{}
		req.AddTag("tag1")
		req.AddTag("tag2")

		if len(req.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(req.Tags))
		}
		if req.Tags[0] != "tag1" || req.Tags[1] != "tag2" {
			t.Error("expected tags to be set correctly")
		}
	})

	t.Run("HasRole", func(t *testing.T) {
		req := Request{
			Roles: []string{"user", "premium"},
		}

		if !req.HasRole("user") {
			t.Error("expected user to have 'user' role")
		}
		if !req.HasRole("premium") {
			t.Error("expected user to have 'premium' role")
		}
		if req.HasRole("admin") {
			t.Error("expected user to not have 'admin' role")
		}
	})
}

func TestPipelineErrorContext(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateStandardMiddleware()

	// Test error context propagation
	req := Request{
		Method:   "GET",
		Path:     "/api/users",
		ClientIP: "192.168.1.100",
		Headers: map[string]string{
			"Authorization": "Bearer invalid-token",
			"User-Agent":    "Test-Client/1.0",
		},
		Metadata: make(map[string]interface{}),
	}

	_, err := pipeline.Process(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check that we get a PipelineError with context
	var pipelineErr *pipz.PipelineError[Request]
	if !errors.As(err, &pipelineErr) {
		t.Fatalf("expected PipelineError, got %T", err)
	}

	// Should fail at authenticate processor
	if pipelineErr.ProcessorName != "authenticate" {
		t.Errorf("expected processor name 'authenticate', got %q", pipelineErr.ProcessorName)
	}
}

func TestExample(t *testing.T) {
	// Just test that the example runs without panicking
	Example()
}
