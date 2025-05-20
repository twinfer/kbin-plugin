package kaitaistruct

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	internalcel "github.com/twinfer/kbin-plugin/internal/cel"
)

// CELExpressionPool caches compiled CEL expressions
type CELExpressionPool struct {
	mu          sync.RWMutex
	expressions map[string]cel.Program // Remove pointer
	celPool     *internalcel.ExpressionPool
}

// NewCELExpressionPool creates a new expression pool
func NewCELExpressionPool() (*CELExpressionPool, error) {
	celPool, err := internalcel.NewExpressionPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL expression pool: %w", err)
	}

	return &CELExpressionPool{
		expressions: make(map[string]cel.Program),
		celPool:     celPool,
	}, nil
}

// GetExpression gets or compiles an expression
func (e *CELExpressionPool) GetExpression(exprStr string) (cel.Program, error) {
	e.mu.RLock()
	if program, ok := e.expressions[exprStr]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// Transform Kaitai expressions to CEL syntax and compile
	program, err := e.celPool.GetExpression(exprStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", err)
	}

	e.mu.Lock()
	e.expressions[exprStr] = program
	e.mu.Unlock()

	return program, nil
}

// EvaluateExpression evaluates a compiled expression with parameters
func (e *CELExpressionPool) EvaluateExpression(program cel.Program, params map[string]any) (any, error) {
	return e.celPool.EvaluateExpression(program, params)
}
