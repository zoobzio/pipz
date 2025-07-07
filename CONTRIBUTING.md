# Contributing to pipz

Thank you for your interest in contributing to pipz! We welcome contributions that maintain the project's focus on simplicity and performance.

## Development Setup

1. Clone the repository
2. Install Go 1.23 or later
3. Install development tools:
   ```bash
   make install-tools
   ```

## Running Tests

```bash
# Run all tests
make test

# Run benchmarks
make bench

# Generate coverage report
make coverage
```

## Code Style

- Follow standard Go conventions
- Run `make lint` before submitting
- Keep the API simple and focused
- Maintain 100% test coverage

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Pull Request Guidelines

- Update tests for any new functionality
- Update documentation as needed
- Add examples if introducing new patterns
- Ensure all CI checks pass
- Keep commits focused and atomic

## Philosophy

When contributing, please keep these principles in mind:

- **Simplicity**: pipz should remain easy to understand
- **Performance**: Zero-allocation operations where possible
- **Type Safety**: Leverage Go's type system
- **No Magic**: Explicit is better than implicit
- **Composability**: Small pieces that work together

## Questions?

Feel free to open an issue for any questions or discussions!