package kaitaistruct

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSerializer(t *testing.T, schema *KaitaiSchema) *KaitaiSerializer {
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// For debugging, you can use:
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s, err := NewKaitaiSerializer(schema, logger)
	require.NoError(t, err)
	return s
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
		assert.Contains(t, err.Error(), "cannot convert <nil> to uint8")
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

// Test type conversion helpers
func TestTypeConversionHelpers(t *testing.T) {
	tests := []struct {
		name      string
		converter func(any) (any, error)
		input     any
		expected  any
		expectErr bool
	}{
		{"toUint8_ok", func(a any) (any, error) { v, e := toUint8(a); return v, e }, 123, uint8(123), false},
		{"toUint8_fail", func(a any) (any, error) { v, e := toUint8(a); return v, e }, "abc", nil, true},
		{"toInt64_ok_float", func(a any) (any, error) { v, e := toInt64(a); return v, e }, 123.0, int64(123), false},
		{"toInt64_ok_int", func(a any) (any, error) { v, e := toInt64(a); return v, e }, 456, int64(456), false},
		{"toFloat32_ok", func(a any) (any, error) { v, e := toFloat32(a); return v, e }, 12.5, float32(12.5), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.converter(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, res)
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
