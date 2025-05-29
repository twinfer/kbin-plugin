package kaitaistruct

import (
	"bytes"
	"compress/zlib"
	"context"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKaitaiSuite_ProcessXorConst tests XOR processing with constant value
func TestKaitaiSuite_ProcessXorConst(t *testing.T) {
	yamlContent := `
meta:
  id: process_xor_const
  endian: le
seq:
  - id: key
    type: u1
  - id: buf
    size-eos: true
    process: xor(0xff)
`

	// Original data: key=0xff, buf="foo bar"
	// XOR with 0xff: 'f'^0xff=0x99, 'o'^0xff=0x90, etc.
	data := []byte{0xff, 0x99, 0x90, 0x90, 0xdf, 0x9d, 0x9e, 0x8d}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, int64(255), dataMap["key"])
	assert.Equal(t, []byte("foo bar"), dataMap["buf"])
}

// TestKaitaiSuite_ProcessXorValue tests XOR processing with value from field
func TestKaitaiSuite_ProcessXorValue(t *testing.T) {
	yamlContent := `
meta:
  id: process_xor_value
  endian: le
seq:
  - id: key
    type: u1
  - id: buf
    size-eos: true
    process: xor(key)
`

	// Original data: key=0xff, buf="foo bar"
	// XOR with 0xff: same as above
	data := []byte{0xff, 0x99, 0x90, 0x90, 0xdf, 0x9d, 0x9e, 0x8d}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, int64(255), dataMap["key"])
	assert.Equal(t, []byte("foo bar"), dataMap["buf"])
}

// TestKaitaiSuite_ProcessRotate tests rotate left and right processing
func TestKaitaiSuite_ProcessRotate(t *testing.T) {
	yamlContent := `
meta:
  id: process_rotate
  endian: le
seq:
  - id: buf1
    size: 5
    process: rol(3)
  - id: buf2
    size: 5
    process: ror(3)
  - id: key
    type: u1
  - id: buf3
    size: 5
    process: rol(key)
`

	// Test data from actual process_rotate.bin file
	// After processing: buf1="Hello", buf2="World", key=1, buf3="There"
	data := []byte{0x09, 0xac, 0x8d, 0x8d, 0xed, 0xba, 0x7b, 0x93, 0x63, 0x23, 0x01, 0x2a, 0x34, 0xb2, 0x39, 0xb2}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Verify the expected values match the official test
	assert.Equal(t, []byte{72, 101, 108, 108, 111}, dataMap["buf1"]) // "Hello"
	assert.Equal(t, []byte{87, 111, 114, 108, 100}, dataMap["buf2"]) // "World"
	assert.EqualValues(t, 1, dataMap["key"])
	assert.Equal(t, []byte{84, 104, 101, 114, 101}, dataMap["buf3"]) // "There"
}

// TestKaitaiSuite_ProcessZlib tests zlib decompression
func TestKaitaiSuite_ProcessZlib(t *testing.T) {
	yamlContent := `
meta:
  id: process_zlib
seq:
  - id: compressed
    size-eos: true
    process: zlib
`

	// Create compressed data
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write([]byte("Hello, compressed world!"))
	require.NoError(t, err)
	err = w.Close()
	require.NoError(t, err)

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(buf.Bytes()))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, []byte("Hello, compressed world!"), dataMap["compressed"])
}

