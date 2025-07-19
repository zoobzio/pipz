# Changelog

All notable changes to pipz will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive examples showcasing real-world use cases
- API_REFERENCE.md for complete API documentation
- CONTRIBUTING.md with contribution guidelines

### Changed
- Updated all documentation to reflect connector-based API
- Improved doc.go with current patterns and best practices

### Fixed
- Removed references to deprecated Pipeline/Chain API in documentation

## [v0.5.1] - 2025-07-15

### Fixed
- Minor bug fixes and performance improvements

## [v0.5.0] - 2025-07-14

### Added
- Connector-based composition system replacing chains
- Sequential connector for ordered processing
- Switch connector for conditional routing
- Fallback connector for error recovery
- Retry and RetryWithBackoff connectors
- Timeout connector for time-bounded operations

### Changed
- **BREAKING**: Removed Chain API in favor of connectors
- Simplified API surface with just Chainable[T] interface
- All composition now done through connector functions

### Removed
- Chain type and associated methods
- Pipeline type (functionality merged into connectors)

## [v0.4.0] - 2025-07-13

### Added
- Pipeline control functions for dynamic modification
- Factory pattern for runtime pipeline assembly
- Schema-based pipeline configuration

## [v0.3.0] - 2025-07-13

### Added
- Context support throughout the API
- Error context with processor names
- Performance benchmarks

### Changed
- All processors now accept context.Context
- Improved error messages with location information

## [v0.2.1] - 2025-07-13

### Fixed
- Type safety issues with generic constraints
- Memory allocations in hot paths
