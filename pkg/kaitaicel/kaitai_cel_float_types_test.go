package kaitaicel

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- KaitaiFloat Tests ---

func TestKaitaiFloat_Constructors(t *testing.T) {
	rawF4 := []byte{0x00, 0x00, 0x80, 0x3F} // 1.0f
	kf4 := NewKaitaiF4(1.0, rawF4)
	assert.Equal(t, float64(1.0), kf4.value)
	assert.Equal(t, "f4", kf4.typeName)
	assert.Equal(t, rawF4, kf4.raw)

	kf8BE := NewF8BEFromValue(-2.5)
	assert.Equal(t, float64(-2.5), kf8BE.value)
	assert.Equal(t, "f8be", kf8BE.typeName)
	assert.Nil(t, kf8BE.raw)
}

func TestKaitaiFloat_Serialize(t *testing.T) {
	t.Run("FromRaw", func(t *testing.T) {
		raw := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40} // 2.0
		kf := NewKaitaiF8(2.0, raw)
		assert.Equal(t, raw, kf.Serialize())
	})

	tests := []struct {
		name     string
		instance *KaitaiFloat
		expected []byte
	}{
		{"f4le", NewF4LEFromValue(1.0), []byte{0x00, 0x00, 0x80, 0x3F}},
		{"f4be", NewF4BEFromValue(1.0), []byte{0x3F, 0x80, 0x00, 0x00}},
		{"f8le", NewF8LEFromValue(-2.0), []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0}},
		{"f8be", NewF8BEFromValue(-2.0), []byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		// Test generic types defaulting to BigEndian
		{"f4_generic", &KaitaiFloat{value: 1.0, typeName: "f4"}, []byte{0x3F, 0x80, 0x00, 0x00}},
		{"f8_generic", &KaitaiFloat{value: -2.0, typeName: "f8"}, []byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		// Test default fallback in Serialize
		{"unknown_float", &KaitaiFloat{value: 3.0, typeName: "fUnknown"}, nil}, // Expected will be set below
	}

	// Correcting byte order for default fallback test (f8be)
	unknownFloatBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(unknownFloatBytes, math.Float64bits(3.0))
	tests[len(tests)-1].expected = unknownFloatBytes

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.instance.Serialize())
		})
	}
}

func TestKaitaiFloat_Methods(t *testing.T) {
	raw := []byte{0x00, 0x00, 0x80, 0x3F}
	kf4 := NewKaitaiF4(1.0, raw)
	assert.Equal(t, float64(1.0), kf4.Value())
	assert.Equal(t, "f4", kf4.KaitaiTypeName())
	assert.Equal(t, raw, kf4.RawBytes())
	assert.Equal(t, KaitaiF4Type, kf4.Type())

	kf8 := NewKaitaiF8(2.5, nil)
	assert.Equal(t, KaitaiF8Type, kf8.Type())
}

func TestKaitaiFloat_Equal(t *testing.T) {
	kf1a := NewF4LEFromValue(1.5)
	kf1b := NewF4LEFromValue(1.5)
	kf2 := NewF4LEFromValue(2.5)
	celDouble1_5 := types.Double(1.5)
	celInt2 := types.Int(2) // For comparing float with int

	assert.True(t, bool(kf1a.Equal(kf1b).(types.Bool)))
	assert.False(t, bool(kf1a.Equal(kf2).(types.Bool)))
	assert.True(t, bool(kf1a.Equal(celDouble1_5).(types.Bool)))
	// KaitaiFloat.Equal for types.Int checks if float value is equal to int value
	assert.False(t, bool(kf1a.Equal(celInt2).(types.Bool)), "1.5 != 2")

	kf_int_val := NewF4LEFromValue(2.0)
	assert.True(t, bool(kf_int_val.Equal(celInt2).(types.Bool)), "2.0 == 2")

	// Test NaN equality
	kfNaN1 := NewF4LEFromValue(float32(math.NaN()))
	kfNaN2 := NewF4LEFromValue(float32(math.NaN()))
	celNaN := types.Double(math.NaN())
	assert.True(t, bool(kfNaN1.Equal(kfNaN2).(types.Bool)), "NaN should equal NaN for KaitaiFloat")
	assert.True(t, bool(kfNaN1.Equal(celNaN).(types.Bool)), "NaN should equal cel NaN")

	// Test Inf equality
	kfPosInf := NewF4LEFromValue(float32(math.Inf(1)))
	kfNegInf := NewF4LEFromValue(float32(math.Inf(-1)))
	celPosInf := types.Double(math.Inf(1))
	assert.True(t, bool(kfPosInf.Equal(celPosInf).(types.Bool)))
	assert.False(t, bool(kfPosInf.Equal(kfNegInf).(types.Bool)))
}

