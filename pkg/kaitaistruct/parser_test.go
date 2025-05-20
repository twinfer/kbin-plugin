package kaitaistruct

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestInterpreter(t *testing.T, schema *KaitaiSchema) *KaitaiInterpreter {
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// For debugging specific tests:
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	interp, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)
	return interp
}

// Helper to get value from ParsedData, simplifying assertions
func getParsedValue(t *testing.T, pd *ParsedData, path ...string) any {
	require.NotNil(t, pd, "ParsedData is nil at path %v", path)
	current := pd
	for i, p := range path {
		require.NotNil(t, current.Children, "Children map is nil at %s in path %v", p, path[:i+1])
		child, ok := current.Children[p]
		require.True(t, ok, "Path element '%s' not found in children at path %v. Available: %v", p, path[:i+1], getMapKeys(current.Children))
		require.NotNil(t, child, "Child ParsedData for '%s' is nil at path %v", p, path[:i+1])
		current = child
	}
	return current.Value
}

func getMapKeys(m map[string]*ParsedData) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestParse_SimpleRootType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "simple_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "magic", Type: "u1"},
			{ID: "length", Type: "u2le"},
			{ID: "message", Type: "str", Size: "length", Encoding: "UTF-8"},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0x42, 0x05, 0x00, 'h', 'e', 'l', 'l', 'o'} // magic=0x42, length=5, message="hello"
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, uint8(0x42), getParsedValue(t, parsed, "magic"))
	assert.Equal(t, uint16(5), getParsedValue(t, parsed, "length"))
	assert.Equal(t, "hello", getParsedValue(t, parsed, "message"))
}

func TestParse_NestedType(t *testing.T) {
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
	interp := newTestInterpreter(t, schema)
	data := []byte{0x01, 0x80, 0x00, 0x01} // version=1, flags=0x80, payload_size=256
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	assert.Equal(t, uint8(1), getParsedValue(t, parsed, "my_header", "version"))
	assert.Equal(t, uint8(0x80), getParsedValue(t, parsed, "my_header", "flags"))
	assert.Equal(t, uint16(256), getParsedValue(t, parsed, "payload_size"))
}

func TestParse_ConditionalField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "conditional_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "has_extra", Type: "u1"}, // 0 or 1
			{ID: "extra_data", Type: "u2le", IfExpr: "has_extra == 1"},
			{ID: "always_data", Type: "u1"},
		},
	}
	interp := newTestInterpreter(t, schema)

	t.Run("extra_data present", func(t *testing.T) {
		data := []byte{0x01, 0xCD, 0xAB, 0xFF} // has_extra=1, extra_data=0xABCD, always_data=0xFF
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(1), getParsedValue(t, parsed, "has_extra"))
		assert.Equal(t, uint16(0xABCD), getParsedValue(t, parsed, "extra_data"))
		assert.Equal(t, uint8(0xFF), getParsedValue(t, parsed, "always_data"))
	})

	t.Run("extra_data absent", func(t *testing.T) {
		data := []byte{0x00, 0xEE} // has_extra=0, always_data=0xEE
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(0), getParsedValue(t, parsed, "has_extra"))
		_, ok := parsed.Children["extra_data"]
		assert.False(t, ok, "extra_data should not be present")
		assert.Equal(t, uint8(0xEE), getParsedValue(t, parsed, "always_data"))
	})
}

func TestParse_RepeatedField_CountExpr(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "repeated_expr_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "count", Type: "u1"},
			{ID: "numbers", Type: "u2le", Repeat: "expr", RepeatExpr: "count"},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0x03, 0x64, 0x00, 0xC8, 0x00, 0x2C, 0x01} // count=3, numbers=[100, 200, 300]
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	assert.Equal(t, uint8(3), getParsedValue(t, parsed, "count"))

	numbersPd, ok := parsed.Children["numbers"]
	require.True(t, ok)
	require.True(t, numbersPd.IsArray)
	numbersArray, ok := numbersPd.Value.([]any)
	require.True(t, ok)
	require.Len(t, numbersArray, 3)
	assert.Equal(t, uint16(100), numbersArray[0].(*ParsedData).Value)
	assert.Equal(t, uint16(200), numbersArray[1].(*ParsedData).Value)
	assert.Equal(t, uint16(300), numbersArray[2].(*ParsedData).Value)
}

