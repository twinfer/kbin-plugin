package kaitaicel

import (
	"reflect"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// ParsedDataWrapper wraps ParsedData for CEL compatibility and provides _sizeof attribute
type ParsedDataWrapper struct {
	data     *ParsedData
	children map[string]any // Converted children for CEL
}

// ParsedData represents parsed data with size tracking (imported from parser package)
type ParsedData struct {
	Value    any
	Children map[string]*ParsedData
	Type     string
	IsArray  bool
	Size     int64
}

// NewParsedDataWrapper creates a new wrapper for CEL compatibility
func NewParsedDataWrapper(data *ParsedData) *ParsedDataWrapper {
	// Convert children recursively
	children := make(map[string]any)
	for k, v := range data.Children {
		children[k] = ConvertForCELActivation(v)
	}

	// Add _sizeof attribute
	children["_sizeof"] = data.Size

	return &ParsedDataWrapper{
		data:     data,
		children: children,
	}
}

// --- CEL ref.Val interface implementation ---

// ConvertToNative implements ref.Val interface
func (pdw *ParsedDataWrapper) ConvertToNative(typeDesc reflect.Type) (any, error) {
	// Return the children map for native conversion
	return pdw.children, nil
}

// ConvertToType implements ref.Val interface
func (pdw *ParsedDataWrapper) ConvertToType(celType ref.Type) ref.Val {
	switch celType {
	case types.MapType:
		// Convert map[string]any to map[ref.Val]ref.Val
		refMap := make(map[ref.Val]ref.Val)
		for k, v := range pdw.children {
			refMap[types.String(k)] = types.DefaultTypeAdapter.NativeToValue(v)
		}
		return types.NewRefValMap(types.DefaultTypeAdapter, refMap)
	case types.TypeType:
		return types.MapType
	default:
		return types.NewErr("type conversion error from ParsedDataWrapper to %s", celType)
	}
}

// Equal implements ref.Val interface
func (pdw *ParsedDataWrapper) Equal(other ref.Val) ref.Val {
	if otherPdw, ok := other.(*ParsedDataWrapper); ok {
		return types.Bool(pdw.data == otherPdw.data)
	}
	return types.Bool(false)
}

// Type implements ref.Val interface
func (pdw *ParsedDataWrapper) Type() ref.Type {
	return types.MapType
}

// Value implements ref.Val interface
func (pdw *ParsedDataWrapper) Value() any {
	return pdw.children
}

// GetAttribute provides map-like access to fields and attributes (Go interface)
func (pdw *ParsedDataWrapper) GetAttribute(key string) (any, bool) {
	val, ok := pdw.children[key]
	return val, ok
}

// GetUnderlyingData returns the underlying ParsedData for serialization purposes
func (pdw *ParsedDataWrapper) GetUnderlyingData() *ParsedData {
	return pdw.data
}

// --- CEL traits.Indexer interface implementation ---

// Get implements traits.Indexer for CEL attribute access
func (pdw *ParsedDataWrapper) Get(index ref.Val) ref.Val {
	indexStr, ok := index.(types.String)
	if !ok {
		return types.NewErr("string index required")
	}

	key := string(indexStr)
	if val, ok := pdw.children[key]; ok {
		return types.DefaultTypeAdapter.NativeToValue(val)
	}

	return types.NewErr("no such key: %s", key)
}
