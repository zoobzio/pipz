// Package ai demonstrates using pipz for building resilient AI/LLM pipelines
// with caching, fallbacks, and content filtering.
package ai

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// AIRequest represents a request to an AI model with its response
type AIRequest struct {
	ID          string
	Prompt      string
	Model       string
	Temperature float32
	MaxTokens   int

	// Response fields
	Response     string
	ResponseTime time.Duration
	TokensUsed   int
	Cost         float64

	// Metadata
	Timestamp time.Time
	UserID    string
	SessionID string
	Tags      []string
}

// AIProvider simulates different AI model providers
type AIProvider interface {
	Complete(ctx context.Context, req AIRequest) (AIRequest, error)
	Name() string
	CostPerToken() float64
}

// Mock providers for the example
var (
	gpt4Provider   = &mockProvider{name: "gpt-4", costPerToken: 0.00003, baseLatency: 2 * time.Second}
	gpt35Provider  = &mockProvider{name: "gpt-3.5-turbo", costPerToken: 0.000002, baseLatency: 800 * time.Millisecond}
	claudeProvider = &mockProvider{name: "claude-2", costPerToken: 0.00001, baseLatency: 1 * time.Second}

	// Simulated rate limits
	rateLimits = &sync.Map{}
)

type mockProvider struct {
	name         string
	costPerToken float64
	baseLatency  time.Duration
}

func (p *mockProvider) Complete(ctx context.Context, req AIRequest) (AIRequest, error) {
	// Simulate rate limiting
	if isRateLimited(p.name) {
		return req, errors.New("rate limit exceeded")
	}

	// Simulate API latency
	jitter := time.Duration(rand.Intn(500)) * time.Millisecond
	select {
	case <-time.After(p.baseLatency + jitter):
		// Success
	case <-ctx.Done():
		return req, ctx.Err()
	}

	// No random failures - deterministic behavior

	// Generate mock response
	req.Model = p.name
	req.Response = generateMockResponse(req.Prompt, p.name)
	req.TokensUsed = len(strings.Fields(req.Prompt)) + len(strings.Fields(req.Response))
	req.Cost = float64(req.TokensUsed) * p.costPerToken
	req.ResponseTime = p.baseLatency + jitter

	return req, nil
}

func (p *mockProvider) Name() string {
	return p.name
}

func (p *mockProvider) CostPerToken() float64 {
	return p.costPerToken
}

// Pipeline processors

// EnhancePrompt adds system instructions and context
func EnhancePrompt(_ context.Context, req AIRequest) AIRequest {
	if !strings.Contains(req.Prompt, "You are") {
		req.Prompt = fmt.Sprintf(
			"You are a helpful, harmless, and honest AI assistant. "+
				"Please provide accurate and useful information. "+
				"User request: %s",
			req.Prompt,
		)
	}
	req.Tags = append(req.Tags, "enhanced")
	return req
}

// ValidateSafety checks for unsafe content
func ValidateSafety(_ context.Context, req AIRequest) error {
	// Simulate safety checks
	prompt := strings.ToLower(req.Prompt)

	// Check for PII patterns
	if containsPII(prompt) {
		return errors.New("prompt contains personally identifiable information")
	}

	// Check for harmful content
	harmfulPatterns := []string{"hack", "exploit", "illegal", "weapon"}
	for _, pattern := range harmfulPatterns {
		if strings.Contains(prompt, pattern) {
			return fmt.Errorf("potentially harmful content detected: %s", pattern)
		}
	}

	return nil
}

// CheckContentPolicy validates against content policy
func CheckContentPolicy(_ context.Context, req AIRequest) error {
	// Simulate content policy checks
	if len(req.Prompt) > 4000 {
		return errors.New("prompt exceeds maximum length")
	}

	if req.Temperature > 1.0 {
		return errors.New("temperature too high for safe generation")
	}

	return nil
}

// CallAIProvider creates a processor for a specific AI provider
func CallAIProvider(provider AIProvider) pipz.Processor[AIRequest] {
	return pipz.Apply(provider.Name(), func(ctx context.Context, req AIRequest) (AIRequest, error) {
		start := time.Now()
		result, err := provider.Complete(ctx, req)
		if err == nil {
			result.ResponseTime = time.Since(start)
		}
		return result, err
	})
}