func TestParse_RepeatedField_EOS(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "repeated_eos_root", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "numbers", Type: "u1", Repeat: "eos"},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0x0A, 0x0B, 0x0C}
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	numbersPd, ok := parsed.Children["numbers"]
	require.True(t, ok)
	require.True(t, numbersPd.IsArray)
	numbersArray, ok := numbersPd.Value.([]any)
	require.True(t, ok)
	require.Len(t, numbersArray, 3)
	assert.Equal(t, uint8(0x0A), numbersArray[0].(*ParsedData).Value)
	assert.Equal(t, uint8(0x0B), numbersArray[1].(*ParsedData).Value)
	assert.Equal(t, uint8(0x0C), numbersArray[2].(*ParsedData).Value)
}

func TestParse_SwitchField(t *testing.T) {
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
				Switch: map[string]any{
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
	interp := newTestInterpreter(t, schema)

	t.Run("selects type_a", func(t *testing.T) {
		data := []byte{0x01, 0xAA} // selector=1, val_a=0xAA
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(1), getParsedValue(t, parsed, "selector"))
		assert.Equal(t, uint8(0xAA), getParsedValue(t, parsed, "data_field", "val_a"))
	})

	t.Run("selects type_b", func(t *testing.T) {
		data := []byte{0x02, 'X', 'Y'} // selector=2, val_b="XY"
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(2), getParsedValue(t, parsed, "selector"))
		assert.Equal(t, "XY", getParsedValue(t, parsed, "data_field", "val_b"))
	})

	t.Run("selects default type_a", func(t *testing.T) {
		data := []byte{0x03, 0xBB} // selector=3 (default), val_a=0xBB
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(3), getParsedValue(t, parsed, "selector"))
		assert.Equal(t, uint8(0xBB), getParsedValue(t, parsed, "data_field", "val_a"))
	})
}

func TestParse_AdHocSwitchType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "adhoc_switch_root", Endian: "le"},
		Types: map[string]Type{
			"type_x": {Seq: []SequenceItem{{ID: "val_x", Type: "s1"}}},
		},
		Seq: []SequenceItem{
			{ID: "switch_key", Type: "u1"},
			{ID: "switched_item", Type: "switch-on: switch_key > 0 ? 'type_x' : 'u2be'"},
		},
	}
	interp := newTestInterpreter(t, schema)

	t.Run("ad-hoc selects type_x", func(t *testing.T) {
		data := []byte{0x01, 0xFF} // switch_key=1, val_x=-1
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(1), getParsedValue(t, parsed, "switch_key"))
		// The switched_item itself is type_x, so its child is val_x
		assert.Equal(t, int8(-1), getParsedValue(t, parsed, "switched_item", "val_x"))
	})

	t.Run("ad-hoc selects u2be", func(t *testing.T) {
		data := []byte{0x00, 0x12, 0x34} // switch_key=0, u2be_val=0x1234
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, uint8(0), getParsedValue(t, parsed, "switch_key"))
		assert.Equal(t, uint16(0x1234), getParsedValue(t, parsed, "switched_item"))
	})
}

func TestParse_ContentsField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "contents_root"},
		Seq: []SequenceItem{
			{ID: "magic_bytes", Contents: []any{float64(0xCA), float64(0xFE), float64(0xBA), float64(0xBE)}},
			{ID: "some_data", Type: "u1"},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0xDD}
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xCA, 0xFE, 0xBA, 0xBE}, getParsedValue(t, parsed, "magic_bytes"))
	assert.Equal(t, uint8(0xDD), getParsedValue(t, parsed, "some_data"))

	// Test content mismatch
	dataMismatch := []byte{0xCA, 0xFE, 0xBA, 0x00, 0xDD} // Last byte of magic is wrong
	streamMismatch := kaitai.NewStream(bytes.NewReader(dataMismatch))
	_, errMismatch := interp.Parse(context.Background(), streamMismatch)
	require.Error(t, errMismatch)
	assert.Contains(t, errMismatch.Error(), "content validation failed")
}

