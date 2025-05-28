package expression

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to parse an expression and return the AST root or an error string
func parseExpr(t *testing.T, input string) (Expr, []string) {
	t.Helper()
	lexer := NewExpressionLexer(strings.NewReader(input))
	parser := NewExpressionParser(lexer)
	expr, err := parser.Parse()
	if err != nil {
		// Return parser errors for detailed checking if primary error is just "parsing errors"
		if strings.HasPrefix(err.Error(), "parsing errors:") {
			return nil, parser.Errors()
		}
		return nil, []string{err.Error()} // Return the direct error if it's not from p.errors
	}
	if len(parser.Errors()) > 0 {
		return nil, parser.Errors()
	}
	return expr, nil
}

// Helper to assert AST node type and basic properties.
// For more complex assertions, tests can do it directly.
func assertNodeType(t *testing.T, node Expr, expectedType interface{}, description string) {
	t.Helper()
	require.NotNil(t, node, "%s: AST node should not be nil", description)
	assert.IsType(t, expectedType, node, "%s: Node type mismatch", description)
}

func TestExpressionParser_Literals(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedAST Expr
	}{
		{"Integer", "123", &IntLit{Value: 123}},
		{"Float", "123.45", &FltLit{Value: 123.45}},
		{"String", `"hello"`, &StrLit{Value: "hello"}},
		{"BooleanTrue", "true", &BoolLit{Value: true}},
		{"BooleanFalse", "false", &BoolLit{Value: false}},
		{"Null", "null", &NullLit{}},
		{"HexInt", "0xCafeBabe", &IntLit{Value: 0xCafeBabe}},
		{"OctalInt", "0o755", &IntLit{Value: 0o755}},
		{"BinaryInt", "0b1010", &IntLit{Value: 0b1010}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, errs := parseExpr(t, tt.input)
			require.Empty(t, errs, "Parser errors: %v", errs)
			assertNodeType(t, ast, tt.expectedAST, "Literal type")
			// Compare values (ignoring Pos)
			switch expected := tt.expectedAST.(type) {
			case *IntLit:
				actual := ast.(*IntLit)
				assert.Equal(t, expected.Value, actual.Value)
			case *FltLit:
				actual := ast.(*FltLit)
				assert.Equal(t, expected.Value, actual.Value)
			case *StrLit:
				actual := ast.(*StrLit)
				assert.Equal(t, expected.Value, actual.Value)
			case *BoolLit:
				actual := ast.(*BoolLit)
				assert.Equal(t, expected.Value, actual.Value)
			case *NullLit:
				// No value to compare
			}
		})
	}
}

func TestExpressionParser_IdentifiersAndSpecialVars(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedAST Expr
	}{
		{"SimpleIdentifier", "my_var", &Id{Name: "my_var"}},
		{"Self", "_", &Self{}},
		{"Io", "_io", &Io{}},
		{"Parent", "_parent", &Parent{}},
		{"Root", "_root", &Root{}},
		{"BytesRemaining", "_bytes_remaining", &BytesRemaining{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, errs := parseExpr(t, tt.input)
			require.Empty(t, errs, "Parser errors: %v", errs)
			assertNodeType(t, ast, tt.expectedAST, "Identifier/Special Var type")
			if idExpected, ok := tt.expectedAST.(*Id); ok {
				idActual := ast.(*Id)
				assert.Equal(t, idExpected.Name, idActual.Name)
			}
		})
	}
}

func TestExpressionParser_UnaryOperations(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedOp  UnOpOp
		operandType interface{}
	}{
		{"LogicalNot", "!my_bool", UnOpNot, &Id{}},
		{"NumericNegation", "-my_num", UnOpNeg, &Id{}},
		{"BitwiseNot", "~my_int", UnOpBitwiseNot, &Id{}},
		{"NotTrue", "!true", UnOpNot, &BoolLit{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, errs := parseExpr(t, tt.input)
			require.Empty(t, errs, "Parser errors: %v", errs)
			assertNodeType(t, ast, &UnOp{}, "Unary operation")
			unOp := ast.(*UnOp)
			assert.Equal(t, tt.expectedOp, unOp.Op, "Unary operator mismatch")
			assertNodeType(t, unOp.Arg, tt.operandType, "Unary operand type")
		})
	}
}

