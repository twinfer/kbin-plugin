package expression

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp" // A better comparison library than reflect.DeepEqual
)

// Helper function to create a simplified Pos for testing, assuming line 1
func p(col int) Pos {
	return Pos{Line: 1, Column: col}
}

// Helper to compare ASTs, including position info (adjust as needed)
// For deep equality, cmp.Diff is excellent for debugging mismatches.
func exprCmpOpts() cmp.Option {
	return cmp.AllowUnexported(
		BoolLit{}, IntLit{}, StrLit{}, FltLit{}, NullLit{},
		Id{}, Self{}, Io{}, Parent{}, Root{}, BytesRemaining{},
		UnOp{}, BinOp{}, TernaryOp{}, Attr{}, Call{}, ArrayIdx{}, CastToType{}, SizeOf{}, AlignOf{},
	)
}

// --- Expression Lexer Tests ---

func TestExpressionLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ExpressionToken
	}{
		{
			name:  "Empty input",
			input: "",
			expected: []ExpressionToken{
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 1},
			},
		},
		{
			name:  "Whitespace only",
			input: "  \t\n\r ",
			expected: []ExpressionToken{
				{Type: EXPR_EOF, Literal: "", Line: 2, Column: 3}, // After space following \r
			},
		},
		{
			name:  "Basic identifiers and literals",
			input: "foo _root 123 \"bar\" 'baz' true false null",
			expected: []ExpressionToken{
				{Type: EXPR_IDENT, Literal: "foo", Line: 1, Column: 1},
				{Type: EXPR_ROOT, Literal: "_root", Line: 1, Column: 5},
				{Type: EXPR_NUMBER, Literal: "123", Line: 1, Column: 11},
				{Type: EXPR_STRING, Literal: "bar", Line: 1, Column: 15}, // Column points to starting quote
				{Type: EXPR_STRING, Literal: "baz", Line: 1, Column: 21},
				{Type: EXPR_BOOLEAN, Literal: "true", Line: 1, Column: 27},
				{Type: EXPR_BOOLEAN, Literal: "false", Line: 1, Column: 32},
				{Type: EXPR_NULL, Literal: "null", Line: 1, Column: 38},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 42},
			},
		},
		{
			name:  "Numbers: decimal, hex, octal, binary, float, scientific",
			input: "123 0xabc 0O77 0b101 1.23 1.23e-5 4E+2",
			expected: []ExpressionToken{
				{Type: EXPR_NUMBER, Literal: "123", Line: 1, Column: 1},
				{Type: EXPR_NUMBER, Literal: "0xabc", Line: 1, Column: 5},
				{Type: EXPR_NUMBER, Literal: "0O77", Line: 1, Column: 11},
				{Type: EXPR_NUMBER, Literal: "0b101", Line: 1, Column: 16},
				{Type: EXPR_NUMBER, Literal: "1.23", Line: 1, Column: 22},
				{Type: EXPR_NUMBER, Literal: "1.23e-5", Line: 1, Column: 27},
				{Type: EXPR_NUMBER, Literal: "4E+2", Line: 1, Column: 35},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 39},
			},
		},
		{
			name:  "String escapes",
			input: `"\n\t\\\"\'\u00A9"`, // Copyright symbol
			expected: []ExpressionToken{
				{Type: EXPR_STRING, Literal: "\n\t\\\"'\u00A9", Line: 1, Column: 1},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 19}, // After end quote of 18-char string
			},
		},
		{
			name:  "Single character operators",
			input: "+-*/%&|~^!()[].,?:",
			expected: []ExpressionToken{
				{Type: EXPR_PLUS, Literal: "+", Line: 1, Column: 1},
				{Type: EXPR_MINUS, Literal: "-", Line: 1, Column: 2},
				{Type: EXPR_STAR, Literal: "*", Line: 1, Column: 3},
				{Type: EXPR_SLASH, Literal: "/", Line: 1, Column: 4},
				{Type: EXPR_MOD, Literal: "%", Line: 1, Column: 5},
				{Type: EXPR_BIT_AND, Literal: "&", Line: 1, Column: 6},
				{Type: EXPR_BIT_OR, Literal: "|", Line: 1, Column: 7},
				{Type: EXPR_BIT_NOT, Literal: "~", Line: 1, Column: 8},
				{Type: EXPR_BIT_XOR, Literal: "^", Line: 1, Column: 9},
				{Type: EXPR_LOGIC_NOT, Literal: "!", Line: 1, Column: 10},
				{Type: EXPR_LPAREN, Literal: "(", Line: 1, Column: 11},
				{Type: EXPR_RPAREN, Literal: ")", Line: 1, Column: 12},
				{Type: EXPR_LBRACKET, Literal: "[", Line: 1, Column: 13},
				{Type: EXPR_RBRACKET, Literal: "]", Line: 1, Column: 14},
				{Type: EXPR_DOT, Literal: ".", Line: 1, Column: 15},
				{Type: EXPR_COMMA, Literal: ",", Line: 1, Column: 16},
				{Type: EXPR_TERNARY_QUESTION, Literal: "?", Line: 1, Column: 17},
				{Type: EXPR_TERNARY_COLON, Literal: ":", Line: 1, Column: 18},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 19},
			},
		},
		{
			name:  "Multi-character operators",
			input: "== != <= >= << >> && ||",
			expected: []ExpressionToken{
				{Type: EXPR_EQ, Literal: "==", Line: 1, Column: 1},
				{Type: EXPR_NEQ, Literal: "!=", Line: 1, Column: 4},
				{Type: EXPR_LE, Literal: "<=", Line: 1, Column: 7},
				{Type: EXPR_GE, Literal: ">=", Line: 1, Column: 10},
				{Type: EXPR_LSHIFT, Literal: "<<", Line: 1, Column: 13},
				{Type: EXPR_RSHIFT, Literal: ">>", Line: 1, Column: 16},
				{Type: EXPR_LOGIC_AND, Literal: "&&", Line: 1, Column: 19},
				{Type: EXPR_LOGIC_OR, Literal: "||", Line: 1, Column: 22},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 24},
			},
		},
		{
			name:  "Built-in functions/keywords",
			input: "sizeof alignof as",
			expected: []ExpressionToken{
				{Type: EXPR_SIZEOF, Literal: "sizeof", Line: 1, Column: 1},
				{Type: EXPR_ALIGNOF, Literal: "alignof", Line: 1, Column: 8},
				{Type: EXPR_AS, Literal: "as", Line: 1, Column: 16},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 18},
			},
		},
		{
			name:  "Complex expression with mixed tokens",
			input: "(_io.pos + 2 * 3) == _parent.my_field[0] && !(1.0 / 2.0)",
			expected: []ExpressionToken{
				{Type: EXPR_LPAREN, Literal: "(", Line: 1, Column: 1},
				{Type: EXPR_IO, Literal: "_io", Line: 1, Column: 2},
				{Type: EXPR_DOT, Literal: ".", Line: 1, Column: 5},
				{Type: EXPR_IDENT, Literal: "pos", Line: 1, Column: 6},
				{Type: EXPR_PLUS, Literal: "+", Line: 1, Column: 10},
				{Type: EXPR_NUMBER, Literal: "2", Line: 1, Column: 12},
				{Type: EXPR_STAR, Literal: "*", Line: 1, Column: 14},
				{Type: EXPR_NUMBER, Literal: "3", Line: 1, Column: 16},
				{Type: EXPR_RPAREN, Literal: ")", Line: 1, Column: 17},
				{Type: EXPR_EQ, Literal: "==", Line: 1, Column: 19},
				{Type: EXPR_PARENT, Literal: "_parent", Line: 1, Column: 22},
				{Type: EXPR_DOT, Literal: ".", Line: 1, Column: 29},
				{Type: EXPR_IDENT, Literal: "my_field", Line: 1, Column: 30},
				{Type: EXPR_LBRACKET, Literal: "[", Line: 1, Column: 38},
				{Type: EXPR_NUMBER, Literal: "0", Line: 1, Column: 39},
				{Type: EXPR_RBRACKET, Literal: "]", Line: 1, Column: 40},
				{Type: EXPR_LOGIC_AND, Literal: "&&", Line: 1, Column: 42},
				{Type: EXPR_LOGIC_NOT, Literal: "!", Line: 1, Column: 45},
				{Type: EXPR_LPAREN, Literal: "(", Line: 1, Column: 46},
				{Type: EXPR_NUMBER, Literal: "1.0", Line: 1, Column: 47},
				{Type: EXPR_SLASH, Literal: "/", Line: 1, Column: 51},
				{Type: EXPR_NUMBER, Literal: "2.0", Line: 1, Column: 53},
				{Type: EXPR_RPAREN, Literal: ")", Line: 1, Column: 56},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 57},
			},
		},
		{
			name:  "Illegal character",
			input: "$",
			expected: []ExpressionToken{
				{Type: EXPR_ILLEGAL, Literal: "$", Line: 1, Column: 1},
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 2},
			},
		},
		{
			name:  "Unclosed string",
			input: `"hello`, // Missing closing quote
			expected: []ExpressionToken{
				{Type: EXPR_STRING, Literal: "hello", Line: 1, Column: 1}, // Lexer will consume until EOF
				{Type: EXPR_EOF, Literal: "", Line: 1, Column: 7},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewExpressionLexer(strings.NewReader(tt.input))
			var actual []ExpressionToken
			for {
				tok := l.NextToken()
				actual = append(actual, tok)
				if tok.Type == EXPR_EOF {
					break
				}
			}

			if diff := cmp.Diff(tt.expected, actual); diff != "" {
				t.Errorf("Lexer mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// --- Expression Parser Tests ---

func TestExpressionParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Expr // Expected AST
		wantErr  bool
	}{
		// --- Literals ---
		{
			name:     "Integer literal",
			input:    "123",
			expected: &IntLit{Value: 123, P: p(1)},
		},
		{
			name:     "Hex integer literal",
			input:    "0xAA",
			expected: &IntLit{Value: 170, P: p(1)},
		},
		{
			name:     "Float literal",
			input:    "1.23",
			expected: &FltLit{Value: 1.23, P: p(1)},
		},
		{
			name:     "String literal",
			input:    `"hello"`,
			expected: &StrLit{Value: "hello", P: p(1)},
		},
		{
			name:     "Boolean true literal",
			input:    "true",
			expected: &BoolLit{Value: true, P: p(1)},
		},
		{
			name:     "Boolean false literal",
			input:    "false",
			expected: &BoolLit{Value: false, P: p(1)},
		},
		{
			name:     "Null literal",
			input:    "null",
			expected: &NullLit{P: p(1)},
		},
		// --- Identifiers and Special Variables ---
		{
			name:     "Regular identifier",
			input:    "my_var",
			expected: &Id{Name: "my_var", P: p(1)},
		},
		{
			name:     "Self variable",
			input:    "_",
			expected: &Self{P: p(1)},
		},
		{
			name:     "IO variable",
			input:    "_io",
			expected: &Io{P: p(1)},
		},
		{
			name:     "Parent variable",
			input:    "_parent",
			expected: &Parent{P: p(1)},
		},
		{
			name:     "Root variable",
			input:    "_root",
			expected: &Root{P: p(1)},
		},
		{
			name:     "Bytes Remaining variable",
			input:    "_bytes_remaining",
			expected: &BytesRemaining{P: p(1)},
		},
		// --- Unary Operations ---
		{
			name:     "Logical NOT",
			input:    "!true",
			expected: &UnOp{Op: UnOpNot, Arg: &BoolLit{Value: true, P: p(2)}, P: p(1)},
		},
		{
			name:     "Numeric negation",
			input:    "-10",
			expected: &UnOp{Op: UnOpNeg, Arg: &IntLit{Value: 10, P: p(2)}, P: p(1)},
		},
		{
			name:     "Bitwise NOT",
			input:    "~a",
			expected: &UnOp{Op: UnOpBitwiseNot, Arg: &Id{Name: "a", P: p(2)}, P: p(1)},
		},
		// --- Grouping ---
		{
			name:     "Grouped expression",
			input:    "(1 + 2)",
			expected: &BinOp{Op: BinOpAdd, Arg1: &IntLit{Value: 1, P: p(2)}, Arg2: &IntLit{Value: 2, P: p(6)}, P: p(4)},
		},
		// --- Binary Operations & Precedence ---
		{
			name:     "Addition",
			input:    "1 + 2",
			expected: &BinOp{Op: BinOpAdd, Arg1: &IntLit{Value: 1, P: p(1)}, Arg2: &IntLit{Value: 2, P: p(5)}, P: p(3)},
		},
		{
			name:  "Multiplication and Addition (precedence)",
			input: "1 + 2 * 3",
			expected: &BinOp{
				Op:   BinOpAdd,
				Arg1: &IntLit{Value: 1, P: p(1)},
				Arg2: &BinOp{Op: BinOpMul, Arg1: &IntLit{Value: 2, P: p(5)}, Arg2: &IntLit{Value: 3, P: p(9)}, P: p(7)},
				P:    p(3),
			},
		},
		{
			name:  "Subtraction associativity",
			input: "5 - 3 - 1",
			expected: &BinOp{
				Op:   BinOpSub,
				Arg1: &BinOp{Op: BinOpSub, Arg1: &IntLit{Value: 5, P: p(1)}, Arg2: &IntLit{Value: 3, P: p(5)}, P: p(3)},
				Arg2: &IntLit{Value: 1, P: p(9)},
				P:    p(7),
			},
		},
		{
			name:     "Comparison",
			input:    "a == b",
			expected: &BinOp{Op: BinOpEq, Arg1: &Id{Name: "a", P: p(1)}, Arg2: &Id{Name: "b", P: p(6)}, P: p(3)},
		},
		{
			name:  "Logical AND and OR",
			input: "a && b || c",
			expected: &BinOp{
				Op:   BinOpOr,
				Arg1: &BinOp{Op: BinOpAnd, Arg1: &Id{Name: "a", P: p(1)}, Arg2: &Id{Name: "b", P: p(6)}, P: p(3)},
				Arg2: &Id{Name: "c", P: p(11)},
				P:    p(9),
			},
		},
		{
			name:  "Bitwise operators",
			input: "a & b | c ^ d << e >> f",
			expected: &BinOp{
				Op: BinOpRShift,
				Arg1: &BinOp{
					Op: BinOpLShift,
					Arg1: &BinOp{
						Op: BinOpBitwiseXor,
						Arg1: &BinOp{
							Op: BinOpBitwiseOr,
							Arg1: &BinOp{
								Op:   BinOpBitwiseAnd,
								Arg1: &Id{Name: "a", P: p(1)},
								Arg2: &Id{Name: "b", P: p(5)},
								P:    p(3)},
							Arg2: &Id{Name: "c", P: p(9)},
							P:    p(7)},
						Arg2: &Id{Name: "d", P: p(13)},
						P:    p(11)},
					Arg2: &Id{Name: "e", P: p(17)},
					P:    p(15)},
				Arg2: &Id{Name: "f", P: p(21)},
				P:    p(19),
			},
		},
		// --- Ternary Operator ---
		{
			name:  "Ternary operator",
			input: "cond ? val_true : val_false",
			expected: &TernaryOp{
				Cond:    &Id{Name: "cond", P: p(1)},
				IfTrue:  &Id{Name: "val_true", P: p(9)},
				IfFalse: &Id{Name: "val_false", P: p(20)},
				P:       p(6),
			},
		},
		// --- Member Access / Calls / Indexing ---
		{
			name:     "Member access",
			input:    "obj.field",
			expected: &Attr{Value: &Id{Name: "obj", P: p(1)}, Name: "field", P: p(4)},
		},
		{
			name:     "Function call with no arguments",
			input:    "func()",
			expected: &Call{Value: &Id{Name: "func", P: p(1)}, Args: []Expr{}, P: p(1)},
		},
		{
			name:  "Function call with arguments",
			input: "func(arg1, 123)",
			expected: &Call{
				Value: &Id{Name: "func", P: p(1)},
				Args: []Expr{
					&Id{Name: "arg1", P: p(6)},
					&IntLit{Value: 123, P: p(12)},
				},
				P: p(1),
			},
		},
		{
			name:  "Nested function call",
			input: "foo(bar())",
			expected: &Call{
				Value: &Id{Name: "foo", P: p(1)},
				Args: []Expr{
					&Call{Value: &Id{Name: "bar", P: p(5)}, Args: []Expr{}, P: p(5)},
				},
				P: p(1),
			},
		},
		{
			name:  "Member access and method call",
			input: "obj.method(arg)",
			expected: &Call{
				Value: &Attr{Value: &Id{Name: "obj", P: p(1)}, Name: "method", P: p(4)},
				Args:  []Expr{&Id{Name: "arg", P: p(12)}},
				P:     p(1), // Pos for the whole call expression
			},
		},
		{
			name:     "Array indexing",
			input:    "arr[idx]",
			expected: &ArrayIdx{Value: &Id{Name: "arr", P: p(1)}, Idx: &Id{Name: "idx", P: p(5)}, P: p(1)},
		},
		{
			name:  "Nested array indexing",
			input: "arr[0][1]",
			expected: &ArrayIdx{
				Value: &ArrayIdx{Value: &Id{Name: "arr", P: p(1)}, Idx: &IntLit{Value: 0, P: p(5)}, P: p(1)},
				Idx:   &IntLit{Value: 1, P: p(8)},
				P:     p(1),
			},
		},
		// --- Type Casting ---
		{
			name:  "Type cast (as<type>)",
			input: "foo.as<u4>()",
			expected: &CastToType{
				Value:    &Id{Name: "foo", P: p(1)},
				TypeName: "u4",
				P:        p(1),
			},
		},
		// --- Built-in Functions ---
		{
			name:     "SizeOf",
			input:    "sizeof(foo)",
			expected: &SizeOf{Value: &Id{Name: "foo", P: p(8)}, P: p(1)},
		},
		{
			name:     "AlignOf",
			input:    "alignof(bar)",
			expected: &AlignOf{Value: &Id{Name: "bar", P: p(9)}, P: p(1)},
		},
		// --- Complex Combinations ---
		{
			name:  "Complex expression 1",
			input: "_io.pos + (2 * my_var) == _parent.some_array[idx_val] && !(1.0 / 2.0)",
			expected: &BinOp{
				Op: BinOpAnd,
				Arg1: &BinOp{
					Op: BinOpEq,
					Arg1: &BinOp{
						Op:   BinOpAdd,
						Arg1: &Attr{Value: &Io{P: p(1)}, Name: "pos", P: p(5)},
						Arg2: &BinOp{Op: BinOpMul, Arg1: &IntLit{Value: 2, P: p(11)}, Arg2: &Id{Name: "my_var", P: p(15)}, P: p(13)},
						P:    p(9),
					},
					Arg2: &ArrayIdx{
						Value: &Attr{Value: &Parent{P: p(22)}, Name: "some_array", P: p(30)},
						Idx:   &Id{Name: "idx_val", P: p(41)},
						P:     p(22),
					},
					P: p(19),
				},
				Arg2: &UnOp{
					Op:  UnOpNot,
					Arg: &BinOp{Op: BinOpDiv, Arg1: &FltLit{Value: 1.0, P: p(50)}, Arg2: &FltLit{Value: 2.0, P: p(56)}, P: p(54)},
					P:   p(48),
				},
				P: p(45),
			},
		},
		{
			name:  "Complex expression 2: Ternary with calls and comparisons",
			input: "cond_val > 10 ? func(1, 2) : obj.meth().as<u2>()",
			expected: &TernaryOp{
				Cond: &BinOp{
					Op:   BinOpGt,
					Arg1: &Id{Name: "cond_val", P: p(1)},
					Arg2: &IntLit{Value: 10, P: p(13)},
					P:    p(10),
				},
				IfTrue: &Call{
					Value: &Id{Name: "func", P: p(17)},
					Args:  []Expr{&IntLit{Value: 1, P: p(22)}, &IntLit{Value: 2, P: p(25)}},
					P:     p(17),
				},
				IfFalse: &CastToType{
					Value: &Call{
						Value: &Attr{Value: &Id{Name: "obj", P: p(31)}, Name: "meth", P: p(35)},
						Args:  []Expr{},
						P:     p(31),
					},
					TypeName: "u2",
					P:        p(31),
				},
				P: p(15),
			},
		},
		// --- Error Cases ---
		{
			name:    "Missing closing parenthesis",
			input:   "(1 + 2",
			wantErr: true,
		},
		{
			name:    "Invalid operator sequence",
			input:   "1 ++ 2",
			wantErr: true, // Should error on the second '+'
		},
		{
			name:    "Unexpected token",
			input:   "1 2",
			wantErr: true,
		},
		{
			name:    "Invalid .as<> syntax (missing >)",
			input:   "foo.as<u4(",
			wantErr: true,
		},
		{
			name:    "Invalid .as<> syntax (missing parentheses)",
			input:   "foo.as<u4>",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewExpressionLexer(strings.NewReader(tt.input))
			p := NewExpressionParser(l)

			actualAST, err := p.Parse()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected an error for input '%s', but got none. AST: %v", tt.input, actualAST)
				}
				// Optionally, check specific error messages if needed:
				// if !strings.Contains(err.Error(), "expected next token to be RPAREN") { ... }
			} else {
				if err != nil {
					t.Errorf("Did not expect an error for input '%s', but got: %v. Errors: %v", tt.input, err, p.Errors())
					return
				}
				if actualAST == nil {
					t.Fatalf("Expected non-nil AST for input '%s', but got nil", tt.input)
				}

				if diff := cmp.Diff(tt.expected, actualAST, exprCmpOpts()); diff != "" {
					t.Errorf("AST mismatch for input '%s' (-want +got):\n%s", tt.input, diff)
				}
			}
		})
	}
}
