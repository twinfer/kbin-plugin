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

// TestSimpleHighImpact focuses on simpler but high-impact scenarios to reach 85% coverage
func TestSimpleHighImpact_RepeatEosAndIfConditions(t *testing.T) {
	// Test repeat-eos and if conditions which are commonly used patterns
	schema := &KaitaiSchema{
		Meta: Meta{ID: "repeat_eos_test"},
		Seq: []SequenceItem{
			{ID: "header", Type: "u1"},
			{ID: "items", Type: "u1", Repeat: "eos"},
		},
		Instances: map[string]InstanceDef{
			"item_count": {Value: "items.length"},
			"header_plus_count": {Value: "header + item_count"},
			"has_items": {Value: "item_count > 0"},
		},
	}

	interp := newSimpleTestInterpreter(t, schema)

	data := []byte{0xFF, 1, 2, 3, 4, 5} // header + 5 items
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	header := getSimpleParsedValue(t, parsed, "header")
	assert.Equal(t, int64(0xFF), header)

	items := getSimpleParsedValue(t, parsed, "items")
	if itemArray, ok := items.([]interface{}); ok {
		assert.Len(t, itemArray, 5)
		// Array items are wrapped in ParsedData, so we need to extract from them
		if item0, ok := itemArray[0].(*ParsedData); ok {
			assert.Equal(t, int64(1), extractValue(item0.Value))
		}
		if item4, ok := itemArray[4].(*ParsedData); ok {
			assert.Equal(t, int64(5), extractValue(item4.Value))
		}
	}

	// Test instances
	itemCount := getSimpleParsedValue(t, parsed, "item_count")
	headerPlusCount := getSimpleParsedValue(t, parsed, "header_plus_count")
	hasItems := getSimpleParsedValue(t, parsed, "has_items")

	assert.Equal(t, int64(5), itemCount)
	assert.Equal(t, int64(260), headerPlusCount) // 0xFF + 5 = 260
	assert.Equal(t, true, hasItems)
}

func TestSimpleHighImpact_ConditionalFields(t *testing.T) {
	// Test conditional fields (if expressions)
	schema := &KaitaiSchema{
		Meta: Meta{ID: "conditional_test"},
		Seq: []SequenceItem{
			{ID: "flags", Type: "u1"},
			{ID: "optional_data", Type: "u2le", IfExpr: "flags & 0x01"},
			{ID: "another_optional", Type: "str", Size: 4, IfExpr: "flags & 0x02"},
		},
		Instances: map[string]InstanceDef{
			"has_data": {Value: "(flags & 0x01) != 0"},
			"has_string": {Value: "(flags & 0x02) != 0"},
		},
	}

	interp := newSimpleTestInterpreter(t, schema)

	tests := []struct {
		name string
		flags byte
		data []byte
		expectData bool
		expectString bool
	}{
		{
			name: "no optional fields",
			flags: 0x00,
			data: []byte{0x00},
			expectData: false,
			expectString: false,
		},
		{
			name: "only data field",
			flags: 0x01,
			data: []byte{0x01, 0x34, 0x12},
			expectData: true,
			expectString: false,
		},
		{
			name: "only string field",
			flags: 0x02,
			data: []byte{0x02, 't', 'e', 's', 't'},
			expectData: false,
			expectString: true,
		},
		{
			name: "both fields",
			flags: 0x03,
			data: []byte{0x03, 0x34, 0x12, 't', 'e', 's', 't'},
			expectData: true,
			expectString: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stream := kaitai.NewStream(bytes.NewReader(test.data))
			parsed, err := interp.Parse(context.Background(), stream)
			require.NoError(t, err)

			flags := getSimpleParsedValue(t, parsed, "flags")
			assert.Equal(t, int64(test.flags), flags)

			hasData := getSimpleParsedValue(t, parsed, "has_data")
			hasString := getSimpleParsedValue(t, parsed, "has_string")
			assert.Equal(t, test.expectData, hasData)
			assert.Equal(t, test.expectString, hasString)

			// Check optional fields exist when expected
			if test.expectData {
				optionalData := getSimpleParsedValue(t, parsed, "optional_data")
				assert.Equal(t, int64(0x1234), optionalData)
			}

			if test.expectString {
				anotherOptional := getSimpleParsedValue(t, parsed, "another_optional")
				assert.Equal(t, "test", anotherOptional)
			}
		})
	}
}

