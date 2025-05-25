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
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
)

func newTestInterpreterWithKaitaicel(t *testing.T, schema *KaitaiSchema) *KaitaiInterpreter {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	interp, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)
	return interp
}

// Test that all kaitaicel types are properly created and returned
func TestKaitaicelIntegration_PrimitiveTypes(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "kaitaicel_primitives", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "u1_val", Type: "u1"},
			{ID: "u2le_val", Type: "u2le"},
			{ID: "u2be_val", Type: "u2be"},
			{ID: "u4le_val", Type: "u4le"},
			{ID: "u4be_val", Type: "u4be"},
			{ID: "u8le_val", Type: "u8le"},
			{ID: "u8be_val", Type: "u8be"},
			{ID: "s1_val", Type: "s1"},
			{ID: "s2le_val", Type: "s2le"},
			{ID: "s2be_val", Type: "s2be"},
			{ID: "s4le_val", Type: "s4le"},
			{ID: "s4be_val", Type: "s4be"},
			{ID: "s8le_val", Type: "s8le"},
			{ID: "s8be_val", Type: "s8be"},
			{ID: "f4le_val", Type: "f4le"},
			{ID: "f4be_val", Type: "f4be"},
			{ID: "f8le_val", Type: "f8le"},
			{ID: "f8be_val", Type: "f8be"},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	// Create test data with specific values
	data := []byte{
		0xA1,                                           // u1: 161
		0x34, 0x12,                                     // u2le: 0x1234
		0x56, 0x78,                                     // u2be: 0x5678
		0x78, 0x56, 0x34, 0x12,                         // u4le: 0x12345678
		0x9A, 0xBC, 0xDE, 0xF0,                         // u4be: 0x9ABCDEF0
		0x78, 0x56, 0x34, 0x12, 0xF0, 0xDE, 0xBC, 0x9A, // u8le: 0x9ABCDEF012345678
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, // u8be: 0x1122334455667788
		0xFF,                                           // s1: -1
		0xFF, 0xFF,                                     // s2le: -1
		0xFF, 0xFF,                                     // s2be: -1
		0xFF, 0xFF, 0xFF, 0xFF,                         // s4le: -1
		0xFF, 0xFF, 0xFF, 0xFF,                         // s4be: -1
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // s8le: -1
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // s8be: -1
		0x00, 0x00, 0x80, 0x3F,                         // f4le: 1.0
		0x3F, 0x80, 0x00, 0x00,                         // f4be: 1.0
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // f8le: 1.0
		0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // f8be: 1.0
	}
	
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	// Test that values are correctly parsed as kaitaicel types
	tests := []struct {
		field    string
		expected interface{}
		typeCheck func(interface{}) bool
	}{
		{"u1_val", int64(161), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"u2le_val", int64(0x1234), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"u2be_val", int64(0x5678), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"u4le_val", int64(0x12345678), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"u4be_val", int64(0x9ABCDEF0), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s1_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s2le_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s2be_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s4le_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s4be_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s8le_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"s8be_val", int64(-1), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiInt); return ok }},
		{"f4le_val", float64(1.0), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiFloat); return ok }},
		{"f4be_val", float64(1.0), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiFloat); return ok }},
		{"f8le_val", float64(1.0), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiFloat); return ok }},
		{"f8be_val", float64(1.0), func(v interface{}) bool { _, ok := v.(*kaitaicel.KaitaiFloat); return ok }},
	}
	
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			value := getParsedValue(t, parsed, test.field)
			
			// Check that it's the correct kaitaicel type
			assert.True(t, test.typeCheck(value), "Field %s should be a kaitaicel type", test.field)
			
			// Check that the underlying value is correct
			if kaitaiType, ok := value.(kaitaicel.KaitaiType); ok {
				assert.Equal(t, test.expected, kaitaiType.Value(), "Field %s should have correct value", test.field)
			}
		})
	}
}

func TestKaitaicelIntegration_BitFields(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "bitfield_test", Endian: "be"},
		Seq: []SequenceItem{
			{ID: "bit1", Type: "b1"},
			{ID: "bit3", Type: "b3"},
			{ID: "bit4", Type: "b4"},
			{ID: "bit8le", Type: "b8le"},
			{ID: "bit12be", Type: "b12be"},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	// Note: This is a simplified test - actual bit field parsing depends on
	// how the bit stream is handled by the Kaitai runtime
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	stream := kaitai.NewStream(bytes.NewReader(data))
	
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	// Test that bit fields are created as KaitaiBitField types
	bitFields := []string{"bit1", "bit3", "bit4", "bit8le", "bit12be"}
	for _, field := range bitFields {
		t.Run(field, func(t *testing.T) {
			value := getParsedValue(t, parsed, field)
			bitField, ok := value.(*kaitaicel.KaitaiBitField)
			assert.True(t, ok, "Field %s should be a KaitaiBitField", field)
			assert.NotNil(t, bitField)
		})
	}
}

