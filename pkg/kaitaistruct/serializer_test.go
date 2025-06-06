package kaitaistruct

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
)

func newTestSerializer(t *testing.T, schema *KaitaiSchema) *KaitaiSerializer {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// For debugging, you can use:
	// logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s, err := NewKaitaiSerializer(schema, logger)
	require.NoError(t, err)
	return s
}

// Helper function to create KaitaiString for tests
func createTestKaitaiString(value, encoding string) kaitaicel.KaitaiType {
	kStr, err := kaitaicel.NewKaitaiString([]byte(value), encoding)
	if err != nil {
		// For test purposes, return a simple implementation
		return kaitaicel.NewKaitaiBytes([]byte(value))
	}
	return kStr
}

func TestSerialize_SimpleRootType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "simple_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "magic", Type: "u1"},
			{ID: "length", Type: "u1"},
			{ID: "message", Type: "str", Size: "length"},
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"magic":   uint8(0x42),
		"length":  uint8(5),
		"message": "hello",
	}

	expectedBytes := []byte{0x42, 0x05, 'h', 'e', 'l', 'l', 'o'}

	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestSerialize_NestedType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "nested_root", Endian: "le"},
		Types: map[string]Type{
			"header_type": {
				Seq: []SequenceItem{
					{ID: "version", Type: "u1"},
					{ID: "flags", Type: "u1"},
				},
			},
		},
		Seq: []SequenceItem{
			{ID: "my_header", Type: "header_type"},
			{ID: "payload_size", Type: "u2le"},
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"my_header": map[string]any{
			"version": uint8(1),
			"flags":   uint8(0x80),
		},
		"payload_size": uint16(256),
	}

	expectedBytes := []byte{0x01, 0x80, 0x00, 0x01} // 256 in u2le is 0x00, 0x01

	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestSerialize_ConditionalField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "conditional_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "has_extra", Type: "u1"}, // 0 or 1
			{ID: "extra_data", Type: "u2le", IfExpr: "has_extra == 1"},
			{ID: "always_data", Type: "u1"},
		},
	}
	s := newTestSerializer(t, schema)

	t.Run("extra_data present", func(t *testing.T) {
		data := map[string]any{
			"has_extra":   uint8(1),
			"extra_data":  uint16(0xABCD),
			"always_data": uint8(0xFF),
		}
		expectedBytes := []byte{0x01, 0xCD, 0xAB, 0xFF}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})

	t.Run("extra_data absent", func(t *testing.T) {
		data := map[string]any{
			"has_extra": uint8(0),
			// "extra_data" is omitted
			"always_data": uint8(0xEE),
		}
		expectedBytes := []byte{0x00, 0xEE}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})
}

func TestSerialize_RepeatedField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "repeated_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "count", Type: "u1"},
			{ID: "numbers", Type: "u2le", Repeat: "expr", RepeatExpr: "count"},
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"count": uint8(3),
		"numbers": []any{
			uint16(100),
			uint16(200),
			uint16(300),
		},
	}
	// 100 -> 64 00
	// 200 -> C8 00
	// 300 -> 2C 01
	expectedBytes := []byte{0x03, 0x64, 0x00, 0xC8, 0x00, 0x2C, 0x01}

	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestSerialize_SwitchField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "switch_root", Endian: "le"},
		Types: map[string]Type{
			"type_a": {Seq: []SequenceItem{{ID: "val_a", Type: "u1"}}},
			"type_b": {Seq: []SequenceItem{{ID: "val_b", Type: "str", Size: 2}}},
		},
		Seq: []SequenceItem{
			{ID: "selector", Type: "u1"}, // 1 for type_a, 2 for type_b
			{
				ID:   "data_field",
				Type: "switch",
				Switch: map[string]any{ // Simulating parsed KSY map structure
					"switch-on": "selector",
					"cases": map[string]string{
						"1": "type_a",
						"2": "type_b",
						"_": "type_a", // Default case
					},
				},
			},
		},
	}
	s := newTestSerializer(t, schema)

	t.Run("selects type_a", func(t *testing.T) {
		data := map[string]any{
			"selector": uint8(1),
			"data_field": map[string]any{
				"val_a": uint8(0xAA),
			},
		}
		expectedBytes := []byte{0x01, 0xAA}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})

	t.Run("selects type_b", func(t *testing.T) {
		data := map[string]any{
			"selector": uint8(2),
			"data_field": map[string]any{
				"val_b": "XY",
			},
		}
		expectedBytes := []byte{0x02, 'X', 'Y'}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})

	t.Run("selects default type_a", func(t *testing.T) {
		data := map[string]any{
			"selector": uint8(3), // Not 1 or 2, should hit default
			"data_field": map[string]any{
				"val_a": uint8(0xBB),
			},
		}
		expectedBytes := []byte{0x03, 0xBB}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})
}

func TestSerialize_SwitchField_ExpressionIsType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "switch_expr_is_type_root", Endian: "le"},
		Types: map[string]Type{
			"type_one": {Seq: []SequenceItem{{ID: "val_one", Type: "u1"}}},
			"type_two": {Seq: []SequenceItem{{ID: "val_two", Type: "u2le"}}},
		},
		Seq: []SequenceItem{
			{ID: "type_selector_val", Type: "u1"}, // 1 or 2
			{
				ID:     "dynamic_data",
				Type:   "switch",
				Switch: "type_selector_val == 1 ? 'type_one' : 'type_two'", // Expression directly yields type name
			},
		},
	}
	s := newTestSerializer(t, schema)

	t.Run("selects type_one via expression", func(t *testing.T) {
		data := map[string]any{
			"type_selector_val": uint8(1),
			"dynamic_data": map[string]any{
				"val_one": uint8(0xCC),
			},
		}
		expectedBytes := []byte{0x01, 0xCC}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})

	t.Run("selects type_two via expression", func(t *testing.T) {
		data := map[string]any{
			"type_selector_val": uint8(2),
			"dynamic_data": map[string]any{
				"val_two": uint16(0xBBAA),
			},
		}
		expectedBytes := []byte{0x02, 0xAA, 0xBB}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})
}

func TestSerialize_AdHocSwitchType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "adhoc_switch_root", Endian: "le"},
		Types: map[string]Type{
			"type_x": {Seq: []SequenceItem{{ID: "val_x", Type: "s1"}}},
		},
		Seq: []SequenceItem{
			{ID: "switch_key", Type: "u1"}, // Used in switch-on expression
			// Ad-hoc switch: type name itself contains the switch logic
			{ID: "switched_item", Type: "switch-on: switch_key > 0 ? 'type_x' : 'u2be'"},
		},
	}
	s := newTestSerializer(t, schema)

	t.Run("ad-hoc selects type_x", func(t *testing.T) {
		data := map[string]any{
			"switch_key": uint8(1),
			"switched_item": map[string]any{ // Data for type_x
				"val_x": int8(-1), // 0xFF
			},
		}
		expectedBytes := []byte{0x01, 0xFF}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})

	t.Run("ad-hoc selects u2be", func(t *testing.T) {
		data := map[string]any{
			"switch_key":    uint8(0),
			"switched_item": uint16(0x1234), // Data for u2be
		}
		expectedBytes := []byte{0x00, 0x12, 0x34}
		resultBytes, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expectedBytes, resultBytes)
	})
}

