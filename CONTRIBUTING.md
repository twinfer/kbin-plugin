# Contributing Guidelines

Thank you for your interest in contributing to the Kaitai Struct Benthos plugin! This document provides guidelines and instructions for contributing.

## Code of Conduct

We expect all contributors to follow our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before participating.

## How to Contribute

### Reporting Bugs

Before submitting a bug report:

1. **Check existing issues** to see if the problem has already been reported
2. **Use the latest version** to confirm the issue still exists
3. **Collect information** about the bug:
   - Steps to reproduce
   - Expected vs. actual behavior
   - Error messages or stack traces
   - Binary samples if possible (or ways to generate them)
   - Schema files relevant to the issue

File your report with a clear title and detailed description.

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

1. **Clear title and description**
2. **Specific use case** explaining why this enhancement would be useful
3. **Describe the solution you'd like** with as much detail as possible
4. **Describe alternatives** you've considered
5. **Additional context** like screenshots or examples

### Pull Requests

1. **Start with an issue**: Create or find an issue describing the problem you're solving
2. **Fork the repository** and create a branch for your feature
3. **Write tests** that verify your code works as expected
4. **Follow the coding style** of the project
5. **Write clear commit messages**
6. **Document your changes** in the PR description
7. **Submit your PR** for review

#### Pull Request Process

1. Update the README.md and/or documentation with details of changes if appropriate
2. Add or update tests for your changes
3. Run the test suite to ensure existing functionality isn't broken
4. The PR requires approval from at least one maintainer before merging
5. After approval, a maintainer will merge your PR

## Development Environment Setup

1. **Prerequisites**:
   - Go 1.19 or later
   - Benthos 4.0 or later
   - Git

2. **Setting up your workspace**:
   ```bash
   # Clone the repository
   git clone https://github.com/yourorg/benthos-kaitai-plugin.git
   cd benthos-kaitai-plugin

   # Install dependencies
   go mod download

   # Prepare test data
   ./scripts/prepare_test_data.sh
   ```

3. **Running tests**:
   ```bash
   # Run unit tests
   go test ./pkg/kaitaistruct

   # Run specific tests
   go test ./pkg/kaitaistruct -run TestParsingGolden

   # Update golden files
   go test ./pkg/kaitaistruct -update

   # Run benchmarks
   go test ./pkg/kaitaistruct -bench .

   # Run integration tests
   ./scripts/integration_test.sh
   ```

4. **Building and running**:
   ```bash
   # Build the plugin
   go build -o benthos-kaitai ./cmd/benthos-kaitai

   # Run Benthos with the plugin
   benthos --plugin-dir=. -c ./test/benthos_test_config.yaml
   ```

## Coding Guidelines

### General

- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use meaningful variable and function names
- Keep functions focused and short when possible
- Add comments for non-obvious code
- Write tests for your code

### Project-Specific Guidelines

1. **Error Handling**:
   - Return meaningful errors with context
   - Format errors with `fmt.Errorf("context: %w", err)`
   - Don't swallow errors unless there's a good reason

2. **Documentation**:
   - Document all exported functions, types, and constants
   - Include examples for complex functionality
   - Keep documentation up to date with code changes

3. **Testing**:
   - Add unit tests for new features
   - Add or update golden tests as appropriate
   - Consider performance when writing tests
   - Test edge cases and error conditions

4. **Performance**:
   - Consider memory usage for streaming contexts
   - Cache expensive operations like schema parsing
   - Avoid unnecessary allocations in hot paths

5. **Compatibility**:
   - Maintain compatibility with the Kaitai Struct specification
   - Follow Benthos processor conventions
   - Support a wide range of schema features

## Adding Kaitai Features

When adding support for a new Kaitai Struct feature:

1. Update schema types in `schema.go` if needed
2. Add parsing logic to `interpreter.go`
3. Add serialization logic to `serializer.go`
4. Add tests with appropriate sample data
5. Update documentation

## Git Workflow

We follow a standard GitHub flow:

1. **Fork** the repository
2. Create a **feature branch** (`feature/your-feature-name`)
3. Make your changes and **commit** them
4. **Push** to your fork
5. Submit a **pull request**

### Commit Messages

Follow these guidelines for commit messages:

- Use a clear and descriptive title
- Separate title from body with a blank line
- Use the imperative mood ("Add feature" not "Added feature")
- Explain what and why, not how
- Reference issues and pull requests

Example:
```
Add support for process/xor in KSY schemas

Implement XOR processing for binary data as specified in the Kaitai docs.
This allows decoding XOR-encoded fields.

Fixes #123
```

## Release Process

Our release process follows these steps:

1. Update version number in code
2. Update CHANGELOG.md with notable changes
3. Create a new GitHub release with release notes
4. Tag the release with a version number
5. Build and publish artifacts

## Questions?

If you have questions or need help, you can:

- Open an issue with your question
- Contact the maintainers directly
- Join our community discussion channels

Thank you for contributing!
