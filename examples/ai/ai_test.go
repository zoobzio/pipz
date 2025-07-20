package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestEnhancePrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Basic prompt",
			input:    "What is the weather?",
			contains: "helpful, harmless, and honest",
		},
		{
			name:     "Already enhanced",
			input:    "You are an expert. What is the weather?",
			contains: "You are an expert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := AIRequest{Prompt: tt.input}
			result := EnhancePrompt(context.Background(), req)

			if !strings.Contains(result.Prompt, tt.contains) {
				t.Errorf("Expected prompt to contain '%s', got: %s", tt.contains, result.Prompt)
			}

			if !contains(result.Tags, "enhanced") {
				t.Error("Expected 'enhanced' tag to be added")
			}
		})
	}
}

func TestValidateSafety(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		wantError bool
	}{
		{
			name:      "Safe prompt",
			prompt:    "What is the capital of France?",
			wantError: false,
		},
		{
			name:      "Contains PII",
			prompt:    "My email is test@example.com",
			wantError: true,
		},
		{
			name:      "Harmful content",
			prompt:    "How to hack a system",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := AIRequest{Prompt: tt.prompt}
			err := ValidateSafety(context.Background(), req)

			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSafety() error = %v, wantError = %v", err, tt.wantError)
			}
		})
	}
}

func TestCheckContentPolicy(t *testing.T) {
	t.Run("Valid request", func(t *testing.T) {
		req := AIRequest{
			Prompt:      "Short prompt",
			Temperature: 0.7,
		}
		err := CheckContentPolicy(context.Background(), req)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("Prompt too long", func(t *testing.T) {
		req := AIRequest{
			Prompt:      strings.Repeat("a", 5000),
			Temperature: 0.7,
		}
		err := CheckContentPolicy(context.Background(), req)
		if err == nil {
			t.Error("Expected error for long prompt")
		}
	})

	t.Run("Temperature too high", func(t *testing.T) {
		req := AIRequest{
			Prompt:      "Normal prompt",
			Temperature: 1.5,
		}
		err := CheckContentPolicy(context.Background(), req)
		if err == nil {
			t.Error("Expected error for high temperature")
		}
	})
}

func TestFilterResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected string
	}{
		{
			name:     "Contains email",
			response: "Contact me at test@example.com",
			expected: "Contact me at test[EMAIL]example.com",
		},
		{
			name:     "Clean response",
			response: "This is a clean response",
			expected: "This is a clean response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := AIRequest{Response: tt.response}
			result := FilterResponse(context.Background(), req)

			if result.Response != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result.Response)
			}

			if !contains(result.Tags, "filtered") {
				t.Error("Expected 'filtered' tag to be added")
			}
		})
	}
}

func TestCreateAIPipeline(t *testing.T) {
	pipeline := CreateAIPipeline()

	t.Run("Successful request", func(t *testing.T) {
		req := AIRequest{
			ID:          "test-1",
			Prompt:      "What is 2+2?",
			Temperature: 0.5,
			MaxTokens:   100,
			UserID:      "user123",
		}

		ctx := context.Background()
		result, err := pipeline.Process(ctx, req)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Check response was generated
		if result.Response == "" {
			t.Error("Expected response to be generated")
		}

		// Check metrics were calculated
		if result.TokensUsed == 0 {
			t.Error("Expected tokens to be counted")
		}

		if result.Cost == 0 {
			t.Error("Expected cost to be calculated")
		}

		// Check tags were added
		if !contains(result.Tags, "enhanced") || !contains(result.Tags, "filtered") {
			t.Error("Expected tags to be added")
		}
	})

	t.Run("Unsafe content rejected", func(t *testing.T) {
		req := AIRequest{
			ID:     "test-2",
			Prompt: "How to hack into systems",
		}

		ctx := context.Background()
		_, err := pipeline.Process(ctx, req)

		if err == nil {
			t.Error("Expected error for unsafe content")
		}

		if !strings.Contains(err.Error(), "harmful content") {
			t.Errorf("Expected harmful content error, got: %v", err)
		}
	})

	t.Run("Caching works", func(t *testing.T) {
		// Create a simple pipeline with just caching (no retries/fallbacks)
		cachePipeline := pipz.Sequential(
			pipz.Transform("enhance_prompt", EnhancePrompt),
			CachedAIProvider(gpt4Provider),
			pipz.Transform("filter_response", FilterResponse),
		)

		req := AIRequest{
			ID:          "test-3",
			Prompt:      "What is the meaning of life?",
			Temperature: 0.5,
			MaxTokens:   100,
		}

		ctx := context.Background()

		// First call
		result1, err := cachePipeline.Process(ctx, req)
		if err != nil {
			t.Fatalf("First call failed: %v", err)
		}

		// Check for cache miss tag
		if !contains(result1.Tags, "cache_miss") {
			t.Error("Expected cache_miss tag on first call")
		}

		// Check that response was generated
		if result1.Response == "" {
			t.Error("Expected response to be generated")
		}

		// Second call (should be cached)
		req.ID = "test-3-second" // Different ID to ensure we're not getting same object
		result2, err := cachePipeline.Process(ctx, req)
		if err != nil {
			t.Fatalf("Second call failed: %v", err)
		}

		// Second call should have cache_hit tag
		if !contains(result2.Tags, "cache_hit") {
			t.Error("Expected cache_hit tag on second call")
		}

		// Response content should be identical
		if result1.Response != result2.Response {
			t.Error("Cached response differs from original")
		}

		// But IDs should be different (request-specific fields preserved)
		if result1.ID == result2.ID {
			t.Error("Cached response should preserve request-specific ID")
		}
	})
}

