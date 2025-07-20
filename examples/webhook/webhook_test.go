package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestIdentifyProvider(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected string
		wantErr  bool
	}{
		{
			name: "Stripe webhook",
			headers: http.Header{
				"Stripe-Signature": []string{"t=123,v1=abc"},
			},
			expected: "stripe",
		},
		{
			name: "GitHub webhook",
			headers: http.Header{
				"X-Hub-Signature-256": []string{"sha256=123abc"},
			},
			expected: "github",
		},
		{
			name: "Slack webhook",
			headers: http.Header{
				"X-Slack-Signature": []string{"v0=123abc"},
			},
			expected: "slack",
		},
		{
			name:     "Unknown provider",
			headers:  http.Header{},
			expected: "",
			wantErr:  true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := Webhook{Headers: tt.headers}
			result, err := IdentifyProvider(ctx, webhook)

			if (err != nil) != tt.wantErr {
				t.Errorf("IdentifyProvider() error = %v, wantErr %v", err, tt.wantErr)
			}

			if result.Provider != tt.expected {
				t.Errorf("Expected provider %s, got %s", tt.expected, result.Provider)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	secrets := map[string]string{
		"stripe": "stripe_secret",
		"github": "github_secret",
		"slack":  "slack_secret",
	}

	tests := []struct {
		name    string
		webhook Webhook
		wantErr bool
	}{
		{
			name: "Valid Stripe signature",
			webhook: Webhook{
				Provider: "stripe",
				Headers: http.Header{
					"Stripe-Signature": []string{generateStripeSignature("stripe_secret", []byte("test"))},
				},
				Body: []byte("test"),
			},
			wantErr: false,
		},
		{
			name: "Invalid Stripe signature",
			webhook: Webhook{
				Provider: "stripe",
				Headers: http.Header{
					"Stripe-Signature": []string{"t=123,v1=wrong"},
				},
				Body: []byte("test"),
			},
			wantErr: true,
		},
		{
			name: "Missing signature",
			webhook: Webhook{
				Provider: "github",
				Headers:  http.Header{},
				Body:     []byte("test"),
			},
			wantErr: true,
		},
	}

	verifier := VerifySignature(secrets)
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := verifier(ctx, tt.webhook)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifySignature() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && !result.Verified {
				t.Error("Expected webhook to be marked as verified")
			}
		})
	}
}

func TestParsePayload(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		body         string
		expectedType string
		expectedID   string
		wantErr      bool
	}{
		{
			name:         "Stripe payment event",
			provider:     "stripe",
			body:         `{"id":"evt_123","type":"payment_intent.succeeded","data":{"object":"payment_intent"}}`,
			expectedType: "payment_intent.succeeded",
			expectedID:   "evt_123",
		},
		{
			name:         "GitHub push event",
			provider:     "github",
			body:         `{"after":"abc123def456","commits":[{"id":"abc123"}]}`,
			expectedType: "push",
			expectedID:   "abc123de",
		},
		{
			name:         "Slack event",
			provider:     "slack",
			body:         `{"type":"event_callback","event_id":"Ev123ABC"}`,
			expectedType: "event_callback",
			expectedID:   "Ev123ABC",
		},
		{
			name:     "Invalid JSON",
			provider: "stripe",
			body:     `{invalid json`,
			wantErr:  true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := Webhook{
				Provider: tt.provider,
				Body:     []byte(tt.body),
			}

			result, err := ParsePayload(ctx, webhook)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePayload() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				if result.EventType != tt.expectedType {
					t.Errorf("Expected event type %s, got %s", tt.expectedType, result.EventType)
				}
				if result.EventID != tt.expectedID {
					t.Errorf("Expected event ID %s, got %s", tt.expectedID, result.EventID)
				}
			}
		})
	}
}

func TestPreventReplay(t *testing.T) {
	// Clear processed webhooks
	processedWebhooks = &sync.Map{}

	ctx := context.Background()
	webhook := Webhook{
		Provider:  "stripe",
		EventID:   "evt_test_123",
		Timestamp: time.Now(),
	}

	// First call should succeed
	err := PreventReplay(ctx, webhook)
	if err != nil {
		t.Errorf("First webhook should not be blocked: %v", err)
	}

	// Second call should fail (replay)
	err = PreventReplay(ctx, webhook)
	if err == nil {
		t.Error("Replay webhook should be blocked")
	}
	if !strings.Contains(err.Error(), "already processed") {
		t.Errorf("Expected 'already processed' error, got: %v", err)
	}

	// Test old timestamp
	oldWebhook := Webhook{
		Provider:  "github",
		EventID:   "evt_old",
		Timestamp: time.Now().Add(-15 * time.Minute),
	}

	err = PreventReplay(ctx, oldWebhook)
	if err == nil {
		t.Error("Old webhook should be blocked")
	}
	if !strings.Contains(err.Error(), "too old") {
		t.Errorf("Expected 'too old' error, got: %v", err)
	}
}

