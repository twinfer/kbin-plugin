# kbin Package

A high-level Go API for parsing and serializing binary data using Kaitai Struct format specifications.

## Features

- **Simple API**: Easy-to-use functions for parsing binary data to Go maps and serializing back to binary
- **JSON Support**: Convert binary data to JSON and back
- **Schema Caching**: Built-in caching for improved performance when processing multiple files with the same schema
- **Context Support**: Full context support for cancellation and timeouts
- **Flexible Options**: Configurable logging, caching, and debug modes

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/twinfer/kbin-plugin/pkg/kbin"
)

func main() {
    // Your binary data
    data := []byte{0x4B, 0x42, 0x49, 0x4E, 0x01, 0x00, 0x05, 0x48, 0x65, 0x6C, 0x6C, 0x6F}
    
    // Parse binary data to a map
    result, err := kbin.ParseBinary(data, "path/to/schema.ksy")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Parsed data: %+v\n", result)
    
    // Convert to JSON
    jsonData, err := kbin.SerializeToJSON(data, "path/to/schema.ksy")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("JSON: %s\n", jsonData)
    
    // Convert JSON back to binary
    binaryData, err := kbin.SerializeFromJSON(jsonData, "path/to/schema.ksy")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Round-trip successful: %v\n", len(data) == len(binaryData))
}
```

### Using Options

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "time"
    
    "github.com/twinfer/kbin-plugin/pkg/kbin"
)

func main() {
    // Create a custom logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))
    
    // Parse with options
    data := []byte{0x01, 0x02, 0x03, 0x04}
    result, err := kbin.ParseBinary(data, "schema.ksy",
        kbin.WithLogger(logger),
        kbin.WithRootType("custom_type"),
        kbin.WithDebugMode(true),
        kbin.WithCaching(10*time.Minute),
        kbin.WithImportPaths("/path/to/imports"),
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Parsed: %+v\n", result)
}
```

### Using a Custom Parser Instance

```go
package main

import (
    "context"
    "time"
    
    "github.com/twinfer/kbin-plugin/pkg/kbin"
)

func main() {
    // Create a parser with specific configuration
    parser := kbin.NewParser(
        kbin.WithCaching(30*time.Minute),
        kbin.WithDebugMode(false),
    )
    
    // Use the parser multiple times
    data1 := []byte{0x01, 0x02, 0x03}
    data2 := []byte{0x04, 0x05, 0x06}
    
    result1, err := parser.ParseBinary(context.Background(), data1, "schema.ksy")
    if err != nil {
        log.Fatal(err)
    }
    
    result2, err := parser.ParseBinary(context.Background(), data2, "schema.ksy")
    if err != nil {
        log.Fatal(err)
    }
    
    // Clear the cache when done
    parser.ClearCache()
}
```

## API Reference

### Global Functions

These functions use a global parser instance for convenience:

- `ParseBinary(data []byte, schemaPath string, opts ...Option) (map[string]any, error)`
- `ParseBinaryWithContext(ctx context.Context, data []byte, schemaPath string, opts ...Option) (map[string]any, error)`
- `SerializeToJSON(data []byte, schemaPath string, opts ...Option) ([]byte, error)`
- `SerializeToJSONWithContext(ctx context.Context, data []byte, schemaPath string, opts ...Option) ([]byte, error)`
- `SerializeFromJSON(jsonData []byte, schemaPath string, opts ...Option) ([]byte, error)`
- `SerializeFromJSONWithContext(ctx context.Context, jsonData []byte, schemaPath string, opts ...Option) ([]byte, error)`
- `ValidateSchema(schemaPath string) error`

### Parser Type

For more control, create your own parser instance:

```go
type Parser struct {
    // internal fields
}

func NewParser(opts ...Option) *Parser
func (p *Parser) ParseBinary(ctx context.Context, data []byte, schemaPath string, opts ...Option) (map[string]any, error)
func (p *Parser) SerializeToJSON(ctx context.Context, data []byte, schemaPath string, opts ...Option) ([]byte, error)
func (p *Parser) SerializeFromJSON(ctx context.Context, jsonData []byte, schemaPath string, opts ...Option) ([]byte, error)
func (p *Parser) ClearCache()
func (p *Parser) ValidateSchema(schemaPath string) error
```

### Options

Configure parser behavior with these options:

- `WithRootType(rootType string)` - Set the root type to parse (defaults to the schema ID)
- `WithLogger(logger *slog.Logger)` - Set a custom logger
- `WithCaching(timeout time.Duration)` - Enable schema caching with timeout
- `WithImportPaths(paths ...string)` - Add paths to search for imported schemas
- `WithDebugMode(enabled bool)` - Enable debug logging

## Data Types

The parser converts Kaitai types to standard Go types:

- **Integers**: All integer types (u1, u2, u4, u8, s1, s2, s4, s8) are converted to `int64`
- **Floats**: f4 and f8 are converted to `float64`
- **Strings**: String types become Go `string`
- **Bytes**: Byte arrays become `[]byte`
- **Arrays**: Repeated fields become `[]any`
- **Objects**: Complex types become `map[string]any`

## Error Handling

All functions return errors for:
- Invalid schema files
- Parse errors in binary data
- Type conversion errors
- File I/O errors

## Performance Notes

- **Schema Caching**: Schemas are cached by default to improve performance when processing multiple files
- **Context Support**: All parsing operations support context for cancellation and timeouts
- **Memory Efficient**: Streaming parsing minimizes memory usage for large files

## Examples

See `example_test.go` for comprehensive examples including:
- Basic parsing and serialization
- Using custom options
- Context handling with timeouts
- Batch processing multiple files
- Schema validation

## Limitations

- Enum serialization requires special handling (see round-trip tests for current limitations)
- Import paths are handled at the interpreter level, not stored in schema objects
- Some advanced Kaitai features may require direct use of the underlying `kaitaistruct` package