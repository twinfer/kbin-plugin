package cel

// import (
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// func TestTransformKaitaiExpression(t *testing.T) {
// 	testCases := []struct {
// 		name     string
// 		input    string
// 		expected string
// 	}{
// 		{
// 			name:     "Simple Property Access",
// 			input:    "foo.length",
// 			expected: "size(foo)",
// 		},
// 		{
// 			name:     "Method Call",
// 			input:    "foo.to_s()",
// 			expected: "to_s(foo)",
// 		},
// 		{
// 			name:     "Bitwise AND",
// 			input:    "a & b",
// 			expected: "bitAnd(a, b)",
// 		},
// 		{
// 			name:     "Bitwise Shift",
// 			input:    "foo >> 4",
// 			expected: "bitShiftRight(foo, 4)",
// 		},
// 		{
// 			name:     "Parenthesized Bitwise",
// 			input:    "(a & 0xFF) >> 4",
// 			expected: "bitShiftRight(bitAnd(a, 0xFF), 4)", // Updated to match CEL syntax
// 		},
// 		{
// 			name:     "Ternary Operator",
// 			input:    "a > b ? a : b",
// 			expected: "a > b ? a : b",
// 		},
// 		{
// 			name:     "Arithmetic with Safe Functions",
// 			input:    "a * 5",
// 			expected: "a * 5", // Keep native CEL arithmetic
// 		},
// 		{
// 			name:     "Complex Bitwise with Hex",
// 			input:    "b1 & 0xF0",
// 			expected: "bitAnd(b1, 0xF0)",
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			result := TransformKaitaiExpression(tc.input)
// 			assert.Equal(t, tc.expected, result)
// 		})
// 	}
// }

// func TestExpressionPoolBasic(t *testing.T) {
// 	pool, err := NewExpressionPool()
// 	require.NoError(t, err)

// 	program, err := pool.GetExpression("1 + 2")
// 	require.NoError(t, err)

// 	result, err := pool.EvaluateExpression(program, nil)
// 	require.NoError(t, err)

// 	// CEL returns integer values, but our AdaptCELResult should convert to float64
// 	assert.Equal(t, float64(3), result)
// }

// func TestExpressionWithVariables(t *testing.T) {
// 	pool, err := NewExpressionPool()
// 	require.NoError(t, err)

// 	program, err := pool.GetExpression("a + b")
// 	require.NoError(t, err)

// 	params := map[string]any{
// 		"a": 5,
// 		"b": 7,
// 	}

// 	result, err := pool.EvaluateExpression(program, params)
// 	require.NoError(t, err)
// 	assert.Equal(t, float64(12), result)
// }

// func TestBitwiseOperations(t *testing.T) {
// 	pool, err := NewExpressionPool()
// 	require.NoError(t, err)

// 	testCases := []struct {
// 		name     string
// 		expr     string
// 		params   map[string]any
// 		expected float64
// 	}{
// 		{
// 			name:     "Bitwise AND",
// 			expr:     "bitAnd(a, b)",
// 			params:   map[string]any{"a": 5, "b": 3},
// 			expected: float64(1), // 5 & 3 = 1
// 		},
// 		{
// 			name:     "Bitwise OR",
// 			expr:     "bitOr(a, b)",
// 			params:   map[string]any{"a": 5, "b": 3},
// 			expected: float64(7), // 5 | 3 = 7
// 		},
// 		{
// 			name:     "Bitwise Shift Left",
// 			expr:     "bitShiftLeft(a, b)",
// 			params:   map[string]any{"a": 5, "b": 2},
// 			expected: float64(20), // 5 << 2 = 20
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			program, err := pool.GetExpression(tc.expr)
// 			require.NoError(t, err)

// 			result, err := pool.EvaluateExpression(program, tc.params)
// 			require.NoError(t, err)
// 			assert.Equal(t, tc.expected, result)
// 		})
// 	}
// }

// func TestStringOperations(t *testing.T) {
// 	pool, err := NewExpressionPool()
// 	require.NoError(t, err)

// 	testCases := []struct {
// 		name     string
// 		expr     string
// 		params   map[string]any
// 		expected interface{}
// 	}{
// 		{
// 			name:     "String Size",
// 			expr:     "size(s)",
// 			params:   map[string]any{"s": "hello"},
// 			expected: float64(5),
// 		},
// 		{
// 			name:     "String Conversion",
// 			expr:     "string(n)",
// 			params:   map[string]any{"n": 123},
// 			expected: "123",
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			program, err := pool.GetExpression(tc.expr)
// 			require.NoError(t, err)

// 			result, err := pool.EvaluateExpression(program, tc.params)
// 			require.NoError(t, err)
// 			assert.Equal(t, tc.expected, result)
// 		})
// 	}
// }
