package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// APIRequest represents a typical API request structure.
type APIRequest struct {
	Method   string                 `json:"method"`
	URL      string                 `json:"url"`
	Headers  map[string]string      `json:"headers"`
	Body     []byte                 `json:"body,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Clone implements pipz.Cloner for safe concurrent processing.
func (r APIRequest) Clone() APIRequest {
	headers := make(map[string]string, len(r.Headers))
	for k, v := range r.Headers {
		headers[k] = v
	}

	body := make([]byte, len(r.Body))
	copy(body, r.Body)

	metadata := make(map[string]interface{}, len(r.Metadata))
	for k, v := range r.Metadata {
		metadata[k] = v
	}

	return APIRequest{
		Method:   r.Method,
		URL:      r.URL,
		Headers:  headers,
		Body:     body,
		Metadata: metadata,
	}
}

// APIResponse represents the response structure.
type APIResponse struct {
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       []byte                 `json:"body,omitempty"`
	Duration   time.Duration          `json:"duration"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Clone implements pipz.Cloner for safe concurrent processing.
func (r APIResponse) Clone() APIResponse {
	headers := make(map[string]string, len(r.Headers))
	for k, v := range r.Headers {
		headers[k] = v
	}

	body := make([]byte, len(r.Body))
	copy(body, r.Body)

	metadata := make(map[string]interface{}, len(r.Metadata))
	for k, v := range r.Metadata {
		metadata[k] = v
	}

	return APIResponse{
		StatusCode: r.StatusCode,
		Headers:    headers,
		Body:       body,
		Duration:   r.Duration,
		Metadata:   metadata,
	}
}

func TestRealWorld_ResilientAPIClient(t *testing.T) {
	// Create a test server with various response behaviors
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)

		switch {
		case count <= 2:
			// First two requests fail with 500
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		case count == 3:
			// Third request succeeds
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // Test server response encoding
				"status":  "success",
				"message": "Request processed successfully",
				"data":    map[string]interface{}{"id": 12345},
			})
		default:
			// Subsequent requests succeed immediately
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck // Test server response encoding
				"status":  "success",
				"message": "Request processed",
			})
		}
	}))
	defer server.Close()

	// Build a production-ready API client pipeline
	apiClient := pipz.NewSequence[APIRequest]("resilient-api-client",
		// Step 1: Request validation and preparation
		pipz.Apply("validate-request", func(_ context.Context, req APIRequest) (APIRequest, error) {
			if req.Method == "" {
				return req, errors.New("method is required")
			}
			if req.URL == "" {
				return req, errors.New("URL is required")
			}

			// Add default headers
			if req.Headers == nil {
				req.Headers = make(map[string]string)
			}
			if req.Headers["User-Agent"] == "" {
				req.Headers["User-Agent"] = "pipz-api-client/1.0"
			}
			if req.Headers["Accept"] == "" {
				req.Headers["Accept"] = "application/json"
			}

			// Initialize metadata
			if req.Metadata == nil {
				req.Metadata = make(map[string]interface{})
			}
			req.Metadata["validated_at"] = time.Now().Unix()

			return req, nil
		}),

		// Step 2-6: Resilient API pipeline with rate limiting, circuit breaker, timeout, retry
		pipz.NewSequence[APIRequest]("resilient-api",
			// Step 2: Rate limiting (10 RPS with burst of 5)
			pipz.NewRateLimiter[APIRequest]("api-rate-limit", 10.0, 5).SetMode("wait"),

			// Step 3: Circuit breaker for API failures
			pipz.NewCircuitBreaker[APIRequest]("api-circuit-breaker",
				// Step 4: Timeout protection (30 seconds)
				pipz.NewTimeout[APIRequest]("api-timeout",
					// Step 5: Retry with exponential backoff
					pipz.NewBackoff[APIRequest]("api-retry",
						// Step 6: Actual HTTP call
						pipz.Apply("http-call", func(ctx context.Context, req APIRequest) (APIRequest, error) {
							start := time.Now()

							// Create HTTP request
							httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, strings.NewReader(string(req.Body)))
							if err != nil {
								return req, fmt.Errorf("failed to create HTTP request: %w", err)
							}

							// Add headers
							for k, v := range req.Headers {
								httpReq.Header.Set(k, v)
							}

							// Make the request
							client := &http.Client{Timeout: 10 * time.Second}
							resp, err := client.Do(httpReq)
							if err != nil {
								return req, fmt.Errorf("HTTP request failed: %w", err)
							}
							defer resp.Body.Close()

							duration := time.Since(start)

							// Handle non-2xx responses as errors for retry logic
							if resp.StatusCode >= 400 {
								return req, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
							}

							// Store response in metadata
							if req.Metadata == nil {
								req.Metadata = make(map[string]interface{})
							}
							req.Metadata["response"] = APIResponse{
								StatusCode: resp.StatusCode,
								Duration:   duration,
								Headers:    make(map[string]string),
							}

							// Copy response headers
							response, _ := req.Metadata["response"].(APIResponse) //nolint:errcheck // Type assertion for response processing
							for k, v := range resp.Header {
								if len(v) > 0 {
									response.Headers[k] = v[0]
								}
							}
							req.Metadata["response"] = response

							return req, nil
						}),
						3,                    // max attempts
						100*time.Millisecond, // initial delay
					),
					30*time.Second, // timeout
				),
				5,           // failure threshold
				time.Minute, // reset timeout
			),
		),

		// Step 7: Response processing and logging
		pipz.Transform("process-response", func(_ context.Context, req APIRequest) APIRequest {
			if req.Metadata == nil {
				req.Metadata = make(map[string]interface{})
			}
			req.Metadata["processed_at"] = time.Now().Unix()
			req.Metadata["pipeline_complete"] = true

			// Log successful response
			if response, ok := req.Metadata["response"].(APIResponse); ok {
				t.Logf("API call successful: %s %s -> %d (took %v)",
					req.Method, req.URL, response.StatusCode, response.Duration)
			}

			return req
		}),
	)

	// Add fallback for complete API failure
	fallbackClient := pipz.NewFallback[APIRequest]("api-with-fallback",
		apiClient,
		pipz.Transform("cache-fallback", func(_ context.Context, req APIRequest) APIRequest {
			if req.Metadata == nil {
				req.Metadata = make(map[string]interface{})
			}
			req.Metadata["fallback_used"] = true
			req.Metadata["response"] = APIResponse{
				StatusCode: 200,
				Body:       []byte(`{"status":"success","source":"cache","message":"Served from fallback cache"}`),
				Headers:    map[string]string{"Content-Type": "application/json"},
				Duration:   time.Millisecond,
			}
			return req
		}),
	)

	// Test the complete API client
	ctx := context.Background()
	request := APIRequest{
		Method: "GET",
		URL:    server.URL + "/api/test",
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
		},
	}

	result, err := fallbackClient.Process(ctx, request)
	if err != nil {
		t.Fatalf("API client pipeline failed: %v", err)
	}

	// Verify the request was processed
	if validated, ok := result.Metadata["validated_at"]; !ok {
		t.Error("expected validation timestamp")
	} else {
		t.Logf("Request validated at: %v", validated)
	}

	if processed, ok := result.Metadata["processed_at"]; !ok {
		t.Error("expected processing timestamp")
	} else {
		t.Logf("Request processed at: %v", processed)
	}

	if complete, ok := result.Metadata["pipeline_complete"]; !ok || complete != true {
		t.Error("expected pipeline to complete successfully")
	}

	// Verify we got a valid response
	if response, ok := result.Metadata["response"].(APIResponse); !ok {
		t.Error("expected response metadata")
	} else {
		if response.StatusCode != 200 {
			t.Errorf("expected status 200, got %d", response.StatusCode)
		}
		if response.Duration <= 0 {
			t.Error("expected positive response duration")
		}
		t.Logf("Response: Status=%d, Duration=%v", response.StatusCode, response.Duration)
	}

	// Verify retry behavior - should have made exactly 3 requests (2 failures + 1 success)
	finalRequestCount := atomic.LoadInt64(&requestCount)
	if finalRequestCount != 3 {
		t.Errorf("expected 3 HTTP requests due to retry, got %d", finalRequestCount)
	}

	// Verify no fallback was used since API eventually succeeded
	if fallback, ok := result.Metadata["fallback_used"]; ok && fallback == true {
		t.Error("expected no fallback usage since API succeeded")
	}
}

