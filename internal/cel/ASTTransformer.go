package cel

import (
	"fmt"
	"strconv"
	"strings"

	expr "github.com/twinfer/kbin-plugin/pkg/expression"
)

// ASTTransformer transforms a Kaitai expression AST into a CEL expression string and implements the expr.Visitor interface.
type ASTTransformer struct {
	// You might need to add fields here to track context, e.g., a symbol table
	// or a reference to the Kaitai schema for type information.
	sb strings.Builder
}

// NewASTTransformer creates a new ASTTransformer.
func NewASTTransformer() *ASTTransformer {
	return &ASTTransformer{}
}

// Transform traverses the Kaitai AST (expr.Expr) and generates the CEL expression string.
func (t *ASTTransformer) Transform(node expr.Expr) (string, error) {
	t.sb.Reset()          // Reset the string builder for each transformation
	err := node.Accept(t) // Use the visitor pattern to traverse the AST
	if err != nil {
		return "", fmt.Errorf("failed to transform AST: %w", err)
	}
	return t.sb.String(), nil
}

// Implement the expression.Visitor interface methods:

// Implementations for Literal nodes
func (t *ASTTransformer) VisitBoolLit(node *expr.BoolLit) error {
	t.sb.WriteString(fmt.Sprintf("%t", node.Value))
	return nil
}

func (t *ASTTransformer) VisitIntLit(node *expr.IntLit) error {
	t.sb.WriteString(fmt.Sprintf("%d", node.Value))
	return nil
}

func (t *ASTTransformer) VisitStrLit(node *expr.StrLit) error {
	// Escape string literals for CEL
	t.sb.WriteString(strconv.Quote(node.Value))
	return nil
}

func (t *ASTTransformer) VisitFltLit(node *expr.FltLit) error {
	t.sb.WriteString(fmt.Sprintf("%g", node.Value))
	return nil
}

func (t *ASTTransformer) VisitNullLit(node *expr.NullLit) error {
	t.sb.WriteString("null")
	return nil
}

// Implementations for Identifier and Special Variable nodes
func (t *ASTTransformer) VisitId(node *expr.Id) error {
	// Map Kaitai identifier to CEL variable name
	t.sb.WriteString(node.Name)
	return nil
}

func (t *ASTTransformer) VisitSelf(node *expr.Self) error { t.sb.WriteString("_"); return nil }
func (t *ASTTransformer) VisitIo(node *expr.Io) error     { t.sb.WriteString("_io"); return nil }
func (t *ASTTransformer) VisitParent(node *expr.Parent) error {
	t.sb.WriteString("_parent")
	return nil
}
func (t *ASTTransformer) VisitRoot(node *expr.Root) error { t.sb.WriteString("_root"); return nil }
func (t *ASTTransformer) VisitBytesRemaining(node *expr.BytesRemaining) error {
	t.sb.WriteString("_bytes_remaining")
	return nil
}

// Implementations for Operations

/*
func (t *ASTTransformer) VisitLiteral(node *expr.LiteralNode) error { // This method signature is incorrect based on expr.Expr types
	// Map Kaitai literal to CEL literal representation
	switch v := node.Value.(type) {
	case int:
		t.sb.WriteString(fmt.Sprintf("%d", v))
	case int64:
		t.sb.WriteString(fmt.Sprintf("%d", v))
	case float64:
		t.sb.WriteString(fmt.Sprintf("%f", v))
	case string:
		// Escape string literals for CEL
		t.sb.WriteString(strconv.Quote(v))
	case bool:
		t.sb.WriteString(fmt.Sprintf("%t", v))
	case []byte:
		// Represent bytes as a byte literal in CEL (if supported) or a function call
		// For now, let's represent as a string literal (might need adjustment based on CEL support)
		t.sb.WriteString(fmt.Sprintf("b%s", strconv.Quote(string(v))))
	default:
		return fmt.Errorf("unsupported literal type: %T", v)
	}
	return nil
}

*/

