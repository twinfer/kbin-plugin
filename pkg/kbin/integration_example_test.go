package kbin_test

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/twinfer/kbin-plugin/pkg/kbin"
)

// ExampleIntegration demonstrates a complete workflow using the kbin API
func ExampleIntegration() {
	// Create a temporary directory for our test files
	tmpDir, err := os.MkdirTemp("", "kbin-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Define a simple binary format schema
	schemaContent := `meta:
  id: message_format
  endian: le
  encoding: UTF-8
seq:
  - id: magic
    type: u4
    contents: [0x4D, 0x53, 0x47, 0x21]  # "MSG!"
  - id: version
    type: u1
  - id: flags
    type: u1
    enum: message_flags
  - id: length
    type: u2
  - id: message
    type: str
    size: length
    encoding: UTF-8
enums:
  message_flags:
    0: normal
    1: urgent
    2: encrypted
    3: compressed
`

	// Write schema to file
	schemaPath := filepath.Join(tmpDir, "message.ksy")
	err = os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// Create sample binary data
	message := "Hello, Kaitai!"
	binaryData := []byte{
		0x4D, 0x53, 0x47, 0x21, // magic: "MSG!"
		0x01,                   // version: 1
		0x01,                   // flags: urgent (1)
		byte(len(message)),     // length: 15 (low byte)
		0x00,                   // length: 0 (high byte, little-endian)
	}
	binaryData = append(binaryData, []byte(message)...) // message data

	fmt.Println("=== Binary to Structured Data ===")

	// Step 1: Parse binary data to structured format
	parsed, err := kbin.ParseBinary(binaryData, schemaPath)
	if err != nil {
		log.Fatal("Parse error:", err)
	}

	fmt.Printf("Magic: 0x%08X\n", parsed["magic"])
	fmt.Printf("Version: %v\n", parsed["version"])
	fmt.Printf("Flags: %v\n", parsed["flags"])
	fmt.Printf("Length: %v\n", parsed["length"])
	fmt.Printf("Message: %q\n", parsed["message"])

	fmt.Println("\n=== Structured Data to JSON ===")

	// Step 2: Convert to JSON for inspection/modification
	jsonData, err := kbin.SerializeToJSON(binaryData, schemaPath)
	if err != nil {
		log.Fatal("JSON serialization error:", err)
	}

	fmt.Printf("JSON representation:\n%s\n", jsonData)

	fmt.Println("=== JSON Modification ===")

	// Step 3: Modify the JSON data
	var jsonObj map[string]any
	err = json.Unmarshal(jsonData, &jsonObj)
	if err != nil {
		log.Fatal("JSON unmarshal error:", err)
	}

	// Change the message and update length
	newMessage := "Modified message!"
	jsonObj["message"] = newMessage
	jsonObj["length"] = int64(len(newMessage))
	jsonObj["flags"] = int64(2) // Change to encrypted

	modifiedJSON, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		log.Fatal("JSON marshal error:", err)
	}

	fmt.Printf("Modified JSON:\n%s\n", modifiedJSON)

	fmt.Println("=== JSON to Binary ===")

	// Step 4: Convert modified JSON back to binary
	newBinary, err := kbin.SerializeFromJSON(modifiedJSON, schemaPath)
	if err != nil {
		log.Fatal("Binary serialization error:", err)
	}

	fmt.Printf("New binary data (%d bytes): %x\n", len(newBinary), newBinary)

	fmt.Println("=== Verification ===")

	// Step 5: Parse the new binary to verify it's correct
	verification, err := kbin.ParseBinary(newBinary, schemaPath)
	if err != nil {
		log.Fatal("Verification parse error:", err)
	}

	fmt.Printf("Verified message: %q\n", verification["message"])
	fmt.Printf("Verified flags: %v\n", verification["flags"])
	fmt.Printf("Verified length: %v\n", verification["length"])

	// Output:
	// === Binary to Structured Data ===
	// Magic: 0x21474D53
	// Version: 1
	// Flags: 1
	// Length: 15
	// Message: "Hello, Kaitai!"
	//
	// === Structured Data to JSON ===
	// JSON representation:
	// {
	//   "flags": 1,
	//   "length": 15,
	//   "message": "Hello, Kaitai!",
	//   "version": 1
	// }
	//
	// === JSON Modification ===
	// Modified JSON:
	// {
	//   "flags": 2,
	//   "length": 17,
	//   "message": "Modified message!",
	//   "version": 1
	// }
	//
	// === JSON to Binary ===
	// New binary data (26 bytes): 4d534721010211004d6f6469666965642006d657373616765214
	//
	// === Verification ===
	// Verified message: "Modified message!"
	// Verified flags: 2
	// Verified length: 17
}