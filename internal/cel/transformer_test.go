// transformer_test.go
package cel

import (
	"testing"
)

func TestTransformKaitaiExpression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		comment  string
	}{
		// Basic method calls
		{
			name:     "Convert to_s method",
			input:    "value.to_s()",
			expected: "to_s(value)",
			comment:  "Should convert to_s method call to function call",
		},
		{
			name:     "Convert to_i method",
			input:    "value.to_i()",
			expected: "to_i(value)",
			comment:  "Should convert to_i method call to function call",
		},
		{
			name:     "Convert to_f method",
			input:    "value.to_f()",
			expected: "to_f(value)",
			comment:  "Should convert to_f method call to function call",
		},

		// Property access
		{
			name:     "Convert length property",
			input:    "data.length",
			expected: "length(data)",
			comment:  "Should convert length property to function call",
		},
		{
			name:     "Convert size property",
			input:    "array.size",
			expected: "size(array)",
			comment:  "Should convert size property to function call",
		},
		{
			name:     "Convert count property",
			input:    "items.count",
			expected: "count(items)",
			comment:  "Should convert count property to function call",
		},
		{
			name:     "Generic property access",
			input:    "user.name",
			expected: "name(user)",
			comment:  "Should convert generic property access to function call",
		},

		// String operations
		{
			name:     "String contains method",
			input:    "text.contains(pattern)",
			expected: "contains(text, pattern)",
			comment:  "Should convert contains method to function call",
		},
		{
			name:     "String starts_with method",
			input:    "text.starts_with(prefix)",
			expected: "startsWith(text, prefix)",
			comment:  "Should convert starts_with method to function call",
		},
		{
			name:     "String ends_with method",
			input:    "text.ends_with(suffix)",
			expected: "endsWith(text, suffix)",
			comment:  "Should convert ends_with method to function call",
		},
		{
			name:     "String reverse method",
			input:    "text.reverse()",
			expected: "reverse(text)",
			comment:  "Should convert reverse method to function call",
		},
		{
			name:     "String substring method",
			input:    "text.substring(start, end)",
			expected: "substring(text, start, end)",
			comment:  "Should convert substring method to function call",
		},

		// Array operations
		{
			name:     "Array indexing with constant",
			input:    "arr[5]",
			expected: "at(arr, 5)",
			comment:  "Should convert array indexing with constant to at() function",
		},
		{
			name:     "Array indexing with variable",
			input:    "arr[idx]",
			expected: "at(arr, idx)",
			comment:  "Should convert array indexing with variable to at() function",
		},
		{
			name:     "Array slice with start",
			input:    "arr[2:]",
			expected: "slice(arr, 2)",
			comment:  "Should convert array slice with start to slice() function",
		},
		{
			name:     "Array slice with end",
			input:    "arr[:10]",
			expected: "sliceEnd(arr, 10)",
			comment:  "Should convert array slice with end to sliceEnd() function",
		},
		{
			name:     "Array slice with range",
			input:    "arr[2:10]",
			expected: "sliceRange(arr, 2, 10)",
			comment:  "Should convert array slice with range to sliceRange() function",
		},

		// Stream operations
		{
			name:     "IO read method",
			input:    "_io.read_u4le()",
			expected: "read_u4le(_io)",
			comment:  "Should convert IO read method to function call",
		},
		{
			name:     "IO position method",
			input:    "_io.pos()",
			expected: "pos(_io)",
			comment:  "Should convert pos method to function call",
		},
		{
			name:     "IO size method",
			input:    "_io.size()",
			expected: "size(_io)",
			comment:  "Should convert size method to function call",
		},
		{
			name:     "IO seek method",
			input:    "_io.seek(position)",
			expected: "seek(_io, position)",
			comment:  "Should convert seek method to function call",
		},
		{
			name:     "Read bytes method",
			input:    "stream.read_bytes(size)",
			expected: "readBytes(stream, size)",
			comment:  "Should convert read_bytes method to function call",
		},
		{
			name:     "Read bytes full method",
			input:    "stream.read_bytes_full()",
			expected: "readBytesFull(stream)",
			comment:  "Should convert read_bytes_full method to function call",
		},

		// Nested property access
		{
			name:     "Two-level property access",
			input:    "user.address.city",
			expected: "city(address(user))",
			comment:  "Should convert two-level property access to nested function calls",
		},
		{
			name:     "Three-level property access",
			input:    "user.address.location.latitude",
			expected: "latitude(location(address(user)))",
			comment:  "Should convert three-level property access to nested function calls",
		},

		// Bitwise operations
		{
			name:     "Bitwise AND",
			input:    "value & 0xFF",
			expected: "bitAnd(value, 0xFF)",
			comment:  "Should convert bitwise AND to bitAnd function",
		},
		{
			name:     "Bitwise OR",
			input:    "value | mask",
			expected: "bitOr(value, mask)",
			comment:  "Should convert bitwise OR to bitOr function",
		},
		{
			name:     "Bitwise XOR",
			input:    "value ^ 0x0F",
			expected: "bitXor(value, 0x0F)",
			comment:  "Should convert bitwise XOR to bitXor function",
		},
		{
			name:     "Bitwise NOT",
			input:    "~value",
			expected: "bitNot(value)",
			comment:  "Should convert bitwise NOT to bitNot function",
		},
		{
			name:     "Shift right",
			input:    "value >> 4",
			expected: "bitShiftRight(value, 4)",
			comment:  "Should convert shift right to bitShiftRight function",
		},
		{
			name:     "Shift left",
			input:    "value << 2",
			expected: "bitShiftLeft(value, 2)",
			comment:  "Should convert shift left to bitShiftLeft function",
		},
		{
			name:     "Parenthesized bitwise AND",
			input:    "(value & 0xFF) >> 4",
			expected: "bitShiftRight(bitAnd(value, 0xFF), 4)",
			comment:  "Should properly handle parenthesized bitwise operations",
		},

		// Ternary operator
		{
			name:     "Ternary operator",
			input:    "condition ? trueValue : falseValue",
			expected: "ternary(condition, trueValue, falseValue)",
			comment:  "Should convert ternary operator to ternary function",
		},

		// Math operations
		{
			name:     "Exponentiation",
			input:    "base ** exponent",
			expected: "pow(base, exponent)",
			comment:  "Should convert exponentiation to pow function",
		},
		{
			name:     "Modulo",
			input:    "value % divisor",
			expected: "mod(value, divisor)",
			comment:  "Should convert modulo to mod function",
		},
		{
			name:     "Multiplication",
			input:    "factor1 * factor2",
			expected: "mul(factor1, factor2)",
			comment:  "Should convert multiplication to mul function",
		},
		{
			name:     "Addition",
			input:    "addend1 + addend2",
			expected: "add(addend1, addend2)",
			comment:  "Should convert addition to add function",
		},

		// Complex expressions
		{
			name:     "Complex bitwise expression",
			input:    "(value & 0xFF) | ((flags << 8) & 0xFF00)",
			expected: "bitOr(bitAnd(value, 0xFF), bitAnd(bitShiftLeft(flags, 8), 0xFF00))",
			comment:  "Should handle complex bitwise expressions with parentheses",
		},
		{
			name:     "Mixed string and math operations",
			input:    "text.length * 2 + offset",
			expected: "add(mul(length(text), 2), offset)",
			comment:  "Should handle mixed string property and math operations",
		},
		{
			name:     "Method with argument and property access",
			input:    "data.substring(2, 10).length",
			expected: "length(substring(data, 2, 10))",
			comment:  "Should handle method calls with arguments followed by property access",
		},
		{
			name:     "Conditional with bitwise operations",
			input:    "(value & mask) == 0 ? default_value : value >> 2",
			expected: "ternary(bitAnd(value, mask) == 0, default_value, bitShiftRight(value, 2))",
			comment:  "Should handle conditionals with bitwise operations",
		},
		{
			name:     "Nested array access",
			input:    "matrix[row][col]",
			expected: "at(at(matrix, row), col)",
			comment:  "Should handle nested array access",
		},
		{
			name:     "IO operations in expressions",
			input:    "_io.size() - _io.pos()",
			expected: "size(_io) - pos(_io)",
			comment:  "Should handle IO operations in expressions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TransformKaitaiExpression(tt.input)
			if result != tt.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s\nComment:  %s",
					tt.input, tt.expected, result, tt.comment)
			}
		})
	}
}