func (t *ASTTransformer) VisitBinOp(node *expr.BinOp) error {
	// For operators that map to CEL functions (e.g., bitwise operations)
	switch node.Op {
	case expr.BinOpBitwiseAnd, expr.BinOpBitwiseOr, expr.BinOpBitwiseXor, expr.BinOpLShift, expr.BinOpRShift:
		celFuncName := ""
		switch node.Op {
		case expr.BinOpBitwiseAnd:
			celFuncName = "bitAnd"
		case expr.BinOpBitwiseOr:
			celFuncName = "bitOr"
		case expr.BinOpBitwiseXor:
			celFuncName = "bitXor"
		case expr.BinOpLShift:
			celFuncName = "bitShiftLeft"
		case expr.BinOpRShift:
			celFuncName = "bitShiftRight"
		}
		t.sb.WriteString(celFuncName)
		t.sb.WriteString("(")
		err := node.Arg1.Accept(t)
		if err != nil {
			return err
		}
		t.sb.WriteString(", ")
		err = node.Arg2.Accept(t)
		if err != nil {
			return err
		}
		t.sb.WriteString(")")
		return nil
	default:
		// For standard infix operators
		t.sb.WriteString("(") // Outer parentheses for the whole operation for safety, CEL precedence will apply
		err := node.Arg1.Accept(t)
		if err != nil {
			return err
		}

		operatorStr := ""
		switch node.Op {
		case expr.BinOpAdd:
			operatorStr = " + "
		case expr.BinOpSub:
			operatorStr = " - "
		case expr.BinOpMul:
			operatorStr = " * "
		case expr.BinOpDiv:
			operatorStr = " / "
		case expr.BinOpMod:
			operatorStr = " % "
		case expr.BinOpEq:
			operatorStr = " == "
		case expr.BinOpNotEq:
			operatorStr = " != "
		case expr.BinOpLt:
			operatorStr = " < "
		case expr.BinOpGt:
			operatorStr = " > "
		case expr.BinOpLtEq:
			operatorStr = " <= "
		case expr.BinOpGtEq:
			operatorStr = " >= "
		case expr.BinOpAnd: // Logical AND
			operatorStr = " && "
		case expr.BinOpOr: // Logical OR
			operatorStr = " || "
		default:
			return fmt.Errorf("unsupported binary operator: %s", node.Op.String())
		}
		t.sb.WriteString(operatorStr)

		err = node.Arg2.Accept(t)
		if err != nil {
			return err
		}
		t.sb.WriteString(")") // Close outer parentheses
		return nil
	}
}

func (t *ASTTransformer) VisitUnOp(node *expr.UnOp) error {
	opStr := ""
	isPrefixStyle := true
	switch node.Op {
	case expr.UnOpNot:
		opStr = "!"
	case expr.UnOpNeg:
		opStr = "-"
	case expr.UnOpBitwiseNot:
		opStr = "bitNot" // CEL function bitNot(arg)
		isPrefixStyle = false
	default:
		return fmt.Errorf("unsupported unary operator: %s", node.Op.String())
	}

	if isPrefixStyle {
		t.sb.WriteString(opStr)
		return node.Arg.Accept(t)
	} else {
		// Function call style for operators like bitNot
		t.sb.WriteString(opStr)
		t.sb.WriteString("(")
		err := node.Arg.Accept(t)
		if err != nil {
			return err
		}
		t.sb.WriteString(")")
		return nil
	}
}

func (t *ASTTransformer) VisitAttr(node *expr.Attr) error {
	// Handle _io attributes that are function calls in CEL without arguments,
	// e.g., Kaitai `_io.pos` -> CEL `pos(_io)`
	if ioNode, ok := node.Value.(*expr.Io); ok {
		celFuncName := ""
		isIoAttrFunc := false
		switch node.Name {
		case "pos":
			celFuncName = "pos" // Maps to pos(_io)
			isIoAttrFunc = true
		case "size":
			celFuncName = "stream_size" // Maps to stream_size(_io)
			isIoAttrFunc = true
		case "eof", "is_eof":
			celFuncName = "isEOF" // Maps to isEOF(_io)
			isIoAttrFunc = true
		}

		if isIoAttrFunc {
			t.sb.WriteString(celFuncName)
			t.sb.WriteString("(")
			err := ioNode.Accept(t) // This writes "_io"
			if err != nil {
				return err
			}
			t.sb.WriteString(")")
			return nil
		}
		// If not one of these specific attributes, it might be an _io attribute
		// that's part of a Call node (e.g., _io.read_u1).
		// In that case, VisitCall will handle the transformation.
		// Here, we just write the receiver.attribute form.
	}

	// Generic attribute access: receiver.name
	err := node.Value.Accept(t) // receiver
	if err != nil {
		return err
	}
	t.sb.WriteString(".")
	t.sb.WriteString(node.Name) // attribute name
	return nil
}