func TestKaitaicelIntegration_StringTypes(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "string_test", Encoding: "UTF-8"},
		Seq: []SequenceItem{
			{ID: "fixed_str", Type: "str", Size: 5},
			{ID: "term_str", Type: "strz"},
			{ID: "encoded_str", Type: "str", Size: 4, Encoding: "ASCII"},
			{ID: "eos_str", Type: "str", SizeEOS: true},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	data := []byte{
		'h', 'e', 'l', 'l', 'o',    // fixed_str: "hello"
		'w', 'o', 'r', 'l', 'd', 0, // term_str: "world"
		't', 'e', 's', 't',         // encoded_str: "test"
		'e', 'n', 'd',              // eos_str: "end"
	}
	
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	tests := []struct {
		field    string
		expected string
		encoding string
	}{
		{"fixed_str", "hello", "UTF-8"},
		{"term_str", "world", "UTF-8"},
		{"encoded_str", "test", "ASCII"},
		{"eos_str", "end", "UTF-8"},
	}
	
	for _, test := range tests {
		t.Run(test.field, func(t *testing.T) {
			value := getParsedValue(t, parsed, test.field)
			
			// Check that it's a KaitaiString
			kaitaiStr, ok := value.(*kaitaicel.KaitaiString)
			assert.True(t, ok, "Field %s should be a KaitaiString", test.field)
			
			// Check the string value
			assert.Equal(t, test.expected, kaitaiStr.Value(), "Field %s should have correct string value", test.field)
			
			// Check encoding methods
			assert.Equal(t, len(test.expected), kaitaiStr.Length(), "Field %s should have correct length", test.field)
			assert.True(t, kaitaiStr.ByteSize() >= len(test.expected), "Field %s should have correct byte size", test.field)
		})
	}
}

func TestKaitaicelIntegration_BytesTypes(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "bytes_test"},
		Seq: []SequenceItem{
			{ID: "len", Type: "u1"},
			{ID: "data", Type: "bytes", Size: "len"},
			{ID: "rest", Type: "bytes", SizeEOS: true},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	data := []byte{
		0x03,                   // len: 3
		0xAA, 0xBB, 0xCC,       // data: [AA, BB, CC]
		0xDD, 0xEE, 0xFF,       // rest: [DD, EE, FF]
	}
	
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	// Check len field
	lenValue := getParsedValue(t, parsed, "len")
	lenInt, ok := lenValue.(*kaitaicel.KaitaiInt)
	assert.True(t, ok)
	assert.Equal(t, int64(3), lenInt.Value())
	
	// Check data field
	dataValue := getParsedValue(t, parsed, "data")
	dataBytes, ok := dataValue.(*kaitaicel.KaitaiBytes)
	assert.True(t, ok, "data field should be KaitaiBytes")
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, dataBytes.Value())
	assert.Equal(t, 3, dataBytes.Length())
	
	// Check rest field
	restValue := getParsedValue(t, parsed, "rest")
	restBytes, ok := restValue.(*kaitaicel.KaitaiBytes)
	assert.True(t, ok, "rest field should be KaitaiBytes")
	assert.Equal(t, []byte{0xDD, 0xEE, 0xFF}, restBytes.Value())
	assert.Equal(t, 3, restBytes.Length())
}

func TestKaitaicelIntegration_TypeConversionInSerialization(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "serialization_test"},
		Seq: []SequenceItem{
			{ID: "int_val", Type: "u2le"},
			{ID: "float_val", Type: "f4le"},
			{ID: "str_val", Type: "str", Size: 4},
			{ID: "bytes_val", Type: "bytes", Size: 3},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	data := []byte{
		0x34, 0x12,             // int_val: 0x1234
		0x00, 0x00, 0x80, 0x3F, // float_val: 1.0
		't', 'e', 's', 't',     // str_val: "test"
		0xAA, 0xBB, 0xCC,       // bytes_val: [AA, BB, CC]
	}
	
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	// Convert to map for serialization
	serialized := ParsedDataToMap(parsed)
	serializedMap, ok := serialized.(map[string]interface{})
	require.True(t, ok)
	
	// Check that kaitaicel types are properly converted for serialization (note: field names become PascalCase)
	assert.Equal(t, int64(0x1234), serializedMap["IntVal"], "Integer should be converted to underlying value")
	assert.Equal(t, float64(1.0), serializedMap["FloatVal"], "Float should be converted to underlying value")
	assert.Equal(t, "test", serializedMap["StrVal"], "String should be converted to underlying value")
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, serializedMap["BytesVal"], "Bytes should be converted to underlying value")
}

