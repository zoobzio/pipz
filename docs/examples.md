# Examples Guide

Complete, runnable examples demonstrating how pipz solves real-world problems.

## Overview

The pipz examples directory contains five production-inspired examples that show the evolution from simple requirements to complex, resilient systems. Each example tells a story and demonstrates specific pipz patterns.

## Running Examples

All examples are self-contained and runnable:

```bash
# Run any example
cd examples/order-processing
go run .

# Run tests
go test -v

# Run benchmarks
go test -bench=. -benchmem
```

## Available Examples

### 1. Order Processing

**Location:** `/examples/order-processing/`

**Story:** Build an e-commerce order processing system that evolves from a simple "take payment" MVP to a sophisticated system with fraud detection, inventory management, and multi-channel notifications.

**Key Concepts Demonstrated:**
- Pipeline evolution from MVP to production
- State management through processing stages
- Concurrent side effects (notifications)
- Compensation patterns for failure recovery
- Smart routing based on risk assessment
- Real business metrics tracking

**Run Modes:**
```bash
# Show full evolution
go run .

# Run specific stages
go run . -sprint=1  # MVP - Just payments
go run . -sprint=5  # Add notifications
go run . -sprint=7  # Add fraud detection
go run . -sprint=11 # Full production system
```

**What You'll Learn:**
- How to start simple and evolve
- When to use concurrent vs sequential processing
- How to handle payment failures gracefully
- Implementing compensation patterns

### 2. User Profile Update

**Location:** `/examples/user-profile-update/`

**Story:** Handle complex multi-step user profile updates with external services, including image processing, content moderation, and distributed cache invalidation.

**Key Concepts Demonstrated:**
- Multi-step operations with external dependencies
- Image processing with fallback strategies
- Content moderation with graceful degradation
- Distributed cache invalidation patterns
- Transactional-like behavior with compensation

**What You'll Learn:**
- Handling external service failures
- Implementing graceful degradation
- Managing distributed state
- Building compensating transactions

### 3. Customer Support

**Location:** `/examples/customer-support/`

**Story:** Build an intelligent ticket routing system with AI-powered classification, sentiment analysis, escalation handling, and SLA management.

**Key Concepts Demonstrated:**
- Intelligent routing with Switch connectors
- AI/ML service integration patterns
- Priority-based processing
- SLA enforcement with timeouts
- Multi-channel support integration

**What You'll Learn:**
- Dynamic routing based on conditions
- Integrating ML services
- Priority queue patterns
- SLA management strategies

### 4. Event Orchestration

**Location:** `/examples/event-orchestration/`

**Story:** Create a complex event processing system with routing, compliance checks, maintenance windows, and parallel distribution.

**Key Concepts Demonstrated:**
- Event routing patterns
- Compliance and safety measures
- Maintenance window handling
- Parallel event distribution
- Event deduplication

**What You'll Learn:**
- Building event-driven architectures
- Implementing compliance checks
- Handling maintenance windows
- Event distribution patterns

### 5. Shipping Fulfillment

**Location:** `/examples/shipping-fulfillment/`

**Story:** Integrate with multiple shipping providers for smart carrier selection, rate shopping, and tracking updates.

**Key Concepts Demonstrated:**
- Multi-provider integration
- Smart selection based on criteria
- Rate shopping and optimization
- Status tracking and updates
- Provider-specific error handling

**What You'll Learn:**
- Integrating multiple external APIs
- Implementing selection algorithms
- Rate comparison patterns
- Handling provider-specific quirks

## Common Patterns Across Examples

### 1. Progressive Enhancement
All examples start simple and add complexity gradually, showing how pipz scales with your needs.

### 2. Error Recovery
Each example demonstrates different error recovery strategies:
- Fallback to defaults
- Retry with backoff
- Circuit breaking
- Compensation patterns

### 3. External Service Integration
Examples show how to integrate with external services safely:
- Timeouts for all external calls
- Graceful degradation
- Caching strategies
- Rate limiting

### 4. Monitoring and Metrics
Examples include metrics collection showing:
- Success rates
- Processing times
- Business metrics
- Error tracking

## Example Structure

Each example follows a consistent structure:

```
example-name/
├── README.md           # Detailed story and explanation
├── main.go            # Demo runner
├── types.go           # Core data types
├── services.go        # Mock external services
├── pipeline.go        # Pipeline implementation
└── pipeline_test.go   # Comprehensive tests
```

## Learning Path

For newcomers to pipz, we recommend this order:

1. **Order Processing** - Start here to understand basic concepts
2. **Shipping Fulfillment** - Learn multi-provider patterns
3. **User Profile Update** - Understand compensation patterns
4. **Customer Support** - Master routing and prioritization
5. **Event Orchestration** - Advanced event-driven patterns

## Key Takeaways

### When to Use pipz

The examples demonstrate that pipz excels when you need:
- Composable, reusable processing steps
- Built-in resilience patterns
- Clear error handling with context
- Evolution from simple to complex
- Type-safe data processing

### Design Philosophy

The examples embody pipz's philosophy:
- **Start simple** - MVP first, enhance later
- **Compose don't configure** - Build from small pieces
- **Fail fast with context** - Rich error information
- **Type safety** - Catch errors at compile time
- **Production ready** - Built-in resilience patterns

## Running All Examples

To run all examples and see the patterns in action:

```bash
#!/bin/bash
for dir in examples/*/; do
    echo "Running $(basename $dir)..."
    (cd "$dir" && go run .)
    echo "---"
done
```

## Contributing Examples

Have a great use case for pipz? We welcome example contributions! Examples should:

1. Tell a compelling story
2. Start simple and evolve
3. Demonstrate specific patterns
4. Include comprehensive tests
5. Use realistic (but stubbed) services

See the existing examples for the expected structure and style.

## Next Steps

After exploring the examples:

1. Read the [Getting Started](./tutorials/getting-started.md) tutorial
2. Review the [Cookbook](./cookbook/) for specific recipes
3. Check the [API Reference](./reference/) for detailed documentation
4. Build your own pipeline!

## Questions?

If the examples don't cover your use case:

1. Check the [Troubleshooting Guide](./troubleshooting.md)
2. Browse the [Cookbook](./cookbook/) for recipes
3. Open an [issue](https://github.com/zoobzio/pipz/issues) with your question