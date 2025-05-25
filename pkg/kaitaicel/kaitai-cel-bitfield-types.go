package kaitaicel

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// --- Bit Field Types ---

// KaitaiBitField represents a bit field extracted from bytes
type KaitaiBitField struct {
	value    uint64
	bits     int    // Number of bits in this field
	typeName string // e.g., "b12", "b1", "b4"
}

var KaitaiBitFieldType = cel.ObjectType("kaitai.BitField", traits.ComparerType)

// NewKaitaiBitField creates a new bit field
func NewKaitaiBitField(value uint64, bits int) (*KaitaiBitField, error) {
	if bits < 1 || bits > 64 {
		return nil, fmt.Errorf("bit field size must be 1-64, got %d", bits)
	}

	// Mask value to ensure it fits in specified bits
	mask := uint64((1 << bits) - 1)
	value = value & mask

	return &KaitaiBitField{
		value:    value,
		bits:     bits,
		typeName: fmt.Sprintf("b%d", bits),
	}, nil
}

// BitField interface methods
func (b *KaitaiBitField) AsBool() bool {
	return b.value != 0
}

func (b *KaitaiBitField) AsInt() int64 {
	return int64(b.value)
}

func (b *KaitaiBitField) AsUint() uint64 {
	return b.value
}

func (b *KaitaiBitField) BitCount() int {
	return b.bits
}

// Test if specific bit is set (0-indexed from LSB)
func (b *KaitaiBitField) TestBit(pos int) bool {
	if pos < 0 || pos >= b.bits {
		return false
	}
	return (b.value & (1 << pos)) != 0
}

// CEL Val interface
func (b *KaitaiBitField) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	switch typeDesc.Kind() {
	case reflect.Bool:
		return b.AsBool(), nil
	case reflect.Int, reflect.Int64:
		return b.AsInt(), nil
	case reflect.Uint, reflect.Uint64:
		return b.AsUint(), nil
	}
	return nil, fmt.Errorf("unsupported conversion from %s to %v", b.typeName, typeDesc)
}

func (b *KaitaiBitField) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.BoolType:
		return types.Bool(b.AsBool())
	case types.IntType:
		return types.Int(b.AsInt())
	case types.UintType:
		return types.Uint(b.AsUint())
	case types.StringType:
		return types.String(fmt.Sprintf("0b%b", b.value))
	}
	return types.NewErr("type conversion error from %s to %v", b.typeName, typeVal)
}

func (b *KaitaiBitField) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiBitField:
		return types.Bool(b.value == o.value && b.bits == o.bits)
	case types.Bool:
		return types.Bool(b.AsBool() == bool(o))
	case types.Int:
		return types.Bool(b.AsInt() == int64(o))
	case types.Uint:
		return types.Bool(b.AsUint() == uint64(o))
	}
	return types.False
}

func (b *KaitaiBitField) Type() ref.Type {
	return KaitaiBitFieldType
}

func (b *KaitaiBitField) Value() interface{} {
	return b
}

func (b *KaitaiBitField) KaitaiTypeName() string {
	return b.typeName
}

func (b *KaitaiBitField) RawBytes() []byte {
	// Bit fields don't have raw bytes - they're extracted from larger values
	return nil
}

func (b *KaitaiBitField) Compare(other ref.Val) ref.Val {
	var otherVal uint64
	switch o := other.(type) {
	case *KaitaiBitField:
		otherVal = o.value
	case types.Int:
		otherVal = uint64(o)
	case types.Uint:
		otherVal = uint64(o)
	default:
		return types.NewErr("cannot compare bit field with %v", other.Type())
	}

	if b.value < otherVal {
		return types.IntNegOne
	} else if b.value > otherVal {
		return types.IntOne
	}
	return types.IntZero
}

// BitReader helps extract bit fields from a byte stream
type BitReader struct {
	data      []byte
	bytePos   int
	bitPos    int // Current bit position within current byte (0-7)
	bigEndian bool
}

func NewBitReader(data []byte, bigEndian bool) *BitReader {
	return &BitReader{
		data:      data,
		bytePos:   0,
		bitPos:    0,
		bigEndian: bigEndian,
	}
}

// ReadBits reads n bits and returns as BitField
func (br *BitReader) ReadBits(n int) (*KaitaiBitField, error) {
	if n < 1 || n > 64 {
		return nil, fmt.Errorf("can only read 1-64 bits at a time")
	}

	var result uint64
	bitsRead := 0

	for bitsRead < n {
		if br.bytePos >= len(br.data) {
			return nil, fmt.Errorf("EOF while reading bits")
		}

		// Bits available in current byte
		bitsAvailable := 8 - br.bitPos
		bitsToRead := n - bitsRead
		if bitsToRead > bitsAvailable {
			bitsToRead = bitsAvailable
		}

		// Extract bits from current byte
		currentByte := br.data[br.bytePos]

		if br.bigEndian {
			// Big-endian: read from MSB
			shift := uint(8 - br.bitPos - bitsToRead)
			mask := byte((1 << bitsToRead) - 1)
			bits := (currentByte >> shift) & mask
			result = (result << bitsToRead) | uint64(bits)
		} else {
			// Little-endian: read from LSB
			mask := byte((1 << bitsToRead) - 1)
			bits := (currentByte >> br.bitPos) & mask
			result = result | (uint64(bits) << bitsRead)
		}

		bitsRead += bitsToRead
		br.bitPos += bitsToRead

		if br.bitPos >= 8 {
			br.bitPos = 0
			br.bytePos++
		}
	}

	return NewKaitaiBitField(result, n)
}