func TestExpressionParser_BinaryOperations_Precedence(t *testing.T) {
	// Test basic precedence: a + b * c -> (a + (b * c))
	input := "a + b * c"
	ast, errs := parseExpr(t, input)
	require.Empty(t, errs, "Parser errors for '%s': %v", input, errs)
	assertNodeType(t, ast, &BinOp{}, "Root node should be BinOp (+)")
	binOpRoot := ast.(*BinOp)
	assert.Equal(t, BinOpAdd, binOpRoot.Op, "Root op should be +")
	assertNodeType(t, binOpRoot.Arg1, &Id{}, "LHS of + should be Id (a)")
	assert.Equal(t, "a", binOpRoot.Arg1.(*Id).Name)
	assertNodeType(t, binOpRoot.Arg2, &BinOp{}, "RHS of + should be BinOp (*)")
	binOpNested := binOpRoot.Arg2.(*BinOp)
	assert.Equal(t, BinOpMul, binOpNested.Op, "Nested op should be *")
	assertNodeType(t, binOpNested.Arg1, &Id{}, "LHS of * should be Id (b)")
	assert.Equal(t, "b", binOpNested.Arg1.(*Id).Name)
	assertNodeType(t, binOpNested.Arg2, &Id{}, "RHS of * should be Id (c)")
	assert.Equal(t, "c", binOpNested.Arg2.(*Id).Name)

	// Test grouping: (a + b) * c -> ((a + b) * c)
	inputGrouped := "(a + b) * c"
	astGrouped, errsGrouped := parseExpr(t, inputGrouped)
	require.Empty(t, errsGrouped, "Parser errors for '%s': %v", inputGrouped, errsGrouped)
	assertNodeType(t, astGrouped, &BinOp{}, "Root node should be BinOp (*)")
	binOpRootG := astGrouped.(*BinOp)
	assert.Equal(t, BinOpMul, binOpRootG.Op, "Root op should be *")
	assertNodeType(t, binOpRootG.Arg2, &Id{}, "RHS of * should be Id (c)")
	assert.Equal(t, "c", binOpRootG.Arg2.(*Id).Name)
	assertNodeType(t, binOpRootG.Arg1, &BinOp{}, "LHS of * should be BinOp (+)")
	binOpNestedG := binOpRootG.Arg1.(*BinOp)
	assert.Equal(t, BinOpAdd, binOpNestedG.Op, "Nested op should be +")
	assertNodeType(t, binOpNestedG.Arg1, &Id{}, "LHS of + should be Id (a)")
	assert.Equal(t, "a", binOpNestedG.Arg1.(*Id).Name)
	assertNodeType(t, binOpNestedG.Arg2, &Id{}, "RHS of + should be Id (b)")
	assert.Equal(t, "b", binOpNestedG.Arg2.(*Id).Name)
}