func TestKaitaiFloat_Compare(t *testing.T) {
	kf10_5 := NewF8BEFromValue(10.5)
	kf20_0 := NewF8BEFromValue(20.0)
	celDouble10_5 := types.Double(10.5)
	celInt5 := types.Int(5)
	celInt15 := types.Int(15)

	assert.Equal(t, types.IntZero, kf10_5.Compare(celDouble10_5))
	assert.Equal(t, types.IntOne, kf10_5.Compare(celInt5))     // 10.5 > 5
	assert.Equal(t, types.IntNegOne, kf10_5.Compare(celInt15)) // 10.5 < 15
	assert.Equal(t, types.IntNegOne, kf10_5.Compare(kf20_0))   // 10.5 < 20.0

	// Error case
	assert.True(t, types.IsError(kf10_5.Compare(types.String("10.5"))))
}

func TestKaitaiFloat_ConvertToNative(t *testing.T) {
	kf := NewF4LEFromValue(float32(42.75)) // Ensure input is float32 for f4

	v64, err := kf.ConvertToNative(reflect.TypeOf(float64(0)))
	require.NoError(t, err)
	assert.InDelta(t, float64(42.75), v64, 1e-6)

	v32, err := kf.ConvertToNative(reflect.TypeOf(float32(0)))
	require.NoError(t, err)
	assert.InDelta(t, float32(42.75), v32, 1e-6)

	vInt, err := kf.ConvertToNative(reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	assert.Equal(t, int64(42), vInt) // Truncation

	_, err = kf.ConvertToNative(reflect.TypeOf(""))
	assert.Error(t, err)
}

func TestKaitaiFloat_ConvertToType(t *testing.T) {
	kf := NewF4BEFromValue(float32(-3.5))
	assert.Equal(t, types.Double(-3.5), kf.ConvertToType(types.DoubleType))
	assert.Equal(t, types.Int(-3), kf.ConvertToType(types.IntType)) // Truncation
	assert.Equal(t, types.String("-3.5"), kf.ConvertToType(types.StringType))
	assert.True(t, types.IsError(kf.ConvertToType(types.BoolType)))
}

func TestKaitaiFloat_Add(t *testing.T) {
	kf1 := NewF4LEFromValue(1.5)
	kf2 := NewF8BEFromValue(2.25)
	celDouble_0_5 := types.Double(0.5)
	celInt2 := types.Int(2)

	res1 := kf1.Add(kf2) // 1.5 + 2.25 = 3.75
	require.IsType(t, types.Double(0), res1)
	assert.InDelta(t, 3.75, float64(res1.(types.Double)), 1e-9)

	res2 := kf1.Add(celDouble_0_5) // 1.5 + 0.5 = 2.0
	require.IsType(t, types.Double(0), res2)
	assert.InDelta(t, 2.0, float64(res2.(types.Double)), 1e-9)

	res3 := kf1.Add(celInt2) // 1.5 + 2 = 3.5
	require.IsType(t, types.Double(0), res3)
	assert.InDelta(t, 3.5, float64(res3.(types.Double)), 1e-9)

	assert.True(t, types.IsError(kf1.Add(types.String("error"))))
}

// --- Helper Function Tests ---

func TestReadFloatTypes(t *testing.T) {
	dataLE := []byte{0x00, 0x00, 0x80, 0x3F, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x40} // 1.0f, 2.0
	dataBE := []byte{0x3F, 0x80, 0x00, 0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00} // 1.0f, 2.0

	f4le, err := ReadF4LE(dataLE, 0)
	require.NoError(t, err)
	assert.Equal(t, float64(1.0), f4le.Value())
	assert.Equal(t, "f4", f4le.typeName)
	assert.Equal(t, dataLE[0:4], f4le.RawBytes())

	f8le, err := ReadF8LE(dataLE, 4)
	require.NoError(t, err)
	assert.Equal(t, float64(2.0), f8le.Value())
	assert.Equal(t, "f8", f8le.typeName)
	assert.Equal(t, dataLE[4:12], f8le.RawBytes())

	f4be, err := ReadF4BE(dataBE, 0)
	require.NoError(t, err)
	assert.Equal(t, float64(1.0), f4be.Value())
	assert.Equal(t, "f4", f4be.typeName)
	assert.Equal(t, dataBE[0:4], f4be.RawBytes())

	f8be, err := ReadF8BE(dataBE, 4)
	require.NoError(t, err)
	assert.Equal(t, float64(2.0), f8be.Value())
	assert.Equal(t, "f8", f8be.typeName)
	assert.Equal(t, dataBE[4:12], f8be.RawBytes())

	// EOF
	_, err = ReadF4LE(dataLE, 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
	_, err = ReadF8BE(dataBE, 8)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
	_, err = ReadF4BE(dataBE[:2], 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
	_, err = ReadF8LE(dataLE[:6], 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
}

func TestFloatTypeOptions_Registration(t *testing.T) {
	opts := FloatTypeOptions()
	assert.NotNil(t, opts)
	// Further testing of CEL function evaluation belongs in internal/cel/cel_test.go
}
