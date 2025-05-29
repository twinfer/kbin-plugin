package cel

import (
	"strconv"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// typeConversionFunctions returns CEL function declarations for type conversions.
func TypeConversionFunctions() cel.EnvOption {
	return cel.Lib(&typeConversionLib{})
}

type typeConversionLib struct{}

func (*typeConversionLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// to_i function (integer conversion)
		cel.Function("to_i",
			cel.Overload("to_i_string", []*cel.Type{cel.StringType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if strVal, ok := val.(types.String); ok {
						intStr, err := strconv.ParseInt(string(strVal), 10, 64)
						if err != nil {
							return types.NewErr("cannot convert string to int: %v", err)
						}
						intVal := types.Int(intStr)
						return intVal
					}
					return types.NewErr("unexpected type for to_i: %T", val.Value())
				}),
			),
			cel.Overload("to_i_uint", []*cel.Type{cel.UintType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if uintVal, ok := val.(types.Uint); ok {
						return types.Int(uintVal)
					}
					return types.NewErr("unexpected type for to_i: %T", val.Value())
				}),
			),
			cel.Overload("to_i_double", []*cel.Type{cel.DoubleType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if doubleVal, ok := val.(types.Double); ok {
						return types.Int(doubleVal)
					}
					return types.NewErr("unexpected type for to_i: %T", val.Value())
				}),
			),
		),

		// to_f function (float conversion)
		cel.Function("to_f",
			cel.Overload("to_f_any", []*cel.Type{cel.AnyType}, cel.DoubleType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					convertedVal := val.ConvertToType(cel.DoubleType)
					if types.IsError(convertedVal) {
						return types.NewErr("cannot convert %v to double: %v", val, convertedVal)
					}
					return convertedVal
				}),
			),
		),
	}
}

func (*typeConversionLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
