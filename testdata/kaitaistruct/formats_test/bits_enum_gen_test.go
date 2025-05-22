package formats_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
	// The alias helps avoid name collisions if bits_enum is a common word.
)

func loadKsySchemaForBitsEnum(t *testing.T, ksyPath string) *kaitaistruct.KaitaiSchema {
	yamlData, err := os.ReadFile(ksyPath)
	require.NoError(t, err, "Failed to read KSY file: %s", ksyPath)
	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err, "Failed to parse KSY YAML: %s", ksyPath)
	return schema
}

func structToMapForSerializerForBitsEnum(t *testing.T, data any) map[string]any {
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

func TestParse_BitsEnum(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "bits_enum.ksy")
	ksySchema := loadKsySchemaForBitsEnum(t, ksyPath)
	interpreter, err := kaitaistruct.NewKaitaiInterpreter(ksySchema, nil)
	require.NoError(t, err)

	// Add a placeholder use of interpreter if there are no .bin test cases but we have assertions
	_ = interpreter
	t.Logf("No binary test cases for bits_enum, specific value assertions might still run if extracted.")

}

func TestSerialize_BitsEnum(t *testing.T) {
	ksyFilePath := filepath.Join("../../../test/formats", "bits_enum.ksy")
	ksySchema := loadKsySchemaForBitsEnum(t, ksyPath)
	serializer, err := kaitaistruct.NewKaitaiSerializer(ksySchema, nil)
	require.NoError(t, err)

	// Add a placeholder use of serializer if there are no .bin test cases
	_ = serializer
	t.Logf("No binary test cases for bits_enum, skipping detailed serialize tests.")

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
