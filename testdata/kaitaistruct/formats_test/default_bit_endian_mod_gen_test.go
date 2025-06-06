// Code generated by kaitai-test-gen-simple.go; DO NOT EDIT.
package formats_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
	default_bit_endian_mod_kaitai "github.com/twinfer/kbin-plugin/testdata/formats_kaitai_go_gen/default_bit_endian_mod"
)

func TestParse_DefaultBitEndianMod(t *testing.T) {
	// Load schema
	ksyPath := filepath.Join("../../../test/formats", "default_bit_endian_mod.ksy")
	yamlData, err := os.ReadFile(ksyPath)
	require.NoError(t, err)

	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	interpreter, err := kaitaistruct.NewKaitaiInterpreter(schema, logger)
	require.NoError(t, err)

	// Read binary file
	binPath := filepath.Join("../../../test/src", "fixed_struct.bin")
	binData, err := os.ReadFile(binPath)
	require.NoError(t, err)

	// Parse with custom parser
	stream := kaitai.NewStream(bytes.NewReader(binData))
	parsed, err := interpreter.Parse(context.Background(), stream)
	require.NoError(t, err)

	// Convert to map for assertions
	customMap := kaitaistruct.ParsedDataToMap(parsed).(map[string]any)

	// Assertions from KSC test

	// Assert Main.One
	if main_one_lvl0_map, ok := customMap["main"].(map[string]any); ok {
		if main_one_val, ok := main_one_lvl0_map["one"]; ok {
			assert.EqualValues(t, 336, main_one_val)
		} else {
			t.Fatalf("Field 'one' not found in main_one_lvl0_map (keys: %v)", maps.Keys(main_one_lvl0_map))
		}
	} else {
		t.Fatalf("Field 'main' not found or not a map while asserting Main.One")
	}

	// Assert Main.Two
	if main_two_lvl0_map, ok := customMap["main"].(map[string]any); ok {
		if main_two_val, ok := main_two_lvl0_map["two"]; ok {
			assert.EqualValues(t, 8608, main_two_val)
		} else {
			t.Fatalf("Field 'two' not found in main_two_lvl0_map (keys: %v)", maps.Keys(main_two_lvl0_map))
		}
	} else {
		t.Fatalf("Field 'main' not found or not a map while asserting Main.Two")
	}

	// Assert Main.Nest.Two
	if main_nest_two_lvl0_map, ok := customMap["main"].(map[string]any); ok {
		if main_nest_two_lvl1_map, ok := main_nest_two_lvl0_map["nest"].(map[string]any); ok {
			if main_nest_two_val, ok := main_nest_two_lvl1_map["two"]; ok {
				assert.EqualValues(t, 11595, main_nest_two_val)
			} else {
				t.Fatalf("Field 'two' not found in main_nest_two_lvl1_map (keys: %v)", maps.Keys(main_nest_two_lvl1_map))
			}
		} else {
			t.Fatalf("Field 'nest' not found or not a map while asserting Main.Nest.Two")
		}
	} else {
		t.Fatalf("Field 'main' not found or not a map while asserting Main.Nest.Two")
	}

	// Assert Main.NestBe.Two
	if main_nest_be_two_lvl0_map, ok := customMap["main"].(map[string]any); ok {
		if main_nest_be_two_lvl1_map, ok := main_nest_be_two_lvl0_map["nest_be"].(map[string]any); ok {
			if main_nest_be_two_val, ok := main_nest_be_two_lvl1_map["two"]; ok {
				assert.EqualValues(t, 12799, main_nest_be_two_val)
			} else {
				t.Fatalf("Field 'two' not found in main_nest_be_two_lvl1_map (keys: %v)", maps.Keys(main_nest_be_two_lvl1_map))
			}
		} else {
			t.Fatalf("Field 'nest_be' not found or not a map while asserting Main.NestBe.Two")
		}
	} else {
		t.Fatalf("Field 'main' not found or not a map while asserting Main.NestBe.Two")
	}

}

