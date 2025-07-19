package webhook

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func BenchmarkIdentifyProvider(b *testing.B) {
	headers := []http.Header{
		{"Stripe-Signature": []string{"t=123,v1=abc"}},
		{"X-Hub-Signature-256": []string{"sha256=123"}},
		{"X-Slack-Signature": []string{"v0=123"}},
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		webhook := Webhook{Headers: headers[i%len(headers)]}
		IdentifyProvider(ctx, webhook)
	}
}

func BenchmarkVerifySignature(b *testing.B) {
	secrets := map[string]string{
		"stripe": "test_secret_key_123",
		"github": "test_secret_key_456",
		"slack":  "test_secret_key_789",
	}

	// Pre-generate test data
	bodies := [][]byte{
		[]byte(`{"id":"evt_1","type":"payment_intent.succeeded","data":{"amount":1000}}`),
		[]byte(`{"after":"abc123def","commits":[{"id":"abc123","message":"test"}]}`),
		[]byte(`{"type":"event_callback","event_id":"Ev123ABC","event":{"type":"message"}}`),
	}

	webhooks := []Webhook{
		{
			Provider: "stripe",
			Headers: http.Header{
				"Stripe-Signature": []string{generateStripeSignature(secrets["stripe"], bodies[0])},
			},
			Body: bodies[0],
		},
		{
			Provider: "github",
			Headers: http.Header{
				"X-Hub-Signature-256": []string{"sha256=" + computeHMAC(secrets["github"], bodies[1])},
			},
			Body: bodies[1],
		},
	}

	verifier := VerifySignature(secrets)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		verifier(ctx, webhooks[i%len(webhooks)])
	}
}

func BenchmarkParsePayload(b *testing.B) {
	payloads := []struct {
		provider string
		body     []byte
	}{
		{
			"stripe",
			[]byte(`{"id":"evt_123","type":"payment_intent.succeeded","data":{"object":"payment_intent","amount":5000,"currency":"usd"}}`),
		},
		{
			"github",
			[]byte(`{"ref":"refs/heads/main","after":"abc123def456","commits":[{"id":"abc123","message":"Update README","author":{"name":"Test"}}]}`),
		},
		{
			"slack",
			[]byte(`{"type":"event_callback","event_id":"Ev123ABC","team_id":"T123","event":{"type":"message","text":"Hello"}}`),
		},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p := payloads[i%len(payloads)]
		webhook := Webhook{
			Provider: p.provider,
			Body:     p.body,
		}
		ParsePayload(ctx, webhook)
	}
}

func BenchmarkPreventReplay(b *testing.B) {
	// Reset for clean benchmark
	processedWebhooks = &sync.Map{}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		webhook := Webhook{
			Provider:  "stripe",
			EventID:   fmt.Sprintf("evt_%d", i),
			Timestamp: time.Now(),
		}
		PreventReplay(ctx, webhook)
	}
}

func BenchmarkFullPipeline(b *testing.B) {
	secrets := map[string]string{
		"stripe": "test_secret",
		"github": "test_secret",
		"slack":  "test_secret",
	}

	pipeline := CreateWebhookPipeline(secrets)
	ctx := context.Background()

	// Pre-generate webhooks
	webhooks := make([]Webhook, 100)
	for i := range webhooks {
		body := fmt.Sprintf(`{"id":"evt_%d","type":"payment_intent.succeeded","data":{}}`, i)
		webhooks[i] = Webhook{
			Headers: http.Header{
				"Stripe-Signature": []string{generateStripeSignature("test_secret", []byte(body))},
			},
			Body:       []byte(body),
			ReceivedAt: time.Now(),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Clear replay prevention occasionally
		if i%1000 == 0 {
			processedWebhooks = &sync.Map{}
		}

		pipeline.Process(ctx, webhooks[i%len(webhooks)])
	}
}

func BenchmarkRouting(b *testing.B) {
	ctx := context.Background()

	// Create router
	routeFunc := func(ctx context.Context, w Webhook) string {
		return w.Provider
	}

	handlers := map[string]pipz.Chainable[Webhook]{
		"stripe": pipz.Apply("stripe", HandleStripeEvent),
		"github": pipz.Apply("github", HandleGitHubEvent),
		"slack":  pipz.Apply("slack", HandleSlackEvent),
	}

	router := pipz.Switch(routeFunc, handlers)

	// Test webhooks
	webhooks := []Webhook{
		{Provider: "stripe", EventType: "payment_intent.succeeded", EventID: "pi_1"},
		{Provider: "github", EventType: "push", EventID: "abc123"},
		{Provider: "slack", EventType: "message", EventID: "Ev123"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		router.Process(ctx, webhooks[i%len(webhooks)])
	}
}

func BenchmarkConcurrentWebhooks(b *testing.B) {
	secrets := map[string]string{
		"stripe": "test_secret",
	}

	pipeline := CreateWebhookPipeline(secrets)
	ctx := context.Background()

	// Generate webhook
	body := []byte(`{"id":"evt_concurrent","type":"payment_intent.succeeded","data":{}}`)
	webhook := Webhook{
		Headers: http.Header{
			"Stripe-Signature": []string{generateStripeSignature("test_secret", body)},
		},
		Body:       body,
		ReceivedAt: time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Unique event ID to avoid replay prevention
			w := webhook
			w.Body = []byte(fmt.Sprintf(`{"id":"evt_%d_%d","type":"payment_intent.succeeded","data":{}}`, time.Now().UnixNano(), i))
			w.Headers = http.Header{
				"Stripe-Signature": []string{generateStripeSignature("test_secret", w.Body)},
			}

			pipeline.Process(ctx, w)
			i++
		}
	})
}

