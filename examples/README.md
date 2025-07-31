# pipz Examples

This directory contains real-world examples demonstrating how pipz solves common problems developers face every day. Each example tells a story about a specific problem and shows how pipz provides an elegant solution.

## Examples

1. **[User Profile Update](./user-profile-update/)**
   - Handle complex multi-step operations with external services
   - Image processing with fallbacks
   - Content moderation with graceful degradation
   - Distributed cache invalidation
   - Transactional-like behavior with compensation

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
cd user-profile-update
go test -v
go run .
```
