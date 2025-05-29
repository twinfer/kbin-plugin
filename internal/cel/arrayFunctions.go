package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// ArrayFunctions returns CEL function declarations for array operations.
func ArrayFunctions() cel.EnvOption {
	return cel.Lib(&arrayLib{})
}

type arrayLib struct{}

func (*arrayLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{

		// array_first function for getting first element
		cel.Function("array_first",
			cel.Overload("array_first_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for array_first function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get first element of empty list")
					}
					return list.Get(types.Int(0))
				}),
			),
		),

		// array_last function for getting last element
		cel.Function("array_last",
			cel.Overload("array_last_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for array_last function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get last element of empty list")
					}
					return list.Get(size - 1)
				}),
			),
		),

		// array_min function for getting minimum element
		cel.Function("array_min",
			cel.Overload("array_min_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for array_min function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get min of empty list")
					}

					min := list.Get(types.Int(0))
					for i := types.Int(1); i < size; i++ {
						elem := list.Get(i)
						// Use CEL's Less trait for comparison
						if lt, ok := elem.(traits.Comparer); ok {
							if lt.Compare(min) == types.IntNegOne {
								min = elem
							}
						}
					}
					return min
				}),
			),
		),

		// array_max function for getting maximum element
		cel.Function("array_max",
			cel.Overload("array_max_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for array_max function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get max of empty list")
					}

					max := list.Get(types.Int(0))
					for i := types.Int(1); i < size; i++ {
						elem := list.Get(i)
						// Use CEL's Less trait for comparison
						if gt, ok := elem.(traits.Comparer); ok {
							if gt.Compare(max) == types.IntOne {
								max = elem
							}
						}
					}
					return max
				}),
			),
		),
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

		// first function - get first element of array
		cel.Function("first",
			cel.Overload("first_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for first function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get first element of empty list")
					}
					return list.Get(types.Int(0))
				}),
			),
		),

		// last function - get last element of array
		cel.Function("last",
			cel.Overload("last_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for last function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get last element of empty list")
					}
					return list.Get(size - 1)
				}),
			),
		),

		// list_min function - get minimum element of array
		cel.Function("list_min",
			cel.Overload("list_min_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for list_min function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get min of empty list")
					}

					min := list.Get(types.Int(0))
					for i := types.Int(1); i < size; i++ {
						elem := list.Get(i)
						// Use CEL's Less trait for comparison
						if lt, ok := elem.(traits.Comparer); ok {
							if lt.Compare(min) == types.IntNegOne {
								min = elem
							}
						}
					}
					return min
				}),
			),
		),

		// list_max function - get maximum element of array
		cel.Function("list_max",
			cel.Overload("list_max_list", []*cel.Type{cel.ListType(cel.AnyType)}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					list, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list for list_max function")
					}
					size := list.Size().(types.Int)
					if size == 0 {
						return types.NewErr("cannot get max of empty list")
					}

					max := list.Get(types.Int(0))
					for i := types.Int(1); i < size; i++ {
						elem := list.Get(i)
						// Use CEL's Less trait for comparison
						if gt, ok := elem.(traits.Comparer); ok {
							if gt.Compare(max) == types.IntOne {
								max = elem
							}
						}
					}
					return max
				}),
			),
		),
	}
}

func (*arrayLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
