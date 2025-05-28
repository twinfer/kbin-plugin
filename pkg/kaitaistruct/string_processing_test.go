package kaitaistruct

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessStringBytes(t *testing.T) {
	// Create a KaitaiInterpreter instance for testing
	k := &KaitaiInterpreter{}

	t.Run("no_processing", func(t *testing.T) {
		// Test with no terminator or padding specified
		field := SequenceItem{
			ID:   "test",
			Type: "str",
		}
		input := []byte("hello world")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello world"), result)
	})

	t.Run("terminator_only", func(t *testing.T) {
		// Test terminator without include
		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: 0x40, // '@'
		}
		input := []byte("hello@world")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), result, "should stop at terminator and exclude it")
	})

	t.Run("terminator_include", func(t *testing.T) {
		// Test terminator with include=true
		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: 0x40, // '@'
			Include:    true,
		}
		input := []byte("hello@world")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello@"), result, "should stop at terminator and include it")
	})

	t.Run("terminator_not_found", func(t *testing.T) {
		// Test when terminator is not found
		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: 0xFF, // not in data
		}
		input := []byte("hello world")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello world"), result, "should return full data when terminator not found")
	})

	t.Run("padding_only", func(t *testing.T) {
		// Test padding removal without terminator
		field := SequenceItem{
			ID:       "test",
			Type:     "str",
			PadRight: 0x40, // '@'
		}
		input := []byte("hello@@@@@")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), result, "should remove trailing padding")
	})

	t.Run("padding_mixed_content", func(t *testing.T) {
		// Test padding removal where content contains pad character
		field := SequenceItem{
			ID:       "test",
			Type:     "str",
			PadRight: 0x2B, // '+'
		}
		input := []byte("a+b+c+++++")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("a+b+c"), result, "should only remove trailing padding")
	})

	t.Run("terminator_and_padding", func(t *testing.T) {
		// Test terminator with padding - padding should be ignored when terminator found
		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: 0x40, // '@'
			PadRight:   0x2B, // '+'
		}
		input := []byte("str+++3bar+++@++++++")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte("str+++3bar+++"), result, "should stop at terminator, ignore padding after")
	})

	t.Run("terminator_at_start", func(t *testing.T) {
		// Test terminator at the beginning
		field := SequenceItem{
			ID:         "test",
			Type:       "str",
			Terminator: 0x00, // null
		}
		input := []byte("\x00hello")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte(""), result, "should return empty string when terminator at start")
	})

	t.Run("all_padding", func(t *testing.T) {
		// Test string that is all padding
		field := SequenceItem{
			ID:       "test",
			Type:     "str",
			PadRight: 0x20, // space
		}
		input := []byte("     ")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte(""), result, "should return empty string when all padding")
	})

	t.Run("empty_input", func(t *testing.T) {
		// Test empty input
		field := SequenceItem{
			ID:       "test",
			Type:     "str",
			PadRight: 0x40,
		}
		input := []byte("")
		
		result, err := k.processStringBytes(input, field)
		require.NoError(t, err)
		assert.Equal(t, []byte(""), result, "should handle empty input")
	})
}

