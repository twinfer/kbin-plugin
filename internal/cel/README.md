# Google CEL Integration for Kaitai Parser

This directory contains the implementation of Google Common Expression Language (CEL) for the Kaitai parser. The goal is to replace the existing `expr-lang/expr` package with CEL while maintaining the same API.

## Structure

- `environment.go`: Sets up the CEL environment with all the required functions for Kaitai expressions
- `transform.go`: Handles the transformation of Kaitai expression syntax to CEL syntax
- `pool.go`: Implements the expression pool for caching and evaluating compiled expressions
- `cel_test.go`: Contains tests for the CEL implementation

## Usage

The CEL implementation is designed to be a drop-in replacement for the existing expression evaluation in the Kaitai parser.

```go
// Create a new CEL expression pool
celPool, err := cel.NewExpressionPool()
if err != nil {
    // Handle error
}

// Compile an expression
program, err := celPool.GetExpression("a + b")
if err != nil {
    // Handle error
}

// Evaluate with parameters
params := map[string]interface{}{
    "a": 5,
    "b": 3,
}

result, err := celPool.EvaluateExpression(program, params)
if err != nil {
    // Handle error
}
```

## Key Features

- Full support for Kaitai expressions
- Improved type safety with CEL's gradual typing
- Better performance and optimized evaluation
- Enhanced error messages

## Implementation Details

The implementation wraps the Google CEL library and adapts it to match the existing API. It:

1. Transforms Kaitai expressions to CEL syntax
2. Compiles expressions using the CEL environment
3. Evaluates expressions with provided parameters
4. Adapts results to match the expected output format

## Tests

The implementation includes tests that verify:
- Correct transformation of Kaitai expressions to CEL syntax
- Proper evaluation of expressions
- Compatibility with the existing expr-based implementation

## Benchmarks

Benchmarks are included to compare the performance of CEL with the original expr implementation:

```
go test -bench=BenchmarkExpressionEvaluation -benchmem
```
