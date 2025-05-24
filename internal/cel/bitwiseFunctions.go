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
	case types.Double: // Handle double by converting to uint64
		l = uint64(lv) // Note: This truncates the fractional part
		lOk = true
	}

	switch rv := rhs.(type) {
	case types.Int:
		r = uint64(rv)
		rOk = true
	case types.Uint:
		r = uint64(rv)
		rOk = true
	case types.Double: // Handle double by converting to uint64
		r = uint64(rv) // Note: This truncates the fractional part
		rOk = true
	}

	if !lOk || !rOk {
		return types.NewErr("bitwise arguments must be numeric (int, uint, double), got %T and %T", lhs.Value(), rhs.Value())
	}

	result := op(l, r)
	// If the result can fit into an int64, prefer returning Int for better compatibility
	// with standard arithmetic operators that might expect Int.
	// This is a heuristic; for true bit patterns, Uint might be more "correct".
	if result <= uint64(^uint64(0)>>1) { // MaxInt is int64 max
		return types.Int(result)
	}
	return types.Uint(result)
}

type bitwiseLib struct{}

func (*bitwiseLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// bitAnd
		cel.Function("bitAnd", // Renamed from bitAnd for clarity if old ones are kept temporarily
			cel.Overload("bitand_numeric", []*cel.Type{cel.DynType, cel.DynType}, cel.DynType, // Result type could be IntType or UintType based on performBitwiseOp
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a & b })
				}),
			),
		),

		// bitOr
		cel.Function("bitOr", // Renamed
			cel.Overload("bitor_numeric", []*cel.Type{cel.DynType, cel.DynType}, cel.DynType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return performBitwiseOp(lhs, rhs, func(a, b uint64) uint64 { return a | b })
				}),
			),
		),

		// bitXor
		cel.Function("bitXor", // Renamed
			cel.Overload("bitxor_numeric", []*cel.Type{cel.DynType, cel.DynType}, cel.DynType,
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
