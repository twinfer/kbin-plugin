package kaitaistruct

import (
	"bytes"
	"compress/zlib"
	"context"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCELProcessFunctions tests the actual CEL-based process functions used by the parser
func TestCELProcessFunctions(t *testing.T) {
	tests := []struct {
		name       string
		yamlSchema string
		inputData  []byte
		expected   map[string]any
	}{
		{
			name: "XOR with constant",
			yamlSchema: `
meta:
  id: process_xor_const
  endian: le
seq:
  - id: key
    type: u1
  - id: buf
    size-eos: true
    process: xor(0xff)
`,
			inputData: []byte{0xff, 0x99, 0x90, 0x90, 0xdf, 0x9d, 0x9e, 0x8d}, // Real Kaitai test data
			expected: map[string]any{
				"key": int64(255),
				"buf": []byte{102, 111, 111, 32, 98, 97, 114}, // "foo bar"
			},
		},
		{
			name: "XOR with field value",
			yamlSchema: `
meta:
  id: test_xor_field
seq:
  - id: key
    type: u1
  - id: processed_data
    size-eos: true
    process: xor(key)
`,
			inputData: []byte{0x20, 0x61, 0x62, 0x63}, // key=0x20, data="abc" XORed with 0x20
			expected: map[string]any{
				"key":            int64(0x20),
				"processed_data": []byte{0x41, 0x42, 0x43}, // "ABC"
			},
		},
		{
			name: "Rotate left",
			yamlSchema: `
meta:
  id: test_rotate_left
seq:
  - id: processed_data
    size-eos: true
    process: rotate(3)
`,
			inputData: []byte{0x99, 0x90, 0x90}, // Data rotated left by 3
			expected: map[string]any{
				"processed_data": []byte{0xCC, 0x84, 0x84}, // Original data
			},
		},
		{
			name: "Zlib decompression",
			yamlSchema: `
meta:
  id: test_zlib
seq:
  - id: compressed_data
    size-eos: true
    process: zlib
`,
			inputData: func() []byte {
				var buf bytes.Buffer
				w := zlib.NewWriter(&buf)
				w.Write([]byte("Hello, Zlib!"))
				w.Close()
				return buf.Bytes()
			}(),
			expected: map[string]any{
				"compressed_data": []byte("Hello, Zlib!"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse schema
			schema, err := NewKaitaiSchemaFromYAML([]byte(tt.yamlSchema))
			require.NoError(t, err)

			// Create interpreter
			interpreter, err := NewKaitaiInterpreter(schema, nil)
			require.NoError(t, err)

			// Parse data
			stream := kaitai.NewStream(bytes.NewReader(tt.inputData))
			result, err := interpreter.Parse(context.Background(), stream)
			require.NoError(t, err)

			// Convert to map and verify results
			resultMap := ParsedDataToMap(result)
			dataMap, ok := resultMap.(map[string]any)
			require.True(t, ok)

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, dataMap[key])
			}
		})
	}
}

// TestCELProcessError tests error handling in CEL process functions
func TestCELProcessError(t *testing.T) {
	tests := []struct {
		name         string
		yamlSchema   string
		inputData    []byte
		expectError  bool
		errorContains string
	}{
		{
			name: "Unknown process function",
			yamlSchema: `
meta:
  id: test_unknown_process
seq:
  - id: data
    size-eos: true
    process: unknown_func(123)
`,
			inputData:     []byte{0x01, 0x02, 0x03},
			expectError:   true,
			errorContains: "unknown process function",
		},
		{
			name: "Invalid zlib data",
			yamlSchema: `
meta:
  id: test_invalid_zlib
seq:
  - id: bad_compressed
    size-eos: true
    process: zlib
`,
			inputData:     []byte{0xFF, 0xFE, 0xFD}, // Invalid zlib data
			expectError:   true,
			errorContains: "zlib decompression error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse schema
			schema, err := NewKaitaiSchemaFromYAML([]byte(tt.yamlSchema))
			require.NoError(t, err)

			// Create interpreter
			interpreter, err := NewKaitaiInterpreter(schema, nil)
			require.NoError(t, err)

			// Parse data - should fail
			stream := kaitai.NewStream(bytes.NewReader(tt.inputData))
			_, err = interpreter.Parse(context.Background(), stream)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestProcessDataWithCEL tests the processDataWithCEL function directly
func TestProcessDataWithCEL(t *testing.T) {
	// Create a basic schema for testing
	yamlContent := `
meta:
  id: test_schema
seq:
  - id: dummy
    type: u1
`
	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	tests := []struct {
		name        string
		processSpec string
		inputData   []byte
		expected    []byte
	}{
		{
			name:        "XOR with hex constant",
			processSpec: "xor(0xFF)",
			inputData:   []byte{0x00, 0xFF, 0x55},
			expected:    []byte{0xFF, 0x00, 0xAA},
		},
		{
			name:        "Rotate left by 1",
			processSpec: "rotate(1)",
			inputData:   []byte{0x80, 0x01},
			expected:    []byte{0x01, 0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal parse context
			pCtx := &ParseContext{
				Root:     &ParseContext{},
				Parent:   nil,
				Children: make(map[string]any),
			}

			result, err := interpreter.processDataWithCEL(context.Background(), tt.inputData, tt.processSpec, pCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}