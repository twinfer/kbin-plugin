package kaitaicel

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- KaitaiInt Tests ---

func TestKaitaiInt_Constructors(t *testing.T) {
	raw1 := []byte{0x12}
	kiU1 := NewKaitaiU1(0x12, raw1)
	assert.Equal(t, int64(0x12), kiU1.value)
	assert.Equal(t, "u1", kiU1.typeName)
	assert.Same(t, raw1, kiU1.raw) // Check if it's the same slice

	kiS4BE := NewS4BEFromValue(-10)
	assert.Equal(t, int64(-10), kiS4BE.value)
	assert.Equal(t, "s4be", kiS4BE.typeName)
	assert.Nil(t, kiS4BE.raw)
}

func TestKaitaiInt_Serialize(t *testing.T) {
	t.Run("FromRaw", func(t *testing.T) {
		raw := []byte{0xFE, 0xDC}
		ki := NewKaitaiU2(0xFEDC, raw) // Value doesn't matter if raw is present for Serialize
		assert.Equal(t, raw, ki.Serialize())
	})

	tests := []struct {
		name     string
		instance *KaitaiInt
		expected []byte
	}{
		{"u1", NewU1FromValue(0xAB), []byte{0xAB}},
		{"s1", NewS1FromValue(-1), []byte{0xFF}},
		{"u2le", NewU2LEFromValue(0x1234), []byte{0x34, 0x12}},
		{"u2be", NewU2BEFromValue(0x1234), []byte{0x12, 0x34}},
		{"s2le", NewS2LEFromValue(-2), []byte{0xFE, 0xFF}},
		{"s2be", NewS2BEFromValue(-2), []byte{0xFF, 0xFE}},
		{"u4le", NewU4LEFromValue(0x12345678), []byte{0x78, 0x56, 0x34, 0x12}},
		{"u4be", NewU4BEFromValue(0x12345678), []byte{0x12, 0x34, 0x56, 0x78}},
		{"s4le", NewS4LEFromValue(-3), []byte{0xFD, 0xFF, 0xFF, 0xFF}},
		{"s4be", NewS4BEFromValue(-3), []byte{0xFF, 0xFF, 0xFF, 0xFD}},
		{"u8le", NewU8LEFromValue(0x0102030405060708), []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}},
		{"u8be", NewU8BEFromValue(0x0102030405060708), []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{"s8le", NewS8LEFromValue(-4), []byte{0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
		{"s8be", NewS8BEFromValue(-4), []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFC}},
		// Test generic types defaulting to BigEndian
		{"u2_generic", &KaitaiInt{value: 0x1234, typeName: "u2"}, []byte{0x12, 0x34}},
		{"s4_generic", &KaitaiInt{value: int64(int32(0x87654321)), typeName: "s4"}, []byte{0x87, 0x65, 0x43, 0x21}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.instance.Serialize())
		})
	}
	
	// Test default fallback in Serialize
	unknownInt := &KaitaiInt{value: 0xAB, typeName: "uUnknown"}
	assert.Equal(t, []byte{0xAB}, unknownInt.Serialize(), "Serialize fallback for unknown type")

}

func TestKaitaiInt_Methods(t *testing.T) {
	raw := []byte{0x12}
	ki := NewKaitaiU1(0x9A, raw)
	assert.Equal(t, int64(0x9A), ki.Value())
	assert.Equal(t, "u1", ki.KaitaiTypeName())
	assert.Equal(t, raw, ki.RawBytes())
	assert.Equal(t, KaitaiU1Type, ki.Type())
}

func TestKaitaiInt_Equal(t *testing.T) {
	ki1a := NewKaitaiS2LEFromValue(100)
	ki1b := NewKaitaiS2LEFromValue(100)
	ki2 := NewKaitaiS2LEFromValue(200)
	celInt100 := types.Int(100)
	celInt200 := types.Int(200)
	celStr := types.String("100")

	assert.True(t, ki1a.Equal(ki1b).(types.Bool))
	assert.False(t, ki1a.Equal(ki2).(types.Bool))
	assert.True(t, ki1a.Equal(celInt100).(types.Bool))
	assert.False(t, ki1a.Equal(celInt200).(types.Bool))
	assert.False(t, ki1a.Equal(celStr).(types.Bool)) // Compare with different type
	assert.True(t, ki1a.Equal(types.Uint(100)).(types.Bool)) // Test against Uint
}

