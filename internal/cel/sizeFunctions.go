package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
)

// SchemaProvider interface for calculating custom type sizes
type SchemaProvider interface {
	CalculateTypeSize(typeName string) int64
}

// Global schema provider for size calculations
var globalSchemaProvider SchemaProvider

// SetGlobalSchemaProvider sets the schema provider for size calculations
func SetGlobalSchemaProvider(provider SchemaProvider) {
	globalSchemaProvider = provider
}

// sizeFunctions returns CEL function declarations for size operations.
func SizeFunctions() cel.EnvOption {
	return cel.Lib(&sizeLib{})
}

type sizeLib struct{}

func (*sizeLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// sizeof_value function to get the size of a parsed value
		cel.Function("sizeof_value",
			cel.Overload("sizeof_value_any", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					// Handle KaitaiCEL types that track their size
					switch v := val.(type) {
					case *kaitaicel.KaitaiInt:
						// Calculate size from type name or raw bytes
						if rawBytes := v.RawBytes(); len(rawBytes) > 0 {
							return types.Int(len(rawBytes))
						}
						// Fallback: calculate from type name
						return types.Int(getIntTypeSizeFromName(v.KaitaiTypeName()))
					case *kaitaicel.KaitaiString:
						return types.Int(v.ByteSize())
					case *kaitaicel.KaitaiBytes:
						if rawBytes := v.RawBytes(); len(rawBytes) > 0 {
							return types.Int(len(rawBytes))
						}
						// Fallback to actual value size
						if size := v.Size(); size.Type() == types.IntType {
							return size
						}
						return types.Int(0)
					case *kaitaicel.KaitaiFloat:
						if rawBytes := v.RawBytes(); len(rawBytes) > 0 {
							return types.Int(len(rawBytes))
						}
						// Fallback: calculate from type name
						return types.Int(getFloatTypeSizeFromName(v.KaitaiTypeName()))
					case *kaitaicel.KaitaiEnum:
						// Enums are backed by integers, get size from underlying type
						if rawBytes := v.RawBytes(); len(rawBytes) > 0 {
							return types.Int(len(rawBytes))
						}
						return types.Int(4) // Default enum size
					default:
						// For maps (parsed types), look for _sizeof metadata
						if mapVal, ok := val.(traits.Mapper); ok {
							// Check if there's size metadata stored in the map
							if sizeVal := mapVal.Get(types.String("_sizeof")); sizeVal != nil && sizeVal.Type() != types.ErrType {
								if size, ok := sizeVal.(types.Int); ok {
									return size
								}
							}
						}
						// Default: return 0 for unknown types
						return types.Int(0)
					}
				}),
			),
		),
		// sizeof_type function to get the size of a type by name
		cel.Function("sizeof_type",
			cel.Overload("sizeof_type_string", []*cel.Type{cel.StringType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					typeName, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string for sizeof_type function")
					}

					typeNameStr := string(typeName)
					size := getTypeSizeFromName(typeNameStr)
					return types.Int(size)
				}),
			),
		),
	}
}

func (*sizeLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// getIntTypeSizeFromName returns the size in bytes for integer types
func getIntTypeSizeFromName(typeName string) int64 {
	switch typeName {
	case "u1", "s1":
		return 1
	case "u2", "s2", "u2le", "u2be", "s2le", "s2be":
		return 2
	case "u4", "s4", "u4le", "u4be", "s4le", "s4be":
		return 4
	case "u8", "s8", "u8le", "u8be", "s8le", "s8be":
		return 8
	default:
		return 0
	}
}

// getFloatTypeSizeFromName returns the size in bytes for float types
func getFloatTypeSizeFromName(typeName string) int64 {
	switch typeName {
	case "f4", "f4le", "f4be":
		return 4
	case "f8", "f8le", "f8be":
		return 8
	default:
		return 0
	}
}

// getTypeSizeFromName returns the size in bytes for any type name (built-in or custom)
func getTypeSizeFromName(typeName string) int64 {
	// First try built-in integer types
	if size := getIntTypeSizeFromName(typeName); size > 0 {
		return size
	}

	// Try built-in float types
	if size := getFloatTypeSizeFromName(typeName); size > 0 {
		return size
	}

	// For custom types, try to use the schema provider
	if globalSchemaProvider != nil {
		if size := globalSchemaProvider.CalculateTypeSize(typeName); size > 0 {
			return size
		}
	}

	// Unknown type
	return 0
}
