package expression

import (
	"fmt"
	"strings"
)

// Loc.Pos maps to this struct for source position
type Pos struct {
	Line   int
	Column int
}

// Expr is the base interface for all expression AST nodes (like Scala's `sealed trait Expr`)
// It includes the Accept method for the Visitor pattern.
type Expr interface {
	Pos() Pos             // All expression nodes should carry their source position
	String() string       // For debugging/representation
	Accept(Visitor) error // Method to accept a visitor
}

// Visitor defines the interface for traversing the expression AST.
type Visitor interface {
	VisitBoolLit(*BoolLit) error
	VisitIntLit(*IntLit) error
	VisitStrLit(*StrLit) error
	VisitFltLit(*FltLit) error
	VisitNullLit(*NullLit) error
	VisitId(*Id) error
	VisitSelf(*Self) error
	VisitIo(*Io) error
	VisitParent(*Parent) error
	VisitRoot(*Root) error
	VisitBytesRemaining(*BytesRemaining) error
	VisitUnOp(*UnOp) error
	VisitBinOp(*BinOp) error
	VisitTernaryOp(*TernaryOp) error
	VisitAttr(*Attr) error
	VisitCall(*Call) error
	VisitArrayIdx(*ArrayIdx) error
	VisitCastToType(*CastToType) error
	VisitSizeOf(*SizeOf) error
	VisitAlignOf(*AlignOf) error
}

// --- Literal Expressions (like Scala's case classes for literals) ---

type BoolLit struct {
	Value bool
	P     Pos // Embedded position
}

func (b *BoolLit) Pos() Pos               { return b.P }
func (b *BoolLit) String() string         { return fmt.Sprintf("%t", b.Value) }
func (b *BoolLit) Accept(v Visitor) error { return v.VisitBoolLit(b) }

type IntLit struct {
	Value int64 // Use int64 for Long
	P     Pos
}

func (i *IntLit) Pos() Pos               { return i.P }
func (i *IntLit) String() string         { return fmt.Sprintf("%d", i.Value) }
func (i *IntLit) Accept(v Visitor) error { return v.VisitIntLit(i) }

type StrLit struct {
	Value string
	P     Pos
}

func (s *StrLit) Pos() Pos               { return s.P }
func (s *StrLit) String() string         { return fmt.Sprintf("%q", s.Value) }
func (s *StrLit) Accept(v Visitor) error { return v.VisitStrLit(s) }

type FltLit struct {
	Value float64 // Use float64 for Double
	P     Pos
}

func (f *FltLit) Pos() Pos               { return f.P }
func (f *FltLit) String() string         { return fmt.Sprintf("%g", f.Value) }
func (f *FltLit) Accept(v Visitor) error { return v.VisitFltLit(f) }

type NullLit struct {
	P Pos
}

func (n *NullLit) Pos() Pos               { return n.P }
func (n *NullLit) Accept(v Visitor) error { return v.VisitNullLit(n) }
func (n *NullLit) String() string         { return "null" }

// --- Identifiers / Variables ---

type Id struct {
	Name string
	P    Pos
}

func (id *Id) Pos() Pos               { return id.P }
func (id *Id) String() string         { return id.Name }
func (id *Id) Accept(v Visitor) error { return v.VisitId(id) }

// Special variables: _ (self), _io, _parent, _root, _bytes_remaining
type Self struct {
	P Pos
}

func (s *Self) Pos() Pos               { return s.P }
func (s *Self) String() string         { return "_" }
func (s *Self) Accept(v Visitor) error { return v.VisitSelf(s) }

type Io struct {
	P Pos
}

func (i *Io) Pos() Pos               { return i.P }
func (i *Io) String() string         { return "_io" }
func (i *Io) Accept(v Visitor) error { return v.VisitIo(i) }

type Parent struct {
	P Pos
}

func (p *Parent) Pos() Pos               { return p.P }
func (p *Parent) String() string         { return "_parent" }
func (p *Parent) Accept(v Visitor) error { return v.VisitParent(p) }

type Root struct {
	P Pos
}

func (r *Root) Pos() Pos               { return r.P }
func (r *Root) String() string         { return "_root" }
func (r *Root) Accept(v Visitor) error { return v.VisitRoot(r) }

type BytesRemaining struct {
	P Pos
}

func (br *BytesRemaining) Pos() Pos               { return br.P }
func (br *BytesRemaining) String() string         { return "_bytes_remaining" }
func (br *BytesRemaining) Accept(v Visitor) error { return v.VisitBytesRemaining(br) }

// --- Operations ---

// UnOpOp (Unary Operator Type)
type UnOpOp int

const (
	UnOpNot        UnOpOp = iota // !
	UnOpBitwiseNot               // ~
	UnOpNeg                      // - (unary negation)
)

func (op UnOpOp) String() string {
	switch op {
	case UnOpNot:
		return "!"
	case UnOpBitwiseNot:
		return "~"
	case UnOpNeg:
		return "-"
	default:
		return "UNKNOWN_UNOP"
	}
}

type UnOp struct {
	Op  UnOpOp
	Arg Expr
	P   Pos
}

func (uo *UnOp) Pos() Pos               { return uo.P }
func (uo *UnOp) String() string         { return fmt.Sprintf("(%s%s)", uo.Op, uo.Arg) }
func (uo *UnOp) Accept(v Visitor) error { return v.VisitUnOp(uo) }

