package kaitaistruct

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/require"
)

func TestBitsSimple_Debug(t *testing.T) {
	// Load schema
	yamlData, err := os.ReadFile("../../test/formats/bits_simple.ksy")
	require.NoError(t, err)

	schema, err := NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interpreter, err := NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Read test data
	binData, err := os.ReadFile("../../test/src/fixed_struct.bin")
	require.NoError(t, err)

	// Parse
	stream := kaitai.NewStream(bytes.NewReader(binData))
	parsed, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	// Convert to map
	result := ParsedDataToMap(parsed).(map[string]any)
	
	// Print all keys
	t.Logf("Keys in result map:")
	for key := range result {
		t.Logf("  - %s", key)
	}
}