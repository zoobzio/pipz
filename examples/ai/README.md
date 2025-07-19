# AI Pipeline Example

This example demonstrates how to build production-ready AI/LLM pipelines using pipz connectors for resilience, performance, and safety.

## Features

- **Prompt Enhancement**: Automatically adds system instructions for better responses
- **Safety Validation**: Checks for PII and harmful content before sending to AI
- **Multi-Provider Support**: GPT-4, GPT-3.5, and Claude with automatic failover
- **Smart Routing**: Routes queries to appropriate models based on complexity
- **Caching**: Reduces costs by caching repeated queries
- **Retry Logic**: Handles transient failures with exponential backoff
- **Timeout Protection**: Prevents hanging on slow AI responses
- **Response Filtering**: Removes PII and unwanted content from responses
- **Cost Tracking**: Monitors token usage and costs per request

## Architecture

### Basic Pipeline
```
Enhance → Safety Check → AI Call (with retry/timeout/fallback) → Filter → Metrics
```

### Routing Pipeline
```
Enhance → Safety Check → Route by Type → Specialized Processing → Filter
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/zoobzio/pipz/examples/ai"
)

func main() {
    // Create the AI pipeline with all resilience features
    pipeline := ai.CreateAIPipeline()
    
    // Process a request
    req := ai.AIRequest{
        ID:          ai.GenerateRequestID("demo"),
        Prompt:      "Explain quantum computing in simple terms",
        Temperature: 0.7,
        MaxTokens:   500,
        UserID:      "user123",
        SessionID:   "session456",
    }
    
    ctx := context.Background()
    result, err := pipeline.Process(ctx, req)
    if err != nil {
        log.Fatalf("AI request failed: %v", err)
    }
    
    fmt.Printf("Response: %s\n", result.Response)
    fmt.Printf("Model used: %s\n", result.Model)
    fmt.Printf("Cost: $%.4f\n", result.Cost)
    fmt.Printf("Response time: %v\n", result.ResponseTime)
}
```

## Connectors in Action

### 1. Custom Caching
Domain-specific caching built into the AI providers:
```go
// Each provider has its own cache based on prompt content
CachedAIProvider(gpt4Provider)
```

### 2. Retry with Backoff
Handles transient failures gracefully:
```go
pipz.RetryWithBackoff(
    aiCall,
    3,                      // 3 attempts
    500*time.Millisecond,   // Initial delay
)
```

### 3. Timeout Protection
Prevents hanging on slow responses:
```go
pipz.Timeout(
    aiCall,
    10*time.Second,  // Max wait time
)
```

### 4. Fallback Chain
Tries multiple providers in order:
```go
pipz.Fallback(
    pipz.Fallback(gpt4, gpt35),  // Try GPT-4, then GPT-3.5
    claude,                       // Final fallback to Claude
)
```

### 5. Switch Router
Routes to different pipelines based on content:
```go
pipz.Switch(detectQueryType, map[string]pipz.Chainable[AIRequest]{
    "code":     codeOptimizedPipeline,
    "creative": creativeWritingPipeline,
    "simple":   cachedSimplePipeline,
    "complex":  powerfulModelPipeline,
})
```

## Performance Characteristics

From our benchmarks:

- **Cache Hit**: ~100μs (1000x faster than API call)
- **Cache Miss**: ~1-2s (depending on model)
- **Failover Overhead**: ~50ms (time to detect failure)
- **Pipeline Overhead**: <1ms (negligible compared to API latency)

Note: Caching is implemented as a custom processor specific to AI requests, 
not as a generic connector, allowing for type-specific cache key generation 
and field preservation.

## Cost Optimization

The example includes several cost-saving features:

1. **Caching**: Eliminates duplicate API calls
2. **Smart Routing**: Uses cheaper models for simple queries
3. **Fallback Strategy**: Only uses expensive models when needed
4. **Token Tracking**: Monitors usage to prevent overruns

## Safety Features

- **PII Detection**: Blocks personal information in prompts
- **Content Policy**: Enforces safety guidelines
- **Response Filtering**: Removes sensitive data from outputs
- **Audit Logging**: Tracks all requests for compliance

## Running the Example

```bash
cd examples/ai
go test -v                    # Run tests
go test -bench=. -benchmem    # Run benchmarks
```

## Key Takeaways

1. **Composability**: Complex behaviors from simple building blocks
2. **Resilience**: Automatic handling of failures and edge cases
3. **Performance**: Caching and smart routing reduce latency and cost
4. **Safety**: Multiple validation layers ensure safe AI usage
5. **Observability**: Built-in metrics and logging for monitoring

This example shows how pipz transforms complex AI integration challenges into manageable, testable components that compose into production-ready pipelines.