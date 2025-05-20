# Developer Guide

This guide provides detailed information for developers who want to understand and contribute to the Kaitai Struct Benthos plugin.

## Project Structure

```
benthos-kaitai-plugin/
├── cmd/
│   └── benthos-kaitai/           # Main application entry point
│       └── main.go
├── internal/
│   └── expression/               # Expression engine extensions
│       ├── arithmetic.go
│       ├── bitwise.go
│       └── functions.go
├── pkg/
│   └── kaitaistruct/             # Core implementation
│       ├── processor.go          # Benthos processor implementation
│       ├── interpreter.go        # Dynamic KSY interpreter
│       ├── serializer.go         # Binary serialization
│       ├── schema.go             # KSY schema types
│       └── expression.go         # Expression evaluation
├── testdata/
│   └── formats/                  # Test format definitions and samples
│       ├── png/
│       ├── gif/
│       └── ...
├── test/
│   ├── results/                  # Test output files
│   └── benthos_test_config.yaml  # Test configuration
├── scripts/
│   ├── prepare_test_data.sh      # Downloads test formats
│   └── integration_test.sh       # Runs integration tests
├── .github/
│   └── workflows/
│       └── build.yml             # CI pipeline definition
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
└── README.md                     # Project documentation
```

## Core Components

### 1. Benthos Processor (`processor.go`)

The plugin's entry point that implements the Benthos processor interface. It handles:
- Plugin registration with Benthos
- Configuration parsing
- Message processing
- Schema loading and caching

### 2. Schema Types (`schema.go`)

Defines Go structures that represent the Kaitai Struct schema (KSY) format:
- `KaitaiSchema`: Top-level schema definition
- `Meta`: Schema metadata
- `SequenceItem`: Field definitions
- `Type`: Custom type definitions
- `InstanceDef`: Calculated field definitions
- `EnumDef`: Enumeration definitions

### 3. Interpreter (`interpreter.go`)

The dynamic parser that reads binary data according to schema definitions:
- `KaitaiInterpreter`: Main interpreter type
- `ParseContext`: Maintains state during parsing
- Field parsing functions for different types and attributes
- Context stack for hierarchical field references

### 4. Serializer (`serializer.go`)

Converts JSON/structured data back to binary format:
- `KaitaiSerializer`: Main serializer type
- `SerializeContext`: Maintains state during serialization
- Field serialization functions matching the interpreter

### 5. Expression Engine (`expression.go` and `internal/expression/`)

Evaluates Kaitai expressions:
- Expression parsing and compilation
- Kaitai-specific syntax transformation
- Custom functions and operators
- Context-based evaluation

## Core Processes

### Binary Parsing Flow

1. `processor.Process()` receives a binary message
2. `parseBinary()` loads schema and creates `KaitaiInterpreter`
3. `interpreter.Parse()` creates a `kaitai.Stream` and initial parse context
4. Parsing processes fields according to schema definitions:
   - For each field in sequence:
     - Check conditions (`if` expressions)
     - Handle field size/repeat attributes
     - Parse field value (primitive, string, bytes, or compound)
     - Add to result structure
   - Process instance calculations
5. `parsedDataToMap()` converts result to JSON-compatible structure
6. Result is returned as a structured Benthos message

### Binary Serialization Flow

1. `processor.Process()` receives a structured message
2. `serializeToBinary()` loads schema and creates `KaitaiSerializer`
3. `serializer.Serialize()` creates output buffer and context
4. Serialization processes fields according to schema:
   - For each field in sequence:
     - Check conditions (`if` expressions)
     - Get field value from input data
     - Serialize according to type and attributes
     - Write to output buffer
5. Binary result is returned as a raw Benthos message

## Expression Evaluation

Kaitai uses a custom expression language for:
- Size calculations
- Repeat counts
- Conditionals
- Instance values

The implementation:
1. Transforms Kaitai syntax to `govaluate` syntax
2. Adds custom functions and operators
3. Maintains proper evaluation context
4. Caches compiled expressions

Example transformations:
- `value & 0x80 != 0` → `bitAnd(value, 0x80) != 0`
- `condition ? value1 : value2` → `ternary(condition, value1, value2)`
- `value.to_s()` → `toString(value)`

## Development Workflow

### Setting Up Your Environment

1. Clone the repository:
   ```bash
   git clone https://github.com/yourorg/benthos-kaitai-plugin.git
   cd benthos-kaitai-plugin
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Prepare test data:
   ```bash
   ./scripts/prepare_test_data.sh
   ```

### Development Cycle

1. Make changes to the code
2. Run unit tests:
   ```bash
   go test ./pkg/kaitaistruct -v
   ```

3. Run integration tests:
   ```bash
   ./scripts/integration_test.sh
   ```

4. Build and run with Benthos:
   ```bash
   go build -o benthos-kaitai ./cmd/benthos-kaitai
   benthos --plugin-dir=. -c ./test/benthos_test_config.yaml
   ```

### Adding Support for New Kaitai Features

1. Identify the feature to implement (e.g., a new processing type)
2. Update schema types in `schema.go` if needed
3. Add parsing logic to `interpreter.go`
4. Add serialization logic to `serializer.go`
5. Add tests using appropriate sample data
6. Add documentation for the feature

### Adding a New Expression Function

1. Define the function in `expression.go` under `getExpressionFunctions()`
2. Add transformation rule in `transformKaitaiExpression()`
3. Add tests to `expression_test.go`

## Testing Strategy

### Unit Tests

- `interpreter_test.go`: Tests for binary parsing
- `serializer_test.go`: Tests for binary serialization
- `expression_test.go`: Tests for expression evaluation
- `processor_test.go`: Tests for Benthos integration

### Golden Tests

- Compare parsing results against known-good JSON
- Test roundtrip conversions (binary → JSON → binary)
- Update golden files with `go test -update`

### Benchmarks

- Measure parsing/serialization performance
- Track memory usage
- Compare against performance targets

### Integration Tests

- Test with Benthos runtime
- Process real binary formats
- Verify pipeline functionality

## Debugging Tips

1. **Logging Binary Data**:
   ```go
   fmt.Printf("Binary data (first 16 bytes): % X\n", binData[:min(16, len(binData))])
   ```

2. **Debugging Expressions**:
   ```go
   result, err := interpreter.evaluateExpression(expr, ctx)
   fmt.Printf("Expression '%s' result: %v (error: %v)\n", expr, result, err)
   ```

3. **Dumping Context**:
   ```go
   jsonCtx, _ := json.MarshalIndent(ctx.Children, "", "  ")
   fmt.Printf("Context: %s\n", jsonCtx)
   ```

4. **Running Benthos with Debug**:
   ```bash
   BENTHOS_LOG_LEVEL=DEBUG benthos --plugin-dir=. -c ./your_config.yaml
   ```

## Common Challenges

1. **Endianness Handling**: Make sure to respect schema's endianness settings
2. **Expression Context**: Expressions require correct parent/root context
3. **Type Conversions**: Handle numeric types properly between Go and JSON
4. **Memory Management**: Avoid deep recursion and excessive copying
5. **Error Contexts**: Provide good error messages with path information