func BenchmarkSignatureGeneration(b *testing.B) {
	secret := "whsec_test_secret_key_for_benchmarking"
	bodies := [][]byte{
		[]byte(`{"small":"payload"}`),
		[]byte(`{"medium":"payload","with":"more","fields":["and","arrays"],"nested":{"objects":"too"}}`),
		[]byte(`{"large":"payload","with":"lots","of":"data","including":{"deeply":{"nested":{"structures":["with","many","values","that","make","the","payload","much","larger","than","typical"]}}},"and":"more","fields":"to","increase":"size"}`),
	}

	b.Run("Small", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			generateStripeSignature(secret, bodies[0])
		}
	})

	b.Run("Medium", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			generateStripeSignature(secret, bodies[1])
		}
	})

	b.Run("Large", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			generateStripeSignature(secret, bodies[2])
		}
	})
}

func BenchmarkReplayPrevention(b *testing.B) {
	scenarios := []struct {
		name    string
		mapSize int
		hitRate float64
	}{
		{"SmallCache_HighHit", 100, 0.9},
		{"SmallCache_LowHit", 100, 0.1},
		{"LargeCache_HighHit", 10000, 0.9},
		{"LargeCache_LowHit", 10000, 0.1},
	}

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			// Pre-populate cache
			processedWebhooks = &sync.Map{}
			for i := 0; i < s.mapSize; i++ {
				key := fmt.Sprintf("stripe:evt_%d", i)
				processedWebhooks.Store(key, time.Now())
			}

			ctx := context.Background()
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var eventID string
				if float64(i%100)/100.0 < s.hitRate {
					// Cache hit
					eventID = fmt.Sprintf("evt_%d", i%s.mapSize)
				} else {
					// Cache miss
					eventID = fmt.Sprintf("evt_new_%d", i)
				}

				webhook := Webhook{
					Provider:  "stripe",
					EventID:   eventID,
					Timestamp: time.Now(),
				}
				PreventReplay(ctx, webhook)
			}
		})
	}
}

var globalWebhook Webhook // Prevent compiler optimizations

func BenchmarkMemoryUsage(b *testing.B) {
	ctx := context.Background()

	b.Run("WebhookAllocation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			webhook := Webhook{
				Provider: "stripe",
				Headers: http.Header{
					"Stripe-Signature": []string{"t=123,v1=abc"},
					"Content-Type":     []string{"application/json"},
				},
				Body:      []byte(`{"id":"evt_123","type":"test"}`),
				EventType: "payment_intent.succeeded",
				EventID:   "evt_123",
				Timestamp: time.Now(),
				Payload: map[string]interface{}{
					"amount":   1000,
					"currency": "usd",
				},
			}
			globalWebhook = webhook
		}
	})

	b.Run("PipelineProcessing", func(b *testing.B) {
		secrets := map[string]string{"stripe": "test"}
		pipeline := CreateWebhookPipeline(secrets)

		body := []byte(`{"id":"evt_bench","type":"test","data":{}}`)
		webhook := Webhook{
			Headers: http.Header{
				"Stripe-Signature": []string{generateStripeSignature("test", body)},
			},
			Body:       body,
			ReceivedAt: time.Now(),
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Unique event ID
			w := webhook
			w.Body = []byte(fmt.Sprintf(`{"id":"evt_%d","type":"test","data":{}}`, i))
			w.Headers = http.Header{
				"Stripe-Signature": []string{generateStripeSignature("test", w.Body)},
			}

			result, _ := pipeline.Process(ctx, w)
			globalWebhook = result
		}
	})
}
