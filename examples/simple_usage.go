// Package main demonstrates how to use the kbin API in a Go program.
//
// This example shows how to import and use the kbin package to parse
// binary data using Kaitai Struct schemas.
package main

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

func main() {
	// Example 1: Simple parsing
	fmt.Println("=== Example 1: Simple Binary Parsing ===")
	simpleParsing()

	// Example 2: Using options
	fmt.Println("\n=== Example 2: Using Parser Options ===")
	parsingWithOptions()

	// Example 3: JSON round-trip
	fmt.Println("\n=== Example 3: JSON Round-Trip ===")
	jsonRoundTrip()

	// Example 4: Custom parser instance
	fmt.Println("\n=== Example 4: Custom Parser Instance ===")
	customParserInstance()

	// Example 5: Error handling
	fmt.Println("\n=== Example 5: Error Handling ===")
	errorHandling()
}

func simpleParsing() {
	// This assumes you have a schema file and binary data
	// For demonstration, we'll show the API calls

	// Sample binary data (would come from file, network, etc.)
	binaryData := []byte{
		0x89, 0x50, 0x4E, 0x47, // PNG signature
		0x0D, 0x0A, 0x1A, 0x0A, // PNG signature continued
		// ... more data would follow
	}

	// Parse using a hypothetical PNG schema
	// Note: You would need an actual PNG.ksy file for this to work
	schemaPath := "formats/png.ksy"

	// This is how you would parse the data:
	result, err := kbin.ParseBinary(binaryData, schemaPath)
	if err != nil {
		fmt.Printf("Parse error (expected): %v\n", err)
		return
	}

	fmt.Printf("Parsed PNG header: %+v\n", result)
}

func parsingWithOptions() {
	// Create a custom logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Sample data
	data := []byte{0x01, 0x02, 0x03, 0x04}

	// Parse with options
	result, err := kbin.ParseBinary(data, "formats/simple.ksy",
		kbin.WithLogger(logger),
		kbin.WithRootType("custom_root"),
		kbin.WithDebugMode(true),
		kbin.WithCaching(10*time.Minute),
		kbin.WithImportPaths("./imports", "/usr/local/kaitai/imports"),
	)

	if err != nil {
		fmt.Printf("Parse error (expected): %v\n", err)
		return
	}

	fmt.Printf("Parsed with options: %+v\n", result)
}

func jsonRoundTrip() {
	// Sample binary data
	data := []byte{0x12, 0x34, 0x56, 0x78}

	// Convert to JSON
	jsonData, err := kbin.SerializeToJSON(data, "formats/integers.ksy")
	if err != nil {
		fmt.Printf("JSON serialization error (expected): %v\n", err)
		return
	}

	fmt.Printf("JSON representation: %s\n", jsonData)

	// Modify the JSON (example)
	var parsed map[string]any
	if err := json.Unmarshal(jsonData, &parsed); err == nil {
		// Modify some field
		parsed["some_field"] = "modified_value"

		// Convert back to JSON
		modifiedJSON, _ := json.Marshal(parsed)

		// Convert back to binary
		newBinary, err := kbin.SerializeFromJSON(modifiedJSON, "formats/integers.ksy")
		if err != nil {
			fmt.Printf("Binary serialization error: %v\n", err)
			return
		}

		fmt.Printf("New binary data: %x\n", newBinary)
	}
}

func customParserInstance() {
	// Create a parser with specific configuration
	parser := kbin.NewParser(
		kbin.WithCaching(1*time.Hour),    // Cache schemas for 1 hour
		kbin.WithDebugMode(false),        // Disable debug output
	)

	// Use the parser for multiple operations
	files := []string{
		"data1.bin",
		"data2.bin",
		"data3.bin",
	}

	for _, filename := range files {
		// In a real application, you'd read the file
		data := []byte{0x01, 0x02, 0x03} // Placeholder data

		result, err := parser.ParseBinary(
			context.Background(),
			data,
			"formats/common.ksy",
		)

		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", filename, err)
			continue
		}

		fmt.Printf("Parsed %s: %d fields\n", filename, len(result))
	}

	// Clear cache when done (optional, for memory management)
	parser.ClearCache()
}

func errorHandling() {
	// Demonstrate various error conditions

	// 1. Missing schema file
	_, err := kbin.ParseBinary([]byte{0x01}, "nonexistent.ksy")
	if err != nil {
		fmt.Printf("Missing schema error: %v\n", err)
	}

	// 2. Invalid binary data
	data := []byte{0x01} // Too short for most formats
	_, err = kbin.ParseBinary(data, "formats/complex.ksy")
	if err != nil {
		fmt.Printf("Invalid data error: %v\n", err)
	}

	// 3. Context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	time.Sleep(2 * time.Microsecond) // Ensure timeout

	_, err = kbin.ParseBinaryWithContext(ctx, []byte{0x01, 0x02}, "formats/simple.ksy")
	if err != nil {
		fmt.Printf("Timeout error: %v\n", err)
	}

	// 4. Schema validation
	err = kbin.ValidateSchema("formats/invalid.ksy")
	if err != nil {
		fmt.Printf("Schema validation error: %v\n", err)
	}
}

// Example of how you might integrate kbin into a larger application
func integrateIntoApp() {
	// Example: HTTP handler that parses uploaded binary files
	/*
	http.HandleFunc("/parse", func(w http.ResponseWriter, r *http.Request) {
		// Read uploaded file
		file, _, err := r.FormFile("binary_data")
		if err != nil {
			http.Error(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "Could not read file", http.StatusInternalServerError)
			return
		}

		// Get schema path from form or URL parameter
		schemaPath := r.FormValue("schema")
		if schemaPath == "" {
			http.Error(w, "Schema path required", http.StatusBadRequest)
			return
		}

		// Parse the binary data
		result, err := kbin.ParseBinary(data, schemaPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Parse error: %v", err), http.StatusBadRequest)
			return
		}

		// Return as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
	*/

	// Example: CLI tool for binary file analysis
	/*
	func main() {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s <binary_file> <schema_file>\n", os.Args[0])
			os.Exit(1)
		}

		binaryFile := os.Args[1]
		schemaFile := os.Args[2]

		data, err := os.ReadFile(binaryFile)
		if err != nil {
			log.Fatalf("Could not read binary file: %v", err)
		}

		result, err := kbin.ParseBinary(data, schemaFile)
		if err != nil {
			log.Fatalf("Parse error: %v", err)
		}

		// Pretty-print the result
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonData))
	}
	*/
}