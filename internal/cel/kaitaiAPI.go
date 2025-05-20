package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// kaitaiApiFunctions returns CEL function declarations that directly use Kaitai API.
func kaitaiApiFunctions() cel.EnvOption {
	return cel.Lib(&kaitaiApiLib{})
}

type kaitaiApiLib struct{}

func (*kaitaiApiLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// BytesStripRight - removes padding bytes from the end
		cel.Function("bytesStripRight",
			cel.Overload("bytesstripright_bytes_int", []*cel.Type{cel.BytesType, cel.IntType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					padInt, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to bytesStripRight")
					}
					result := kaitai.BytesStripRight([]byte(data), byte(padInt))
					return types.Bytes(result)
				}),
			),
		),

		// BytesTerminate - extracts bytes until terminator byte
		cel.Function("bytesTerminate",
			cel.Overload("bytesterminate_bytes_int_bool", []*cel.Type{cel.BytesType, cel.IntType, cel.BoolType}, cel.BytesType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 3 {
						return types.NewErr("bytesTerminate requires 3 arguments")
					}

					data, ok1 := args[0].(types.Bytes)
					termInt, ok2 := args[1].(types.Int)
					includeTerm, ok3 := args[2].(types.Bool)

					if !ok1 || !ok2 || !ok3 {
						return types.NewErr("invalid arguments to bytesTerminate")
					}

					result := kaitai.BytesTerminate([]byte(data), byte(termInt), bool(includeTerm))
					return types.Bytes(result)
				}),
			),
		),

		// BytesToStr - converts bytes to string with encoding
		cel.Function("bytesToStr",
			cel.Overload("bytestostr_bytes", []*cel.Type{cel.BytesType}, cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					data, ok := val.(types.Bytes)
					if !ok {
						return types.NewErr("expected bytes for bytesToStr")
					}
					str, err := kaitai.BytesToStr([]byte(data), nil) // Default decoder
					if err != nil {
						return types.NewErr("failed to convert bytes to string: %v", err)
					}
					return types.String(str)
				}),
			),
			// Add decoder parameter variant later if needed
		),

		// ProcessRotateLeft - rotates bytes left
		cel.Function("processRotateLeft",
			cel.Overload("processrotateleft_bytes_int", []*cel.Type{cel.BytesType, cel.IntType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					amount, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to processRotateLeft")
					}
					result := kaitai.ProcessRotateLeft([]byte(data), int(amount))
					return types.Bytes(result)
				}),
			),
		),

		// ProcessRotateRight - rotates bytes right
		cel.Function("processRotateRight",
			cel.Overload("processrotateright_bytes_int", []*cel.Type{cel.BytesType, cel.IntType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					amount, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to processRotateRight")
					}
					result := kaitai.ProcessRotateRight([]byte(data), int(amount))
					return types.Bytes(result)
				}),
			),
		),

		// ProcessXOR - XORs bytes with key
		cel.Function("processXOR",
			cel.Overload("processxor_bytes_bytes", []*cel.Type{cel.BytesType, cel.BytesType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					keyBytes, ok2 := rhs.(types.Bytes)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to processXOR")
					}
					if len(keyBytes) == 0 {
						return types.NewErr("key bytes cannot be empty")
					}
					result := kaitai.ProcessXOR([]byte(data), []byte(keyBytes))
					return types.Bytes(result)
				}),
			),
			cel.Overload("processxor_bytes_int", []*cel.Type{cel.BytesType, cel.IntType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					keyInt, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to processXOR")
					}
					key := []byte{byte(keyInt)}
					result := kaitai.ProcessXOR([]byte(data), key)
					return types.Bytes(result)
				}),
			),
		),

		// ProcessZlib - decompresses zlib data
		cel.Function("processZlib",
			cel.Overload("processzlib_bytes", []*cel.Type{cel.BytesType}, cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					data, ok := val.(types.Bytes)
					if !ok {
						return types.NewErr("expected bytes for processZlib")
					}
					result, err := kaitai.ProcessZlib([]byte(data))
					if err != nil {
						return types.NewErr("zlib decompression error: %v", err)
					}
					return types.Bytes(result)
				}),
			),
		),

		// StringReverse - reverses a string
		cel.Function("stringReverse",
			cel.Overload("stringreverse_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					str, ok := val.(types.String)
					if !ok {
						return types.NewErr("expected string type for stringReverse")
					}
					result := kaitai.StringReverse(string(str))
					return types.String(result)
				}),
			),
		),
	}
}

func (*kaitaiApiLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
