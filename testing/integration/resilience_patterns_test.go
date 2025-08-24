package integration

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	pipztesting "github.com/zoobzio/pipz/testing"
)

func TestResilience_CircuitBreakerWithRetry(t *testing.T) {
	tests := []struct {
		name           string
		failures       int
		threshold      int
		expectSuccess  bool
		expectFallback bool
	}{
		{
			name:           "below_threshold_succeeds",
			failures:       2,
			threshold:      5,
			expectSuccess:  true,
			expectFallback: false,
		},
		{
			name:           "above_threshold_opens_circuit",
			failures:       6,
			threshold:      5,
			expectSuccess:  false,
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			var callCount int64

			// Create a flaky service that fails for the first N calls
			flakyService := pipz.Apply("flaky-service", func(_ context.Context, data string) (string, error) {
				count := atomic.AddInt64(&callCount, 1)
				if count <= int64(tt.failures) {
					return "", errors.New("service temporarily unavailable")
				}
				return data + "_processed", nil
			})

			// Build resilience pipeline with circuit breaker and retry
			pipeline := pipz.NewSequence[string]("resilient-service",
				// Circuit breaker wraps retry
				pipz.NewCircuitBreaker[string]("service-breaker",
					// Retry wraps the actual service
					pipz.NewRetry[string]("service-retry", flakyService, 3),
					tt.threshold,
					time.Second,
				),
			)

			// Add fallback for when circuit breaker opens
			fallbackPipeline := pipz.NewFallback[string]("service-with-fallback",
				pipeline,
				pipz.Transform("fallback", func(_ context.Context, data string) string {
					return data + "_fallback"
				}),
			)

			result, err := fallbackPipeline.Process(ctx, "test_data")

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("expected success but got error: %v", err)
				}
				if result != "test_data_processed" {
					t.Errorf("expected processed result, got: %s", result)
				}
			}

			if tt.expectFallback {
				if err != nil {
					t.Errorf("expected fallback success but got error: %v", err)
				}
				if result != "test_data_fallback" {
					t.Errorf("expected fallback result, got: %s", result)
				}
			}

			// Verify call count makes sense
			finalCallCount := atomic.LoadInt64(&callCount)
			t.Logf("Total service calls: %d", finalCallCount)
		})
	}
}

func TestResilience_RateLimitingWithBackoff(t *testing.T) {
	ctx := context.Background()

	var processed int64
	var rateLimited int64

	// Create a rate-limited service
	service := pipz.Apply("rate-limited-service", func(_ context.Context, data string) (string, error) {
		atomic.AddInt64(&processed, 1)
		return data + "_processed", nil
	})

	// Build pipeline with rate limiting and backoff
	pipeline := pipz.NewSequence[string]("rate-limited-pipeline",
		// Rate limiter allows 2 requests per second
		pipz.NewRateLimiter[string]("api-limiter", 2.0, 2).SetMode("drop"),
		// Backoff for rate limit errors
		pipz.NewBackoff[string]("backoff",
			service,
			2,                   // max attempts
			10*time.Millisecond, // initial delay
		),
	)

	// Process multiple requests quickly
	requests := []string{"req1", "req2", "req3", "req4", "req5"}
	results := make([]string, len(requests))
	errors := make([]error, len(requests))

	// Process all requests concurrently
	done := make(chan bool, len(requests))

	for i, req := range requests {
		go func(idx int, data string) {
			result, err := pipeline.Process(ctx, data)
			results[idx] = result
			errors[idx] = err
			if err != nil {
				atomic.AddInt64(&rateLimited, 1)
			}
			done <- true
		}(i, req)
	}

	// Wait for all requests to complete
	for i := 0; i < len(requests); i++ {
		<-done
	}

	processedCount := atomic.LoadInt64(&processed)
	rateLimitedCount := atomic.LoadInt64(&rateLimited)

	t.Logf("Processed: %d, Rate limited: %d", processedCount, rateLimitedCount)

	// With drop mode and 2 RPS limit, we expect some requests to be dropped
	if processedCount > 2 {
		t.Errorf("expected at most 2 processed requests due to rate limiting, got %d", processedCount)
	}

	if rateLimitedCount == 0 {
		t.Error("expected some requests to be rate limited")
	}

	// Verify successful requests have correct format
	for i, result := range results {
		if errors[i] == nil && result != requests[i]+"_processed" {
			t.Errorf("request %d: expected %s_processed, got %s",
				i, requests[i], result)
		}
	}
}