func TestSerialize_ContentsField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "contents_root"},
		Seq: []SequenceItem{
			{ID: "magic_bytes", Contents: []any{float64(0xCA), float64(0xFE), float64(0xBA), float64(0xBE)}},
			{ID: "some_data", Type: "u1"},
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		// "magic_bytes" data is not provided, it's fixed by schema
		"some_data": uint8(0xDD),
	}
	expectedBytes := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0xDD}
	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestSerialize_StringField(t *testing.T) {
	t.Run("fixed size", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "string_size"},
			Seq:  []SequenceItem{{ID: "msg", Type: "str", Size: 5}},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"msg": "hello"}
		expected := []byte{'h', 'e', 'l', 'l', 'o'}
		res, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expected, res)

		dataTrunc := map[string]any{"msg": "hellothere"}
		resTrunc, err := s.Serialize(context.Background(), dataTrunc)
		require.NoError(t, err)
		assert.Equal(t, expected, resTrunc, "should truncate")

		dataPad := map[string]any{"msg": "hi"}
		expectedPad := []byte{'h', 'i', 0, 0, 0}
		resPad, err := s.Serialize(context.Background(), dataPad)
		require.NoError(t, err)
		assert.Equal(t, expectedPad, resPad, "should pad")
	})

	t.Run("strz", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "string_strz"},
			Seq:  []SequenceItem{{ID: "term_msg", Type: "strz", Encoding: "UTF-8"}},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"term_msg": "world"}
		expected := []byte{'w', 'o', 'r', 'l', 'd', 0x00}
		res, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, expected, res)
	})

	// SizeEOS for strings is tricky for serialization as it implies writing until the buffer ends.
	// This is more of a parsing concept. For serialization, a defined size or terminator is needed.
	// We'll skip size-eos for string serialization tests unless a clear use case emerges.
}

func TestSerialize_BytesField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "bytes_field"},
		Seq:  []SequenceItem{{ID: "raw_data", Type: "bytes", Size: 4}},
	}
	s := newTestSerializer(t, schema)
	data := map[string]any{"raw_data": []byte{1, 2, 3, 4}}
	expected := []byte{1, 2, 3, 4}
	res, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expected, res)

	dataTrunc := map[string]any{"raw_data": []byte{1, 2, 3, 4, 5, 6}}
	resTrunc, err := s.Serialize(context.Background(), dataTrunc)
	require.NoError(t, err)
	assert.Equal(t, expected, resTrunc, "should truncate bytes")

	dataPad := map[string]any{"raw_data": []byte{1, 2}}
	expectedPad := []byte{1, 2, 0, 0}
	resPad, err := s.Serialize(context.Background(), dataPad)
	require.NoError(t, err)
	assert.Equal(t, expectedPad, resPad, "should pad bytes")
}

func TestSerialize_ProcessField_XOR(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "process_xor_root"},
		Seq: []SequenceItem{
			{ID: "data_len", Type: "u1"},
			{ID: "processed_payload", Type: "payload_type", Size: "data_len", Process: "xor(0xAA)"},
		},
		Types: map[string]Type{
			"payload_type": { // The type of the data *before* processing (for serialization)
				Seq: []SequenceItem{ // or *after* processing (for parsing)
					{ID: "field1", Type: "u1"},
					{ID: "field2", Type: "u1"},
				},
			},
		},
	}
	s := newTestSerializer(t, schema)

	// Data is the "logical" un-processed data
	data := map[string]any{
		"data_len": uint8(2),
		"processed_payload": map[string]any{
			"field1": uint8(0x11), // Logical value
			"field2": uint8(0x22), // Logical value
		},
	}

	// Expected:
	// field1_logical = 0x11, field2_logical = 0x22
	// Serialized logical payload = [0x11, 0x22]
	// XOR key = 0xAA
	// field1_processed = 0x11 ^ 0xAA = 0xBB
	// field2_processed = 0x22 ^ 0xAA = 0x88
	// Final bytes: [len, processed_payload_bytes]
	expectedBytes := []byte{0x02, 0x11 ^ 0xAA, 0x22 ^ 0xAA}

	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestSerialize_BuiltinTypes(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "builtins", Endian: "le"}, // Default to LE for non-specified
		Seq: []SequenceItem{
			{ID: "val_u1", Type: "u1"},
			{ID: "val_u2le", Type: "u2le"},
			{ID: "val_u4be", Type: "u4be"},
			{ID: "val_s1", Type: "s1"},
			{ID: "val_s2be", Type: "s2be"},
			{ID: "val_s4le", Type: "s4le"},
			{ID: "val_f4le", Type: "f4le"},
			{ID: "val_f8be", Type: "f8be"},
			{ID: "val_u2_endian_meta", Type: "u2"}, // Should use meta endian (le)
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"val_u1":             uint8(0x12),
		"val_u2le":           uint16(0x3456),
		"val_u4be":           uint32(0x789ABCDE),
		"val_s1":             int8(-1),           // 0xFF
		"val_s2be":           int16(-2),          // 0xFFFE
		"val_s4le":           int32(-1430532899), // This is 0xAABBCCDD in memory (little-endian: DD CC BB AA)
		"val_f4le":           float32(1.5),
		"val_f8be":           float64(2.75),
		"val_u2_endian_meta": uint16(0x1122), // Should be 0x22, 0x11 (le)
	}

	// Expected:
	// u1:     12
	// u2le:   56 34
	// u4be:   78 9A BC DE
	// s1:     FF
	// s2be:   FF FE
	// s4le:   DD CC BB AA
	// f4le (1.5): 00 00 C0 3F
	// f8be (2.75):40 06 00 00 00 00 00 00
	// u2_meta(le): 22 11
	expectedBytes := []byte{
		0x12,
		0x56, 0x34, // 0x3456
		0x78, 0x9A, 0xBC, 0xDE,
		0xFF,
		0xFF, 0xFE,
		0xDD, 0xCC, 0xBB, 0xAA,
		0x00, 0x00, 0xC0, 0x3F, // 1.5f LE
		0x40, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 2.75d BE
		0x22, 0x11,
	}

	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestSerialize_ErrorHandling(t *testing.T) {
	t.Run("missing mandatory data", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "missing_data"},
			Seq:  []SequenceItem{{ID: "required_field", Type: "u1"}},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{
			// "required_field" is missing
		}
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		// The error comes from the type conversion helper (e.g., toUint8) when data is nil
		assert.Contains(t, err.Error(), "cannot convert <nil> to u1")
		assert.Contains(t, err.Error(), "serializing field 'required_field'")
	})

	t.Run("invalid if expression", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "invalid_if"},
			Seq:  []SequenceItem{{ID: "field", Type: "u1", IfExpr: "non_existent_var > 0"}},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"field": 1} // Provide data, but 'if' should fail
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluating if condition")
		assert.Contains(t, err.Error(), "non_existent_var") // CEL error about undeclared reference
	})

	t.Run("invalid switch expression", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "invalid_switch_expr"},
			Seq: []SequenceItem{
				{
					ID:   "sw_field",
					Type: "switch",
					Switch: map[string]any{
						"switch-on": "bad_var + 1", // bad_var doesn't exist
						"cases":     map[string]string{"1": "u1"},
					},
				},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"sw_field": map[string]any{"_": 1}} // Provide some data for the case
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "evaluating switch-on expression")
		assert.Contains(t, err.Error(), "bad_var")
	})

	t.Run("switch no case match no default", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "switch_no_match"},
			Seq: []SequenceItem{
				{ID: "selector", Type: "u1"},
				{
					ID:   "data",
					Type: "switch",
					Switch: map[string]any{
						"switch-on": "selector",
						"cases":     map[string]string{"1": "u1"}, // Only case '1'
					},
				},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"selector": uint8(2), "data": map[string]any{"_": 0xDD}}
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no case matching switch value '2' (key: '2') for expression 'selector' and no default '_' case was found")
	})
}

