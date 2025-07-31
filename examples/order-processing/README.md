# Order Processing Pipeline

This example demonstrates building a production-ready e-commerce order processing system that evolves from a simple "take payment" MVP to a sophisticated system with fraud detection, inventory management, and multi-channel notifications.

## The Story

You're building the order processing system for "ShopFast", an e-commerce platform. Follow the journey from startup MVP to enterprise-scale order management.

## Running the Example

```bash
# Run the full demo showing evolution through sprints
go run .

# Run with specific sprint stages
go run . -sprint=1  # MVP - Just take payment
go run . -sprint=3  # Add inventory management
go run . -sprint=5  # Add notifications
go run . -sprint=7  # Add fraud detection
go run . -sprint=9  # Add payment failure handling
go run . -sprint=11 # Full production system

# Run tests
go test -v

# Run benchmarks
go test -bench=. -benchmem
```

## Key Features Demonstrated

1. **State Evolution**: Order status changes through the pipeline
2. **Concurrent Side Effects**: Parallel notifications after payment
3. **Compensation Patterns**: Rollback inventory on payment failure
4. **Smart Routing**: Risk-based order processing paths
5. **External Service Integration**: Payment, inventory, notifications
6. **Real Business Metrics**: Conversion rates, revenue, fraud prevention

## Architecture Evolution

### Sprint 1: MVP (Week 1)
"Just take payments and ship orders!"
- Basic validation
- Charge payment
- Save order
- **Problem**: Payment succeeded but database failed - customer charged with no order!

### Sprint 3: Inventory Management (Week 3)
"We're overselling items!"
- Reserve inventory before payment
- Confirm after payment
- **Problem**: Black Friday - payments timeout but inventory stuck in "reserved"!

### Sprint 5: Customer Notifications (Week 5)
"Customers keep calling asking where their order is!"
- Email confirmations
- SMS updates
- Analytics tracking
- **Improvement**: All notifications in parallel - 2s → 400ms

### Sprint 7: Fraud Detection (Week 7)
"We lost $50k to fraud last month!"
- Risk scoring
- High-risk manual review
- Different processing paths
- **Result**: 90% fraud reduction

### Sprint 9: Payment Resilience (Week 9)
"Payment provider went down during our sale!"
- Retry logic
- Fallback handling
- Inventory cleanup
- **Result**: 99.9% order success rate

### Sprint 11: Enterprise Features (Week 11)
"We need loyalty points, recommendations, and B2B support!"
- Complete production system
- All features integrated
- **Achievement**: Processing 10k orders/hour

## Example Output

The demo shows real metrics as it runs:
- Order success rates
- Processing times
- Revenue processed
- Fraud prevented
- Inventory accuracy

## Code Structure

- `main.go` - Demo runner showing progression
- `types.go` - Core order types and constants
- `services.go` - Mock external services (payment, inventory, etc.)
- `pipeline.go` - Pipeline evolution with detailed sprint comments
- `pipeline_test.go` - Comprehensive test coverage

## Key Lessons

This example demonstrates how pipz enables:
- Building systems that evolve gracefully
- Handling complex business logic clearly
- Integrating unreliable external services
- Measuring and improving performance
- Maintaining code clarity as complexity grows