func TestSimpleHighImpact_NestedTypes(t *testing.T) {
	// Test nested types with instances
	schema := &KaitaiSchema{
		Meta: Meta{ID: "nested_test"},
		Seq: []SequenceItem{
			{ID: "count", Type: "u1"},
			{ID: "records", Type: "record", Repeat: "expr", RepeatExpr: "count"},
		},
		Types: map[string]Type{
			"record": {
				Seq: []SequenceItem{
					{ID: "id", Type: "u1"},
					{ID: "value", Type: "u2le"},
				},
				Instances: map[string]InstanceDef{
					"is_special": {Value: "id == 0xFF"},
					"doubled_value": {Value: "value * 2"},
					"id_plus_value": {Value: "id + value"},
				},
			},
		},
	}

	interp := newSimpleTestInterpreter(t, schema)

	data := []byte{
		2,          // count
		0xAA, 0x34, 0x12, // record 1: id=0xAA, value=0x1234
		0xFF, 0x78, 0x56, // record 2: id=0xFF, value=0x5678
	}

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	count := getSimpleParsedValue(t, parsed, "count")
	assert.Equal(t, int64(2), count)

	records := getSimpleParsedValue(t, parsed, "records")
	if recordArray, ok := records.([]interface{}); ok {
		assert.Len(t, recordArray, 2)

		// Check first record
		if rec1, ok := recordArray[0].(*ParsedData); ok {
			id1 := getSimpleParsedValue(t, rec1, "id")
			value1 := getSimpleParsedValue(t, rec1, "value")
			isSpecial1 := getSimpleParsedValue(t, rec1, "is_special")
			doubled1 := getSimpleParsedValue(t, rec1, "doubled_value")
			idPlusValue1 := getSimpleParsedValue(t, rec1, "id_plus_value")

			assert.Equal(t, int64(0xAA), id1)
			assert.Equal(t, int64(0x1234), value1)
			assert.Equal(t, false, isSpecial1)
			assert.Equal(t, int64(0x1234*2), doubled1)
			assert.Equal(t, int64(0xAA+0x1234), idPlusValue1)
		}

		// Check second record
		if rec2, ok := recordArray[1].(*ParsedData); ok {
			id2 := getSimpleParsedValue(t, rec2, "id")
			value2 := getSimpleParsedValue(t, rec2, "value")
			isSpecial2 := getSimpleParsedValue(t, rec2, "is_special")
			doubled2 := getSimpleParsedValue(t, rec2, "doubled_value")
			idPlusValue2 := getSimpleParsedValue(t, rec2, "id_plus_value")

			assert.Equal(t, int64(0xFF), id2)
			assert.Equal(t, int64(0x5678), value2)
			assert.Equal(t, true, isSpecial2)
			assert.Equal(t, int64(0x5678*2), doubled2)
			assert.Equal(t, int64(0xFF+0x5678), idPlusValue2)
		}
	}
}

func TestSimpleHighImpact_StringProcessing(t *testing.T) {
	// Test string processing and encoding
	schema := &KaitaiSchema{
		Meta: Meta{ID: "string_test", Encoding: "UTF-8"},
		Seq: []SequenceItem{
			{ID: "str_len", Type: "u1"},
			{ID: "text", Type: "str", Size: "str_len"},
			{ID: "fixed_text", Type: "str", Size: 5},
		},
		Instances: map[string]InstanceDef{
			"text_len": {Value: "text.length"},
			"combined_length": {Value: "text.length + fixed_text.length"},
			"is_hello": {Value: "text == \"hello\""},
		},
	}

	interp := newSimpleTestInterpreter(t, schema)

	text := "hello"
	fixedText := "world"
	data := []byte{byte(len(text))}
	data = append(data, []byte(text)...)
	data = append(data, []byte(fixedText)...)

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	strLen := getSimpleParsedValue(t, parsed, "str_len")
	textField := getSimpleParsedValue(t, parsed, "text")
	fixedTextField := getSimpleParsedValue(t, parsed, "fixed_text")

	assert.Equal(t, int64(5), strLen)
	assert.Equal(t, "hello", textField)
	assert.Equal(t, "world", fixedTextField)

	// Test instances
	textLen := getSimpleParsedValue(t, parsed, "text_len")
	combinedLength := getSimpleParsedValue(t, parsed, "combined_length")
	isHello := getSimpleParsedValue(t, parsed, "is_hello")

	assert.Equal(t, int64(5), textLen)
	assert.Equal(t, int64(10), combinedLength)
	assert.Equal(t, true, isHello)
}

