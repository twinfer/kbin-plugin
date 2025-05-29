package kaitaicel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// KaitaiCELProvider provides all Kaitai types and functions to CEL
type KaitaiCELProvider struct{}

// CreateKaitaiCELEnv creates a CEL environment with Kaitai types
func CreateKaitaiCELEnv() (*cel.Env, error) {
	return CreateKaitaiCELEnvWithRegistry(nil)
}

// CreateKaitaiCELEnvWithRegistry creates a CEL environment with enum registry
func CreateKaitaiCELEnvWithRegistry(enumRegistry *EnumRegistry) (*cel.Env, error) {
	provider := &KaitaiCELProvider{}

	// Default enum registry if not provided
	if enumRegistry == nil {
		enumRegistry = NewEnumRegistry()
	}

	opts := []cel.EnvOption{
		cel.StdLib(),
	}

	// Add all Kaitai type options
	opts = append(opts, provider.IntegerTypeOptions()...)
	opts = append(opts, provider.StringTypeOptions()...)
	opts = append(opts, provider.BytesTypeOptions()...)
	opts = append(opts, BcdTypeOptions()...)
	opts = append(opts, FloatTypeOptions()...)
	opts = append(opts, EnumTypeOptions(enumRegistry)...)
	opts = append(opts, BitFieldTypeOptions()...)
	opts = append(opts, provider.HelperFunctionOptions()...)

	return cel.NewEnv(opts...)
}

// IntegerTypeOptions provides CEL options for integer types
func (p *KaitaiCELProvider) IntegerTypeOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Constructor functions
		cel.Function("u1",
			cel.Overload("u1_int",
				[]*cel.Type{cel.IntType},
				KaitaiU1Type,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					i := int64(val.(types.Int))
					if i < 0 || i > 255 {
						return types.NewErr("u1 value out of range: %d", i)
					}
					return NewKaitaiU1(uint8(i), nil)
				}),
			),
		),

		cel.Function("u2",
			cel.Overload("u2_int",
				[]*cel.Type{cel.IntType},
				KaitaiU2Type,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					i := int64(val.(types.Int))
					if i < 0 || i > 65535 {
						return types.NewErr("u2 value out of range: %d", i)
					}
					return NewKaitaiU2(uint16(i), nil)
				}),
			),
		),

		cel.Function("u4",
			cel.Overload("u4_int",
				[]*cel.Type{cel.IntType},
				KaitaiU4Type,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					i := int64(val.(types.Int))
					if i < 0 || i > 4294967295 {
						return types.NewErr("u4 value out of range: %d", i)
					}
					return NewKaitaiU4(uint32(i), nil)
				}),
			),
		),

		// Arithmetic operations
		cel.Function("_+_",
			cel.Overload("add_kaitai_int_int",
				[]*cel.Type{KaitaiU1Type, cel.IntType},
				cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*KaitaiInt).Add(rhs)
				}),
			),
			cel.Overload("add_kaitai_int_kaitai_int",
				[]*cel.Type{KaitaiU1Type, KaitaiU1Type},
				cel.IntType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*KaitaiInt).Add(rhs)
				}),
			),
		),

		// Comparison operations
		cel.Function("_<_",
			cel.Overload("less_kaitai_int_int",
				[]*cel.Type{KaitaiU1Type, cel.IntType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					cmp := lhs.(*KaitaiInt).Compare(rhs)
					return types.Bool(cmp == types.IntNegOne)
				}),
			),
		),

		// Conversion methods
		cel.Function("int",
			cel.MemberOverload("kaitai_int_to_int",
				[]*cel.Type{KaitaiU1Type},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiInt).value)
				}),
			),
		),

		cel.Function("bytes",
			cel.MemberOverload("kaitai_int_to_bytes",
				[]*cel.Type{KaitaiU1Type},
				cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bytes(val.(*KaitaiInt).raw)
				}),
			),
		),
	}
}

// StringTypeOptions provides CEL options for string types
func (p *KaitaiCELProvider) StringTypeOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Constructor function
		cel.Function("kaitai_str",
			cel.Overload("kaitai_str_bytes_string",
				[]*cel.Type{cel.BytesType, cel.StringType},
				KaitaiStringType,
				cel.BinaryBinding(func(data, encoding ref.Val) ref.Val {
					bytes := []byte(data.(types.Bytes))
					enc := string(encoding.(types.String))
					str, err := NewKaitaiString(bytes, enc)
					if err != nil {
						return types.NewErr("failed to create string: %v", err)
					}
					return str
				}),
			),
		),

		// String methods
		cel.Function("length",
			cel.MemberOverload("kaitai_string_length",
				[]*cel.Type{KaitaiStringType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiString).Length())
				}),
			),
		),

		cel.Function("byte_size",
			cel.MemberOverload("kaitai_string_byte_size",
				[]*cel.Type{KaitaiStringType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiString).ByteSize())
				}),
			),
		),

		cel.Function("str",
			cel.MemberOverload("kaitai_string_to_string",
				[]*cel.Type{KaitaiStringType},
				cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.String(val.(*KaitaiString).value)
				}),
			),
		),

		cel.Function("bytes",
			cel.MemberOverload("kaitai_string_to_bytes",
				[]*cel.Type{KaitaiStringType},
				cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bytes(val.(*KaitaiString).raw)
				}),
			),
		),

		// Comparison
		// Note: Commented out to avoid collision with standard CEL string equality
		// cel.Function("_==_",
		// 	cel.Overload("eq_kaitai_string_string",
		// 		[]*cel.Type{KaitaiStringType, cel.StringType},
		// 		cel.BoolType,
		// 		cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
		// 			return lhs.(*KaitaiString).Equal(rhs)
		// 		}),
		// 	),
		// ),
	}
}

