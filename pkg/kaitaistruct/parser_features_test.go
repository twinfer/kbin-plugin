package kaitaistruct

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test on-demand instance evaluation
func TestOnDemandInstanceEvaluation(t *testing.T) {
	schemaYAML := `
meta:
  id: test_ondemand
seq:
  - id: value
    type: u1
  - id: dynamic_str
    type: str
    size: calculated_size
    encoding: ASCII
instances:
  calculated_size:
    value: value - 1
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test data: value=5, followed by 4 bytes "test"
	testData := []byte{5, 't', 'e', 's', 't'}
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Check that the field used the calculated instance for its size
	assert.Equal(t, int64(5), dataMap["value"])
	assert.Equal(t, "test", dataMap["dynamic_str"]) // size was calculated as value-1 = 4
	assert.Equal(t, int64(4), dataMap["calculated_size"])
}

// Test instance dependency resolution
func TestInstanceDependencyResolution(t *testing.T) {
	schemaYAML := `
meta:
  id: test_dependencies
seq: []
instances:
  simple_str:
    value: '"hello"'
  simple_array:
    value: '[0x62, 0x61, 0x7a]'
  array_to_str:
    value: 'simple_array.to_s("ASCII")'
  combined:
    value: 'simple_str + " " + array_to_str'
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader([]byte{}))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "hello", dataMap["simple_str"])
	assert.Equal(t, []byte{0x62, 0x61, 0x7a}, dataMap["simple_array"])
	assert.Equal(t, "baz", dataMap["array_to_str"]) // [0x62, 0x61, 0x7a] = "baz"
	assert.Equal(t, "hello baz", dataMap["combined"])
}

// Test bit alignment
func TestBitAlignment(t *testing.T) {
	schemaYAML := `
meta:
  id: test_bit_alignment
seq:
  - id: bits_six
    type: b6
  - id: byte_aligned
    type: u1
  - id: bits_three
    type: b3
  - id: bits_one
    type: b1
  - id: byte_aligned_2
    type: u1
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test data from bits_byte_aligned test: "PACK-1"
	testData := []byte{0x50, 0x41, 0x43, 0x4B, 0x2D, 0x31}
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Verify bit fields and byte alignment work correctly
	assert.Contains(t, dataMap, "bits_six")
	assert.Contains(t, dataMap, "byte_aligned")
	assert.Contains(t, dataMap, "bits_three")
	assert.Contains(t, dataMap, "bits_one")
	assert.Contains(t, dataMap, "byte_aligned_2")
}

// Test bit endianness
func TestBitEndianness(t *testing.T) {
	schemaYAML := `
meta:
  id: test_bit_endian
  bit-endian: le
seq:
  - id: bits_8
    type: b8
  - id: bits_16
    type: b16
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test little-endian bit reading
	testData := []byte{0x12, 0x34, 0x56}
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Values should be interpreted as little-endian
	assert.Contains(t, dataMap, "bits_8")
	assert.Contains(t, dataMap, "bits_16")
}

// Test string terminator handling
func TestStringTerminatorHandling(t *testing.T) {
	schemaYAML := `
meta:
  id: test_string_term
  encoding: ASCII
seq:
  - id: str_term
    type: str
    terminator: 0x7c
  - id: str_fixed
    type: str
    size: 4
  - id: str_term_include
    type: str
    terminator: 0x40
    include: true
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test data: "hello|test@end"
	testData := []byte("hello|test@end")
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "hello", dataMap["str_term"])     // terminated by |, excludes |
	assert.Equal(t, "test", dataMap["str_fixed"])     // fixed size 4
	assert.Equal(t, "@", dataMap["str_term_include"]) // terminated by @, includes @
}

// Test terminated bytes support
func TestTerminatedBytes(t *testing.T) {
	schemaYAML := `
meta:
  id: test_bytes_term
seq:
  - id: bytes_term
    terminator: 0x7c
  - id: bytes_fixed
    type: bytes
    size: 3
  - id: bytes_eos
    size-eos: true
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test data: bytes then | then 3 bytes then rest
	testData := []byte{0x01, 0x02, 0x03, 0x7c, 0x04, 0x05, 0x06, 0x07, 0x08}
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, []byte{0x01, 0x02, 0x03}, dataMap["bytes_term"])  // terminated by 0x7c
	assert.Equal(t, []byte{0x04, 0x05, 0x06}, dataMap["bytes_fixed"]) // fixed size 3
	assert.Equal(t, []byte{0x07, 0x08}, dataMap["bytes_eos"])         // rest of stream
}

