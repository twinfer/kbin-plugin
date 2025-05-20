package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// safeArithmeticFunctions returns CEL function declarations for safe arithmetic operations.
func safeArithmeticFunctions() cel.EnvOption {
	return cel.Lib(&safeArithmeticLib{})
}

type safeArithmeticLib struct{}

func (*safeArithmeticLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("mul",
			cel.Overload("mul_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Double)
					y, ok2 := rhs.(types.Double)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to mul must be doubles")
					}
					return types.Double(x * y)
				}),
			),
			cel.Overload("mul_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Int)
					y, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to mul must be integers")
					}
					return types.Int(x * y)
				}),
			),
		),

		// add - safe addition
		cel.Function("add",
			cel.Overload("add_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Double)
					y, ok2 := rhs.(types.Double)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to add must be doubles")
					}
					return types.Double(x + y)
				}),
			),
			cel.Overload("add_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Int)
					y, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to add must be integers")
					}
					return types.Int(x + y)
				}),
			),
		),

		// ternary function for conditionals
		cel.Function("ternary",
			cel.Overload("ternary_any_any_any", []*cel.Type{cel.BoolType, cel.AnyType, cel.AnyType}, cel.AnyType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 3 {
						return types.NewErr("ternary requires 3 arguments")
					}

					condition, ok := args[0].(types.Bool)
					if !ok {
						return types.NewErr("first argument to ternary must be bool")
					}

					if condition {
						return args[1]
					}
					return args[2]
				}),
			),
		),
	}
}

func (*safeArithmeticLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