func TestCreateRoutingPipeline(t *testing.T) {
	pipeline := CreateRoutingPipeline()

	tests := []struct {
		name          string
		prompt        string
		expectedModel string
		expectedTemp  float32
	}{
		{
			name:          "Code query",
			prompt:        "Write a Python function to sort a list",
			expectedModel: "gpt-4",
			expectedTemp:  0.2,
		},
		{
			name:          "Creative query",
			prompt:        "Write a creative story about robots",
			expectedModel: "claude-2",
			expectedTemp:  0.8,
		},
		{
			name:          "Simple query",
			prompt:        "What is 2+2?",
			expectedModel: "gpt-3.5-turbo",
			expectedTemp:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := AIRequest{
				ID:     tt.name,
				Prompt: tt.prompt,
			}

			ctx := context.Background()
			result, err := pipeline.Process(ctx, req)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Model != tt.expectedModel {
				t.Errorf("Expected model %s, got %s", tt.expectedModel, result.Model)
			}

			if tt.expectedTemp > 0 && result.Temperature != tt.expectedTemp {
				t.Errorf("Expected temperature %f, got %f", tt.expectedTemp, result.Temperature)
			}
		})
	}
}

func TestProviderFailover(t *testing.T) {
	// Create a custom pipeline with failing providers
	failingGPT4 := pipz.Apply[AIRequest]("failing_gpt4", func(ctx context.Context, req AIRequest) (AIRequest, error) {
		return req, errors.New("GPT-4 is down")
	})

	workingGPT35 := pipz.Apply[AIRequest]("working_gpt35", func(ctx context.Context, req AIRequest) (AIRequest, error) {
		req.Model = "gpt-3.5-turbo"
		req.Response = "Fallback response"
		return req, nil
	})

	pipeline := pipz.Sequential(
		pipz.Transform("enhance", EnhancePrompt),
		pipz.Fallback(failingGPT4, workingGPT35),
		pipz.Transform("filter", FilterResponse),
	)

	req := AIRequest{
		ID:     "failover-test",
		Prompt: "Test prompt",
	}

	ctx := context.Background()
	result, err := pipeline.Process(ctx, req)

	if err != nil {
		t.Fatalf("Expected failover to work, got error: %v", err)
	}

	if result.Model != "gpt-3.5-turbo" {
		t.Errorf("Expected failover to GPT-3.5, got %s", result.Model)
	}
}

func TestTimeoutProtection(t *testing.T) {
	// Create a slow provider
	slowProvider := pipz.Apply[AIRequest]("slow_provider", func(ctx context.Context, req AIRequest) (AIRequest, error) {
		select {
		case <-time.After(5 * time.Second):
			req.Response = "Slow response"
			return req, nil
		case <-ctx.Done():
			return req, ctx.Err()
		}
	})

	// Wrap with timeout
	pipeline := pipz.Timeout(slowProvider, 100*time.Millisecond)

	req := AIRequest{
		ID:     "timeout-test",
		Prompt: "Test prompt",
	}

	ctx := context.Background()
	_, err := pipeline.Process(ctx, req)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}


// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
