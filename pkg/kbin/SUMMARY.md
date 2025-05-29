# kbin Package Summary

## Overview

The `pkg/kbin` package provides a high-level, easy-to-use Go API for working with Kaitai Struct binary format specifications. It abstracts away the complexity of the underlying `kaitaistruct` package while providing powerful features like caching, JSON serialization, and flexible configuration.

## Package Structure

```
pkg/kbin/
├── doc.go                      # Package documentation
├── kbin.go                     # Main API implementation
├── kbin_test.go               # Comprehensive tests
├── example_test.go            # Example functions for documentation
├── integration_example_test.go # Complete workflow example
├── README.md                  # User documentation
└── SUMMARY.md                 # This file
```

## API Surface

### Core Functions (Global Convenience API)

```go
// Basic parsing
func ParseBinary(data []byte, schemaPath string, opts ...Option) (map[string]any, error)
func ParseBinaryWithContext(ctx context.Context, data []byte, schemaPath string, opts ...Option) (map[string]any, error)

// JSON conversion
func SerializeToJSON(data []byte, schemaPath string, opts ...Option) ([]byte, error)
func SerializeToJSONWithContext(ctx context.Context, data []byte, schemaPath string, opts ...Option) ([]byte, error)

// JSON to binary
func SerializeFromJSON(jsonData []byte, schemaPath string, opts ...Option) ([]byte, error)
func SerializeFromJSONWithContext(ctx context.Context, jsonData []byte, schemaPath string, opts ...Option) ([]byte, error)

// Validation
func ValidateSchema(schemaPath string) error
```

### Parser Type (Advanced API)

```go
type Parser struct {
    // Private fields for caching and configuration
}

func NewParser(opts ...Option) *Parser
func (p *Parser) ParseBinary(ctx context.Context, data []byte, schemaPath string, opts ...Option) (map[string]any, error)
func (p *Parser) SerializeToJSON(ctx context.Context, data []byte, schemaPath string, opts ...Option) ([]byte, error)
func (p *Parser) SerializeFromJSON(ctx context.Context, jsonData []byte, schemaPath string, opts ...Option) ([]byte, error)
func (p *Parser) ClearCache()
func (p *Parser) ValidateSchema(schemaPath string) error
```

### Configuration Options

```go
type Option func(*options)

func WithRootType(rootType string) Option
func WithLogger(logger *slog.Logger) Option
func WithCaching(timeout time.Duration) Option
func WithImportPaths(paths ...string) Option
func WithDebugMode(enabled bool) Option
```

## Key Features

### 1. **Simplicity**
- Clean, intuitive API that hides internal complexity
- Sensible defaults for most use cases
- Both global convenience functions and configurable parser instances

### 2. **Performance**
- Built-in schema caching with configurable timeouts
- Reusable parser instances to avoid recreation overhead
- Efficient streaming parsing via kaitai.Stream

### 3. **Flexibility**
- Comprehensive configuration options
- Support for custom loggers and debug modes
- Context support for cancellation and timeouts

### 4. **Type Safety**
- All functions return well-defined error types
- Consistent data type conversion (integers → int64, etc.)
- Clear separation between parsed data and metadata

### 5. **JSON Integration**
- Seamless conversion between binary ↔ structured data ↔ JSON
- Pretty-printed JSON output for human readability
- Round-trip support for data modification workflows

## Type Conversions

| Kaitai Type | Go Type |
|-------------|---------|
| u1, u2, u4, u8, s1, s2, s4, s8 | int64 |
| f4, f8 | float64 |
| str | string |
| bytes | []byte |
| arrays | []any |
| objects | map[string]any |

## Usage Patterns

### 1. **Simple One-Off Parsing**
```go
result, err := kbin.ParseBinary(data, "schema.ksy")
```

### 2. **Batch Processing**
```go
parser := kbin.NewParser(kbin.WithCaching(1*time.Hour))
for _, file := range files {
    result, err := parser.ParseBinary(ctx, data, "schema.ksy")
    // Process result...
}
```

### 3. **JSON Workflow**
```go
// Binary → JSON
jsonData, err := kbin.SerializeToJSON(binaryData, "schema.ksy")

// Modify JSON...

// JSON → Binary
newBinary, err := kbin.SerializeFromJSON(jsonData, "schema.ksy")
```

### 4. **Error Handling**
```go
result, err := kbin.ParseBinary(data, schemaPath,
    kbin.WithLogger(logger),
    kbin.WithDebugMode(true),
)
if err != nil {
    // Handle schema loading, parsing, or conversion errors
    log.Printf("Parse failed: %v", err)
    return
}
```

## Integration Examples

The package includes comprehensive examples in `examples/simple_usage.go` showing:

- HTTP handler integration
- CLI tool development
- Error handling patterns
- Configuration best practices

## Testing

The test suite includes:

- **Unit tests**: Core functionality verification
- **Integration tests**: Complete workflow validation
- **Example tests**: Documentation and usage patterns
- **Error condition tests**: Comprehensive error handling

Coverage includes:
- ✅ Basic parsing and serialization
- ✅ JSON round-trips
- ✅ Configuration options
- ✅ Schema validation
- ✅ Caching behavior
- ✅ Error conditions
- ⚠️ Enum serialization (known limitation)

## Dependencies

- `github.com/kaitai-io/kaitai_struct_go_runtime/kaitai`: Kaitai runtime
- `github.com/twinfer/kbin-plugin/pkg/kaitaistruct`: Core interpreter
- Standard library: `context`, `encoding/json`, `log/slog`, etc.

## Performance Characteristics

- **Memory**: Efficient streaming parsing, minimal memory overhead
- **CPU**: Schema and expression caching reduces repetitive parsing
- **I/O**: Single file read per schema, with optional caching
- **Concurrency**: Thread-safe design with proper synchronization

## Limitations and Future Work

### Current Limitations
1. Enum serialization requires special handling for round-trips
2. Import paths are handled at interpreter level, not in schema objects
3. Some advanced Kaitai features may require direct `kaitaistruct` usage

### Potential Enhancements
1. Stream-based parsing for very large files
2. Schema compilation and caching for maximum performance
3. Enhanced enum and validation support
4. Plugin system for custom data processors

## Conclusion

The `kbin` package successfully provides a clean, high-level API that makes Kaitai Struct functionality accessible to Go developers while maintaining the power and flexibility of the underlying system. It's designed for both simple one-off use cases and complex production applications requiring performance and configurability.