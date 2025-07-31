package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func init() {
	// Enable test mode for faster responses.
	TestMode = true
}

// TestSprint1MVP tests the basic MVP pipeline.
func TestSprint1MVP(t *testing.T) {
	// Reset to MVP state.
	ResetPipeline()
	ResetAllProviders()
	ResponseCache.Clear()

	ctx := context.Background()

	query := SupportQuery{
		CustomerID: "TEST-001",
		Query:      "Where is my order #12345?",
		Channel:    "test",
	}

	result, err := ProcessQuery(ctx, query)
	if err != nil {
		t.Fatalf("Sprint 1 query failed: %v", err)
	}

	// Should use GPT-4 (the only provider in MVP).
	if result.Provider != "gpt-4" {
		t.Errorf("Expected GPT-4 provider, got %s", result.Provider)
	}

	// Should have appropriate response.
	if !strings.Contains(result.Response, "order #12345") {
		t.Errorf("Response should mention order number, got: %s", result.Response)
	}

	// Should have cost (GPT-4 is expensive).
	if result.Cost < 0.0005 {
		t.Errorf("GPT-4 cost seems too low: $%.4f", result.Cost)
	}
}

// TestSprint3CostOptimization tests intelligent routing.
func TestSprint3CostOptimization(t *testing.T) {
	ResetPipeline()
	EnableCostOptimization()
	ResetAllProviders()
	ResponseCache.Clear()

	ctx := context.Background()

	tests := []struct {
		name        string
		query       string
		expectType  QueryType
		expectModel string
		expectCheap bool
	}{
		{
			name:        "simple_order_status",
			query:       "Where is my order #12345?",
			expectType:  QueryTypeOrderStatus,
			expectModel: "gpt-3.5-turbo",
			expectCheap: true,
		},
		{
			name:        "complex_refund",
			query:       "I want a refund for my damaged item",
			expectType:  QueryTypeRefund,
			expectModel: "gpt-4",
			expectCheap: false,
		},
		{
			name:        "technical_issue",
			query:       "Getting API error 403 when trying to integrate",
			expectType:  QueryTypeTechnical,
			expectModel: "gpt-4",
			expectCheap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := SupportQuery{
				CustomerID: "TEST-003",
				Query:      tt.query,
				Channel:    "test",
			}

			result, err := ProcessQuery(ctx, query)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			if result.QueryType != tt.expectType {
				t.Errorf("Expected type %v, got %v", tt.expectType, result.QueryType)
			}

			if result.Provider != tt.expectModel {
				t.Errorf("Expected provider %s, got %s", tt.expectModel, result.Provider)
			}

			if tt.expectCheap && result.Cost > 0.0005 {
				t.Errorf("Expected cheap query, but cost was $%.4f", result.Cost)
			}
			if !tt.expectCheap && result.Cost < 0.0005 {
				t.Errorf("Expected expensive query, but cost was only $%.4f", result.Cost)
			}
		})
	}
}

// TestSprint3Caching tests response caching.
func TestSprint3Caching(t *testing.T) {
	ResetPipeline()
	EnableCostOptimization()
	ResponseCache.Clear()

	ctx := context.Background()

	query := SupportQuery{
		CustomerID: "TEST-CACHE",
		Query:      "Where is my order #99999?",
		Channel:    "test",
	}

	// First query should miss cache.
	result1, err := ProcessQuery(ctx, query)
	if err != nil {
		t.Fatalf("First query failed: %v", err)
	}

	if result1.CacheHit {
		t.Error("First query should be cache miss")
	}

	if result1.Cost == 0 {
		t.Error("First query should have cost")
	}

	// Second identical query should hit cache.
	result2, err := ProcessQuery(ctx, query)
	if err != nil {
		t.Fatalf("Second query failed: %v", err)
	}

	if !result2.CacheHit {
		t.Error("Second query should be cache hit")
	}

	if result2.Cost != 0 {
		t.Error("Cached query should have zero cost")
	}

	if !strings.Contains(result2.Provider, "cached") {
		t.Errorf("Provider should indicate cache: %s", result2.Provider)
	}
}

// TestSprint5Fallback tests multi-provider fallback.
func TestSprint5Fallback(t *testing.T) {
	ResetPipeline()
	EnableProviderFallback()
	ResponseCache.Clear()

	ctx := context.Background()

	// Simulate OpenAI being down.
	GPT4Service.Behavior = BehaviorError
	GPT35Service.Behavior = BehaviorError
	defer ResetAllProviders()

	query := SupportQuery{
		CustomerID: "TEST-FALLBACK",
		Query:      "I need help with my order",
		Channel:    "test",
	}

	result, err := ProcessQuery(ctx, query)
	if err != nil {
		t.Fatalf("Fallback failed: %v", err)
	}

	// Should fallback to Claude or local model.
	if result.Provider != "claude-2" && result.Provider != "local-llama" {
		t.Errorf("Expected fallback provider, got %s", result.Provider)
	}

	// Should still get a valid response.
	if result.Response == "" {
		t.Error("Fallback should provide valid response")
	}
}

