package kaitaicel

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"unicode/utf8"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/encoding/unicode/utf32"
)

// KaitaiType is the base interface for all Kaitai types
type KaitaiType interface {
	ref.Val
	// KaitaiTypeName returns the Kaitai type name (e.g., "u4", "str", etc.)
	KaitaiTypeName() string
	// RawBytes returns the original bytes if available
	RawBytes() []byte
}

// --- Integer Types ---

// KaitaiInt represents all Kaitai integer types
type KaitaiInt struct {
	value    int64
	typeName string // "u1", "u2", "u4", "u8", "s1", "s2", "s4", "s8"
	raw      []byte
}

// Integer type definitions
var (
	KaitaiU1Type = cel.ObjectType("kaitai.U1", traits.ComparerType, traits.AdderType)
	KaitaiU2Type = cel.ObjectType("kaitai.U2", traits.ComparerType, traits.AdderType)
	KaitaiU4Type = cel.ObjectType("kaitai.U4", traits.ComparerType, traits.AdderType)
	KaitaiU8Type = cel.ObjectType("kaitai.U8", traits.ComparerType, traits.AdderType)
	KaitaiS1Type = cel.ObjectType("kaitai.S1", traits.ComparerType, traits.AdderType)
	KaitaiS2Type = cel.ObjectType("kaitai.S2", traits.ComparerType, traits.AdderType)
	KaitaiS4Type = cel.ObjectType("kaitai.S4", traits.ComparerType, traits.AdderType)
	KaitaiS8Type = cel.ObjectType("kaitai.S8", traits.ComparerType, traits.AdderType)
)

// NewKaitaiU1 creates a new unsigned 1-byte integer
func NewKaitaiU1(value uint8, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "u1", raw: raw}
}

// NewKaitaiU2 creates a new unsigned 2-byte integer
func NewKaitaiU2(value uint16, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "u2", raw: raw}
}

// NewKaitaiU4 creates a new unsigned 4-byte integer
func NewKaitaiU4(value uint32, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "u4", raw: raw}
}

// NewKaitaiU8 creates a new unsigned 8-byte integer
func NewKaitaiU8(value uint64, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "u8", raw: raw}
}

// NewKaitaiS1 creates a new signed 1-byte integer
func NewKaitaiS1(value int8, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "s1", raw: raw}
}

// NewKaitaiS2 creates a new signed 2-byte integer
func NewKaitaiS2(value int16, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "s2", raw: raw}
}

// NewKaitaiS4 creates a new signed 4-byte integer
func NewKaitaiS4(value int32, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: int64(value), typeName: "s4", raw: raw}
}

// NewKaitaiS8 creates a new signed 8-byte integer
func NewKaitaiS8(value int64, raw []byte) *KaitaiInt {
	return &KaitaiInt{value: value, typeName: "s8", raw: raw}
}

// KaitaiInt interface implementation
func (k *KaitaiInt) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	switch typeDesc.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(k.value).Convert(typeDesc).Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if k.value >= 0 {
			return reflect.ValueOf(uint64(k.value)).Convert(typeDesc).Interface(), nil
		}
		return nil, fmt.Errorf("cannot convert negative value to unsigned type")
	}
	return nil, fmt.Errorf("unsupported conversion from %s to %v", k.typeName, typeDesc)
}

func (k *KaitaiInt) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.IntType:
		return types.Int(k.value)
	case types.UintType:
		if k.value >= 0 {
			return types.Uint(uint64(k.value))
		}
		return types.NewErr("cannot convert negative value to uint")
	case types.DoubleType:
		return types.Double(float64(k.value))
	case types.StringType:
		return types.String(fmt.Sprintf("%d", k.value))
	}
	return types.NewErr("type conversion error from %s to %v", k.typeName, typeVal)
}

func (k *KaitaiInt) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiInt:
		return types.Bool(k.value == o.value)
	case types.Int:
		return types.Bool(k.value == int64(o))
	case types.Uint:
		return types.Bool(k.value == int64(o))
	}
	return types.False
}

func (k *KaitaiInt) Type() ref.Type {
	switch k.typeName {
	case "u1":
		return KaitaiU1Type
	case "u2":
		return KaitaiU2Type
	case "u4":
		return KaitaiU4Type
	case "u8":
		return KaitaiU8Type
	case "s1":
		return KaitaiS1Type
	case "s2":
		return KaitaiS2Type
	case "s4":
		return KaitaiS4Type
	case "s8":
		return KaitaiS8Type
	}
	return types.IntType
}