func TestRealWorld_EventProcessingPipeline(t *testing.T) {
	// Event represents incoming events to process
	type Event struct {
		ID        string                 `json:"id"`
		Type      string                 `json:"type"`
		Source    string                 `json:"source"`
		Timestamp time.Time              `json:"timestamp"`
		Data      map[string]interface{} `json:"data"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	}

	// Clone implementation for Event
	cloneEvent := func(e Event) Event {
		data := make(map[string]interface{}, len(e.Data))
		for k, v := range e.Data {
			data[k] = v
		}
		metadata := make(map[string]interface{}, len(e.Metadata))
		for k, v := range e.Metadata {
			metadata[k] = v
		}
		return Event{
			ID:        e.ID,
			Type:      e.Type,
			Source:    e.Source,
			Timestamp: e.Timestamp,
			Data:      data,
			Metadata:  metadata,
		}
	}
	_ = cloneEvent // Silence unused warning

	var processedEvents int64
	var filteredEvents int64
	var enrichedEvents int64

	// Build event processing pipeline
	eventPipeline := pipz.NewSequence[Event]("event-processing",
		// Step 1: Event validation
		pipz.Apply("validate-event", func(_ context.Context, event Event) (Event, error) {
			if event.ID == "" {
				return event, errors.New("event ID is required")
			}
			if event.Type == "" {
				return event, errors.New("event type is required")
			}
			if event.Timestamp.IsZero() {
				event.Timestamp = time.Now()
			}

			if event.Metadata == nil {
				event.Metadata = make(map[string]interface{})
			}
			event.Metadata["validated"] = true
			event.Metadata["pipeline_start"] = time.Now().Unix()

			return event, nil
		}),

		// Step 2: Event filtering based on type and age
		pipz.Apply("event-filter", func(_ context.Context, event Event) (Event, error) {
			// Filter out old events (older than 1 hour)
			if time.Since(event.Timestamp) > time.Hour {
				atomic.AddInt64(&filteredEvents, 1)
				return event, errors.New("event filtered: too old")
			}

			// Filter out test events in production
			if event.Source == "test" && event.Data["environment"] == "production" {
				atomic.AddInt64(&filteredEvents, 1)
				return event, errors.New("event filtered: test event in production")
			}

			// Event passes filter - mark it
			if event.Metadata == nil {
				event.Metadata = make(map[string]interface{})
			}
			event.Metadata["passed_filter"] = true
			return event, nil
		}),

		// Step 3: Event enrichment based on type
		pipz.NewSwitch[Event, string]("event-enrichment", func(_ context.Context, event Event) string {
			return event.Type // Route based on event type
		}).
			AddRoute("user.action",
				pipz.Transform("enrich-user", func(_ context.Context, event Event) Event {
					atomic.AddInt64(&enrichedEvents, 1)
					if event.Metadata == nil {
						event.Metadata = make(map[string]interface{})
					}
					event.Metadata["enriched"] = true
					event.Metadata["enrichment_type"] = "user"

					// Simulate user data enrichment
					if userID, ok := event.Data["user_id"]; ok {
						event.Data["user_profile"] = map[string]interface{}{
							"id":       userID,
							"segment":  "premium",
							"location": "US",
						}
					}
					return event
				}),
			).
			AddRoute("system.alert",
				pipz.Transform("enrich-system", func(_ context.Context, event Event) Event {
					atomic.AddInt64(&enrichedEvents, 1)
					if event.Metadata == nil {
						event.Metadata = make(map[string]interface{})
					}
					event.Metadata["enriched"] = true
					event.Metadata["enrichment_type"] = "system"

					// Simulate system context enrichment
					event.Data["system_context"] = map[string]interface{}{
						"service":     "api-gateway",
						"environment": "production",
						"region":      "us-west-2",
					}
					return event
				}),
			),

		// Step 4: Event routing to different processors based on priority
		pipz.NewSequence[Event]("event-routing",
			// High priority events get immediate processing
			pipz.NewFilter[Event]("high-priority",
				func(_ context.Context, event Event) bool {
					priority, ok := event.Data["priority"].(string)
					return ok && priority == "high"
				},
				pipz.Transform("high-priority-process", func(_ context.Context, event Event) Event {
					if event.Metadata == nil {
						event.Metadata = make(map[string]interface{})
					}
					event.Metadata["processed_priority"] = "high"
					event.Metadata["processed_immediately"] = true
					return event
				}),
			),

			// Normal priority events get standard processing
			pipz.NewFilter[Event]("normal-priority",
				func(_ context.Context, event Event) bool {
					priority, ok := event.Data["priority"].(string)
					return !ok || priority == "normal" || priority == ""
				},
				pipz.Transform("normal-priority-process", func(_ context.Context, event Event) Event {
					if event.Metadata == nil {
						event.Metadata = make(map[string]interface{})
					}
					event.Metadata["processed_priority"] = "normal"
					return event
				}),
			),
		),

		// Step 5: Final event processing and storage preparation
		pipz.Transform("finalize-event", func(_ context.Context, event Event) Event {
			atomic.AddInt64(&processedEvents, 1)
			if event.Metadata == nil {
				event.Metadata = make(map[string]interface{})
			}
			event.Metadata["processing_complete"] = true
			event.Metadata["pipeline_end"] = time.Now().Unix()

			// Calculate processing duration
			if start, ok := event.Metadata["pipeline_start"].(int64); ok {
				end, _ := event.Metadata["pipeline_end"].(int64) //nolint:errcheck // Type assertion for duration calculation
				event.Metadata["processing_duration_ms"] = (end - start) * 1000
			}

			return event
		}),
	)

	// Test with various event types and scenarios
	testEvents := []Event{
		{
			ID:        "evt-001",
			Type:      "user.action",
			Source:    "web-app",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"user_id":  "user-123",
				"action":   "login",
				"priority": "normal",
			},
		},
		{
			ID:        "evt-002",
			Type:      "system.alert",
			Source:    "monitoring",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"alert_type": "cpu_high",
				"priority":   "high",
				"threshold":  85.5,
			},
		},
		{
			ID:        "evt-003",
			Type:      "user.action",
			Source:    "test",
			Timestamp: time.Now().Add(-2 * time.Hour), // Old event, should be filtered
			Data: map[string]interface{}{
				"environment": "production",
				"user_id":     "test-user",
			},
		},
		{
			ID:        "evt-004",
			Type:      "analytics.track",
			Source:    "mobile-app",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"event_name": "page_view",
				"priority":   "normal",
			},
		},
	}

	ctx := context.Background()

	// Reset counters
	atomic.StoreInt64(&processedEvents, 0)
	atomic.StoreInt64(&filteredEvents, 0)
	atomic.StoreInt64(&enrichedEvents, 0)

	// Process each event
	results := make([]Event, 0, len(testEvents))
	for i, event := range testEvents {
		result, err := eventPipeline.Process(ctx, event)
		if err != nil {
			// Check if this is a filtering error (expected)
			if strings.Contains(err.Error(), "event filtered:") {
				t.Logf("Event %s filtered as expected: %v", event.ID, err)
				continue
			}
			// Unexpected error
			t.Errorf("event %d processing failed with unexpected error: %v", i, err)
			continue
		}
		results = append(results, result)
	}

	// Verify processing results
	finalProcessed := atomic.LoadInt64(&processedEvents)
	finalFiltered := atomic.LoadInt64(&filteredEvents)
	finalEnriched := atomic.LoadInt64(&enrichedEvents)

	t.Logf("Event processing summary: Processed=%d, Filtered=%d, Enriched=%d",
		finalProcessed, finalFiltered, finalEnriched)

	// Should have processed 3 events (evt-003 should be filtered out)
	if finalProcessed != 3 {
		t.Errorf("expected 3 processed events, got %d", finalProcessed)
	}

	// Should have filtered 1 event (evt-003 - old test event)
	if finalFiltered != 1 {
		t.Errorf("expected 1 filtered event, got %d", finalFiltered)
	}

	// Should have enriched 2 events (evt-001 and evt-002)
	if finalEnriched != 2 {
		t.Errorf("expected 2 enriched events, got %d", finalEnriched)
	}

	// Verify individual event processing
	for _, result := range results {
		if complete, ok := result.Metadata["processing_complete"]; !ok || complete != true {
			t.Errorf("event %s: expected processing_complete=true", result.ID)
		}

		if duration, ok := result.Metadata["processing_duration_ms"]; ok {
			t.Logf("Event %s processed in %v ms", result.ID, duration)
		}

		// Verify enrichment for user and system events
		if result.Type == "user.action" && result.ID == "evt-001" {
			if enrichType, ok := result.Metadata["enrichment_type"]; !ok || enrichType != "user" {
				t.Errorf("event %s: expected user enrichment", result.ID)
			}
			if profile, ok := result.Data["user_profile"]; !ok {
				t.Errorf("event %s: expected user_profile data", result.ID)
			} else {
				t.Logf("Event %s enriched with profile: %v", result.ID, profile)
			}
		}

		if result.Type == "system.alert" {
			if enrichType, ok := result.Metadata["enrichment_type"]; !ok || enrichType != "system" {
				t.Errorf("event %s: expected system enrichment", result.ID)
			}
			if context, ok := result.Data["system_context"]; !ok {
				t.Errorf("event %s: expected system_context data", result.ID)
			} else {
				t.Logf("Event %s enriched with context: %v", result.ID, context)
			}
		}

		// Verify priority processing
		if priority, ok := result.Data["priority"]; ok && priority == "high" {
			if processed, ok := result.Metadata["processed_immediately"]; !ok || processed != true {
				t.Errorf("high priority event %s should be processed immediately", result.ID)
			}
		}
	}
}

func TestRealWorld_DataValidationPipeline(t *testing.T) {
	// UserRegistration represents user registration data that needs validation
	type UserRegistration struct {
		Email     string                 `json:"email"`
		Password  string                 `json:"password"`
		Name      string                 `json:"name"`
		Age       int                    `json:"age"`
		Country   string                 `json:"country"`
		Terms     bool                   `json:"terms_accepted"`
		Marketing bool                   `json:"marketing_consent"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	}

	// Build comprehensive validation pipeline
	validationPipeline := pipz.NewSequence[UserRegistration]("user-validation",
		// Step 1: Basic field validation
		pipz.Apply("basic-validation", func(_ context.Context, user UserRegistration) (UserRegistration, error) {
			var validationErrors []string

			if user.Email == "" {
				validationErrors = append(validationErrors, "email is required")
			}
			if user.Password == "" {
				validationErrors = append(validationErrors, "password is required")
			}
			if user.Name == "" {
				validationErrors = append(validationErrors, "name is required")
			}
			if user.Age < 13 {
				validationErrors = append(validationErrors, "age must be at least 13")
			}
			if !user.Terms {
				validationErrors = append(validationErrors, "terms acceptance is required")
			}

			if len(validationErrors) > 0 {
				return user, fmt.Errorf("validation errors: %s", strings.Join(validationErrors, ", "))
			}

			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata["basic_validation"] = "passed"

			return user, nil
		}),

		// Step 2: Email format validation
		pipz.Apply("email-validation", func(_ context.Context, user UserRegistration) (UserRegistration, error) {
			// Simple email validation
			if !strings.Contains(user.Email, "@") || !strings.Contains(user.Email, ".") {
				return user, errors.New("invalid email format")
			}

			// Normalize email
			user.Email = strings.ToLower(strings.TrimSpace(user.Email))

			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}
			user.Metadata["email_validation"] = "passed"
			user.Metadata["email_normalized"] = true

			return user, nil
		}),

		// Step 3: Password strength validation with fallback
		pipz.NewFallback[UserRegistration]("password-validation",
			// Primary: Strict password validation
			pipz.Apply("strict-password", func(_ context.Context, user UserRegistration) (UserRegistration, error) {
				if len(user.Password) < 12 {
					return user, errors.New("password too weak - must be at least 12 characters")
				}

				hasUpper := strings.ContainsAny(user.Password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
				hasLower := strings.ContainsAny(user.Password, "abcdefghijklmnopqrstuvwxyz")
				hasDigit := strings.ContainsAny(user.Password, "0123456789")
				hasSpecial := strings.ContainsAny(user.Password, "!@#$%^&*()_+-=[]{}|;:,.<>?")

				if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
					return user, errors.New("password must contain uppercase, lowercase, digit, and special character")
				}

				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["password_strength"] = "strong"

				return user, nil
			}),

			// Fallback: Basic password validation
			pipz.Apply("basic-password", func(_ context.Context, user UserRegistration) (UserRegistration, error) {
				if len(user.Password) < 8 {
					return user, errors.New("password must be at least 8 characters")
				}

				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}
				user.Metadata["password_strength"] = "basic"
				user.Metadata["password_fallback"] = true

				return user, nil
			}),
		),

		// Step 4: Concurrent validation checks
		pipz.NewSequence[UserRegistration]("concurrent-validations",
			// Age-based validation
			pipz.Transform("age-validation", func(_ context.Context, user UserRegistration) UserRegistration {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}

				if user.Age < 18 {
					user.Metadata["minor_account"] = true
					user.Metadata["requires_parental_consent"] = true
				} else {
					user.Metadata["adult_account"] = true
				}

				return user
			}),

			// Country-based validation
			pipz.Transform("country-validation", func(_ context.Context, user UserRegistration) UserRegistration {
				if user.Metadata == nil {
					user.Metadata = make(map[string]interface{})
				}

				// Simulate country-specific rules
				switch user.Country {
				case "US", "CA":
					user.Metadata["privacy_law"] = "CCPA"
				case "DE", "FR", "UK":
					user.Metadata["privacy_law"] = "GDPR"
				default:
					user.Metadata["privacy_law"] = "standard"
				}

				return user
			}),
		),

		// Step 5: Final processing and cleanup
		pipz.Transform("finalize-registration", func(_ context.Context, user UserRegistration) UserRegistration {
			if user.Metadata == nil {
				user.Metadata = make(map[string]interface{})
			}

			// Clear password for security (in real app, would be hashed)
			user.Password = "[REDACTED]"

			user.Metadata["validation_complete"] = true
			user.Metadata["registered_at"] = time.Now().Unix()

			return user
		}),
	)

	tests := []struct {
		name        string
		user        UserRegistration
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_adult_registration",
			user: UserRegistration{
				Email:     "john@example.com",
				Password:  "StrongPass123!",
				Name:      "John Doe",
				Age:       25,
				Country:   "US",
				Terms:     true,
				Marketing: true,
			},
			expectError: false,
		},
		{
			name: "valid_minor_registration",
			user: UserRegistration{
				Email:     "teen@example.com",
				Password:  "GoodPass1!",
				Name:      "Teen User",
				Age:       16,
				Country:   "DE",
				Terms:     true,
				Marketing: false,
			},
			expectError: false,
		},
		{
			name: "weak_password_fallback",
			user: UserRegistration{
				Email:     "user@example.com",
				Password:  "simple123", // Fails strict validation, passes basic
				Name:      "Simple User",
				Age:       30,
				Country:   "CA",
				Terms:     true,
				Marketing: false,
			},
			expectError: false,
		},
		{
			name: "missing_email",
			user: UserRegistration{
				Password:  "StrongPass123!",
				Name:      "No Email",
				Age:       25,
				Country:   "US",
				Terms:     true,
				Marketing: false,
			},
			expectError: true,
			errorMsg:    "email is required",
		},
		{
			name: "too_young",
			user: UserRegistration{
				Email:     "child@example.com",
				Password:  "StrongPass123!",
				Name:      "Too Young",
				Age:       10,
				Country:   "US",
				Terms:     true,
				Marketing: false,
			},
			expectError: true,
			errorMsg:    "age must be at least 13",
		},
		{
			name: "invalid_email_format",
			user: UserRegistration{
				Email:     "notanemail",
				Password:  "StrongPass123!",
				Name:      "Bad Email",
				Age:       25,
				Country:   "US",
				Terms:     true,
				Marketing: false,
			},
			expectError: true,
			errorMsg:    "invalid email format",
		},
		{
			name: "terms_not_accepted",
			user: UserRegistration{
				Email:     "user@example.com",
				Password:  "StrongPass123!",
				Name:      "No Terms",
				Age:       25,
				Country:   "US",
				Terms:     false,
				Marketing: false,
			},
			expectError: true,
			errorMsg:    "terms acceptance is required",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validationPipeline.Process(ctx, tt.user)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				t.Logf("Expected error caught: %v", err)
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify successful processing
			if complete, ok := result.Metadata["validation_complete"]; !ok || complete != true {
				t.Error("expected validation_complete=true")
			}

			if registeredAt, ok := result.Metadata["registered_at"]; !ok {
				t.Error("expected registered_at timestamp")
			} else {
				t.Logf("Registration completed at: %v", registeredAt)
			}

			// Verify password was redacted
			if result.Password != "[REDACTED]" {
				t.Error("expected password to be redacted")
			}

			// Verify email normalization
			if !strings.EqualFold(result.Email, tt.user.Email) {
				t.Errorf("expected email to be normalized to lowercase")
			}

			// Verify age-based metadata
			if tt.user.Age < 18 {
				if minor, ok := result.Metadata["minor_account"]; !ok || minor != true {
					t.Error("expected minor_account=true for users under 18")
				}
				if consent, ok := result.Metadata["requires_parental_consent"]; !ok || consent != true {
					t.Error("expected requires_parental_consent=true for minors")
				}
			} else {
				if adult, ok := result.Metadata["adult_account"]; !ok || adult != true {
					t.Error("expected adult_account=true for users 18+")
				}
			}

			// Verify country-based privacy law assignment
			if privacyLaw, ok := result.Metadata["privacy_law"]; ok {
				t.Logf("Privacy law for %s: %v", tt.user.Country, privacyLaw)
			} else {
				t.Error("expected privacy_law metadata")
			}

			// Verify password strength handling
			if strength, ok := result.Metadata["password_strength"]; ok {
				t.Logf("Password strength: %v", strength)
				if tt.user.Password == "simple123" && strength != "basic" {
					t.Error("expected basic password strength for simple password")
				}
			}
		})
	}
}
