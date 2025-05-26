package kaitaicel

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- EnumRegistry Tests ---

func TestEnumRegistry(t *testing.T) {
	registry := NewEnumRegistry()
	require.NotNil(t, registry)
	assert.Empty(t, registry.enums)

	mapping1 := map[int64]string{0: "Zero", 1: "One"}
	registry.Register("MyEnum1", mapping1)

	retrievedMap1, ok := registry.Get("MyEnum1")
	assert.True(t, ok)
	assert.Equal(t, mapping1, retrievedMap1)

	_, ok = registry.Get("UnknownEnum")
	assert.False(t, ok)
}

// --- KaitaiEnum Tests ---

func getTestEnumMapping() map[int64]string {
	return map[int64]string{
		10: "Ten",
		20: "Twenty",
		30: "Thirty",
	}
}

const testEnumName = "TestNumericEnum"

func TestKaitaiEnum_NewKaitaiEnum(t *testing.T) {
	mapping := getTestEnumMapping()

	t.Run("ValidValue", func(t *testing.T) {
		ke, err := NewKaitaiEnum(20, testEnumName, mapping)
		require.NoError(t, err)
		assert.Equal(t, int64(20), ke.value)
		assert.Equal(t, "Twenty", ke.name)
		assert.Equal(t, testEnumName, ke.enumName)
		assert.True(t, ke.IsValid())
	})

	t.Run("InvalidValue", func(t *testing.T) {
		ke, err := NewKaitaiEnum(40, testEnumName, mapping) // 40 is not in mapping
		require.NoError(t, err)                             // Constructor itself doesn't error for unknown values
		assert.Equal(t, int64(40), ke.value)
		assert.Equal(t, "<TestNumericEnum::40>", ke.name) // Default name for unknown value
		assert.Equal(t, testEnumName, ke.enumName)
		assert.False(t, ke.IsValid())
	})
}

func TestKaitaiEnum_NewKaitaiEnumByName(t *testing.T) {
	mapping := getTestEnumMapping()

	t.Run("ValidName", func(t *testing.T) {
		ke, err := NewKaitaiEnumByName("Ten", testEnumName, mapping)
		require.NoError(t, err)
		assert.Equal(t, int64(10), ke.value)
		assert.Equal(t, "Ten", ke.name)
		assert.Equal(t, testEnumName, ke.enumName)
		assert.True(t, ke.IsValid())
	})

	t.Run("InvalidName", func(t *testing.T) {
		_, err := NewKaitaiEnumByName("Forty", testEnumName, mapping)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid enum value 'Forty' for TestNumericEnum")
	})
}

func TestKaitaiEnum_Methods(t *testing.T) {
	mapping := getTestEnumMapping()
	keValid, _ := NewKaitaiEnum(10, testEnumName, mapping)
	keInvalid, _ := NewKaitaiEnum(15, testEnumName, mapping)

	assert.Equal(t, int64(10), keValid.IntValue())
	assert.Equal(t, "Ten", keValid.Name())
	assert.True(t, keValid.IsValid())

	assert.Equal(t, int64(15), keInvalid.IntValue())
	assert.Equal(t, "<TestNumericEnum::15>", keInvalid.Name())
	assert.False(t, keInvalid.IsValid())

	assert.Equal(t, "enum:TestNumericEnum", keValid.KaitaiTypeName())
	assert.Nil(t, keValid.RawBytes())
	assert.Nil(t, keValid.Serialize())
	assert.Equal(t, KaitaiEnumType, keValid.Type())
	assert.Same(t, keValid, keValid.Value())
}

func TestKaitaiEnum_Equal(t *testing.T) {
	mapping := getTestEnumMapping()
	enumName2 := "OtherEnum"
	mapping2 := map[int64]string{10: "X"}

	ke1_10a, _ := NewKaitaiEnum(10, testEnumName, mapping)
	ke1_10b, _ := NewKaitaiEnum(10, testEnumName, mapping)
	ke1_20, _ := NewKaitaiEnum(20, testEnumName, mapping)
	ke2_10, _ := NewKaitaiEnum(10, enumName2, mapping2) // Same value, different enum type

	celInt10 := types.Int(10)
	celInt20 := types.Int(20)
	celStringTen := types.String("Ten")
	celStringTwenty := types.String("Twenty")

	assert.True(t, bool(ke1_10a.Equal(ke1_10b).(types.Bool)), "Same enum, same value")
	assert.False(t, bool(ke1_10a.Equal(ke1_20).(types.Bool)), "Same enum, different value")
	assert.False(t, bool(ke1_10a.Equal(ke2_10).(types.Bool)), "Different enum, same value")

	assert.True(t, bool(ke1_10a.Equal(celInt10).(types.Bool)), "Enum vs CEL Int (same value)")
	assert.False(t, bool(ke1_10a.Equal(celInt20).(types.Bool)), "Enum vs CEL Int (diff value)")

	assert.True(t, bool(ke1_10a.Equal(celStringTen).(types.Bool)), "Enum vs CEL String (same name)")
	assert.False(t, bool(ke1_10a.Equal(celStringTwenty).(types.Bool)), "Enum vs CEL String (diff name)")
	assert.False(t, bool(ke1_20.Equal(celStringTen).(types.Bool)), "Enum (val 20, name Twenty) vs CEL String (Ten)")

	assert.False(t, bool(ke1_10a.Equal(types.Double(10.0)).(types.Bool)), "Enum vs unrelated type")
}