func TestKaitaiInt_Compare(t *testing.T) {
	ki10 := NewKaitaiS4BEFromValue(10)
	ki20 := NewKaitaiS4BEFromValue(20)
	celInt10 := types.Int(10)
	celInt5 := types.Int(5)
	celInt25 := types.Int(25)

	assert.Equal(t, types.IntZero, ki10.Compare(celInt10))      // 10 == 10
	assert.Equal(t, types.IntOne, ki10.Compare(celInt5))       // 10 > 5
	assert.Equal(t, types.IntNegOne, ki10.Compare(celInt25))   // 10 < 25
	assert.Equal(t, types.IntNegOne, ki10.Compare(ki20))       // 10 < 20

	// Error case
	assert.True(t, types.IsError(ki10.Compare(types.String("10"))))
}

func TestKaitaiInt_ConvertToNative(t *testing.T) {
	kiPos := NewKaitaiU1(42, nil)
	kiNeg := NewKaitaiS1(-42, nil)

	v, err := kiPos.ConvertToNative(reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)

	v, err = kiPos.ConvertToNative(reflect.TypeOf(uint8(0)))
	require.NoError(t, err)
	assert.Equal(t, uint8(42), v)
	
	_, err = kiNeg.ConvertToNative(reflect.TypeOf(uint8(0)))
	assert.Error(t, err, "Should error converting negative to unsigned")

	_, err = kiPos.ConvertToNative(reflect.TypeOf(""))
	assert.Error(t, err, "Should error converting to unsupported type string")
}

func TestKaitaiInt_ConvertToType(t *testing.T) {
	kiPos := NewKaitaiU1(42, nil)
	kiNeg := NewKaitaiS1(-42, nil)

	assert.Equal(t, types.Int(42), kiPos.ConvertToType(types.IntType))
	assert.Equal(t, types.Uint(42), kiPos.ConvertToType(types.UintType))
	assert.Equal(t, types.Double(42.0), kiPos.ConvertToType(types.DoubleType))
	assert.Equal(t, types.String("42"), kiPos.ConvertToType(types.StringType))

	assert.True(t, types.IsError(kiNeg.ConvertToType(types.UintType)), "Negative to Uint should be error")
	assert.True(t, types.IsError(kiPos.ConvertToType(types.BoolType)), "Int to Bool should be error")
}

func TestKaitaiInt_Add(t *testing.T) {
	ki1 := NewKaitaiU1(10, nil)
	ki2 := NewKaitaiU1(5, nil)
	celInt20 := types.Int(20)

	result1 := ki1.Add(ki2)
	assert.Equal(t, types.Int(15), result1)

	result2 := ki1.Add(celInt20)
	assert.Equal(t, types.Int(30), result2)

	assert.True(t, types.IsError(ki1.Add(types.String("5"))))
}


// --- KaitaiString Tests ---

func TestKaitaiString_NewKaitaiString(t *testing.T) {
	rawUTF8 := []byte("hello")
	ksUTF8, err := NewKaitaiString(rawUTF8, "UTF-8")
	require.NoError(t, err)
	assert.Equal(t, "hello", ksUTF8.value)
	assert.Equal(t, "UTF-8", ksUTF8.encoding)
	assert.Equal(t, rawUTF8, ksUTF8.raw)

	rawASCII := []byte("world")
	ksASCII, err := NewKaitaiString(rawASCII, "ASCII")
	require.NoError(t, err)
	assert.Equal(t, "world", ksASCII.value)

	_, err = NewKaitaiString([]byte{0x80, 0x81}, "ASCII") // Invalid ASCII
	assert.Error(t, err)

	_, err = NewKaitaiString([]byte("test"), "INVALID_ENCODING")
	assert.Error(t, err)
}

func TestKaitaiString_Serialize(t *testing.T) {
	raw := []byte("test_raw")
	ks, _ := NewKaitaiString(raw, "UTF-8")
	assert.Equal(t, raw, ks.Serialize())

	// Test serialization when raw is nil (should encode value)
	ksNoRaw := &KaitaiString{value: "hello", encoding: "UTF-8", raw: nil}
	assert.Equal(t, []byte("hello"), ksNoRaw.Serialize())

	ksASCII := &KaitaiString{value: "ascii", encoding: "ASCII", raw: nil}
	assert.Equal(t, []byte("ascii"), ksASCII.Serialize())
	
	ksFallback := &KaitaiString{value: "fallback", encoding: "NON_EXISTENT_FOR_ENCODE_FALLBACK", raw: nil}
	assert.Equal(t, []byte("fallback"), ksFallback.Serialize(), "Should fallback to UTF-8 bytes if encoding fails")
}