// FilterResponse removes unwanted content from AI responses
func FilterResponse(_ context.Context, req AIRequest) AIRequest {
	// Remove any leaked PII
	req.Response = sanitizePII(req.Response)

	// Remove repetitive content
	req.Response = removeRepetitions(req.Response)

	// Add filtering tag
	req.Tags = append(req.Tags, "filtered")

	return req
}

// LogMetrics logs performance and cost metrics
func LogMetrics(_ context.Context, req AIRequest) error {
	// In production, this would send to monitoring service
	fmt.Printf("[METRICS] Model: %s, Latency: %v, Tokens: %d, Cost: $%.4f\n",
		req.Model, req.ResponseTime, req.TokensUsed, req.Cost)
	return nil
}

// CachedAIProvider wraps an AI provider with caching based on prompt content
func CachedAIProvider(provider AIProvider) pipz.Processor[AIRequest] {
	cache := &sync.Map{}

	return pipz.Apply("cached_"+provider.Name(), func(ctx context.Context, req AIRequest) (AIRequest, error) {
		// Create cache key from prompt and temperature
		cacheKey := fmt.Sprintf("%s:%.2f:%d", req.Prompt, req.Temperature, req.MaxTokens)

		// Check cache
		if cached, ok := cache.Load(cacheKey); ok {
			cachedResp := cached.(AIRequest)
			// Preserve request-specific fields
			cachedResp.ID = req.ID
			cachedResp.UserID = req.UserID
			cachedResp.SessionID = req.SessionID
			cachedResp.Timestamp = req.Timestamp
			cachedResp.Temperature = req.Temperature // Preserve temperature from request
			cachedResp.MaxTokens = req.MaxTokens     // Preserve max tokens from request
			cachedResp.Tags = append(req.Tags, "cache_hit")
			return cachedResp, nil
		}

		// Call provider
		result, err := provider.Complete(ctx, req)
		if err != nil {
			return result, err
		}

		// Add cache miss tag
		result.Tags = append(result.Tags, "cache_miss")

		// Cache successful result
		cache.Store(cacheKey, result)
		return result, nil
	})
}

// CreateAIPipeline creates a production-ready AI processing pipeline
func CreateAIPipeline() pipz.Chainable[AIRequest] {
	// Create cached processors for each provider
	gpt4 := CachedAIProvider(gpt4Provider)
	gpt35 := CachedAIProvider(gpt35Provider)
	claude := CachedAIProvider(claudeProvider)

	// Build the pipeline with all resilience patterns
	return pipz.Sequential(
		// Pre-processing
		pipz.Transform("enhance_prompt", EnhancePrompt),
		pipz.Effect("safety_check", ValidateSafety),
		pipz.Effect("content_policy", CheckContentPolicy),

		// AI calling with resilience
		pipz.RetryWithBackoff(
			pipz.Timeout(
				pipz.Fallback(
					pipz.Fallback(gpt4, gpt35), // Try GPT-4, fall back to GPT-3.5
					claude,                     // Final fallback to Claude
				),
				10*time.Second, // 10 second timeout per attempt
			),
			3,                    // 3 retry attempts
			500*time.Millisecond, // Start with 500ms backoff
		),

		// Post-processing
		pipz.Transform("filter_response", FilterResponse),
		pipz.Effect("log_metrics", LogMetrics),
	)
}

