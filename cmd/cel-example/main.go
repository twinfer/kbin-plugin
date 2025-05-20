package main

import (
	"fmt"
	"log"

	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
)

func main() {
	// Example usage of the new CEL-based expression pool
	celPool, err := kaitaistruct.NewCELExpressionPool()
	if err != nil {
		log.Fatalf("Failed to create CEL expression pool: %v", err)
	}

	// Test a simple expression
	expr := "a + b"
	program, err := celPool.GetExpression(expr)
	if err != nil {
		log.Fatalf("Failed to compile expression: %v", err)
	}

	// Evaluate with parameters
	params := map[string]interface{}{
		"a": 5,
		"b": 3,
	}

	result, err := celPool.EvaluateExpression(program, params)
	if err != nil {
		log.Fatalf("Failed to evaluate expression: %v", err)
	}

	fmt.Printf("Result of '%s' with a=5, b=3: %v\n", expr, result)

	// Test a more complex bitwise expression
	bitwiseExpr := "(a & 0xFF) >> 2"
	bitwiseProgram, err := celPool.GetExpression(bitwiseExpr)
	if err != nil {
		log.Fatalf("Failed to compile bitwise expression: %v", err)
	}

	bitwiseResult, err := celPool.EvaluateExpression(bitwiseProgram, params)
	if err != nil {
		log.Fatalf("Failed to evaluate bitwise expression: %v", err)
	}

	fmt.Printf("Result of '%s' with a=5: %v\n", bitwiseExpr, bitwiseResult)
}