func TestKaitaiString_Methods(t *testing.T) {
	raw := []byte("Kaitai")
	ks, _ := NewKaitaiString(raw, "UTF-8")
	assert.Equal(t, "Kaitai", ks.Value())
	assert.Equal(t, "str", ks.KaitaiTypeName())
	assert.Equal(t, raw, ks.RawBytes())
	assert.Equal(t, KaitaiStringType, ks.Type())
	assert.Equal(t, 6, ks.Length()) // Character length
	assert.Equal(t, 6, ks.ByteSize()) // Byte length
	assert.Equal(t, types.Int(6), ks.Size()) // CEL Size (runes)
}

// ... (Equal, Compare, ConvertToNative, ConvertToType for KaitaiString - similar structure to KaitaiInt)

// --- KaitaiBytes Tests ---

func TestKaitaiBytes_NewKaitaiBytes(t *testing.T) {
	data := []byte{1, 2, 3}
	kb := NewKaitaiBytes(data)
	assert.Equal(t, data, kb.value)
}

func TestKaitaiBytes_Serialize(t *testing.T) {
	data := []byte{4, 5, 6}
	kb := NewKaitaiBytes(data)
	assert.Equal(t, data, kb.Serialize())
}

func TestKaitaiBytes_Methods(t *testing.T) {
	data := []byte{0xA, 0xB, 0xC}
	kb := NewKaitaiBytes(data)
	assert.Equal(t, data, kb.Value())
	assert.Equal(t, "bytes", kb.KaitaiTypeName())
	assert.Equal(t, data, kb.RawBytes())
	assert.Equal(t, KaitaiBytesType, kb.Type())
	assert.Equal(t, 3, kb.Length())
	assert.Equal(t, types.Int(3), kb.Size())
	
	b, err := kb.At(1)
	require.NoError(t, err)
	assert.Equal(t, byte(0xB), b)

	_, err = kb.At(5)
	assert.Error(t, err)
}

// ... (Equal, Compare, ConvertToNative, ConvertToType for KaitaiBytes)

// --- Factory NewKaitaiTypeFromValue Tests ---
func TestNewKaitaiTypeFromValue_Integers(t *testing.T) {
	// Successful conversions
	u1Val, err := NewKaitaiTypeFromValue(uint8(10), "u1")
	require.NoError(t, err)
	assert.Equal(t, int64(10), u1Val.(*KaitaiInt).Value())
	assert.Equal(t, "u1", u1Val.(*KaitaiInt).KaitaiTypeName())

	s2leVal, err := NewKaitaiTypeFromValue(int16(-5), "s2le")
	require.NoError(t, err)
	assert.Equal(t, int64(-5), s2leVal.(*KaitaiInt).Value())
	assert.Equal(t, "s2le", s2leVal.(*KaitaiInt).KaitaiTypeName())
	
	// From float64
	u4beVal, err := NewKaitaiTypeFromValue(float64(100.0), "u4be")
	require.NoError(t, err)
	assert.Equal(t, int64(100), u4beVal.(*KaitaiInt).Value())

	// Range error from float64
	_, err = NewKaitaiTypeFromValue(float64(300.0), "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range for u1")
	
	// Range error from int64
	_, err = NewKaitaiTypeFromValue(int64(300), "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range for u1")

	// Fractional part error for float to int
	_, err = NewKaitaiTypeFromValue(123.45, "u2be")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has fractional part, cannot convert to integer type u2be")
	
	// Conversion from string (should fail as NewKaitaiTypeFromValue doesn't handle string to int)
	_, err = NewKaitaiTypeFromValue("123", "u1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot convert string to u1")
}

func TestNewKaitaiTypeFromValue_Floats(t *testing.T) {
	f4leVal, err := NewKaitaiTypeFromValue(float32(1.5), "f4le")
	require.NoError(t, err)
	assert.InDelta(t, float64(1.5), f4leVal.(*KaitaiFloat).Value(), 0.00001)
	assert.Equal(t, "f4le", f4leVal.(*KaitaiFloat).KaitaiTypeName())

	f8beVal, err := NewKaitaiTypeFromValue(float64(123.456), "f8be")
	require.NoError(t, err)
	assert.Equal(t, float64(123.456), f8beVal.(*KaitaiFloat).Value())
	
	// Overflow float64 to float32
	_, err = NewKaitaiTypeFromValue(float64(math.MaxFloat64), "f4le")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "overflows float32")
}

func TestNewKaitaiTypeFromValue_Unsupported(t *testing.T) {
	_, err := NewKaitaiTypeFromValue(true, "bool_type_ksy") // Assuming bool_type_ksy is not in the factory
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type: bool_type_ksy")
}