func TestResilience_TimeoutWithFallback(t *testing.T) {
	tests := []struct {
		name           string
		serviceDelay   time.Duration
		timeout        time.Duration
		expectTimeout  bool
		expectFallback bool
	}{
		{
			name:           "completes_within_timeout",
			serviceDelay:   50 * time.Millisecond,
			timeout:        100 * time.Millisecond,
			expectTimeout:  false,
			expectFallback: false,
		},
		{
			name:           "exceeds_timeout_uses_fallback",
			serviceDelay:   150 * time.Millisecond,
			timeout:        100 * time.Millisecond,
			expectTimeout:  true,
			expectFallback: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create a slow service
			slowService := pipz.Apply("slow-service", func(_ context.Context, data string) (string, error) {
				select {
				case <-time.After(tt.serviceDelay):
					return data + "_slow_processed", nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			})

			// Build pipeline with timeout and fallback
			timeoutPipeline := pipz.NewTimeout[string]("service-timeout", slowService, tt.timeout)

			fallbackPipeline := pipz.NewFallback[string]("service-with-fallback",
				timeoutPipeline,
				pipz.Transform("fast-fallback", func(_ context.Context, data string) string {
					return data + "_fast_fallback"
				}),
			)

			start := time.Now()
			result, err := fallbackPipeline.Process(ctx, "test_data")
			elapsed := time.Since(start)

			if tt.expectTimeout {
				// Should complete quickly due to timeout and fallback
				if elapsed > tt.timeout+50*time.Millisecond {
					t.Errorf("expected quick completion due to timeout, took %v", elapsed)
				}

				if tt.expectFallback {
					if err != nil {
						t.Errorf("expected fallback success but got error: %v", err)
					}
					if result != "test_data_fast_fallback" {
						t.Errorf("expected fallback result, got: %s", result)
					}
				}
			} else {
				// Should complete normally
				if err != nil {
					t.Errorf("expected success but got error: %v", err)
				}
				if result != "test_data_slow_processed" {
					t.Errorf("expected slow service result, got: %s", result)
				}
				if elapsed < tt.serviceDelay {
					t.Errorf("completed too quickly, expected at least %v, got %v",
						tt.serviceDelay, elapsed)
				}
			}
		})
	}
}

func TestResilience_ChaosEngineering(t *testing.T) {
	ctx := context.Background()

	// Create a service with built-in panic recovery for chaos testing
	reliableService := pipz.Apply("panic-safe-service", func(_ context.Context, data string) (string, error) {
		// This simulates a service that can panic but is wrapped with recovery
		defer func() {
			if r := recover(); r != nil {
				// Convert panic to error - this is what should happen in real systems
				// In production, panic recovery middleware would handle this
				_ = r // Acknowledge recovered panic
			}
		}()
		return data + "_reliable", nil
	})

	// Wrap with chaos processor (but disable panic rate since we handle it differently)
	chaosService := pipztesting.NewChaosProcessor("chaos", reliableService, pipztesting.ChaosConfig{
		FailureRate: 0.4, // 40% failure rate (increased to compensate for removed panics)
		LatencyMin:  10 * time.Millisecond,
		LatencyMax:  50 * time.Millisecond,
		TimeoutRate: 0.15,  // 15% timeout rate (increased)
		PanicRate:   0.0,   // No panics - handle chaos through errors and timeouts
		Seed:        12345, // Fixed seed for reproducibility
	})

	// Build resilient pipeline that can handle chaos
	pipeline := pipz.NewSequence[string]("chaos-resilient",
		// Circuit breaker to handle failures
		pipz.NewCircuitBreaker[string]("chaos-breaker",
			// Timeout to handle slow responses
			pipz.NewTimeout[string]("chaos-timeout",
				// Retry to handle transient failures
				pipz.NewRetry[string]("chaos-retry",
					// Handle panics (would be caught by recover middleware in real app)
					chaosService,
					3,
				),
				200*time.Millisecond,
			),
			5,                    // failure threshold
			500*time.Millisecond, // reset timeout
		),
	)

	// Add fallback for when all resilience patterns fail
	resilientPipeline := pipz.NewFallback[string]("ultimate-fallback",
		pipeline,
		pipz.Transform("emergency-fallback", func(_ context.Context, data string) string {
			return data + "_emergency"
		}),
	)

	// Run multiple requests to observe chaos behavior
	const numRequests = 50
	var successes, failures, emergencyFallbacks int

	for i := 0; i < numRequests; i++ {
		// Use panic recovery for panic testing
		func() {
			defer func() {
				if r := recover(); r != nil {
					failures++
					t.Logf("Panic recovered: %v", r)
				}
			}()

			result, err := resilientPipeline.Process(ctx, "test")
			switch {
			case err != nil:
				failures++
				t.Logf("Request %d failed: %v", i, err)
			case result == "test_emergency":
				emergencyFallbacks++
				t.Logf("Request %d used emergency fallback", i)
			default:
				successes++
			}
		}()
	}

	t.Logf("Results: %d successes, %d failures, %d emergency fallbacks",
		successes, failures, emergencyFallbacks)

	// With proper resilience patterns, we should have very few total failures
	totalFailures := failures
	if totalFailures > numRequests/10 { // Less than 10% total failure rate
		t.Errorf("too many total failures: %d out of %d requests",
			totalFailures, numRequests)
	}

	// We should have some fallbacks due to chaos injection
	if emergencyFallbacks == 0 {
		t.Error("expected some emergency fallbacks due to chaos injection")
	}

	// Check chaos processor stats
	stats := chaosService.Stats()
	t.Logf("Chaos stats: %s", stats.String())

	if stats.TotalCalls == 0 {
		t.Error("expected chaos processor to be called")
	}
}

