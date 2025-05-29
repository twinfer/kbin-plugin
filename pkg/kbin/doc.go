// Package kbin provides a high-level API for parsing and serializing binary data
// using Kaitai Struct format specifications.
//
// # Overview
//
// This package simplifies the use of Kaitai Struct in Go applications by providing
// easy-to-use functions that handle the complexity of schema loading, parsing,
// and serialization. It supports:
//
//   - Binary data parsing to Go maps
//   - JSON serialization and deserialization
//   - Schema caching for performance
//   - Context support for cancellation and timeouts
//   - Flexible configuration options
//
// # Quick Start
//
// The simplest way to parse binary data is using the global functions:
//
//	data := []byte{0x4B, 0x42, 0x49, 0x4E, 0x01, 0x00}
//	result, err := kbin.ParseBinary(data, "format.ksy")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Parsed: %+v\n", result)
//
// # JSON Support
//
// Convert binary data to JSON for inspection or modification:
//
//	jsonData, err := kbin.SerializeToJSON(binaryData, "format.ksy")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	
//	// Modify JSON data as needed...
//	
//	// Convert back to binary
//	newBinary, err := kbin.SerializeFromJSON(jsonData, "format.ksy")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Custom Parser Instance
//
// For more control, create a custom parser with specific options:
//
//	parser := kbin.NewParser(
//	    kbin.WithCaching(1*time.Hour),
//	    kbin.WithDebugMode(true),
//	)
//	
//	result, err := parser.ParseBinary(ctx, data, "schema.ksy")
//
// # Configuration Options
//
// Configure parser behavior using options:
//
//   - WithRootType(string): Specify root type (default: schema ID)
//   - WithLogger(*slog.Logger): Custom logging
//   - WithCaching(time.Duration): Enable schema caching
//   - WithImportPaths(...string): Additional import search paths
//   - WithDebugMode(bool): Enable debug output
//
// # Data Type Conversion
//
// The parser converts Kaitai types to standard Go types:
//
//   - Integers (u1, u2, u4, u8, s1, s2, s4, s8) → int64
//   - Floats (f4, f8) → float64
//   - Strings → string
//   - Bytes → []byte
//   - Arrays → []any
//   - Objects → map[string]any
//
// # Error Handling
//
// All functions return descriptive errors for:
//
//   - Invalid or missing schema files
//   - Binary data parsing failures
//   - Type conversion errors
//   - I/O errors
//
// # Performance Considerations
//
// The package includes several performance optimizations:
//
//   - Schema caching reduces repeated parsing overhead
//   - Streaming parsing minimizes memory usage
//   - Context support enables proper resource management
//   - Connection pooling for CEL expression evaluation
//
// # Thread Safety
//
// The package is thread-safe:
//
//   - Global parser instance uses proper synchronization
//   - Schema cache uses read-write mutexes
//   - Multiple goroutines can safely use the same Parser instance
//
// # Examples
//
// See the package tests and examples for comprehensive usage patterns including:
//
//   - Basic parsing and serialization
//   - Batch processing multiple files
//   - Custom configuration and logging
//   - Context handling and timeouts
//   - Schema validation
//
package kbin