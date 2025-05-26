package kaitaicel

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- KaitaiBitField Tests ---

func TestKaitaiBitField_NewKaitaiBitField(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		bf, err := NewKaitaiBitField(0b10101, 5) // value 21, 5 bits
		require.NoError(t, err)
		assert.Equal(t, uint64(21), bf.value)
		assert.Equal(t, 5, bf.bits)
		assert.Equal(t, "b5", bf.typeName)

		bf2, err := NewKaitaiBitField(0xFF, 3) // value 0xFF masked to 3 bits (0x07)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), bf2.value)
		assert.Equal(t, 3, bf2.bits)
		assert.Equal(t, "b3", bf2.typeName)

		bf64, err := NewKaitaiBitField(uint64(1)<<63, 64)
		require.NoError(t, err)
		assert.Equal(t, uint64(1)<<63, bf64.value)
		assert.Equal(t, 64, bf64.bits)
		assert.Equal(t, "b64", bf64.typeName)
	})

	t.Run("InvalidBitSize", func(t *testing.T) {
		_, err := NewKaitaiBitField(5, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bit field size must be 1-64")

		_, err = NewKaitaiBitField(5, 65)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bit field size must be 1-64")

		_, err = NewKaitaiBitField(5, -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bit field size must be 1-64")
	})
}

func TestKaitaiBitField_Accessors(t *testing.T) {
	bf5_val21, _ := NewKaitaiBitField(0b10101, 5) // 21
	bf1_val0, _ := NewKaitaiBitField(0, 1)
	bf1_val1, _ := NewKaitaiBitField(1, 1)

	assert.True(t, bf5_val21.AsBool())
	assert.Equal(t, int64(21), bf5_val21.AsInt())
	assert.Equal(t, uint64(21), bf5_val21.AsUint())
	assert.Equal(t, 5, bf5_val21.BitCount())

	assert.False(t, bf1_val0.AsBool())
	assert.True(t, bf1_val1.AsBool())

	// TestBit (0-indexed from LSB)
	// 0b10101 -> LSB is bit 0
	assert.True(t, bf5_val21.TestBit(0))  // 1
	assert.False(t, bf5_val21.TestBit(1)) // 0
	assert.True(t, bf5_val21.TestBit(2))  // 1
	assert.False(t, bf5_val21.TestBit(3)) // 0
	assert.True(t, bf5_val21.TestBit(4))  // 1

	assert.False(t, bf5_val21.TestBit(5), "Out of bounds")
	assert.False(t, bf5_val21.TestBit(-1), "Out of bounds negative")
}

func TestKaitaiBitField_InterfaceMethods(t *testing.T) {
	bf, _ := NewKaitaiBitField(0b110, 3)
	assert.Equal(t, "b3", bf.KaitaiTypeName())
	assert.Nil(t, bf.RawBytes())
	assert.Nil(t, bf.Serialize())
	assert.Equal(t, KaitaiBitFieldType, bf.Type())
	assert.Same(t, bf, bf.Value())
}

func TestKaitaiBitField_Equal(t *testing.T) {
	bf_5_val21a, _ := NewKaitaiBitField(21, 5)
	bf_5_val21b, _ := NewKaitaiBitField(21, 5)
	bf_5_val10, _ := NewKaitaiBitField(10, 5)
	bf_3_val5, _ := NewKaitaiBitField(5, 3)

	assert.True(t, bool(bf_5_val21a.Equal(bf_5_val21b).(types.Bool)))
	assert.False(t, bool(bf_5_val21a.Equal(bf_5_val10).(types.Bool)))
	assert.False(t, bool(bf_5_val21a.Equal(bf_3_val5).(types.Bool)), "Different bit counts")

	assert.True(t, bool(bf_5_val21a.Equal(types.Int(21)).(types.Bool)))
	assert.False(t, bool(bf_5_val21a.Equal(types.Int(10)).(types.Bool)))
	assert.True(t, bool(bf_5_val21a.Equal(types.Uint(21)).(types.Bool)))

	bf_1_true, _ := NewKaitaiBitField(1, 1)
	bf_1_false, _ := NewKaitaiBitField(0, 1)
	assert.True(t, bool(bf_1_true.Equal(types.True).(types.Bool)))
	assert.False(t, bool(bf_1_true.Equal(types.False).(types.Bool)))
	assert.True(t, bool(bf_1_false.Equal(types.False).(types.Bool)))
	assert.False(t, bool(bf_1_false.Equal(types.True).(types.Bool)))

	assert.False(t, bool(bf_5_val21a.Equal(types.String("21")).(types.Bool)))
}

func TestKaitaiBitField_Compare(t *testing.T) {
	bf10, _ := NewKaitaiBitField(10, 8)
	bf20, _ := NewKaitaiBitField(20, 8)
	bf10_4bits, _ := NewKaitaiBitField(10, 4)

	assert.Equal(t, types.IntZero, bf10.Compare(bf10))
	assert.Equal(t, types.IntNegOne, bf10.Compare(bf20)) // 10 < 20
	assert.Equal(t, types.IntOne, bf20.Compare(bf10))    // 20 > 10

	// Comparing different bit sizes should still work based on value
	assert.Equal(t, types.IntZero, bf10.Compare(bf10_4bits))

	assert.Equal(t, types.IntZero, bf10.Compare(types.Int(10)))
	assert.Equal(t, types.IntOne, bf10.Compare(types.Uint(5)))
	assert.Equal(t, types.IntNegOne, bf10.Compare(types.Int(15)))

	assert.True(t, types.IsError(bf10.Compare(types.String("10"))))
}

func TestKaitaiBitField_ConvertToNative(t *testing.T) {
	bf_true, _ := NewKaitaiBitField(1, 1)
	bf_false, _ := NewKaitaiBitField(0, 1)
	bf_val42, _ := NewKaitaiBitField(42, 7)

	bVal, err := bf_true.ConvertToNative(reflect.TypeOf(true))
	require.NoError(t, err)
	assert.Equal(t, true, bVal)
	bVal, err = bf_false.ConvertToNative(reflect.TypeOf(true))
	require.NoError(t, err)
	assert.Equal(t, false, bVal)

	iVal, err := bf_val42.ConvertToNative(reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	assert.Equal(t, int64(42), iVal)

	uVal, err := bf_val42.ConvertToNative(reflect.TypeOf(uint64(0)))
	require.NoError(t, err)
	assert.Equal(t, uint64(42), uVal)

	_, err = bf_val42.ConvertToNative(reflect.TypeOf(""))
	assert.Error(t, err)
}

func TestKaitaiBitField_ConvertToType(t *testing.T) {
	bf_true, _ := NewKaitaiBitField(1, 1)
	bf_val42, _ := NewKaitaiBitField(42, 7) // 0b0101010

	assert.Equal(t, types.Bool(true), bf_true.ConvertToType(types.BoolType))
	assert.Equal(t, types.Int(42), bf_val42.ConvertToType(types.IntType))
	assert.Equal(t, types.Uint(42), bf_val42.ConvertToType(types.UintType))
	// String representation is 0b prefix + binary string
	assert.Equal(t, types.String("0b101010"), bf_val42.ConvertToType(types.StringType))

	assert.True(t, types.IsError(bf_val42.ConvertToType(types.DoubleType)))
}

// --- BitReader Tests ---

func TestBitReader_NewBitReader(t *testing.T) {
	data := []byte{0xFF}
	brBE := NewBitReader(data, true)  // Big Endian
	brLE := NewBitReader(data, false) // Little Endian

	assert.Equal(t, data, brBE.data)
	assert.True(t, brBE.bigEndian)
	assert.Equal(t, 0, brBE.bytePos)
	assert.Equal(t, 0, brBE.bitPos)

	assert.Equal(t, data, brLE.data)
	assert.False(t, brLE.bigEndian)
	assert.Equal(t, 0, brLE.bytePos)
	assert.Equal(t, 0, brLE.bitPos)
}

func TestBitReader_ReadBits_BigEndian(t *testing.T) {
	// Data: 0xB4, 0x5A  (10110100, 01011010)
	data := []byte{0xB4, 0x5A}
	br := NewBitReader(data, true) // Big Endian

	// Read 3 bits from 10110100 -> 101 (5)
	bf, err := br.ReadBits(3)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b101), bf.AsUint())
	assert.Equal(t, 3, bf.BitCount())
	assert.Equal(t, 0, br.bytePos)
	assert.Equal(t, 3, br.bitPos)

	// Read 4 bits from 10110100 -> 1010 (10)
	bf, err = br.ReadBits(4)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b1010), bf.AsUint())
	assert.Equal(t, 4, bf.BitCount())
	assert.Equal(t, 0, br.bytePos)
	assert.Equal(t, 7, br.bitPos)

	// Read 2 bits (1 from 0xB4: 0, 1 from 0x5A: 0) -> 00 (0)
	// 0xB4: 1011010(0) - remaining bit
	// 0x5A: (0)1011010 - first bit
	bf, err = br.ReadBits(2)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b00), bf.AsUint())
	assert.Equal(t, 2, bf.BitCount())
	assert.Equal(t, 1, br.bytePos)
	assert.Equal(t, 1, br.bitPos) // bytePos advanced, bitPos is 1 (after reading 1st bit of 0x5A)

	// Read 7 bits from 01011010 (remaining after 1st bit: 1011010) -> 1011010 (90)
	bf, err = br.ReadBits(7)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b1011010), bf.AsUint())
	assert.Equal(t, 7, bf.BitCount())
	assert.Equal(t, 2, br.bytePos)
	assert.Equal(t, 0, br.bitPos) // EOF (bytePos is 2, bitPos is 0, indicating end of data)

	_, err = br.ReadBits(1) // Read past EOF
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
}