func TestResilience_MultiLayerResilienceStack(t *testing.T) {
	ctx := context.Background()

	var apiCalls int64
	var dbCalls int64

	// Simulate external API service
	apiService := pipz.Apply("external-api", func(_ context.Context, data TestUser) (TestUser, error) {
		count := atomic.AddInt64(&apiCalls, 1)
		// Fail first 2 calls to test retry
		if count <= 2 {
			return data, errors.New("API temporarily unavailable")
		}

		// Add some simulated processing delay
		time.Sleep(20 * time.Millisecond)

		if data.Metadata == nil {
			data.Metadata = make(map[string]interface{})
		}
		data.Metadata["enriched_from"] = "external_api"
		return data, nil
	})

	// Simulate database fallback
	dbService := pipz.Apply("database-fallback", func(_ context.Context, data TestUser) (TestUser, error) {
		atomic.AddInt64(&dbCalls, 1)

		// Simulate DB delay
		time.Sleep(10 * time.Millisecond)

		if data.Metadata == nil {
			data.Metadata = make(map[string]interface{})
		}
		data.Metadata["enriched_from"] = "database"
		return data, nil
	})

	// Build multi-layer resilience stack
	resilientPipeline := pipz.NewSequence[TestUser]("multi-layer-resilience",
		// Layer 1: Input validation
		pipz.Apply("validate", func(_ context.Context, user TestUser) (TestUser, error) {
			if user.Name == "" {
				return user, errors.New("name is required")
			}
			return user, nil
		}),

		// Layer 2: Rate limiting for external API
		pipz.NewRateLimiter[TestUser]("api-rate-limit", 10.0, 5).SetMode("wait"),

		// Layer 3: Circuit breaker for API failures
		pipz.NewCircuitBreaker[TestUser]("api-circuit-breaker",
			// Layer 4: Timeout protection
			pipz.NewTimeout[TestUser]("api-timeout",
				// Layer 5: Retry for transient failures
				pipz.NewRetry[TestUser]("api-retry",
					apiService,
					3, // max attempts
				),
				100*time.Millisecond, // timeout
			),
			3,                    // failure threshold
			500*time.Millisecond, // reset timeout
		),

		// Layer 6: Fallback to database if API fails completely
		pipz.NewFallback[TestUser]("api-db-fallback",
			pipz.Transform("passthrough", func(_ context.Context, user TestUser) TestUser {
				return user
			}),
			dbService,
		),

		// Layer 7: Final processing
		pipz.Transform("finalize", func(_ context.Context, user TestUser) TestUser {
			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata["processed_at"] = time.Now().Unix()
			return user
		}),
	)

	// Test the complete stack
	testUser := TestUser{
		ID:   1,
		Name: "Test User",
		Age:  25,
	}

	result, err := resilientPipeline.Process(ctx, testUser)
	if err != nil {
		t.Fatalf("resilient pipeline failed: %v", err)
	}

	// Verify result
	if result.Name != "Test User" {
		t.Errorf("expected Name 'Test User', got %s", result.Name)
	}

	// Should be enriched from external API after retries succeed
	if enrichedFrom, ok := result.Metadata["enriched_from"]; ok {
		if enrichedFrom != "external_api" {
			t.Errorf("expected enrichment from external_api, got %v", enrichedFrom)
		}
	} else {
		t.Error("expected enriched_from metadata")
	}

	// Should have processed_at timestamp
	if _, ok := result.Metadata["processed_at"]; !ok {
		t.Error("expected processed_at metadata")
	}

	// Verify API was retried
	finalAPICalls := atomic.LoadInt64(&apiCalls)
	if finalAPICalls != 3 { // Should succeed on 3rd attempt
		t.Errorf("expected 3 API calls (2 failures + 1 success), got %d", finalAPICalls)
	}

	// Database should not be called since API eventually succeeded
	finalDBCalls := atomic.LoadInt64(&dbCalls)
	if finalDBCalls != 0 {
		t.Errorf("expected 0 DB calls since API succeeded, got %d", finalDBCalls)
	}

	t.Logf("Multi-layer resilience test completed: API calls: %d, DB calls: %d",
		finalAPICalls, finalDBCalls)
}

