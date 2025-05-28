package kaitaicel

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// --- Float Types ---

// KaitaiFloat represents Kaitai float types (f4, f8)
type KaitaiFloat struct {
	value    float64
	typeName string // "f4" or "f8"
	raw      []byte
}

var (
	KaitaiF4Type = types.NewTypeValue("kaitai.F4", traits.ComparerType, traits.AdderType)
	KaitaiF8Type = types.NewTypeValue("kaitai.F8", traits.ComparerType, traits.AdderType)
)

// NewKaitaiF4 creates a new 4-byte float
func NewKaitaiF4(value float32, raw []byte) *KaitaiFloat {
	return &KaitaiFloat{value: float64(value), typeName: "f4", raw: raw}
}

// NewKaitaiF8 creates a new 8-byte float
func NewKaitaiF8(value float64, raw []byte) *KaitaiFloat {
	return &KaitaiFloat{value: value, typeName: "f8", raw: raw}
}

// --- Factory functions for serialization (value-only constructors) ---

// NewF4LEFromValue creates a f4le type from a value for serialization
func NewF4LEFromValue(value float32) *KaitaiFloat {
	return &KaitaiFloat{value: float64(value), typeName: "f4le", raw: nil}
}

// NewF4BEFromValue creates a f4be type from a value for serialization
func NewF4BEFromValue(value float32) *KaitaiFloat {
	return &KaitaiFloat{value: float64(value), typeName: "f4be", raw: nil}
}

// NewF8LEFromValue creates a f8le type from a value for serialization
func NewF8LEFromValue(value float64) *KaitaiFloat {
	return &KaitaiFloat{value: value, typeName: "f8le", raw: nil}
}

// NewF8BEFromValue creates a f8be type from a value for serialization
func NewF8BEFromValue(value float64) *KaitaiFloat {
	return &KaitaiFloat{value: value, typeName: "f8be", raw: nil}
}

// KaitaiFloat interface implementation
func (k *KaitaiFloat) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	switch typeDesc.Kind() {
	case reflect.Float32:
		if k.typeName == "f4" {
			return float32(k.value), nil
		}
		return float32(k.value), nil
	case reflect.Float64:
		return k.value, nil
	case reflect.Int, reflect.Int64:
		return int64(k.value), nil
	}
	return nil, fmt.Errorf("unsupported conversion from %s to %v", k.typeName, typeDesc)
}

func (k *KaitaiFloat) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.DoubleType:
		return types.Double(k.value)
	case types.IntType:
		return types.Int(int64(k.value))
	case types.StringType:
		return types.String(fmt.Sprintf("%g", k.value))
	}
	return types.NewErr("type conversion error from %s to %v", k.typeName, typeVal)
}

func (k *KaitaiFloat) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiFloat:
		return types.Bool(k.value == o.value)
	case types.Double:
		return types.Bool(k.value == float64(o))
	case types.Int:
		return types.Bool(k.value == float64(o))
	}
	return types.False
}

func (k *KaitaiFloat) Type() ref.Type {
	if k.typeName == "f4" {
		return KaitaiF4Type
	}
	return KaitaiF8Type
}

func (k *KaitaiFloat) Value() interface{} {
	return k.value
}

func (k *KaitaiFloat) KaitaiTypeName() string {
	return k.typeName
}

func (k *KaitaiFloat) RawBytes() []byte {
	return k.raw
}

// Serialize returns the binary representation of this float according to its type and endianness
func (k *KaitaiFloat) Serialize() []byte {
	if k.raw != nil && len(k.raw) > 0 {
		return k.raw
	}

	// Create binary data based on type name
	switch k.typeName {
	case "f4":
		// Default to big-endian for generic types
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, math.Float32bits(float32(k.value)))
		return buf
	case "f4le":
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(k.value)))
		return buf
	case "f4be":
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, math.Float32bits(float32(k.value)))
		return buf
	case "f8":
		// Default to big-endian for generic types
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, math.Float64bits(k.value))
		return buf
	case "f8le":
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(k.value))
		return buf
	case "f8be":
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, math.Float64bits(k.value))
		return buf
	default:
		// Fallback for unknown types
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, math.Float64bits(k.value))
		return buf
	}
}

