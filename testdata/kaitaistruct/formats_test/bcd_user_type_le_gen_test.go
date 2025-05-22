package formats_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"

	// The alias helps avoid name collisions if bcd_user_type_le is a common word.
	bcd_user_type_le_kaitai "github.com/twinfer/kbin-plugin/testdata/formats_kaitai_go_gen/bcd_user_type_le"
)

func loadKsySchemaForBcdUserTypeLe(t *testing.T, ksyPath string) *kaitaistruct.KaitaiSchema {
	yamlData, err := os.ReadFile(ksyPath)
	require.NoError(t, err, "Failed to read KSY file: %s", ksyPath)
	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err, "Failed to parse KSY YAML: %s", ksyPath)
	return schema
}

func structToMapForSerializerForBcdUserTypeLe(t *testing.T, data any) map[string]any {
	jsonData, err := json.Marshal(data)
	require.NoError(t, err, "Failed to marshal KSC struct to JSON")
	var resultMap map[string]any
	if err := json.Unmarshal(jsonData, &resultMap); err != nil {
		t.Logf("Warning: Could not unmarshal KSC struct directly to map for serializer (type: %T). Wrapping in '_value'. Error: %v", data, err)
		return map[string]any{"_value": data}
	}
	return convertKeysToLowerRecursive(resultMap)
}

func convertKeysToLowerRecursive(data any) any {
	if m, ok := data.(map[string]any); ok {
		lowerMap := make(map[string]any)
		for k, v := range m {
			lowerMap[strings.ToLower(k)] = convertKeysToLowerRecursive(v)
		}
		return lowerMap
	}
	if s, ok := data.([]any); ok {
		lowerSlice := make([]any, len(s))
		for i, v := range s {
			lowerSlice[i] = convertKeysToLowerRecursive(v)
		}
		return lowerSlice
	}
	return data
}

func TestParse_BcdUserTypeLe(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "bcd_user_type_le.ksy")
	ksySchema := loadKsySchemaForBcdUserTypeLe(t, ksyPath)
	interpreter, err := kaitaistruct.NewKaitaiInterpreter(ksySchema, nil)
	require.NoError(t, err)

	t.Run("bcd_user_type_le_bcd_user_type_le_Parse", func(t *testing.T) {
		samplePath := filepath.Join("../../../test/src", "bcd_user_type_le.bin")
		binData, err := os.ReadFile(samplePath)
		require.NoError(t, err)
		stream := kaitai.NewStream(bytes.NewReader(binData))

		customParsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err, "Custom parser failed")

		stream = kaitai.NewStream(bytes.NewReader(binData)) // Reset stream
		kscParsed := bcd_user_type_le_kaitai.NewBcdUserTypeLe()
		err = kscParsed.Read(stream, kscParsed, kscParsed)
		require.NoError(t, err, "KSC generated parser failed")

		customMap := kaitaistruct.ParsedDataToMap(customParsed)
		kscJSON, err := json.Marshal(kscParsed)
		require.NoError(t, err, "Failed to marshal KSC parsed struct to JSON")
		var kscMap map[string]any
		err = json.Unmarshal(kscJSON, &kscMap)
		require.NoError(t, err, "Failed to unmarshal KSC JSON to map")

		filteredCustomMap := filterMapKeys(customMap.(map[string]any), kscMap)
		if diff := cmp.Diff(kscMap, filteredCustomMap, numericComparer, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Parser output mismatch for bcd_user_type_le (-want ksc_map, +got filtered_custom_map):\n%s", diff)
			if fullDiff := cmp.Diff(kscMap, customMap, numericComparer, cmpopts.EquateEmpty()); fullDiff != diff && len(customMap.(map[string]any)) != len(filteredCustomMap) {
				t.Logf("Full customMap diff (includes instances):\n%s", fullDiff)
			}
		}

		// Perform specific assertions extracted from KSC Go tests

	})

}

func TestSerialize_BcdUserTypeLe(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "bcd_user_type_le.ksy")
	ksySchema := loadKsySchemaForBcdUserTypeLe(t, ksyPath)
	serializer, err := kaitaistruct.NewKaitaiSerializer(ksySchema, nil)
	require.NoError(t, err)

	t.Run("bcd_user_type_le_bcd_user_type_le_Serialize", func(t *testing.T) {
		samplePath := filepath.Join("../../../test/src", "bcd_user_type_le.bin")
		binData, err := os.ReadFile(samplePath)
		require.NoError(t, err)

		goldenStream := kaitai.NewStream(bytes.NewReader(binData))
		goldenStruct := bcd_user_type_le_kaitai.NewBcdUserTypeLe()
		err = goldenStruct.Read(goldenStream, goldenStruct, goldenStruct)
		require.NoError(t, err, "Failed to parse golden .bin with KSC parser")

		inputMap := structToMapForSerializerForBcdUserTypeLe(t, goldenStruct)
		t.Logf("Serializer inputMap for bcd_user_type_le: %#v", inputMap)

		serializedBytes, err := serializer.Serialize(context.Background(), inputMap)
		require.NoError(t, err, "Custom serializer failed")

		reparsedStream := kaitai.NewStream(bytes.NewReader(serializedBytes))
		reparsedStruct := bcd_user_type_le_kaitai.NewBcdUserTypeLe()
		err = reparsedStruct.Read(reparsedStream, reparsedStruct, reparsedStruct)
		require.NoError(t, err, "Failed to parse bytes from custom serializer with KSC parser")

		if diff := cmp.Diff(goldenStruct, reparsedStruct, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("Serializer output mismatch for bcd_user_type_le (-want golden_ksc_struct, +got reparsed_ksc_struct):\n%s", diff)
		}
	})

}

// Helper function to convert various numeric types to int64 for comparison
func convertToInt64(i any) (int64, bool) {
	switch v := i.(type) {
	case float64:
		if v == float64(int64(v)) {
			return int64(v), true
		}
		return 0, false
	case float32:
		if v == float32(int64(v)) {
			return int64(v), true
		}
		return 0, false
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		if v <= math.MaxInt64 {
			return int64(v), true
		}
		return 0, false
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		if v <= math.MaxInt64 {
			return int64(v), true
		}
		return 0, false
	case uint64:
		if v <= math.MaxInt64 {
			return int64(v), true
		}
		return 0, false
	default:
		return 0, false
	}
}

var numericComparer = cmp.Comparer(func(x, y any) bool {
	xInt, xOk := convertToInt64(x)
	yInt, yOk := convertToInt64(y)
	if xOk && yOk {
		return xInt == yInt
	}
	if xFloat, xIsFloat := x.(float64); xIsFloat {
		if yFloat, yIsFloat := y.(float64); yIsFloat {
			return math.Abs(xFloat-yFloat) < 1e-9
		}
	}
	return cmp.Equal(x, y)
})

// filterMapKeys recursively creates a new map from 'source' containing only keys present in 'reference'.
func filterMapKeys(source map[string]any, reference map[string]any) map[string]any {
	result := make(map[string]any)
	for key, refVal := range reference {
		if srcVal, ok := source[key]; ok {
			if refSubMap, refIsMap := refVal.(map[string]any); refIsMap {
				if srcSubMap, srcIsMap := srcVal.(map[string]any); srcIsMap {
					result[key] = filterMapKeys(srcSubMap, refSubMap)
				} else {
					result[key] = srcVal
				}
			} else {
				result[key] = srcVal
			}
		}
	}
	return result
}

func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
