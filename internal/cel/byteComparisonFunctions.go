package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// ByteComparisonFunctions returns CEL function declarations for byte array comparison operations.
func ByteComparisonFunctions() cel.EnvOption {
	return cel.Lib(&byteComparisonLib{})
}

type byteComparisonLib struct{}

func (*byteComparisonLib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		// Helper function to convert integer list to bytes for comparison
		cel.Function("bytes_from_ints",
			cel.Overload("bytes_from_ints_list", []*cel.Type{cel.ListType(cel.IntType)}, cel.BytesType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					lister, ok := val.(traits.Lister)
					if !ok {
						return types.NewErr("expected list type")
					}

					listBytes := make([]byte, 0, int(lister.Size().(types.Int)))
					for i := types.Int(0); i < lister.Size().(types.Int); i++ {
						elem := lister.Get(i)
						if intVal, ok := elem.(types.Int); ok {
							if intVal < 0 || intVal > 255 {
								return types.NewErr("list element out of byte range: %d", intVal)
							}
							listBytes = append(listBytes, byte(intVal))
						} else {
							return types.NewErr("list element must be integer")
						}
					}

					return types.Bytes(listBytes)
				}),
			),
		),

		// Custom comparison functions to avoid conflicts with CEL stdlib
		cel.Function("bytes_lt",
			cel.Overload("bytes_lt_bytes_bytes", []*cel.Type{cel.BytesType, cel.BytesType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left := []byte(lhs.(types.Bytes))
					right := []byte(rhs.(types.Bytes))

					// Lexicographic comparison
					minLen := len(left)
					if len(right) < minLen {
						minLen = len(right)
					}

					for i := 0; i < minLen; i++ {
						if left[i] < right[i] {
							return types.True
						} else if left[i] > right[i] {
							return types.False
						}
					}

					// If all bytes are equal, shorter is less
					return types.Bool(len(left) < len(right))
				}),
			),
		),

		cel.Function("bytes_gt",
			cel.Overload("bytes_gt_bytes_bytes", []*cel.Type{cel.BytesType, cel.BytesType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					left := []byte(lhs.(types.Bytes))
					right := []byte(rhs.(types.Bytes))

					// Lexicographic comparison
					minLen := len(left)
					if len(right) < minLen {
						minLen = len(right)
					}

					for i := 0; i < minLen; i++ {
						if left[i] > right[i] {
							return types.True
						} else if left[i] < right[i] {
							return types.False
						}
					}

					// If all bytes are equal, longer is greater
					return types.Bool(len(left) > len(right))
				}),
			),
		),

		cel.Function("bytes_le",
			cel.Overload("bytes_le_bytes_bytes", []*cel.Type{cel.BytesType, cel.BytesType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					// lhs <= rhs is equivalent to !(lhs > rhs)
					left := []byte(lhs.(types.Bytes))
					right := []byte(rhs.(types.Bytes))

					// Lexicographic comparison
					minLen := len(left)
					if len(right) < minLen {
						minLen = len(right)
					}

					for i := 0; i < minLen; i++ {
						if left[i] < right[i] {
							return types.True
						} else if left[i] > right[i] {
							return types.False
						}
					}

					// If all bytes are equal, shorter or equal length is <=
					return types.Bool(len(left) <= len(right))
				}),
			),
		),

		cel.Function("bytes_ge",
			cel.Overload("bytes_ge_bytes_bytes", []*cel.Type{cel.BytesType, cel.BytesType}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					// lhs >= rhs is equivalent to !(lhs < rhs)
					left := []byte(lhs.(types.Bytes))
					right := []byte(rhs.(types.Bytes))

					// Lexicographic comparison
					minLen := len(left)
					if len(right) < minLen {
						minLen = len(right)
					}

					for i := 0; i < minLen; i++ {
						if left[i] > right[i] {
							return types.True
						} else if left[i] < right[i] {
							return types.False
						}
					}

					// If all bytes are equal, longer or equal length is >=
					return types.Bool(len(left) >= len(right))
				}),
			),
		),
	}
}

func (*byteComparisonLib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