func TestBitReader_ReadBits_LittleEndian(t *testing.T) {
	// Data: 0xB4, 0x5A  (10110100, 01011010)
	data := []byte{0xB4, 0x5A}
	br := NewBitReader(data, false) // Little Endian

	// Read 3 bits from 10110100 (0xB4) -> 100 (4) (LSB first: 0,0,1)
	bf, err := br.ReadBits(3)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b100), bf.AsUint())
	assert.Equal(t, 3, bf.BitCount())
	assert.Equal(t, 0, br.bytePos)
	assert.Equal(t, 3, br.bitPos)

	// Read 4 bits from 10110100 -> 1011 (11) (next 4 bits: 1,0,1,1)
	bf, err = br.ReadBits(4)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b1011), bf.AsUint())
	assert.Equal(t, 4, bf.BitCount())
	assert.Equal(t, 0, br.bytePos)
	assert.Equal(t, 7, br.bitPos)

	// Read 2 bits (1 from 0xB4: 1, 1 from 0x5A: 0) -> 01 (1)
	// 0xB4: (1)0110100 - remaining bit is MSB '1' (but read LSB-first from this byte, so this is the last bit of this byte)
	// 0x5A: 0101101(0) - first bit from LSB is '0'
	// Result: (bit0 from 0x5A) then (bit7 from 0xB4) = 01
	bf, err = br.ReadBits(2)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b01), bf.AsUint())
	assert.Equal(t, 2, bf.BitCount())
	assert.Equal(t, 1, br.bytePos)
	assert.Equal(t, 1, br.bitPos)

	// Read 7 bits from 01011010 (remaining after 1st bit: 0101101) -> 0101101 (45)
	// (Bits read: 1,0,1,1,0,1,0)
	bf, err = br.ReadBits(7)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b0101101), bf.AsUint())
	assert.Equal(t, 7, bf.BitCount())
	assert.Equal(t, 2, br.bytePos)
	assert.Equal(t, 0, br.bitPos) // EOF

	_, err = br.ReadBits(1) // Read past EOF
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
}