func (k *KaitaiInt) Value() interface{} {
	return k.value
}

func (k *KaitaiInt) KaitaiTypeName() string {
	return k.typeName
}

func (k *KaitaiInt) RawBytes() []byte {
	return k.raw
}

// Arithmetic operations
func (k *KaitaiInt) Add(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiInt:
		return types.Int(k.value + o.value)
	case types.Int:
		return types.Int(k.value + int64(o))
	}
	return types.NewErr("cannot add %v to %s", other.Type(), k.typeName)
}

func (k *KaitaiInt) Compare(other ref.Val) ref.Val {
	var otherVal int64
	switch o := other.(type) {
	case *KaitaiInt:
		otherVal = o.value
	case types.Int:
		otherVal = int64(o)
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

// --- String Types ---

// KaitaiString represents Kaitai string types with encoding
type KaitaiString struct {
	value    string
	encoding string // "UTF-8", "ASCII", "UTF-16LE", etc.
	raw      []byte
}

var KaitaiStringType = cel.ObjectType("kaitai.String", traits.ComparerType, traits.SizerType)

// NewKaitaiString creates a new Kaitai string with specified encoding
func NewKaitaiString(raw []byte, encoding string) (*KaitaiString, error) {
	str, err := decodeString(raw, encoding)
	if err != nil {
		return nil, err
	}
	return &KaitaiString{
		value:    str,
		encoding: encoding,
		raw:      raw,
	}, nil
}

func decodeString(data []byte, encodingName string) (string, error) {
	var enc encoding.Encoding

	switch encodingName {
	case "ASCII", "UTF-8":
		// For ASCII and UTF-8, we can use string conversion directly
		if encodingName == "ASCII" {
			// Validate ASCII
			for _, b := range data {
				if b > 127 {
					return "", fmt.Errorf("invalid ASCII character: %d", b)
				}
			}
		}
		return string(data), nil
	case "UTF-16LE":
		enc = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	case "UTF-16BE":
		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "UTF-32LE":
		enc = utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM)
	case "UTF-32BE":
		enc = utf32.UTF32(utf32.BigEndian, utf32.IgnoreBOM)
	case "CP437", "IBM437":
		enc = charmap.CodePage437
	case "SHIFT_JIS", "SJIS":
		enc = japanese.ShiftJIS
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encodingName)
	}

	if enc != nil {
		decoder := enc.NewDecoder()
		result, err := decoder.String(string(data))
		if err != nil {
			return "", err
		}
		return result, nil
	}

	return string(data), nil
}

// KaitaiString interface implementation
func (k *KaitaiString) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	if typeDesc.Kind() == reflect.String {
		return k.value, nil
	}
	if typeDesc == reflect.TypeOf([]byte{}) {
		return k.raw, nil
	}
	return nil, fmt.Errorf("unsupported conversion from kaitai.String to %v", typeDesc)
}

func (k *KaitaiString) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.StringType:
		return types.String(k.value)
	case types.BytesType:
		return types.Bytes(k.raw)
	}
	return types.NewErr("type conversion error from kaitai.String to %v", typeVal)
}

func (k *KaitaiString) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiString:
		return types.Bool(k.value == o.value)
	case types.String:
		return types.Bool(k.value == string(o))
	}
	return types.False
}

func (k *KaitaiString) Type() ref.Type {
	return KaitaiStringType
}

func (k *KaitaiString) Value() interface{} {
	return k.value
}

func (k *KaitaiString) KaitaiTypeName() string {
	return "str"
}

func (k *KaitaiString) RawBytes() []byte {
	return k.raw
}

func (k *KaitaiString) Compare(other ref.Val) ref.Val {
	var otherStr string
	switch o := other.(type) {
	case *KaitaiString:
		otherStr = o.value
	case types.String:
		otherStr = string(o)
	default:
		return types.NewErr("cannot compare string with %v", other.Type())
	}

	if k.value < otherStr {
		return types.IntNegOne
	} else if k.value > otherStr {
		return types.IntOne
	}
	return types.IntZero
}

func (k *KaitaiString) Size() ref.Val {
	return types.Int(utf8.RuneCountInString(k.value))
}

// Additional string methods
func (k *KaitaiString) Length() int {
	return len(k.value)
}

func (k *KaitaiString) ByteSize() int {
	return len(k.raw)
}

