package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/ai"
)

// AIExample implements the Example interface for AI/LLM integration
type AIExample struct{}

func (a *AIExample) Name() string {
	return "ai"
}

func (a *AIExample) Description() string {
	return "LLM integration with caching, fallbacks, and rate limiting"
}

func (a *AIExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ AI/LLM INTEGRATION EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Smart caching to reduce API calls and costs")
	fmt.Println("• Fallback between different LLM providers")
	fmt.Println("• Rate limiting and retry strategies")
	fmt.Println("• Response validation and sanitization")
	fmt.Println("• Cost tracking and optimization")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("LLM integration requires handling many concerns:")
	fmt.Println(colorGray + `
func generateResponse(prompt string) (string, error) {
    // Check cache first
    cacheKey := hash(prompt)
    if cached, found := cache.Get(cacheKey); found {
        metrics.Inc("llm.cache_hit")
        return cached.(string), nil
    }
    metrics.Inc("llm.cache_miss")
    
    // Rate limiting
    if !rateLimiter.Allow() {
        metrics.Inc("llm.rate_limited")
        return "", errors.New("rate limited")
    }
    
    // Try primary provider (OpenAI)
    response, err := openai.Complete(prompt)
    if err != nil {
        log.Printf("OpenAI failed: %v", err)
        
        // Fallback to Anthropic
        response, err = anthropic.Complete(prompt)
        if err != nil {
            log.Printf("Anthropic failed: %v", err)
            
            // Last resort: use local model
            response, err = localLLM.Complete(prompt)
            if err != nil {
                return "", fmt.Errorf("all providers failed: %w", err)
            }
        }
    }
    
    // Validate response
    if containsToxicContent(response) {
        return "", errors.New("toxic content detected")
    }
    
    // Cache successful response
    cache.Set(cacheKey, response, 1*time.Hour)
    
    return response, nil
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Compose a robust AI pipeline with the actual ai example:")
	fmt.Println(colorGray + `
// Using the real ai.AIRequest type and processors from examples/ai
import "github.com/zoobzio/pipz/examples/ai"

// Build pipeline with real AI providers
aiPipeline := pipz.Sequential(
    ai.ValidatePrompt(),
    ai.EnhancePrompt(),
    ai.CachedAIProvider(ai.GPT4Provider()),
    ai.FilterResponse(),
    ai.TrackCosts(),
)

// Execute with full cost tracking and caching
request := ai.AIRequest{
    ID:          "req_123",
    Prompt:      "Explain quantum computing",
    Model:       "gpt-4",
    Temperature: 0.7,
    MaxTokens:   150,
    UserID:      "user_456",
}

result, err := aiPipeline.Process(ctx, request)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's generate some AI responses!" + colorReset)
	return a.runInteractive(ctx)
}

func (a *AIExample) runInteractive(ctx context.Context) error {
	// Reset rate limits for clean demo
	ai.ResetRateLimits()

	// Create the demo pipeline using actual ai example processors
	pipeline := createDemoAIPipeline()

	// Test scenarios
	scenarios := []struct {
		name    string
		request ai.AIRequest
		special string
	}{
		{
			name: "Simple Question (Cache Miss)",
			request: ai.AIRequest{
				ID:          "req_001",
				Prompt:      "What is the capital of France?",
				Model:       "gpt-4",
				Temperature: 0.7,
				MaxTokens:   50,
				UserID:      "demo_user",
				Timestamp:   time.Now(),
			},
		},
		{
			name: "Same Question (Cache Hit)",
			request: ai.AIRequest{
				ID:          "req_002",
				Prompt:      "What is the capital of France?", // Same prompt for cache hit
				Model:       "gpt-4",
				Temperature: 0.7,
				MaxTokens:   50,
				UserID:      "demo_user",
				Timestamp:   time.Now(),
			},
		},
		{
			name: "Code Generation Request",
			request: ai.AIRequest{
				ID:          "req_003",
				Prompt:      "Write a Python function to calculate fibonacci numbers",
				Model:       "gpt-3.5-turbo",
				Temperature: 0.3,
				MaxTokens:   200,
				UserID:      "developer_user",
				Timestamp:   time.Now(),
			},
		},
		{
			name: "Complex Analysis",
			request: ai.AIRequest{
				ID:          "req_004",
				Prompt:      "Explain the differences between quantum computing and classical computing in detail",
				Model:       "claude-2",
				Temperature: 0.5,
				MaxTokens:   300,
				UserID:      "researcher_user",
				Timestamp:   time.Now(),
			},
		},
		{
			name:    "Rate Limiting Test",
			special: "rate_limit_burst",
			request: ai.AIRequest{
				ID:          "req_burst",
				Prompt:      "Tell me a joke",
				Model:       "gpt-4",
				Temperature: 0.9,
				MaxTokens:   100,
				UserID:      "burst_user",
				Timestamp:   time.Now(),
			},
		},
	}

	totalCost := 0.0
	cacheHits := 0

	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		if scenario.special == "rate_limit_burst" {
			// Test rate limiting with burst requests
			fmt.Printf("\n%sSending multiple requests to test rate limiting...%s\n", 
				colorYellow, colorReset)
			
			successCount := 0
			rateLimitedCount := 0
			
			for j := 0; j < 8; j++ {
				req := scenario.request
				req.ID = fmt.Sprintf("burst_%d", j)
				req.Prompt = fmt.Sprintf("Tell me joke #%d", j+1)
				
				start := time.Now()
				result, err := pipeline.Process(ctx, req)
				duration := time.Since(start)
				
				if err != nil {
					if strings.Contains(err.Error(), "rate limit") {
						rateLimitedCount++
						fmt.Printf("  %s⚠️  Request %d: Rate limited%s\n", 
							colorYellow, j+1, colorReset)
					} else {
						fmt.Printf("  %s❌ Request %d: %s%s\n", 
							colorRed, j+1, err.Error(), colorReset)
					}
				} else {
					successCount++
					fmt.Printf("  %s✅ Request %d: %s (%v, $%.4f)%s\n", 
						colorGreen, j+1, result.Model, duration, result.Cost, colorReset)
					totalCost += result.Cost
				}
				
				time.Sleep(100 * time.Millisecond)
			}
			
			fmt.Printf("\nBurst Results: %d successful, %d rate limited\n", 
				successCount, rateLimitedCount)
		} else {
			// Single request
			fmt.Printf("\nPrompt: \"%s\"\n", scenario.request.Prompt)
			fmt.Printf("Model: %s (temp: %.1f, max tokens: %d)\n", 
				scenario.request.Model, scenario.request.Temperature, scenario.request.MaxTokens)

			fmt.Printf("\n%sProcessing...%s\n", colorYellow, colorReset)
			start := time.Now()
			
			result, err := pipeline.Process(ctx, scenario.request)
			duration := time.Since(start)
			
			if err != nil {
				fmt.Printf("\n%s❌ Request Failed%s\n", colorRed, colorReset)
				
				var pipelineErr *pipz.PipelineError[ai.AIRequest]
				if errors.As(err, &pipelineErr) {
					fmt.Printf("  Failed at: %s (stage %d)\n",
						pipelineErr.ProcessorName, pipelineErr.StageIndex)
				}
				
				fmt.Printf("  Error: %s\n", err.Error())
			} else {
				fmt.Printf("\n%s✅ Request Successful%s\n", colorGreen, colorReset)
				fmt.Printf("  Model: %s\n", result.Model)
				fmt.Printf("  Tokens Used: %d\n", result.TokensUsed)
				fmt.Printf("  Cost: $%.4f\n", result.Cost)
				fmt.Printf("  Response Time: %v\n", result.ResponseTime)
				fmt.Printf("  Processing Time: %v\n", duration)
				
				totalCost += result.Cost
				
				// Detect cache hits (very fast response time)
				if result.ResponseTime < 100*time.Millisecond {
					cacheHits++
					fmt.Printf("\n  %s💰 Served from cache - significant cost savings!%s\n",
						colorGreen, colorReset)
				}
				
				// Show response preview
				response := result.Response
				if len(response) > 150 {
					response = response[:150] + "..."
				}
				fmt.Printf("\n  Response: %s\"%s\"%s\n", colorGray, response, colorReset)
			}
		}
		
		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show final statistics
	fmt.Printf("\n%s═══ AI PIPELINE STATISTICS ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\nTotal Cost: $%.4f\n", totalCost)
	fmt.Printf("Cache Hits: %d\n", cacheHits)
	fmt.Printf("Average Cost per Request: $%.4f\n", totalCost/float64(len(scenarios)-1)) // -1 for burst test
	
	// Show cost comparison
	estimatedWithoutCache := totalCost + float64(cacheHits)*0.002 // Assume $0.002 per cached request
	fmt.Printf("Estimated Cost Without Cache: $%.4f\n", estimatedWithoutCache)
	if cacheHits > 0 {
		savings := estimatedWithoutCache - totalCost
		fmt.Printf("Cache Savings: $%.4f (%.1f%% reduction)\n", 
			savings, (savings/estimatedWithoutCache)*100)
	}

	return nil
}

func (a *AIExample) Benchmark(b *testing.B) error {
	return nil
}

func createDemoAIPipeline() pipz.Chainable[ai.AIRequest] {
	// Use the actual pipeline from the ai example
	return ai.CreateAIPipeline()
}