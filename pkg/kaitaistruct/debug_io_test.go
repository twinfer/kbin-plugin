package kaitaistruct

import (
	"bytes"
	"context"
	"testing"

	kaitai "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/require"
)

func TestDebugIoAccess(t *testing.T) {
	yamlContent := `
meta:
  id: debug_io_access
seq:
  - id: obj1
    size: 4
    type: one
types:
  one:
    seq:
      - id: one
        type: u1
`

	// Test data: obj1 data (4 bytes)
	data := []byte{0x50, 0x00, 0x00, 0x00} // obj1 (4 bytes): one=0x50, padding

	schema, err := NewKaitaiSchemaFromYAML([]byte(yamlContent))
	require.NoError(t, err)

	stream := kaitai.NewStream(bytes.NewReader(data))
	interpreter, err := NewKaitaiInterpreter(schema, nil)
	require.NoError(t, err)

	result, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	resultData := ParsedDataToMap(result)
	dataMap, ok := resultData.(map[string]any)
	require.True(t, ok)

	// Check if obj1 has _io metadata
	obj1, ok := dataMap["obj1"].(map[string]any)
	require.True(t, ok)
	t.Logf("obj1 keys: %v", mapKeys(obj1))

	if ioData, hasIo := obj1["_io"]; hasIo {
		t.Logf("obj1._io: %v (type: %T)", ioData, ioData)
		if ioMap, ok := ioData.(map[string]any); ok {
			t.Logf("obj1._io keys: %v", mapKeys(ioMap))
			if size, hasSize := ioMap["size"]; hasSize {
				t.Logf("obj1._io.size: %v (type: %T)", size, size)
			}
		}
	} else {
		t.Log("obj1 does not have _io")
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