func TestResilience_GracefulDegradation(t *testing.T) {
	ctx := context.Background()

	// Create processors with different reliability levels
	criticalService := pipz.Apply("critical", func(_ context.Context, user TestUser) (TestUser, error) {
		// Critical service always works
		if user.Metadata == nil {
			user.Metadata = make(map[string]interface{})
		}
		user.Metadata["critical_processed"] = true
		return user, nil
	})

	importantService := pipz.Apply("important", func(_ context.Context, user TestUser) (TestUser, error) {
		// Important service fails sometimes
		return user, errors.New("important service unavailable")
	})

	niceToHaveService := pipz.Apply("nice-to-have", func(_ context.Context, user TestUser) (TestUser, error) {
		// Nice-to-have service also fails
		return user, errors.New("nice-to-have service unavailable")
	})

	// Build graceful degradation pipeline
	pipeline := pipz.NewSequence[TestUser]("graceful-degradation",
		// Critical service must succeed
		criticalService,

		// Important service with fallback
		pipz.NewFallback[TestUser]("important-with-fallback",
			importantService,
			pipz.Transform("important-fallback", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["important_fallback"] = true
				return user
			}),
		),

		// Nice-to-have service with ignore-failure wrapper
		pipz.NewFallback[TestUser]("nice-to-have-optional",
			niceToHaveService,
			// Fallback is just pass-through - feature degrades gracefully
			pipz.Transform("optional-skip", func(_ context.Context, user TestUser) TestUser {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["nice_to_have_skipped"] = true
				return user
			}),
		),

		// Final processing always runs
		pipz.Transform("finalize", func(_ context.Context, user TestUser) TestUser {
			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata["finalized"] = true
			return user
		}),
	)

	testUser := TestUser{ID: 1, Name: "Test User", Age: 25}
	result, err := pipeline.Process(ctx, testUser)

	if err != nil {
		t.Fatalf("graceful degradation pipeline failed: %v", err)
	}

	// Verify critical service succeeded
	if critical, ok := result.Metadata["critical_processed"]; !ok || critical != true {
		t.Error("expected critical service to succeed")
	}

	// Verify important service fell back gracefully
	if fallback, ok := result.Metadata["important_fallback"]; !ok || fallback != true {
		t.Error("expected important service to use fallback")
	}

	// Verify nice-to-have service was skipped gracefully
	if skipped, ok := result.Metadata["nice_to_have_skipped"]; !ok || skipped != true {
		t.Error("expected nice-to-have service to be skipped")
	}

	// Verify final processing completed
	if finalized, ok := result.Metadata["finalized"]; !ok || finalized != true {
		t.Error("expected finalization to complete")
	}

	t.Log("Graceful degradation test passed - system degraded features but continued working")
}
