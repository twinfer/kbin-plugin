package formats_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
	// Adjust the import path to where your ksc-generated Go files will reside
	// The alias helps avoid name collisions if switch_repeat_expr_invalid is a common word.
)

// loadKsySchema is a helper to load a .ksy file for the interpreter.
// It's defined once per generated test file.
func loadKsySchemaForSwitchRepeatExprInvalid(t *testing.T, ksyPath string) *kaitaistruct.KaitaiSchema {
	yamlData, err := os.ReadFile(ksyPath)
	require.NoError(t, err, "Failed to read KSY file: %s", ksyPath)
	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err, "Failed to parse KSY YAML: %s", ksyPath)
	return schema
}

// structToMapForSerializer converts a KSC-generated struct to map[string]any
// for the custom serializer. This is a placeholder and needs robust implementation.
func structToMapForSerializerForSwitchRepeatExprInvalid(t *testing.T, data any) map[string]any {
	jsonData, err := json.Marshal(data)
	require.NoError(t, err, "Failed to marshal KSC struct to JSON")

	var resultMap map[string]any
	if err := json.Unmarshal(jsonData, &resultMap); err != nil {
		t.Logf("Warning: Could not unmarshal KSC struct directly to map for serializer (type: %T). Wrapping in '_value'. Error: %v", data, err)
		return map[string]any{"_value": data}
	}
	return resultMap
}

func TestParse_SwitchRepeatExprInvalid(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "switch_repeat_expr_invalid.ksy")
	ksySchema := loadKsySchemaForSwitchRepeatExprInvalid(t, ksyFilePath)

	interpreter, err := kaitaistruct.NewKaitaiInterpreter(ksySchema, nil)
	require.NoError(t, err)

	// Add a placeholder use of interpreter if there are no test cases
	_ = interpreter
	t.Logf("No binary test cases for switch_repeat_expr_invalid, skipping detailed parse tests.")

}

func TestSerialize_SwitchRepeatExprInvalid(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "switch_repeat_expr_invalid.ksy")
	ksySchema := loadKsySchemaForSwitchRepeatExprInvalid(t, ksyFilePath)

	serializer, err := kaitaistruct.NewKaitaiSerializer(ksySchema, nil)
	require.NoError(t, err)

	// Add a placeholder use of serializer if there are no test cases
	_ = serializer
	t.Logf("No binary test cases for switch_repeat_expr_invalid, skipping detailed serialize tests.")

}