// BinOpOp (Binary Operator Type)
type BinOpOp int

const (
	// Arithmetic
	BinOpAdd BinOpOp = iota // +
	BinOpSub                // -
	BinOpMul                // *
	BinOpDiv                // /
	BinOpMod                // %
	// Bitwise
	BinOpLShift     // <<
	BinOpRShift     // >>
	BinOpBitwiseAnd // &
	BinOpBitwiseOr  // |
	BinOpBitwiseXor // ^
	// Comparison
	BinOpEq    // ==
	BinOpNotEq // !=
	BinOpLt    // <
	BinOpGt    // >
	BinOpLtEq  // <=
	BinOpGtEq  // >=
	// Logical
	BinOpAnd // &&
	BinOpOr  // ||
)

func (op BinOpOp) String() string {
	switch op {
	case BinOpAdd:
		return "+"
	case BinOpSub:
		return "-"
	case BinOpMul:
		return "*"
	case BinOpDiv:
		return "/"
	case BinOpMod:
		return "%"
	case BinOpLShift:
		return "<<"
	case BinOpRShift:
		return ">>"
	case BinOpBitwiseAnd:
		return "&"
	case BinOpBitwiseOr:
		return "|"
	case BinOpBitwiseXor:
		return "^"
	case BinOpEq:
		return "=="
	case BinOpNotEq:
		return "!="
	case BinOpLt:
		return "<"
	case BinOpGt:
		return ">"
	case BinOpLtEq:
		return "<="
	case BinOpGtEq:
		return ">="
	case BinOpAnd:
		return "&&"
	case BinOpOr:
		return "||"
	default:
		return "UNKNOWN_BINOP"
	}
}

type BinOp struct {
	Op   BinOpOp
	Arg1 Expr
	Arg2 Expr
	P    Pos
}

func (bo *BinOp) Pos() Pos               { return bo.P }
func (bo *BinOp) String() string         { return fmt.Sprintf("(%s %s %s)", bo.Arg1, bo.Op, bo.Arg2) }
func (bo *BinOp) Accept(v Visitor) error { return v.VisitBinOp(bo) }

type TernaryOp struct {
	Cond    Expr
	IfTrue  Expr
	IfFalse Expr
	P       Pos
}

func (to *TernaryOp) Pos() Pos { return to.P }
func (to *TernaryOp) String() string {
	return fmt.Sprintf("(%s ? %s : %s)", to.Cond, to.IfTrue, to.IfFalse)
}
func (to *TernaryOp) Accept(v Visitor) error {
	return v.VisitTernaryOp(to)
}

// --- Member access / function calls / array access ---

type Attr struct {
	Value Expr   // The expression being accessed (e.g., `foo`)
	Name  string // The attribute name (e.g., `bar` in `foo.bar`)
	P     Pos
}

func (a *Attr) Pos() Pos               { return a.P }
func (a *Attr) String() string         { return fmt.Sprintf("%s.%s", a.Value, a.Name) }
func (a *Attr) Accept(v Visitor) error { return v.VisitAttr(a) }

type Call struct {
	Value Expr   // The function expression (e.g., `foo` or `foo.bar`)
	Args  []Expr // List of arguments
	P     Pos
}

func (c *Call) Pos() Pos { return c.P }
func (c *Call) String() string {
	argsStr := make([]string, len(c.Args))
	for i, arg := range c.Args {
		argsStr[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", c.Value, strings.Join(argsStr, ", "))
}
func (c *Call) Accept(v Visitor) error {
	return v.VisitCall(c)
}

type ArrayIdx struct {
	Value Expr // The array expression (e.g., `foo`)
	Idx   Expr // The index expression (e.g., `bar` in `foo[bar]`)
	P     Pos
}

func (ai *ArrayIdx) Pos() Pos               { return ai.P }
func (ai *ArrayIdx) String() string         { return fmt.Sprintf("%s[%s]", ai.Value, ai.Idx) }
func (ai *ArrayIdx) Accept(v Visitor) error { return v.VisitArrayIdx(ai) }

// --- Type conversions / Built-ins ---

type CastToType struct {
	Value    Expr
	TypeName string // The type to cast to
	P        Pos
}

func (ct *CastToType) Pos() Pos               { return ct.P }
func (ct *CastToType) String() string         { return fmt.Sprintf("%s.as<%s>()", ct.Value, ct.TypeName) }
func (ct *CastToType) Accept(v Visitor) error { return v.VisitCastToType(ct) }

type SizeOf struct {
	Value Expr // The expression to get the size of
	P     Pos
}

func (s *SizeOf) Pos() Pos               { return s.P }
func (s *SizeOf) String() string         { return fmt.Sprintf("sizeof(%s)", s.Value) }
func (s *SizeOf) Accept(v Visitor) error { return v.VisitSizeOf(s) }

type AlignOf struct {
	Value Expr // The expression to get the alignment of
	P     Pos
}

func (a *AlignOf) Pos() Pos               { return a.P }
func (a *AlignOf) String() string         { return fmt.Sprintf("alignof(%s)", a.Value) }
func (a *AlignOf) Accept(v Visitor) error { return v.VisitAlignOf(a) }

// --- End of AST Definitions ---
