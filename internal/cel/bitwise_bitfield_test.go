package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
)

func TestBitwiseOperationsWithKaitaiBitField(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		vars       map[string]interface{}
		expected   interface{}
	}{
		{
			name:       "bitAnd function with bitfields",
			expression: "bitAnd(bf1, bf2)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b1010, 4), // 10
				"bf2": mustCreateBitField(t, 0b1100, 4), // 12
			},
			expected: int64(0b1000), // 8
		},
		{
			name:       "bitOr function with bitfields",
			expression: "bitOr(bf1, bf2)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b1010, 4), // 10
				"bf2": mustCreateBitField(t, 0b0101, 4), // 5
			},
			expected: int64(0b1111), // 15
		},
		{
			name:       "bitXor function with bitfields",
			expression: "bitXor(bf1, bf2)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b1010, 4), // 10
				"bf2": mustCreateBitField(t, 0b1100, 4), // 12
			},
			expected: int64(0b0110), // 6
		},
		{
			name:       "mixed bitfield and int",
			expression: "bitAnd(bf1, 15)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b1010, 4), // 10
			},
			expected: int64(0b1010), // 10
		},
		{
			name:       "bitfield operations with kaitai int",
			expression: "bitOr(bf1, ki1)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b1010, 4), // 10
				"ki1": kaitaicel.NewKaitaiU1(5, []byte{5}), // 5
			},
			expected: int64(0b1111), // 15
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create environment with variable declarations
			envOpts := []cel.EnvOption{cel.StdLib()}
			
			// Add variables based on test vars
			for varName := range test.vars {
				envOpts = append(envOpts, cel.Variable(varName, cel.DynType))
			}

			// Create environment with bitwise functions
			baseEnv, err := NewEnvironment()
			require.NoError(t, err)

			// Add variables to the base environment
			env, err := baseEnv.Extend(envOpts...)
			require.NoError(t, err)

			ast, issues := env.Compile(test.expression)
			require.NoError(t, issues.Err(), "Expression compilation failed")

			prg, err := env.Program(ast)
			require.NoError(t, err, "Program creation failed")

			activation, err := cel.NewActivation(test.vars)
			require.NoError(t, err)
			result, _, err := prg.Eval(activation)
			require.NoError(t, err, "Expression evaluation failed")

			// Convert result to comparable type
			var actual interface{}
			switch res := result.(type) {
			case types.Int:
				actual = int64(res)
			case types.Uint:
				actual = uint64(res)
			case *kaitaicel.KaitaiBitField:
				actual = int64(res.AsInt())
			default:
				actual = result.Value()
			}

			assert.Equal(t, test.expected, actual, "Expression %s should return expected value", test.expression)
		})
	}
}

func mustCreateBitField(t *testing.T, value uint64, bits int) *kaitaicel.KaitaiBitField {
	bf, err := kaitaicel.NewKaitaiBitField(value, bits)
	require.NoError(t, err)
	return bf
}

func TestBitwiseShiftOperationsWithBitField(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		vars       map[string]interface{}
		expected   interface{}
	}{
		{
			name:       "bitShiftLeft function",
			expression: "bitShiftLeft(10, 2)",
			vars:       map[string]interface{}{},
			expected:   int64(40),
		},
		{
			name:       "bitShiftRight function",
			expression: "bitShiftRight(40, 2)",
			vars:       map[string]interface{}{},
			expected:   int64(10),
		},
		{
			name:       "bitShiftLeft with uint",
			expression: "bitShiftLeft(uint(10), 2)",
			vars:       map[string]interface{}{},
			expected:   uint64(40),
		},
		{
			name:       "bitShiftLeft with KaitaiBitField",
			expression: "bitShiftLeft(bf1, 2)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b0101, 4), // 5
			},
			expected: int64(20), // 5 << 2 = 20
		},
		{
			name:       "bitShiftRight with KaitaiBitField",
			expression: "bitShiftRight(bf1, 1)",
			vars: map[string]interface{}{
				"bf1": mustCreateBitField(t, 0b1010, 4), // 10
			},
			expected: int64(5), // 10 >> 1 = 5
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create environment with variable declarations if needed
			envOpts := []cel.EnvOption{cel.StdLib()}
			
			// Add variables based on test vars
			for varName := range test.vars {
				envOpts = append(envOpts, cel.Variable(varName, cel.DynType))
			}

			// Create environment with bitwise functions
			baseEnv, err := NewEnvironment()
			require.NoError(t, err)

			// Add variables to the base environment if any
			env := baseEnv
			if len(test.vars) > 0 {
				env, err = baseEnv.Extend(envOpts...)
				require.NoError(t, err)
			}

			ast, issues := env.Compile(test.expression)
			require.NoError(t, issues.Err(), "Expression compilation failed")

			prg, err := env.Program(ast)
			require.NoError(t, err, "Program creation failed")

			activation, err := cel.NewActivation(test.vars)
			require.NoError(t, err)
			result, _, err := prg.Eval(activation)
			require.NoError(t, err, "Expression evaluation failed")

			// Convert result to comparable type
			var actual interface{}
			switch res := result.(type) {
			case types.Int:
				actual = int64(res)
			case types.Uint:
				actual = uint64(res)
			default:
				actual = result.Value()
			}

			assert.Equal(t, test.expected, actual, "Expression %s should return expected value", test.expression)
		})
	}
}