func TestKaitaiEnum_Compare(t *testing.T) {
	mapping := getTestEnumMapping()
	enumName2 := "OtherEnum"
	mapping2 := map[int64]string{5: "Five"}

	ke1_10, _ := NewKaitaiEnum(10, testEnumName, mapping)
	ke1_20, _ := NewKaitaiEnum(20, testEnumName, mapping)
	ke2_5, _ := NewKaitaiEnum(5, enumName2, mapping2)

	celInt5 := types.Int(5)
	celInt10 := types.Int(10)
	celInt15 := types.Int(15)

	assert.Equal(t, types.IntZero, ke1_10.Compare(ke1_10))
	assert.Equal(t, types.IntNegOne, ke1_10.Compare(ke1_20)) // 10 < 20
	assert.Equal(t, types.IntOne, ke1_20.Compare(ke1_10))    // 20 > 10

	assert.Equal(t, types.IntZero, ke1_10.Compare(celInt10))
	assert.Equal(t, types.IntOne, ke1_10.Compare(celInt5))     // 10 > 5
	assert.Equal(t, types.IntNegOne, ke1_10.Compare(celInt15)) // 10 < 15

	// Comparison with different enum types
	assert.True(t, types.IsError(ke1_10.Compare(ke2_5)), "Comparing different enum types should error")
	// Comparison with non-int/non-KaitaiEnum
	assert.True(t, types.IsError(ke1_10.Compare(types.String("10"))), "Comparing enum with string should error")
}

func TestKaitaiEnum_ConvertToNative(t *testing.T) {
	mapping := getTestEnumMapping()
	ke, _ := NewKaitaiEnum(20, testEnumName, mapping)

	valInt, err := ke.ConvertToNative(reflect.TypeOf(int64(0)))
	require.NoError(t, err)
	assert.Equal(t, int64(20), valInt)

	valStr, err := ke.ConvertToNative(reflect.TypeOf(""))
	require.NoError(t, err)
	assert.Equal(t, "Twenty", valStr)

	valMap, err := ke.ConvertToNative(reflect.TypeOf(map[string]interface{}{}))
	require.NoError(t, err)
	expectedMap := map[string]interface{}{"value": int64(20), "name": "Twenty", "enum": testEnumName}
	assert.Equal(t, expectedMap, valMap)

	_, err = ke.ConvertToNative(reflect.TypeOf(false)) // Unsupported type
	assert.Error(t, err)
}

func TestKaitaiEnum_ConvertToType(t *testing.T) {
	mapping := getTestEnumMapping()
	ke, _ := NewKaitaiEnum(20, testEnumName, mapping)

	assert.Equal(t, types.Int(20), ke.ConvertToType(types.IntType))
	assert.Equal(t, types.String("Twenty"), ke.ConvertToType(types.StringType))

	// Error for unsupported CEL type conversion
	assert.True(t, types.IsError(ke.ConvertToType(types.BoolType)))
}

// Basic check for EnumTypeOptions - more detailed testing of CEL functions in internal/cel/cel_test.go
func TestEnumTypeOptions_FunctionRegistration(t *testing.T) {
	registry := NewEnumRegistry()
	opts := EnumTypeOptions(registry) // This is a cel.EnvOption

	// This is a very basic check. To truly test, you'd need to create a CEL env
	// with these options and try to compile/evaluate expressions using these functions.
	// For now, just check if it returns a non-nil option.
	assert.NotNil(t, opts)

	// Example: Check if the 'enum' function is declared (structure might vary)
	// This is more of an introspection and might be too brittle.
	// A better test would be in a cel_test.go file evaluating `enum(10, "MyEnum")`.
	// For this file, focusing on the Go type methods is more direct.
}