// CreateRoutingPipeline creates a pipeline that routes to different models based on request type
func CreateRoutingPipeline() pipz.Chainable[AIRequest] {
	// Define routing condition
	routeByComplexity := func(_ context.Context, req AIRequest) string {
		wordCount := len(strings.Fields(req.Prompt))

		// Route based on prompt complexity
		lowerPrompt := strings.ToLower(req.Prompt)
		switch {
		case strings.Contains(lowerPrompt, "code") ||
			strings.Contains(lowerPrompt, "function") ||
			strings.Contains(lowerPrompt, "program") ||
			strings.Contains(lowerPrompt, "algorithm"):
			return "code"
		case strings.Contains(lowerPrompt, "creative") ||
			strings.Contains(lowerPrompt, "story") ||
			strings.Contains(lowerPrompt, "poem"):
			return "creative"
		case wordCount > 100:
			return "complex"
		default:
			return "simple"
		}
	}

	// Create specialized pipelines for each route
	routes := map[string]pipz.Chainable[AIRequest]{
		"code": pipz.Sequential(
			pipz.Transform("code_context", func(_ context.Context, req AIRequest) AIRequest {
				req.Prompt = "You are an expert programmer. " + req.Prompt
				req.Temperature = 0.2 // Lower temperature for code
				return req
			}),
			pipz.Retry(CachedAIProvider(gpt4Provider), 3), // GPT-4 for code with retry
		),
		"creative": pipz.Sequential(
			pipz.Transform("creative_context", func(_ context.Context, req AIRequest) AIRequest {
				req.Temperature = 0.8 // Higher temperature for creativity
				return req
			}),
			pipz.Retry(CachedAIProvider(claudeProvider), 3), // Claude for creative with retry
		),
		"complex": pipz.Timeout(
			pipz.Retry(CachedAIProvider(gpt4Provider), 3), // GPT-4 for complex with retry
			15*time.Second,
		),
		"simple": pipz.Retry(CachedAIProvider(gpt35Provider), 3), // GPT-3.5 for simple with retry
	}

	// Build routing pipeline
	return pipz.Sequential(
		pipz.Transform("enhance_prompt", EnhancePrompt),
		pipz.Effect("safety_check", ValidateSafety),
		pipz.Switch(routeByComplexity, routes),
		pipz.Transform("filter_response", FilterResponse),
	)
}

// Utility functions

func generateMockResponse(prompt, model string) string {
	// Generate deterministic response based on prompt and model
	// In real implementation, this would be the actual AI response
	hash := 0
	for _, ch := range prompt {
		hash = (hash*31 + int(ch)) % 1000
	}

	responses := map[string][]string{
		"gpt-4": {
			"Based on my analysis, here's a comprehensive response to your query...",
			"Let me provide you with a detailed explanation...",
			"Here's a thoughtful approach to your question...",
		},
		"gpt-3.5-turbo": {
			"Here's what I found regarding your question...",
			"I can help you with that. Here's the information...",
			"Based on the available data, here's my response...",
		},
		"claude-2": {
			"I understand your question. Let me elaborate...",
			"That's an interesting query. Here's my perspective...",
			"Allow me to provide some insights on this topic...",
		},
	}

	modelResponses := responses[model]
	if len(modelResponses) == 0 {
		modelResponses = []string{"Here's my response to your query..."}
	}

	// Use hash to pick deterministic response
	base := modelResponses[hash%len(modelResponses)]
	return fmt.Sprintf("%s\n\nRegarding '%s', %s", base, prompt,
		"this is a simulated response for demonstration purposes.")
}

func containsPII(text string) bool {
	// Simplified PII detection (in production, use proper regex)
	// Check for common PII patterns
	if strings.Contains(text, "@") {
		return true // Likely email
	}
	if strings.Count(text, "-") >= 2 && len(text) > 10 {
		return true // Might be SSN format
	}
	// Check for long digit sequences (credit cards)
	digitCount := 0
	for _, r := range text {
		if r >= '0' && r <= '9' {
			digitCount++
			if digitCount >= 12 {
				return true
			}
		} else {
			digitCount = 0
		}
	}
	return false
}

func sanitizePII(text string) string {
	// Simple PII removal
	text = strings.ReplaceAll(text, "@", "[EMAIL]")
	// In production, use proper regex
	return text
}

func removeRepetitions(text string) string {
	// Simplified repetition removal
	lines := strings.Split(text, "\n")
	seen := make(map[string]bool)
	var result []string

	for _, line := range lines {
		if line != "" && !seen[line] {
			seen[line] = true
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// ResetRateLimits clears all rate limits (for testing)
func ResetRateLimits() {
	rateLimits = &sync.Map{}
}

func isRateLimited(provider string) bool {
	key := fmt.Sprintf("%s:%d", provider, time.Now().Unix()/60)

	if count, ok := rateLimits.Load(key); ok {
		if count.(int) > 10 { // 10 requests per minute
			return true
		}
		rateLimits.Store(key, count.(int)+1)
	} else {
		rateLimits.Store(key, 1)
	}

	return false
}

// GenerateRequestID creates a unique request ID
func GenerateRequestID(prompt string) string {
	hash := md5.Sum([]byte(prompt + time.Now().String()))
	return hex.EncodeToString(hash[:])[:8]
}
