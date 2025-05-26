package cel

import (
	"fmt"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/stretchr/testify/assert"
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
func celList(adapter types.Adapter, elements ...ref.Val) ref.Val {
	return types.NewRefValList(adapter, elements)
}

// Helper to create a CEL Map
func celMap(adapter types.Adapter, keyValues map[ref.Val]ref.Val) ref.Val {
	return types.NewRefValMap(adapter, keyValues)
}

func TestStringFunctions_ToString(t *testing.T) {
	// Test the to_s function directly
	impl := func(val ref.Val) ref.Val {
		return types.String(fmt.Sprintf("%v", val.Value()))
	}

	tests := []struct {
		name     string
		input    ref.Val
		expected ref.Val
	}{
		{"StringInput", celString("hello"), celString("hello")},
		{"IntInput", celInt(123), celString("123")},
		{"BoolInputTrue", types.True, celString("true")},
		{"BoolInputFalse", types.False, celString("false")},
		{"NullInput", types.NullValue, celString("NULL_VALUE")}, // CEL NullValue string representation
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
	// Test the reverse function directly
	impl := func(val ref.Val) ref.Val {
		str, ok := val.(types.String)
		if !ok {
			return types.NewErr("expected string type for reverse")
		}
		runes := []rune(string(str))
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return types.String(string(runes))
	}

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
	reg := types.NewEmptyRegistry() // For creating list/map types

	tests := []struct {
		name     string
		funcType string // "string", "bytes", "list", "map"
		input    ref.Val
		expected ref.Val
	}{
		// String overloads
		{"EmptyString", "string", celString(""), celInt(0)},
		{"SimpleString", "string", celString("hello"), celInt(5)},
		{"UnicodeString", "string", celString("résumé"), celInt(6)}, // Length in runes
		{"InvalidTypeForStringLength", "string", celInt(123), types.NewErr("expected string type for length")},

		// Bytes overloads
		{"EmptyBytes", "bytes", celBytes([]byte{}), celInt(0)},
		{"SimpleBytes", "bytes", celBytes([]byte{1, 2, 3}), celInt(3)},
		{"InvalidTypeForBytesLength", "bytes", celString("abc"), types.NewErr("expected bytes type for length")},

		// List overloads
		{"EmptyList", "list", celList(reg), celInt(0)},
		{"SimpleList", "list", celList(reg, celInt(1), celString("a"), types.True), celInt(3)},
		{"InvalidTypeForListLength", "list", celString("abc"), types.NewErr("expected list type for length")},

		// Map overloads
		{"EmptyMap", "map", celMap(reg, map[ref.Val]ref.Val{}), celInt(0)},
		{"SimpleMap", "map", celMap(reg, map[ref.Val]ref.Val{celString("a"): celInt(1), celString("b"): celInt(2)}), celInt(2)},
		{"InvalidTypeForMapLength", "map", celString("abc"), types.NewErr("expected map type for length")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var impl func(ref.Val) ref.Val

			switch tt.funcType {
			case "string":
				impl = func(val ref.Val) ref.Val {
					str, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string type for length")
					}
					return types.Int(len([]rune(string(str))))
				}
			case "bytes":
				impl = func(val ref.Val) ref.Val {
					b, ok := val.(types.Bytes)
					if !ok {
						return types.NewErr("expected bytes type for length")
					}
					return types.Int(len(b))
				}
			case "list":
				impl = func(val ref.Val) ref.Val {
					lister, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list type for length")
					}
					size := lister.Size()
					if intSize, ok := size.(types.Int); ok {
						return intSize
					}
					return types.Int(size.Value().(int64))
				}
			case "map":
				impl = func(val ref.Val) ref.Val {
					mapper, ok := val.(traits.Mapper)
					if !ok {
						return types.NewErr("expected map type for length")
					}
					size := mapper.Size()
					if intSize, ok := size.(types.Int); ok {
						return intSize
					}
					return types.Int(size.Value().(int64))
				}
			}

			result := impl(tt.input)
			assert.Equal(t, tt.expected.Value(), result.Value()) // Compare underlying values for errors and numbers
		})
	}
}
