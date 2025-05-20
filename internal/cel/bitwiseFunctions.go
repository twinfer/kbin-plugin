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
		),
	}
}

func (*bitwiseLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