func TestExpressionParser_BinaryOperations_AllTypes(t *testing.T) {
	ops := []struct {
		opStr  string
		opEnum BinOpOp
	}{
		{"+", BinOpAdd}, {"-", BinOpSub}, {"*", BinOpMul}, {"/", BinOpDiv}, {"%", BinOpMod},
		{"==", BinOpEq}, {"!=", BinOpNotEq},
		{"<", BinOpLt}, {">", BinOpGt}, {"<=", BinOpLtEq}, {">=", BinOpGtEq},
		{"&&", BinOpAnd}, {"||", BinOpOr},
		{"&", BinOpBitwiseAnd}, {"|", BinOpBitwiseOr}, {"^", BinOpBitwiseXor},
		{"<<", BinOpLShift}, {">>", BinOpRShift},
	}

	for _, opTest := range ops {
		t.Run(opTest.opStr, func(t *testing.T) {
			input := fmt.Sprintf("var1 %s var2", opTest.opStr)
			ast, errs := parseExpr(t, input)
			require.Empty(t, errs, "Parser errors for '%s': %v", input, errs)
			assertNodeType(t, ast, &BinOp{}, "Binary operation root")
			binOp := ast.(*BinOp)
			assert.Equal(t, opTest.opEnum, binOp.Op, "Operator enum mismatch")
			assertNodeType(t, binOp.Arg1, &Id{Name: "var1"}, "LHS")
			assertNodeType(t, binOp.Arg2, &Id{Name: "var2"}, "RHS")
		})
	}
}

func TestExpressionParser_AttributeAccess(t *testing.T) {
	input := "obj.field1.field2"
	ast, errs := parseExpr(t, input)
	require.Empty(t, errs, "Parser errors for '%s': %v", input, errs)

	// Expected: Attr{Value: Attr{Value: Id{obj}, Name:"field1"}, Name:"field2"}
	assertNodeType(t, ast, &Attr{}, "Root node (obj.field1.field2)")
	attr2 := ast.(*Attr)
	assert.Equal(t, "field2", attr2.Name)

	assertNodeType(t, attr2.Value, &Attr{}, "Nested node (obj.field1)")
	attr1 := attr2.Value.(*Attr)
	assert.Equal(t, "field1", attr1.Name)

	assertNodeType(t, attr1.Value, &Id{}, "Innermost node (obj)")
	assert.Equal(t, "obj", attr1.Value.(*Id).Name)
}

func TestExpressionParser_ArrayIndex(t *testing.T) {
	input := "my_array[index_val][0]"
	ast, errs := parseExpr(t, input)
	require.Empty(t, errs, "Parser errors for '%s': %v", input, errs)

	// Expected: ArrayIdx{Value: ArrayIdx{Value: Id{my_array}, Idx: Id{index_val}}, Idx: IntLit{0}}
	assertNodeType(t, ast, &ArrayIdx{}, "Root node (my_array[index_val][0])")
	outerIdx := ast.(*ArrayIdx)
	assertNodeType(t, outerIdx.Idx, &IntLit{}, "Outer index should be IntLit(0)")
	assert.Equal(t, int64(0), outerIdx.Idx.(*IntLit).Value)

	assertNodeType(t, outerIdx.Value, &ArrayIdx{}, "Inner node (my_array[index_val])")
	innerIdx := outerIdx.Value.(*ArrayIdx)
	assertNodeType(t, innerIdx.Idx, &Id{}, "Inner index should be Id(index_val)")
	assert.Equal(t, "index_val", innerIdx.Idx.(*Id).Name)

	assertNodeType(t, innerIdx.Value, &Id{}, "Array base should be Id(my_array)")
	assert.Equal(t, "my_array", innerIdx.Value.(*Id).Name)
}

