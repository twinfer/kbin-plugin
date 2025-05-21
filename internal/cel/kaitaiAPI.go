package cel

import (
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
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
				// Default to UTF-8 if no encoding is specified
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					data, ok := val.(types.Bytes)
					if !ok {
						return types.NewErr("expected bytes for bytesToStr")
					}
					// Use UTF-8 decoder
					decoder := unicode.UTF8.NewDecoder()
					src := []byte(data)
					dst := make([]byte, len(src)*3) // UTF-8 can be up to 3x the size
					nDst, _, err := decoder.Transform(dst, src, true)
					str := string(dst[:nDst])
					if err != nil {
						return types.NewErr("failed to convert bytes to string: %v", err)
					}
					return types.String(str)
				}),
			),
			cel.Overload("bytestostr_bytes_string", []*cel.Type{cel.BytesType, cel.StringType}, cel.StringType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					encodingName, ok2 := rhs.(types.String)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to bytesToStr(bytes, string)")
					}

					// Get the decoder based on name
					var decoder *encoding.Decoder
					switch strings.ToUpper(string(encodingName)) {
					case "ASCII", "UTF-8", "UTF8":
						decoder = unicode.UTF8.NewDecoder() // ASCII is a subset of UTF-8
					case "UTF-16LE", "UTF16LE":
						decoder = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
					case "UTF-16BE", "UTF16BE":
						decoder = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
					default:
						// Fallback to Kaitai's lookup if needed, or return error.
						// For now, return error for unsupported explicit encodings
						return types.NewErr("unsupported encoding: %s", encodingName)
					}
					// Transform the bytes
					src := []byte(data)
					dst := make([]byte, len(src)*3) // Allow for expansion
					nDst, _, err := decoder.Transform(dst, src, true)
					str := string(dst[:nDst])
					if err != nil {
						return types.NewErr("failed to decode string with encoding '%s': %v", encodingName, err)
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
			cel.Overload("processxor_bytes_uint", []*cel.Type{cel.BytesType, cel.UintType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					keyUint, ok2 := rhs.(types.Uint)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to processXOR(bytes, uint)")
					}
					key := []byte{byte(keyUint)} // Convert uint to byte for the key
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
	}
}

func (*kaitaiApiLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
