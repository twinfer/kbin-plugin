package kaitaicel

import (
	"reflect"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// KaitaiStream wraps kaitai.Stream for CEL compatibility and provides I/O operations
type KaitaiStream struct {
	stream *kaitai.Stream
}

// NewKaitaiStream creates a new KaitaiStream wrapper from a native kaitai.Stream
func NewKaitaiStream(stream *kaitai.Stream) *KaitaiStream {
	return &KaitaiStream{stream: stream}
}

// GetNativeStream returns the underlying kaitai.Stream
func (ks *KaitaiStream) GetNativeStream() *kaitai.Stream {
	return ks.stream
}

// --- CEL ref.Val interface implementation ---

// ConvertToNative implements ref.Val interface
func (ks *KaitaiStream) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return ks, nil
}

// ConvertToType implements ref.Val interface
func (ks *KaitaiStream) ConvertToType(celType ref.Type) ref.Val {
	switch celType {
	case types.TypeType:
		return types.MapType
	default:
		return types.NewErr("type conversion error from KaitaiStream to %s", celType)
	}
}

// Equal implements ref.Val interface
func (ks *KaitaiStream) Equal(other ref.Val) ref.Val {
	if otherStream, ok := other.(*KaitaiStream); ok {
		return types.Bool(ks.stream == otherStream.stream)
	}
	return types.Bool(false)
}

// Type implements ref.Val interface
func (ks *KaitaiStream) Type() ref.Type {
	return types.MapType
}

// Value implements ref.Val interface
func (ks *KaitaiStream) Value() any {
	return ks
}

// --- Stream property access for CEL ---

// Get provides map-like access to stream properties for CEL expressions
// Supports: "pos", "size"
func (ks *KaitaiStream) Get(key any) (any, bool) {
	keyStr, ok := key.(string)
	if !ok {
		return nil, false
	}

	switch keyStr {
	case "pos":
		pos, err := ks.stream.Pos()
		if err != nil {
			return nil, false
		}
		return pos, true
	case "size":
		size, err := ks.stream.Size()
		if err != nil {
			return nil, false
		}
		return size, true
	default:
		return nil, false
	}
}

// --- High-level I/O operations ---

// Pos returns the current stream position
func (ks *KaitaiStream) Pos() (int64, error) {
	return ks.stream.Pos()
}

// Size returns the stream size
func (ks *KaitaiStream) Size() (int64, error) {
	return ks.stream.Size()
}

// EOF checks if stream is at end of file
func (ks *KaitaiStream) EOF() (bool, error) {
	return ks.stream.EOF()
}

// Seek sets the stream position
func (ks *KaitaiStream) Seek(offset int64, whence int) (int64, error) {
	return ks.stream.Seek(offset, whence)
}