func TestExpressionParser_FunctionCalls(t *testing.T) {
	t.Run("GlobalFunction", func(t *testing.T) {
		input := "my_func(arg1, 20)"
		ast, errs := parseExpr(t, input)
		require.Empty(t, errs, "Parser errors: %v", errs)
		assertNodeType(t, ast, &Call{}, "Call node")
		callNode := ast.(*Call)
		assertNodeType(t, callNode.Value, &Id{Name: "my_func"}, "Function name")
		require.Len(t, callNode.Args, 2, "Number of arguments")
		assertNodeType(t, callNode.Args[0], &Id{Name: "arg1"}, "Arg1 type")
		assertNodeType(t, callNode.Args[1], &IntLit{Value: 20}, "Arg2 type")
	})

	t.Run("MethodCall", func(t *testing.T) {
		input := "obj.method(arg1)"
		ast, errs := parseExpr(t, input)
		require.Empty(t, errs, "Parser errors: %v", errs)
		assertNodeType(t, ast, &Call{}, "Call node")
		callNode := ast.(*Call)
		assertNodeType(t, callNode.Value, &Attr{}, "Method attribute")
		attrNode := callNode.Value.(*Attr)
		assert.Equal(t, "method", attrNode.Name)
		assertNodeType(t, attrNode.Value, &Id{Name: "obj"}, "Object identifier")
		require.Len(t, callNode.Args, 1, "Number of arguments")
		assertNodeType(t, callNode.Args[0], &Id{Name: "arg1"}, "Arg1 type")
	})

	t.Run("ChainedCallsAndAttributes", func(t *testing.T) {
		input := "obj.prop.method1().another_prop[0].call2(x)"
		// Expected AST roughly:
		// Call{ // call2(x)
		//   Value: Attr{ // .call2
		//     Value: ArrayIdx{ // [0]
		//       Value: Attr{ // .another_prop
		//         Value: Call{ // method1()
		//           Value: Attr{ // .method1
		//             Value: Attr{ // .prop
		//               Value: Id{obj}, Name: "prop"},
		//             Name: "method1"},
		//           Args: []},
		//         Name: "another_prop"},
		//       Idx: IntLit{0}},
		//     Name: "call2"},
		//   Args: [Id{x}]}

		ast, errs := parseExpr(t, input)
		require.Empty(t, errs, "Parser errors for '%s': %v", input, errs)
		assertNodeType(t, ast, &Call{}, "Outer call node (call2)")
		call2Node := ast.(*Call)
		assertNodeType(t, call2Node.Args[0], &Id{Name: "x"}, "Arg for call2")

		attrCall2 := call2Node.Value.(*Attr)
		assert.Equal(t, "call2", attrCall2.Name)

		arrayIdxNode := attrCall2.Value.(*ArrayIdx)
		assertNodeType(t, arrayIdxNode.Idx, &IntLit{Value: 0}, "Index for array")

		attrAnotherProp := arrayIdxNode.Value.(*Attr)
		assert.Equal(t, "another_prop", attrAnotherProp.Name)

		call1Node := attrAnotherProp.Value.(*Call)
		require.Empty(t, call1Node.Args, "Args for method1 should be empty")

		attrMethod1 := call1Node.Value.(*Attr)
		assert.Equal(t, "method1", attrMethod1.Name)

		attrProp := attrMethod1.Value.(*Attr)
		assert.Equal(t, "prop", attrProp.Name)
		assertNodeType(t, attrProp.Value, &Id{Name: "obj"}, "Base object")
	})
}

func TestExpressionParser_TypeCasts(t *testing.T) {
	input := "value.as<my_type>()"
	ast, errs := parseExpr(t, input)
	require.Empty(t, errs, "Parser errors: %v", errs)
	assertNodeType(t, ast, &CastToType{}, "CastToType node")
	castNode := ast.(*CastToType)
	assert.Equal(t, "my_type", castNode.TypeName)
	assertNodeType(t, castNode.Value, &Id{Name: "value"}, "Value being cast")
}