func TestParse_StringField(t *testing.T) {
	t.Run("fixed size utf8", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "string_size"},
			Seq:  []SequenceItem{{ID: "msg", Type: "str", Size: 5, Encoding: "UTF-8"}},
		}
		interp := newTestInterpreter(t, schema)
		data := []byte{'h', 'e', 'l', 'l', 'o', 'w', 'o', 'r', 'l', 'd'} // "helloworld"
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, "hello", getParsedValue(t, parsed, "msg"))
	})

	t.Run("strz utf8", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "string_strz"},
			Seq:  []SequenceItem{{ID: "term_msg", Type: "strz", Encoding: "UTF-8"}},
		}
		interp := newTestInterpreter(t, schema)
		data := []byte{'w', 'o', 'r', 'l', 'd', 0x00, 'e', 'x', 't', 'r', 'a'}
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, "world", getParsedValue(t, parsed, "term_msg"))
	})

	t.Run("size_eos utf8", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "string_eos"},
			Seq:  []SequenceItem{{ID: "eos_msg", Type: "str", SizeEOS: true, Encoding: "UTF-8"}},
		}
		interp := newTestInterpreter(t, schema)
		data := []byte{'e', 'n', 'd'}
		stream := kaitai.NewStream(bytes.NewReader(data))
		parsed, err := interp.Parse(context.Background(), stream)
		require.NoError(t, err)
		assert.Equal(t, "end", getParsedValue(t, parsed, "eos_msg"))
	})
}

func TestParse_BytesField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "bytes_field_root"},
		Seq: []SequenceItem{
			{ID: "len", Type: "u1"},
			{ID: "raw_data", Type: "bytes", Size: "len"},
			{ID: "eos_data", Type: "bytes", SizeEOS: true},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0x03, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE} // len=3, raw_data=[AA,BB,CC], eos_data=[DD,EE]
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	assert.Equal(t, uint8(3), getParsedValue(t, parsed, "len"))
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, getParsedValue(t, parsed, "raw_data"))
	assert.Equal(t, []byte{0xDD, 0xEE}, getParsedValue(t, parsed, "eos_data"))
}

func TestParse_ProcessField_XOR(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "process_xor_root"},
		Seq: []SequenceItem{
			{ID: "key", Type: "u1"}, // The XOR key
			{ID: "data_len", Type: "u1"},
			// The data in the stream is ALREADY XORed. We process it to get the logical value.
			{ID: "processed_payload", Type: "payload_type", Size: "data_len", Process: "xor(key)"},
		},
		Types: map[string]Type{
			"payload_type": { // This is the type of the *logical*, un-processed data
				Seq: []SequenceItem{
					{ID: "field1", Type: "u1"},
					{ID: "field2", Type: "u1"},
				},
			},
		},
	}
	interp := newTestInterpreter(t, schema)

	key := byte(0xAA)
	f1Logical := byte(0x11)
	f2Logical := byte(0x22)
	f1Processed := f1Logical ^ key // 0xBB
	f2Processed := f2Logical ^ key // 0x88

	// Data in stream: key, len, processed_f1, processed_f2
	data := []byte{key, 0x02, f1Processed, f2Processed}
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	assert.Equal(t, key, getParsedValue(t, parsed, "key"))
	assert.Equal(t, uint8(2), getParsedValue(t, parsed, "data_len"))
	assert.Equal(t, f1Logical, getParsedValue(t, parsed, "processed_payload", "field1"), "Field1 should be logical value")
	assert.Equal(t, f2Logical, getParsedValue(t, parsed, "processed_payload", "field2"), "Field2 should be logical value")
}

func TestParse_BuiltinTypesWithEndianMeta(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "builtins_meta_endian", Endian: "le"}, // Default to LE
		Seq: []SequenceItem{
			{ID: "val_u2_meta", Type: "u2"}, // Should use meta endian (le)
			{ID: "val_s4_meta", Type: "s4"}, // Should use meta endian (le)
			{ID: "val_f8_meta", Type: "f8"}, // Should use meta endian (le)
			{ID: "val_u2_be", Type: "u2be"}, // Explicitly BE
		},
	}
	interp := newTestInterpreter(t, schema)
	// u2_meta (le): 22 11 (0x1122)
	// s4_meta (le): DD CC BB AA (0xAABBCCDD)
	// f8_meta (le): 00 00 00 00 00 00 06 40 (2.75)
	// u2_be:        33 44 (0x3344)
	data := []byte{
		0x22, 0x11,
		0xDD, 0xCC, 0xBB, 0xAA,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x40,
		0x33, 0x44,
	}
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	assert.Equal(t, uint16(0x1122), getParsedValue(t, parsed, "val_u2_meta"))
	assert.Equal(t, int32(0xAABBCCDD), getParsedValue(t, parsed, "val_s4_meta"))
	assert.Equal(t, float64(2.75), getParsedValue(t, parsed, "val_f8_meta"))
	assert.Equal(t, uint16(0x3344), getParsedValue(t, parsed, "val_u2_be"))
}

