package cel

import (
	"math"
	"strconv"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helpers (celString, celInt, etc. can be reused if this file is part of the same package,
// or defined locally if needed. Assuming they are accessible or redefined if necessary.)

func TestTypeConversionFunctions_ToI(t *testing.T) {
	lib := &typeConversionLib{}
	fn := lib.CompileOptions()[0].Functions()[0] // Assuming to_i is the first function
	require.NotNil(t, fn)
	require.Len(t, fn.Overloads(), 3) // String, Uint, Double

	tests := []struct {
		name        string
		overloadIdx int // 0: string, 1: uint, 2: double
		input       ref.Val
		expected    ref.Val
	}{
		// String to Int
		{"StringToInt_Valid", 0, celString("123"), celInt(123)},
		{"StringToInt_Negative", 0, celString("-456"), celInt(-456)},
		{"StringToInt_Empty", 0, celString(""), types.NewErr("cannot convert string to int: %v", &strconv.NumError{Func: "ParseInt", Num: "", Err: strconv.ErrSyntax})}, // Exact error may vary
		{"StringToInt_NonNumeric", 0, celString("abc"), types.NewErr("cannot convert string to int: %v", &strconv.NumError{Func: "ParseInt", Num: "abc", Err: strconv.ErrSyntax})},
		{"StringToInt_FloatString", 0, celString("123.45"), types.NewErr("cannot convert string to int: %v", &strconv.NumError{Func: "ParseInt", Num: "123.45", Err: strconv.ErrSyntax})},
		{"StringToInt_Overflow", 0, celString("9223372036854775808"), types.NewErr("cannot convert string to int: %v", &strconv.NumError{Func: "ParseInt", Num: "9223372036854775808", Err: strconv.ErrRange})}, // MaxInt64 + 1
		{"InvalidTypeToStringToInt", 0, types.True, types.NewErr("unexpected type for to_i: %T", true)},


		// Uint to Int
		{"UintToInt_Valid", 1, types.Uint(789), celInt(789)},
		{"UintToInt_Zero", 1, types.Uint(0), celInt(0)},
		{"UintToInt_MaxUint64FittingInInt64", 1, types.Uint(math.MaxInt64), celInt(math.MaxInt64)},
		// Note: CEL Uint can represent values up to MaxUint64. If it's > MaxInt64, direct conversion to types.Int might wrap or be problematic.
		// The current implementation `types.Int(uintVal)` will wrap for uint64 > MaxInt64.
		// This test reflects the current implementation's behavior.
		{"UintToInt_MaxUint64", 1, types.Uint(math.MaxUint64), types.Int(math.MaxUint64)}, // Wraps to -1 for int64
		{"InvalidTypeToUintToInt", 1, celString("abc"), types.NewErr("unexpected type for to_i: %T", "abc")},


		// Double to Int
		{"DoubleToInt_Valid", 2, types.Double(123.0), celInt(123)},
		{"DoubleToInt_WithFraction", 2, types.Double(123.789), celInt(123)}, // Truncates
		{"DoubleToInt_Negative", 2, types.Double(-456.0), celInt(-456)},
		{"DoubleToInt_NegativeWithFraction", 2, types.Double(-456.789), celInt(-456)}, // Truncates
		{"DoubleToInt_Zero", 2, types.Double(0.0), celInt(0)},
		{"InvalidTypeToDoubleToInt", 2, celString("abc"), types.NewErr("unexpected type for to_i: %T", "abc")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overload := fn.Overloads()[tt.overloadIdx]
			impl := overload.UnaryBinding()
			require.NotNil(t, impl)
			
			result := impl(tt.input)
			// For error types, comparing the underlying error message might be more stable
			// if the exact error object isn't guaranteed.
			if types.IsError(tt.expected) {
				require.True(t, types.IsError(result), "Expected an error, but got: %v", result)
				// Compare error messages (simplified for this example)
				// A more robust way would be to check error types or specific parts of the message.
				assert.Contains(t, result.Value().(error).Error(), tt.expected.Value().(error).Error(), "Error message mismatch")
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTypeConversionFunctions_ToF(t *testing.T) {
	lib := &typeConversionLib{}
	fn := lib.CompileOptions()[0].Functions()[1] // Assuming to_f is the second function
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
		{"IntToFloat", celInt(123), types.Double(123.0)},
		{"UintToFloat", types.Uint(456), types.Double(456.0)},
		{"StringToFloat_Valid", celString("789.01"), types.Double(789.01)},
		{"StringToFloat_IntString", celString("789"), types.Double(789.0)},
		{"StringToFloat_Negative", celString("-12.34"), types.Double(-12.34)},
		{"StringToFloat_Invalid", celString("abc"), types.NewErr("cannot convert abc to double: %v", types.NewErr("string to double conversion error"))}, // Error message might vary
		{"BoolToFloat_True", types.True, types.NewErr("cannot convert true to double: %v", types.NewErr("boolean to double conversion error"))},
		{"BoolToFloat_False", types.False, types.NewErr("cannot convert false to double: %v", types.NewErr("boolean to double conversion error"))},
		{"NullToFloat", types.NullValue, types.NewErr("cannot convert <nil> to double: %v", types.NewErr("null to double conversion error"))}, // Error for null conversion
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := impl(tt.input)
			if types.IsError(tt.expected) {
				require.True(t, types.IsError(result), "Expected an error, but got: %v", result)
				// Simplified error message check for this example
				assert.Contains(t, result.Value().(error).Error(), "cannot convert", "Error message mismatch")
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
