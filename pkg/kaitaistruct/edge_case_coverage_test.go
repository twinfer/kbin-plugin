package kaitaistruct

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
)

// TestEdgeCaseCoverage focuses on edge cases and less common code paths to improve coverage
func TestEdgeCaseCoverage_RepeatUntilConditions(t *testing.T) {
	// Test repeat-until with simple conditions that work reliably
	schema := &KaitaiSchema{
		Meta: Meta{ID: "repeat_until_test"},
		Seq: []SequenceItem{
			{ID: "items", Type: "u1", Repeat: "until", RepeatUntil: "_ == 0xFF"},
			{ID: "terminator", Type: "u1"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	data := []byte{1, 2, 3, 0xFF, 0xAA} // items: [1,2,3,0xFF], terminator: 0xAA
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	items := getEdgeParsedValue(t, parsed, "items")
	if itemArray, ok := items.([]interface{}); ok {
		assert.Len(t, itemArray, 4)
		// Check that 0xFF (termination condition) is included
		if lastItem, ok := itemArray[3].(*ParsedData); ok {
			assert.Equal(t, int64(0xFF), extractEdgeValue(lastItem.Value))
		}
	}

	terminator := getEdgeParsedValue(t, parsed, "terminator")
	assert.Equal(t, int64(0xAA), terminator)
}

func TestEdgeCaseCoverage_IOOperations(t *testing.T) {
	// Test IO operations like _io.pos, _io.size, _io.eof
	schema := &KaitaiSchema{
		Meta: Meta{ID: "io_test"},
		Seq: []SequenceItem{
			{ID: "header", Type: "u2le"},
			{ID: "data", Type: "bytes", Size: 4},
		},
		Instances: map[string]InstanceDef{
			"current_pos": {Value: "_io.pos"},
			"stream_size": {Value: "_io.size"},
			"at_eof": {Value: "_io.eof"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	data := []byte{0x34, 0x12, 0xAA, 0xBB, 0xCC, 0xDD} // header + 4 bytes data
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	header := getEdgeParsedValue(t, parsed, "header")
	dataField := getEdgeParsedValue(t, parsed, "data")
	
	assert.Equal(t, int64(0x1234), header)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC, 0xDD}, dataField)

	// Test IO instances
	currentPos := getEdgeParsedValue(t, parsed, "current_pos")
	streamSize := getEdgeParsedValue(t, parsed, "stream_size")
	atEof := getEdgeParsedValue(t, parsed, "at_eof")

	assert.Equal(t, int64(6), currentPos)   // Should be at end of stream
	assert.Equal(t, int64(6), streamSize)   // Total size of stream
	assert.Equal(t, true, atEof)            // Should be at EOF
}

func TestEdgeCaseCoverage_VariableEncoding(t *testing.T) {
	// Test different string encodings
	schema := &KaitaiSchema{
		Meta: Meta{ID: "encoding_test"},
		Seq: []SequenceItem{
			{ID: "len", Type: "u1"},
			{ID: "utf8_str", Type: "str", Size: "len", Encoding: "UTF-8"},
			{ID: "ascii_str", Type: "str", Size: 5, Encoding: "ASCII"},
		},
		Instances: map[string]InstanceDef{
			"utf8_length": {Value: "utf8_str.length"},
			"ascii_length": {Value: "ascii_str.length"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	utf8Text := "héllo" // 6 bytes in UTF-8
	asciiText := "world"
	data := []byte{6} // length of UTF-8 string in bytes
	data = append(data, []byte(utf8Text)...)
	data = append(data, []byte(asciiText)...)

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	len_field := getEdgeParsedValue(t, parsed, "len")
	utf8Str := getEdgeParsedValue(t, parsed, "utf8_str")
	asciiStr := getEdgeParsedValue(t, parsed, "ascii_str")

	assert.Equal(t, int64(6), len_field)
	assert.Equal(t, utf8Text, utf8Str)
	assert.Equal(t, asciiText, asciiStr)

	// Test instances
	utf8Length := getEdgeParsedValue(t, parsed, "utf8_length")
	asciiLength := getEdgeParsedValue(t, parsed, "ascii_length")

	assert.Equal(t, int64(5), utf8Length)  // Character count
	assert.Equal(t, int64(5), asciiLength) // Character count
}

func TestEdgeCaseCoverage_TerminatedStrings(t *testing.T) {
	// Test terminated strings (strz)
	schema := &KaitaiSchema{
		Meta: Meta{ID: "strz_test"},
		Seq: []SequenceItem{
			{ID: "text1", Type: "strz"},
			{ID: "text2", Type: "strz"},
		},
		Instances: map[string]InstanceDef{
			"combined_length": {Value: "text1.length + text2.length"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	data := []byte{
		'h', 'e', 'l', 'l', 'o', 0, // text1: "hello"
		'w', 'o', 'r', 'l', 'd', 0, // text2: "world"
	}

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	text1 := getEdgeParsedValue(t, parsed, "text1")
	text2 := getEdgeParsedValue(t, parsed, "text2")

	assert.Equal(t, "hello", text1)
	assert.Equal(t, "world", text2)

	// Test instance
	combinedLength := getEdgeParsedValue(t, parsed, "combined_length")
	assert.Equal(t, int64(10), combinedLength) // 5 + 5
}

func TestEdgeCaseCoverage_ProcessingFunctions(t *testing.T) {
	// Test process functions like XOR
	schema := &KaitaiSchema{
		Meta: Meta{ID: "process_test"},
		Seq: []SequenceItem{
			{ID: "len", Type: "u1"},
			{ID: "encrypted_data", Type: "bytes", Size: "len", Process: "xor(0xAA)"},
		},
		Instances: map[string]InstanceDef{
			"data_length": {Value: "encrypted_data.length"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	plaintext := []byte{0x11, 0x22, 0x33} // Original data
	// XOR with 0xAA: [0x11^0xAA, 0x22^0xAA, 0x33^0xAA] = [0xBB, 0x88, 0x99]
	encryptedData := []byte{0xBB, 0x88, 0x99}

	data := []byte{byte(len(encryptedData))}
	data = append(data, encryptedData...)

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	len_field := getEdgeParsedValue(t, parsed, "len")
	decryptedData := getEdgeParsedValue(t, parsed, "encrypted_data")

	assert.Equal(t, int64(3), len_field)
	assert.Equal(t, plaintext, decryptedData) // Should be decrypted back to original

	// Test instance
	dataLength := getEdgeParsedValue(t, parsed, "data_length")
	assert.Equal(t, int64(3), dataLength)
}

func TestEdgeCaseCoverage_ModuloAndDivisionOperations(t *testing.T) {
	// Test modulo and division operations which might have edge cases
	schema := &KaitaiSchema{
		Meta: Meta{ID: "math_test"},
		Seq: []SequenceItem{
			{ID: "dividend", Type: "u1"},
			{ID: "divisor", Type: "u1"},
		},
		Instances: map[string]InstanceDef{
			"quotient": {Value: "dividend / divisor"},
			"remainder": {Value: "dividend % divisor"},
			"is_divisible": {Value: "(dividend % divisor) == 0"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	tests := []struct {
		name      string
		dividend  byte
		divisor   byte
		quotient  int64
		remainder int64
		divisible bool
	}{
		{"exact division", 20, 4, 5, 0, true},
		{"with remainder", 23, 7, 3, 2, false},
		{"dividend smaller", 3, 7, 0, 3, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := []byte{test.dividend, test.divisor}
			stream := kaitai.NewStream(bytes.NewReader(data))
			parsed, err := interp.Parse(context.Background(), stream)
			require.NoError(t, err)

			dividend := getEdgeParsedValue(t, parsed, "dividend")
			divisor := getEdgeParsedValue(t, parsed, "divisor")
			quotient := getEdgeParsedValue(t, parsed, "quotient")
			remainder := getEdgeParsedValue(t, parsed, "remainder")
			isDivisible := getEdgeParsedValue(t, parsed, "is_divisible")

			assert.Equal(t, int64(test.dividend), dividend)
			assert.Equal(t, int64(test.divisor), divisor)
			assert.Equal(t, test.quotient, quotient)
			assert.Equal(t, test.remainder, remainder)
			assert.Equal(t, test.divisible, isDivisible)
		})
	}
}

func TestEdgeCaseCoverage_FloatingPointTypes(t *testing.T) {
	// Test floating point types
	schema := &KaitaiSchema{
		Meta: Meta{ID: "float_test"},
		Seq: []SequenceItem{
			{ID: "float32_val", Type: "f4le"},
			{ID: "float64_val", Type: "f8le"},
		},
		Instances: map[string]InstanceDef{
			"sum": {Value: "float32_val + float64_val"},
			"is_positive": {Value: "float32_val > 0.0"},
		},
	}

	interp := newEdgeTestInterpreter(t, schema)

	// Create binary data for float32 (1.5) and float64 (2.5)
	data := []byte{
		0x00, 0x00, 0xC0, 0x3F, // f4le: 1.5
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x40, // f8le: 2.5
	}

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	float32Val := getEdgeParsedValue(t, parsed, "float32_val")
	float64Val := getEdgeParsedValue(t, parsed, "float64_val")

	assert.InDelta(t, 1.5, float32Val, 0.001)
	assert.InDelta(t, 2.5, float64Val, 0.001)

	// Test instances
	sum := getEdgeParsedValue(t, parsed, "sum")
	isPositive := getEdgeParsedValue(t, parsed, "is_positive")

	assert.InDelta(t, 4.0, sum, 0.001) // 1.5 + 2.5
	assert.Equal(t, true, isPositive)
}

// Helper functions
func newEdgeTestInterpreter(t *testing.T, schema *KaitaiSchema) *KaitaiInterpreter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interp, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)
	return interp
}

func getEdgeParsedValue(t *testing.T, data *ParsedData, key string) interface{} {
	if child, exists := data.Children[key]; exists {
		return extractEdgeValue(child.Value)
	}
	t.Errorf("Key '%s' not found in parsed data", key)
	return nil
}

func extractEdgeValue(value interface{}) interface{} {
	// Extract underlying value from kaitaicel types
	switch v := value.(type) {
	case *kaitaicel.KaitaiInt:
		return v.Value()
	case *kaitaicel.KaitaiString:
		return v.Value()
	case *kaitaicel.KaitaiBytes:
		return v.Value()
	case *kaitaicel.KaitaiBitField:
		return v.AsInt()
	case *kaitaicel.KaitaiFloat:
		return v.Value()
	default:
		return value
	}
}