func TestExtractByteValue(t *testing.T) {
	k := &KaitaiInterpreter{}

	t.Run("int_values", func(t *testing.T) {
		testCases := []struct {
			input    any
			expected byte
			shouldErr bool
		}{
			{64, 64, false},
			{0, 0, false},
			{255, 255, false},
			{256, 0, true},  // out of range
			{-1, 0, true},   // negative
		}

		for _, tc := range testCases {
			result, err := k.extractByteValue(tc.input)
			if tc.shouldErr {
				assert.Error(t, err, "input %v should cause error", tc.input)
			} else {
				require.NoError(t, err, "input %v should not cause error", tc.input)
				assert.Equal(t, tc.expected, result, "input %v should produce %d", tc.input, tc.expected)
			}
		}
	})

	t.Run("different_int_types", func(t *testing.T) {
		testCases := []struct {
			input    any
			expected byte
		}{
			{int8(64), 64},
			{int16(64), 64},
			{int32(64), 64},
			{int64(64), 64},
			{uint8(64), 64},
			{uint16(64), 64},
			{uint32(64), 64},
			{uint64(64), 64},
		}

		for _, tc := range testCases {
			result, err := k.extractByteValue(tc.input)
			require.NoError(t, err, "input %T(%v) should not cause error", tc.input, tc.input)
			assert.Equal(t, tc.expected, result, "input %T(%v) should produce %d", tc.input, tc.input, tc.expected)
		}
	})

	t.Run("float_values", func(t *testing.T) {
		testCases := []struct {
			input     any
			expected  byte
			shouldErr bool
		}{
			{float32(64.0), 64, false},
			{float64(64.0), 64, false},
			{float32(64.5), 0, true},   // fractional
			{float64(64.7), 0, true},   // fractional
			{float32(-1.0), 0, true},   // negative
			{float64(256.0), 0, true},  // out of range
		}

		for _, tc := range testCases {
			result, err := k.extractByteValue(tc.input)
			if tc.shouldErr {
				assert.Error(t, err, "input %v should cause error", tc.input)
			} else {
				require.NoError(t, err, "input %v should not cause error", tc.input)
				assert.Equal(t, tc.expected, result, "input %v should produce %d", tc.input, tc.expected)
			}
		}
	})

	t.Run("hex_strings", func(t *testing.T) {
		testCases := []struct {
			input     string
			expected  byte
			shouldErr bool
		}{
			{"0x40", 64, false},
			{"0x00", 0, false},
			{"0xFF", 255, false},
			{"0xff", 255, false},  // lowercase
			{"0x100", 0, true},    // out of range
			{"0xZZ", 0, true},     // invalid hex
			{"40", 0, true},       // missing 0x prefix
			{"not_hex", 0, true},  // not hex
		}

		for _, tc := range testCases {
			result, err := k.extractByteValue(tc.input)
			if tc.shouldErr {
				assert.Error(t, err, "input %s should cause error", tc.input)
			} else {
				require.NoError(t, err, "input %s should not cause error", tc.input)
				assert.Equal(t, tc.expected, result, "input %s should produce %d", tc.input, tc.expected)
			}
		}
	})

	t.Run("unsupported_types", func(t *testing.T) {
		unsupportedValues := []any{
			[]byte{64},
			map[string]int{"test": 64},
			struct{ val int }{64},
			nil,
		}

		for _, val := range unsupportedValues {
			_, err := k.extractByteValue(val)
			assert.Error(t, err, "input %T should cause error", val)
		}
	})
}

func TestStringProcessingIntegration(t *testing.T) {
	// Integration tests that combine multiple features
	k := &KaitaiInterpreter{}

	t.Run("kaitai_test_suite_str_pad_term", func(t *testing.T) {
		// Test cases based on the actual Kaitai test suite str_pad_term.ksy
		testCases := []struct {
			name     string
			field    SequenceItem
			input    []byte
			expected []byte
		}{
			{
				name: "str_pad",
				field: SequenceItem{
					ID:       "str_pad",
					Type:     "str",
					PadRight: 0x40, // '@'
				},
				input:    []byte("str1@@@@@@@@@@@@@@@@"),
				expected: []byte("str1"),
			},
			{
				name: "str_term", 
				field: SequenceItem{
					ID:         "str_term",
					Type:       "str",
					Terminator: 0x40, // '@'
				},
				input:    []byte("str2foo@++++++++++++"),
				expected: []byte("str2foo"),
			},
			{
				name: "str_term_and_pad",
				field: SequenceItem{
					ID:         "str_term_and_pad",
					Type:       "str",
					Terminator: 0x40, // '@'
					PadRight:   0x2B, // '+'
				},
				input:    []byte("str+++3bar+++@++++++"),
				expected: []byte("str+++3bar+++"),
			},
			{
				name: "str_term_include",
				field: SequenceItem{
					ID:         "str_term_include",
					Type:       "str",
					Terminator: 0x40, // '@'
					Include:    true,
				},
				input:    []byte("str4baz@............"),
				expected: []byte("str4baz@"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := k.processStringBytes(tc.input, tc.field)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result, 
					"field %s should process correctly", tc.name)
			})
		}
	})

	t.Run("complex_edge_cases", func(t *testing.T) {
		testCases := []struct {
			name     string
			field    SequenceItem
			input    []byte
			expected []byte
		}{
			{
				name: "multiple_terminators",
				field: SequenceItem{
					ID:         "test",
					Type:       "str", 
					Terminator: 0x7C, // '|'
				},
				input:    []byte("first|second|third"),
				expected: []byte("first"),
			},
			{
				name: "terminator_same_as_padding",
				field: SequenceItem{
					ID:         "test",
					Type:       "str",
					Terminator: 0x2B, // '+'
					PadRight:   0x2B, // '+' same as terminator
				},
				input:    []byte("text++++"),
				expected: []byte("text"),
			},
			{
				name: "unicode_like_bytes",
				field: SequenceItem{
					ID:       "test",
					Type:     "str",
					PadRight: 0x00, // null padding
				},
				input:    []byte("hello\x00\x00\x00"),
				expected: []byte("hello"),
			},
			{
				name: "binary_data_with_terminator",
				field: SequenceItem{
					ID:         "test",
					Type:       "str",
					Terminator: 0x00, // null terminator
				},
				input:    []byte("\xFF\xFE\x00\x01\x02"),
				expected: []byte("\xFF\xFE"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := k.processStringBytes(tc.input, tc.field)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result,
					"case %s should process correctly", tc.name)
			})
		}
	})
}