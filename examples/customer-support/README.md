# Customer Support Chatbot Pipeline

This example demonstrates building a production-ready AI-powered customer support system that evolves from a simple MVP to a sophisticated multi-provider solution with intelligent routing, fallbacks, and performance optimization.

## The Story

You're building customer support for an e-commerce platform handling 10,000+ queries daily. Follow the journey from "just ship it" to production-grade AI support.

## Running the Example

```bash
# Run the full demo showing evolution through sprints
go run .

# Run with specific sprint stages
go run . -sprint=1  # MVP - Basic GPT
go run . -sprint=3  # Add cost optimization
go run . -sprint=5  # Add provider fallback
go run . -sprint=7  # Add speed optimization
go run . -sprint=9  # Add priority routing
go run . -sprint=11 # Full production

# Run tests
go test -v

# Run benchmarks
go test -bench=. -benchmem
```

## Key Features Demonstrated

1. **Progressive Enhancement**: Start simple, add complexity via feature flags
2. **Race Pattern**: Compete providers for fastest response
3. **Intelligent Routing**: Route queries based on type and urgency
4. **Multi-Provider Fallback**: Never let AI downtime stop support
5. **Real-time Metrics**: See cost savings and performance improvements

## Architecture Evolution

### Sprint 1: MVP (Week 1)
"Just get AI support working!"
- Basic GPT integration
- Simple question → answer flow

### Sprint 3: Cost Crisis (Week 3)
"We spent $50k on 'where is my order?' questions!"
- Route simple queries to GPT-3.5 ($0.002 vs $0.03)
- Keep complex issues on GPT-4
- **Savings**: 70% cost reduction

### Sprint 5: The Outage (Week 5)
"OpenAI is down and support queue is exploding!"
- Add Claude as fallback
- Add local model as last resort
- **Result**: 99.9% uptime

### Sprint 7: Speed Wars (Week 7)
"Customers leave after waiting 5 seconds!"
- Race providers for urgent queries
- Take first response
- **Result**: 2s → 800ms (60% faster)

### Sprint 9: Priority Lane (Week 9)
"Angry customers become chargebacks!"
- Detect sentiment
- Route urgent+angry to fastest path
- **Result**: 90% reduction in escalations

### Sprint 11: Production Ready (Week 11)
"We need safety, monitoring, and scale!"
- Rate limiting
- Safety checks
- Comprehensive metrics
- **Result**: Production-grade system

## Query Types

- **Simple** (70%): "Where is order #12345?"
- **Complex** (20%): "I want a refund for damaged items"
- **Urgent** (8%): "MY WEDDING IS TOMORROW WHERE IS MY DRESS???"
- **Technical** (2%): "API integration returns 403 error"

## Performance Metrics

The example shows real metrics as it runs:
- Response times for each provider
- Cost per query
- Cache hit rates
- Fallback usage
- Overall savings

## Code Structure

- `main.go` - Demo runner showing progression
- `types.go` - Core data types
- `services.go` - Mock AI providers and services
- `pipeline.go` - Pipeline evolution with detailed comments
- `pipeline_test.go` - Comprehensive test coverage