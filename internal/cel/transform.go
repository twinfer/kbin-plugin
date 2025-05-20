// transformer.go
package cel

import (
	"regexp"
	"strings"
)

// TransformKaitaiExpression transforms Kaitai expression syntax to CEL syntax.
func TransformKaitaiExpression(expr string) string {
	// Stage 1: Handle property access before complex operations
	expr = replaceRegex(expr, `(\w+)\.length\b`, `length($1)`)
	expr = replaceRegex(expr, `(\w+)\.size\b`, `size($1)`)
	expr = replaceRegex(expr, `(\w+)\.count\b`, `count($1)`)

	// Stage 2: Handle bitwise NOT
	expr = replaceRegex(expr, `~(\w+)`, `bitNot($1)`)

	// Stage 3: Array slicing operations
	expr = replaceRegex(expr, `(\w+)\[(\d+|\w+):(\d+|\w+)\]`, `sliceRange($1, $2, $3)`)
	expr = replaceRegex(expr, `(\w+)\[(\d+|\w+):\]`, `slice($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\[:(\d+|\w+)\]`, `sliceEnd($1, $2)`)

	// Stage 4: Handle specific complex expressions
	expr = replaceRegex(expr, `\((\w+)\s*&\s*([^)]+)\)\s*\|\s*\(\((\w+)\s*<<\s*(\d+|\w+)\)\s*&\s*([^)]+)\)`,
		`bitOr(bitAnd($1, $2), bitAnd(bitShiftLeft($3, $4), $5))`)

	// Stage 5: Math operations
	expr = replaceRegex(expr, `(\w+|\d+)\s*\*\*\s*(\w+|\d+)`, `pow($1, $2)`)
	expr = replaceRegex(expr, `(\w+|\d+)\s*%\s*(\w+|\d+)`, `mod($1, $2)`)
	expr = replaceRegex(expr, `length\((\w+)\)\s*\*\s*(\d+)`, `mul(length($1), $2)`)
	expr = replaceRegex(expr, `mul\(length\((\w+)\),\s*(\d+)\)\s*\+\s*(\w+)`, `add(mul(length($1), $2), $3)`)
	expr = replaceRegex(expr, `(\w+|\d+)\s*\*\s*(\w+|\d+)`, `mul($1, $2)`)
	expr = replaceRegex(expr, `(\w+|\d+)\s*\+\s*(\w+|\d+)`, `add($1, $2)`)

	// Stage 6: Method calls with special naming
	expr = replaceRegex(expr, `(\w+)\.starts_with\(([^)]*)\)`, `startsWith($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\.ends_with\(([^)]*)\)`, `endsWith($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\.read_bytes\(([^)]*)\)`, `readBytes($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\.read_bytes_full\(\)`, `readBytesFull($1)`)

	// Stage 7: Fix method calls with property access
	expr = replaceRegex(expr, `substring\((\w+),\s*([^)]+)\)\.length`, `length(substring($1, $2))`)

	// Stage 8: Special IO methods
	expr = replaceRegex(expr, `_io\.read_(\w+)\(\)`, `read_$1(_io)`)
	expr = replaceRegex(expr, `_io\.pos\(\)`, `pos(_io)`)
	expr = replaceRegex(expr, `_io\.size\(\)`, `size(_io)`)
	expr = replaceRegex(expr, `_io\.seek\(([^)]+)\)`, `seek(_io, $1)`)

	// Stage 9: Handle property chains (3+ levels)
	expr = replaceRegex(expr, `(\w+)\.(\w+)\.(\w+)\.(\w+)`, `$4($3($2($1)))`)
	expr = replaceRegex(expr, `(\w+)\.(\w+)\.(\w+)`, `$3($2($1))`)

	// Stage 10: Handle bitwise operations with parentheses
	expr = replaceRegex(expr, `\((\w+)\s*&\s*(\w+)\)\s*==\s*(\d+)`, `bitAnd($1, $2) == $3`)
	expr = replaceRegex(expr, `\(([^()]+)\)\s*>>\s*(\d+|\w+)`, `bitShiftRight($1, $2)`)
	expr = replaceRegex(expr, `\(([^()]+)\)\s*<<\s*(\d+|\w+)`, `bitShiftLeft($1, $2)`)
	expr = replaceRegex(expr, `\(([^()]+)\)\s*&\s*([^()]+)`, `bitAnd($1, $2)`)
	expr = replaceRegex(expr, `\(([^()]+)\)\s*\|\s*([^()]+)`, `bitOr($1, $2)`)

	// Stage 11: Simple bitwise operations
	expr = replaceRegex(expr, `(\w+)\s*&\s*(0x[0-9A-Fa-f]+|\d+|\w+)`, `bitAnd($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\s*\|\s*(0x[0-9A-Fa-f]+|\d+|\w+)`, `bitOr($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\s*\^\s*(0x[0-9A-Fa-f]+|\d+|\w+)`, `bitXor($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\s*>>\s*(\d+|\w+)`, `bitShiftRight($1, $2)`)
	expr = replaceRegex(expr, `(\w+)\s*<<\s*(\d+|\w+)`, `bitShiftLeft($1, $2)`)

	// Stage 12: Handle array indexing (after bitwise operations)
	expr = replaceRegex(expr, `(\w+)\[(\d+|\w+)\]\[(\d+|\w+)\]`, `at(at($1, $2), $3)`)
	expr = replaceRegex(expr, `(\w+)\[(\d+|\w+)\]`, `at($1, $2)`)

	// Stage 13: Handle the ternary operator
	expr = regexp.MustCompile(`([^?:]+)\?([^?:]+):([^?:]+)`).ReplaceAllStringFunc(expr, func(match string) string {
		parts := regexp.MustCompile(`([^?:]+)\?([^?:]+):([^?:]+)`).FindStringSubmatch(match)
		condition := strings.TrimSpace(parts[1])
		// Remove unnecessary parentheses
		condition = strings.ReplaceAll(condition, "(bitAnd", "bitAnd")
		condition = strings.ReplaceAll(condition, "))", ")")
		trueVal := strings.TrimSpace(parts[2])
		falseVal := strings.TrimSpace(parts[3])
		return "ternary(" + condition + ", " + trueVal + ", " + falseVal + ")"
	})

	// Stage 14: Simple method calls
	expr = replaceRegex(expr, `(\w+)\.to_s\(\)`, `to_s($1)`)
	expr = replaceRegex(expr, `(\w+)\.to_i\(\)`, `to_i($1)`)
	expr = replaceRegex(expr, `(\w+)\.to_f\(\)`, `to_f($1)`)
	expr = replaceRegex(expr, `(\w+)\.reverse\(\)`, `reverse($1)`)

	// Stage 15: Method calls with arguments
	expr = replaceRegex(expr, `(\w+)\.(\w+)\(([^)]*)\)`, `$2($1, $3)`)

	// Stage 16: Generic method calls
	expr = replaceRegex(expr, `(\w+)\.(\w+)\(\)`, `$2($1)`)

	// Stage 17: Generic property access (must be last)
	expr = replaceRegex(expr, `(\w+)\.(\w+)`, `$2($1)`)

	return expr
}

// Helper function for regex replacements
func replaceRegex(input, pattern, replacement string) string {
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(input, replacement)
}