func (k *KaitaiFloat) Add(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiFloat:
		return types.Double(k.value + o.value)
	case types.Double:
		return types.Double(k.value + float64(o))
	case types.Int:
		return types.Double(k.value + float64(o))
	}
	return types.NewErr("cannot add %v to %s", other.Type(), k.typeName)
}

func (k *KaitaiFloat) Compare(other ref.Val) ref.Val {
	var otherVal float64
	switch o := other.(type) {
	case *KaitaiFloat:
		otherVal = o.value
	case types.Double:
		otherVal = float64(o)
	case types.Int:
		otherVal = float64(o)
	default:
		return types.NewErr("cannot compare %v with %s", other.Type(), k.typeName)
	}

	if k.value < otherVal {
		return types.IntNegOne
	} else if k.value > otherVal {
		return types.IntOne
	}
	return types.IntZero
}

// Float helper functions
func ReadF4LE(data []byte, offset int) (*KaitaiFloat, error) {
	if offset+4 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read f4le at offset %d", offset)
	}
	bits := binary.LittleEndian.Uint32(data[offset:])
	value := math.Float32frombits(bits)
	return NewKaitaiF4(value, data[offset:offset+4]), nil
}

func ReadF4BE(data []byte, offset int) (*KaitaiFloat, error) {
	if offset+4 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read f4be at offset %d", offset)
	}
	bits := binary.BigEndian.Uint32(data[offset:])
	value := math.Float32frombits(bits)
	return NewKaitaiF4(value, data[offset:offset+4]), nil
}

func ReadF8LE(data []byte, offset int) (*KaitaiFloat, error) {
	if offset+8 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read f8le at offset %d", offset)
	}
	bits := binary.LittleEndian.Uint64(data[offset:])
	value := math.Float64frombits(bits)
	return NewKaitaiF8(value, data[offset:offset+8]), nil
}

func ReadF8BE(data []byte, offset int) (*KaitaiFloat, error) {
	if offset+8 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read f8be at offset %d", offset)
	}
	bits := binary.BigEndian.Uint64(data[offset:])
	value := math.Float64frombits(bits)
	return NewKaitaiF8(value, data[offset:offset+8]), nil
}

// FloatTypeOptions provides CEL options for float types
func FloatTypeOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Types(&KaitaiFloat{}),

		cel.Function("f4",
			cel.Overload("f4_double",
				[]*cel.Type{cel.DoubleType},
				KaitaiF4Type,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return NewKaitaiF4(float32(val.(types.Double)), nil)
				}),
			),
		),

		cel.Function("f8",
			cel.Overload("f8_double",
				[]*cel.Type{cel.DoubleType},
				KaitaiF8Type,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return NewKaitaiF8(float64(val.(types.Double)), nil)
				}),
			),
		),

		cel.Function("double",
			cel.MemberOverload("kaitai_float_to_double",
				[]*cel.Type{KaitaiF4Type},
				cel.DoubleType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Double(val.(*KaitaiFloat).value)
				}),
			),
		),

		cel.Function("int",
			cel.MemberOverload("kaitai_float_to_int",
				[]*cel.Type{KaitaiF4Type},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(int64(val.(*KaitaiFloat).value))
				}),
			),
		),

		cel.Function("_+_",
			cel.Overload("add_kaitai_float_double",
				[]*cel.Type{KaitaiF4Type, cel.DoubleType},
				cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*KaitaiFloat).Add(rhs)
				}),
			),
		),

		cel.Function("is_nan",
			cel.MemberOverload("kaitai_float_is_nan",
				[]*cel.Type{KaitaiF4Type},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bool(math.IsNaN(val.(*KaitaiFloat).value))
				}),
			),
		),

		cel.Function("is_inf",
			cel.MemberOverload("kaitai_float_is_inf",
				[]*cel.Type{KaitaiF4Type},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bool(math.IsInf(val.(*KaitaiFloat).value, 0))
				}),
			),
		),
	}
}
