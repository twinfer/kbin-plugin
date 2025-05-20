package kaitaistruct

// import (
// 	"fmt"
// 	"math"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// func TestExpressionPool(t *testing.T) {
// 	pool := NewExpressionPool()

// 	// Test caching
// 	expr1, err := pool.GetExpression("1 + 2")
// 	require.NoError(t, err)
// 	assert.NotNil(t, expr1)

// 	expr2, err := pool.GetExpression("1 + 2")
// 	require.NoError(t, err)
// 	assert.Equal(t, expr1, expr2, "Same expression should be cached")

// 	expr3, err := pool.GetExpression("3 + 4")
// 	require.NoError(t, err)
// 	assert.NotEqual(t, expr1, expr3, "Different expressions should not be equal")
// }

// func TestTransformKaitaiExpression(t *testing.T) {
// 	tests := []struct {
// 		input    string
// 		expected string
// 	}{
// 		// Ternary operator
// 		{"condition ? trueValue : falseValue", "ternary(condition, trueValue, falseValue)"},

// 		// Bitwise operators
// 		{"value & 0xFF", "bitAnd(value, 0xFF)"},
// 		{"value | 0x01", "bitOr(value, 0x01)"},
// 		{"value ^ 0xFF", "bitXor(value, 0xFF)"},
// 		{"value << 8", "bitShiftLeft(value, 8)"},
// 		{"value >> 4", "bitShiftRight(value, 4)"},

// 		// Method calls
// 		{"value.to_s()", "to_s(value)"},
// 		{"value.to_i()", "to_i(value)"},
// 		{"array.length", "length(array)"},
// 		{"value.reverse", "reverse(value)"},

// 		// Complex expressions
// 		{"(flags & 0x80) != 0", "(bitAnd(flags, 0x80)) != 0"},
// 		{"value < 3 ? 'small' : 'large'", "ternary(value < 3, 'small', 'large')"},
// 		{"data.length > 10 && data[0] == 0xFF", "length(data) > 10 && data[0] == 0xFF"},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.input, func(t *testing.T) {
// 			result := transformKaitaiExpression(tt.input)
// 			assert.Equal(t, tt.expected, result)
// 		})
// 	}
// }

// func TestExpressionEvaluation(t *testing.T) {
// 	pool := NewExpressionPool()

// 	tests := []struct {
// 		expr     string
// 		params   map[string]any
// 		expected any
// 		isError  bool
// 	}{
// 		// Basic arithmetic
// 		{"1 + 2", nil, float64(3), false},
// 		{"a + b", map[string]any{"a": 5, "b": 3}, float64(8), false},
// 		{"a * (b + c)", map[string]any{"a": 2, "b": 3, "c": 4}, float64(14), false},

// 		// Bitwise operations
// 		{"bitAnd(a, b)", map[string]any{"a": 15, "b": 5}, float64(5), false},
// 		{"bitOr(a, b)", map[string]any{"a": 10, "b": 5}, float64(15), false},
// 		{"bitXor(a, b)", map[string]any{"a": 15, "b": 5}, float64(10), false},
// 		{"bitShiftLeft(a, b)", map[string]any{"a": 1, "b": 4}, float64(16), false},
// 		{"bitShiftRight(a, b)", map[string]any{"a": 16, "b": 2}, float64(4), false},

// 		// String operations
// 		{"to_s(a)", map[string]any{"a": 42}, "42", false},
// 		{"length(str)", map[string]any{"str": "hello"}, float64(5), false},
// 		{"reverse(str)", map[string]any{"str": "hello"}, "olleh", false},

// 		// Array operations
// 		{"length(arr)", map[string]any{"arr": []any{1, 2, 3}}, float64(3), false},

// 		// Boolean operations
// 		{"a && b", map[string]any{"a": true, "b": false}, false, false},
// 		{"a || b", map[string]any{"a": true, "b": false}, true, false},
// 		{"!a", map[string]any{"a": true}, false, false},

// 		// Ternary
// 		{"ternary(a > b, 'yes', 'no')", map[string]any{"a": 5, "b": 3}, "yes", false},
// 		{"ternary(a < b, 'yes', 'no')", map[string]any{"a": 5, "b": 3}, "no", false},

// 		// Math functions
// 		{"abs(a)", map[string]any{"a": -5}, float64(5), false},
// 		{"min(a, b)", map[string]any{"a": 5, "b": 3}, float64(3), false},
// 		{"max(a, b)", map[string]any{"a": 5, "b": 3}, float64(5), false},

// 		// Errors
// 		{"a + b", map[string]any{"a": 5}, nil, true}, // Missing parameter
// 		{"1 / 0", nil, math.Inf(1), false},           // Division by zero returns +Inf, not error
// 		{"undefined_function()", nil, nil, true},     // Undefined function
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.expr, func(t *testing.T) {
// 			program, err := pool.GetExpression(tt.expr)
// 			require.NoError(t, err, "Failed to compile expression")

// 			result, err := pool.EvaluateExpression(program, tt.params)

// 			if tt.isError {
// 				assert.Error(t, err)
// 			} else {
// 				assert.NoError(t, err)
// 				// Handle int/float64 comparison
// 				if expectedFloat, ok := tt.expected.(float64); ok {
// 					switch v := result.(type) {
// 					case int:
// 						assert.Equal(t, expectedFloat, float64(v))
// 					case float64:
// 						assert.Equal(t, expectedFloat, v)
// 					default:
// 						assert.Equal(t, tt.expected, result)
// 					}
// 				} else {
// 					assert.Equal(t, tt.expected, result)
// 				}
// 			}
// 		})
// 	}
// }

// func TestIsTrueFunction(t *testing.T) {
// 	tests := []struct {
// 		value    any
// 		expected bool
// 	}{
// 		{true, true},
// 		{false, false},
// 		{1, true},
// 		{0, false},
// 		{"hello", true},
// 		{"", false},
// 		{nil, false},
// 	}

// 	for _, tt := range tests {
// 		t.Run(fmt.Sprintf("%v", tt.value), func(t *testing.T) {
// 			result := isTrue(tt.value)
// 			assert.Equal(t, tt.expected, result)
// 		})
// 	}
// }
