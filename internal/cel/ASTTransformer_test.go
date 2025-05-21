package cel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/expression"
)

func TestASTTransformer_Transform(t *testing.T) {
	tests := []struct {
		name          string
		kaitaiExpr    string
		expectedCEL   string
		expectError   bool
		errorContains string
	}{
		// --- Literals ---
		{
			name:        "Boolean Literal True",
			kaitaiExpr:  "true",
			expectedCEL: "true",
		},
		{
			name:        "Boolean Literal False",
			kaitaiExpr:  "false",
			expectedCEL: "false",
		},
		{
			name:        "Integer Literal",
			kaitaiExpr:  "12345",
			expectedCEL: "12345",
		},
		{
			name:        "Float Literal",
			kaitaiExpr:  "123.45",
			expectedCEL: "123.45",
		},
		{
			name:        "String Literal",
			kaitaiExpr:  `"hello world"`,
			expectedCEL: `"hello world"`,
		},
		{
			name:        "String Literal with Escapes",
			kaitaiExpr:  `"hello\nworld"`,
			expectedCEL: `"hello\nworld"`, // strconv.Quote handles this
		},
		{
			name:        "Null Literal",
			kaitaiExpr:  "null",
			expectedCEL: "null",
		},

		// --- Identifiers and Special Variables ---
		{
			name:        "Identifier",
			kaitaiExpr:  "my_variable",
			expectedCEL: "my_variable",
		},
		{
			name:        "Self Variable",
			kaitaiExpr:  "_",
			expectedCEL: "_",
		},
		{
			name:        "IO Variable",
			kaitaiExpr:  "_io",
			expectedCEL: "_io",
		},
		{
			name:        "Parent Variable",
			kaitaiExpr:  "_parent",
			expectedCEL: "_parent",
		},
		{
			name:        "Root Variable",
			kaitaiExpr:  "_root",
			expectedCEL: "_root",
		},
		{
			name:        "Bytes Remaining Variable",
			kaitaiExpr:  "_bytes_remaining",
			expectedCEL: "_bytes_remaining",
		},

		// --- Unary Operations ---
		{
			name:        "Logical NOT",
			kaitaiExpr:  "!some_var",
			expectedCEL: "!some_var",
		},
		{
			name:        "Numeric Negation",
			kaitaiExpr:  "-another_var",
			expectedCEL: "-another_var",
		},
		{
			name:        "Bitwise NOT",
			kaitaiExpr:  "~counter",
			expectedCEL: "bitNot(counter)",
		},

		// --- Binary Operations ---
		{
			name:        "Addition",
			kaitaiExpr:  "var1 + var2",
			expectedCEL: "(var1 + var2)",
		},
		{
			name:        "Subtraction",
			kaitaiExpr:  "var1 - 10",
			expectedCEL: "(var1 - 10)",
		},
		{
			name:        "Multiplication",
			kaitaiExpr:  "5 * count",
			expectedCEL: "(5 * count)",
		},
		{
			name:        "Division",
			kaitaiExpr:  "total / items",
			expectedCEL: "(total / items)",
		},
		{
			name:        "Modulo",
			kaitaiExpr:  "value % 2",
			expectedCEL: "(value % 2)",
		},
		{
			name:        "Equality",
			kaitaiExpr:  `name == "test"`,
			expectedCEL: `(name == "test")`,
		},
		{
			name:        "Inequality",
			kaitaiExpr:  `age != 30`,
			expectedCEL: `(age != 30)`,
		},
		{
			name:        "Less Than",
			kaitaiExpr:  "a < b",
			expectedCEL: "(a < b)",
		},
		{
			name:        "Greater Than",
			kaitaiExpr:  "a > b",
			expectedCEL: "(a > b)",
		},
		{
			name:        "Less Than or Equal",
			kaitaiExpr:  "a <= b",
			expectedCEL: "(a <= b)",
		},
		{
			name:        "Greater Than or Equal",
			kaitaiExpr:  "a >= b",
			expectedCEL: "(a >= b)",
		},
		{
			name:        "Logical AND",
			kaitaiExpr:  "is_valid && has_data",
			expectedCEL: "(is_valid && has_data)",
		},
		{
			name:        "Logical OR",
			kaitaiExpr:  "is_error || is_empty",
			expectedCEL: "(is_error || is_empty)",
		},
		{
			name:        "Bitwise AND",
			kaitaiExpr:  "flags & 0x0F",
			expectedCEL: "bitAnd(flags, 15)", // 0x0F should be parsed as int 15
		},
		{
			name:        "Bitwise OR",
			kaitaiExpr:  "mask | 0b1010",
			expectedCEL: "bitOr(mask, 10)", // 0b1010 is 10
		},
		{
			name:        "Bitwise XOR",
			kaitaiExpr:  "val1 ^ val2",
			expectedCEL: "bitXor(val1, val2)",
		},
		{
			name:        "Left Shift",
			kaitaiExpr:  "data << 2",
			expectedCEL: "bitShiftLeft(data, 2)",
		},
		{
			name:        "Right Shift",
			kaitaiExpr:  "value >> shift_amount",
			expectedCEL: "bitShiftRight(value, shift_amount)",
		},
		{
			name:        "Operator Precedence: a + b * c",
			kaitaiExpr:  "a + b * c",
			expectedCEL: "(a + (b * c))",
		},
		{
			name:        "Operator Precedence with Grouping: (a + b) * c",
			kaitaiExpr:  "(a + b) * c",
			expectedCEL: "((a + b) * c)",
		},

		// --- Ternary Operator ---
		{
			name:        "Ternary Operator",
			kaitaiExpr:  "is_active ? 1 : 0",
			expectedCEL: "ternary(is_active, 1, 0)",
		},

		// --- Attribute Access ---
		{
			name:        "Simple Attribute Access",
			kaitaiExpr:  "my_object.field_name",
			expectedCEL: "my_object.field_name",
		},
		{
			name:        "IO Position Attribute",
			kaitaiExpr:  "_io.pos",
			expectedCEL: "pos(_io)",
		},
		{
			name:        "IO Size Attribute",
			kaitaiExpr:  "_io.size",
			expectedCEL: "stream_size(_io)",
		},
		{
			name:        "IO EOF Attribute",
			kaitaiExpr:  "_io.eof",
			expectedCEL: "isEOF(_io)",
		},
		{
			name:        "Chained Attribute Access",
			kaitaiExpr:  "data.header.version",
			expectedCEL: "data.header.version",
		},

		// --- Array Indexing ---
		{
			name:        "Array Indexing with Identifier",
			kaitaiExpr:  "my_array[index_var]",
			expectedCEL: "at(my_array, index_var)",
		},
		{
			name:        "Array Indexing with Literal",
			kaitaiExpr:  "elements[0]",
			expectedCEL: "at(elements, 0)",
		},

		// --- Function Calls ---
		{
			name:        "Global Function Call No Args",
			kaitaiExpr:  "get_version()",
			expectedCEL: "get_version()", // Assuming get_version is mapped or used directly
		},
		{
			name:        "Global Function Call With Args",
			kaitaiExpr:  "calculate(val1, 25)",
			expectedCEL: "calculate(val1, 25)", // Assuming calculate is mapped or used directly
		},
		{
			name:        "IO Method Call: read_u1",
			kaitaiExpr:  "_io.read_u1()",
			expectedCEL: "readU1(_io)",
		},
		{
			name:        "IO Method Call: read_bytes",
			kaitaiExpr:  "_io.read_bytes(16)",
			expectedCEL: "readBytes(_io, 16)",
		},
		{
			name:        "Object Method Call No Args",
			kaitaiExpr:  "my_string.reverse()",
			expectedCEL: "reverse(my_string)", // Assuming 'reverse' is mapped for strings
		},
		{
			name:        "Object Method Call With Args",
			kaitaiExpr:  "my_list.contains(item)",
			expectedCEL: "contains(my_list, item)", // Assuming 'contains' is mapped
		},

		// --- Type Casts ---
		{
			name:        "Cast to s1",
			kaitaiExpr:  "value.as<s1>()",
			expectedCEL: "to_i(value)",
		},
		{
			name:        "Cast to f4",
			kaitaiExpr:  "num_str.as<f4>()",
			expectedCEL: "to_f(num_str)",
		},
		{
			name:          "Unsupported Cast Type",
			kaitaiExpr:    "value.as<unknown_type>()",
			expectError:   true,
			errorContains: "unsupported cast type: unknown_type",
		},

		// --- Unsupported Operations ---
		{
			name:          "SizeOf Operator",
			kaitaiExpr:    "sizeof(my_data)",
			expectError:   true,
			errorContains: "sizeof not directly supported",
		},
		{
			name:          "AlignOf Operator",
			kaitaiExpr:    "alignof(my_data)",
			expectError:   true,
			errorContains: "alignof not directly supported",
		},

		// --- Complex/Combined Expressions ---
		{
			name:        "Complex: (a.b + c[0]) * d.e()",
			kaitaiExpr:  "(my_obj.field + arr[0]) * another_obj.get_val()",
			expectedCEL: "((my_obj.field + at(arr, 0)) * get_val(another_obj))", // Corrected expected output
		},
		{
			name:        "Complex: Ternary with function calls",
			kaitaiExpr:  "check() ? _io.read_u2le() : _io.read_u4le()",
			expectedCEL: "ternary(check(), readU2le(_io), readU4le(_io))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Parse Kaitai expression to AST
			lexer := expression.NewExpressionLexer(strings.NewReader(tt.kaitaiExpr))
			parser := expression.NewExpressionParser(lexer)
			astNode, err := parser.Parse()

			// If parsing itself is expected to fail for a malformed Kaitai expression (not covered here, but good for parser tests)
			// or if the expression is valid Kaitai but uses features the transformer doesn't support yet.
			if err != nil && !tt.expectError { // Kaitai parsing error, but test wasn't expecting an error from transform
				t.Fatalf("Kaitai AST parsing failed for '%s': %v. Parser errors: %s", tt.kaitaiExpr, err, strings.Join(parser.Errors(), "; "))
				return
			}
			if err == nil && astNode == nil { // Should not happen if parser.Parse() is correct
				t.Fatalf("Kaitai AST parsing returned nil AST without error for '%s'", tt.kaitaiExpr)
				return
			}

			// 2. Transform AST to CEL
			transformer := NewASTTransformer()
			actualCEL, transformErr := transformer.Transform(astNode)

			// 3. Assertions
			if tt.expectError {
				assert.Error(t, transformErr, "Expected an error during transformation")
				if tt.errorContains != "" {
					assert.Contains(t, transformErr.Error(), tt.errorContains, "Error message mismatch")
				}
			} else {
				require.NoError(t, transformErr, "Transformation failed unexpectedly")
				assert.Equal(t, tt.expectedCEL, actualCEL, "Generated CEL string does not match expected")
			}
		})
	}
}
