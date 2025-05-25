package kaitaicel

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// --- Enum Types ---

// KaitaiEnum represents an enumerated type with validation
type KaitaiEnum struct {
	value    int64
	name     string           // Human-readable name if mapped
	enumName string           // Name of the enum type
	enumMap  map[int64]string // Value to name mapping
	nameMap  map[string]int64 // Name to value mapping
}

var KaitaiEnumType = cel.ObjectType("kaitai.Enum", traits.ComparerType)

// NewKaitaiEnum creates a new enum with validation
func NewKaitaiEnum(value int64, enumName string, mapping map[int64]string) (*KaitaiEnum, error) {
	enum := &KaitaiEnum{
		value:    value,
		enumName: enumName,
		enumMap:  mapping,
		nameMap:  make(map[string]int64),
	}

	// Build reverse mapping
	for val, name := range mapping {
		enum.nameMap[name] = val
	}

	// Validate and set name
	if name, ok := mapping[value]; ok {
		enum.name = name
	} else {
		// Value not in enum - could be error or just unnamed value
		enum.name = fmt.Sprintf("<%s::%d>", enumName, value)
	}

	return enum, nil
}

// NewKaitaiEnumByName creates enum from string name
func NewKaitaiEnumByName(name string, enumName string, mapping map[int64]string) (*KaitaiEnum, error) {
	// Build name to value map
	nameMap := make(map[string]int64)
	for val, n := range mapping {
		nameMap[n] = val
	}

	value, ok := nameMap[name]
	if !ok {
		return nil, fmt.Errorf("invalid enum value '%s' for %s", name, enumName)
	}

	return &KaitaiEnum{
		value:    value,
		name:     name,
		enumName: enumName,
		enumMap:  mapping,
		nameMap:  nameMap,
	}, nil
}

// Getters
func (e *KaitaiEnum) IntValue() int64 {
	return e.value
}

func (e *KaitaiEnum) Name() string {
	return e.name
}

func (e *KaitaiEnum) IsValid() bool {
	_, ok := e.enumMap[e.value]
	return ok
}

// CEL Val interface
func (e *KaitaiEnum) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	switch typeDesc.Kind() {
	case reflect.Int, reflect.Int64:
		return e.value, nil
	case reflect.String:
		return e.name, nil
	}

	if typeDesc == reflect.TypeOf(map[string]interface{}{}) {
		return map[string]interface{}{
			"value": e.value,
			"name":  e.name,
			"enum":  e.enumName,
		}, nil
	}

	return nil, fmt.Errorf("unsupported conversion from enum to %v", typeDesc)
}

func (e *KaitaiEnum) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.IntType:
		return types.Int(e.value)
	case types.StringType:
		return types.String(e.name)
	}
	return types.NewErr("type conversion error from enum to %v", typeVal)
}

func (e *KaitaiEnum) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiEnum:
		return types.Bool(e.value == o.value && e.enumName == o.enumName)
	case types.Int:
		return types.Bool(e.value == int64(o))
	case types.String:
		return types.Bool(e.name == string(o))
	}
	return types.False
}

func (e *KaitaiEnum) Type() ref.Type {
	return KaitaiEnumType
}

func (e *KaitaiEnum) Value() interface{} {
	return e
}

func (e *KaitaiEnum) KaitaiTypeName() string {
	return "enum:" + e.enumName
}

func (e *KaitaiEnum) RawBytes() []byte {
	// Enums don't have raw bytes - they're derived values
	return nil
}

// Serialize returns the binary representation of this enum
func (e *KaitaiEnum) Serialize() []byte {
	// Enums don't serialize directly - they use their underlying value's serialization
	return nil
}

func (e *KaitaiEnum) Compare(other ref.Val) ref.Val {
	var otherVal int64
	switch o := other.(type) {
	case *KaitaiEnum:
		if e.enumName != o.enumName {
			return types.NewErr("cannot compare different enum types")
		}
		otherVal = o.value
	case types.Int:
		otherVal = int64(o)
	default:
		return types.NewErr("cannot compare enum with %v", other.Type())
	}

	if e.value < otherVal {
		return types.IntNegOne
	} else if e.value > otherVal {
		return types.IntOne
	}
	return types.IntZero
}

// EnumRegistry holds all enum definitions
type EnumRegistry struct {
	enums map[string]map[int64]string // enumName -> value -> name
}

func NewEnumRegistry() *EnumRegistry {
	return &EnumRegistry{
		enums: make(map[string]map[int64]string),
	}
}

func (r *EnumRegistry) Register(enumName string, mapping map[int64]string) {
	r.enums[enumName] = mapping
}

func (r *EnumRegistry) Get(enumName string) (map[int64]string, bool) {
	mapping, ok := r.enums[enumName]
	return mapping, ok
}

// EnumTypeOptions provides CEL options for enum types
func EnumTypeOptions(registry *EnumRegistry) []cel.EnvOption {
	return []cel.EnvOption{
		cel.Types(&KaitaiEnum{}),

		// Generic enum constructor
		cel.Function("enum",
			cel.Overload("enum_int_string",
				[]*cel.Type{cel.IntType, cel.StringType},
				KaitaiEnumType,
				cel.BinaryBinding(func(value, enumName ref.Val) ref.Val {
					val := int64(value.(types.Int))
					name := string(enumName.(types.String))

					mapping, ok := registry.Get(name)
					if !ok {
						return types.NewErr("unknown enum type: %s", name)
					}

					enum, err := NewKaitaiEnum(val, name, mapping)
					if err != nil {
						return types.NewErr("enum creation error: %v", err)
					}
					return enum
				}),
			),
		),

		// Enum by name constructor
		cel.Function("enum_name",
			cel.Overload("enum_name_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				KaitaiEnumType,
				cel.BinaryBinding(func(name, enumName ref.Val) ref.Val {
					n := string(name.(types.String))
					en := string(enumName.(types.String))

					mapping, ok := registry.Get(en)
					if !ok {
						return types.NewErr("unknown enum type: %s", en)
					}

					enum, err := NewKaitaiEnumByName(n, en, mapping)
					if err != nil {
						return types.NewErr("enum creation error: %v", err)
					}
					return enum
				}),
			),
		),

		// Enum methods
		cel.Function("int",
			cel.MemberOverload("enum_to_int",
				[]*cel.Type{KaitaiEnumType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiEnum).value)
				}),
			),
		),

		cel.Function("name",
			cel.MemberOverload("enum_name",
				[]*cel.Type{KaitaiEnumType},
				cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.String(val.(*KaitaiEnum).name)
				}),
			),
		),

		cel.Function("is_valid",
			cel.MemberOverload("enum_is_valid",
				[]*cel.Type{KaitaiEnumType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bool(val.(*KaitaiEnum).IsValid())
				}),
			),
		),

		// Type checking
		cel.Function("is_enum",
			cel.Overload("is_enum_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					_, ok := val.(*KaitaiEnum)
					return types.Bool(ok)
				}),
			),
		),

		// Comparison with names
		cel.Function("_==_",
			cel.Overload("eq_enum_string",
				[]*cel.Type{KaitaiEnumType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*KaitaiEnum).Equal(rhs)
				}),
			),
		),
	}
}
