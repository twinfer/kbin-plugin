# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build
```bash
# Build the plugin binary
go build -o kbin-benthos-plugin ./cmd/kbin-plugin

# Ensure dependencies are correct
go mod tidy
```

### Test
```bash
# Run all tests
go test ./...

# Run tests for specific packages with verbose output
go test ./pkg/kaitaistruct -v
go test ./internal/cel -v
go test ./pkg/kaitaicel -v

# Run tests with coverage
go test -cover ./...
```

### Static Analysis
```bash
# Run go vet (will fail on scripts/kaitai-testgen-simple.go - this is a known issue)
go vet ./...

# Format code
go fmt ./...
```

## High-Level Architecture

This is a Benthos plugin that dynamically interprets Kaitai Struct (.ksy) schemas to parse and serialize binary data without requiring pre-compilation.

### Core Components

1. **KaitaiProcessor** (`cmd/kbin-plugin/main.go`)
   - Benthos processor interface implementation
   - Manages schema caching (main and framing schemas)
   - Handles message batch processing and metrics
   - Entry point for parse/serialize operations

2. **KaitaiInterpreter** (`pkg/kaitaistruct/parser.go`)
   - Core parsing/serialization engine
   - Recursively traverses schema structure
   - Manages ParseContext for state tracking
   - Integrates with CEL for expression evaluation

3. **KaitaiSchema** (`pkg/kaitaistruct/schema.go`)
   - Go representation of parsed .ksy YAML files
   - Defines types, sequences, instances, and metadata

4. **CEL Integration** (`internal/cel/` and `pkg/kaitaicel/`)
   - Custom CEL environment with Kaitai-specific functions
   - KaitaiCEL types implement Kaitai data types (KaitaiInt, KaitaiString, etc.)
   - Expression pool caches compiled CEL programs for performance

### Key Design Patterns

- **Dynamic Interpretation**: No code generation; schemas are interpreted at runtime
- **CEL for Expressions**: All conditional logic, calculations, and dynamic fields use CEL
- **Type Safety**: Custom KaitaiCEL types preserve both raw bytes and interpreted values
- **Caching**: Schemas and compiled expressions are cached for performance

### Package Structure

- `cmd/kbin-plugin/`: Main entry point and Benthos processor
- `pkg/kaitaistruct/`: Core parsing/serialization logic
- `pkg/kaitaicel/`: Kaitai-specific CEL types (integers, strings, enums, bitfields)
- `internal/cel/`: CEL environment setup and custom functions
- `pkg/expression/`: Expression parser for legacy support
- `testutil/`: Test utilities
- `test/`: Kaitai test format files (.ksy) and test data

## Important Notes

- The project uses Go 1.24+ with modern features like generics
- There's a known build issue with `scripts/kaitai-testgen-simple.go` (fmt.Sprintf argument mismatch)
- Tests extensively use Kaitai Struct's official test formats for validation
- The plugin supports both parsing (binary to JSON) and serialization (JSON to binary)

Memory Update:
   - If any new information was gathered during the interaction, update your memory as follows:
     a) Create entities for recurring API signature , package documentation, and significant change
     b) Connect them to the current entities using relations
     b) Store facts about them as observations