package cel

import (
	"fmt"
	"strconv"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// StringFunctions returns CEL function declarations for string operations.
func StringFunctions() cel.EnvOption {
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
			cel.Overload("to_s_any_encoding", []*cel.Type{cel.AnyType, cel.StringType}, cel.StringType,
				cel.BinaryBinding(func(val, encoding ref.Val) ref.Val {
					// For byte arrays, convert using specified encoding
					switch v := val.(type) {
					case types.String:
						return v // Already a string
					case types.Bytes:
						// Convert CEL bytes directly to string
						return types.String(string(v))
					case traits.Lister:
						// Convert list of integers to bytes then to string
						var bytes []byte
						for i := types.Int(0); i < v.Size().(types.Int); i++ {
							elem := v.Get(types.Int(i))
							if intVal, ok := elem.(types.Int); ok {
								bytes = append(bytes, byte(intVal))
							}
						}
						return types.String(string(bytes))
					default:
						return types.String(fmt.Sprintf("%v", val.Value()))
					}
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
		// substring function
		cel.Function("substring",
			cel.Overload("substring_string_int_int", []*cel.Type{cel.StringType, cel.IntType, cel.IntType}, cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 3 {
						return types.NewErr("substring requires exactly 3 arguments: string, start, end")
					}

					str, ok := args[0].(types.String)
					if !ok {
						return types.NewErr("first argument must be string")
					}

					start, ok := args[1].(types.Int)
					if !ok {
						return types.NewErr("start index must be integer")
					}

					end, ok := args[2].(types.Int)
					if !ok {
						return types.NewErr("end index must be integer")
					}

					runes := []rune(string(str))
					startIdx := int(start)
					endIdx := int(end)

					// Handle bounds checking
					if startIdx < 0 {
						startIdx = 0
					}
					if endIdx > len(runes) {
						endIdx = len(runes)
					}
					if startIdx >= endIdx || startIdx >= len(runes) {
						return types.String("")
					}

					return types.String(string(runes[startIdx:endIdx]))
				}),
			),
		),
		// to_i function for string to integer conversion
		cel.Function("to_i",
			// to_i with default base 10
			cel.Overload("to_i_string", []*cel.Type{cel.StringType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					str, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string for to_i")
					}
					return stringToInt(string(str), 10)
				}),
			),
			// to_i with specified base
			cel.Overload("to_i_string_int", []*cel.Type{cel.StringType, cel.IntType}, cel.IntType,
				cel.BinaryBinding(func(str, base ref.Val) ref.Val {
					strVal, ok := str.(types.String)
					if !ok {
						return types.NewErr("first argument must be string")
					}
					baseVal, ok := base.(types.Int)
					if !ok {
						return types.NewErr("base must be integer")
					}
					return stringToInt(string(strVal), int(baseVal))
				}),
			),
		),
	}
}

func (*stringLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// stringToInt converts a string to integer with the specified base
func stringToInt(str string, base int) ref.Val {
	// Handle different bases
	if base < 2 || base > 36 {
		return types.NewErr("base must be between 2 and 36")
	}

	result, err := strconv.ParseInt(str, base, 64)
	if err != nil {
		return types.NewErr("invalid integer format: %v", err)
	}

	return types.Int(result)
}
