package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// NewEnvironment creates a CEL environment with all Kaitai-specific functions.
func NewEnvironment() (*cel.Env, error) {
	// Create base options
	opts := []cel.EnvOption{
		// Add custom type adapter to handle conversion between Go and CEL types
		cel.CustomTypeAdapter(NewKaitaiTypeAdapter()),

		// Add custom type providers for stream access
		cel.Variable("_io", cel.DynType),

		// Enable standard CEL library functions
		cel.StdLib(),

		// Register Kaitai-specific functions organized by category
		StringFunctions(),
		TypeConversionFunctions(),
		ArrayFunctions(),
		SizeFunctions(),           // Re-added for _sizeof support
		ByteComparisonFunctions(), // New functions for byte/int array comparisons
		BitwiseFunctions(),
		MathFunctions(),
		KaitaiApiFunctions(), // New function category using direct Kaitai API calls
		//processFunctions(),
		ErrorHandlingFunctions(),
		SafeArithmeticFunctions(),
		StreamOperations(),  // New function category for stream operations
		WriterOperations(),  // New functions for serialization
		EncodingFunctions(), // New functions for string encoding
	}

	// Add kaitaicel bitfield type options
	opts = append(opts, kaitaicel.BitFieldTypeOptions()...)

	// Create the CEL environment with all options
	env, err := cel.NewEnv(opts...)

	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return env, nil
}

func ErrorHandlingFunctions() cel.EnvOption {
	return cel.Lib(&errorLib{})
}

type errorLib struct{}

func (*errorLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("error",
			cel.Overload("error_string", []*cel.Type{cel.StringType}, cel.AnyType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					msg, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string for error message")
					}
					return types.NewErr("%s", msg)
				}),
			),
		),
		cel.Function("isError",
			cel.Overload("iserror_any", []*cel.Type{cel.AnyType}, cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					return types.Bool(types.IsError(val))
				}),
			),
		),
	}
}

func (*errorLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// KaitaiTypeAdapter extends the default type adapter to handle Go's smaller numeric types
type KaitaiTypeAdapter struct {
	types.Adapter
}

// NewKaitaiTypeAdapter creates a new type adapter that handles Kaitai-specific type conversions
func NewKaitaiTypeAdapter() *KaitaiTypeAdapter {
	return &KaitaiTypeAdapter{
		Adapter: types.DefaultTypeAdapter,
	}
}

// NativeToValue converts Go native types to CEL values, handling smaller integer types
func (k *KaitaiTypeAdapter) NativeToValue(value any) ref.Val {
	switch v := value.(type) {
	case int8:
		return types.Int(v)
	case int16:
		return types.Int(v)
	case int32:
		return types.Int(v)
	case uint8:
		return types.Int(v)
	case uint16:
		return types.Int(v)
	case uint32:
		return types.Uint(v)
	case float32:
		return types.Double(v)
	default:
		// Fall back to the default adapter for other types
		return k.Adapter.NativeToValue(value)
	}
}

// Functions for interacting with the CEL types and protobuf
func ValueToCEL(val any) ref.Val {
	return types.DefaultTypeAdapter.NativeToValue(val)
}

func CELToValue(val ref.Val) any {
	if val == nil {
		return nil
	}
	return val.Value()
}

// ValueToRefValue converts a Go value to a CEL ref.Val using the default type adapter
func ValueToRefValue(val any) ref.Val {
	return ValueToCEL(val)
}

// RefValueToValue converts a CEL ref.Val back to a Go value
func RefValueToValue(val ref.Val) (any, error) {
	if val == nil {
		return nil, nil
	}

	if types.IsError(val) {
		return nil, fmt.Errorf("CEL error: %v", val)
	}

	if types.IsUnknown(val) {
		return nil, fmt.Errorf("unknown CEL value")
	}

	return val.Value(), nil
}

// ValueAsProto converts a CEL value to a protobuf value
func ValueAsProto(val ref.Val) (*exprpb.Value, error) {
	if val == nil {
		return nil, nil
	}

	return cel.ValueAsAlphaProto(val)
}