func TestParse_Instances(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "instance_root"},
		Seq: []SequenceItem{
			{ID: "value1", Type: "u1"},
			{ID: "value2", Type: "u1"},
		},
		Instances: map[string]InstanceDef{
			"sum_val":      {Value: "value1 + value2"},
			"product_val":  {Value: "value1 * value2", Type: "u2le"}, // Type here is for info, not parsing
			"is_value1_gt": {Value: "value1 > 10"},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0x05, 0x0A} // value1=5, value2=10
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	assert.Equal(t, uint8(5), getParsedValue(t, parsed, "value1"))
	assert.Equal(t, uint8(10), getParsedValue(t, parsed, "value2"))

	// Instances are evaluated and added to the children of the current type/root
	assert.EqualValues(t, 15, getParsedValue(t, parsed, "sum_val"))     // 5 + 10
	assert.EqualValues(t, 50, getParsedValue(t, parsed, "product_val")) // 5 * 10
	assert.Equal(t, false, getParsedValue(t, parsed, "is_value1_gt"))   // 5 > 10 is false
}

func TestParse_ErrorHandling(t *testing.T) {
	t.Run("circular dependency", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "circular_root"},
			Types: map[string]Type{
				"type_a": {Seq: []SequenceItem{{ID: "b", Type: "type_b"}}},
				"type_b": {Seq: []SequenceItem{{ID: "a", Type: "type_a"}}},
			},
			Seq: []SequenceItem{{ID: "entry", Type: "type_a"}},
		}
		interp := newTestInterpreter(t, schema)
		stream := kaitai.NewStream(bytes.NewReader([]byte{}))
		_, err := interp.Parse(context.Background(), stream)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular type dependency detected")
	})

	t.Run("unknown type", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "unknown_type_root"},
			Seq:  []SequenceItem{{ID: "field1", Type: "non_existent_type"}},
		}
		interp := newTestInterpreter(t, schema)
		stream := kaitai.NewStream(bytes.NewReader([]byte{1, 2, 3}))
		_, err := interp.Parse(context.Background(), stream)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown type: non_existent_type")
	})

	t.Run("eof during read", func(t *testing.T) {
		schema := &KaitaiSchema{
			Meta: Meta{ID: "eof_root"},
			Seq:  []SequenceItem{{ID: "val", Type: "u4le"}},
		}
		interp := newTestInterpreter(t, schema)
		stream := kaitai.NewStream(bytes.NewReader([]byte{1, 2})) // Not enough bytes for u4le
		_, err := interp.Parse(context.Background(), stream)
		require.Error(t, err)
		assert.ErrorIs(t, err, io.EOF) // Kaitai runtime wraps EOF
	})
}

func TestParseContext_AsActivation(t *testing.T) {
	rootChildren := map[string]any{"root_field": "root_val"}
	parentChildren := map[string]any{"parent_field": "parent_val", "common_field": "parent_common"}
	currentChildren := map[string]any{"current_field": 123, "common_field": "current_common"}
	mockIO := kaitai.NewStream(bytes.NewReader(nil))

	rootCtx := &ParseContext{Children: rootChildren, IO: mockIO}
	rootCtx.Root = rootCtx

	parentPCtx := &ParseContext{Children: parentChildren, IO: mockIO, Root: rootCtx, Parent: rootCtx}
	pCtx := &ParseContext{Children: currentChildren, IO: mockIO, Root: rootCtx, Parent: parentPCtx}

	act, err := pCtx.AsActivation()
	require.NoError(t, err)

	// Check current fields (should take precedence)
	val, found := act.ResolveName("current_field")
	require.True(t, found)
	assert.EqualValues(t, 123, val.Value())

	val, found = act.ResolveName("common_field")
	require.True(t, found)
	assert.EqualValues(t, "current_common", val.Value()) // Current overrides parent

	// Check _io
	val, found = act.ResolveName("_io")
	require.True(t, found)
	assert.Same(t, mockIO, val.Value())

	// Check _root
	val, found = act.ResolveName("_root")
	require.True(t, found)
	assert.Equal(t, rootChildren, val.Value())

	// Check _parent
	val, found = act.ResolveName("_parent")
	require.True(t, found)
	assert.Equal(t, parentChildren, val.Value())

	// Check field from parent (not overridden by current)
	val, found = act.ResolveName("parent_field")
	require.True(t, found)
	assert.EqualValues(t, "parent_val", val.Value())
}