// BytesTypeOptions provides CEL options for bytes types
func (p *KaitaiCELProvider) BytesTypeOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Constructor
		cel.Function("kaitai_bytes",
			cel.Overload("kaitai_bytes_bytes",
				[]*cel.Type{cel.BytesType},
				KaitaiBytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return NewKaitaiBytes([]byte(val.(types.Bytes)))
				}),
			),
		),

		// Methods
		cel.Function("length",
			cel.MemberOverload("kaitai_bytes_length",
				[]*cel.Type{KaitaiBytesType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiBytes).Length())
				}),
			),
		),

		cel.Function("at",
			cel.MemberOverload("kaitai_bytes_at",
				[]*cel.Type{KaitaiBytesType, cel.IntType},
				cel.IntType,
				cel.BinaryBinding(func(bytes, index ref.Val) ref.Val {
					b := bytes.(*KaitaiBytes)
					idx := int(index.(types.Int))
					val, err := b.At(idx)
					if err != nil {
						return types.NewErr("bytes access error: %v", err)
					}
					return types.Int(val)
				}),
			),
		),

		cel.Function("bytes",
			cel.MemberOverload("kaitai_bytes_to_bytes",
				[]*cel.Type{KaitaiBytesType},
				cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bytes(val.(*KaitaiBytes).value)
				}),
			),
		),

		// Note: Comparison operators are not explicitly defined here because
		// KaitaiBytes implements the Equal and Compare methods which CEL will use
		// automatically through the traits.ComparerType in the type definition
	}
}

// HelperFunctionOptions provides utility functions
func (p *KaitaiCELProvider) HelperFunctionOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Type checking functions
		cel.Function("is_u1",
			cel.Overload("is_u1_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if ki, ok := val.(*KaitaiInt); ok {
						return types.Bool(ki.typeName == "u1")
					}
					return types.False
				}),
			),
		),

		cel.Function("is_u2",
			cel.Overload("is_u2_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if ki, ok := val.(*KaitaiInt); ok {
						return types.Bool(ki.typeName == "u2")
					}
					return types.False
				}),
			),
		),

		cel.Function("is_kaitai_str",
			cel.Overload("is_kaitai_str_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					_, ok := val.(*KaitaiString)
					return types.Bool(ok)
				}),
			),
		),

		cel.Function("is_kaitai_bytes",
			cel.Overload("is_kaitai_bytes_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					_, ok := val.(*KaitaiBytes)
					return types.Bool(ok)
				}),
			),
		),

		// Raw byte access
		cel.Function("raw_bytes",
			cel.Overload("raw_bytes_any",
				[]*cel.Type{cel.AnyType},
				cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if kt, ok := val.(KaitaiType); ok {
						return types.Bytes(kt.RawBytes())
					}
					return types.NewErr("value does not have raw bytes")
				}),
			),
		),
	}
}

// CreateActivation creates a CEL activation with Kaitai types
func CreateActivation(vars map[string]interface{}) (cel.Activation, error) {
	// Convert any special values to Kaitai types if needed
	processedVars := make(map[string]interface{})

	for k, v := range vars {
		switch val := v.(type) {
		case uint8:
			processedVars[k] = NewKaitaiU1(val, []byte{val})
		case uint16:
			processedVars[k] = NewKaitaiU2(val, nil)
		case uint32:
			processedVars[k] = NewKaitaiU4(val, nil)
		case uint64:
			processedVars[k] = NewKaitaiU8(val, nil)
		case int8:
			processedVars[k] = NewKaitaiS1(val, []byte{byte(val)})
		case int16:
			processedVars[k] = NewKaitaiS2(val, nil)
		case int32:
			processedVars[k] = NewKaitaiS4(val, nil)
		case int64:
			processedVars[k] = NewKaitaiS8(val, nil)
		default:
			processedVars[k] = v
		}
	}

	return cel.NewActivation(processedVars)
}

// Example usage for expressions
func EvaluateKaitaiExpression(expr string, vars map[string]interface{}) (interface{}, error) {
	env, err := CreateKaitaiCELEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL env: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compilation error: %w", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program creation error: %w", err)
	}

	activation, err := CreateActivation(vars)
	if err != nil {
		return nil, fmt.Errorf("activation error: %w", err)
	}

	out, _, err := prg.Eval(activation)
	if err != nil {
		return nil, fmt.Errorf("evaluation error: %w", err)
	}

	return out.Value(), nil
}
