package middleware_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz/examples/middleware"
)

func BenchmarkMiddleware_SingleProcessors(b *testing.B) {
	ctx := context.Background()

	b.Run("GenerateRequestID", func(b *testing.B) {
		req := middleware.Request{
			ClientIP: "192.168.1.100",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.GenerateRequestID(ctx, req)
		}
	})

	b.Run("ParseHeaders", func(b *testing.B) {
		req := middleware.Request{
			Headers: map[string]string{
				"User-Agent":      "Test-Client/1.0",
				"Content-Length":  "1024",
				"X-Forwarded-For": "203.0.113.1, 192.168.1.100",
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.ParseHeaders(ctx, req)
		}
	})

	b.Run("AuthenticateToken", func(b *testing.B) {
		req := middleware.Request{
			Headers: map[string]string{
				"Authorization": "Bearer valid-token-123",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.AuthenticateToken(ctx, req)
		}
	})

	b.Run("CheckPermissions", func(b *testing.B) {
		req := middleware.Request{
			Method:     "GET",
			Path:       "/api/users",
			Authorized: true,
			IsAdmin:    false,
			UserID:     "user-123",
			Roles:      []string{"user"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})

	b.Run("EnforceRateLimit", func(b *testing.B) {
		req := middleware.Request{
			UserID:     "bench-user",
			Authorized: true,
			IsAdmin:    false,
			Metadata:   make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.EnforceRateLimit(ctx, req)
		}
	})

	b.Run("ValidateRequest", func(b *testing.B) {
		req := middleware.Request{
			Method: "GET",
			Path:   "/api/users",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.ValidateRequest(ctx, req)
		}
	})

	b.Run("AddSecurityHeaders", func(b *testing.B) {
		req := middleware.Request{
			RequestID: "test-123",
			RateLimit: &middleware.RateLimit{
				Limit:     1000,
				Remaining: 999,
				Reset:     time.Now().Add(time.Hour),
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.AddSecurityHeaders(ctx, req)
		}
	})
}

func BenchmarkMiddleware_Authentication(b *testing.B) {
	ctx := context.Background()

	b.Run("ValidToken", func(b *testing.B) {
		req := middleware.Request{
			Headers: map[string]string{
				"Authorization": "Bearer valid-token-123",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.AuthenticateToken(ctx, req)
		}
	})

	b.Run("AdminToken", func(b *testing.B) {
		req := middleware.Request{
			Headers: map[string]string{
				"Authorization": "Bearer admin-token-456",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.AuthenticateToken(ctx, req)
		}
	})

	b.Run("InvalidToken", func(b *testing.B) {
		req := middleware.Request{
			Headers: map[string]string{
				"Authorization": "Bearer invalid-token",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.AuthenticateToken(ctx, req)
		}
	})

	b.Run("MissingToken", func(b *testing.B) {
		req := middleware.Request{
			Headers:  map[string]string{},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.AuthenticateToken(ctx, req)
		}
	})
}

func BenchmarkMiddleware_Authorization(b *testing.B) {
	ctx := context.Background()

	b.Run("RegularPath", func(b *testing.B) {
		req := middleware.Request{
			Method:     "GET",
			Path:       "/api/users",
			Authorized: true,
			IsAdmin:    false,
			UserID:     "user-123",
			Roles:      []string{"user"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})

	b.Run("AdminPath_AdminUser", func(b *testing.B) {
		req := middleware.Request{
			Method:     "GET",
			Path:       "/admin/users",
			Authorized: true,
			IsAdmin:    true,
			UserID:     "admin-456",
			Roles:      []string{"admin"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})

	b.Run("AdminPath_RegularUser", func(b *testing.B) {
		req := middleware.Request{
			Method:     "GET",
			Path:       "/admin/users",
			Authorized: true,
			IsAdmin:    false,
			UserID:     "user-123",
			Roles:      []string{"user"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})

	b.Run("PremiumPath_PremiumUser", func(b *testing.B) {
		req := middleware.Request{
			Method:     "GET",
			Path:       "/premium/features",
			Authorized: true,
			IsAdmin:    false,
			UserID:     "user-789",
			Roles:      []string{"user", "premium"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})

	b.Run("DeleteOwnResource", func(b *testing.B) {
		req := middleware.Request{
			Method:     "DELETE",
			Path:       "/users/user-123",
			Authorized: true,
			IsAdmin:    false,
			UserID:     "user-123",
			Roles:      []string{"user"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})

	b.Run("DeleteOtherResource", func(b *testing.B) {
		req := middleware.Request{
			Method:     "DELETE",
			Path:       "/users/user-456",
			Authorized: true,
			IsAdmin:    false,
			UserID:     "user-123",
			Roles:      []string{"user"},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.CheckPermissions(ctx, req)
		}
	})
}

func BenchmarkMiddleware_RateLimit(b *testing.B) {
	ctx := context.Background()

	b.Run("RegularUser", func(b *testing.B) {
		req := middleware.Request{
			UserID:     "rate-limit-user",
			Authorized: true,
			IsAdmin:    false,
			Metadata:   make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.EnforceRateLimit(ctx, req)
		}
	})

	b.Run("AdminUser", func(b *testing.B) {
		req := middleware.Request{
			UserID:     "rate-limit-admin",
			Authorized: true,
			IsAdmin:    true,
			Metadata:   make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.EnforceRateLimit(ctx, req)
		}
	})

	b.Run("ConcurrentUsers", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			userID := fmt.Sprintf("concurrent-user-%d", time.Now().UnixNano())
			req := middleware.Request{
				UserID:     userID,
				Authorized: true,
				IsAdmin:    false,
				Metadata:   make(map[string]interface{}),
			}

			for pb.Next() {
				_, _ = middleware.EnforceRateLimit(ctx, req)
			}
		})
	})
}

func BenchmarkMiddleware_Validation(b *testing.B) {
	ctx := context.Background()

	b.Run("ValidRequest", func(b *testing.B) {
		req := middleware.Request{
			Method: "GET",
			Path:   "/api/users",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.ValidateRequest(ctx, req)
		}
	})

	b.Run("InvalidMethod", func(b *testing.B) {
		req := middleware.Request{
			Method: "INVALID",
			Path:   "/api/users",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.ValidateRequest(ctx, req)
		}
	})

	b.Run("PathTraversal", func(b *testing.B) {
		req := middleware.Request{
			Method: "GET",
			Path:   "/api/../admin/users",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.ValidateRequest(ctx, req)
		}
	})

	b.Run("XSSAttempt", func(b *testing.B) {
		req := middleware.Request{
			Method: "GET",
			Path:   "/search?q=<script>alert('xss')</script>",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = middleware.ValidateRequest(ctx, req)
		}
	})
}

func BenchmarkMiddleware_Pipeline(b *testing.B) {
	ctx := context.Background()

	b.Run("StandardPipeline_Success", func(b *testing.B) {
		pipeline := middleware.CreateStandardMiddleware()
		req := middleware.Request{
			Method:   "GET",
			Path:     "/api/users",
			ClientIP: "192.168.1.100",
			Headers: map[string]string{
				"Authorization": "Bearer valid-token-123",
				"User-Agent":    "Test-Client/1.0",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, req)
		}
	})

	b.Run("StandardPipeline_AuthFailure", func(b *testing.B) {
		pipeline := middleware.CreateStandardMiddleware()
		req := middleware.Request{
			Method:   "GET",
			Path:     "/api/users",
			ClientIP: "192.168.1.100",
			Headers: map[string]string{
				"Authorization": "Bearer invalid-token",
				"User-Agent":    "Test-Client/1.0",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, req)
		}
	})

	b.Run("PublicPipeline", func(b *testing.B) {
		pipeline := middleware.CreatePublicMiddleware()
		req := middleware.Request{
			Method:   "GET",
			Path:     "/public/health",
			ClientIP: "192.168.1.100",
			Headers: map[string]string{
				"User-Agent": "Test-Client/1.0",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, req)
		}
	})

	b.Run("AdminPipeline_Success", func(b *testing.B) {
		pipeline := middleware.CreateAdminMiddleware()
		req := middleware.Request{
			Method:   "GET",
			Path:     "/admin/users",
			ClientIP: "192.168.1.100",
			Headers: map[string]string{
				"Authorization": "Bearer admin-token-456",
				"User-Agent":    "Admin-Client/1.0",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, req)
		}
	})

	b.Run("AdminPipeline_AuthFailure", func(b *testing.B) {
		pipeline := middleware.CreateAdminMiddleware()
		req := middleware.Request{
			Method:   "GET",
			Path:     "/admin/users",
			ClientIP: "192.168.1.100",
			Headers: map[string]string{
				"Authorization": "Bearer valid-token-123",
				"User-Agent":    "Test-Client/1.0",
			},
			Metadata: make(map[string]interface{}),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, req)
		}
	})
}

func BenchmarkMiddleware_HTTPMethods(b *testing.B) {
	ctx := context.Background()
	pipeline := middleware.CreateStandardMiddleware()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		b.Run(method, func(b *testing.B) {
			req := middleware.Request{
				Method:   method,
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, req)
			}
		})
	}
}

func BenchmarkMiddleware_RequestSize(b *testing.B) {
	ctx := context.Background()
	pipeline := middleware.CreateStandardMiddleware()

	sizes := []int{100, 1000, 10000, 100000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			headers := make(map[string]string)
			headers["Authorization"] = "Bearer valid-token-123"
			headers["User-Agent"] = "Test-Client/1.0"
			headers["Content-Length"] = fmt.Sprintf("%d", size)

			// Add extra headers to simulate larger requests
			for i := 0; i < size/100; i++ {
				headers[fmt.Sprintf("X-Custom-Header-%d", i)] = fmt.Sprintf("value-%d", i)
			}

			req := middleware.Request{
				Method:   "POST",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers:  headers,
				Body:     make([]byte, size),
				Metadata: make(map[string]interface{}),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, req)
			}
		})
	}
}

func BenchmarkMiddleware_ConcurrentRequests(b *testing.B) {
	ctx := context.Background()
	pipeline := middleware.CreateStandardMiddleware()

	b.Run("ConcurrentProcessing", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			req := middleware.Request{
				Method:   "GET",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			}

			for pb.Next() {
				_, _ = pipeline.Process(ctx, req)
			}
		})
	})
}

func BenchmarkMiddleware_ErrorScenarios(b *testing.B) {
	ctx := context.Background()
	pipeline := middleware.CreateStandardMiddleware()

	errorCases := []struct {
		name string
		req  middleware.Request
	}{
		{
			name: "MissingAuth",
			req: middleware.Request{
				Method:   "GET",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"User-Agent": "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
		},
		{
			name: "InvalidToken",
			req: middleware.Request{
				Method:   "GET",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer invalid-token",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
		},
		{
			name: "AdminPathRegularUser",
			req: middleware.Request{
				Method:   "GET",
				Path:     "/admin/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
		},
		{
			name: "InvalidMethod",
			req: middleware.Request{
				Method:   "INVALID",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
		},
		{
			name: "PathTraversal",
			req: middleware.Request{
				Method:   "GET",
				Path:     "/api/../admin/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": "Bearer valid-token-123",
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			},
		},
	}

	for _, errorCase := range errorCases {
		b.Run(errorCase.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, errorCase.req)
			}
		})
	}
}

func BenchmarkMiddleware_UserTypes(b *testing.B) {
	ctx := context.Background()
	pipeline := middleware.CreateStandardMiddleware()

	userTypes := []struct {
		name  string
		token string
	}{
		{"RegularUser", "valid-token-123"},
		{"AdminUser", "admin-token-456"},
		{"PremiumUser", "premium-token-789"},
	}

	for _, userType := range userTypes {
		b.Run(userType.name, func(b *testing.B) {
			req := middleware.Request{
				Method:   "GET",
				Path:     "/api/users",
				ClientIP: "192.168.1.100",
				Headers: map[string]string{
					"Authorization": fmt.Sprintf("Bearer %s", userType.token),
					"User-Agent":    "Test-Client/1.0",
				},
				Metadata: make(map[string]interface{}),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, req)
			}
		})
	}
}

func BenchmarkMiddleware_HeaderParsing(b *testing.B) {
	ctx := context.Background()

	b.Run("MinimalHeaders", func(b *testing.B) {
		req := middleware.Request{
			Headers: map[string]string{
				"User-Agent": "Test-Client/1.0",
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.ParseHeaders(ctx, req)
		}
	})

	b.Run("ManyHeaders", func(b *testing.B) {
		headers := make(map[string]string)
		headers["User-Agent"] = "Test-Client/1.0"
		headers["Content-Length"] = "1024"
		headers["X-Forwarded-For"] = "203.0.113.1, 192.168.1.100, 10.0.0.1"
		headers["X-Real-IP"] = "203.0.113.1"
		headers["Accept"] = "application/json"
		headers["Accept-Encoding"] = "gzip, deflate"
		headers["Accept-Language"] = "en-US,en;q=0.9"
		headers["Cache-Control"] = "no-cache"
		headers["Connection"] = "keep-alive"
		headers["Host"] = "example.com"

		req := middleware.Request{
			Headers: headers,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.ParseHeaders(ctx, req)
		}
	})

	b.Run("LargeUserAgent", func(b *testing.B) {
		largeUserAgent := strings.Repeat("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 ", 10)

		req := middleware.Request{
			Headers: map[string]string{
				"User-Agent": largeUserAgent,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = middleware.ParseHeaders(ctx, req)
		}
	})
}
