package cel

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// writerOperations returns CEL function declarations for Kaitai writer operations
func writerOperations() cel.EnvOption {
	return cel.Lib(&writerLib{})
}

type writerLib struct{}

func (*writerLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Writer position
		// Writer position - use io.Seeker interface
		cel.Function("writerPos",
			cel.Overload("writerpos_writer", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if writer, ok := val.Value().(*kaitai.Writer); ok {
						// Try to use underlying io.WriteSeeker if available
						if seeker, ok := writer.Writer.(io.Seeker); ok {
							pos, err := seeker.Seek(0, io.SeekCurrent)
							if err != nil {
								return types.NewErr("failed to get position: %v", err)
							}
							return types.Int(pos)
						}
						return types.NewErr("writer doesn't support position tracking")
					}
					return types.NewErr("expected Writer for writerPos function")
				}),
			),
		),

		// Basic writer functions
		cel.Function("writeBytes",
			cel.Overload("writebytes_writer_bytes", []*cel.Type{cel.AnyType, cel.BytesType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					data, ok2 := rhs.(types.Bytes)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeBytes")
					}
					err := writer.WriteBytes([]byte(data))
					if err != nil {
						return types.NewErr("failed to write bytes: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU1",
			cel.Overload("writeu1_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU1")
					}
					err := writer.WriteU1(byte(val))
					if err != nil {
						return types.NewErr("failed to write u1: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU2le",
			cel.Overload("writeu2le_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU2le")
					}
					err := writer.WriteU2le(uint16(val))
					if err != nil {
						return types.NewErr("failed to write u2le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU4le",
			cel.Overload("writeu4le_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU4le")
					}
					err := writer.WriteU4le(uint32(val))
					if err != nil {
						return types.NewErr("failed to write u4le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU8le",
			cel.Overload("writeu8le_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU8le")
					}
					err := writer.WriteU8le(uint64(val))
					if err != nil {
						return types.NewErr("failed to write u8le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU2be",
			cel.Overload("writeu2be_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU2be")
					}
					err := writer.WriteU2be(uint16(val))
					if err != nil {
						return types.NewErr("failed to write u2be: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU4be",
			cel.Overload("writeu4be_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU4be")
					}
					err := writer.WriteU4be(uint32(val))
					if err != nil {
						return types.NewErr("failed to write u4be: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeU8be",
			cel.Overload("writeu8be_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeU8be")
					}
					err := writer.WriteU8be(uint64(val))
					if err != nil {
						return types.NewErr("failed to write u8be: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		// Signed integer writer functions
		cel.Function("writeS1",
			cel.Overload("writes1_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS1")
					}
					err := writer.WriteS1(int8(val))
					if err != nil {
						return types.NewErr("failed to write s1: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeS2le",
			cel.Overload("writes2le_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS2le")
					}
					err := writer.WriteS2le(int16(val))
					if err != nil {
						return types.NewErr("failed to write s2le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeS4le",
			cel.Overload("writes4le_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS4le")
					}
					err := writer.WriteS4le(int32(val))
					if err != nil {
						return types.NewErr("failed to write s4le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeS8le",
			cel.Overload("writes8le_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS8le")
					}
					err := writer.WriteS8le(int64(val))
					if err != nil {
						return types.NewErr("failed to write s8le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		// Float writer functions
		cel.Function("writeF4le",
			cel.Overload("writef4le_writer_double", []*cel.Type{cel.AnyType, cel.DoubleType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Double)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeF4le")
					}
					err := writer.WriteF4le(float32(val))
					if err != nil {
						return types.NewErr("failed to write f4le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeF8le",
			cel.Overload("writef8le_writer_double", []*cel.Type{cel.AnyType, cel.DoubleType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Double)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeF8le")
					}
					err := writer.WriteF8le(float64(val))
					if err != nil {
						return types.NewErr("failed to write f8le: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		// Signed integer BE writer functions
		cel.Function("writeS2be",
			cel.Overload("writes2be_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS2be")
					}
					err := writer.WriteS2be(int16(val))
					if err != nil {
						return types.NewErr("failed to write s2be: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeS4be",
			cel.Overload("writes4be_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS4be")
					}
					err := writer.WriteS4be(int32(val))
					if err != nil {
						return types.NewErr("failed to write s4be: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		cel.Function("writeS8be",
			cel.Overload("writes8be_writer_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					writer, ok1 := lhs.Value().(*kaitai.Writer)
					val, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to writeS8be")
					}
					err := writer.WriteS8be(int64(val))
					if err != nil {
						return types.NewErr("failed to write s8be: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),
		// Unified write function
		cel.Function("write",
			cel.Overload("write_writer_string_any", []*cel.Type{cel.AnyType, cel.StringType, cel.AnyType}, cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 3 {
						return types.NewErr("write requires 3 arguments")
					}

					writer, ok := args[0].Value().(*kaitai.Writer)
					if !ok {
						return types.NewErr("expected Writer for write function")
					}

					typeStr, ok := args[1].(types.String)
					if !ok {
						return types.NewErr("expected string type name")
					}

					val := args[2]

					// Call appropriate writer function based on type
					switch string(typeStr) {
					case "u1":
						intVal, err := toInt(val)
						if err != nil {
							return types.NewErr("conversion error: %v", err)
						}
						if err := writer.WriteU1(byte(intVal)); err != nil {
							return types.NewErr("write error: %v", err)
						}
					case "u2le":
						intVal, err := toInt(val)
						if err != nil {
							return types.NewErr("conversion error: %v", err)
						}
						if err := writer.WriteU2le(uint16(intVal)); err != nil {
							return types.NewErr("write error: %v", err)
						}
					case "u4le":
						intVal, err := toInt(val)
						if err != nil {
							return types.NewErr("conversion error: %v", err)
						}
						if err := writer.WriteU4le(uint32(intVal)); err != nil {
							return types.NewErr("write error: %v", err)
						}
					// Add more cases for other types...
					default:
						return types.NewErr("unknown type: %s", typeStr)
					}

					return types.Bool(true)
				}),
			),
		),
	}
}

func (*writerLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// encodingFunctions returns CEL function declarations for string encoding/decoding
func encodingFunctions() cel.EnvOption {
	return cel.Lib(&encodingLib{})
}

type encodingLib struct{}

func (*encodingLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Encode string to bytes with specific encoding
		cel.Function("encodeString",
			cel.Overload("encodestring_string_string", []*cel.Type{cel.StringType, cel.StringType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					str, ok1 := lhs.(types.String)
					encodingName, ok2 := rhs.(types.String)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to encodeString")
					}

					var encoded []byte
					var err error

					// Handle different encodings
					switch strings.ToUpper(string(encodingName)) {
					case "ASCII", "UTF-8", "UTF8":
						encoded = []byte(string(str))
					case "UTF-16LE", "UTF16LE":
						encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
						encoded, err = encoder.Bytes([]byte(string(str)))
						if err != nil {
							return types.NewErr("failed to encode to UTF-16LE: %v", err)
						}
					case "UTF-16BE", "UTF16BE":
						encoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
						encoded, err = encoder.Bytes([]byte(string(str)))
						if err != nil {
							return types.NewErr("failed to encode to UTF-16BE: %v", err)
						}
					default:
						return types.NewErr("unsupported encoding: %s", encodingName)
					}

					return types.Bytes(encoded)
				}),
			),
		),

		// Decode bytes to string with specific encoding
		cel.Function("decodeString",
			cel.Overload("decodestring_bytes_string", []*cel.Type{cel.BytesType, cel.StringType}, cel.StringType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					data, ok1 := lhs.(types.Bytes)
					encodingName, ok2 := rhs.(types.String)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to decodeString")
					}

					var decoded string
					// var err error

					// Handle different encodings
					switch strings.ToUpper(string(encodingName)) {
					case "ASCII", "UTF-8", "UTF8":
						decoded = string(data)
					case "UTF-16LE", "UTF16LE":
						decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
						utf8Str, _, err := transform.Bytes(decoder, []byte(data))
						if err != nil {
							return types.NewErr("failed to decode UTF-16LE: %v", err)
						}
						decoded = string(utf8Str)
					case "UTF-16BE", "UTF16BE":
						decoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
						utf8Str, _, err := transform.Bytes(decoder, []byte(data))
						if err != nil {
							return types.NewErr("failed to decode UTF-16BE: %v", err)
						}
						decoded = string(utf8Str)
					default:
						return types.NewErr("unsupported encoding: %s", encodingName)
					}

					return types.String(decoded)
				}),
			),
		),

		// Create a new writer
		cel.Function("newWriter",
			cel.Overload("newwriter", []*cel.Type{}, cel.AnyType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					buf := bytes.NewBuffer(nil)
					writer := kaitai.NewWriter(buf)
					return types.DefaultTypeAdapter.NativeToValue(writer)
				}),
			),
		),

		// Get buffer from writer
		cel.Function("writerBuffer",
			cel.Overload("writerbuffer_writer", []*cel.Type{cel.AnyType}, cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if writer, ok := val.Value().(*kaitai.Writer); ok {
						// Try to get buffer from underlying Writer if it's a bytes.Buffer
						if buf, ok := writer.Writer.(*bytes.Buffer); ok {
							return types.Bytes(buf.Bytes())
						}
						return types.NewErr("writer doesn't support buffer access")
					}
					return types.NewErr("expected Writer for writerBuffer function")
				}),
			),
		),
	}
}

func (*encodingLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// Helper function to convert values to int64
func toInt(val ref.Val) (int64, error) {
	switch v := val.(type) {
	case types.Int:
		return int64(v), nil
	case types.Double:
		return int64(v), nil
	case types.String:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", val.Value())
	}
}