func TestParsedDataToMap(t *testing.T) {
	pd := &ParsedData{
		Type: "root",
		Children: map[string]*ParsedData{
			"field_int": {Type: "u1", Value: uint8(10)},
			"field_str": {Type: "str", Value: "hello"},
			"field_nested": {
				Type: "nested_type",
				Children: map[string]*ParsedData{
					"sub_field": {Type: "u2le", Value: uint16(100)},
				},
			},
			"field_array": {
				Type:    "u1",
				IsArray: true,
				Value: []any{
					&ParsedData{Type: "u1", Value: uint8(1)},
					&ParsedData{Type: "u1", Value: uint8(2)},
				},
			},
		},
	}

	expectedMap := map[string]any{
		"field_int": 10, // uint8(10) will be converted by ParsedDataToMap
		"field_str": "hello",
		"field_nested": map[string]any{
			"sub_field": 100, // uint16(100)
		},
		"field_array": []any{
			1, // uint8(1)
			2, // uint8(2)
		},
	}
	// Note: The direct values (uint8, uint16) are used in expectedMap because ParsedDataToMap
	// recursively calls itself and for primitive types (no children), it returns data.Value directly.
	// JSON marshaling would handle the number types appropriately.

	actualMap := ParsedDataToMap(pd)
	assert.EqualValues(t, expectedMap, actualMap) // Use EqualValues due to potential number type differences (e.g. int vs uint8)
}

func TestParse_RootTypeSpecifiedInMeta(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "actual_root_id", RootType: "my_real_root", Endian: "le"},
		Types: map[string]Type{
			"my_real_root": {
				Seq: []SequenceItem{
					{ID: "data_field", Type: "u2le"},
				},
			},
			"unused_type": { // This type should not be parsed as root
				Seq: []SequenceItem{
					{ID: "other_field", Type: "u1"},
				},
			},
		},
		// Seq at top level of schema is ignored if RootType is specified
		Seq: []SequenceItem{
			{ID: "ignored_field", Type: "u4le"},
		},
	}
	interp := newTestInterpreter(t, schema)
	data := []byte{0x34, 0x12} // data_field = 0x1234
	stream := kaitai.NewStream(bytes.NewReader(data))

	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "my_real_root", parsed.Type) // Check that the correct root type was parsed

	assert.Equal(t, uint16(0x1234), getParsedValue(t, parsed, "data_field"))
	_, ok := parsed.Children["ignored_field"]
	assert.False(t, ok, "Top-level seq field should be ignored when RootType is set")
	_, ok = parsed.Children["other_field"]
	assert.False(t, ok, "Unused type field should not be present")
}

func ExampleParsedDataToMap() {
	pd := &ParsedData{
		Type: "root",
		Children: map[string]*ParsedData{
			"field_int": {Type: "u1", Value: uint8(10)},
			"field_str": {Type: "str", Value: "hello"},
			"field_nested": {
				Type: "nested_type",
				Children: map[string]*ParsedData{
					"sub_field": {Type: "u2le", Value: uint16(100)},
				},
			},
			"field_array": {
				Type:    "u1",
				IsArray: true,
				Value: []any{
					&ParsedData{Type: "u1", Value: uint8(1)},
					&ParsedData{Type: "u1", Value: uint8(2)},
				},
			},
		},
	}
	fmt.Printf("%#v\n", ParsedDataToMap(pd))
	// Output:
	// map[string]interface {}{"field_array":[]interface {}{0x1, 0x2}, "field_int":0xa, "field_nested":map[string]interface {}{"sub_field":0x64}, "field_str":"hello"}
}
