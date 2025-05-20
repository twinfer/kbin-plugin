package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// arrayFunctions returns CEL function declarations for array operations.
func arrayFunctions() cel.EnvOption {
	return cel.Lib(&arrayLib{})
}

type arrayLib struct{}

func (*arrayLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// at function to get element at index
		cel.Function("at",
			cel.Overload("at_list_int", []*cel.Type{cel.ListType(cel.AnyType), cel.IntType}, cel.AnyType,
				cel.BinaryBinding(func(list, idx ref.Val) ref.Val {
					lister, ok := list.(traits.Lister)
					if !ok {
						return types.NewErr("expected list type for at function")
					}

					i, ok := idx.(types.Int)
					if !ok {
						return types.NewErr("expected int index for at function")
					}

					size := lister.Size()
					if sizeInt, ok := size.(types.Int); ok {
						if i < 0 || i >= sizeInt {
							return types.NewErr("index out of bounds: %v", i)
						}
					}
					return lister.Get(idx)
				}),
			),
			cel.Overload("at_string_int", []*cel.Type{cel.StringType, cel.IntType}, cel.StringType,
				cel.BinaryBinding(func(str, idx ref.Val) ref.Val {
					s, ok := str.(types.String)
					if !ok {
						return types.NewErr("expected string type for at function")
					}
					i, ok := idx.(types.Int)
					if !ok {
						return types.NewErr("expected int index for at function")
					}
					runes := []rune(string(s))
					if i < 0 || i >= types.Int(len(runes)) {
						return types.NewErr("index out of bounds: %v", i)
					}
					return types.String(string(runes[i]))
				}),
			),
		),
	}
}

func (*arrayLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
