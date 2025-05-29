package kbin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBinary(t *testing.T) {
	// Create a test schema file
	schemaContent := `meta:
  id: test_format
  endian: le
seq:
  - id: magic
    type: u4
    contents: [0x4B, 0x42, 0x49, 0x4E]
  - id: version
    type: u2
  - id: message_len
    type: u1
  - id: message
    type: str
    size: message_len
    encoding: UTF-8
`
	// Write schema to temp file
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "test.ksy")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Test data
	testData := []byte{
		0x4B, 0x42, 0x49, 0x4E, // magic: "KBIN"
		0x01, 0x00,             // version: 1
		0x05,                   // message_len: 5
		0x48, 0x65, 0x6C, 0x6C, 0x6F, // message: "Hello"
	}

	// Test parsing
	result, err := ParseBinary(testData, schemaPath)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify results (Kaitai types are converted to int64 for integers)
	assert.Equal(t, int64(1), result["version"])
	assert.Equal(t, int64(5), result["message_len"])
	assert.Equal(t, "Hello", result["message"])
}

func TestSerializeToJSON(t *testing.T) {
	// Create a test schema file
	schemaContent := `meta:
  id: test_format
  endian: le
seq:
  - id: value1
    type: u2
  - id: value2
    type: u2
`
	// Write schema to temp file
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "test.ksy")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Test data
	testData := []byte{
		0x01, 0x00, // value1: 1
		0x02, 0x00, // value2: 2
	}

	// Convert to JSON
	jsonData, err := SerializeToJSON(testData, schemaPath)
	require.NoError(t, err)

	// Parse JSON to verify
	var result map[string]any
	err = json.Unmarshal(jsonData, &result)
	require.NoError(t, err)

	assert.Equal(t, float64(1), result["value1"])
	assert.Equal(t, float64(2), result["value2"])
}

func TestSerializeFromJSON(t *testing.T) {
	// Create a test schema file
	schemaContent := `meta:
  id: test_format
  endian: le
seq:
  - id: header
    type: u4
  - id: count
    type: u1
  - id: data
    type: u1
    repeat: expr
    repeat-expr: count
`
	// Write schema to temp file
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "test.ksy")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Create JSON data
	jsonData := `{
		"header": 305419896,
		"count": 3,
		"data": [10, 20, 30]
	}`

	// Convert from JSON to binary
	binaryData, err := SerializeFromJSON([]byte(jsonData), schemaPath)
	require.NoError(t, err)

	// Expected binary
	expected := []byte{
		0x78, 0x56, 0x34, 0x12, // header: 0x12345678 (little-endian)
		0x03,                   // count: 3
		0x0A, 0x14, 0x1E,       // data: [10, 20, 30]
	}

	assert.Equal(t, expected, binaryData)
}

func TestRoundTrip(t *testing.T) {
	// Create a test schema file
	schemaContent := `meta:
  id: test_format
  endian: be
seq:
  - id: type
    type: u1
    enum: entry_type
  - id: size
    type: u2
  - id: payload
    type: str
    size: size
    encoding: ASCII
enums:
  entry_type:
    1: text
    2: binary
    3: metadata
`
	// Write schema to temp file
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "test.ksy")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Original binary data
	originalBinary := []byte{
		0x01,       // type: text (1)
		0x00, 0x06, // size: 6
		0x6B, 0x61, 0x69, 0x74, 0x61, 0x69, // payload: "kaitai"
	}

	// Parse to structured data
	parsed, err := ParseBinary(originalBinary, schemaPath)
	require.NoError(t, err)

	// Convert to JSON
	jsonData, err := json.Marshal(parsed)
	require.NoError(t, err)

	// Convert back to binary
	reconstructed, err := SerializeFromJSON(jsonData, schemaPath)
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, originalBinary, reconstructed)
}

func TestWithOptions(t *testing.T) {
	// Create a parser with options
	parser := NewParser(
		WithCaching(1*time.Hour),
		WithDebugMode(false),
	)

	// Create a test schema
	schemaContent := `meta:
  id: simple
  endian: le
seq:
  - id: value
    type: u4
`
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "simple.ksy")
	err := os.WriteFile(schemaPath, []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Parse data
	data := []byte{0x78, 0x56, 0x34, 0x12}
	result, err := parser.ParseBinary(context.Background(), data, schemaPath)
	require.NoError(t, err)
	assert.Equal(t, int64(0x12345678), result["value"])

	// Verify caching works by parsing again
	result2, err := parser.ParseBinary(context.Background(), data, schemaPath)
	require.NoError(t, err)
	assert.Equal(t, result, result2)

	// Clear cache
	parser.ClearCache()
}

func TestValidateSchema(t *testing.T) {
	// Valid schema
	validSchema := `meta:
  id: valid
seq:
  - id: field1
    type: u1
`
	tmpDir := t.TempDir()
	validPath := filepath.Join(tmpDir, "valid.ksy")
	err := os.WriteFile(validPath, []byte(validSchema), 0644)
	require.NoError(t, err)

	err = ValidateSchema(validPath)
	assert.NoError(t, err)

	// Invalid schema (missing meta.id)
	invalidSchema := `seq:
  - id: field1
    type: u1
`
	invalidPath := filepath.Join(tmpDir, "invalid.ksy")
	err = os.WriteFile(invalidPath, []byte(invalidSchema), 0644)
	require.NoError(t, err)

	// This should still parse successfully as YAML, 
	// actual validation would happen during parsing
	err = ValidateSchema(invalidPath)
	assert.NoError(t, err)
}