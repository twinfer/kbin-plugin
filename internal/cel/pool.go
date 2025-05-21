// pool.go
package cel

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/twinfer/kbin-plugin/pkg/expression"
)

// ExpressionPool caches compiled CEL expressions
type ExpressionPool struct {
	mu          sync.RWMutex
	expressions map[string]cel.Program
	env         *cel.Env
}

// NewExpressionPool creates a new expression pool with a configured CEL environment
func NewExpressionPool() (*ExpressionPool, error) {
	// Use the existing NewEnvironment function instead of creating a new one
	env, err := NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return &ExpressionPool{
		env:         env,
		expressions: make(map[string]cel.Program),
	}, nil
}

// GetExpression retrieves or compiles an expression
func (e *ExpressionPool) GetExpression(exprStr string) (cel.Program, error) {
	e.mu.RLock()
	if program, ok := e.expressions[exprStr]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// 1. Parse Kaitai expression string to Kaitai AST
	lexer := expression.NewExpressionLexer(strings.NewReader(exprStr))
	parser := expression.NewExpressionParser(lexer)
	kaitaiAST, pErr := parser.Parse()
	if pErr != nil {
		return nil, fmt.Errorf("failed to parse Kaitai expression '%s': %w. Parser errors: %s", exprStr, pErr, strings.Join(parser.Errors(), "; "))
	}

	// 2. Transform Kaitai AST to CEL string using ASTTransformer
	transformer := NewASTTransformer() // Assuming NewASTTransformer is in the same 'cel' package
	celExprStr, tErr := transformer.Transform(kaitaiAST)
	if tErr != nil {
		return nil, fmt.Errorf("failed to transform Kaitai AST to CEL for expression '%s': %w", exprStr, tErr)
	}

	transformed := celExprStr // Use the CEL string from ASTTransformer

	// Extract variable names from the transformed expression
	vars := extractVariables(transformed)

	// Create environment with these variables
	envOpts := []cel.EnvOption{}
	for _, varName := range vars {
		envOpts = append(envOpts, cel.Variable(varName, cel.DynType))
	}

	// Extend the base environment with the variables
	extEnv, err := e.env.Extend(envOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to extend environment: %w", err)
	}

	// Parse and check the expression
	ast, issues := extEnv.Compile(transformed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	// Create a Program from the AST
	program, err := extEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create program: %w", err)
	}

	// Cache the program
	e.mu.Lock()
	e.expressions[exprStr] = program
	e.mu.Unlock()

	return program, nil
}

// EvaluateExpression evaluates a compiled expression with parameters
func (e *ExpressionPool) EvaluateExpression(program cel.Program, params map[string]any) (any, error) {
	// Create empty activation if params is nil
	if params == nil {
		params = make(map[string]any)
	}

	// Convert all parameters to CEL ref.Val types
	activation, err := cel.NewActivation(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create activation: %w", err)
	}

	// Evaluate using the CEL program
	val, _, err := program.Eval(activation)
	if err != nil {
		return nil, fmt.Errorf("expression evaluation error: %w", err)
	}

	// Convert result back to Go native type
	return adaptCELResult(val.Value()), nil
}

// adaptCELResult converts CEL result values to Go native types
func adaptCELResult(val any) any {
	switch v := val.(type) {
	case types.Int:
		return float64(v) // Use float64 for all numbers for consistency
	case types.Uint:
		return float64(v)
	case types.Double:
		return float64(v)
	case types.Bool:
		return bool(v)
	case types.String:
		return string(v)
	case types.Bytes:
		return []byte(v)
	case types.Null:
		return nil
	case ref.Val:
		// Handle lists
		if lister, ok := v.(traits.Lister); ok {
			size := lister.Size().(types.Int)
			result := make([]any, size)
			for i := types.Int(0); i < size; i++ {
				item := lister.Get(types.Int(i))
				result[i] = adaptCELResult(item.Value())
			}
			return result
		}

		// Handle maps
		if mapper, ok := v.(traits.Mapper); ok {
			result := make(map[string]any)
			iter := mapper.Iterator()
			for iter.HasNext() == types.True {
				key := iter.Next()
				val := mapper.Get(key)
				keyStr, ok := key.Value().(string)
				if !ok {
					keyStr = fmt.Sprintf("%v", key.Value())
				}
				result[keyStr] = adaptCELResult(val.Value())
			}
			return result
		}

		// For any other ref.Val, unwrap the value
		return v.Value()
	default:
		// For any other type, return as is
		return v
	}
}

// extractVariables analyzes an expression to find variable references
func extractVariables(expr string) []string {
	// Simple implementation that looks for word-like tokens
	var vars []string
	varSet := make(map[string]bool)

	// Skip known function names and keywords
	keywords := map[string]bool{
		"true": true, "false": true, "null": true, // Keep basic literals/keywords
		"to_s": true, "to_i": true, "to_f": true,
		"bitAnd": true, "bitOr": true, "bitXor": true, "bitNot": true,
		"bitShiftLeft": true, "bitShiftRight": true,
		"mul": true, "add": true, "pow": true, "mod": true,
		"startsWith": true, "endsWith": true, "contains": true,
		"substring": true, "reverse": true,
		"at": true, "slice": true, "sliceEnd": true, "sliceRange": true,
		"size": true, "length": true, "count": true, // These are also function names
		"abs": true, "min": true, "max": true, "ceil": true, "floor": true, "round": true, // Math functions
		"processXOR": true, "processZlib": true, "processRotateLeft": true, "processRotateRight": true, // Process functions
		"encodeString": true, "decodeString": true, // Encoding functions
		"writerPos": true, "writeBytes": true, "writeU1": true, "writeU2le": true, "writeU4le": true, "writeU8le": true, "writeS1": true, "writeS2le": true, "writeS4le": true, "writeS8le": true, "writeF4le": true, "writeF8le": true, "writeU2be": true, "writeU4be": true, "writeU8be": true, "writeF4be": true, "writeF8be": true, "write": true, "newWriter": true, "writerBuffer": true, // Writer functions
		"input": true, "_io": true, "ternary": true,
	}

	// Simple regex-free tokenizer
	inWord := false
	start := 0
	for i, c := range expr {
		isWordChar := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'

		if isWordChar && !inWord {
			inWord = true
			start = i
		} else if !isWordChar && inWord {
			inWord = false
			word := expr[start:i]

			// Skip numbers, keywords and already seen variables
			if _, isKeyword := keywords[word]; !isKeyword && !varSet[word] && !(word[0] >= '0' && word[0] <= '9') {
				varSet[word] = true
				vars = append(vars, word)
			}
		}
	}

	// Check the last word if the string ends with a word
	if inWord {
		word := expr[start:]
		if _, isKeyword := keywords[word]; !isKeyword && !varSet[word] && !(word[0] >= '0' && word[0] <= '9') {
			vars = append(vars, word)
		}
	}

	return vars
}

// ConvertToRefVal converts a Go value to a CEL ref.Val
func ConvertToRefVal(val any) ref.Val {
	return types.DefaultTypeAdapter.NativeToValue(val)
}

// ConvertFromRefVal converts a CEL ref.Val to a Go value
func ConvertFromRefVal(val ref.Val) (any, error) {
	if val == nil {
		return nil, nil
	}

	if types.IsError(val) {
		return nil, fmt.Errorf("CEL error: %v", val)
	}

	if types.IsUnknown(val) {
		return nil, fmt.Errorf("unknown CEL value")
	}

	return adaptCELResult(val.Value()), nil
}