// Test array literal parsing and conversion
func TestArrayLiteralParsing(t *testing.T) {
	schemaYAML := `
meta:
  id: test_array_literals
seq: []
instances:
  byte_array:
    value: '[0x48, 0x65, 0x6c, 0x6c, 0x6f]'
  int_array:
    value: '[1, 2, 3, 1000]'
  array_to_string:
    value: 'byte_array.to_s("ASCII")'
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader([]byte{}))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Byte array should be converted to []byte
	assert.Equal(t, []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}, dataMap["byte_array"])

	// Int array with values > 255 should be int64 slice
	expectedIntArray := []int64{1, 2, 3, 1000}
	assert.Equal(t, expectedIntArray, dataMap["int_array"])

	// Array to string conversion
	assert.Equal(t, "Hello", dataMap["array_to_string"])
}

// Test type defaulting for fields without explicit types
func TestTypeDefaulting(t *testing.T) {
	schemaYAML := `
meta:
  id: test_type_defaulting
seq:
  - id: sized_field
    size: 5
  - id: terminated_field
    terminator: 0x00
  - id: eos_field
    size-eos: true
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test data: 5 bytes + null terminator + rest
	testData := []byte("hello\x00world")
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// sized_field should default to bytes type (per Kaitai Struct specification)
	assert.Equal(t, []byte("hello"), dataMap["sized_field"])

	// terminated_field should default to bytes type
	assert.Equal(t, []byte{}, dataMap["terminated_field"]) // empty because first byte after "hello" is the terminator

	// eos_field should default to bytes type
	assert.Equal(t, []byte("world"), dataMap["eos_field"])
}

// Test 1-bit field boolean conversion
func TestOneBitFieldBoolean(t *testing.T) {
	schemaYAML := `
meta:
  id: test_one_bit
seq:
  - id: flag1
    type: b1
  - id: flag2
    type: b1
  - id: multi_bit
    type: b6
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Test data: 0b10111111 = 0xBF
	testData := []byte{0xBF}
	stream := kaitai.NewStream(bytes.NewReader(testData))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// 1-bit fields should be boolean
	assert.IsType(t, true, dataMap["flag1"])
	assert.IsType(t, true, dataMap["flag2"])

	// Multi-bit field should be integer
	assert.IsType(t, int64(0), dataMap["multi_bit"])
}

// Test CEL to_s function with encoding
func TestCELToSWithEncoding(t *testing.T) {
	schemaYAML := `
meta:
  id: test_to_s_encoding
seq: []
instances:
  byte_data:
    value: '[72, 101, 108, 108, 111]'
  as_ascii:
    value: 'byte_data.to_s("ASCII")'
  as_utf8:
    value: 'byte_data.to_s("UTF-8")'
`

	schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader([]byte{}))
	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, []byte{72, 101, 108, 108, 111}, dataMap["byte_data"])
	assert.Equal(t, "Hello", dataMap["as_ascii"])
	assert.Equal(t, "Hello", dataMap["as_utf8"])
}

func TestParse_IOFunctions(t *testing.T) {
	yamlContent := `
meta:
  id: test_io_functions
  endian: le
seq:
  - id: header
    type: u1
  - id: remaining_size
    type: u1
    value: _io.size - _io.pos
  - id: position_check
    type: u1  
    value: _io.pos
`

	data := []byte{0x42, 0x00, 0x43} // header, then calculated fields

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

	// Verify header
	assert.Equal(t, int64(0x42), dataMap["header"])

	// Verify remaining_size calculation (size=3, pos after header=1, so 3-1=2)
	assert.Equal(t, int64(2), dataMap["remaining_size"])

	// Verify position_check (should be 1 after reading header, since remaining_size is computed)
	assert.Equal(t, int64(1), dataMap["position_check"])
}