// TestKaitaiSuite_EnumBasic tests basic enum functionality
func TestKaitaiSuite_EnumBasic(t *testing.T) {
	yamlContent := `
meta:
  id: enum_basic
  endian: le
seq:
  - id: pet_1
    type: u1
    enum: animal
  - id: pet_2
    type: u1
    enum: animal
enums:
  animal:
    4: cat
    7: dog
    12: chicken
`

	// Test data: pet_1=7 (dog), pet_2=4 (cat)
	data := []byte{0x07, 0x04}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Check enum values (enums return maps with name/value/valid)
	pet1, ok := dataMap["pet_1"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "dog", pet1["name"])
	assert.Equal(t, int64(7), pet1["value"])
	assert.Equal(t, true, pet1["valid"])

	pet2, ok := dataMap["pet_2"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cat", pet2["name"])
	assert.Equal(t, int64(4), pet2["value"])
	assert.Equal(t, true, pet2["valid"])
}

// TestKaitaiSuite_XorProcessRoundTrip tests XOR process function in complete round-trip
func TestKaitaiSuite_XorProcessRoundTrip(t *testing.T) {
	yamlContent := `
meta:
  id: process_xor_const
  endian: le
seq:
  - id: key
    type: u1
  - id: buf
    type: bytes
    size-eos: true
    process: xor(0xff)
`

	// Original data: key=0xff, buf="foo bar" XORed with 0xff
	originalData := []byte{0xff, 0x99, 0x90, 0x90, 0xdf, 0x9d, 0x9e, 0x8d}

	// Parse
	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(originalData))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	parsed, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	parsedMap := ParsedDataToMap(parsed)
	dataMap, ok := parsedMap.(map[string]any)
	require.True(t, ok)

	// Verify parsing worked correctly
	assert.Equal(t, int64(255), dataMap["key"])
	assert.Equal(t, []byte("foo bar"), dataMap["buf"])

	// Serialize back to binary
	serializer, err := NewKaitaiSerializer(schema, nil)
	require.NoError(t, err)

	serializedData, err := serializer.Serialize(context.Background(), dataMap)
	require.NoError(t, err)

	// Verify serialized data matches original
	assert.Equal(t, originalData, serializedData, "Serialized data should match original")

	// Parse again to verify complete round-trip
	stream2 := kaitai.NewStream(bytes.NewReader(serializedData))
	parsed2, err := interpreter.Parse(context.Background(), stream2)
	require.NoError(t, err)

	parsedMap2 := ParsedDataToMap(parsed2)
	dataMap2, ok := parsedMap2.(map[string]any)
	require.True(t, ok)

	// Verify round-trip integrity
	assert.Equal(t, int64(255), dataMap2["key"])
	assert.Equal(t, []byte("foo bar"), dataMap2["buf"])
}

// TestKaitaiSuite_ZlibProcessRoundTrip tests zlib process function in complete round-trip  
func TestKaitaiSuite_ZlibProcessRoundTrip(t *testing.T) {
	yamlContent := `
meta:
  id: process_zlib
seq:
  - id: data
    type: bytes
    size-eos: true
    process: zlib
`

	// Create zlib compressed "test data"
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write([]byte("test data"))
	require.NoError(t, err)
	err = w.Close()
	require.NoError(t, err)
	originalData := buf.Bytes()

	// Parse
	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(originalData))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	parsed, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	parsedMap := ParsedDataToMap(parsed)
	dataMap, ok := parsedMap.(map[string]any)
	require.True(t, ok)

	// Verify parsing worked correctly
	assert.Equal(t, []byte("test data"), dataMap["data"])

	// Serialize back to binary
	serializer, err := NewKaitaiSerializer(schema, nil)
	require.NoError(t, err)

	serializedData, err := serializer.Serialize(context.Background(), dataMap)
	require.NoError(t, err)

	// Verify serialized data matches original
	assert.Equal(t, originalData, serializedData, "Serialized data should match original")

	// Parse again to verify complete round-trip
	stream2 := kaitai.NewStream(bytes.NewReader(serializedData))
	parsed2, err := interpreter.Parse(context.Background(), stream2)
	require.NoError(t, err)

	parsedMap2 := ParsedDataToMap(parsed2)
	dataMap2, ok := parsedMap2.(map[string]any)
	require.True(t, ok)

	// Verify round-trip integrity
	assert.Equal(t, []byte("test data"), dataMap2["data"])
}

// TestKaitaiSuite_BasicRoundTrip tests basic parsing and serialization round-trip
func TestKaitaiSuite_BasicRoundTrip(t *testing.T) {
	yamlContent := `
meta:
  id: basic_roundtrip
  endian: le
seq:
  - id: header
    type: u2le
  - id: data_len
    type: u1
  - id: data
    type: bytes
    size: data_len
  - id: footer
    type: u4le
`

	testData := []byte{
		0x34, 0x12,             // header = 0x1234 (LE)
		0x05,                   // data_len = 5
		0x48, 0x65, 0x6c, 0x6c, 0x6f, // data = "Hello"
		0x78, 0x56, 0x34, 0x12, // footer = 0x12345678 (LE)
	}

	// Parse
	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(testData))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	parsed, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	parsedMap := ParsedDataToMap(parsed)
	dataMap, ok := parsedMap.(map[string]any)
	require.True(t, ok)

	// Verify parsed data
	assert.Equal(t, int64(0x1234), dataMap["header"])
	assert.Equal(t, int64(5), dataMap["data_len"])
	assert.Equal(t, []byte("Hello"), dataMap["data"])
	assert.Equal(t, int64(0x12345678), dataMap["footer"])

	// Serialize back
	serializer, err := NewKaitaiSerializer(schema, nil)
	require.NoError(t, err)

	serializedData, err := serializer.Serialize(context.Background(), dataMap)
	require.NoError(t, err)

	// Verify binary round-trip
	assert.Equal(t, testData, serializedData, "Serialized data should match original")

	// Parse serialized data to verify round-trip integrity
	stream2 := kaitai.NewStream(bytes.NewReader(serializedData))
	parsed2, err := interpreter.Parse(context.Background(), stream2)
	require.NoError(t, err)

	parsedMap2 := ParsedDataToMap(parsed2)
	dataMap2, ok := parsedMap2.(map[string]any)
	require.True(t, ok)

	// Verify complete round-trip
	assert.Equal(t, dataMap["header"], dataMap2["header"])
	assert.Equal(t, dataMap["data_len"], dataMap2["data_len"])
	assert.Equal(t, dataMap["data"], dataMap2["data"])
	assert.Equal(t, dataMap["footer"], dataMap2["footer"])
}