func TestKaitaicelIntegration_CELExpressionsWithKaitaiTypes(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "cel_test"},
		Seq: []SequenceItem{
			{ID: "len", Type: "u1"},
			{ID: "data", Type: "str", Size: "len"},
		},
		Instances: map[string]InstanceDef{
			"data_length": {Value: "len"},
			"is_short":    {Value: "len < 5"},
			"doubled":     {Value: "len * 2"},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	data := []byte{
		0x04,                   // len: 4
		'h', 'i', '!', '!',     // data: "hi!!"
	}
	
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	// Check that CEL expressions work with kaitaicel types
	assert.EqualValues(t, 4, getParsedValue(t, parsed, "data_length"))
	assert.Equal(t, true, getParsedValue(t, parsed, "is_short"))
	assert.EqualValues(t, 8, getParsedValue(t, parsed, "doubled"))
}

func TestKaitaicelIntegration_EndianHandling(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "endian_test", Endian: "be"}, // Default big-endian
		Seq: []SequenceItem{
			{ID: "default_u2", Type: "u2"},     // Should use meta endian (be)
			{ID: "explicit_le", Type: "u2le"},  // Explicit little-endian
			{ID: "explicit_be", Type: "u2be"},  // Explicit big-endian
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	data := []byte{
		0x12, 0x34, // default_u2: 0x1234 (be)
		0x56, 0x78, // explicit_le: 0x7856 (le)
		0x9A, 0xBC, // explicit_be: 0x9ABC (be)
	}
	
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	// Check that endianness is handled correctly
	defaultVal := getParsedValue(t, parsed, "default_u2").(*kaitaicel.KaitaiInt)
	assert.Equal(t, int64(0x1234), defaultVal.Value())
	
	leVal := getParsedValue(t, parsed, "explicit_le").(*kaitaicel.KaitaiInt)
	assert.Equal(t, int64(0x7856), leVal.Value())
	
	beVal := getParsedValue(t, parsed, "explicit_be").(*kaitaicel.KaitaiInt)
	assert.Equal(t, int64(0x9ABC), beVal.Value())
}

func TestKaitaicelIntegration_RawBytesAccess(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "raw_bytes_test"},
		Seq: []SequenceItem{
			{ID: "value", Type: "u4le"},
		},
	}
	
	interp := newTestInterpreterWithKaitaicel(t, schema)
	
	data := []byte{0x78, 0x56, 0x34, 0x12} // u4le: 0x12345678
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	
	value := getParsedValue(t, parsed, "value")
	kaitaiInt, ok := value.(*kaitaicel.KaitaiInt)
	require.True(t, ok)
	
	// Check that we can access raw bytes
	rawBytes := kaitaiInt.RawBytes()
	assert.Equal(t, []byte{0x78, 0x56, 0x34, 0x12}, rawBytes)
	
	// Check type name
	assert.Equal(t, "u4", kaitaiInt.KaitaiTypeName())
}

func TestKaitaicelIntegration_ErrorHandling(t *testing.T) {
	t.Run("invalid encoding", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "error_test"},
			Seq: []SequenceItem{
				{ID: "bad_str", Type: "str", Size: 4, Encoding: "INVALID_ENCODING"},
			},
		}
		
		interp := newTestInterpreterWithKaitaicel(t, schema)
		data := []byte{'t', 'e', 's', 't'}
		stream := kaitai.NewStream(bytes.NewReader(data))
		
		_, err := interp.Parse(context.Background(), stream)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encoding")
	})
	
	t.Run("bit field out of range", func(t *testing.T) {
		// This would require internal bit field creation to fail
		// The actual test depends on how bit fields are validated
		schema := &KaitaiSchema{
			Meta: Meta{ID: "bitfield_error_test"},
			Seq: []SequenceItem{
				{ID: "normal_bit", Type: "b8"},
			},
		}
		
		interp := newTestInterpreterWithKaitaicel(t, schema)
		data := []byte{0xFF}
		stream := kaitai.NewStream(bytes.NewReader(data))
		
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err) // Normal bit field should work
		
		value := getParsedValue(t, parsed, "normal_bit")
		bitField, ok := value.(*kaitaicel.KaitaiBitField)
		assert.True(t, ok)
		assert.Equal(t, 8, bitField.BitCount())
	})
}

// Benchmark to ensure kaitaicel integration doesn't significantly impact performance
func BenchmarkKaitaicelIntegration_SimpleTypes(b *testing.B) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "benchmark_test"},
		Seq: []SequenceItem{
			{ID: "u1_val", Type: "u1"},
			{ID: "u2le_val", Type: "u2le"},
			{ID: "u4le_val", Type: "u4le"},
			{ID: "str_val", Type: "str", Size: 4},
		},
	}
	
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	interp, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(b, err)
	
	data := []byte{
		0xFF,                   // u1_val
		0x34, 0x12,             // u2le_val
		0x78, 0x56, 0x34, 0x12, // u4le_val
		't', 'e', 's', 't',     // str_val
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := kaitai.NewStream(bytes.NewReader(data))
		_, err := interp.Parse(context.Background(), stream)
		if err != nil {
			b.Fatal(err)
		}
	}
}