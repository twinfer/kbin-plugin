package kaitaistruct

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

// TODO: consildate with cel #package cel process.go, also KaitaiStructruntime util.go already has
// process functions
// Register and manage process functions (like XOR, zlib, rotate)
// Allow dynamic registration of custom process functions
// Handle process parameter parsing
// Support both forward processing (for parsing) and reverse processing (for serialization)
// ProcessFunc defines a function that processes byte data
type ProcessFunc func(data []byte, params []any) ([]byte, error)

// ProcessRegistry manages processing functions for binary data transformations
type ProcessRegistry struct {
	functions map[string]ProcessFunc
}

// NewProcessRegistry creates a new registry with default process functions
func NewProcessRegistry() *ProcessRegistry {
	registry := &ProcessRegistry{
		functions: make(map[string]ProcessFunc),
	}

	// Register default processors
	registry.Register("xor", processXOR)
	registry.Register("zlib", processZlib)
	registry.Register("rotate", processRotate)

	return registry
}

// Register adds a new process function to the registry
func (r *ProcessRegistry) Register(name string, fn ProcessFunc) {
	r.functions[name] = fn
}

// Get retrieves a process function by name
func (r *ProcessRegistry) Get(name string) (ProcessFunc, bool) {
	fn, exists := r.functions[name]
	return fn, exists
}

// ProcessData applies a processing function to data
func ProcessData(data []byte, processSpec string, registry *ProcessRegistry) ([]byte, error) {
	// Parse the process spec (e.g., "xor(0x5F)" or "xor([0x5F, 0x10])")
	funcName, params, err := parseProcessSpec(processSpec)
	if err != nil {
		return nil, fmt.Errorf("invalid process specification: %w", err)
	}

	// Get the process function
	processFn, exists := registry.Get(funcName)
	if !exists {
		return nil, fmt.Errorf("unknown process function: %s", funcName)
	}

	// Apply the process
	return processFn(data, params)
}

// Helper function to parse a process specification string
func parseProcessSpec(processSpec string) (string, []any, error) {
	// Extract function name and parameter string
	openParenIndex := strings.Index(processSpec, "(")
	closeParenIndex := strings.LastIndex(processSpec, ")")

	if openParenIndex == -1 || closeParenIndex == -1 || closeParenIndex < openParenIndex {
		return "", nil, fmt.Errorf("invalid process format: %s", processSpec)
	}

	funcName := strings.TrimSpace(processSpec[:openParenIndex])
	paramStr := strings.TrimSpace(processSpec[openParenIndex+1 : closeParenIndex])

	// Parse parameters
	var params []any

	// Handle array parameter
	if strings.HasPrefix(paramStr, "[") && strings.HasSuffix(paramStr, "]") {
		// Parse array of values
		arrayStr := paramStr[1 : len(paramStr)-1]
		parts := strings.Split(arrayStr, ",")

		for _, part := range parts {
			part = strings.TrimSpace(part)
			param, err := parseParam(part)
			if err != nil {
				return "", nil, err
			}
			params = append(params, param)
		}
	} else if paramStr != "" {
		// Parse single parameter
		param, err := parseParam(paramStr)
		if err != nil {
			return "", nil, err
		}
		params = append(params, param)
	}

	return funcName, params, nil
}

// Helper function to parse a parameter string
func parseParam(paramStr string) (any, error) {
	// Try to parse as hex
	if strings.HasPrefix(paramStr, "0x") || strings.HasPrefix(paramStr, "0X") {
		val, err := strconv.ParseInt(paramStr[2:], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid hex parameter: %s", paramStr)
		}
		return val, nil
	}

	// Try to parse as integer
	if val, err := strconv.ParseInt(paramStr, 10, 64); err == nil {
		return val, nil
	}

	// Try to parse as float
	if val, err := strconv.ParseFloat(paramStr, 64); err == nil {
		return val, nil
	}

	// Return as string
	return paramStr, nil
}

// XOR process implementation
func processXOR(data []byte, params []any) ([]byte, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("xor process requires at least one parameter")
	}

	// Handle single key value
	if len(params) == 1 {
		var key byte

		switch v := params[0].(type) {
		case int64:
			key = byte(v)
		case float64:
			key = byte(v)
		default:
			return nil, fmt.Errorf("invalid xor key type: %T", params[0])
		}

		// Create key as single byte
		keyBytes := []byte{key}
		return kaitai.ProcessXOR(data, keyBytes), nil
	}

	// Handle multiple byte values
	keyBytes := make([]byte, len(params))
	for i, param := range params {
		switch v := param.(type) {
		case int64:
			keyBytes[i] = byte(v)
		case float64:
			keyBytes[i] = byte(v)
		default:
			return nil, fmt.Errorf("invalid xor key type at index %d: %T", i, param)
		}
	}

	return kaitai.ProcessXOR(data, keyBytes), nil
}

// Zlib process implementation
func processZlib(data []byte, params []any) ([]byte, error) {
	return kaitai.ProcessZlib(data)
}

// Rotate process implementation
func processRotate(data []byte, params []any) ([]byte, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("rotate process requires exactly one parameter")
	}

	var amount int
	switch v := params[0].(type) {
	case int64:
		amount = int(v)
	case float64:
		amount = int(v)
	default:
		return nil, fmt.Errorf("invalid rotate amount type: %T", params[0])
	}

	if amount >= 0 {
		return kaitai.ProcessRotateLeft(data, amount), nil
	} else {
		return kaitai.ProcessRotateRight(data, -amount), nil
	}
}