// TestSprint7SpeedOptimization tests racing for urgent queries.
func TestSprint7SpeedOptimization(t *testing.T) {
	ResetPipeline()
	EnableSpeedOptimization()
	ResetAllProviders()

	// Make GPT-4 extra slow for this test.
	GPT4Service.Behavior = BehaviorSlow
	defer func() { GPT4Service.Behavior = BehaviorNormal }()

	ctx := context.Background()

	tests := []struct {
		name          string
		query         string
		expectUrgency UrgencyLevel
		expectFast    bool
	}{
		{
			name:          "urgent_wedding",
			query:         "MY WEDDING IS TOMORROW AND MY DRESS HASN'T ARRIVED!!!",
			expectUrgency: UrgencyCritical,
			expectFast:    true,
		},
		{
			name:          "angry_customer",
			query:         "This is the WORST service ever! I want my money back!",
			expectUrgency: UrgencyHigh,
			expectFast:    true,
		},
		{
			name:          "normal_query",
			query:         "How do I update my shipping address?",
			expectUrgency: UrgencyNormal,
			expectFast:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := SupportQuery{
				CustomerID: "TEST-SPEED",
				Query:      tt.query,
				Channel:    "test",
			}

			start := time.Now()
			result, err := ProcessQuery(ctx, query)
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			if result.Urgency != tt.expectUrgency {
				t.Errorf("Expected urgency %v, got %v", tt.expectUrgency, result.Urgency)
			}

			if tt.expectFast && duration > 100*time.Millisecond {
				t.Errorf("Urgent query too slow: %v", duration)
			}

			if tt.expectFast && result.Provider == "gpt-4" {
				t.Error("Urgent query should not use slow GPT-4")
			}
		})
	}
}

// TestSentimentAnalysis tests sentiment detection.
func TestSentimentAnalysis(t *testing.T) {
	ResetPipeline()
	EnableSpeedOptimization() // This includes sentiment analysis

	ctx := context.Background()

	tests := []struct {
		query           string
		expectSentiment SentimentScore
	}{
		{
			query:           "Your service is terrible and I'm furious!",
			expectSentiment: SentimentAngry,
		},
		{
			query:           "I'm disappointed with the delayed delivery",
			expectSentiment: SentimentNegative,
		},
		{
			query:           "Thanks for the great support!",
			expectSentiment: SentimentPositive,
		},
		{
			query:           "Where is my order?",
			expectSentiment: SentimentNeutral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			query := SupportQuery{
				CustomerID: "TEST-SENTIMENT",
				Query:      tt.query,
				Channel:    "test",
			}

			result, err := ProcessQuery(ctx, query)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			if result.Sentiment != tt.expectSentiment {
				t.Errorf("Expected sentiment %v, got %v", tt.expectSentiment, result.Sentiment)
			}
		})
	}
}

// TestMetricsCollection tests that metrics are properly collected.
func TestMetricsCollection(t *testing.T) {
	ResetPipeline()
	EnableProductionFeatures()
	MetricsCollector.Clear()

	ctx := context.Background()

	// Process several queries.
	queries := []string{
		"Where is my order #12345?",
		"I want a refund",
		"How does the API work?",
	}

	for _, q := range queries {
		query := SupportQuery{
			CustomerID: "TEST-METRICS",
			Query:      q,
			Channel:    "test",
		}

		_, err := ProcessQuery(ctx, query)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
	}

	// Check metrics.
	summary := MetricsCollector.GetSummary()

	if summary.TotalQueries != len(queries) {
		t.Errorf("Expected %d queries in metrics, got %d", len(queries), summary.TotalQueries)
	}

	if summary.TotalCost == 0 {
		t.Error("Metrics should track total cost")
	}

	if summary.AverageResponse == 0 {
		t.Error("Metrics should track average response time")
	}

	if len(summary.ProviderBreakdown) == 0 {
		t.Error("Metrics should track provider usage")
	}
}

// BenchmarkSimpleQuery benchmarks simple query processing.
func BenchmarkSimpleQuery(b *testing.B) {
	ResetPipeline()
	EnableCostOptimization()
	ResponseCache.Clear()

	ctx := context.Background()
	query := SupportQuery{
		CustomerID: "BENCH-001",
		Query:      "Where is my order #12345?",
		Channel:    "bench",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ProcessQuery(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
		ResponseCache.Clear() // Clear cache to measure actual processing
	}
}

// BenchmarkComplexQuery benchmarks complex query processing.
func BenchmarkComplexQuery(b *testing.B) {
	ResetPipeline()
	EnableProductionFeatures()

	ctx := context.Background()
	query := SupportQuery{
		CustomerID: "BENCH-002",
		Query:      "MY WEDDING IS TOMORROW AND I NEED MY DRESS! THIS IS URGENT!",
		Channel:    "bench",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ProcessQuery(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCachedQuery benchmarks cached query performance.
func BenchmarkCachedQuery(b *testing.B) {
	ResetPipeline()
	EnableCostOptimization()
	ResponseCache.Clear()

	ctx := context.Background()
	query := SupportQuery{
		CustomerID: "BENCH-003",
		Query:      "Where is my order #99999?",
		Channel:    "bench",
	}

	// Prime the cache.
	_, err := ProcessQuery(ctx, query)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ProcessQuery(ctx, query)
		if err != nil {
			b.Fatal(err)
		}
	}
}