func (t *ASTTransformer) VisitArrayIdx(node *expr.ArrayIdx) error {
	// Map Kaitai array access to CEL `at` function call
	t.sb.WriteString("at(")

	// Transform the array/list
	err := node.Value.Accept(t)
	if err != nil {
		return err
	}

	t.sb.WriteString(", ")

	// Transform the index, using node.ArrayIdx.Idx
	err = node.Idx.Accept(t)
	if err != nil {
		return err
	}

	t.sb.WriteString(")")
	return nil
}

func (t *ASTTransformer) VisitCall(node *expr.Call) error {
	// Case 1: Method call on an object, e.g., obj.method(args) or _io.stream_method(args)
	if attrNode, ok := node.Value.(*expr.Attr); ok {
		receiver := attrNode.Value
		methodName := attrNode.Name
		celFuncName := ""

		// Special handling for _io methods like _io.read_u1() -> readU1(_io, args...)
		if _, okIo := receiver.(*expr.Io); okIo {
			mappedName, found := mapKaitaiIoMethodToCEL(methodName)
			if !found {
				return fmt.Errorf("unsupported method '%s' on _io", methodName)
			}
			celFuncName = mappedName
		} else {
			// Generic object method: obj.method(args) -> mapped_method(obj, args)
			// This assumes methods on user types are mapped to global CEL functions
			// where the receiver becomes the first argument.
			mappedName, found := mapKaitaiFunctionToCEL(methodName)
			if !found {
				// If not in a specific map, assume the method name is the CEL function name.
				// This could be an error if no such global CEL function exists.
				celFuncName = methodName
			} else {
				celFuncName = mappedName
			}
		}

		t.sb.WriteString(celFuncName)
		t.sb.WriteString("(")

		// Receiver is the first argument for these mapped methods
		err := receiver.Accept(t)
		if err != nil {
			return err
		}

		// Append other arguments from the call
		if len(node.Args) > 0 {
			t.sb.WriteString(", ")
			for i, arg := range node.Args {
				if i > 0 {
					t.sb.WriteString(", ")
				}
				err := arg.Accept(t)
				if err != nil {
					return err
				}
			}
		}
		t.sb.WriteString(")")
		return nil
	}

	// Case 2: Global function call, e.g., func(args)
	if idNode, ok := node.Value.(*expr.Id); ok {
		celFuncName, found := mapKaitaiFunctionToCEL(idNode.Name)
		if !found {
			// If the function is not in our map, it might be a direct CEL function or an error.
			// For now, use the name directly.
			t.sb.WriteString(idNode.Name)
		} else {
			t.sb.WriteString(celFuncName)
		}

		t.sb.WriteString("(")
		for i, arg := range node.Args {
			if i > 0 {
				t.sb.WriteString(", ")
			}
			err := arg.Accept(t)
			if err != nil {
				return err
			}
		}
		t.sb.WriteString(")")
		return nil
	}

	return fmt.Errorf("unsupported call on AST node type %T", node.Value)
}

func (t *ASTTransformer) VisitTernaryOp(node *expr.TernaryOp) error {
	// Map Kaitai ternary operator to CEL ternary function call
	t.sb.WriteString("ternary(")

	// Transform condition
	err := node.Cond.Accept(t)
	if err != nil {
		return err
	}

	t.sb.WriteString(", ")

	// Transform true expression
	err = node.IfTrue.Accept(t)
	if err != nil {
		return err
	}

	t.sb.WriteString(", ")

	// Transform false expression
	err = node.IfFalse.Accept(t)
	if err != nil {
		return err
	}

	t.sb.WriteString(")")
	return nil
}

// Implementations for Type Conversions / Built-ins
func (t *ASTTransformer) VisitCastToType(node *expr.CastToType) error {
	// Map Kaitai cast to CEL conversion function (e.g., `to_i`, `to_f`)
	// This requires mapping Kaitai type names to CEL function names
	celFunctionName, found := mapKaitaiTypeToCELConversion(node.TypeName)
	if !found {
		return fmt.Errorf("unsupported cast type: %s", node.TypeName)
	}
	t.sb.WriteString(celFunctionName)
	t.sb.WriteString("(")
	err := node.Value.Accept(t)
	if err != nil {
		return err
	}
	t.sb.WriteString(")")
	return nil
}

func (t *ASTTransformer) VisitSizeOf(node *expr.SizeOf) error {
	return fmt.Errorf("sizeof not directly supported in CEL transformation")
} // Or map to a function if possible
func (t *ASTTransformer) VisitAlignOf(node *expr.AlignOf) error {
	return fmt.Errorf("alignof not directly supported in CEL transformation")
} // Or map to a function if possible