// TestKaitaiSuite_SimpleRoundTrip tests basic round-trip functionality without complex types  
func TestKaitaiSuite_SimpleRoundTrip(t *testing.T) {
	yamlContent := `
meta:
  id: simple_roundtrip
  endian: le
seq:
  - id: count
    type: u1
  - id: data
    type: bytes
    size: count
`

	testData := []byte{0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f} // count=5, data="Hello"

	// Parse
	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(testData))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	parsed, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	parsedMap := ParsedDataToMap(parsed)
	dataMap, ok := parsedMap.(map[string]any)
	require.True(t, ok)

	// Verify parsed data
	assert.Equal(t, int64(5), dataMap["count"])
	assert.Equal(t, []byte("Hello"), dataMap["data"])

	// Serialize back
	serializer, err := NewKaitaiSerializer(schema, nil)
	require.NoError(t, err)

	serializedData, err := serializer.Serialize(context.Background(), dataMap)
	require.NoError(t, err)

	// Verify binary data matches original
	assert.Equal(t, testData, serializedData, "Simple serialization should produce original binary data")

	// Parse again to verify round-trip
	stream2 := kaitai.NewStream(bytes.NewReader(serializedData))
	parsed2, err := interpreter.Parse(context.Background(), stream2)
	require.NoError(t, err)

	parsedMap2 := ParsedDataToMap(parsed2)
	dataMap2, ok := parsedMap2.(map[string]any)
	require.True(t, ok)

	// Verify round-trip integrity
	assert.Equal(t, int64(5), dataMap2["count"])
	assert.Equal(t, []byte("Hello"), dataMap2["data"])
}

// === VALIDATION TESTS ===
// These tests validate that validation rules are correctly enforced during parsing

