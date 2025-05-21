package formats_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"

	// Adjust the import path to where your ksc-generated Go files will reside
	// The alias helps avoid name collisions if repeat_until_complex is a common word.
	repeat_until_complex_kaitai "github.com/twinfer/kbin-plugin/testdata/formats_kaitai_go_gen/repeat_until_complex"
)

// loadKsySchema is a helper to load a .ksy file for the interpreter.
// It's defined once per generated test file.
func loadKsySchemaForRepeatUntilComplex(t *testing.T, ksyPath string) *kaitaistruct.KaitaiSchema {
	yamlData, err := os.ReadFile(ksyPath)
	require.NoError(t, err, "Failed to read KSY file: %s", ksyPath)
	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err, "Failed to parse KSY YAML: %s", ksyPath)
	return schema
}

// structToMapForSerializer converts a KSC-generated struct to map[string]any
// for the custom serializer. This is a placeholder and needs robust implementation.
func structToMapForSerializerForRepeatUntilComplex(t *testing.T, data any) map[string]any {
	jsonData, err := json.Marshal(data)
	require.NoError(t, err, "Failed to marshal KSC struct to JSON")

	var resultMap map[string]any
	if err := json.Unmarshal(jsonData, &resultMap); err != nil {
		t.Logf("Warning: Could not unmarshal KSC struct directly to map for serializer (type: %T). Wrapping in '_value'. Error: %v", data, err)
		return map[string]any{"_value": data}
	}
	return resultMap
}

func TestParse_RepeatUntilComplex(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "repeat_until_complex.ksy")
	ksySchema := loadKsySchemaForRepeatUntilComplex(t, ksyFilePath)

	interpreter, err := kaitaistruct.NewKaitaiInterpreter(ksySchema, nil)
	require.NoError(t, err)

	t.Run("repeat_until_complex_repeat_until_complex_Parse", func(t *testing.T) {
		samplePath := filepath.Join("../../../test/src", "repeat_until_complex.bin")
		binData, err := os.ReadFile(samplePath)
		require.NoError(t, err)

		stream := kaitai.NewStream(bytes.NewReader(binData))

		// 1. Parse with custom KaitaiInterpreter
		customParsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err, "Custom parser failed")

		// Reset stream for ksc parser
		stream = kaitai.NewStream(bytes.NewReader(binData))

		// 2. Parse with ksc-generated Go struct
		kscParsed := repeat_until_complex_kaitai.NewRepeatUntilComplex() // Use $ here
		err = kscParsed.Read(stream, kscParsed, kscParsed)
		require.NoError(t, err, "KSC generated parser failed")

		// 3. Compare results
		customMap := kaitaistruct.ParsedDataToMap(customParsed)

		kscJSON, err := json.Marshal(kscParsed)
		require.NoError(t, err, "Failed to marshal KSC parsed struct to JSON")

		var kscMap map[string]any
		err = json.Unmarshal(kscJSON, &kscMap)
		require.NoError(t, err, "Failed to unmarshal KSC JSON to map")

		if diff := cmp.Diff(kscMap, customMap); diff != "" {
			t.Errorf("Parser output mismatch for repeat_until_complex (-want ksc_map, +got custom_map):\n%s", diff)
		}
	})

}

func TestSerialize_RepeatUntilComplex(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "repeat_until_complex.ksy")
	ksySchema := loadKsySchemaForRepeatUntilComplex(t, ksyFilePath)

	serializer, err := kaitaistruct.NewKaitaiSerializer(ksySchema, nil)
	require.NoError(t, err)

	t.Run("repeat_until_complex_repeat_until_complex_Serialize", func(t *testing.T) {
		samplePath := filepath.Join("../../../test/src", "repeat_until_complex.bin")
		binData, err := os.ReadFile(samplePath)
		require.NoError(t, err)

		goldenStream := kaitai.NewStream(bytes.NewReader(binData))
		// Corrected: Use $ to access fields from the root TemplateData context
		goldenStruct := repeat_until_complex_kaitai.NewRepeatUntilComplex()
		err = goldenStruct.Read(goldenStream, goldenStruct, goldenStruct)
		require.NoError(t, err, "Failed to parse golden .bin with KSC parser")

		// Corrected: Use $ to access fields from the root TemplateData context
		inputMap := structToMapForSerializerForRepeatUntilComplex(t, goldenStruct)

		serializedBytes, err := serializer.Serialize(context.Background(), inputMap)
		require.NoError(t, err, "Custom serializer failed")

		reparsedStream := kaitai.NewStream(bytes.NewReader(serializedBytes))
		// Corrected: Use $ to access fields from the root TemplateData context
		reparsedStruct := repeat_until_complex_kaitai.NewRepeatUntilComplex()
		err = reparsedStruct.Read(reparsedStream, reparsedStruct, reparsedStruct)
		require.NoError(t, err, "Failed to parse bytes from custom serializer with KSC parser")

		if diff := cmp.Diff(goldenStruct, reparsedStruct); diff != "" {
			t.Errorf("Serializer output mismatch for repeat_until_complex (-want golden_ksc_struct, +got reparsed_ksc_struct):\n%s", diff)
		}
	})

}
