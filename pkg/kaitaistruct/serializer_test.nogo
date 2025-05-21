package kaitaistruct

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerialize_SimpleU1(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "simple", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "foo", Type: "u1"},
		},
	}
	serializer := NewKaitaiSerializer(schema)
	data := map[string]any{"foo": uint8(42)}
	bin, err := serializer.Serialize(data)
	require.NoError(t, err)
	assert.Equal(t, []byte{42}, bin)
}

func TestSerialize_String_UTF8(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "strtest", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "s", Type: "str"},
		},
	}
	serializer := NewKaitaiSerializer(schema)
	data := map[string]any{"s": "abc"}
	bin, err := serializer.Serialize(data)
	require.NoError(t, err)
	assert.Equal(t, []byte("abc"), bin)
}

func TestSerialize_String_UTF16LE(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "strtest", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "s", Type: "str", Encoding: "utf-16le"},
		},
	}
	serializer := NewKaitaiSerializer(schema)
	data := map[string]any{"s": "A"}
	bin, err := serializer.Serialize(data)
	require.NoError(t, err)
	// UTF-16LE for "A" is 0x41 0x00
	assert.Equal(t, []byte{0x41, 0x00}, bin)
}

func TestSerialize_RepeatedU1(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "repeated", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "nums", Type: "u1", Repeat: "expr", RepeatExpr: "3"},
		},
	}
	serializer := NewKaitaiSerializer(schema)
	data := map[string]any{"nums": []any{uint8(1), uint8(2), uint8(3)}}
	bin, err := serializer.Serialize(data)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, bin)
}

func TestSerialize_ConditionalField(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "cond", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "flag", Type: "u1"},
			{ID: "val", Type: "u1", IfExpr: "flag == 1"},
		},
	}
	serializer := NewKaitaiSerializer(schema)
	data := map[string]any{"flag": uint8(1), "val": uint8(99)}
	bin, err := serializer.Serialize(data)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 99}, bin)

	data2 := map[string]any{"flag": uint8(0)}
	bin2, err := serializer.Serialize(data2)
	require.NoError(t, err)
	assert.Equal(t, []byte{0}, bin2)
}

func TestSerialize_CustomType(t *testing.T) {
	schema := &KaitaiSchema{
		Meta: Meta{ID: "main", Endian: "le"},
		Seq: []SequenceItem{
			{ID: "foo", Type: "mytype"},
		},
		Types: map[string]Type{
			"mytype": {
				Seq: []SequenceItem{
					{ID: "bar", Type: "u1"},
				},
			},
		},
	}
	serializer := NewKaitaiSerializer(schema)
	data := map[string]any{"foo": map[string]any{"bar": uint8(7)}}
	bin, err := serializer.Serialize(data)
	require.NoError(t, err)
	assert.Equal(t, []byte{7}, bin)
}