func TestBitReader_ReadBits_AcrossMultipleBytes_BigEndian(t *testing.T) {
	data := []byte{0b10101010, 0b11001100, 0b11110000}
	br := NewBitReader(data, true)

	// Read 12 bits: 101010101100 (2732)
	bf, err := br.ReadBits(12)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b101010101100), bf.AsUint())
	assert.Equal(t, 1, br.bytePos)
	assert.Equal(t, 4, br.bitPos) // 12 bits = 1 byte and 4 bits

	// Read remaining 4 bits of current byte: 1100
	bf, err = br.ReadBits(4)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b1100), bf.AsUint())
	assert.Equal(t, 2, br.bytePos) // Advanced to next byte
	assert.Equal(t, 0, br.bitPos)

	// Read next 8 bits: 11110000
	bf, err = br.ReadBits(8)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b11110000), bf.AsUint())
	assert.Equal(t, 3, br.bytePos)
	assert.Equal(t, 0, br.bitPos) // EOF
}

func TestBitReader_ReadBits_AcrossMultipleBytes_LittleEndian(t *testing.T) {
	// Data: 0xAA (10101010), 0xCC (11001100), 0xF0 (11110000)
	data := []byte{0xAA, 0xCC, 0xF0}
	br := NewBitReader(data, false)

	// Read 12 bits
	// Byte 0 (0xAA = 10101010): Read 8 bits -> 01010101 (reversed)
	// Byte 1 (0xCC = 11001100): Read 4 bits -> 0011 (reversed LSBs of 1100)
	// Concatenated LSB-first: (0011)(01010101) = 001101010101
	bf, err := br.ReadBits(12)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b001101010101), bf.AsUint())
	assert.Equal(t, 1, br.bytePos)
	assert.Equal(t, 4, br.bitPos)

	// Read remaining 4 bits of current byte (0xCC = 11001100, remaining LSBs after 4 are 1100) -> 0011
	bf, err = br.ReadBits(4)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b0011), bf.AsUint())
	assert.Equal(t, 2, br.bytePos)
	assert.Equal(t, 0, br.bitPos)

	// Read next 8 bits (0xF0 = 11110000) -> 00001111
	bf, err = br.ReadBits(8)
	require.NoError(t, err)
	assert.Equal(t, uint64(0b00001111), bf.AsUint())
	assert.Equal(t, 3, br.bytePos)
	assert.Equal(t, 0, br.bitPos)
}

func TestBitReader_ReadBits_ErrorCases(t *testing.T) {
	br := NewBitReader([]byte{0x01}, true)
	_, err := br.ReadBits(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bit count must be between 1 and 64")
	_, err = br.ReadBits(65)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bit count must be between 1 and 64")

	_, err = br.ReadBits(8) // Read 8 bits, okay
	require.NoError(t, err)
	_, err = br.ReadBits(1) // Read 1 more bit, should be EOF
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
}

// Basic check for BitFieldTypeOptions - more detailed testing of CEL functions in internal/cel/cel_test.go
func TestBitFieldTypeOptions_FunctionRegistration(t *testing.T) {
	opts := BitFieldTypeOptions() // This is a cel.EnvOption
	assert.NotNil(t, opts)
	// Further testing of actual CEL function behavior should be in internal/cel/cel_test.go
}