func TestSimpleHighImpact_BytesProcessing(t *testing.T) {
	// Test bytes processing and size calculations
	schema := &KaitaiSchema{
		Meta: Meta{ID: "bytes_test"},
		Seq: []SequenceItem{
			{ID: "len1", Type: "u1"},
			{ID: "len2", Type: "u1"},
			{ID: "data1", Type: "bytes", Size: "len1"},
			{ID: "data2", Type: "bytes", Size: "len2"},
		},
		Instances: map[string]InstanceDef{
			"total_length": {Value: "len1 + len2"},
			"total_data_size": {Value: "data1.length + data2.length"},
			"sizes_match": {Value: "total_length == total_data_size"},
			"has_data": {Value: "data1.length > 0 && data2.length > 0"},
		},
	}

	interp := newSimpleTestInterpreter(t, schema)

	data1 := []byte{0xAA, 0xBB, 0xCC}
	data2 := []byte{0xDD, 0xEE}

	data := []byte{
		byte(len(data1)), // len1
		byte(len(data2)), // len2
	}
	data = append(data, data1...)
	data = append(data, data2...)

	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	len1 := getSimpleParsedValue(t, parsed, "len1")
	len2 := getSimpleParsedValue(t, parsed, "len2")
	data1Field := getSimpleParsedValue(t, parsed, "data1")
	data2Field := getSimpleParsedValue(t, parsed, "data2")

	assert.Equal(t, int64(3), len1)
	assert.Equal(t, int64(2), len2)
	assert.Equal(t, data1, data1Field)
	assert.Equal(t, data2, data2Field)

	// Test instances
	totalLength := getSimpleParsedValue(t, parsed, "total_length")
	totalDataSize := getSimpleParsedValue(t, parsed, "total_data_size")
	sizesMatch := getSimpleParsedValue(t, parsed, "sizes_match")
	hasData := getSimpleParsedValue(t, parsed, "has_data")

	assert.Equal(t, int64(5), totalLength)
	assert.Equal(t, int64(5), totalDataSize)
	assert.Equal(t, true, sizesMatch)
	assert.Equal(t, true, hasData)
}

func TestSimpleHighImpact_ArithmeticAndComparisons(t *testing.T) {
	// Test arithmetic operations and comparisons in instances
	schema := &KaitaiSchema{
		Meta: Meta{ID: "arithmetic_test"},
		Seq: []SequenceItem{
			{ID: "a", Type: "u1"},
			{ID: "b", Type: "u1"},
			{ID: "c", Type: "u1"},
		},
		Instances: map[string]InstanceDef{
			"sum": {Value: "a + b + c"},
			"product": {Value: "a * b"},
			"difference": {Value: "a - b"},
			"quotient": {Value: "c / 2"},
			"modulo": {Value: "c % 3"},
			"max_ab": {Value: "a > b ? a : b"},
			"all_equal": {Value: "a == b && b == c"},
			"any_zero": {Value: "a == 0 || b == 0 || c == 0"},
			"in_range": {Value: "a >= 10 && a <= 20"},
		},
	}

	interp := newSimpleTestInterpreter(t, schema)

	data := []byte{15, 10, 12} // a=15, b=10, c=12
	stream := kaitai.NewStream(bytes.NewReader(data))
	parsed, err := interp.Parse(context.Background(), stream)
	require.NoError(t, err)

	a := getSimpleParsedValue(t, parsed, "a")
	b := getSimpleParsedValue(t, parsed, "b")
	c := getSimpleParsedValue(t, parsed, "c")

	assert.Equal(t, int64(15), a)
	assert.Equal(t, int64(10), b)
	assert.Equal(t, int64(12), c)

	// Test arithmetic instances
	sum := getSimpleParsedValue(t, parsed, "sum")
	product := getSimpleParsedValue(t, parsed, "product")
	difference := getSimpleParsedValue(t, parsed, "difference")
	quotient := getSimpleParsedValue(t, parsed, "quotient")
	modulo := getSimpleParsedValue(t, parsed, "modulo")
	maxAb := getSimpleParsedValue(t, parsed, "max_ab")
	allEqual := getSimpleParsedValue(t, parsed, "all_equal")
	anyZero := getSimpleParsedValue(t, parsed, "any_zero")
	inRange := getSimpleParsedValue(t, parsed, "in_range")

	assert.Equal(t, int64(37), sum)      // 15 + 10 + 12
	assert.Equal(t, int64(150), product) // 15 * 10
	assert.Equal(t, int64(5), difference) // 15 - 10
	assert.Equal(t, int64(6), quotient)   // 12 / 2
	assert.Equal(t, int64(0), modulo)     // 12 % 3
	assert.Equal(t, int64(15), maxAb)     // max(15, 10)
	assert.Equal(t, false, allEqual)      // not all equal
	assert.Equal(t, false, anyZero)       // no zeros
	assert.Equal(t, true, inRange)        // 15 is in [10, 20]
}

// Helper functions
func newSimpleTestInterpreter(t *testing.T, schema *KaitaiSchema) *KaitaiInterpreter {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interp, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)
	return interp
}

func getSimpleParsedValue(t *testing.T, data *ParsedData, key string) interface{} {
	if child, exists := data.Children[key]; exists {
		return extractValue(child.Value)
	}
	t.Errorf("Key '%s' not found in parsed data", key)
	return nil
}

func extractValue(value interface{}) interface{} {
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
	default:
		return value
	}
}