func TestWebhookPipeline(t *testing.T) {
	secrets := map[string]string{
		"stripe": "test_secret",
		"github": "test_secret",
		"slack":  "test_secret",
	}

	pipeline := CreateWebhookPipeline(secrets)
	ctx := context.Background()

	// Clear state
	ResetWebhookState()

	t.Run("Valid Stripe webhook", func(t *testing.T) {
		webhook := Webhook{
			Headers: http.Header{
				"Stripe-Signature": []string{generateStripeSignature("test_secret", []byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{}}`))},
			},
			Body:       []byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{}}`),
			ReceivedAt: time.Now(),
		}

		result, err := pipeline.Process(ctx, webhook)
		if err != nil {
			t.Fatalf("Pipeline failed: %v", err)
		}

		if result.Provider != "stripe" {
			t.Errorf("Expected provider stripe, got %s", result.Provider)
		}
		if !result.Verified {
			t.Error("Expected webhook to be verified")
		}
		if result.ProcessedAt.IsZero() {
			t.Error("Expected ProcessedAt to be set")
		}
	})

	t.Run("Invalid signature", func(t *testing.T) {
		webhook := Webhook{
			Headers: http.Header{
				"X-Hub-Signature-256": []string{"sha256=invalid"},
			},
			Body:       []byte(`{"after":"abc123"}`),
			ReceivedAt: time.Now(),
		}

		_, err := pipeline.Process(ctx, webhook)
		if err == nil {
			t.Error("Expected signature verification to fail")
		}
	})

	t.Run("Replay attack", func(t *testing.T) {
		// Process once
		webhook := Webhook{
			Headers: http.Header{
				"Stripe-Signature": []string{generateStripeSignature("test_secret", []byte(`{"id":"evt_2","type":"charge.succeeded","data":{}}`))},
			},
			Body:       []byte(`{"id":"evt_2","type":"charge.succeeded","data":{}}`),
			ReceivedAt: time.Now(),
		}

		_, err := pipeline.Process(ctx, webhook)
		if err != nil {
			t.Fatalf("First processing failed: %v", err)
		}

		// Try to replay
		_, err = pipeline.Process(ctx, webhook)
		if err == nil {
			t.Error("Replay should have been prevented")
		}
	})
}

func TestProviderHandlers(t *testing.T) {
	ctx := context.Background()

	t.Run("Stripe handler", func(t *testing.T) {
		webhook := Webhook{
			Provider:  "stripe",
			EventType: "payment_intent.succeeded",
			EventID:   "pi_123",
		}

		result, err := HandleStripeEvent(ctx, webhook)
		if err != nil {
			t.Errorf("HandleStripeEvent failed: %v", err)
		}
		if result.ProcessedAt.IsZero() {
			t.Error("Expected ProcessedAt to be set")
		}
	})

	t.Run("GitHub handler", func(t *testing.T) {
		webhook := Webhook{
			Provider:  "github",
			EventType: "push",
			EventID:   "abc123",
		}

		result, err := HandleGitHubEvent(ctx, webhook)
		if err != nil {
			t.Errorf("HandleGitHubEvent failed: %v", err)
		}
		if result.ProcessedAt.IsZero() {
			t.Error("Expected ProcessedAt to be set")
		}
	})

	t.Run("Slack handler", func(t *testing.T) {
		webhook := Webhook{
			Provider:  "slack",
			EventType: "event_callback",
			EventID:   "Ev123",
		}

		result, err := HandleSlackEvent(ctx, webhook)
		if err != nil {
			t.Errorf("HandleSlackEvent failed: %v", err)
		}
		if result.ProcessedAt.IsZero() {
			t.Error("Expected ProcessedAt to be set")
		}
	})
}

func TestRouting(t *testing.T) {
	// Test that Switch connector routes correctly
	routeFunc := func(ctx context.Context, w Webhook) string {
		return w.Provider
	}

	handlers := map[string]pipz.Chainable[Webhook]{
		"stripe": pipz.Apply("stripe", func(ctx context.Context, w Webhook) (Webhook, error) {
			w.ProcessedAt = time.Now()
			return w, nil
		}),
		"github": pipz.Apply("github", func(ctx context.Context, w Webhook) (Webhook, error) {
			return w, fmt.Errorf("github error")
		}),
	}

	router := pipz.Switch(routeFunc, handlers)
	ctx := context.Background()

	// Test successful routing
	webhook1 := Webhook{Provider: "stripe"}
	result1, err := router.Process(ctx, webhook1)
	if err != nil {
		t.Errorf("Stripe routing failed: %v", err)
	}
	if result1.ProcessedAt.IsZero() {
		t.Error("Stripe handler didn't run")
	}

	// Test error routing
	webhook2 := Webhook{Provider: "github"}
	_, err = router.Process(ctx, webhook2)
	if err == nil || !strings.Contains(err.Error(), "github error") {
		t.Error("Expected github error")
	}
}

func TestWebhookStats(t *testing.T) {
	// Reset stats
	globalStats = &WebhookStats{
		ByProvider: make(map[string]int64),
		ByType:     make(map[string]int64),
	}

	// Update with some webhooks
	UpdateStats(Webhook{Provider: "stripe", EventType: "payment_intent.succeeded"}, nil)
	UpdateStats(Webhook{Provider: "stripe", EventType: "charge.failed"}, fmt.Errorf("error"))
	UpdateStats(Webhook{Provider: "github", EventType: "push"}, nil)

	stats := GetStats()

	if stats.Total != 3 {
		t.Errorf("Expected 3 total webhooks, got %d", stats.Total)
	}

	if stats.ByProvider["stripe"] != 2 {
		t.Errorf("Expected 2 Stripe webhooks, got %d", stats.ByProvider["stripe"])
	}

	if stats.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", stats.Errors)
	}
}

func TestRetryWithBackoff(t *testing.T) {
	attempts := 0
	failing := pipz.Apply("failing", func(ctx context.Context, w Webhook) (Webhook, error) {
		attempts++
		if attempts < 3 {
			return w, fmt.Errorf("temporary error")
		}
		return w, nil
	})

	pipeline := pipz.RetryWithBackoff(failing, 3, 10*time.Millisecond)

	ctx := context.Background()
	webhook := Webhook{EventID: "test"}

	_, err := pipeline.Process(ctx, webhook)
	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// Helper functions

func generateStripeSignature(secret string, body []byte) string {
	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("%d.%s", timestamp, string(body))
	sig := computeHMAC(secret, []byte(payload))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}
