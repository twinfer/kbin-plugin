package kaitaicel

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// BcdType represents a BCD (Binary-Coded Decimal) value
type BcdType struct {
	raw      []byte
	digits   []int // Decoded digits
	asInt    int64
	asStr    string
	encoding string // "le", "be", "ltr", "rtl"
}

var KaitaiBcdType = cel.ObjectType("kaitai.Bcd", traits.ComparerType)

// BCD encoding modes
const (
	BcdLtr = "ltr" // Left-to-right (most significant digit first)
	BcdRtl = "rtl" // Right-to-left (least significant digit first)
	BcdLe  = "le"  // Little-endian BCD
	BcdBe  = "be"  // Big-endian BCD
)

// NewBcdType creates a new BCD type with specified encoding
func NewBcdType(data []byte, encoding string) (*BcdType, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty BCD data")
	}

	bcd := &BcdType{
		raw:      data,
		encoding: encoding,
		digits:   make([]int, 0, len(data)*2),
	}

	if err := bcd.decode(); err != nil {
		return nil, err
	}

	return bcd, nil
}

// decode processes the BCD bytes according to the encoding
func (b *BcdType) decode() error {
	// Extract digits from BCD bytes
	for _, byte := range b.raw {
		highNibble := int((byte >> 4) & 0xF)
		lowNibble := int(byte & 0xF)

		// Validate BCD digits (0-9)
		if highNibble > 9 || lowNibble > 9 {
			return fmt.Errorf("invalid BCD digit: byte 0x%02X contains non-decimal nibbles", byte)
		}

		b.digits = append(b.digits, highNibble, lowNibble)
	}

	// Process according to encoding
	switch b.encoding {
	case BcdLtr, BcdBe:
		// Left-to-right: most significant digit first
		b.processLtr()
	case BcdRtl, BcdLe:
		// Right-to-left: least significant digit first
		b.processRtl()
	default:
		return fmt.Errorf("unsupported BCD encoding: %s", b.encoding)
	}

	return nil
}

// processLtr processes digits in left-to-right order
func (b *BcdType) processLtr() {
	b.asInt = 0
	var strBuilder strings.Builder

	for _, digit := range b.digits {
		b.asInt = b.asInt*10 + int64(digit)
		strBuilder.WriteByte('0' + byte(digit))
	}

	b.asStr = strBuilder.String()
}

// processRtl processes digits in right-to-left order
func (b *BcdType) processRtl() {
	b.asInt = 0
	multiplier := int64(1)

	// Build integer from right to left
	for i := len(b.digits) - 1; i >= 0; i-- {
		b.asInt += int64(b.digits[i]) * multiplier
		multiplier *= 10
	}

	// Build string in reverse order
	var strBuilder strings.Builder
	for i := len(b.digits) - 1; i >= 0; i-- {
		strBuilder.WriteByte('0' + byte(b.digits[i]))
	}

	b.asStr = strBuilder.String()
}

// AsInt returns the integer representation of the BCD value
func (b *BcdType) AsInt() int64 {
	return b.asInt
}

// AsStr returns the string representation of the BCD value
func (b *BcdType) AsStr() string {
	return b.asStr
}

// CEL Val interface implementation
func (b *BcdType) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	switch typeDesc.Kind() {
	case reflect.Int, reflect.Int64:
		return b.asInt, nil
	case reflect.String:
		return b.asStr, nil
	case reflect.Slice:
		if typeDesc.Elem().Kind() == reflect.Uint8 {
			return b.raw, nil
		}
	}

	// Support conversion to map for compatibility
	if typeDesc == reflect.TypeOf(map[string]interface{}{}) {
		return map[string]interface{}{
			"AsInt":    b.asInt,
			"AsStr":    b.asStr,
			"raw":      b.raw,
			"encoding": b.encoding,
		}, nil
	}

	return nil, fmt.Errorf("unsupported conversion from BCD to %v", typeDesc)
}

func (b *BcdType) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.IntType:
		return types.Int(b.asInt)
	case types.StringType:
		return types.String(b.asStr)
	case types.BytesType:
		return types.Bytes(b.raw)
	case types.MapType:
		// Return a map with AsInt and AsStr fields
		return types.NewDynamicMap(types.DefaultTypeAdapter, map[string]ref.Val{
			"AsInt":    types.Int(b.asInt),
			"AsStr":    types.String(b.asStr),
			"raw":      types.Bytes(b.raw),
			"encoding": types.String(b.encoding),
		})
	}
	return types.NewErr("type conversion error from BCD to %v", typeVal)
}