// BitFieldTypeOptions provides CEL options for bit field types
func BitFieldTypeOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Types(&KaitaiBitField{}),

		// Constructor
		cel.Function("bits",
			cel.Overload("bits_int_int",
				[]*cel.Type{cel.IntType, cel.IntType},
				KaitaiBitFieldType,
				cel.BinaryBinding(func(value, bits ref.Val) ref.Val {
					val := uint64(value.(types.Int))
					b := int(bits.(types.Int))

					bf, err := NewKaitaiBitField(val, b)
					if err != nil {
						return types.NewErr("bit field creation error: %v", err)
					}
					return bf
				}),
			),
		),

		// Methods
		cel.Function("bool",
			cel.MemberOverload("bitfield_to_bool",
				[]*cel.Type{KaitaiBitFieldType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bool(val.(*KaitaiBitField).AsBool())
				}),
			),
		),

		cel.Function("int",
			cel.MemberOverload("bitfield_to_int",
				[]*cel.Type{KaitaiBitFieldType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiBitField).AsInt())
				}),
			),
		),

		cel.Function("uint",
			cel.MemberOverload("bitfield_to_uint",
				[]*cel.Type{KaitaiBitFieldType},
				cel.UintType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Uint(val.(*KaitaiBitField).AsUint())
				}),
			),
		),

		cel.Function("test_bit",
			cel.MemberOverload("bitfield_test_bit",
				[]*cel.Type{KaitaiBitFieldType, cel.IntType},
				cel.BoolType,
				cel.BinaryBinding(func(bf, pos ref.Val) ref.Val {
					return types.Bool(bf.(*KaitaiBitField).TestBit(int(pos.(types.Int))))
				}),
			),
		),

		cel.Function("bit_count",
			cel.MemberOverload("bitfield_bit_count",
				[]*cel.Type{KaitaiBitFieldType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Int(val.(*KaitaiBitField).BitCount())
				}),
			),
		),

		// Bitwise operations
		cel.Function("_&_",
			cel.Overload("and_bitfield_bitfield",
				[]*cel.Type{KaitaiBitFieldType, KaitaiBitFieldType},
				KaitaiBitFieldType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := lhs.(*KaitaiBitField)
					r := rhs.(*KaitaiBitField)
					bits := l.bits
					if r.bits > bits {
						bits = r.bits
					}
					bf, _ := NewKaitaiBitField(l.value&r.value, bits)
					return bf
				}),
			),
		),

		cel.Function("_|_",
			cel.Overload("or_bitfield_bitfield",
				[]*cel.Type{KaitaiBitFieldType, KaitaiBitFieldType},
				KaitaiBitFieldType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := lhs.(*KaitaiBitField)
					r := rhs.(*KaitaiBitField)
					bits := l.bits
					if r.bits > bits {
						bits = r.bits
					}
					bf, _ := NewKaitaiBitField(l.value|r.value, bits)
					return bf
				}),
			),
		),

		cel.Function("_^_",
			cel.Overload("xor_bitfield_bitfield",
				[]*cel.Type{KaitaiBitFieldType, KaitaiBitFieldType},
				KaitaiBitFieldType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := lhs.(*KaitaiBitField)
					r := rhs.(*KaitaiBitField)
					bits := l.bits
					if r.bits > bits {
						bits = r.bits
					}
					bf, _ := NewKaitaiBitField(l.value^r.value, bits)
					return bf
				}),
			),
		),

		// Bit shift operations
		cel.Function("_<<_",
			cel.Overload("lshift_bitfield_int",
				[]*cel.Type{KaitaiBitFieldType, cel.IntType},
				KaitaiBitFieldType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					bf := lhs.(*KaitaiBitField)
					shift := int(rhs.(types.Int))
					result, _ := NewKaitaiBitField(bf.value<<shift, bf.bits)
					return result
				}),
			),
		),

		cel.Function("_>>_",
			cel.Overload("rshift_bitfield_int",
				[]*cel.Type{KaitaiBitFieldType, cel.IntType},
				KaitaiBitFieldType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					bf := lhs.(*KaitaiBitField)
					shift := int(rhs.(types.Int))
					result, _ := NewKaitaiBitField(bf.value>>shift, bf.bits)
					return result
				}),
			),
		),

		// Type checking
		cel.Function("is_bitfield",
			cel.Overload("is_bitfield_any",
				[]*cel.Type{cel.AnyType},
				cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					_, ok := val.(*KaitaiBitField)
					return types.Bool(ok)
				}),
			),
		),
	}
}
