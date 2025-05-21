package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// bitwiseFunctions returns CEL function declarations for bitwise operations.
func bitwiseFunctions() cel.EnvOption {
	return cel.Lib(&bitwiseLib{})
}

// Helper function to perform bitwise operations, promoting to uint64
func performBitwiseOp(lhs, rhs ref.Val, op func(uint64, uint64) uint64) ref.Val {
	var l, r uint64
	var lOk, rOk bool

	switch lv := lhs.(type) {
	case types.Int:
		l = uint64(lv)
		lOk = true
	case types.Uint:
		l = uint64(lv)
		lOk = true
	}

	switch rv := rhs.(type) {
	case types.Int:
		r = uint64(rv)
		rOk = true
	case types.Uint:
		r = uint64(rv)
		rOk = true
	}

	if !lOk || !rOk {
		return types.NewErr("bitwise arguments must be integers or unsigned integers, got %T and %T", lhs.Value(), rhs.Value())
	}

	// If both original inputs were Int, and the operation is one that typically preserves "int-ness"
	// (like AND, OR, XOR if not resulting in a value > max int64), one might consider returning Int.
	// However, for general bitwise ops, returning Uint is often safer to avoid overflow/sign issues.
	// For simplicity and consistency with Kaitai's typical unsigned view of bit patterns, we'll return Uint.
	return types.Uint(op(l, r))
}

type bitwiseLib struct{}

func (*bitwiseLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// bitAnd
		cel.Function("bitAnd",
			cel.Overload("bitand_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Int)
					right, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitAnd must be integers")
					}
					return types.Int(left & right)
				}),
			),
			cel.Overload("bitand_uint_uint", []*cel.Type{cel.UintType, cel.UintType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a & b })
				}),
			),
			cel.Overload("bitand_int_uint", []*cel.Type{cel.IntType, cel.UintType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a & b })
				}),
			),
			cel.Overload("bitand_uint_int", []*cel.Type{cel.UintType, cel.IntType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a & b })
				}),
			),
		),

		// bitOr
		cel.Function("bitOr",
			cel.Overload("bitor_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Int)
					right, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitOr must be integers")
					}
					return types.Int(left | right)
				}),
			),
			cel.Overload("bitor_uint_uint", []*cel.Type{cel.UintType, cel.UintType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a | b })
				}),
			),
			cel.Overload("bitor_int_uint", []*cel.Type{cel.IntType, cel.UintType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a | b })
				}),
			),
			cel.Overload("bitor_uint_int", []*cel.Type{cel.UintType, cel.IntType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a | b })
				}),
			),
		),

		// bitXor
		cel.Function("bitXor",
			cel.Overload("bitxor_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Int)
					right, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitXor must be integers")
					}
					return types.Int(left ^ right)
				}),
			),
			cel.Overload("bitxor_uint_uint", []*cel.Type{cel.UintType, cel.UintType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a ^ b })
				}),
			),
			cel.Overload("bitxor_int_uint", []*cel.Type{cel.IntType, cel.UintType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a ^ b })
				}),
			),
			cel.Overload("bitxor_uint_int", []*cel.Type{cel.UintType, cel.IntType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a ^ b })
				}),
			),
		),

		// bitShiftLeft
		cel.Function("bitShiftLeft",
			cel.Overload("bitshiftleft_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Int)
					right, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitShiftLeft must be integers")
					}
					if right < 0 {
						return types.NewErr("shift amount cannot be negative: %v", right)
					}
					return types.Int(left << uint(right))
				}),
			),
			cel.Overload("bitshiftleft_uint_int", []*cel.Type{cel.UintType, cel.IntType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Uint)
					right, ok2 := rhs.(types.Int) // Shift amount is typically int
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitShiftLeft must be uint and int")
					}
					if right < 0 {
						return types.NewErr("shift amount cannot be negative: %v", right)
					}
					return types.Uint(left << uint(right))
				}),
			),
		),

		// bitShiftRight
		cel.Function("bitShiftRight",
			cel.Overload("bitshiftright_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Int)
					right, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitShiftRight must be integers")
					}
					if right < 0 {
						return types.NewErr("shift amount cannot be negative: %v", right)
					}
					return types.Int(left >> uint(right))
				}),
			),
			cel.Overload("bitshiftright_uint_int", []*cel.Type{cel.UintType, cel.IntType}, cel.UintType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left, ok1 := lhs.(types.Uint)
					right, ok2 := rhs.(types.Int) // Shift amount is typically int
					if !ok1 || !ok2 {
						return types.NewErr("arguments to bitShiftRight must be uint and int")
					}
					if right < 0 {
						return types.NewErr("shift amount cannot be negative: %v", right)
					}
					return types.Uint(left >> uint(right))
				}),
			),
		),
	}
}

func (*bitwiseLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
