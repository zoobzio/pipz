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

## Commit Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automatic semantic versioning.

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: A new feature (triggers MINOR version bump)
- **fix**: A bug fix (triggers PATCH version bump)
- **docs**: Documentation only changes
- **style**: Changes that don't affect code meaning (formatting, etc.)
- **refactor**: Code changes that neither fix bugs nor add features
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **chore**: Changes to build process or auxiliary tools

### Breaking Changes

Add `BREAKING CHANGE:` in the commit footer or `!` after the type/scope to trigger a MAJOR version bump:

```
feat!: remove deprecated API endpoints

BREAKING CHANGE: The /v1/users endpoint has been removed
```

### Examples

```bash
# Patch release (0.0.1 -> 0.0.2)
git commit -m "fix: correct validation logic in pipeline processor"

# Minor release (0.0.2 -> 0.1.0)
git commit -m "feat: add support for parallel execution"

# Major release (0.1.0 -> 1.0.0)
git commit -m "feat!: redesign configuration format

BREAKING CHANGE: Config files now use YAML instead of JSON"
```

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes using conventional commits
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