// TestKaitaiSuite_ValidFailEqInt tests validation failure for exact value match
func TestKaitaiSuite_ValidFailEqInt(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_eq_int
seq:
  - id: foo
    type: u1
    valid: 123  # expects 123 but file contains 0x50 (80)
`

	// Test data: first byte is 0x50 (80), but validation expects 123
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailRangeInt tests validation failure for range check
func TestKaitaiSuite_ValidFailRangeInt(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_range_int
seq:
  - id: foo
    type: u1
    valid:
      min: 5
      max: 10  # expects 5-10 but file contains 0x50 (80)
`

	// Test data: first byte is 0x50 (80), outside range 5-10
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailMinInt tests validation failure for minimum value
func TestKaitaiSuite_ValidFailMinInt(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_min_int
seq:
  - id: foo
    type: u1
    valid:
      min: 123  # expects >= 123 but file contains 0x50 (80)
`

	// Test data: first byte is 0x50 (80), less than min 123
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailMaxInt tests validation failure for maximum value
func TestKaitaiSuite_ValidFailMaxInt(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_max_int
seq:
  - id: foo
    type: u1
    valid:
      max: 12  # expects <= 12 but file contains 0x50 (80)
`

	// Test data: first byte is 0x50 (80), greater than max 12
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailContents tests validation failure for fixed contents
func TestKaitaiSuite_ValidFailContents(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_contents
seq:
  - id: foo
    contents: [0x51, 0x41]  # expects [0x51, 0x41] but file has [0x50, 0x41]
`

	// Test data: [0x50, 0x41] but validation expects [0x51, 0x41]
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailAnyofInt tests validation failure for any-of list
func TestKaitaiSuite_ValidFailAnyofInt(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_anyof_int
seq:
  - id: foo
    type: u1
    valid:
      any-of:
        - 5
        - 6
        - 7
        - 8
        - 10
        - 11
        - 12
        - 47  # expects one of these values but file contains 0x50 (80)
`

	// Test data: first byte is 0x50 (80), not in any-of list
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailExpr tests validation failure for expression-based validation
func TestKaitaiSuite_ValidFailExpr(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_expr
seq:
  - id: foo
    type: u1
    valid:
      expr: _ == 1  # expects foo == 1 but file contains 0x50 (80)
  - id: bar
    type: s2le
    valid:
      expr: _ < -190 or _ > -190  # impossible condition (always false)
`

	// Test data: foo=0x50 (80), bar=-190 as s2le
	data := []byte{0x50, 0x42, 0xFF} // foo=80, bar=-190 (0xFF42 = -190 in s2le)

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailEqBytes tests validation failure for byte array comparison
func TestKaitaiSuite_ValidFailEqBytes(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_eq_bytes
seq:
  - id: foo
    type: bytes
    size: 2
    valid: [0x50, 0x42]  # expects [0x50, 0x42] but file has [0x50, 0x41]
`

	// Test data: [0x50, 0x41] but validation expects [0x50, 0x42]
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailEqStr tests validation failure for string comparison
func TestKaitaiSuite_ValidFailEqStr(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_eq_str
seq:
  - id: foo
    type: str
    size: 2
    encoding: ASCII
    valid: "AB"  # expects "AB" but file contains "PA"
`

	// Test data: "PA" (0x50 0x41) but validation expects "AB"
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailRangeBytes tests validation failure for byte array range
func TestKaitaiSuite_ValidFailRangeBytes(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_range_bytes
seq:
  - id: foo
    type: bytes
    size: 2
    valid:
      min: [0x50, 0x50]
      max: [0x50, 0x50]  # expects exactly [0x50, 0x50] but file has [0x50, 0x41]
`

	// Test data: [0x50, 0x41] but validation expects [0x50, 0x50]
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidFailRangeStr tests validation failure for string range
func TestKaitaiSuite_ValidFailRangeStr(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_range_str
seq:
  - id: foo
    type: str
    size: 2
    encoding: ASCII
    valid:
      min: "AA"
      max: "AZ"  # expects "AA" to "AZ" but file contains "PA"
`

	// Test data: "PA" (0x50 0x41) but validation expects range "AA" to "AZ"
	data := []byte{0x50, 0x41}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}

// TestKaitaiSuite_ValidSuccess tests successful validation
func TestKaitaiSuite_ValidSuccess(t *testing.T) {
	yamlContent := `
meta:
  id: valid_success
seq:
  - id: magic
    contents: [0x50, 0x41]  # expects [0x50, 0x41] - matches file
  - id: value
    type: u1
    valid:
      min: 60
      max: 80  # expects 60-80, file contains 0x43 (67) - valid
  - id: choice
    type: u1
    valid:
      any-of: [1, 2, 3, 4]  # expects one of these, file contains 0x02 (2) - valid
`

	// Test data: [0x50, 0x41] (magic), 0x43 (67, in range 60-80), 0x02 (2, in any-of list)
	data := []byte{0x50, 0x41, 0x43, 0x02}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err, "Validation should succeed")
	require.NotNil(t, result, "Result should not be nil on validation success")

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Verify the parsed values
	assert.Equal(t, []byte{0x50, 0x41}, dataMap["magic"])
	assert.Equal(t, int64(67), dataMap["value"])
	assert.Equal(t, int64(2), dataMap["choice"])
}

// TestKaitaiSuite_ValidFailInEnum tests validation failure for invalid enum value
func TestKaitaiSuite_ValidFailInEnum(t *testing.T) {
	yamlContent := `
meta:
  id: valid_fail_in_enum
seq:
  - id: foo
    type: u1
    enum: animal
    valid:
      in-enum: true  # expects valid enum value but 0x50 (80) is not defined in enum
enums:
  animal:
    4: cat
    7: dog
    12: chicken
`

	// Test data: 0x50 (80) which is not a valid enum value
	data := []byte{0x50}

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	assert.Error(t, err, "Should fail validation for invalid enum")
	assert.Contains(t, err.Error(), "validation failed", "Error should mention validation failure")
	assert.Nil(t, result, "Result should be nil on validation failure")
}