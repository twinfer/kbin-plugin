package cel

import (
	"math"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// mathFunctions returns CEL function declarations for math operations.
func mathFunctions() cel.EnvOption {
	return cel.Lib(&mathLib{})
}

type mathLib struct{}

func (*mathLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// abs function
		cel.Function("abs",
			cel.Overload("abs_int", []*cel.Type{cel.IntType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					x, ok := val.(types.Int)
					if !ok {
						return types.NewErr("expected int argument to abs, got %T", val)
					}
					if x < 0 {
						return types.Int(-x)
					}
					return x
				}),
			),
			cel.Overload("abs_double", []*cel.Type{cel.DoubleType}, cel.DoubleType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					x, ok := val.(types.Double)
					if !ok {
						return types.NewErr("expected double argument to abs, got %T", val)
					}
					if x < 0 {
						return types.Double(-x)
					}
					return x
				}),
			),
		),

		// min function
		cel.Function("min",
			cel.Overload("min_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Int)
					y, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to min must be integers")
					}
					if x < y {
						return x
					}
					return y
				}),
			),
			cel.Overload("min_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Double)
					y, ok2 := rhs.(types.Double)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to min must be doubles")
					}
					if x < y {
						return x
					}
					return y
				}),
			),
		),

		// max function
		cel.Function("max",
			cel.Overload("max_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Int)
					y, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to max must be integers")
					}
					if x > y {
						return x
					}
					return y
				}),
			),
			cel.Overload("max_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					x, ok1 := lhs.(types.Double)
					y, ok2 := rhs.(types.Double)
					if !ok1 || !ok2 {
						return types.NewErr("arguments to max must be doubles")
					}
					if x > y {
						return x
					}
					return y
				}),
			),
		),

		// ceil function
		cel.Function("ceil",
			cel.Overload("ceil_double", []*cel.Type{cel.DoubleType}, cel.DoubleType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					x, ok := val.(types.Double)
					if !ok {
						return types.NewErr("expected double argument to ceil, got %T", val)
					}
					return types.Double(math.Ceil(float64(x)))
				}),
			),
		),

		// floor function
		cel.Function("floor",
			cel.Overload("floor_double", []*cel.Type{cel.DoubleType}, cel.DoubleType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					x, ok := val.(types.Double)
					if !ok {
						return types.NewErr("expected double argument to floor, got %T", val)
					}
					return types.Double(math.Floor(float64(x)))
				}),
			),
		),

		// round function
		cel.Function("round",
			cel.Overload("round_double", []*cel.Type{cel.DoubleType}, cel.DoubleType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					x, ok := val.(types.Double)
					if !ok {
						return types.NewErr("expected double argument to round, got %T", val)
					}
					return types.Double(math.Round(float64(x)))
				}),
			),
		),
	}
}

func (*mathLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
