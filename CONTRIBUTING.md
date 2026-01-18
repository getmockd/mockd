# Contributing to mockd

Thank you for your interest in contributing to mockd! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.23 or later
- Git

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/mockd.git
   cd mockd
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/getmockd/mockd.git
   ```

### Building

```bash
go build ./...
```

### Running Tests

Run the full test suite with race detection:
```bash
go test -race ./...
```

Run with coverage:
```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Run specific test packages:
```bash
go test -v ./pkg/engine/...
go test -v ./tests/integration/...
```

### Running Benchmarks

```bash
go test -bench=. -benchmem ./tests/performance/...
```

## Code Style

- Follow standard Go conventions and idioms
- Run `go fmt` before committing
- Run `go vet` to check for issues
- Keep the stdlib-only approach - no external dependencies in core packages
- Use `testify` for test assertions (testing only)

## Project Structure

```
mockd/
├── pkg/
│   ├── admin/     # REST admin API
│   ├── config/    # Configuration types and file I/O
│   ├── engine/    # Core mock server engine
│   └── tls/       # TLS certificate generation
├── tests/
│   ├── integration/   # Integration tests
│   └── performance/   # Benchmark tests
└── examples/      # Example programs
```

## Making Changes

### Branch Naming

- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring

### Commit Messages

Write clear, descriptive commit messages:
- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Fix bug" not "Fixes bug")
- Keep the first line under 72 characters
- Reference issues when applicable

### Pull Request Process

1. Create a branch for your changes
2. Make your changes with tests
3. Run the full test suite
4. Update documentation if needed
5. Submit a pull request

### Code Review

All submissions require review. Expect feedback on:
- Code correctness and test coverage
- Performance implications
- API design and backwards compatibility
- Documentation completeness

## Testing Guidelines

### Unit Tests

- Place unit tests alongside the code they test (`*_test.go`)
- Test both success and error cases
- Use table-driven tests for multiple scenarios

### Integration Tests

- Place integration tests in `tests/integration/`
- Use dynamic port allocation to avoid conflicts
- Clean up resources after tests

### Performance Tests

- Place benchmarks in `tests/performance/`
- Include memory allocation metrics
- Document performance expectations

## Reporting Issues

When reporting issues, please include:
- Go version (`go version`)
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

## Feature Requests

Feature requests are welcome! Please describe:
- The problem you're trying to solve
- Your proposed solution
- Any alternatives you've considered

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
