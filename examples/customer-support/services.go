package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// ============================================================================.
// MOCK SERVICES FOR DEMONSTRATION PURPOSES ONLY.
// ============================================================================.
// These are simplified mock implementations to demonstrate the pipz library.
// In production, you would integrate with real AI providers like OpenAI,.
// Anthropic, or your own models.
//
// The mock services simulate:.
// - Variable latency and response times.
// - Service failures and errors.
// - Different cost structures.
// - Basic query understanding.
//
// DO NOT use these mock services in production!.

// ServiceBehavior controls how mock services behave for testing.
type ServiceBehavior int

const (
	BehaviorNormal ServiceBehavior = iota
	BehaviorSlow
	BehaviorTimeout
	BehaviorError
	BehaviorDegraded // Works but slow
	BehaviorTest     // Fast responses for testing
)

// TestMode enables fast responses for testing.
var TestMode = false

// AIProvider is a MOCK AI service for demonstration.
// Real implementations would use actual API clients.
type AIProvider struct {
	Name         string
	CostPerToken float64
	BaseLatency  time.Duration
	Behavior     ServiceBehavior

	mu           sync.Mutex
	requestCount int
	errorRate    float64 // 0.0 to 1.0
}

// Respond generates a response for the query.
func (p *AIProvider) Respond(ctx context.Context, query SupportQuery) (SupportQuery, error) {
	p.mu.Lock()
	p.requestCount++
	p.mu.Unlock()

	// Check for configured behavior.
	switch p.Behavior {
	case BehaviorTimeout:
		select {
		case <-ctx.Done():
			return query, ctx.Err()
		case <-time.After(30 * time.Second):
			return query, errors.New("timeout")
		}
	case BehaviorError:
		return query, fmt.Errorf("%s: service unavailable", p.Name)
	}

	// Simulate random errors based on error rate.
	if p.errorRate > 0 && rand.Float64() < p.errorRate { //nolint:gosec // Using weak RNG is fine for simulation
		return query, fmt.Errorf("%s: transient error", p.Name)
	}

	// Calculate response time with jitter.
	baseLatency := p.BaseLatency
	switch {
	case TestMode:
		// Much faster for tests.
		baseLatency = 10 * time.Millisecond
	case p.Behavior == BehaviorSlow:
		baseLatency *= 3
	case p.Behavior == BehaviorDegraded:
		baseLatency *= 2
	}

	jitter := time.Duration(rand.Intn(int(baseLatency / 4))) //nolint:gosec // Mock jitter calculation
	responseTime := baseLatency + jitter

	// Simulate API call.
	start := time.Now()
	select {
	case <-time.After(responseTime):
	// Success.
	case <-ctx.Done():
		return query, ctx.Err()
	}

	// Generate response based on query type.
	query.Response = p.generateResponse(query)
	query.ResponseTime = time.Since(start)
	query.Provider = p.Name

	// Calculate tokens and cost.
	query.TokensUsed = len(strings.Fields(query.Query)) + len(strings.Fields(query.Response))
	query.Cost = float64(query.TokensUsed) * p.CostPerToken

	return query, nil
}

func (*AIProvider) generateResponse(query SupportQuery) string {
	// MOCK RESPONSE GENERATION - FOR DEMO ONLY!.
	// This simulates basic AI understanding using simple pattern matching.
	// Real AI would provide much more sophisticated responses.
	//
	// For Sprint 1 (MVP), we don't have classification yet.
	// So we need to do basic pattern matching here.
	queryLower := strings.ToLower(query.Query)

	// Check if it's an order status query.
	if strings.Contains(queryLower, "order") && (strings.Contains(queryLower, "where") || strings.Contains(queryLower, "status")) {
		// Extract order ID if present.
		orderID := ""
		words := strings.Fields(query.Query)
		for _, word := range words {
			if strings.HasPrefix(word, "#") || strings.Contains(word, "12345") || strings.Contains(word, "99999") || strings.Contains(word, "55555") {
				orderID = strings.TrimPrefix(word, "#")
				orderID = strings.TrimSuffix(orderID, "?")
				orderID = strings.TrimSuffix(orderID, "!")
				break
			}
		}

		if orderID != "" {
			return fmt.Sprintf("I've checked on order #%s for you. Your order is currently in transit and is expected to arrive within 2-3 business days. You can track your package using the tracking number sent to your email. Is there anything else I can help you with?", orderID)
		}
		return "I'd be happy to help you track your order. Could you please provide your order number? It usually starts with '#' and can be found in your confirmation email."
	}

	// Check for refund requests.
	if strings.Contains(queryLower, "refund") || strings.Contains(queryLower, "return") || strings.Contains(queryLower, "money back") {
		return "I understand you'd like to request a refund. I can help you with that. To process your refund request, I'll need some information about your order. Could you please provide your order number and briefly describe the issue you're experiencing?"
	}

	// Check for technical issues.
	if strings.Contains(queryLower, "error") || strings.Contains(queryLower, "api") || strings.Contains(queryLower, "403") {
		return "I see you're experiencing a technical issue. Let me help you resolve this. The error you're seeing typically occurs when there's an authentication problem. Please try refreshing your API credentials in the dashboard. If the issue persists, I can escalate this to our technical team."
	}

	// Check for complaints.
	if strings.Contains(queryLower, "terrible") || strings.Contains(queryLower, "awful") || strings.Contains(queryLower, "worst") {
		return "I sincerely apologize for your negative experience. Your feedback is very important to us. I'd like to make this right for you. Could you please share more details about what went wrong so I can assist you better and ensure this doesn't happen again?"
	}

	// Check for product questions.
	if strings.Contains(queryLower, "how") && (strings.Contains(queryLower, "work") || strings.Contains(queryLower, "use") || strings.Contains(queryLower, "integrate")) {
		return "I'd be glad to help you understand our product better. Based on your question, here's what you need to know: Our products come with detailed instructions, and we also have video tutorials available on our support portal. Would you like me to send you a direct link to the relevant guide?"
	}

	// Default response.
	return "Thank you for contacting support. I'm here to help! Could you please provide more details about what you need assistance with today?"
}

