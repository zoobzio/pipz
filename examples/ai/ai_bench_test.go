package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func BenchmarkAIPipeline(b *testing.B) {
	pipeline := CreateAIPipeline()
	ctx := context.Background()

	b.Run("Simple Query", func(b *testing.B) {
		req := AIRequest{
			Prompt:      "What is 2+2?",
			Temperature: 0.5,
			MaxTokens:   100,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("bench-%d", i)
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Complex Query", func(b *testing.B) {
		req := AIRequest{
			Prompt:      "Explain the theory of relativity in simple terms, including both special and general relativity. Provide examples and explain the key concepts like time dilation and space-time curvature.",
			Temperature: 0.7,
			MaxTokens:   500,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("bench-complex-%d", i)
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("With Cache Hit", func(b *testing.B) {
		req := AIRequest{
			ID:          "cache-test",
			Prompt:      "What is the capital of France?",
			Temperature: 0.5,
			MaxTokens:   100,
		}

		// Prime the cache
		_, _ = pipeline.Process(ctx, req)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("cache-bench-%d", i) // Different ID each time
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkRoutingPipeline(b *testing.B) {
	pipeline := CreateRoutingPipeline()
	ctx := context.Background()

	queries := []struct {
		name   string
		prompt string
	}{
		{"Code", "Write a function to calculate fibonacci numbers"},
		{"Creative", "Write a creative haiku about programming"},
		{"Simple", "What is the weather today?"},
		{"Complex", strings.Repeat("Analyze this data: ", 20) + "and provide insights"},
	}

	for _, q := range queries {
		b.Run(q.name, func(b *testing.B) {
			req := AIRequest{
				Prompt:      q.prompt,
				Temperature: 0.5,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req.ID = fmt.Sprintf("bench-route-%d", i)
				_, err := pipeline.Process(ctx, req)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConnectorOverhead(b *testing.B) {
	ctx := context.Background()

	// Direct call without pipeline
	b.Run("Direct Provider Call", func(b *testing.B) {
		provider := gpt35Provider
		req := AIRequest{
			Prompt: "Test prompt",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := provider.Complete(ctx, req)
			if err != nil && err.Error() != "rate limit exceeded" {
				b.Fatal(err)
			}
		}
	})

	// With minimal pipeline
	b.Run("Minimal Pipeline", func(b *testing.B) {
		pipeline := CallAIProvider(gpt35Provider)
		req := AIRequest{
			Prompt: "Test prompt",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := pipeline.Process(ctx, req)
			if err != nil && err.Error() != "rate limit exceeded" {
				b.Fatal(err)
			}
		}
	})

	// With full pipeline
	b.Run("Full Pipeline", func(b *testing.B) {
		pipeline := CreateAIPipeline()
		req := AIRequest{
			Prompt: "Test prompt",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("bench-%d", i)
			_, err := pipeline.Process(ctx, req)
			if err != nil && err.Error() != "rate limit exceeded" {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCacheEfficiency(b *testing.B) {
	// Measure cache efficiency with different query patterns
	pipeline := CreateAIPipeline()
	ctx := context.Background()

	b.Run("All Different Queries", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := AIRequest{
				ID:     fmt.Sprintf("unique-%d", i),
				Prompt: fmt.Sprintf("Query number %d", i),
			}
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("50% Cache Hit Rate", func(b *testing.B) {
		queries := []string{
			"What is AI?",
			"Explain machine learning",
			"What is deep learning?",
			"How does NLP work?",
			"What is computer vision?",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Rotate through 5 queries for ~50% hit rate after warmup
			req := AIRequest{
				ID:     fmt.Sprintf("rotating-%d", i%10),
				Prompt: queries[i%5],
			}
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("90% Cache Hit Rate", func(b *testing.B) {
		// Use only 2 queries for high hit rate
		queries := []string{"What is 2+2?", "What is 3+3?"}

		// Warm up cache
		for _, q := range queries {
			req := AIRequest{ID: q, Prompt: q}
			pipeline.Process(ctx, req)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := AIRequest{
				ID:     queries[i%2],
				Prompt: queries[i%2],
			}
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkFailoverPerformance(b *testing.B) {
	// Measure failover overhead

	// Always failing primary
	failingPrimary := pipz.Apply[AIRequest]("failing", func(ctx context.Context, req AIRequest) (AIRequest, error) {
		time.Sleep(50 * time.Millisecond) // Simulate some work before failing
		return req, errors.New("primary failed")
	})

	// Working fallback
	workingFallback := pipz.Apply[AIRequest]("fallback", func(ctx context.Context, req AIRequest) (AIRequest, error) {
		time.Sleep(100 * time.Millisecond) // Simulate work
		req.Response = "Fallback response"
		return req, nil
	})

	pipeline := pipz.Fallback(failingPrimary, workingFallback)
	ctx := context.Background()

	b.Run("Failover Overhead", func(b *testing.B) {
		req := AIRequest{
			Prompt: "Test prompt",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("failover-%d", i)
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkRetryBackoffOverhead(b *testing.B) {
	// Measure retry with backoff overhead

	attempts := 0
	// Fails twice, succeeds on third
	flakyProvider := pipz.Apply[AIRequest]("flaky", func(ctx context.Context, req AIRequest) (AIRequest, error) {
		attempts++
		if attempts%3 == 0 {
			req.Response = "Success"
			return req, nil
		}
		return req, errors.New("temporary failure")
	})

	pipeline := pipz.RetryWithBackoff(flakyProvider, 3, 10*time.Millisecond)
	ctx := context.Background()

	b.Run("Retry Backoff Overhead", func(b *testing.B) {
		req := AIRequest{
			Prompt: "Test prompt",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("retry-%d", i)
			_, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

var globalResult AIRequest // Prevent compiler optimizations

func BenchmarkMemoryAllocation(b *testing.B) {
	pipeline := CreateAIPipeline()
	ctx := context.Background()

	b.Run("Memory Per Request", func(b *testing.B) {
		req := AIRequest{
			Prompt:      "What is artificial intelligence?",
			Temperature: 0.7,
			MaxTokens:   100,
		}

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			req.ID = fmt.Sprintf("mem-%d", i)
			result, err := pipeline.Process(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
			globalResult = result // Prevent optimization
		}
	})
}
