package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// streamOperations returns CEL function declarations for Kaitai stream operations
func streamOperations() cel.EnvOption {
	return cel.Lib(&streamLib{})
}

type streamLib struct{}

func (*streamLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Stream position
		cel.Function("pos",
			cel.Overload("pos_stream", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						pos, err := stream.Pos()
						if err != nil {
							return types.NewErr("failed to get position: %v", err)
						}
						return types.Int(pos)
					}
					return types.NewErr("expected Stream for pos function")
				}),
			),
		),

		// Stream size
		cel.Function("stream_size",
			cel.Overload("stream_size_any", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						size, err := stream.Size()
						if err != nil {
							return types.NewErr("failed to get stream size: %v", err)
						}
						return types.Int(size)
					}
					return types.NewErr("expected Stream for size function")
				}),
			),
		),

		// Stream seek
		cel.Function("seek",
			cel.Overload("seek_stream_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					stream, ok1 := lhs.Value().(*kaitai.Stream)
					pos, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to seek")
					}
					_, err := stream.Seek(int64(pos), 0)
					if err != nil {
						return types.NewErr("failed to seek: %v", err)
					}
					return types.Bool(true)
				}),
			),
		),

		// Stream EOF check
		cel.Function("isEOF",
			cel.Overload("iseof_stream", []*cel.Type{cel.AnyType}, cel.BoolType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						isEof, err := stream.EOF()
						if err != nil {
							return types.NewErr("failed to check EOF: %v", err)
						}
						return types.Bool(isEof)
					}
					return types.NewErr("expected Stream for isEOF function")
				}),
			),
		),

		// Read functions for common types
		cel.Function("readU1",
			cel.Overload("readu1_stream", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						v, err := stream.ReadU1()
						if err != nil {
							return types.NewErr("failed to read u1: %v", err)
						}
						return types.Int(v)
					}
					return types.NewErr("expected Stream for readU1 function")
				}),
			),
		),

		cel.Function("readU2le",
			cel.Overload("readu2le_stream", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						v, err := stream.ReadU2le()
						if err != nil {
							return types.NewErr("failed to read u2le: %v", err)
						}
						return types.Int(v)
					}
					return types.NewErr("expected Stream for readU2le function")
				}),
			),
		),

		cel.Function("readU4le",
			cel.Overload("readu4le_stream", []*cel.Type{cel.AnyType}, cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						v, err := stream.ReadU4le()
						if err != nil {
							return types.NewErr("failed to read u4le: %v", err)
						}
						return types.Int(v)
					}
					return types.NewErr("expected Stream for readU4le function")
				}),
			),
		),

		cel.Function("readBytes",
			cel.Overload("readbytes_stream_int", []*cel.Type{cel.AnyType, cel.IntType}, cel.BytesType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					stream, ok1 := lhs.Value().(*kaitai.Stream)
					size, ok2 := rhs.(types.Int)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to readBytes")
					}
					data, err := stream.ReadBytes(int(size))
					if err != nil {
						return types.NewErr("failed to read bytes: %v", err)
					}
					return types.Bytes(data)
				}),
			),
		),

		cel.Function("readBytesFull",
			cel.Overload("readbytesfull_stream", []*cel.Type{cel.AnyType}, cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					if stream, ok := val.Value().(*kaitai.Stream); ok {
						data, err := stream.ReadBytesFull()
						if err != nil {
							return types.NewErr("failed to read all bytes: %v", err)
						}
						return types.Bytes(data)
					}
					return types.NewErr("expected Stream for readBytesFull function")
				}),
			),
		),

		cel.Function("readBytesTerm",
			cel.Overload("readbytesterm_stream_int_bool_bool_bool",
				[]*cel.Type{cel.AnyType, cel.IntType, cel.BoolType, cel.BoolType, cel.BoolType},
				cel.BytesType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 5 {
						return types.NewErr("readBytesTerm requires 5 arguments")
					}

					stream, ok := args[0].Value().(*kaitai.Stream)
					if !ok {
						return types.NewErr("expected Stream for readBytesTerm")
					}

					term, ok := args[1].(types.Int)
					if !ok {
						return types.NewErr("expected int for terminator")
					}

					includeTerm, ok := args[2].(types.Bool)
					if !ok {
						return types.NewErr("expected bool for includeTerm")
					}

					consumeTerm, ok := args[3].(types.Bool)
					if !ok {
						return types.NewErr("expected bool for consumeTerm")
					}

					eosError, ok := args[4].(types.Bool)
					if !ok {
						return types.NewErr("expected bool for eosError")
					}

					data, err := stream.ReadBytesTerm(byte(term), bool(includeTerm), bool(consumeTerm), bool(eosError))
					if err != nil {
						return types.NewErr("failed to read bytes until terminator: %v", err)
					}
					return types.Bytes(data)
				}),
			),
		),

		// Unified read function that handles different types
		cel.Function("read",
			cel.Overload("read_stream_string", []*cel.Type{cel.AnyType, cel.StringType}, cel.AnyType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					stream, ok1 := lhs.Value().(*kaitai.Stream)
					typeStr, ok2 := rhs.(types.String)
					if !ok1 || !ok2 {
						return types.NewErr("invalid arguments to read")
					}

					switch string(typeStr) {
					case "u1":
						val, err := stream.ReadU1()
						if err != nil {
							return types.NewErr("failed to read u1: %v", err)
						}
						return types.Int(val)
					case "u2le":
						val, err := stream.ReadU2le()
						if err != nil {
							return types.NewErr("failed to read u2le: %v", err)
						}
						return types.Int(val)
					case "u4le":
						val, err := stream.ReadU4le()
						if err != nil {
							return types.NewErr("failed to read u4le: %v", err)
						}
						return types.Int(val)
					case "u8le":
						val, err := stream.ReadU8le()
						if err != nil {
							return types.NewErr("failed to read u8le: %v", err)
						}
						return types.Int(val)
					case "s1":
						val, err := stream.ReadS1()
						if err != nil {
							return types.NewErr("failed to read s1: %v", err)
						}
						return types.Int(val)
					case "s2le":
						val, err := stream.ReadS2le()
						if err != nil {
							return types.NewErr("failed to read s2le: %v", err)
						}
						return types.Int(val)
					case "s4le":
						val, err := stream.ReadS4le()
						if err != nil {
							return types.NewErr("failed to read s4le: %v", err)
						}
						return types.Int(val)
					case "s8le":
						val, err := stream.ReadS8le()
						if err != nil {
							return types.NewErr("failed to read s8le: %v", err)
						}
						return types.Int(val)
					case "u2be":
						val, err := stream.ReadU2be()
						if err != nil {
							return types.NewErr("failed to read u2be: %v", err)
						}
						return types.Int(val)
					case "u4be":
						val, err := stream.ReadU4be()
						if err != nil {
							return types.NewErr("failed to read u4be: %v", err)
						}
						return types.Int(val)
					case "u8be":
						val, err := stream.ReadU8be()
						if err != nil {
							return types.NewErr("failed to read u8be: %v", err)
						}
						return types.Int(val)
					default:
						return types.NewErr("unknown type: %s", typeStr)
					}
				}),
			),
		),
	}
}

func (*streamLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