// Mock service instances - FOR DEMONSTRATION ONLY.
// These simulate different AI providers with varying performance characteristics.
var (
	// Primary providers.
	GPT4Service = &AIProvider{
		Name:         "gpt-4",
		CostPerToken: 0.00003, // $0.03 per 1K tokens
		BaseLatency:  2 * time.Second,
		Behavior:     BehaviorNormal,
	}

	GPT35Service = &AIProvider{
		Name:         "gpt-3.5-turbo",
		CostPerToken: 0.000002, // $0.002 per 1K tokens
		BaseLatency:  800 * time.Millisecond,
		Behavior:     BehaviorNormal,
	}

	ClaudeService = &AIProvider{
		Name:         "claude-2",
		CostPerToken: 0.00001, // $0.01 per 1K tokens
		BaseLatency:  1200 * time.Millisecond,
		Behavior:     BehaviorNormal,
		errorRate:    0.05, // 5% error rate for realism
	}

	LocalModelService = &AIProvider{
		Name:         "local-llama",
		CostPerToken: 0.0, // Free but slower
		BaseLatency:  1500 * time.Millisecond,
		Behavior:     BehaviorNormal,
	}

	// Cache service.
	ResponseCache = &CacheService{
		data: make(map[string]CachedResponse),
	}

	// Metrics collector.
	MetricsCollector = &Metrics{
		queries: make([]SupportQuery, 0),
	}
)

// CacheService provides simple response caching.
// MOCK IMPLEMENTATION - Real caching would use Redis, Memcached, etc.
//
//nolint:govet // Field alignment is optimized for readability
type CacheService struct {
	mu   sync.RWMutex
	data map[string]CachedResponse
}

//nolint:govet // Field alignment is optimized for readability
type CachedResponse struct {
	Response  string
	Provider  string
	Timestamp time.Time
}

func (c *CacheService) Get(query string) (CachedResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Simple cache key - in production would be more sophisticated.
	key := strings.ToLower(strings.TrimSpace(query))
	resp, found := c.data[key]

	// Cache expires after 1 hour.
	if found && time.Since(resp.Timestamp) > time.Hour {
		return CachedResponse{}, false
	}

	return resp, found
}

func (c *CacheService) Set(query string, response string, provider string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(query))
	c.data[key] = CachedResponse{
		Response:  response,
		Provider:  provider,
		Timestamp: time.Now(),
	}
}

func (c *CacheService) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]CachedResponse)
}

// Metrics tracks performance metrics.
//
//nolint:govet // Field alignment is optimized for readability
type Metrics struct {
	mu      sync.Mutex
	queries []SupportQuery
}

func (m *Metrics) Record(query SupportQuery) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = append(m.queries, query)
}

func (m *Metrics) GetSummary() MetricsSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queries) == 0 {
		return MetricsSummary{}
	}

	summary := MetricsSummary{
		TotalQueries:      len(m.queries),
		ProviderBreakdown: make(map[string]int),
	}

	var totalResponse time.Duration
	var totalCost float64
	var cacheHits int
	var fallbacks int

	for i := range m.queries {
		q := m.queries[i]
		totalResponse += q.ResponseTime
		totalCost += q.Cost

		if q.CacheHit {
			cacheHits++
		}
		if q.Metrics.FallbackUsed {
			fallbacks++
		}

		if i == 0 || q.ResponseTime < summary.FastestResponse {
			summary.FastestResponse = q.ResponseTime
		}
		if i == 0 || q.ResponseTime > summary.SlowestResponse {
			summary.SlowestResponse = q.ResponseTime
		}

		summary.ProviderBreakdown[q.Provider]++
	}

	summary.AverageResponse = totalResponse / time.Duration(len(m.queries))
	summary.TotalCost = totalCost
	summary.CacheHitRate = float64(cacheHits) / float64(len(m.queries))
	summary.FallbackRate = float64(fallbacks) / float64(len(m.queries))

	// Calculate cost savings (comparing to if everything used GPT-4).
	gpt4Cost := 0.0
	for i := range m.queries {
		q := m.queries[i]
		if !q.CacheHit {
			gpt4Cost += float64(q.TokensUsed) * 0.00003
		}
	}
	summary.CostSavings = gpt4Cost - totalCost

	return summary
}

func (m *Metrics) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = make([]SupportQuery, 0)
}

// Helper functions for testing different scenarios.
func SetProviderBehavior(provider *AIProvider, behavior ServiceBehavior) {
	provider.Behavior = behavior
}

func ResetAllProviders() {
	GPT4Service.Behavior = BehaviorNormal
	GPT35Service.Behavior = BehaviorNormal
	ClaudeService.Behavior = BehaviorNormal
	LocalModelService.Behavior = BehaviorNormal

	GPT4Service.errorRate = 0
	GPT35Service.errorRate = 0
	ClaudeService.errorRate = 0.05
	LocalModelService.errorRate = 0
}