func TestExpressionParser_TernaryOperator(t *testing.T) {
	input := "cond_expr ? true_expr : false_expr"
	ast, errs := parseExpr(t, input)
	require.Empty(t, errs, "Parser errors: %v", errs)
	assertNodeType(t, ast, &TernaryOp{}, "TernaryOp node")
	ternaryNode := ast.(*TernaryOp)
	assertNodeType(t, ternaryNode.Cond, &Id{Name: "cond_expr"}, "Condition")
	assertNodeType(t, ternaryNode.IfTrue, &Id{Name: "true_expr"}, "True expression")
	assertNodeType(t, ternaryNode.IfFalse, &Id{Name: "false_expr"}, "False expression")

	// Test precedence: a && b ? c : d || e -> (a && b) ? c : (d || e)
	// CEL translation will be ternary((a && b), c, (d || e))
	// Parser should produce TernaryOp{ Cond: BinOp{AND}, IfTrue: Id{c}, IfFalse: BinOp{OR} }
	inputPrec := "a && b ? c : d || e"
	astPrec, errsPrec := parseExpr(t, inputPrec)
	require.Empty(t, errsPrec, "Parser errors for '%s': %v", inputPrec, errsPrec)
	assertNodeType(t, astPrec, &TernaryOp{}, "TernaryOp node for precedence test")
	ternaryPrecNode := astPrec.(*TernaryOp)
	assertNodeType(t, ternaryPrecNode.Cond, &BinOp{}, "Ternary condition should be BinOp (&&)")
	assert.Equal(t, BinOpAnd, ternaryPrecNode.Cond.(*BinOp).Op)
	assertNodeType(t, ternaryPrecNode.IfTrue, &Id{Name: "c"}, "Ternary true branch")
	assertNodeType(t, ternaryPrecNode.IfFalse, &BinOp{}, "Ternary false branch should be BinOp (||)")
	assert.Equal(t, BinOpOr, ternaryPrecNode.IfFalse.(*BinOp).Op)
}

func TestExpressionParser_Errors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		errorContains string
	}{
		{"UnterminatedString", `"hello`, "unexpected EOF"},
		{"UnexpectedToken", `1 2`, "unexpected token NUMBER(2) at 1:3 after NUMBER(1) at 1:1"}, // More specific now
		{"MissingOperand", `a +`, "no prefix parse function for EOF"},
		{"MissingClosingParen", `(a + b`, "expected next token to be RPAREN, got EOF instead"},
		{"MissingClosingBracket", `a[b`, "expected next token to be RBRACKET, got EOF instead"},
		{"DotWithoutIdentifier", `a.`, "expected identifier after '.', got EOF"},
		{"CastWithoutType", `a.as<>`, "expected type name after 'as<', got GT"},
		{"CastUnterminated", `a.as<type`, "expected next token to be GT, got EOF"},
		{"TernaryMissingColon", `a ? b`, "expected next token to be TERNARY_COLON, got EOF"},
		{"TernaryMissingFalse", `a ? b :`, "no prefix parse function for EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := parseExpr(t, tt.input)
			require.NotEmpty(t, errs, "Expected parser errors for input: %s", tt.input)
			// Join errors for easier matching if multiple specific errors are relevant
			fullError := strings.Join(errs, "; ")
			assert.Contains(t, fullError, tt.errorContains, "Error message mismatch")
		})
	}
}

func TestExpressionParser_ComplexPreviouslyHardcoded(t *testing.T) {
	// These were previously hardcoded in ExpressionParser.Parse()
	// Now they should be parsed by the generic Pratt parser.
	expressions := []string{
		"_io.pos + (2 * my_var) == _parent.some_array[idx_val] && !(1.0 / 2.0)",
		"cond_val > 10 ? func(1, 2) : obj.meth().as<u2>()",
		"a & b | c ^ d << e >> f",     // This was also special-cased
		"cond ? val_true : val_false", // And this
	}

	for i, exprStr := range expressions {
		t.Run(fmt.Sprintf("Complex%d", i+1), func(t *testing.T) {
			ast, errs := parseExpr(t, exprStr)
			require.Empty(t, errs, "Parser errors for '%s': %v", exprStr, errs)
			require.NotNil(t, ast, "AST should not be nil for '%s'", exprStr)
			// For these, we're mostly checking that they parse without error.
			// Specific AST structure validation would be more involved here but
			// covered by ASTTransformer tests if the transformations are correct.
			// A smoke test for String() to catch panics in AST representation.
			assert.NotEmpty(t, ast.String(), "AST String() method should not return empty")
		})
	}
}
