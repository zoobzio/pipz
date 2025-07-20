// Package webhook demonstrates using pipz for processing webhooks from multiple
// providers with signature verification, replay protection, and routing.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// Webhook represents an incoming webhook request
type Webhook struct {
	// Request data
	Provider  string
	Headers   http.Header
	Body      []byte
	Signature string

	// Parsed data
	EventType string
	EventID   string
	Timestamp time.Time
	Payload   interface{}

	// Processing metadata
	ReceivedAt  time.Time
	Verified    bool
	ProcessedAt time.Time
}

// Provider configurations
type ProviderConfig struct {
	Name            string
	SignatureHeader string
	TimestampHeader string
	SigningSecret   string
	VerifyFunc      func(secret string, signature string, body []byte) error
	ParseFunc       func(body []byte) (eventType string, eventID string, payload interface{}, err error)
}

// Provider registry
var providers = map[string]ProviderConfig{
	"stripe": {
		Name:            "stripe",
		SignatureHeader: "Stripe-Signature",
		VerifyFunc:      verifyStripeSignature,
		ParseFunc:       parseStripePayload,
	},
	"github": {
		Name:            "github",
		SignatureHeader: "X-Hub-Signature-256",
		VerifyFunc:      verifyGitHubSignature,
		ParseFunc:       parseGitHubPayload,
	},
	"slack": {
		Name:            "slack",
		SignatureHeader: "X-Slack-Signature",
		TimestampHeader: "X-Slack-Request-Timestamp",
		VerifyFunc:      verifySlackSignature,
		ParseFunc:       parseSlackPayload,
	},
}

// Replay attack prevention
var processedWebhooks = &sync.Map{} // event_id -> timestamp
var cleanupOnce sync.Once

// Signature Verification

func verifyStripeSignature(secret, signature string, body []byte) error {
	// Stripe signature format: t=timestamp,v1=signature
	parts := strings.Split(signature, ",")
	if len(parts) != 2 {
		return errors.New("invalid Stripe signature format")
	}

	timestamp := strings.TrimPrefix(parts[0], "t=")
	sig := strings.TrimPrefix(parts[1], "v1=")

	// Verify timestamp (prevent replay attacks)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return errors.New("webhook timestamp too old")
	}

	// Compute expected signature
	payload := fmt.Sprintf("%s.%s", timestamp, string(body))
	expectedSig := computeHMAC(secret, []byte(payload))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return errors.New("signature verification failed")
	}

	return nil
}

func verifyGitHubSignature(secret, signature string, body []byte) error {
	if !strings.HasPrefix(signature, "sha256=") {
		return errors.New("invalid GitHub signature format")
	}

	sig := strings.TrimPrefix(signature, "sha256=")
	expectedSig := computeHMAC(secret, body)

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return errors.New("signature verification failed")
	}

	return nil
}

func verifySlackSignature(secret, signature string, body []byte) error {
	// Slack requires timestamp from header
	// In real implementation, pass timestamp through context or webhook struct
	baseString := fmt.Sprintf("v0:%d:%s", time.Now().Unix(), string(body))
	expectedSig := "v0=" + computeHMAC(secret, []byte(baseString))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return errors.New("signature verification failed")
	}

	return nil
}

func computeHMAC(secret string, data []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Payload Parsing

func parseStripePayload(body []byte) (string, string, interface{}, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", nil, err
	}

	eventType, _ := payload["type"].(string)
	eventID, _ := payload["id"].(string)

	return eventType, eventID, payload["data"], nil
}

func parseGitHubPayload(body []byte) (string, string, interface{}, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", nil, err
	}

	// GitHub sends event type in header, not body
	// For demo, we'll infer from payload
	eventType := "push" // Would come from X-GitHub-Event header

	// Generate event ID from payload
	var eventID string
	if id, ok := payload["after"].(string); ok {
		eventID = id[:8] // Short commit SHA
	} else if pr, ok := payload["pull_request"].(map[string]interface{}); ok {
		if id, ok := pr["id"].(float64); ok {
			eventID = fmt.Sprintf("pr-%d", int(id))
		}
	}

	return eventType, eventID, payload, nil
}

func parseSlackPayload(body []byte) (string, string, interface{}, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", nil, err
	}

	eventType, _ := payload["type"].(string)
	eventID, _ := payload["event_id"].(string)

	return eventType, eventID, payload, nil
}

// Pipeline Processors

// IdentifyProvider determines which provider sent the webhook
func IdentifyProvider(_ context.Context, webhook Webhook) (Webhook, error) {
	// Check headers for provider signatures
	if webhook.Headers.Get("Stripe-Signature") != "" {
		webhook.Provider = "stripe"
	} else if webhook.Headers.Get("X-Hub-Signature-256") != "" {
		webhook.Provider = "github"
	} else if webhook.Headers.Get("X-Slack-Signature") != "" {
		webhook.Provider = "slack"
	} else {
		return webhook, errors.New("unknown webhook provider")
	}

	return webhook, nil
}