func (b *BcdType) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *BcdType:
		// BCDs are equal if they have the same integer value
		return types.Bool(b.asInt == o.asInt)
	case types.Int:
		return types.Bool(b.asInt == int64(o))
	case types.String:
		return types.Bool(b.asStr == string(o))
	}
	return types.False
}

func (b *BcdType) Type() ref.Type {
	return KaitaiBcdType
}

func (b *BcdType) Value() interface{} {
	return b
}

func (b *BcdType) KaitaiTypeName() string {
	return "bcd"
}

func (b *BcdType) RawBytes() []byte {
	return b.raw
}

// Serialize returns the binary representation of this BCD value
func (b *BcdType) Serialize() []byte {
	return b.raw
}

func (b *BcdType) Compare(other ref.Val) ref.Val {
	var otherInt int64

	switch o := other.(type) {
	case *BcdType:
		otherInt = o.asInt
	case types.Int:
		otherInt = int64(o)
	default:
		return types.NewErr("cannot compare BCD with %v", other.Type())
	}

	if b.asInt < otherInt {
		return types.IntNegOne
	} else if b.asInt > otherInt {
		return types.IntOne
	}
	return types.IntZero
}

// BcdTypeOptions provides CEL options for BCD type
func BcdTypeOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Register BCD type
		cel.Types(&BcdType{}),

		// Constructor functions
		cel.Function("bcd",
			cel.Overload("bcd_bytes_string",
				[]*cel.Type{cel.BytesType, cel.StringType},
				KaitaiBcdType,
				cel.BinaryBinding(func(data, encoding ref.Val) ref.Val {
					bytes := []byte(data.(types.Bytes))
					enc := string(encoding.(types.String))

					bcd, err := NewBcdType(bytes, enc)
					if err != nil {
						return types.NewErr("failed to create BCD: %v", err)
					}
					return bcd
				}),
			),
		),

		// BCD methods
		cel.Function("asInt",
			cel.MemberOverload("bcd_as_int",
				[]*cel.Type{KaitaiBcdType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*BcdType).asInt)
				}),
			),
		),

		cel.Function("asStr",
			cel.MemberOverload("bcd_as_str",
				[]*cel.Type{KaitaiBcdType},
				cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.String(val.(*BcdType).asStr)
				}),
			),
		),

		// Allow direct int() and str() conversions
		cel.Function("int",
			cel.MemberOverload("bcd_to_int",
				[]*cel.Type{KaitaiBcdType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*BcdType).asInt)
				}),
			),
		),

		cel.Function("str",
			cel.MemberOverload("bcd_to_str",
				[]*cel.Type{KaitaiBcdType},
				cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.String(val.(*BcdType).asStr)
				}),
			),
		),

		// Comparison operations
		cel.Function("_==_",
			cel.Overload("eq_bcd_int",
				[]*cel.Type{KaitaiBcdType, cel.IntType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*BcdType).Equal(rhs)
				}),
			),
			cel.Overload("eq_bcd_string",
				[]*cel.Type{KaitaiBcdType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*BcdType).Equal(rhs)
				}),
			),
			cel.Overload("eq_bcd_bcd",
				[]*cel.Type{KaitaiBcdType, KaitaiBcdType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return lhs.(*BcdType).Equal(rhs)
				}),
			),
		),

		cel.Function("_<_",
			cel.Overload("less_bcd_int",
				[]*cel.Type{KaitaiBcdType, cel.IntType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					cmp := lhs.(*BcdType).Compare(rhs)
					return types.Bool(cmp == types.IntNegOne)
				}),
			),
			cel.Overload("less_bcd_bcd",
				[]*cel.Type{KaitaiBcdType, KaitaiBcdType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					cmp := lhs.(*BcdType).Compare(rhs)
					return types.Bool(cmp == types.IntNegOne)
				}),
			),
		),

		// Type checking
		cel.Function("is_bcd",
			cel.Overload("is_bcd_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					_, ok := val.(*BcdType)
					return types.Bool(ok)
				}),
			),
		),
	}
}

// Helper functions for common BCD operations

// DecodeBcdLtr decodes BCD in left-to-right order
func DecodeBcdLtr(data []byte) (*BcdType, error) {
	return NewBcdType(data, BcdLtr)
}

// DecodeBcdRtl decodes BCD in right-to-left order
func DecodeBcdRtl(data []byte) (*BcdType, error) {
	return NewBcdType(data, BcdRtl)
}

// IsBcdValid checks if bytes contain valid BCD data
func IsBcdValid(data []byte) bool {
	for _, b := range data {
		if (b>>4) > 9 || (b&0xF) > 9 {
			return false
		}
	}
	return true
}
