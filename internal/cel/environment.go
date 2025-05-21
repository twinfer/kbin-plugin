package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// NewEnvironment creates a CEL environment with all Kaitai-specific functions.
func NewEnvironment() (*cel.Env, error) {
	// Create the CEL environment with standard library functions and Kaitai-specific functions
	env, err := cel.NewEnv(
		// Register variable types and declare core functions
		cel.Variable("input", cel.BytesType),

		// Add custom type adapter to handle conversion between Go and CEL types
		cel.CustomTypeAdapter(types.DefaultTypeAdapter),

		// Enable standard CEL library functions
		cel.StdLib(),

		// Register Kaitai-specific functions organized by category
		stringFunctions(),
		typeConversionFunctions(),
		arrayFunctions(),
		// Removed sizeFunctions() to avoid conflict with stdlib
		bitwiseFunctions(),
		mathFunctions(),
		kaitaiApiFunctions(), // New function category using direct Kaitai API calls
		//processFunctions(),
		errorHandlingFunctions(),
		safeArithmeticFunctions(),
		streamOperations(),  // New function category for stream operations
		writerOperations(),  // New functions for serialization
		encodingFunctions(), // New functions for string encoding
		errorHandlingFunctions(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return env, nil
}

// sizeFunctions returns CEL function declarations for size operations (separate from stringFunctions).
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
// 					lister, ok := val.(traits.Lister)
// 					if !ok {
// 						return types.NewErr("expected list type for size")
// 					}
// 					size := lister.Size()
// 					if intSize, ok := size.(types.Int); ok {
// 						return intSize
// 					}
// 					return types.Int(size.Value().(int64))
// 				}),
// 			),
// 			cel.Overload("size_map", []*cel.Type{cel.MapType(cel.AnyType, cel.AnyType)}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					mapper, ok := val.(traits.Mapper)
// 					if !ok {
// 						return types.NewErr("expected map type for size")
// 					}
// 					size := mapper.Size()
// 					if intSize, ok := size.(types.Int); ok {
// 						return intSize
// 					}
// 					return types.Int(size.Value().(int64))
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
// 					lister, ok := val.(traits.Lister)
// 					if !ok {
// 						return types.NewErr("expected list type for count")
// 					}
// 					size := lister.Size()
// 					if intSize, ok := size.(types.Int); ok {
// 						return intSize
// 					}
// 					return types.Int(size.Value().(int64))
// 				}),
// 			),
// 			cel.Overload("count_map", []*cel.Type{cel.MapType(cel.AnyType, cel.AnyType)}, cel.IntType,
// 				cel.UnaryBinding(func(val ref.Val) ref.Val {
// 					mapper, ok := val.(traits.Mapper)
// 					if !ok {
// 						return types.NewErr("expected map type for count")
// 					}
// 					size := mapper.Size()
// 					if intSize, ok := size.(types.Int); ok {
// 						return intSize
// 					}
// 					return types.Int(size.Value().(int64))
// 				}),
// 			),
// 		),
// 	}
// }

// func (*sizeLib) ProgramOptions() []cel.ProgramOption {
// 	return []cel.ProgramOption{}
// }

func errorHandlingFunctions() cel.EnvOption {
	return cel.Lib(&errorLib{})
}

type errorLib struct{}

func (*errorLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("error",
			cel.Overload("error_string", []*cel.Type{cel.StringType}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					msg, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string for error message")
					}
					return types.NewErr("%s", msg)
				}),
			),
		),
		cel.Function("isError",
			cel.Overload("iserror_any", []*cel.Type{cel.AnyType}, cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bool(types.IsError(val))
				}),
			),
		),
	}
}

func (*errorLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// Functions for interacting with the CEL types and protobuf
func ValueToCEL(val any) ref.Val {
	return types.DefaultTypeAdapter.NativeToValue(val)
}

func CELToValue(val ref.Val) any {
	if val == nil {
		return nil
	}
	return val.Value()
}

// ValueToRefValue converts a Go value to a CEL ref.Val using the default type adapter
func ValueToRefValue(val any) ref.Val {
	return ValueToCEL(val)
}

// RefValueToValue converts a CEL ref.Val back to a Go value
func RefValueToValue(val ref.Val) (any, error) {
	if val == nil {
		return nil, nil
	}

	if types.IsError(val) {
		return nil, fmt.Errorf("CEL error: %v", val)
	}

	if types.IsUnknown(val) {
		return nil, fmt.Errorf("unknown CEL value")
	}

	return val.Value(), nil
}

// ValueAsProto converts a CEL value to a protobuf value
func ValueAsProto(val ref.Val) (*exprpb.Value, error) {
	if val == nil {
		return nil, nil
	}

	return cel.ValueAsAlphaProto(val)
}