// VerifySignature validates the webhook signature
func VerifySignature(secrets map[string]string) func(context.Context, Webhook) (Webhook, error) {
	return func(_ context.Context, webhook Webhook) (Webhook, error) {
		config, ok := providers[webhook.Provider]
		if !ok {
			return webhook, fmt.Errorf("unknown provider: %s", webhook.Provider)
		}

		secret, ok := secrets[webhook.Provider]
		if !ok {
			return webhook, fmt.Errorf("no secret configured for provider: %s", webhook.Provider)
		}

		signature := webhook.Headers.Get(config.SignatureHeader)
		if signature == "" {
			return webhook, errors.New("missing signature header")
		}

		if err := config.VerifyFunc(secret, signature, webhook.Body); err != nil {
			return webhook, fmt.Errorf("signature verification failed: %w", err)
		}

		webhook.Verified = true
		return webhook, nil
	}
}

// ParsePayload extracts event data from the webhook body
func ParsePayload(_ context.Context, webhook Webhook) (Webhook, error) {
	config, ok := providers[webhook.Provider]
	if !ok {
		return webhook, fmt.Errorf("unknown provider: %s", webhook.Provider)
	}

	eventType, eventID, payload, err := config.ParseFunc(webhook.Body)
	if err != nil {
		return webhook, fmt.Errorf("failed to parse payload: %w", err)
	}

	webhook.EventType = eventType
	webhook.EventID = eventID
	webhook.Payload = payload
	webhook.Timestamp = time.Now() // Would be extracted from payload

	return webhook, nil
}

// PreventReplay checks if this webhook was already processed
func PreventReplay(_ context.Context, webhook Webhook) error {
	if webhook.EventID == "" {
		return errors.New("missing event ID for replay prevention")
	}

	key := fmt.Sprintf("%s:%s", webhook.Provider, webhook.EventID)

	// Check if already processed
	if existing, ok := processedWebhooks.Load(key); ok {
		existingTime := existing.(time.Time)
		return fmt.Errorf("webhook already processed at %v", existingTime)
	}

	// Check timestamp (prevent old webhook replay)
	if time.Since(webhook.Timestamp) > 10*time.Minute {
		return errors.New("webhook timestamp too old")
	}

	// Mark as processed
	processedWebhooks.Store(key, time.Now())

	// Start cleanup goroutine only once
	cleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cleanupOldWebhooks()
			}
		}()
	})

	return nil
}

// ValidatePayload performs provider-specific validation
func ValidatePayload(_ context.Context, webhook Webhook) error {
	switch webhook.Provider {
	case "stripe":
		// Validate Stripe-specific fields
		if webhook.EventType == "" {
			return errors.New("missing event type")
		}
		if webhook.EventID == "" {
			return errors.New("missing event ID")
		}

	case "github":
		// Validate GitHub-specific fields
		if webhook.EventType == "push" {
			if payload, ok := webhook.Payload.(map[string]interface{}); ok {
				if commits, ok := payload["commits"].([]interface{}); ok {
					if len(commits) == 0 {
						return errors.New("push event with no commits")
					}
				}
			}
		}

	case "slack":
		// Validate Slack-specific fields
		if webhook.EventType == "" {
			return errors.New("missing event type")
		}
	}

	return nil
}

// Event Handlers

// HandleStripeEvent processes Stripe webhooks
func HandleStripeEvent(_ context.Context, webhook Webhook) (Webhook, error) {
	fmt.Printf("[STRIPE] Processing %s event %s\n", webhook.EventType, webhook.EventID)

	switch webhook.EventType {
	case "payment_intent.succeeded":
		// Handle successful payment
		fmt.Println("[STRIPE] Payment succeeded")

	case "customer.subscription.updated":
		// Handle subscription changes
		fmt.Println("[STRIPE] Subscription updated")

	case "invoice.payment_failed":
		// Handle failed payments
		fmt.Println("[STRIPE] Payment failed - sending notification")

	default:
		fmt.Printf("[STRIPE] Unhandled event type: %s\n", webhook.EventType)
	}

	webhook.ProcessedAt = time.Now()
	return webhook, nil
}

// HandleGitHubEvent processes GitHub webhooks
func HandleGitHubEvent(_ context.Context, webhook Webhook) (Webhook, error) {
	fmt.Printf("[GITHUB] Processing %s event %s\n", webhook.EventType, webhook.EventID)

	switch webhook.EventType {
	case "push":
		// Trigger CI/CD pipeline
		fmt.Println("[GITHUB] Triggering build pipeline")

	case "pull_request":
		// Run automated checks
		fmt.Println("[GITHUB] Running PR checks")

	case "issues":
		// Update issue tracker
		fmt.Println("[GITHUB] Syncing issue tracker")

	default:
		fmt.Printf("[GITHUB] Unhandled event type: %s\n", webhook.EventType)
	}

	webhook.ProcessedAt = time.Now()
	return webhook, nil
}