func TestSerialize_DefaultBitEndianMod(t *testing.T) {
	// Load schema
	ksyPath := filepath.Join("../../../test/formats", "default_bit_endian_mod.ksy")
	yamlData, err := os.ReadFile(ksyPath)
	require.NoError(t, err)

	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(yamlData)
	require.NoError(t, err)

	serializer, err := kaitaistruct.NewKaitaiSerializer(schema, nil)
	require.NoError(t, err)

	// Read original binary
	binPath := filepath.Join("../../../test/src", "fixed_struct.bin")
	originalData, err := os.ReadFile(binPath)
	require.NoError(t, err)

	// Parse original
	stream := kaitai.NewStream(bytes.NewReader(originalData))
	original := default_bit_endian_mod_kaitai.NewDefaultBitEndianMod()
	err = original.Read(stream, original, original)
	require.NoError(t, err)

	// Serialize and verify round-trip
	inputMap := structToMapDefaultBitEndianMod(t, original)

	serialized, err := serializer.Serialize(context.Background(), inputMap)
	require.NoError(t, err)

	// Parse serialized data
	stream2 := kaitai.NewStream(bytes.NewReader(serialized))
	reparsed := default_bit_endian_mod_kaitai.NewDefaultBitEndianMod()
	err = reparsed.Read(stream2, reparsed, reparsed)
	require.NoError(t, err)

	// Compare structures with Phase 2/3 enhanced comparison
	cmpOpts := []cmp.Option{
		cmpopts.IgnoreUnexported(
			kaitai.Stream{},
		),
		cmpopts.IgnoreFields(default_bit_endian_mod_kaitai.DefaultBitEndianMod{}, "_io", "_parent", "_root"),
		cmpopts.EquateEmpty(),
	}
	if diff := cmp.Diff(original, reparsed, cmpOpts...); diff != "" {
		t.Errorf("Serialization mismatch (-original +reparsed):\n%s", diff)
	}
}

// structToMapDefaultBitEndianMod converts KSC struct to map for serializer
func structToMapDefaultBitEndianMod(t *testing.T, data any) map[string]any {
	// Use reflection to handle KSC structs with method-based values
	return structToMapReflectiveDefaultBitEndianMod(t, data)
}

// structToMapReflectiveDefaultBitEndianMod uses reflection to convert KSC structs including method calls
func structToMapReflectiveDefaultBitEndianMod(t *testing.T, data any) map[string]any {
	result := make(map[string]any)

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	typ := val.Type()

	// Process fields
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip internal KSC fields
		if strings.HasPrefix(field.Name, "_") {
			continue
		}

		fieldName := strings.ToLower(field.Name)

		if !fieldVal.IsValid() || fieldVal.IsZero() {
			continue
		}

		// Handle different field types
		switch fieldVal.Kind() {
		case reflect.Ptr:
			if !fieldVal.IsNil() {
				// For custom types, check for AsInt/AsStr methods
				subResult := make(map[string]any)

				// Try to call AsInt method
				if method := fieldVal.MethodByName("AsInt"); method.IsValid() {
					if results := method.Call(nil); len(results) == 2 && results[1].IsNil() {
						subResult["asint"] = results[0].Interface()
					}
				}

				// Try to call AsStr method
				if method := fieldVal.MethodByName("AsStr"); method.IsValid() {
					if results := method.Call(nil); len(results) == 2 && results[1].IsNil() {
						subResult["asstr"] = results[0].Interface()
					}
				}

				// Also include raw fields
				subMap := structToMapReflectiveDefaultBitEndianMod(t, fieldVal.Interface())
				for k, v := range subMap {
					if _, exists := subResult[k]; !exists {
						subResult[k] = v
					}
				}

				result[fieldName] = subResult
			}
		case reflect.Slice:
			if fieldVal.Len() > 0 {
				slice := make([]any, fieldVal.Len())
				for j := 0; j < fieldVal.Len(); j++ {
					slice[j] = fieldVal.Index(j).Interface()
				}
				result[fieldName] = slice
			}
		default:
			result[fieldName] = fieldVal.Interface()
		}
	}

	return result
}
