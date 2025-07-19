# pipz Examples

This directory contains practical examples demonstrating various pipz patterns and use cases.

## Example Overview

### Core Patterns

#### 🏢 Business Validation (`validation/`)
Simple business rule validation for domain objects like orders and users.
- **Use when**: Validating business logic constraints
- **Key patterns**: Sequential validation, error accumulation
- **Good for**: Order validation, user registration, form processing

#### 🔐 Security (`security/`)
Data security patterns including authentication, authorization, and audit trails.
- **Use when**: Implementing access control and compliance
- **Key patterns**: Auth checks, data redaction, audit logging
- **Good for**: GDPR compliance, multi-tenant systems, sensitive data handling

### Protocol-Specific Examples

#### 🌐 HTTP Middleware (`middleware/`)
Request/response processing for HTTP services.
- **Use when**: Building REST APIs or web services
- **Key patterns**: Auth middleware, rate limiting, request logging
- **Good for**: API gateways, microservices, web applications

#### 📨 Event Processing (`events/`)
High-throughput event processing with routing and deduplication.
- **Use when**: Building event-driven architectures
- **Key patterns**: Event routing, deduplication, dead letter queues
- **Good for**: Message queues, webhooks, real-time systems

### Domain-Specific Examples

#### 💳 Payment Processing (`payment/`)
Financial transaction processing with sophisticated error recovery.
- **Use when**: Handling payment failures and recovery
- **Key patterns**: Error categorization, retry strategies, provider failover
- **Good for**: E-commerce, subscription services, payment gateways

#### 🤖 AI Pipeline (`ai/`)
LLM request processing with caching, routing, and fallbacks.
- **Use when**: Integrating with AI/ML services
- **Key patterns**: Provider routing, response caching, content filtering
- **Good for**: Chatbots, content generation, AI-powered features

### Comprehensive Examples

#### 🔄 ETL Pipeline (`etl/`)
Full-featured data processing with multiple formats and transformations.
- **Use when**: Processing data files or streams
- **Key patterns**: Schema validation, type conversion, progress tracking
- **Good for**: Data migration, batch processing, analytics pipelines

## Pattern Reference

### Need to validate data?
- **Simple validation** → See `validation/`
- **With schema support** → See `etl/`
- **Security validation** → See `security/`

### Need error handling?
- **Basic error handling** → See `validation/`
- **With retry/recovery** → See `payment/`
- **With fallback providers** → See `ai/`

### Need routing/switching?
- **Content-based routing** → See `events/`
- **Provider selection** → See `ai/`
- **Priority routing** → See `events/`

### Need transformation?
- **Field mapping** → See `etl/`
- **Type conversion** → See `etl/`
- **Data enrichment** → See `etl/` or `events/`

### Need caching?
- **Request caching** → See `ai/`
- **Deduplication** → See `events/`

### Need batch processing?
- **File processing** → See `etl/`
- **Event batches** → See `events/`

## Running Examples

Each example includes:
- `go.mod` - Module definition
- `*.go` - Implementation
- `*_test.go` - Tests demonstrating usage
- `*_bench_test.go` - Performance benchmarks

To run an example:

```bash
cd validation
go test -v        # Run tests
go test -bench=.  # Run benchmarks
```

## Key Concepts Demonstrated

1. **Composability** - All examples show how to compose simple processors into complex pipelines
2. **Error Context** - Errors include location and context for debugging
3. **Type Safety** - Generic pipelines maintain type safety throughout
4. **Performance** - Benchmarks show real-world performance characteristics
5. **Testability** - Each processor can be tested in isolation

## Which Example Should I Start With?

- **New to pipz?** → Start with `validation/`
- **Building APIs?** → Check out `middleware/`
- **Processing files?** → Look at `etl/`
- **Event-driven system?** → See `events/`
- **Need resilience?** → Study `payment/` or `ai/`