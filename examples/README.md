# pipz Examples

This directory contains real-world examples demonstrating how pipz solves common problems developers face every day. Each example tells a story about a specific problem and shows how pipz provides an elegant solution.

## Examples

1. **[Order Processing](./order-processing/)**
   - Production e-commerce order processing system
   - Evolution from MVP to enterprise scale
   - Payment processing with fraud detection
   - Inventory management and notifications
   - Compensation patterns and error recovery

2. **[User Profile Update](./user-profile-update/)**
   - Handle complex multi-step operations with external services
   - Image processing with fallbacks
   - Content moderation with graceful degradation
   - Distributed cache invalidation
   - Transactional-like behavior with compensation

3. **[Customer Support](./customer-support/)**
   - Intelligent ticket routing and prioritization
   - AI-powered classification and sentiment analysis
   - Escalation handling and SLA management
   - Multi-channel support integration

4. **[Event Orchestration](./event-orchestration/)**
   - Complex event routing and processing
   - Compliance and safety measures
   - Maintenance window handling
   - Parallel event distribution

5. **[Shipping Fulfillment](./shipping-fulfillment/)**
   - Multi-provider shipping integration
   - Smart carrier selection based on criteria
   - Rate shopping and optimization
   - Tracking and status updates

## Philosophy

Each example follows these principles:

- **Real Problems**: Address actual pain points developers experience
- **Production Patterns**: Show patterns you'd use in production systems
- **Progressive Complexity**: Start simple, build up to advanced patterns
- **Stubbed Services**: Use stubbed external services to demonstrate behavior
- **Error Scenarios**: Show how pipz handles common failure modes

## Running Examples

Each example includes:

- A detailed README explaining the problem and solution
- Stubbed implementations demonstrating the pipeline behavior
- Tests showing success and failure scenarios
- Benchmarks comparing with traditional approaches

```bash
# Run any example
cd order-processing
go test -v
go run .

# Most examples support different scenarios or modes
cd order-processing
go run . -sprint=1  # Show MVP version
go run . -sprint=11 # Show full production version
```