// Add visit methods for any other AST node types you have defined.

// Helper function to map Kaitai type names to CEL conversion function names
func mapKaitaiTypeToCELConversion(kaitaiTypeName string) (string, bool) {
	mapping := map[string]string{
		"s1": "to_i", "s2": "to_i", "s4": "to_i", "s8": "to_i",
		"u1": "to_i", "u2": "to_i", "u4": "to_i", "u8": "to_i",
		"f4": "to_f", "f8": "to_f",
	}
	celName, found := mapping[strings.ToLower(kaitaiTypeName)] // Case-insensitive matching
	return celName, found
}

// Helper function to map Kaitai function names to CEL function names
func mapKaitaiFunctionToCEL(kaitaiFuncName string) (string, bool) {
	mapping := map[string]string{
		"bytes2str": "bytesToStr", // bytes2str(input)
		"reverse":   "reverse",    // reverse(input)
		"size":      "size",       // size(input)
		"pos":       "pos",        // pos(_io)
		"eof":       "isEOF",      // isEOF(_io)
		"abs":       "abs",
		"min":       "min",
		"max":       "max",
		"ceil":      "ceil",
		"floor":     "floor",
		"round":     "round",
		// Add mappings for other Kaitai functions you support
		// Process functions:
		"process_xor":    "processXOR", // Assuming process functions are prefixed
		"process_zlib":   "processZlib",
		"process_rotate": "processRotateLeft", // Assuming process_rotate maps to rotateLeft
		// Encoding functions
		"encode_string": "encodeString", // encodeString(str, encoding)
		"decode_string": "decodeString",
		// Writer functions
		"new_writer":    "newWriter",
		"writer_buffer": "writerBuffer",
		"write_bytes":   "writeBytes",
		"write_u1":      "writeU1",
		"write_u2le":    "writeU2le",
		"write_u4le":    "writeU4le",
		"write_u8le":    "writeU8le",
		"write_u2be":    "writeU2be",
		"write_u4be":    "writeU4be",
		"write_u8be":    "writeU8be",
		"write_s1":      "writeS1",
		"write_s2le":    "writeS2le",
		"write_s4le":    "writeS4le",
		"write_s8le":    "writeS8le",
		"write_f4le":    "writeF4le",
		"write_f8le":    "writeF8le",
		"write":         "write", // Unified write function
		"writer_pos":    "writerPos",
	}
	celName, found := mapping[strings.ToLower(kaitaiFuncName)] // Case-insensitive matching
	return celName, found
}

// mapKaitaiIoMethodToCEL maps Kaitai _io method names (when called as _io.method())
// to CEL function names. These CEL functions typically take _io as their first argument.
func mapKaitaiIoMethodToCEL(kaitaiMethodName string) (string, bool) {
	// Based on streamOperations.go for functions that are CALLED on _io
	mapping := map[string]string{
		// Methods called with arguments or no arguments but are explicit calls:
		"read_u1":         "readU1",   // _io.read_u1() -> readU1(_io)
		"read_u2le":       "readU2le", // _io.read_u2le() -> readU2le(_io)
		"read_u4le":       "readU4le",
		"read_u8le":       "readU8le",
		"read_s1":         "readS1",
		"read_s2le":       "readS2le",
		"read_s4le":       "readS4le",
		"read_s8le":       "readS8le",
		"read_u2be":       "readU2be",
		"read_u4be":       "readU4be",
		"read_u8be":       "readU8be",
		"read_bytes":      "readBytes",     // _io.read_bytes(len) -> readBytes(_io, len)
		"read_bytes_full": "readBytesFull", // _io.read_bytes_full() -> readBytesFull(_io)
		"read_bytes_term": "readBytesTerm", // _io.read_bytes_term(...) -> readBytesTerm(_io, ...)
		"seek":            "seek",          // _io.seek(offset) -> seek(_io, offset)
		"read":            "read",          // _io.read(type_str) -> read(_io, type_str)
	}
	celName, found := mapping[strings.ToLower(kaitaiMethodName)]
	return celName, found
}

// Helper function to convert values to int64 - might be needed for argument type checking
// func toInt(val ref.Val) (int64, error) {
// 	switch v := val.(type) {
// 	case types.Int:
// 		return int64(v), nil
// 	case types.Double:
// 		return int64(v), nil // Potential loss of precision
// 	case types.String:
// 		return strconv.ParseInt(string(v), 10, 64)
// 	default:
// 		return 0, fmt.Errorf("cannot convert %T to int", val.Value())
// 	}
// }
