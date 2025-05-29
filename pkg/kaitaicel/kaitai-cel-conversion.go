package kaitaicel

import (
	"github.com/google/cel-go/common/types"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// ConvertForCELActivation converts values to be compatible with CEL activation
// This abstracts CEL-specific type conversions from the parser
func ConvertForCELActivation(value any) any {
	if value == nil {
		return nil
	}

	// Handle kaitai.Stream specifically - wrap in KaitaiStream for CEL compatibility
	if stream, ok := value.(*kaitai.Stream); ok {
		return NewKaitaiStream(stream)
	}

	// Handle ParsedData - wrap to provide _sizeof attribute
	if pd, ok := value.(*ParsedData); ok {
		// Special case: if this is _io metadata with a map Value, use the Value directly
		if pd.Type == "io_metadata" && pd.Value != nil {
			if mapVal, isMap := pd.Value.(map[string]any); isMap {
				return types.DefaultTypeAdapter.NativeToValue(mapVal)
			}
		}
		return NewParsedDataWrapper(pd)
	}

	// Handle kaitaicel types - convert to standard CEL-compatible types
	if kaitaiType, ok := value.(KaitaiType); ok {
		underlying := kaitaiType.Value()
		// Convert byte arrays to CEL bytes type for proper comparison operator support
		if byteSlice, isByteSlice := underlying.([]byte); isByteSlice {
			return types.Bytes(byteSlice)
		}
		return underlying // Return the underlying value for CEL compatibility
	}

	// Return primitive types as-is for CEL compatibility
	return value
}