// HandleSlackEvent processes Slack webhooks
func HandleSlackEvent(_ context.Context, webhook Webhook) (Webhook, error) {
	fmt.Printf("[SLACK] Processing %s event %s\n", webhook.EventType, webhook.EventID)

	switch webhook.EventType {
	case "url_verification":
		// Respond to Slack challenge
		fmt.Println("[SLACK] URL verification challenge")

	case "event_callback":
		// Handle app events
		fmt.Println("[SLACK] Processing app event")

	case "slash_command":
		// Handle slash commands
		fmt.Println("[SLACK] Executing slash command")

	default:
		fmt.Printf("[SLACK] Unhandled event type: %s\n", webhook.EventType)
	}

	webhook.ProcessedAt = time.Now()
	return webhook, nil
}

// AuditLog records all webhook processing
func AuditLog(_ context.Context, webhook Webhook) error {
	entry := map[string]interface{}{
		"provider":     webhook.Provider,
		"event_type":   webhook.EventType,
		"event_id":     webhook.EventID,
		"received_at":  webhook.ReceivedAt,
		"processed_at": webhook.ProcessedAt,
		"verified":     webhook.Verified,
		"duration":     webhook.ProcessedAt.Sub(webhook.ReceivedAt),
	}

	// In production, write to audit log
	fmt.Printf("[AUDIT] %+v\n", entry)
	return nil
}

// Pipeline Builders

// CreateWebhookPipeline creates the main webhook processing pipeline
func CreateWebhookPipeline(secrets map[string]string) pipz.Chainable[Webhook] {
	// Route by provider
	routeByProvider := func(_ context.Context, webhook Webhook) string {
		return webhook.Provider
	}

	// Provider-specific handlers
	handlers := map[string]pipz.Chainable[Webhook]{
		"stripe": pipz.Apply("handle_stripe", HandleStripeEvent),
		"github": pipz.Apply("handle_github", HandleGitHubEvent),
		"slack":  pipz.Apply("handle_slack", HandleSlackEvent),
		"default": pipz.Apply("unknown_provider", func(_ context.Context, w Webhook) (Webhook, error) {
			return w, fmt.Errorf("no handler for provider: %s", w.Provider)
		}),
	}

	return pipz.Sequential(
		// Identify and verify
		pipz.Apply("identify_provider", IdentifyProvider),
		pipz.Apply("verify_signature", VerifySignature(secrets)),
		pipz.Apply("parse_payload", ParsePayload),

		// Security checks
		pipz.Effect("prevent_replay", PreventReplay),
		pipz.Effect("validate_payload", ValidatePayload),

		// Route to provider handler
		pipz.Switch(routeByProvider, handlers),

		// Always audit
		pipz.Effect("audit_log", AuditLog),
	)
}

// CreateWebhookHandler creates an HTTP handler using the pipeline
func CreateWebhookHandler(pipeline pipz.Chainable[Webhook]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Create webhook
		webhook := Webhook{
			Headers:    r.Header,
			Body:       body,
			ReceivedAt: time.Now(),
		}

		// Process through pipeline
		ctx := r.Context()
		result, err := pipeline.Process(ctx, webhook)
		if err != nil {
			// Log error but return 200 to prevent retries
			fmt.Printf("Webhook processing error: %v\n", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"error"}`))
			return
		}

		// Success response
		response := map[string]interface{}{
			"status":     "success",
			"event_id":   result.EventID,
			"event_type": result.EventType,
			"provider":   result.Provider,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// Utility Functions

func cleanupOldWebhooks() {
	cutoff := time.Now().Add(-1 * time.Hour)

	processedWebhooks.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(time.Time); ok {
			if timestamp.Before(cutoff) {
				processedWebhooks.Delete(key)
			}
		}
		return true
	})
}

// ResetWebhookState clears all processed webhooks (for testing)
func ResetWebhookState() {
	processedWebhooks.Range(func(key, value interface{}) bool {
		processedWebhooks.Delete(key)
		return true
	})
}

// ConfigureProvider allows runtime configuration of providers
func ConfigureProvider(name string, config ProviderConfig) {
	providers[name] = config
}

// WebhookStats tracks processing statistics
type WebhookStats struct {
	Total      int64
	ByProvider map[string]int64
	ByType     map[string]int64
	Errors     int64
	mu         sync.RWMutex
}

var globalStats = &WebhookStats{
	ByProvider: make(map[string]int64),
	ByType:     make(map[string]int64),
}

// UpdateStats updates webhook processing statistics
func UpdateStats(webhook Webhook, err error) {
	globalStats.mu.Lock()
	defer globalStats.mu.Unlock()

	globalStats.Total++
	globalStats.ByProvider[webhook.Provider]++

	if webhook.EventType != "" {
		globalStats.ByType[webhook.EventType]++
	}

	if err != nil {
		globalStats.Errors++
	}
}

// GetStats returns current statistics
func GetStats() WebhookStats {
	globalStats.mu.RLock()
	defer globalStats.mu.RUnlock()

	return WebhookStats{
		Total:      globalStats.Total,
		ByProvider: copyMap(globalStats.ByProvider),
		ByType:     copyMap(globalStats.ByType),
		Errors:     globalStats.Errors,
	}
}

func copyMap(m map[string]int64) map[string]int64 {
	copy := make(map[string]int64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}