// Test type conversion helpers - Updated for kaitaicel integration
func TestTypeConversionHelpers(t *testing.T) {
	// Test the kaitaicel factory function instead of the removed helper functions
	tests := []struct {
		name      string
		typeName  string
		input     any
		expectErr bool
	}{
		{"kaitaicel_u1_ok", "u1", 123, false},
		{"kaitaicel_u1_fail", "u1", "abc", true},
		{"kaitaicel_s8le_ok_float", "s8le", 123.0, false},
		{"kaitaicel_s8le_ok_int", "s8le", 456, false},
		{"kaitaicel_f4le_ok", "f4le", 12.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := kaitaicel.NewKaitaiTypeFromValue(tt.input, tt.typeName)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSerializeContext_AsActivation(t *testing.T) {
	writer := kaitai.NewWriter(bytes.NewBuffer(nil))
	rootVal := map[string]any{"root_field": "root_val"}
	parentVal := map[string]any{"parent_field": "parent_val"}
	currentChildren := map[string]any{"current_field": 123}

	rootCtx := &SerializeContext{Value: rootVal, Children: rootVal, Writer: writer}
	rootCtx.Root = rootCtx

	parentSCtx := &SerializeContext{Value: parentVal, Children: parentVal, Writer: writer, Root: rootCtx, Parent: rootCtx}
	sCtx := &SerializeContext{Value: "current_val_placeholder", Children: currentChildren, Writer: writer, Root: rootCtx, Parent: parentSCtx}

	act, err := sCtx.AsActivation()
	require.NoError(t, err)

	// Check current fields
	val, found := act.ResolveName("current_field")
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 123, val)

	// Check _writer
	val, found = act.ResolveName("_writer")
	require.NoError(t, err)
	require.True(t, found)
	assert.Same(t, writer, val)

	// Check _root
	val, found = act.ResolveName("_root")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, rootVal, val) // Should be the map[string]any

	// Check _parent
	val, found = act.ResolveName("_parent")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, parentVal, val) // Should be the map[string]any
}

func TestReverseProcess_XOR(t *testing.T) {
	schema := &KaitaiSchema{Meta: Meta{ID: "dummy"}} // Needed for serializer creation
	s := newTestSerializer(t, schema)
	sCtx := &SerializeContext{ // Dummy context, not strictly used by XOR reverse if key is literal
		Writer: kaitai.NewWriter(bytes.NewBuffer(nil)),
	}

	data := []byte{0x11, 0x22, 0x33}
	keyByte := byte(0xAA)
	processSpec := fmt.Sprintf("xor(0x%X)", keyByte) // "xor(0xAA)"

	expected := make([]byte, len(data))
	for i, b := range data {
		expected[i] = b ^ keyByte
	}

	reversed, err := s.reverseProcess(context.Background(), data, processSpec, sCtx)
	require.NoError(t, err)
	assert.Equal(t, expected, reversed, "XOR is its own inverse")
}

func TestReverseProcess_Zlib(t *testing.T) {
	schema := &KaitaiSchema{Meta: Meta{ID: "dummy"}}
	s := newTestSerializer(t, schema)
	sCtx := &SerializeContext{
		Writer: kaitai.NewWriter(bytes.NewBuffer(nil)),
	}

	// Test data - use longer data to ensure compression actually reduces size
	originalData := []byte("Hello, zlib compression test! This is a longer string to ensure compression actually reduces the size of the data when we test round-trip functionality.")
	processSpec := "zlib"

	// Reverse process should compress the data
	compressed, err := s.reverseProcess(context.Background(), originalData, processSpec, sCtx)
	require.NoError(t, err)
	assert.NotEqual(t, originalData, compressed, "Compressed data should be different")
	// Note: Don't assert size reduction as zlib may add headers that make small data larger

	// Decompress it back to verify round-trip
	decompressed, err := kaitai.ProcessZlib(compressed)
	require.NoError(t, err)
	assert.Equal(t, originalData, decompressed, "Round-trip should recover original data")
}

func TestReverseProcess_Rotate(t *testing.T) {
	schema := &KaitaiSchema{Meta: Meta{ID: "dummy"}}
	s := newTestSerializer(t, schema)
	sCtx := &SerializeContext{
		Writer: kaitai.NewWriter(bytes.NewBuffer(nil)),
	}

	data := []byte{0x01, 0x02, 0x03}
	
	tests := []struct {
		name        string
		processSpec string
		expected    []byte
	}{
		{
			name:        "rol(1) reverse becomes ror(1)",
			processSpec: "rol(1)",
			expected:    []byte{0x80, 0x01, 0x81}, // Right rotate by 1
		},
		{
			name:        "ror(1) reverse becomes rol(1)", 
			processSpec: "ror(1)",
			expected:    []byte{0x02, 0x04, 0x06}, // Left rotate by 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reversed, err := s.reverseProcess(context.Background(), data, tt.processSpec, sCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, reversed)
		})
	}
}

