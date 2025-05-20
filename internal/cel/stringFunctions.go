package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// stringFunctions returns CEL function declarations for string operations.
func stringFunctions() cel.EnvOption {
	return cel.Lib(&stringLib{})
}

type stringLib struct{}

func (*stringLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// to_s function
		cel.Function("to_s",
			cel.Overload("to_s_any", []*cel.Type{cel.AnyType}, cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.String(fmt.Sprintf("%v", val.Value()))
				}),
			),
		),

		// reverse function - now uses Kaitai's StringReverse
		cel.Function("reverse",
			cel.Overload("reverse_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					str, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string type for reverse")
					}
					result := kaitai.StringReverse(string(str))
					return types.String(result)
				}),
			),
		),
		// length function
		cel.Function("length",
			cel.Overload("length_string", []*cel.Type{cel.StringType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					str, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string type for length")
					}
					return types.Int(len([]rune(string(str))))
				}),
			),
			cel.Overload("length_bytes", []*cel.Type{cel.BytesType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					b, ok := val.(types.Bytes)
					if !ok {
						return types.NewErr("expected bytes type for length")
					}
					return types.Int(len(b))
				}),
			),
			cel.Overload("length_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					lister, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list type for length")
					}
					size := lister.Size()
					if intSize, ok := size.(types.Int); ok {
						return intSize
					}
					return types.Int(size.Value().(int64))
				}),
			),
			cel.Overload("length_map", []*cel.Type{cel.MapType(cel.AnyType, cel.AnyType)}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					mapper, ok := val.(traits.Mapper)
					if !ok {
						return types.NewErr("expected map type for length")
					}
					size := mapper.Size()
					if intSize, ok := size.(types.Int); ok {
						return intSize
					}
					return types.Int(size.Value().(int64))
				}),
			),
		),
	}
}

func (*stringLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
