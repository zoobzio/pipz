package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/webhook"
)

// WebhookExample implements the Example interface for webhook processing
type WebhookExample struct{}

func (w *WebhookExample) Name() string {
	return "webhook"
}

func (w *WebhookExample) Description() string {
	return "Multi-provider webhook processing with verification"
}

func (w *WebhookExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ WEBHOOK PROCESSING EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Multi-provider support (Stripe, GitHub, Slack)")
	fmt.Println("• Signature verification for security")
	fmt.Println("• Replay attack prevention")
	fmt.Println("• Dynamic routing based on provider")
	fmt.Println("• Comprehensive audit logging")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Webhook processing involves complex security and routing:")
	fmt.Println(colorGray + `
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    body, _ := ioutil.ReadAll(r.Body)
    
    // Identify provider
    var provider string
    if r.Header.Get("Stripe-Signature") != "" {
        provider = "stripe"
    } else if r.Header.Get("X-Hub-Signature-256") != "" {
        provider = "github"
    } else if r.Header.Get("X-Slack-Signature") != "" {
        provider = "slack"
    } else {
        http.Error(w, "Unknown provider", 400)
        return
    }
    
    // Verify signature (each provider is different!)
    switch provider {
    case "stripe":
        sig := r.Header.Get("Stripe-Signature")
        if !verifyStripeSignature(sig, body, stripeSecret) {
            http.Error(w, "Invalid signature", 401)
            return
        }
    case "github":
        // Different verification logic...
    // ... more providers
    }
    
    // Check for replay attacks
    eventID := extractEventID(provider, body)
    if wasProcessed(eventID) {
        w.WriteHeader(200) // Return success to prevent retries
        return
    }
    
    // Parse and route
    switch provider {
    case "stripe":
        var stripeEvent StripeEvent
        json.Unmarshal(body, &stripeEvent)
        
        switch stripeEvent.Type {
        case "payment_intent.succeeded":
            handlePaymentSuccess(stripeEvent)
        case "customer.subscription.created":
            handleNewSubscription(stripeEvent)
        // ... many more event types
        }
    // ... other providers with their own event types
    }
    
    // Audit logging
    logWebhook(provider, eventID, time.Now())
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Compose a secure, maintainable webhook pipeline:")
	fmt.Println(colorGray + `
// Provider identification
identifyProvider := pipz.Apply("identify", func(ctx context.Context, w Webhook) (Webhook, error) {
    if w.Headers.Get("Stripe-Signature") != "" {
        w.Provider = "stripe"
    } else if w.Headers.Get("X-Hub-Signature-256") != "" {
        w.Provider = "github"
    } else if w.Headers.Get("X-Slack-Signature") != "" {
        w.Provider = "slack"
    } else {
        return w, errors.New("unknown provider")
    }
    return w, nil
})

// Signature verification (with provider-specific logic)
verifySignature := pipz.Apply("verify", func(ctx context.Context, w Webhook) (Webhook, error) {
    secret := secrets[w.Provider]
    
    switch w.Provider {
    case "stripe":
        return verifyStripe(w, secret)
    case "github":
        return verifyGitHub(w, secret)
    case "slack":
        return verifySlack(w, secret)
    }
    return w, errors.New("unsupported provider")
})

// Route by provider
routeByProvider := func(ctx context.Context, w Webhook) string {
    return w.Provider
}

handlers := map[string]pipz.Chainable[Webhook]{
    "stripe": stripeHandler,
    "github": githubHandler,
    "slack":  slackHandler,
}

// Build the pipeline
webhookPipeline := pipz.Sequential(
    identifyProvider,
    verifySignature,
    pipz.Apply("parse", parsePayload),
    pipz.Effect("prevent_replay", checkReplay),
    pipz.Switch(routeByProvider, handlers),
    pipz.Effect("audit", logWebhook),
)

// Clean HTTP handler
func webhookHandler(pipeline pipz.Chainable[Webhook]) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        webhook := Webhook{
            Headers:    r.Header,
            Body:       readBody(r),
            ReceivedAt: time.Now(),
        }
        
        result, err := pipeline.Process(r.Context(), webhook)
        if err != nil {
            // Log but return 200 to prevent retries
            log.Printf("Webhook error: %v", err)
            w.WriteHeader(200)
            json.NewEncoder(w).Encode(map[string]string{
                "status": "received",
            })
            return
        }
        
        json.NewEncoder(w).Encode(map[string]string{
            "status": "processed",
            "id":     result.EventID,
        })
    }
}` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's process some webhooks!" + colorReset)
	return w.runInteractive(ctx)
}

func (w *WebhookExample) runInteractive(ctx context.Context) error {
	// Setup
	webhook.ResetWebhookState()
	secrets := map[string]string{
		"stripe": "test_stripe_secret",
		"github": "test_github_secret",
		"slack":  "test_slack_secret",
	}

	pipeline := webhook.CreateWebhookPipeline(secrets)

	// Test scenarios
	scenarios := []struct {
		name     string
		webhook  webhook.Webhook
		valid    bool
		replay   bool
		provider string
		event    string
	}{
		{
			name: "Valid Stripe Payment Event",
			webhook: webhook.Webhook{
				Headers: http.Header{
					"Stripe-Signature": []string{generateTestStripeSignature("test_stripe_secret", `{"id":"evt_123","type":"payment_intent.succeeded"}`)},
				},
				Body:       []byte(`{"id":"evt_123","type":"payment_intent.succeeded","data":{"object":{"amount":9999,"currency":"usd"}}}`),
				ReceivedAt: time.Now(),
			},
			valid:    true,
			provider: "stripe",
			event:    "payment_intent.succeeded",
		},
		{
			name: "Invalid Signature",
			webhook: webhook.Webhook{
				Headers: http.Header{
					"X-Hub-Signature-256": []string{"sha256=invalid_signature"},
				},
				Body:       []byte(`{"action":"opened","number":42}`),
				ReceivedAt: time.Now(),
			},
			valid: false,
		},
		{
			name: "GitHub Push Event",
			webhook: webhook.Webhook{
				Headers: http.Header{
					"X-Hub-Signature-256": []string{generateTestGitHubSignature("test_github_secret", `{"ref":"refs/heads/main","commits":[{"id":"abc123"}]}`)},
				},
				Body:       []byte(`{"ref":"refs/heads/main","after":"abc123def","commits":[{"id":"abc123","message":"Update README"}]}`),
				ReceivedAt: time.Now(),
			},
			valid:    true,
			provider: "github",
			event:    "push",
		},
		{
			name: "Replay Attack (Duplicate Event)",
			webhook: webhook.Webhook{
				Headers: http.Header{
					"Stripe-Signature": []string{generateTestStripeSignature("test_stripe_secret", `{"id":"evt_123","type":"payment_intent.succeeded"}`)},
				},
				Body:       []byte(`{"id":"evt_123","type":"payment_intent.succeeded","data":{}}`),
				ReceivedAt: time.Now(),
			},
			valid:  false,
			replay: true,
		},
		{
			name: "Slack Interactive Event",
			webhook: webhook.Webhook{
				Headers: http.Header{
					"X-Slack-Signature":         []string{generateTestSlackSignature("test_slack_secret", `{"type":"event_callback","event_id":"Ev123"}`)},
					"X-Slack-Request-Timestamp": []string{fmt.Sprintf("%d", time.Now().Unix())},
				},
				Body:       []byte(`{"type":"event_callback","event_id":"Ev123","event":{"type":"message","text":"Hello"}}`),
				ReceivedAt: time.Now(),
			},
			valid:    true,
			provider: "slack",
			event:    "event_callback",
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		// Show webhook details
		fmt.Printf("\n%sIncoming Webhook:%s\n", colorGray, colorReset)
		for key, values := range scenario.webhook.Headers {
			if strings.Contains(key, "Signature") {
				fmt.Printf("  %s: %s\n", key, values[0])
			}
		}
		
		// Show body preview
		bodyStr := string(scenario.webhook.Body)
		if len(bodyStr) > 100 {
			bodyStr = bodyStr[:100] + "..."
		}
		fmt.Printf("  Body: %s\n", bodyStr)

		// Process webhook
		fmt.Printf("\n%sProcessing...%s\n", colorYellow, colorReset)
		start := time.Now()

		result, err := pipeline.Process(ctx, scenario.webhook)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("\n%s❌ Webhook Rejected%s\n", colorRed, colorReset)
			
			// Show detailed error
			var pipelineErr *pipz.PipelineError[webhook.Webhook]
			if errors.As(err, &pipelineErr) {
				fmt.Printf("  Failed at: %s%s%s (stage %d)\n",
					colorYellow, pipelineErr.ProcessorName, colorReset, 
					pipelineErr.StageIndex)
			}

			if scenario.replay {
				fmt.Printf("  Reason: %sDuplicate event detected (replay attack prevention)%s\n",
					colorYellow, colorReset)
			} else {
				fmt.Printf("  Reason: %s\n", err.Error())
			}
		} else {
			fmt.Printf("\n%s✅ Webhook Processed%s\n", colorGreen, colorReset)
			fmt.Printf("  Provider: %s\n", result.Provider)
			fmt.Printf("  Event Type: %s\n", result.EventType)
			fmt.Printf("  Event ID: %s\n", result.EventID)
			fmt.Printf("  Verified: %v\n", result.Verified)
			
			// Show payload sample
			if result.Payload != nil {
				fmt.Printf("  Payload: %+v\n", result.Payload)
			}
		}

		fmt.Printf("  Processing Time: %s%v%s\n", colorCyan, duration, colorReset)

		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show statistics
	fmt.Printf("\n%s═══ PROCESSING STATISTICS ═══%s\n", colorCyan, colorReset)
	stats := webhook.GetStats()
	
	fmt.Printf("\nTotal Webhooks: %d\n", stats.Total)
	fmt.Printf("Errors: %d\n", stats.Errors)
	
	fmt.Printf("\nBy Provider:\n")
	for provider, count := range stats.ByProvider {
		fmt.Printf("  %s: %d\n", provider, count)
	}
	
	fmt.Printf("\nBy Event Type:\n")
	for eventType, count := range stats.ByType {
		fmt.Printf("  %s: %d\n", eventType, count)
	}

	// Show performance tip
	fmt.Printf("\n%sPerformance Tip:%s\n", colorYellow, colorReset)
	fmt.Printf("Use goroutines to process webhooks asynchronously:\n")
	fmt.Printf(colorGray + `
// Return immediately to webhook provider
go func() {
    result, err := pipeline.Process(ctx, webhook)
    if err != nil {
        // Send to dead letter queue
        dlq.Send(webhook, err)
    }
}()

// Respond with 200 immediately
w.WriteHeader(200)` + colorReset)

	return nil
}

// Test signature generators
func generateTestStripeSignature(secret string, payload string) string {
	timestamp := time.Now().Unix()
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedPayload))
	signature := hex.EncodeToString(h.Sum(nil))
	
	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}

func generateTestGitHubSignature(secret string, payload string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func generateTestSlackSignature(secret string, payload string) string {
	timestamp := time.Now().Unix()
	baseString := fmt.Sprintf("v0:%d:%s", timestamp, payload)
	
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(baseString))
	
	return "v0=" + hex.EncodeToString(h.Sum(nil))
}

func (w *WebhookExample) Benchmark(b *testing.B) error {
	return nil
}