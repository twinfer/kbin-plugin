package kaitaistruct

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringProcessingIntegrationWithParser(t *testing.T) {
	// Integration tests with the full parser pipeline
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("padding_removal_integration", func(t *testing.T) {
		schemaYAML := `
meta:
  id: padding_test
  encoding: UTF-8
seq:
  - id: padded_field
    type: str
    size: 10
    pad-right: 0x40  # '@'
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		// Test data: "hello" + 5 '@' padding characters
		testData := []byte("hello@@@@@")
		require.Equal(t, 10, len(testData), "test data should be exactly 10 bytes")

		stream := kaitai.NewStream(bytes.NewReader(testData))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		result := ParsedDataToMap(parsed).(map[string]any)
		assert.Equal(t, "hello", result["padded_field"], "padding should be removed")
	})

	t.Run("terminator_integration", func(t *testing.T) {
		schemaYAML := `
meta:
  id: terminator_test
  encoding: UTF-8
seq:
  - id: terminated_field
    type: str
    size: 15
    terminator: 0x7C  # '|'
  - id: terminated_include
    type: str
    size: 10
    terminator: 0x21  # '!'
    include: true
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		// Test data: "hello|world..." + "test!....."
		testData := []byte("hello|world....test!.....")
		require.Equal(t, 25, len(testData), "test data should be exactly 25 bytes")

		stream := kaitai.NewStream(bytes.NewReader(testData))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		result := ParsedDataToMap(parsed).(map[string]any)
		assert.Equal(t, "hello", result["terminated_field"], "should stop at terminator")
		assert.Equal(t, "test!", result["terminated_include"], "should include terminator")
	})

	t.Run("complex_terminator_and_padding", func(t *testing.T) {
		schemaYAML := `
meta:
  id: complex_test
  encoding: UTF-8
seq:
  - id: complex_field
    type: str
    size: 20
    terminator: 0x40  # '@'
    pad-right: 0x2B   # '+'
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		// Test case similar to str_term_and_pad from Kaitai test suite
		testData := []byte("str+++3bar+++@++++++")
		require.Equal(t, 20, len(testData), "test data should be exactly 20 bytes")

		stream := kaitai.NewStream(bytes.NewReader(testData))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		result := ParsedDataToMap(parsed).(map[string]any)
		assert.Equal(t, "str+++3bar+++", result["complex_field"],
			"should preserve content padding but stop at terminator")
	})

	t.Run("multiple_string_fields", func(t *testing.T) {
		schemaYAML := `
meta:
  id: multi_string_test
  encoding: UTF-8
seq:
  - id: field1
    type: str
    size: 8
    pad-right: 0x20  # space
  - id: field2
    type: str
    size: 10
    terminator: 0x00  # null
  - id: field3
    type: str
    size: 6
    terminator: 0x7C  # '|'
    include: true
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		// Test data: "hello   " + "world\x00...." + "end|.."
		testData := []byte("hello   world\x00....end|..")
		require.Equal(t, 24, len(testData), "test data should be exactly 24 bytes")

		stream := kaitai.NewStream(bytes.NewReader(testData))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		result := ParsedDataToMap(parsed).(map[string]any)
		assert.Equal(t, "hello", result["field1"], "field1 should have padding removed")
		assert.Equal(t, "world", result["field2"], "field2 should stop at null terminator")
		assert.Equal(t, "end|", result["field3"], "field3 should include terminator")
	})

	t.Run("edge_case_empty_fields", func(t *testing.T) {
		schemaYAML := `
meta:
  id: empty_test
  encoding: UTF-8
seq:
  - id: all_padding
    type: str
    size: 5
    pad-right: 0x20  # space
  - id: immediate_terminator
    type: str
    size: 8
    terminator: 0x00  # null
  - id: empty_with_include
    type: str
    size: 3
    terminator: 0x21  # '!'
    include: true
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		// Test data: "     " + "\x00......." + "!.."
		testData := []byte("     \x00.......!..")
		require.Equal(t, 16, len(testData), "test data should be exactly 16 bytes")

		stream := kaitai.NewStream(bytes.NewReader(testData))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		result := ParsedDataToMap(parsed).(map[string]any)
		assert.Equal(t, "", result["all_padding"], "all padding should result in empty string")
		assert.Equal(t, "", result["immediate_terminator"], "immediate terminator should result in empty string")
		assert.Equal(t, "!", result["empty_with_include"], "should include terminator even when no content")
	})

	t.Run("no_terminator_found_uses_padding", func(t *testing.T) {
		schemaYAML := `
meta:
  id: no_term_test
  encoding: UTF-8
seq:
  - id: no_terminator
    type: str
    size: 12
    terminator: 0xFF  # not present in data
    pad-right: 0x2E   # '.'
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		// Test data: "content....." (no 0xFF terminator, so padding should be applied)
		testData := []byte("content.....")
		require.Equal(t, 12, len(testData), "test data should be exactly 12 bytes")

		stream := kaitai.NewStream(bytes.NewReader(testData))
		parsed, err := interpreter.Parse(context.Background(), stream)
		require.NoError(t, err)

		result := ParsedDataToMap(parsed).(map[string]any)
		assert.Equal(t, "content", result["no_terminator"],
			"should remove padding when no terminator found")
	})
}

func TestStringProcessingErrorCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("invalid_terminator_value", func(t *testing.T) {
		schemaYAML := `
meta:
  id: error_test
  encoding: UTF-8
seq:
  - id: bad_terminator
    type: str
    size: 10
    terminator: 0x100  # out of byte range
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		testData := []byte("hello world test")
		stream := kaitai.NewStream(bytes.NewReader(testData))

		_, err = interpreter.Parse(context.Background(), stream)
		assert.Error(t, err, "should fail with invalid terminator value")
		assert.Contains(t, err.Error(), "out of range", "error should mention range issue")
	})

	t.Run("invalid_padding_value", func(t *testing.T) {
		schemaYAML := `
meta:
  id: error_test
  encoding: UTF-8
seq:
  - id: bad_padding
    type: str
    size: 10
    pad-right: -1  # negative value
`
		schema, err := NewKaitaiSchemaFromYAML([]byte(schemaYAML))
		require.NoError(t, err)

		interpreter, err := NewKaitaiInterpreter(schema, logger)
		require.NoError(t, err)

		testData := []byte("hello world test")
		stream := kaitai.NewStream(bytes.NewReader(testData))

		_, err = interpreter.Parse(context.Background(), stream)
		assert.Error(t, err, "should fail with invalid padding value")
		assert.Contains(t, err.Error(), "out of range", "error should mention range issue")
	})

	t.Run("unsupported_terminator_type", func(t *testing.T) {
		// This test verifies behavior when YAML contains unsupported types
		// Note: This might require manual SequenceItem construction since YAML parsing
		// might not allow such values
		k := &KaitaiInterpreter{}

		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: []byte{64}, // unsupported type
		}

		_, err := k.processStringBytes([]byte("test"), field)
		assert.Error(t, err, "should fail with unsupported terminator type")
		assert.Contains(t, err.Error(), "unsupported value type", "error should mention type issue")
	})
}

func TestStringProcessingPerformance(t *testing.T) {
	// Basic performance regression test
	k := &KaitaiInterpreter{}

	t.Run("large_string_with_padding", func(t *testing.T) {
		// Create a large string with padding
		content := make([]byte, 10000)
		for i := 0; i < 5000; i++ {
			content[i] = byte('A' + (i % 26)) // Fill with letters
		}
		for i := 5000; i < 10000; i++ {
			content[i] = 0x40 // Fill rest with padding '@'
		}

		field := SequenceItem{
			ID:       "test",
			Type:     "str",
			PadRight: 0x40,
		}

		result, err := k.processStringBytes(content, field)
		require.NoError(t, err)
		assert.Equal(t, 5000, len(result), "should remove all padding efficiently")
		assert.Equal(t, byte('A'), result[0], "should preserve content")
		assert.Equal(t, byte('A'+(4999%26)), result[4999], "should preserve content end")
	})

	t.Run("many_terminators", func(t *testing.T) {
		// Test with data that has many potential terminators
		content := make([]byte, 1000)
		for i := 0; i < 1000; i++ {
			if i%100 == 99 {
				content[i] = 0x40 // Terminator every 100 bytes
			} else {
				content[i] = byte('X')
			}
		}

		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: 0x40,
		}

		result, err := k.processStringBytes(content, field)
		require.NoError(t, err)
		assert.Equal(t, 99, len(result), "should stop at first terminator")
		// Verify all content before first terminator is preserved
		for i := 0; i < 99; i++ {
			assert.Equal(t, byte('X'), result[i], "content should be preserved")
		}
	})
}
