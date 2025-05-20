package cel

// import (
// 	"github.com/google/cel-go/cel"
// 	"github.com/google/cel-go/common/types"
// 	"github.com/google/cel-go/common/types/ref"
// 	"github.com/google/cel-go/common/types/traits"
// )

// // sizeFunctions returns CEL function declarations for size operations.
// func sizeFunctions() cel.EnvOption {
// 	return cel.Lib(&sizeLib{})
// }

// type sizeLib struct{}

// func (*sizeLib) CompileOptions() []cel.EnvOption {
// 	return []cel.EnvOption{
// 		// size function
// 		cel.Function("size",
// 			cel.Overload("size_string", []*cel.Type{cel.StringType}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					str, ok := val.(types.String)
// 					if !ok {
// 						return types.NewErr("expected string type for size")
// 					}
// 					return types.Int(len([]rune(string(str))))
// 				}),
// 			),
// 			cel.Overload("size_bytes", []*cel.Type{cel.BytesType}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					b, ok := val.(types.Bytes)
// 					if !ok {
// 						return types.NewErr("expected bytes type for size")
// 					}
// 					return types.Int(len(b))
// 				}),
// 			),
// 			cel.Overload("size_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					if val.Type().HasTrait(traits.Lister) {
// 						list := val.(traits.Lister)
// 						return types.Int(list.Size().Value().(int64))
// 					}
// 					return types.NewErr("expected list type for size")
// 				}),
// 			),
// 			cel.Overload("size_map", []*cel.Type{cel.MapType(cel.AnyType, cel.AnyType)}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					if val.Type().HasTrait(traits.Mapper) {
// 						m := val.(traits.Mapper)
// 						return types.Int(m.Size().Value().(int64))
// 					}
// 					return types.NewErr("expected map type for size")
// 				}),
// 			),
// 		),

// 		// count function (alias to size)
// 		cel.Function("count",
// 			cel.Overload("count_string", []*cel.Type{cel.StringType}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					str, ok := val.(types.String)
// 					if !ok {
// 						return types.NewErr("expected string type for count")
// 					}
// 					return types.Int(len([]rune(string(str))))
// 				}),
// 			),
// 			cel.Overload("count_bytes", []*cel.Type{cel.BytesType}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					b, ok := val.(types.Bytes)
// 					if !ok {
// 						return types.NewErr("expected bytes type for count")
// 					}
// 					return types.Int(len(b))
// 				}),
// 			),
// 			cel.Overload("count_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					if val.Type().HasTrait(traits.Lister) {
// 						list := val.(traits.Lister)
// 						return types.Int(list.Size().Value().(int64))
// 					}
// 					return types.NewErr("expected list type for count")
// 				}),
// 			),
// 			cel.Overload("count_map", []*cel.Type{cel.MapType(cel.AnyType, cel.AnyType)}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					if val.Type().HasTrait(traits.Mapper) {
// 						m := val.(traits.Mapper)
// 						return types.Int(m.Size().Value().(int64))
// 					}
// 					return types.NewErr("expected map type for count")
// 				}),
// 			),
// 		),
// 	}
// }

// func (*sizeLib) ProgramOptions() []cel.ProgramOption {
// 	return []cel.ProgramOption{}
// }

// func (*sizeLib) Types() map[string]*types.Type {
// 	return map[string]*types.Type{}
// }
