package kaitaistruct

// import (
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// func TestExpressionPoolComparisonWithCEL(t *testing.T) {
// 	// Create both expression pools
// 	exprPool := NewExpressionPool()        // Original expr-based pool
// 	celPool, err := NewCELExpressionPool() // New CEL-based pool
// 	require.NoError(t, err)

// 	testCases := []struct {
// 		name   string
// 		expr   string
// 		params map[string]interface{}
// 	}{
// 		{
// 			name: "Simple Addition",
// 			expr: "a + b",
// 			params: map[string]interface{}{
// 				"a": 5,
// 				"b": 3,
// 			},
// 		},
// 		{
// 			name: "Ternary Operator",
// 			expr: "a > b ? a : b",
// 			params: map[string]interface{}{
// 				"a": 5,
// 				"b": 3,
// 			},
// 		},
// 		{
// 			name: "Bitwise AND",
// 			expr: "a & b",
// 			params: map[string]interface{}{
// 				"a": 5,
// 				"b": 3,
// 			},
// 		},
// 		{
// 			name: "Bitwise Shift",
// 			expr: "a >> 2",
// 			params: map[string]interface{}{
// 				"a": 20,
// 			},
// 		},
// 		{
// 			name: "Complex Bitwise",
// 			expr: "(a & 0xFF) >> 2",
// 			params: map[string]interface{}{
// 				"a": 0x1234,
// 			},
// 		},
// 		{
// 			name: "String Operation",
// 			expr: "str.to_s()",
// 			params: map[string]interface{}{
// 				"str": "hello",
// 			},
// 		},
// 		{
// 			name: "String Length",
// 			expr: "str.length",
// 			params: map[string]interface{}{
// 				"str": "hello",
// 			},
// 		},
// 	}

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			// Evaluate using original expr pool
// 			exprProgram, err := exprPool.GetExpression(tc.expr)
// 			require.NoError(t, err, "Failed to compile expression with expr")

// 			exprResult, err := exprPool.EvaluateExpression(exprProgram, tc.params)
// 			require.NoError(t, err, "Failed to evaluate expression with expr")

// 			// Evaluate using new CEL pool
// 			celProgram, err := celPool.GetExpression(tc.expr)
// 			require.NoError(t, err, "Failed to compile expression with CEL")

// 			celResult, err := celPool.EvaluateExpression(celProgram, tc.params)
// 			require.NoError(t, err, "Failed to evaluate expression with CEL")

// 			// Compare results
// 			assert.Equal(t, exprResult, celResult, "Results from expr and CEL should match")
// 		})
// 	}
// }

// func BenchmarkExpressionEvaluation(b *testing.B) {
// 	// Create both expression pools
// 	exprPool := NewExpressionPool()
// 	celPool, _ := NewCELExpressionPool()

// 	// Compile test expressions
// 	exprProgram, _ := exprPool.GetExpression("(a & 0xFF) >> 2")
// 	celProgram, _ := celPool.GetExpression("(a & 0xFF) >> 2")

// 	params := map[string]any{
// 		"a": 0x1234,
// 	}

// 	b.Run("Original Expr", func(b *testing.B) {
// 		for b.Loop() {
// 			_, _ = exprPool.EvaluateExpression(exprProgram, params)
// 		}
// 	})

// 	b.Run("CEL Implementation", func(b *testing.B) {
// 		for b.Loop() {
// 			_, _ = celPool.EvaluateExpression(celProgram, params)
// 		}
// 	})
// }