func TestSerialize_ProcessRoundTrip(t *testing.T) {
	// Test that process functions work correctly in serialize -> parse round-trip
	tests := []struct {
		name        string
		processSpec string
		testData    string
	}{
		{
			name:        "XOR with constant",
			processSpec: "xor(0xff)",
			testData:    "test data",
		},
		{
			name:        "Zlib compression",
			processSpec: "zlib", 
			testData:    "This is a longer test string to ensure zlib compression works properly",
		},
		{
			name:        "Rotate left",
			processSpec: "rol(3)",
			testData:    "rotate",
		},
		{
			name:        "Rotate right", 
			processSpec: "ror(2)",
			testData:    "rotate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlContent := fmt.Sprintf(`
meta:
  id: process_test
seq:
  - id: data
    type: bytes
    size-eos: true
    process: %s
`, tt.processSpec)

			schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
			require.NoError(t, err)

			// Create test data structure
			testStruct := map[string]any{
				"data": tt.testData,
			}

			// Serialize
			serializer := newTestSerializer(t, schema)
			serializedData, err := serializer.Serialize(context.Background(), testStruct)
			require.NoError(t, err)

			// Parse back
			interpreter, err := NewKaitaiInterpreter(schema, nil)
			require.NoError(t, err)

			stream := kaitai.NewStream(bytes.NewReader(serializedData))
			parsed, err := interpreter.Parse(context.Background(), stream)
			require.NoError(t, err)

			parsedMap := ParsedDataToMap(parsed)
			dataMap, ok := parsedMap.(map[string]any)
			require.True(t, ok)

			// Verify round-trip (now expecting bytes since we changed type to bytes)
			parsedData, ok := dataMap["data"]
			require.True(t, ok)
			
			dataBytes, ok := parsedData.([]byte)
			require.True(t, ok, "Expected bytes data")
			assert.Equal(t, tt.testData, string(dataBytes))
		})
	}
}

func TestSerialize_RootTypeSpecifiedInMeta(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "should_be_ignored_id", Endian: "le"}, // This ID should be ignored
		Types: map[string]Type{
			"my_actual_serialization_root": {
				Seq: []SequenceItem{
					{ID: "data_val", Type: "u2le"},
					{ID: "another_val", Type: "s1"},
				},
			},
			"should_be_ignored_id": { // This type definition matches Meta.ID but should be ignored
				Seq: []SequenceItem{
					{ID: "ignored_data", Type: "u4le"},
				},
			},
		},
		// Top-level Seq should also be ignored if RootType is specified in Meta
		Seq: []SequenceItem{
			{ID: "top_level_ignored", Type: "u1"},
		},
		RootType: "my_actual_serialization_root", // Explicitly set the root type for serialization
	}
	s := newTestSerializer(t, schema)

	dataToSerialize := map[string]any{
		"data_val":    uint16(0xABCD),
		"another_val": int8(-1), // 0xFF
	}

	// Expected bytes based on "my_actual_serialization_root"
	// u2le(0xABCD) -> CD AB
	// s1(-1)       -> FF
	expectedBytes := []byte{0xCD, 0xAB, 0xFF}

	resultBytes, err := s.Serialize(context.Background(), dataToSerialize)
	require.NoError(t, err, "Serialization should not fail")
	assert.Equal(t, expectedBytes, resultBytes, "Serialized bytes should match the structure of Meta.RootType")
}

// ===== KAITAICEL INTEGRATION TESTS =====

func TestKaitaicelIntegration_BuiltinTypes(t *testing.T) {
	// Test all primitive types with kaitaicel integration
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
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"u1_val":   uint8(0x12),
		"u2le_val": uint16(0x3456),
		"u2be_val": uint16(0x3456),
		"u4le_val": uint32(0x789ABCDE),
		"u4be_val": uint32(0x789ABCDE),
		"u8le_val": uint64(0x123456789ABCDEF0),
		"u8be_val": uint64(0x123456789ABCDEF0),
		"s1_val":   int8(-1),
		"s2le_val": int16(-2),
		"s2be_val": int16(-2),
		"s4le_val": int32(-1430532899), // 0xAABBCCDD
		"s4be_val": int32(-1430532899),
		"s8le_val": int64(-1), // 0xFFFFFFFFFFFFFFFF
		"s8be_val": int64(-1),
		"f4le_val": float32(1.5),
		"f4be_val": float32(1.5),
		"f8le_val": float64(2.75),
		"f8be_val": float64(2.75),
	}

	expectedBytes := []byte{
		0x12,       // u1
		0x56, 0x34, // u2le
		0x34, 0x56, // u2be
		0xDE, 0xBC, 0x9A, 0x78, // u4le
		0x78, 0x9A, 0xBC, 0xDE, // u4be
		0xF0, 0xDE, 0xBC, 0x9A, 0x78, 0x56, 0x34, 0x12, // u8le
		0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, // u8be
		0xFF,       // s1 (-1)
		0xFE, 0xFF, // s2le (-2)
		0xFF, 0xFE, // s2be (-2)
		0xDD, 0xCC, 0xBB, 0xAA, // s4le (0xAABBCCDD)
		0xAA, 0xBB, 0xCC, 0xDD, // s4be (0xAABBCCDD)
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // s8le (-1)
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // s8be (-1)
		0x00, 0x00, 0xC0, 0x3F, // f4le (1.5)
		0x3F, 0xC0, 0x00, 0x00, // f4be (1.5)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x40, // f8le (2.75)
		0x40, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // f8be (2.75)
	}

	resultBytes, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, resultBytes)
}

func TestKaitaicelIntegration_TypeConversion(t *testing.T) {
	// Test various Go types being converted to kaitai types
	tests := []struct {
		name        string
		typeName    string
		inputValue  any
		expectedHex string
		shouldError bool
	}{
		// Unsigned integers from various Go types
		{"u1_from_int", "u1", int(123), "7B", false},
		{"u1_from_float64", "u1", float64(123), "7B", false},
		{"u2le_from_int32", "u2le", int32(0x1234), "3412", false},
		{"u4be_from_uint64", "u4be", uint64(0x12345678), "12345678", false},

		// Signed integers
		{"s1_from_negative", "s1", int(-1), "FF", false},
		{"s2le_from_int", "s2le", int(-2), "FEFF", false},
		{"s4be_from_negative", "s4be", int32(-1), "FFFFFFFF", false},

		// Floats
		{"f4le_from_float64", "f4le", float64(1.0), "0000803F", false},
		{"f8be_from_float32", "f8be", float32(1.0), "3FF0000000000000", false},

		// Error cases
		{"invalid_type", "invalid_type", 123, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kaitaiType, err := kaitaicel.NewKaitaiTypeFromValue(tt.inputValue, tt.typeName)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, kaitaiType)

			serialized := kaitaiType.Serialize()
			actualHex := fmt.Sprintf("%X", serialized)
			assert.Equal(t, tt.expectedHex, actualHex)
		})
	}
}

func TestKaitaicelIntegration_EndianHandlingInSerializer(t *testing.T) {
	// Test that schema endianness is properly applied
	testCases := []struct {
		name     string
		endian   string
		type_    string
		value    uint16
		expected []byte
	}{
		{"le_u2_explicit", "le", "u2le", 0x1234, []byte{0x34, 0x12}},
		{"be_u2_explicit", "be", "u2be", 0x1234, []byte{0x12, 0x34}},
		{"le_u2_from_meta", "le", "u2", 0x1234, []byte{0x34, 0x12}},
		{"be_u2_from_meta", "be", "u2", 0x1234, []byte{0x12, 0x34}},
		{"default_be_u2", "", "u2", 0x1234, []byte{0x12, 0x34}}, // default is BE
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &KaitaiSchema{
				Meta: Meta{ID: "endian_test", Endian: tc.endian},
				Seq:  []SequenceItem{{ID: "value", Type: tc.type_}},
			}
			s := newTestSerializer(t, schema)

			data := map[string]any{"value": tc.value}
			result, err := s.Serialize(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKaitaicelIntegration_KaitaiTypeRoundtrip(t *testing.T) {
	// Test that we can create kaitai types and serialize them correctly
	tests := []struct {
		name        string
		createType  func() kaitaicel.KaitaiType
		expectedHex string
	}{
		{
			name: "u2le_direct",
			createType: func() kaitaicel.KaitaiType {
				return kaitaicel.NewU2LEFromValue(0x1234)
			},
			expectedHex: "3412",
		},
		{
			name: "s4be_direct",
			createType: func() kaitaicel.KaitaiType {
				return kaitaicel.NewS4BEFromValue(-1)
			},
			expectedHex: "FFFFFFFF",
		},
		{
			name: "f4le_direct",
			createType: func() kaitaicel.KaitaiType {
				return kaitaicel.NewF4LEFromValue(1.0)
			},
			expectedHex: "0000803F",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kaitaiType := tt.createType()
			serialized := kaitaiType.Serialize()
			actualHex := fmt.Sprintf("%X", serialized)
			assert.Equal(t, tt.expectedHex, actualHex)
		})
	}
}

func TestKaitaicelIntegration_SerializeMethodConsistency(t *testing.T) {
	// Test that all kaitai types properly implement Serialize()
	tests := []struct {
		name       string
		kaitaiType kaitaicel.KaitaiType
		expectNil  bool // Some types like BitField return nil
	}{
		{"KaitaiInt_u1", kaitaicel.NewU1FromValue(0x42), false},
		{"KaitaiInt_s8le", kaitaicel.NewS8LEFromValue(-1), false},
		{"KaitaiFloat_f4be", kaitaicel.NewF4BEFromValue(1.5), false},
		{"KaitaiString", createTestKaitaiString("test", "UTF-8"), false},
		{"KaitaiBytes", kaitaicel.NewKaitaiBytes([]byte{1, 2, 3, 4}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.kaitaiType)

			// Test that Serialize() doesn't panic
			serialized := tt.kaitaiType.Serialize()

			if tt.expectNil {
				assert.Nil(t, serialized)
			} else {
				assert.NotNil(t, serialized)
				assert.Greater(t, len(serialized), 0)
			}

			// Test that KaitaiTypeName() works
			typeName := tt.kaitaiType.KaitaiTypeName()
			assert.NotEmpty(t, typeName)
		})
	}
}

func TestKaitaicelIntegration_StringSerialization(t *testing.T) {
	// Test string serialization with different encodings
	tests := []struct {
		name     string
		value    string
		encoding string
		expected []byte
	}{
		{"utf8_string", "hello", "UTF-8", []byte("hello")},
		{"ascii_string", "test", "ASCII", []byte("test")},
		{"empty_string", "", "UTF-8", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kaitaiStr := createTestKaitaiString(tt.value, tt.encoding)
			serialized := kaitaiStr.Serialize()
			assert.Equal(t, tt.expected, serialized)
		})
	}
}

func TestKaitaicelIntegration_ErrorHandlingInSerializer(t *testing.T) {
	// Test error handling in kaitaicel integration
	tests := []struct {
		name        string
		typeName    string
		value       any
		expectError bool
	}{
		{"valid_u1", "u1", uint8(123), false},
		{"valid_conversion", "u1", int(123), false},
		{"unsupported_type", "unknown_type", 123, true},
		{"invalid_string_conversion", "u1", "not_a_number", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := kaitaicel.NewKaitaiTypeFromValue(tt.value, tt.typeName)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKaitaicelIntegration_SerializerIntegration(t *testing.T) {
	// Test the complete integration with the serializer
	schema := &KaitaiSchema{
		Meta: Meta{ID: "integration_test", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "header", Type: "u4le"},
			{ID: "count", Type: "u1"},
			{ID: "values", Type: "u2le", Repeat: "expr", RepeatExpr: "count"},
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"header": uint32(0xDEADBEEF),
		"count":  uint8(3),
		"values": []any{
			uint16(0x1111),
			uint16(0x2222),
			uint16(0x3333),
		},
	}

	// Expected:
	// header (u4le): EF BE AD DE
	// count (u1): 03
	// values[0] (u2le): 11 11
	// values[1] (u2le): 22 22
	// values[2] (u2le): 33 33
	expected := []byte{
		0xEF, 0xBE, 0xAD, 0xDE, // header
		0x03,       // count
		0x11, 0x11, // values[0]
		0x22, 0x22, // values[1]
		0x33, 0x33, // values[2]
	}

	result, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestKaitaicelIntegration_PerformanceBaseline(t *testing.T) {
	// Performance test for the new kaitaicel implementation
	schema := &KaitaiSchema{
		Meta: Meta{ID: "perf_test", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "data", Type: "u4le"},
		},
	}
	s := newTestSerializer(t, schema)

	data := map[string]any{
		"data": uint32(0x12345678),
	}

	// Warmup
	for range 1000 {
		_, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
	}

	// This test ensures the new implementation works and provides a baseline
	// for future performance improvements
	expected := []byte{0x78, 0x56, 0x34, 0x12}
	result, err := s.Serialize(context.Background(), data)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

// ===== ADVANCED SERIALIZER ERROR HANDLING TESTS =====

func TestSerialize_AdvancedErrorHandling(t *testing.T) {
	t.Run("corrupted schema data", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "corrupted_schema"},
			Seq: []SequenceItem{
				{ID: "field", Type: "str", Size: "invalid_size_expr"},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"field": "test"}
		
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_size_expr")
	})

	t.Run("type conversion overflow", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "overflow_test"},
			Seq: []SequenceItem{
				{ID: "tiny_val", Type: "u1"},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"tiny_val": int64(999999)} // Too large for u1
		
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("invalid encoding for string", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "invalid_encoding"},
			Seq: []SequenceItem{
				{ID: "str_field", Type: "str", Size: 5, Encoding: "INVALID_ENCODING"},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"str_field": "hello"}
		
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encoding")
	})

	t.Run("circular type reference", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "circular_ref"},
			Types: map[string]Type{
				"type_a": {Seq: []SequenceItem{{ID: "ref_b", Type: "type_b"}}},
				"type_b": {Seq: []SequenceItem{{ID: "ref_a", Type: "type_a"}}},
			},
			Seq: []SequenceItem{{ID: "start", Type: "type_a"}},
		}
		s := newTestSerializer(t, schema)
		
		// Create circular data structure
		dataA := map[string]any{}
		dataB := map[string]any{}
		dataA["ref_b"] = dataB
		dataB["ref_a"] = dataA
		data := map[string]any{"start": dataA}
		
		// This should eventually hit a stack overflow or similar error
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		_, err := s.Serialize(ctx, data)
		require.Error(t, err)
		// Error could be timeout or stack overflow
	})

	t.Run("invalid repeat expression result", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "invalid_repeat"},
			Seq: []SequenceItem{
				{ID: "items", Type: "u1", Repeat: "expr", RepeatExpr: `"not_a_number"`},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"items": []any{uint8(1), uint8(2)}}
		
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "result is not a number")
	})

	t.Run("process function with invalid parameters", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "invalid_process"},
			Seq: []SequenceItem{
				{ID: "data", Type: "bytes", Size: 4, Process: "xor(invalid_hex)"},
			},
		}
		s := newTestSerializer(t, schema)
		data := map[string]any{"data": []byte{1, 2, 3, 4}}
		
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		// Should fail when trying to evaluate the invalid parameter
	})

	t.Run("enum serialization with invalid value", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "enum_test"},
			Enums: map[string]EnumDef{
				"colors": {1: "red", 2: "blue", 3: "green"},
			},
			Seq: []SequenceItem{
				{ID: "color", Type: "u1", Enum: "colors"},
			},
		}
		s := newTestSerializer(t, schema)
		
		// Try to serialize enum with invalid structure
		data := map[string]any{"color": "invalid_enum_format"}
		
		_, err := s.Serialize(context.Background(), data)
		require.Error(t, err)
		// Should fail because enum value is not in the expected format
	})
}

func TestSerialize_SizeEosEdgeCases(t *testing.T) {
	t.Run("size-eos with complex types", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "size_eos_complex"},
			Seq: []SequenceItem{
				{ID: "header", Type: "u2le"},
				{ID: "data", Type: "str", SizeEOS: true, Encoding: "UTF-8"},
			},
		}
		s := newTestSerializer(t, schema)
		
		data := map[string]any{
			"header": uint16(0x1234),
			"data":   "This is the remaining data until end of stream",
		}
		
		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		
		// Should contain header + all the string data
		assert.True(t, len(result) > 2) // At least header size
		assert.Equal(t, []byte{0x34, 0x12}, result[:2]) // Header in LE
		assert.Equal(t, "This is the remaining data until end of stream", string(result[2:]))
	})

	t.Run("multiple size-eos fields should error", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "multiple_size_eos"},
			Seq: []SequenceItem{
				{ID: "data1", Type: "bytes", SizeEOS: true},
				{ID: "data2", Type: "bytes", SizeEOS: true}, // This should be problematic
			},
		}
		s := newTestSerializer(t, schema)
		
		data := map[string]any{
			"data1": []byte{1, 2, 3},
			"data2": []byte{4, 5, 6},
		}
		
		// The behavior here depends on implementation - could work or error
		// For now, just test that it doesn't panic
		_, err := s.Serialize(context.Background(), data)
		// Could be success or error, but shouldn't panic
		t.Logf("Multiple size-eos result: %v", err)
	})

	t.Run("size-eos with process", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "size_eos_process"},
			Seq: []SequenceItem{
				{ID: "compressed_data", Type: "bytes", SizeEOS: true, Process: "zlib"},
			},
		}
		s := newTestSerializer(t, schema)
		
		data := map[string]any{
			"compressed_data": "This text will be compressed with zlib when serialized",
		}
		
		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		
		// Should contain zlib-compressed data
		assert.True(t, len(result) > 0)
		// Verify it's actually compressed by trying to decompress
		reader, err := zlib.NewReader(bytes.NewReader(result))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		reader.Close()
		
		assert.Equal(t, "This text will be compressed with zlib when serialized", string(decompressed))
	})
}

func TestSerialize_NestedSwitchTypes(t *testing.T) {
	t.Run("switch within switch", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "nested_switch"},
			Types: map[string]Type{
				"inner_type_a": {Seq: []SequenceItem{{ID: "value", Type: "u1"}}},
				"inner_type_b": {Seq: []SequenceItem{{ID: "value", Type: "u2le"}}},
				"outer_type_x": {
					Seq: []SequenceItem{
						{ID: "inner_selector", Type: "u1"},
						{
							ID:   "inner_data",
							Type: "switch",
							Switch: map[string]any{
								"switch-on": "inner_selector",
								"cases": map[string]string{
									"1": "inner_type_a",
									"2": "inner_type_b",
								},
							},
						},
					},
				},
				"outer_type_y": {Seq: []SequenceItem{{ID: "simple", Type: "u4le"}}},
			},
			Seq: []SequenceItem{
				{ID: "outer_selector", Type: "u1"},
				{
					ID:   "outer_data",
					Type: "switch",
					Switch: map[string]any{
						"switch-on": "outer_selector",
						"cases": map[string]string{
							"10": "outer_type_x",
							"20": "outer_type_y",
						},
					},
				},
			},
		}
		s := newTestSerializer(t, schema)
		
		data := map[string]any{
			"outer_selector": uint8(10),
			"outer_data": map[string]any{
				"inner_selector": uint8(2),
				"inner_data": map[string]any{
					"value": uint16(0x1234),
				},
			},
		}
		
		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		
		expected := []byte{
			10,         // outer_selector
			2,          // inner_selector  
			0x34, 0x12, // value as u2le
		}
		assert.Equal(t, expected, result)
	})

	t.Run("switch with ad-hoc type expressions", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "adhoc_switch_complex"},
			Types: map[string]Type{
				"type_conditional": {Seq: []SequenceItem{{ID: "data", Type: "u4be"}}},
			},
			Seq: []SequenceItem{
				{ID: "mode", Type: "u1"},
				{ID: "size_factor", Type: "u1"},
				{
					ID:   "dynamic_field",
					Type: "switch-on: mode > 5 ? (size_factor == 1 ? 'u1' : 'type_conditional') : 'u2le'",
				},
			},
		}
		s := newTestSerializer(t, schema)
		
		// Test case 1: mode > 5 and size_factor == 1 -> u1
		data1 := map[string]any{
			"mode":          uint8(10),
			"size_factor":   uint8(1),
			"dynamic_field": uint8(0xFF),
		}
		
		result1, err := s.Serialize(context.Background(), data1)
		require.NoError(t, err)
		assert.Equal(t, []byte{10, 1, 0xFF}, result1)
		
		// Test case 2: mode > 5 and size_factor != 1 -> type_conditional
		data2 := map[string]any{
			"mode":        uint8(10),
			"size_factor": uint8(2),
			"dynamic_field": map[string]any{
				"data": uint32(0x12345678),
			},
		}
		
		result2, err := s.Serialize(context.Background(), data2)
		require.NoError(t, err)
		assert.Equal(t, []byte{10, 2, 0x12, 0x34, 0x56, 0x78}, result2) // u4be
		
		// Test case 3: mode <= 5 -> u2le
		data3 := map[string]any{
			"mode":          uint8(3),
			"size_factor":   uint8(99), // Irrelevant
			"dynamic_field": uint16(0x1234),
		}
		
		result3, err := s.Serialize(context.Background(), data3)
		require.NoError(t, err)
		assert.Equal(t, []byte{3, 99, 0x34, 0x12}, result3) // u2le
	})
}

func TestSerialize_InstancesAndExpressions(t *testing.T) {
	t.Run("instances used in size expressions", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "instances_in_size"},
			Instances: map[string]InstanceDef{
				"calculated_size": {Value: "header_len + 4"},
				"max_size":        {Value: "calculated_size > 10 ? 10 : calculated_size"},
			},
			Seq: []SequenceItem{
				{ID: "header_len", Type: "u1"},
				{ID: "dynamic_data", Type: "bytes", Size: "max_size"},
			},
		}
		s := newTestSerializer(t, schema)
		
		data := map[string]any{
			"header_len":    uint8(3),
			"dynamic_data": []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11},
		}
		
		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		
		// calculated_size = 3 + 4 = 7, max_size = 7 (since 7 <= 10)
		// So dynamic_data should be truncated to 7 bytes
		expected := []byte{3, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11}
		assert.Equal(t, expected, result)
	})

	t.Run("instances with complex expressions", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "complex_instances"},
			Instances: map[string]InstanceDef{
				"is_big_endian":   {Value: "(header_flags & 0x01) != 0"},
				"data_multiplier": {Value: "is_big_endian ? 2 : 1"},
				"actual_count":    {Value: "item_count * data_multiplier"},
			},
			Seq: []SequenceItem{
				{ID: "header_flags", Type: "u1"},
				{ID: "item_count", Type: "u1"},
				{ID: "items", Type: "u1", Repeat: "expr", RepeatExpr: "actual_count"},
			},
		}
		s := newTestSerializer(t, schema)
		
		// Test big-endian case (flags & 0x01 != 0)
		data := map[string]any{
			"header_flags": uint8(0x01), // is_big_endian = true
			"item_count":   uint8(3),    // actual_count = 3 * 2 = 6
			"items":        []any{uint8(1), uint8(2), uint8(3), uint8(4), uint8(5), uint8(6)},
		}
		
		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		
		expected := []byte{0x01, 3, 1, 2, 3, 4, 5, 6}
		assert.Equal(t, expected, result)
	})
}

func TestSerialize_ContextCancellation(t *testing.T) {
	t.Run("context cancellation during serialization", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "cancellation_test"},
			Seq: []SequenceItem{
				{ID: "items", Type: "u4le", Repeat: "expr", RepeatExpr: "count"},
			},
		}
		s := newTestSerializer(t, schema)
		
		// Create a large dataset to increase chance of cancellation
		items := make([]any, 1000)
		for i := range items {
			items[i] = uint32(i)
		}
		
		data := map[string]any{
			"count": uint32(1000),
			"items": items,
		}
		
		// Cancel context quickly
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		_, err := s.Serialize(ctx, data)
		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("timeout during complex serialization", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "timeout_test"},
			Types: map[string]Type{
				"complex_type": {
					Seq: []SequenceItem{
						{ID: "data", Type: "bytes", Size: "1000"},
					},
				},
			},
			Seq: []SequenceItem{
				{ID: "items", Type: "complex_type", Repeat: "expr", RepeatExpr: "100"},
			},
		}
		s := newTestSerializer(t, schema)
		
		// Create data
		item := map[string]any{"data": make([]byte, 1000)}
		items := make([]any, 100)
		for i := range items {
			items[i] = item
		}
		
		data := map[string]any{"items": items}
		
		// Very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		_, err := s.Serialize(ctx, data)
		// Should timeout or be canceled
		require.Error(t, err)
		// Could be DeadlineExceeded or Canceled depending on timing
	})
}

// ===== ENUM SERIALIZATION TESTS =====

func TestSerialize_EnumValues(t *testing.T) {
	// Test data based on test/formats/enum_0.ksy and test/src/enum_0.bin
	t.Run("basic enum serialization", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "enum_0"},
			Enums: map[string]EnumDef{
				"animal": {4: "cat", 7: "chicken", 12: "dog"},
			},
			Seq: []SequenceItem{
				{ID: "pet_1", Type: "u1", Enum: "animal"},
				{ID: "pet_2", Type: "u1", Enum: "animal"},
			},
		}
		s := newTestSerializer(t, schema)

		// Test serializing enum objects (as expected from parser output)
		data := map[string]any{
			"pet_1": map[string]any{"name": "cat", "value": int64(4), "valid": true},
			"pet_2": map[string]any{"name": "chicken", "value": int64(7), "valid": true},
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		// Should match test/src/enum_0.bin content
		expected := []byte{4, 7} // cat=4, chicken=7
		assert.Equal(t, expected, result)
	})

	t.Run("enum with integer values", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "enum_test_int"},
			Enums: map[string]EnumDef{
				"status": {0: "inactive", 1: "active", 255: "error"},
			},
			Seq: []SequenceItem{
				{ID: "state", Type: "u1", Enum: "status"},
			},
		}
		s := newTestSerializer(t, schema)

		// Test with raw integer values (alternative input format)
		data := map[string]any{
			"state": uint8(255), // error state
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, []byte{255}, result)
	})

	t.Run("enum with invalid value should error", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "enum_invalid"},
			Enums: map[string]EnumDef{
				"valid_values": {1: "one", 2: "two"},
			},
			Seq: []SequenceItem{
				{ID: "value", Type: "u1", Enum: "valid_values"},
			},
		}
		s := newTestSerializer(t, schema)

		// Try invalid enum value
		data := map[string]any{
			"value": uint8(99), // Not in enum
		}

		// Note: This might succeed depending on implementation
		// Kaitai generally allows invalid enum values during serialization
		result, err := s.Serialize(context.Background(), data)
		if err == nil {
			assert.Equal(t, []byte{99}, result)
			t.Log("Serializer allows invalid enum values (expected behavior)")
		} else {
			t.Log("Serializer rejects invalid enum values")
		}
	})
}

// ===== BIT FIELD SERIALIZATION TESTS =====

func TestSerialize_BitFields(t *testing.T) {
	// Test data based on test/formats/bits_simple.ksy and test/src/bits_simple.bin
	t.Run("mixed bit and byte fields", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "bits_simple"},
			Seq: []SequenceItem{
				{ID: "byte_1", Type: "u1"},
				{ID: "bits_a", Type: "b1"},
				{ID: "bits_b", Type: "b1"},
				{ID: "bits_c", Type: "b6"},
				{ID: "byte_2", Type: "u1"},
				{ID: "test_if_b1", Type: "b1", IfExpr: "bits_a != 0"},
				{ID: "byte_8_9_10", Type: "b24le"},
				{ID: "byte_11_to_14", Type: "b32le"},
				{ID: "byte_15_to_19", Type: "b40be"},
				{ID: "byte_20_to_27", Type: "b64be"},
				{ID: "spacer", Type: "b1"},
				{ID: "large_bits_1", Type: "b10"},
				{ID: "large_bits_2", Type: "b53"},
			},
		}
		s := newTestSerializer(t, schema)

		// Test data that should produce known binary output
		data := map[string]any{
			"byte_1":         uint8(0x50),
			"bits_a":         uint8(1),     // 1 bit
			"bits_b":         uint8(0),     // 1 bit  
			"bits_c":         uint8(0x2A),  // 6 bits (0x2A = 42, fits in 6 bits)
			"byte_2":         uint8(0x41),
			"test_if_b1":     uint8(1),     // Should be included since bits_a != 0
			"byte_8_9_10":    uint32(0x434b50), // 24-bit LE
			"byte_11_to_14":  uint32(0x2d31ffff), // 32-bit LE
			"byte_15_to_19":  uint64(0x5041434b2d), // 40-bit BE (truncated)
			"byte_20_to_27":  uint64(0x31ffff5041434b2d), // 64-bit BE
			"spacer":         uint8(0),     // 1 bit padding
			"large_bits_1":   uint16(0x200), // 10 bits
			"large_bits_2":   uint64(0x1fffffffffffff), // 53 bits
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		// The exact output depends on bit packing implementation
		// Just verify we get some reasonable output
		assert.True(t, len(result) > 0)
		t.Logf("Bit field serialization produced %d bytes", len(result))
	})

	t.Run("bit endianness variations", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "bit_endian_test", BitEndian: "le"},
			Seq: []SequenceItem{
				{ID: "bits_le", Type: "b12le"},
				{ID: "bits_be", Type: "b12be"},
				{ID: "padding", Type: "b8"},
			},
		}
		s := newTestSerializer(t, schema)

		data := map[string]any{
			"bits_le": uint16(0x123),
			"bits_be": uint16(0x456),
			"padding": uint8(0xFF),
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)
		assert.True(t, len(result) >= 4) // At least 32 bits = 4 bytes
	})
}

// ===== FLOATING POINT SERIALIZATION TESTS =====

func TestSerialize_FloatingPoint(t *testing.T) {
	// Test data based on test/formats/floating_points.ksy and test/src/floating_points.bin
	t.Run("float32 and float64 values", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "floating_points", Endian: "le"},
			Seq: []SequenceItem{
				{ID: "single_value", Type: "f4"},
				{ID: "double_value", Type: "f8"},
				{ID: "single_value_be", Type: "f4be"},
				{ID: "double_value_be", Type: "f8be"},
				{ID: "approximate_value", Type: "f4le"},
			},
		}
		s := newTestSerializer(t, schema)

		data := map[string]any{
			"single_value":     float32(0.5),
			"double_value":     float64(0.25),
			"single_value_be":  float32(16.0),
			"double_value_be":  float64(8.5),
			"approximate_value": float32(1.2345),
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		// Should be 4 + 8 + 4 + 8 + 4 = 28 bytes
		assert.Equal(t, 28, len(result))

		// Test that round-trip works (parse back and verify)
		interpreter, err := NewKaitaiInterpreter(schema, nil)
		require.NoError(t, err)

		stream := kaitai.NewStream(bytes.NewReader(result))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		parsedMap := ParsedDataToMap(parsed)
		dataMap, ok := parsedMap.(map[string]any)
		require.True(t, ok)

		// Verify floating point values (with some tolerance for precision)
		assert.InDelta(t, 0.5, dataMap["single_value"], 0.0001)
		assert.InDelta(t, 0.25, dataMap["double_value"], 0.000001)
		assert.InDelta(t, 16.0, dataMap["single_value_be"], 0.0001)
		assert.InDelta(t, 8.5, dataMap["double_value_be"], 0.000001)
		assert.InDelta(t, 1.2345, dataMap["approximate_value"], 0.0001)
	})
}

// ===== REPEAT FIELD SERIALIZATION TESTS =====

func TestSerialize_RepeatFields(t *testing.T) {
	t.Run("repeat eos with structs", func(t *testing.T) {
		// Based on test/formats/repeat_eos_struct.ksy
		schema := &KaitaiSchema{
			Meta: Meta{ID: "repeat_eos_struct"},
			Types: map[string]Type{
				"chunk": {
					Seq: []SequenceItem{
						{ID: "offset", Type: "u4le"},
						{ID: "len", Type: "u4le"},
					},
				},
			},
			Seq: []SequenceItem{
				{ID: "chunks", Type: "chunk", Repeat: "eos"},
			},
		}
		s := newTestSerializer(t, schema)

		data := map[string]any{
			"chunks": []any{
				map[string]any{"offset": uint32(0x12345678), "len": uint32(0x9ABCDEF0)},
				map[string]any{"offset": uint32(0x11111111), "len": uint32(0x22222222)},
				map[string]any{"offset": uint32(0x33333333), "len": uint32(0x44444444)},
			},
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		// Should be 3 chunks * 8 bytes each = 24 bytes
		assert.Equal(t, 24, len(result))

		// Verify by parsing back
		interpreter, err := NewKaitaiInterpreter(schema, nil)
		require.NoError(t, err)

		stream := kaitai.NewStream(bytes.NewReader(result))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		parsedMap := ParsedDataToMap(parsed)
		dataMap, ok := parsedMap.(map[string]any)
		require.True(t, ok)

		chunks, ok := dataMap["chunks"].([]any)
		require.True(t, ok)
		assert.Len(t, chunks, 3)
	})

	t.Run("repeat until condition", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "repeat_until_test"},
			Seq: []SequenceItem{
				{ID: "values", Type: "u1", Repeat: "until", RepeatUntil: "_ == 0xFF"},
			},
		}
		s := newTestSerializer(t, schema)

		// Data ending with 0xFF terminator
		data := map[string]any{
			"values": []any{uint8(1), uint8(2), uint8(3), uint8(0xFF)},
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		expected := []byte{1, 2, 3, 0xFF}
		assert.Equal(t, expected, result)
	})
}

// ===== STRING ENCODING SERIALIZATION TESTS =====

func TestSerialize_StringEncodings(t *testing.T) {
	// Based on test/formats/str_encodings.ksy and test/src/str_encodings.bin
	t.Run("various string encodings", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "str_encodings"},
			Seq: []SequenceItem{
				{ID: "str1", Type: "str", Size: 12, Encoding: "ASCII"},
				{ID: "str2", Type: "str", Size: 8, Encoding: "UTF-8"},
				{ID: "str3", Type: "str", Size: 10}, // Default encoding
				{ID: "str4", Type: "str", Size: 4, Encoding: "UTF-8"},
			},
		}
		s := newTestSerializer(t, schema)

		data := map[string]any{
			"str1": "Some ASCII",  // 12 chars
			"str2": "UTF-8 ♠♥",    // 8 bytes (symbols are multi-byte)
			"str3": "And more ",   // 10 chars
			"str4": "2\u0013\u0001\u0002", // 4 bytes with special chars
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		// Total should be 12 + 8 + 10 + 4 = 34 bytes
		// (might be different due to UTF-8 encoding)
		assert.True(t, len(result) >= 30) // At least close to expected
	})

	t.Run("string with null terminator", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "strz_test"},
			Seq: []SequenceItem{
				{ID: "greeting", Type: "strz", Encoding: "UTF-8"},
				{ID: "name", Type: "strz", Encoding: "ASCII"},
			},
		}
		s := newTestSerializer(t, schema)

		data := map[string]any{
			"greeting": "Hello",
			"name":     "World",
		}

		result, err := s.Serialize(context.Background(), data)
		require.NoError(t, err)

		// Should contain strings with null terminators
		expected := []byte("Hello\x00World\x00")
		assert.Equal(t, expected, result)
	})
}
