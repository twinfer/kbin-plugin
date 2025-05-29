package kbin_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/twinfer/kbin-plugin/pkg/kbin"
)

// Example demonstrates basic usage of the kbin package
func Example() {
	// Sample binary data (a simple structure with magic number, version, and length-prefixed string)
	binaryData := []byte{
		0x4B, 0x42, 0x49, 0x4E, // Magic: "KBIN"
		0x01, 0x00,             // Version: 1 (little-endian u2)
		0x05,                   // String length: 5
		0x48, 0x65, 0x6C, 0x6C, 0x6F, // String: "Hello"
	}

	// Assuming we have a schema file at "example.ksy"
	schemaPath := "example.ksy"

	// Parse binary data to a map
	data, err := kbin.ParseBinary(binaryData, schemaPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Parsed data: %+v\n", data)

	// Convert to JSON
	jsonData, err := kbin.SerializeToJSON(binaryData, schemaPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("JSON representation:\n%s\n", jsonData)

	// Convert JSON back to binary
	reconstructed, err := kbin.SerializeFromJSON(jsonData, schemaPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Round-trip successful: %v\n", string(binaryData) == string(reconstructed))
}

// Example_withOptions demonstrates using parser options
func Example_withOptions() {
	// Create a custom logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Binary data
	data := []byte{0x01, 0x02, 0x03, 0x04}

	// Parse with custom options
	result, err := kbin.ParseBinary(data, "schema.ksy",
		kbin.WithLogger(logger),
		kbin.WithRootType("custom_type"),
		kbin.WithDebugMode(true),
		kbin.WithCaching(10*time.Minute),
		kbin.WithImportPaths("/path/to/imports", "/another/path"),
	)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Parsed with options: %+v\n", result)
}

// Example_parser demonstrates using a custom parser instance
func Example_parser() {
	// Create a parser with specific configuration
	parser := kbin.NewParser(
		kbin.WithCaching(30*time.Minute),
		kbin.WithDebugMode(true),
	)

	// Use the parser multiple times with the same configuration
	data1 := []byte{0x01, 0x02, 0x03}
	data2 := []byte{0x04, 0x05, 0x06}

	result1, _ := parser.ParseBinary(context.Background(), data1, "schema.ksy")
	result2, _ := parser.ParseBinary(context.Background(), data2, "schema.ksy")

	fmt.Printf("Result 1: %+v\n", result1)
	fmt.Printf("Result 2: %+v\n", result2)

	// Clear the cache when done
	parser.ClearCache()
}

// Example_withContext demonstrates using context for cancellation
func Example_withContext() {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data := []byte{0x01, 0x02, 0x03, 0x04}

	// Parse with context
	result, err := kbin.ParseBinaryWithContext(ctx, data, "schema.ksy")
	if err != nil {
		// Handle timeout or cancellation
		if ctx.Err() != nil {
			log.Printf("Operation cancelled or timed out: %v", ctx.Err())
		}
		log.Fatal(err)
	}

	fmt.Printf("Parsed before timeout: %+v\n", result)
}

// Example_roundTrip demonstrates a complete round-trip conversion
func Example_roundTrip() {
	// Original binary data
	originalBinary := []byte{
		0x89, 0x50, 0x4E, 0x47, // PNG signature start
		0x0D, 0x0A, 0x1A, 0x0A, // PNG signature end
		// ... more PNG data
	}

	// Step 1: Parse to structured data
	structuredData, err := kbin.ParseBinary(originalBinary, "png.ksy")
	if err != nil {
		log.Fatal(err)
	}

	// Step 2: Convert to JSON for inspection/modification
	jsonData, err := json.MarshalIndent(structuredData, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Structured data as JSON:\n%s\n", jsonData)

	// Step 3: Potentially modify the JSON data here
	// For this example, we'll use it as-is

	// Step 4: Convert back to binary
	reconstructedBinary, err := kbin.SerializeFromJSON(jsonData, "png.ksy")
	if err != nil {
		log.Fatal(err)
	}

	// Verify the round-trip
	if len(originalBinary) == len(reconstructedBinary) {
		match := true
		for i := range originalBinary {
			if originalBinary[i] != reconstructedBinary[i] {
				match = false
				break
			}
		}
		fmt.Printf("Round-trip successful: %v\n", match)
	}
}

// ExampleValidateSchema demonstrates schema validation
func ExampleValidateSchema() {
	// Validate a schema file
	err := kbin.ValidateSchema("my_format.ksy")
	if err != nil {
		log.Printf("Schema validation failed: %v", err)
		return
	}

	fmt.Println("Schema is valid!")
}

// Example_batchProcessing demonstrates processing multiple files
func Example_batchProcessing() {
	// Create a parser instance for batch processing
	parser := kbin.NewParser(
		kbin.WithCaching(1*time.Hour), // Cache schemas for an hour
	)

	// Process multiple files with the same schema
	files := []string{"file1.bin", "file2.bin", "file3.bin"}
	schemaPath := "common_format.ksy"

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Failed to read %s: %v", file, err)
			continue
		}

		result, err := parser.ParseBinary(context.Background(), data, schemaPath)
		if err != nil {
			log.Printf("Failed to parse %s: %v", file, err)
			continue
		}

		// Process the result
		fmt.Printf("Processed %s: found %d fields\n", file, len(result))
	}
}