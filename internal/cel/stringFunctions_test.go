package cel

import (
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a CEL String
func celString(s string) ref.Val {
	return types.String(s)
}

// Helper to create CEL Bytes
func celBytes(b []byte) ref.Val {
	return types.Bytes(b)
}

// Helper to create a CEL Int
func celInt(i int64) ref.Val {
	return types.Int(i)
}

// Helper to create a CEL List
func celList(adapter *types.Registry, elements ...ref.Val) ref.Val {
	return types.NewMutableList(adapter, elements)
}

// Helper to create a CEL Map
func celMap(adapter *types.Registry, keyValues map[ref.Val]ref.Val) ref.Val {
	return types.NewRefValMap(adapter, keyValues)
}


func TestStringFunctions_ToString(t *testing.T) {
	lib := &stringLib{}
	fn := lib.CompileOptions()[0].Functions()[0] // Assuming to_s is the first function
	require.NotNil(t, fn)
	require.Len(t, fn.Overloads(), 1)
	overload := fn.Overloads()[0]
	impl := overload.UnaryBinding()
	require.NotNil(t, impl)

	tests := []struct {
		name     string
		input    ref.Val
		expected ref.Val
	}{
		{"StringInput", celString("hello"), celString("hello")},
		{"IntInput", celInt(123), celString("123")},
		{"BoolInputTrue", types.True, celString("true")},
		{"BoolInputFalse", types.False, celString("false")},
		{"NullInput", types.NullValue, celString("<nil>")}, // fmt.Sprintf("%v", nil) gives "<nil>"
		{"DoubleInput", types.Double(123.45), celString("123.45")},
		{"BytesInput", celBytes([]byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}), celString("[104 101 108 108 111]")}, // fmt.Sprintf("%v", []byte)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := impl(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringFunctions_Reverse(t *testing.T) {
	lib := &stringLib{}
	fn := lib.CompileOptions()[0].Functions()[1] // Assuming reverse is the second function
	require.NotNil(t, fn)
	require.Len(t, fn.Overloads(), 1)
	overload := fn.Overloads()[0]
	impl := overload.UnaryBinding()
	require.NotNil(t, impl)

	tests := []struct {
		name     string
		input    ref.Val
		expected ref.Val
	}{
		{"EmptyString", celString(""), celString("")},
		{"SimpleString", celString("hello"), celString("olleh")},
		{"Palindrome", celString("madam"), celString("madam")},
		{"WithSpaces", celString("hello world"), celString("dlrow olleh")},
		{"Unicode", celString("résumé"), celString("émusér")}, // Kaitai StringReverse handles unicode
		{"InvalidType", celInt(123), types.NewErr("expected string type for reverse")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := impl(tt.input)
			assert.Equal(t, tt.expected.Value(), result.Value()) // Compare underlying values for errors
		})
	}
}

func TestStringFunctions_Length(t *testing.T) {
	lib := &stringLib{}
	fn := lib.CompileOptions()[0].Functions()[2] // Assuming length is the third function
	require.NotNil(t, fn)
	require.Len(t, fn.Overloads(), 4) // String, Bytes, List, Map

	reg := types.NewEmptyRegistry() // For creating list/map types

	tests := []struct {
		name        string
		overloadIdx int // 0: string, 1: bytes, 2: list, 3: map
		input       ref.Val
		expected    ref.Val
	}{
		// String overloads
		{"EmptyString", 0, celString(""), celInt(0)},
		{"SimpleString", 0, celString("hello"), celInt(5)},
		{"UnicodeString", 0, celString("résumé"), celInt(6)}, // Length in runes
		{"InvalidTypeForStringLength", 0, celInt(123), types.NewErr("expected string type for length")},

		// Bytes overloads
		{"EmptyBytes", 1, celBytes([]byte{}), celInt(0)},
		{"SimpleBytes", 1, celBytes([]byte{1, 2, 3}), celInt(3)},
		{"InvalidTypeForBytesLength", 1, celString("abc"), types.NewErr("expected bytes type for length")},
		
		// List overloads
		{"EmptyList", 2, celList(reg), celInt(0)},
		{"SimpleList", 2, celList(reg, celInt(1), celString("a"), types.True), celInt(3)},
		{"InvalidTypeForListLength", 2, celString("abc"), types.NewErr("expected list type for length")},

		// Map overloads
		{"EmptyMap", 3, celMap(reg, map[ref.Val]ref.Val{}), celInt(0)},
		{"SimpleMap", 3, celMap(reg, map[ref.Val]ref.Val{celString("a"): celInt(1), celString("b"): celInt(2)}), celInt(2)},
		{"InvalidTypeForMapLength", 3, celString("abc"), types.NewErr("expected map type for length")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overload := fn.Overloads()[tt.overloadIdx]
			impl := overload.UnaryBinding()
			require.NotNil(t, impl)
			
			result := impl(tt.input)
			assert.Equal(t, tt.expected.Value(), result.Value()) // Compare underlying values for errors and numbers
		})
	}
}