// --- Bytes Type ---

// KaitaiBytes represents raw byte arrays
type KaitaiBytes struct {
	value []byte
}

var KaitaiBytesType = cel.ObjectType("kaitai.Bytes", traits.ComparerType, traits.SizerType)

// NewKaitaiBytes creates a new Kaitai bytes value
func NewKaitaiBytes(data []byte) *KaitaiBytes {
	return &KaitaiBytes{value: data}
}

func (k *KaitaiBytes) ConvertToNative(typeDesc reflect.Type) (interface{}, error) {
	if typeDesc == reflect.TypeOf([]byte{}) {
		return k.value, nil
	}
	return nil, fmt.Errorf("unsupported conversion from kaitai.Bytes to %v", typeDesc)
}

func (k *KaitaiBytes) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.BytesType:
		return types.Bytes(k.value)
	case types.StringType:
		return types.String(string(k.value))
	}
	return types.NewErr("type conversion error from kaitai.Bytes to %v", typeVal)
}

func (k *KaitaiBytes) Equal(other ref.Val) ref.Val {
	switch o := other.(type) {
	case *KaitaiBytes:
		return types.Bool(string(k.value) == string(o.value))
	case types.Bytes:
		return types.Bool(string(k.value) == string(o))
	}
	return types.False
}

func (k *KaitaiBytes) Type() ref.Type {
	return KaitaiBytesType
}

func (k *KaitaiBytes) Value() interface{} {
	return k.value
}

func (k *KaitaiBytes) KaitaiTypeName() string {
	return "bytes"
}

func (k *KaitaiBytes) RawBytes() []byte {
	return k.value
}

func (k *KaitaiBytes) Compare(other ref.Val) ref.Val {
	var otherBytes []byte
	switch o := other.(type) {
	case *KaitaiBytes:
		otherBytes = o.value
	case types.Bytes:
		otherBytes = []byte(o)
	default:
		return types.NewErr("cannot compare bytes with %v", other.Type())
	}

	result := string(k.value)
	otherStr := string(otherBytes)

	if result < otherStr {
		return types.IntNegOne
	} else if result > otherStr {
		return types.IntOne
	}
	return types.IntZero
}

func (k *KaitaiBytes) Size() ref.Val {
	return types.Int(len(k.value))
}

// Additional bytes methods
func (k *KaitaiBytes) Length() int {
	return len(k.value)
}

func (k *KaitaiBytes) At(index int) (byte, error) {
	if index < 0 || index >= len(k.value) {
		return 0, fmt.Errorf("index %d out of bounds", index)
	}
	return k.value[index], nil
}

// --- Helper Functions ---

// ReadU1 reads unsigned 1-byte integer
func ReadU1(data []byte, offset int) (*KaitaiInt, error) {
	if offset >= len(data) {
		return nil, fmt.Errorf("EOF: cannot read u1 at offset %d", offset)
	}
	return NewKaitaiU1(data[offset], data[offset:offset+1]), nil
}

// ReadU2LE reads unsigned 2-byte little-endian integer
func ReadU2LE(data []byte, offset int) (*KaitaiInt, error) {
	if offset+2 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read u2le at offset %d", offset)
	}
	value := binary.LittleEndian.Uint16(data[offset:])
	return NewKaitaiU2(value, data[offset:offset+2]), nil
}

// ReadU2BE reads unsigned 2-byte big-endian integer
func ReadU2BE(data []byte, offset int) (*KaitaiInt, error) {
	if offset+2 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read u2be at offset %d", offset)
	}
	value := binary.BigEndian.Uint16(data[offset:])
	return NewKaitaiU2(value, data[offset:offset+2]), nil
}

// ReadU4LE reads unsigned 4-byte little-endian integer
func ReadU4LE(data []byte, offset int) (*KaitaiInt, error) {
	if offset+4 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read u4le at offset %d", offset)
	}
	value := binary.LittleEndian.Uint32(data[offset:])
	return NewKaitaiU4(value, data[offset:offset+4]), nil
}

// ReadU4BE reads unsigned 4-byte big-endian integer
func ReadU4BE(data []byte, offset int) (*KaitaiInt, error) {
	if offset+4 > len(data) {
		return nil, fmt.Errorf("EOF: cannot read u4be at offset %d", offset)
	}
	value := binary.BigEndian.Uint32(data[offset:])
	return NewKaitaiU4(value, data[offset:offset+4]), nil
}
