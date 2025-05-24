// /Users/khalid/dev/kbin-plugin/testutil/helpers.go
package testutil

import (
	"math"
	"strings" // For strings.ToLower in convertKeysToLowerRecursive
	"unicode" // For toPascalCase

	"github.com/google/go-cmp/cmp"
)

// ToPascalCase converts a snake_case or kebab-case string to PascalCase.
func ToPascalCase(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '_' || r == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ConvertToInt64 converts various numeric types to int64 for comparison.
// Returns the int64 value and a boolean indicating success.
func ConvertToInt64(i any) (int64, bool) {
	switch v := i.(type) {
	case float64:
		if v == float64(int64(v)) { // Check if it's a whole number
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
		// A uint32 will always fit into an int64 as a positive number
		// because math.MaxUint32 < math.MaxInt64.
		return int64(v), true
		// No explicit check against math.MaxInt64 needed here after casting to uint32.
	case uint64:
		if v <= math.MaxInt64 { // Check against MaxInt64 for uint64
			return int64(v), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// NumericComparer is a cmp.Comparer for flexible numeric comparison.
var NumericComparer = cmp.Comparer(func(x, y any) bool {
	xInt, xOk := ConvertToInt64(x)
	yInt, yOk := ConvertToInt64(y)
	if xOk && yOk {
		return xInt == yInt
	}
	if xFloat, xIsFloat := x.(float64); xIsFloat {
		if yFloat, yIsFloat := y.(float64); yIsFloat {
			return math.Abs(xFloat-yFloat) < 1e-9
		}
	}
	return cmp.Equal(x, y) // Fallback
})

// FilterMapKeys recursively creates a new map from 'source' containing only keys present in 'reference'.
func FilterMapKeys(source map[string]any, reference map[string]any) map[string]any {
	result := make(map[string]any)
	for key, refVal := range reference {
		if srcVal, ok := source[key]; ok {
			if refSubMap, refIsMap := refVal.(map[string]any); refIsMap {
				if srcSubMap, srcIsMap := srcVal.(map[string]any); srcIsMap {
					result[key] = FilterMapKeys(srcSubMap, refSubMap)
				} else {
					result[key] = srcVal // Type mismatch, will be caught by cmp.Diff
				}
			} else {
				result[key] = srcVal
			}
		}
	}
	return result
}

// GetMapKeys returns a slice of keys from a map.
func GetMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ConvertKeysToLowerRecursive recursively converts map keys to lowercase.
// Used for preparing serializer input.
func ConvertKeysToLowerRecursive(data any) any {
	if m, ok := data.(map[string]any); ok {
		lowerMap := make(map[string]any)
		for k, v := range m {
			lowerMap[strings.ToLower(k)] = ConvertKeysToLowerRecursive(v)
		}
		return lowerMap
	}
	if s, ok := data.([]any); ok {
		lowerSlice := make([]any, len(s))
		for i, v := range s {
			lowerSlice[i] = ConvertKeysToLowerRecursive(v)
		}
		return lowerSlice
	}
	return data
}
