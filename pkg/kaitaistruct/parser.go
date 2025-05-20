package kaitaistruct

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"maps"
	"slices"

	"github.com/google/cel-go/cel"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	internalCel "github.com/twinfer/kbin-plugin/internal/cel"
)

// KaitaiInterpreter provides dynamic parsing of binary data using a Kaitai schema
type KaitaiInterpreter struct {
	schema          *KaitaiSchema
	expressionPool  *internalCel.ExpressionPool
	typeStack       []string        // Stack of type names being processed
	valueStack      []*ParseContext // Stack of parent values for expression evaluation
	logger          *slog.Logger
	processRegistry *ProcessRegistry // Registry of process handlers
}

// ParseContext contains the context for parsing a particular section
type ParseContext struct {
	Value    any            // Current value being processed
	Parent   *ParseContext  // Parent context
	Root     *ParseContext  // Root context of the parse tree
	IO       *kaitai.Stream // Current IO stream
	Children map[string]any // Map of child fields
}

// ParsedData represents the result of parsing a binary stream with the schema
type ParsedData struct {
	Value    any
	Children map[string]*ParsedData
	Type     string
	IsArray  bool
}

// NewKaitaiInterpreter creates a new interpreter for a given schema
func NewKaitaiInterpreter(schema *KaitaiSchema, logger *slog.Logger) (*KaitaiInterpreter, error) {
	// Create expression pool with our enhanced CEL environment
	pool, err := internalCel.NewExpressionPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create expression pool: %w", err)
	}

	log := logger
	if log == nil {
		log = slog.Default()
	}

	return &KaitaiInterpreter{
		schema:          schema,
		expressionPool:  pool,
		typeStack:       make([]string, 0),
		valueStack:      make([]*ParseContext, 0),
		logger:          log,
		processRegistry: NewProcessRegistry(),
	}, nil
}

// AsActivation creates a CEL activation from the parse context
func (ctx *ParseContext) AsActivation() (cel.Activation, error) {
	// Create map of variables for CEL
	vars := make(map[string]any)

	// Add current context values
	if ctx.Children != nil {
		maps.Copy(vars, ctx.Children)
	}

	// Add special variables
	vars["_io"] = ctx.IO
	if ctx.Root != nil {
		vars["_root"] = ctx.Root.Children
	}
	if ctx.Parent != nil {
		vars["_parent"] = ctx.Parent.Children
	}

	return cel.NewActivation(vars)
}

// Parse parses binary data according to the schema
func (k *KaitaiInterpreter) Parse(ctx context.Context, stream *kaitai.Stream) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Starting Kaitai parsing", "root_type_meta", k.schema.Meta.ID, "root_type_schema", k.schema.RootType)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Create root context
	rootCtx := &ParseContext{
		Children: make(map[string]any),
		IO:       stream,
	}
	rootCtx.Root = rootCtx

	// Push root context
	k.valueStack = append(k.valueStack, rootCtx)

	// Parse root type
	rootType := k.schema.Meta.ID
	if k.schema.RootType != "" {
		rootType = k.schema.RootType
	}

	// Parse according to root type
	result, err := k.parseType(ctx, rootType, stream)
	if err != nil {
		return nil, fmt.Errorf("failed parsing root type '%s': %w", rootType, err)
	}

	// Copy parsed fields to rootCtx.Children for instance evaluation
	for k, v := range result.Children {
		rootCtx.Children[k] = v.Value
	}

	// Process instances if any
	//kaitai instance is a special case, we need to evaluate them after parsing the root type
	// This is because instances can reference fields that are only available after parsing the root type
	if k.schema.Instances != nil {
		for name, inst := range k.schema.Instances { // TODO: Pass ctx to evaluateInstance
			val, err := k.evaluateInstance(inst, rootCtx)
			if err != nil {
				return nil, fmt.Errorf("failed evaluating instance '%s': %w", name, err)
			}
			result.Children[name] = val
		}
	}

	k.logger.DebugContext(ctx, "Finished Kaitai parsing")
	return result, nil
}

// parseType parses a Kaitai type from the stream
func (k *KaitaiInterpreter) parseType(ctx context.Context, typeName string, stream *kaitai.Stream) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Parsing type", "type_name", typeName, "current_stack", strings.Join(k.typeStack, " -> "))
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Check for circular dependency
	if slices.Contains(k.typeStack, typeName) {
		k.logger.ErrorContext(ctx, "Circular type dependency detected", "type_name", typeName, "stack", strings.Join(k.typeStack, " -> "))
		return nil, fmt.Errorf("circular type dependency detected: %s", typeName)
	}

	// Push current type to stack
	k.typeStack = append(k.typeStack, typeName)
	defer func() {
		// Pop current type from stack when done
		k.logger.DebugContext(ctx, "Finished parsing type", "type_name", typeName)
		k.typeStack = k.typeStack[:len(k.typeStack)-1]
	}()

	// Create result structure
	result := &ParsedData{
		Children: make(map[string]*ParsedData),
		Type:     typeName,
	}

	// Check if it's a built-in type
	if parsedData, handled, err := k.parseBuiltinType(ctx, typeName, stream); handled {
		if err != nil {
			return nil, fmt.Errorf("parsing built-in type '%s': %w", typeName, err)
		}
		return parsedData, nil
	}

	// Check if it's a switch type
	if strings.Contains(typeName, "switch-on:") {
		// Parse switch type format: switch-on:expression:default_type
		parts := strings.Split(typeName, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid switch type format: %s", typeName)
		}

		// Create context for evaluating switch expression
		evalCtx := &ParseContext{ // Renamed from ctx to evalCtx to avoid shadowing the Go context
			Children: make(map[string]any),
			IO:       stream,
			Parent:   k.valueStack[len(k.valueStack)-1],
			Root:     k.valueStack[0].Root,
		}

		// Evaluate switch expression using CEL, passing the Go context `ctx`
		switchExpr := parts[1]
		switchValue, err := k.evaluateExpression(context.Background(), switchExpr, evalCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate switch expression '%s' for type '%s': %w", switchExpr, typeName, err)
		}

		// Determine actual type based on switch value
		var actualType string
		// TODO: Implement switch cases mapping
		// For now, use default type if specified
		if len(parts) > 2 {
			actualType = parts[2]
		} else {
			k.logger.ErrorContext(ctx, "No matching case for switch value", "type_name", typeName, "switch_on_expr", switchExpr, "switch_value", switchValue)
			return nil, fmt.Errorf("no matching case for switch value: %v", switchValue)
		}
		k.logger.DebugContext(ctx, "Switch resolved", "original_type", typeName, "switch_on_expr", switchExpr, "switch_value", switchValue, "resolved_type", actualType)
		// Parse using actual type, passing the Go context `ctx`
		return k.parseType(ctx, actualType, stream)
	}

	// Look for the type in the schema
	var typeObj Type
	var found bool

	if typeName == k.schema.Meta.ID {
		// Parse root level sequence
		evalCtx := &ParseContext{
			Children: make(map[string]any),
			IO:       stream,
			Parent:   k.valueStack[len(k.valueStack)-1],
			Root:     k.valueStack[0].Root,
		}

		// Parse sequence fields
		for _, seq := range k.schema.Seq {
			field, err := k.parseField(ctx, seq, evalCtx)
			if err != nil {
				return nil, fmt.Errorf("parsing field '%s' in root type '%s': %w", seq.ID, typeName, err)
			}
			if field != nil { // Only add if not nil
				result.Children[seq.ID] = field        // Store the ParsedData
				evalCtx.Children[seq.ID] = field.Value // Store the primitive for expressions
			}
		}

		return result, nil
	} else if typeObj, found = k.schema.Types[typeName]; found {
		// Create evaluation context for this specific type
		typeEvalCtx := &ParseContext{
			Children: make(map[string]any),
			IO:       stream,
			Parent:   k.valueStack[len(k.valueStack)-1],
			Root:     k.valueStack[0].Root,
		}

		// Push this type's evaluation context
		k.valueStack = append(k.valueStack, typeEvalCtx)
		defer func() {
			// Pop this type's evaluation context when done
			k.valueStack = k.valueStack[:len(k.valueStack)-1]
		}()

		// Parse sequence fields
		for _, seq := range typeObj.Seq {
			// Check if this is a switch type
			if seq.Type == "switch" {
				// Handle switch case
				switchSelector, err := NewSwitchTypeSelector(seq.Switch, k.schema)
				if err != nil {
					return nil, fmt.Errorf("creating switch selector for field '%s' in type '%s': %w", seq.ID, typeName, err)
				}

				// Resolve actual type using CEL for switch expressions
				actualType, err := switchSelector.ResolveType(typeEvalCtx, k) // Pass the Go context `ctx` from parseType signature
				if err != nil {
					return nil, fmt.Errorf("resolving switch type for field '%s' in type '%s': %w", seq.ID, typeName, err)
				}

				// Override type for this field
				seqCopy := seq
				seqCopy.Type = actualType

				// Parse field with resolved type
				field, err := k.parseField(ctx, seqCopy, typeEvalCtx) // Pass Go context `ctx`, and *ParseContext `typeEvalCtx`
				if err != nil {
					return nil, fmt.Errorf("parsing switch field '%s' (resolved as '%s') in type '%s': %w",
						seq.ID, actualType, typeName, err)
				}

				result.Children[seq.ID] = field            // Store the ParsedData
				typeEvalCtx.Children[seq.ID] = field.Value // Store the primitive for expressions in typeEvalCtx
				continue
			}

			// Parse regular field
			field, err := k.parseField(ctx, seq, typeEvalCtx) // Pass Go context `ctx`, and *ParseContext `typeEvalCtx`
			if err != nil {
				return nil, fmt.Errorf("parsing field '%s' in type '%s': %w", seq.ID, typeName, err)
			}
			if field != nil { // Only add if not nil
				result.Children[seq.ID] = field            // Store the ParsedData
				typeEvalCtx.Children[seq.ID] = field.Value // Store the primitive for expressions in typeEvalCtx
			}
		}

		// Process instances if any
		if typeObj.Instances != nil {
			for name, inst := range typeObj.Instances { // TODO: Pass ctx to evaluateInstance
				val, err := k.evaluateInstance(inst, typeEvalCtx)
				if err != nil {
					return nil, fmt.Errorf("evaluating instance '%s' in type '%s': %w",
						name, typeName, err)
				}
				result.Children[name] = val
			}
		}

		return result, nil
	}
	k.logger.ErrorContext(ctx, "Unknown type encountered", "type_name", typeName)
	return nil, fmt.Errorf("unknown type: %s", typeName)
}

// parseBuiltinType handles built-in Kaitai types
func (k *KaitaiInterpreter) parseBuiltinType(ctx context.Context, typeName string, stream *kaitai.Stream) (*ParsedData, bool, error) {
	result := &ParsedData{
		Type:     typeName,
		Children: make(map[string]*ParsedData),
	}
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}
	// Process built-in types
	switch typeName {
	case "u1":
		val, err := stream.ReadU1()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u1", "error", err)
			return nil, true, fmt.Errorf("reading u1: %w", err)
		}
		result.Value = uint8(val)
		return result, true, nil
	case "u2le":
		val, err := stream.ReadU2le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u2le", "error", err)
			return nil, true, fmt.Errorf("reading u2le: %w", err)
		}
		result.Value = uint16(val)
		return result, true, nil
	case "u4le":
		val, err := stream.ReadU4le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u4le", "error", err)
			return nil, true, fmt.Errorf("reading u4le: %w", err)
		}
		result.Value = uint32(val)
		return result, true, nil
	case "u8le":
		val, err := stream.ReadU8le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u8le", "error", err)
			return nil, true, fmt.Errorf("reading u8le: %w", err)
		}
		result.Value = uint64(val)
		return result, true, nil
	case "u2be":
		val, err := stream.ReadU2be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u2be", "error", err)
			return nil, true, fmt.Errorf("reading u2be: %w", err)
		}
		result.Value = uint16(val)
		return result, true, nil
	case "u4be":
		val, err := stream.ReadU4be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u4be", "error", err)
			return nil, true, fmt.Errorf("reading u4be: %w", err)
		}
		result.Value = uint32(val)
		return result, true, nil
	case "u8be":
		val, err := stream.ReadU8be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read u8be", "error", err)
			return nil, true, fmt.Errorf("reading u8be: %w", err)
		}
		result.Value = uint64(val)
		return result, true, nil
	case "s1":
		val, err := stream.ReadS1()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s1", "error", err)
			return nil, true, fmt.Errorf("reading s1: %w", err)
		}
		result.Value = int8(val)
		return result, true, nil
	case "s2le":
		val, err := stream.ReadS2le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s2le", "error", err)
			return nil, true, fmt.Errorf("reading s2le: %w", err)
		}
		result.Value = int16(val)
		return result, true, nil
	case "s4le":
		val, err := stream.ReadS4le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s4le", "error", err)
			return nil, true, fmt.Errorf("reading s4le: %w", err)
		}
		result.Value = int32(val)
		return result, true, nil
	case "s8le":
		val, err := stream.ReadS8le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s8le", "error", err)
			return nil, true, fmt.Errorf("reading s8le: %w", err)
		}
		result.Value = int64(val)
		return result, true, nil
	case "s2be":
		val, err := stream.ReadS2be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s2be", "error", err)
			return nil, true, fmt.Errorf("reading s2be: %w", err)
		}
		result.Value = int16(val)
		return result, true, nil
	case "s4be":
		val, err := stream.ReadS4be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s4be", "error", err)
			return nil, true, fmt.Errorf("reading s4be: %w", err)
		}
		result.Value = int32(val)
		return result, true, nil
	case "s8be":
		val, err := stream.ReadS8be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read s8be", "error", err)
			return nil, true, fmt.Errorf("reading s8be: %w", err)
		}
		result.Value = int64(val)
		return result, true, nil
	case "f4le":
		val, err := stream.ReadF4le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read f4le", "error", err)
			return nil, true, fmt.Errorf("reading f4le: %w", err)
		}
		result.Value = float32(val)
		return result, true, nil
	case "f8le":
		val, err := stream.ReadF8le()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read f8le", "error", err)
			return nil, true, fmt.Errorf("reading f8le: %w", err)
		}
		result.Value = float64(val)
		return result, true, nil
	case "f4be":
		val, err := stream.ReadF4be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read f4be", "error", err)
			return nil, true, fmt.Errorf("reading f4be: %w", err)
		}
		result.Value = float32(val)
		return result, true, nil
	case "f8be":
		val, err := stream.ReadF8be()
		if err != nil {
			k.logger.ErrorContext(ctx, "Failed to read f8be", "error", err)
			return nil, true, fmt.Errorf("reading f8be: %w", err)
		}
		result.Value = float64(val)
		return result, true, nil
	case "str", "strz":
		// Handled in parseField since we need encoding info
		return nil, false, nil
	}

	// Handle type-specific endianness based on schema
	if strings.HasPrefix(typeName, "u") || strings.HasPrefix(typeName, "s") ||
		strings.HasPrefix(typeName, "f") {
		// Only append endianness if not already present
		if !strings.HasSuffix(typeName, "le") && !strings.HasSuffix(typeName, "be") {
			endian := k.schema.Meta.Endian
			if endian == "" {
				endian = "be" // Default big-endian if not specified
			}
			newType := typeName + endian
			return k.parseBuiltinType(ctx, newType, stream)
		}
	}

	// Not a built-in type
	return nil, false, nil
}

// parseField parses a field from the sequence
func (k *KaitaiInterpreter) parseField(ctx context.Context, field SequenceItem, pCtx *ParseContext) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Parsing field", "field_id", field.ID, "field_type", field.Type)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check if field has a condition using CEL
	if field.IfExpr != "" {
		k.logger.DebugContext(ctx, "Evaluating if condition for field", "field_id", field.ID, "if_expr", field.IfExpr)
		result, err := k.evaluateExpression(ctx, field.IfExpr, pCtx)
		if err != nil {
			return nil, fmt.Errorf("evaluating if condition for field '%s' ('%s'): %w", field.ID, field.IfExpr, err)
		}

		// Skip field if condition is false
		if !isTrue(result) {
			k.logger.DebugContext(ctx, "Skipping field due to if condition", "field_id", field.ID, "if_expr", field.IfExpr, "result", result)
			return nil, nil
		}
		k.logger.DebugContext(ctx, "Field will be parsed (if condition true)", "field_id", field.ID, "if_expr", field.IfExpr, "result", result)
	}

	// Handle size attribute if present
	var size int
	if field.Size != nil {
		switch v := field.Size.(type) {
		case int:
			size = v
		case float64:
			size = int(v)
		case string:
			// Size is an expression - use CEL to evaluate
			k.logger.DebugContext(ctx, "Evaluating size expression for field", "field_id", field.ID, "size_expr", v)
			result, err := k.evaluateExpression(ctx, v, pCtx)
			if err != nil {
				return nil, fmt.Errorf("evaluating size expression for field '%s' ('%s'): %w", field.ID, v, err)
			}

			// Convert to int
			switch r := result.(type) {
			case int:
				size = r
			case int64:
				size = int(r)
			case float64:
				size = int(r)
			case uint8:
				size = int(r)
			case uint16:
				size = int(r)
			case uint32:
				size = int(r)
			case uint64:
				size = int(r)
			default:
				k.logger.ErrorContext(ctx, "Size expression result is not a number", "field_id", field.ID, "size_expr", v, "result_type", fmt.Sprintf("%T", result))
				return nil, fmt.Errorf("size expression for field '%s' ('%s') result is not a number: %v (type %T)", field.ID, v, result, result)
			}
		default:
			return nil, fmt.Errorf("unsupported size type for field '%s': %T", field.ID, v)
		}
		k.logger.DebugContext(ctx, "Determined size for field", "field_id", field.ID, "size", size)
	}

	// Handle repeat attribute
	if field.Repeat != "" {
		return k.parseRepeatedField(ctx, field, pCtx, size)
	}

	// Handle contents attribute
	if field.Contents != nil {
		return k.parseContentsField(ctx, field, pCtx)
	}

	// Parse string type
	if field.Type == "str" || field.Type == "strz" {
		return k.parseStringField(ctx, field, pCtx, size)
	}

	// Parse bytes type
	if field.Type == "bytes" {
		return k.parseBytesField(ctx, field, pCtx, size)
	}

	// Read data based on size
	var fieldData []byte
	var subStream *kaitai.Stream
	var err error

	if size > 0 {
		// Read sized data
		fieldData, err = pCtx.IO.ReadBytes(size)
		if err != nil { // pCtx.IO
			return nil, fmt.Errorf("reading %d bytes for field '%s': %w", size, field.ID, err)
		}

		// Create substream
		subStream = kaitai.NewStream(bytes.NewReader(fieldData))
	} else {
		// Use current stream directly
		subStream = pCtx.IO
	}

	// Apply process if specified
	if field.Process != "" && size > 0 {
		k.logger.DebugContext(ctx, "Processing field data", "field_id", field.ID, "process_spec", field.Process)
		// Process the field data using CEL for process functions
		processedData, err := k.processDataWithCEL(ctx, fieldData, field.Process, pCtx)
		if err != nil {
			return nil, fmt.Errorf("processing field '%s' data with spec '%s': %w", field.ID, field.Process, err)
		}

		// Create new substream with processed data
		subStream = kaitai.NewStream(bytes.NewReader(processedData))
	}
	k.logger.DebugContext(ctx, "Recursively parsing field type", "field_id", field.ID, "field_type_to_parse", field.Type)
	// Parse using the appropriate stream
	return k.parseType(ctx, field.Type, subStream)
}

// processDataWithCEL processes data using CEL expressions
func (k *KaitaiInterpreter) processDataWithCEL(ctx context.Context, data []byte, processSpec string, pCtx *ParseContext) ([]byte, error) {
	k.logger.DebugContext(ctx, "Processing data with CEL", "process_spec", processSpec, "data_len", len(data))
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Parse the process spec (e.g., "xor(0x5F)")
	parts := strings.Split(processSpec, "(")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid process specification format: '%s'", processSpec)
	}

	processFn := parts[0]
	paramStr := strings.TrimRight(parts[1], ")")

	// Create expression based on the process type
	var expr string
	switch processFn {
	case "xor":
		// Handle either numeric or byte array parameter
		if strings.HasPrefix(paramStr, "[") {
			// Byte array parameter
			expr = fmt.Sprintf("processXOR(input, %s)", paramStr)
		} else {
			// Numeric parameter
			expr = fmt.Sprintf("processXOR(input, %s)", paramStr)
		}
	case "zlib":
		expr = "processZlib(input)"
	case "rotate":
		expr = fmt.Sprintf("processRotateLeft(input, %s)", paramStr)
	default:
		return nil, fmt.Errorf("unknown process function in spec '%s': '%s'", processSpec, processFn)
	}
	k.logger.DebugContext(ctx, "Constructed CEL process expression", "cel_expr", expr)

	// Get the CEL program
	program, err := k.expressionPool.GetExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("compiling process expression '%s': %w", expr, err)
	}

	// Create a new map for activation, starting with the current context's children.
	// Then add _parent, _root, _io, and finally the 'input' for the process function.
	evalMap := make(map[string]any)
	if pCtx.Children != nil {
		for k, v := range pCtx.Children {
			evalMap[k] = v
		}
	}
	if pCtx.Parent != nil {
		evalMap["_parent"] = pCtx.Parent.Children // Expose parent's children map
	}
	if pCtx.Root != nil {
		evalMap["_root"] = pCtx.Root.Children // Expose root's children map
	}
	evalMap["_io"] = pCtx.IO
	evalMap["input"] = data // Add the data to be processed as 'input'

	result, err := k.expressionPool.EvaluateExpression(program, evalMap)
	if err != nil {
		return nil, fmt.Errorf("evaluating process expression '%s': %w", expr, err)
	}

	// Convert result back to byte array
	if bytesResult, ok := result.([]byte); ok {
		k.logger.DebugContext(ctx, "Data processing successful", "process_spec", processSpec, "output_len", len(bytesResult))
		return bytesResult, nil
	}
	k.logger.ErrorContext(ctx, "Process result is not a byte array", "process_spec", processSpec, "result_type", fmt.Sprintf("%T", result))
	return nil, fmt.Errorf("process expression '%s' result is not a byte array: %T", expr, result)
}

// parseRepeatedField handles repeating fields
func (k *KaitaiInterpreter) parseRepeatedField(ctx context.Context, field SequenceItem, pCtx *ParseContext, size int) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Parsing repeated field", "field_id", field.ID, "repeat_type", field.Repeat, "repeat_expr", field.RepeatExpr)
	result := &ParsedData{
		Type:    field.Type,
		IsArray: true,
		Value:   make([]any, 0),
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Determine repeat count
	var count int

	if field.RepeatExpr != "" {
		// Evaluate repeat expression using CEL
		expr, err := k.evaluateExpression(ctx, field.RepeatExpr, pCtx) // Pass Go context `ctx` and ParseContext pCtx
		if err != nil {
			return nil, fmt.Errorf("evaluating repeat expression for field '%s' ('%s'): %w", field.ID, field.RepeatExpr, err)
		}
		k.logger.DebugContext(ctx, "Evaluated repeat expression", "field_id", field.ID, "repeat_expr", field.RepeatExpr, "count_result", expr)

		// Convert to int
		switch v := expr.(type) {
		case int:
			count = v
		case int64:
			count = int(v)
		case float64:
			count = int(v)
		case uint8:
			count = int(v)
		case uint16:
			count = int(v)
		case uint32:
			count = int(v)
		case uint64:
			count = int(v)
		default:
			k.logger.ErrorContext(ctx, "Repeat expression result is not a number", "field_id", field.ID, "repeat_expr", field.RepeatExpr, "result_type", fmt.Sprintf("%T", expr))
			return nil, fmt.Errorf("repeat expression for field '%s' ('%s') result is not a number: %v (type %T)", field.ID, field.RepeatExpr, expr, expr)
		}
	} else if field.Repeat == "eos" {
		// Repeat until end of stream
		k.logger.DebugContext(ctx, "Repeating field until EOS", "field_id", field.ID)
		count = -1
	} else {
		return nil, fmt.Errorf("unsupported repeat type for field '%s': %s", field.ID, field.Repeat)
	}
	k.logger.DebugContext(ctx, "Determined repeat count for field", "field_id", field.ID, "count", count)

	// Read items
	items := make([]*ParsedData, 0)

	if count > 0 {
		for i := 0; i < count; i++ {
			select {
			case <-ctx.Done():
				k.logger.InfoContext(ctx, "Parsing repeated field cancelled", "field_id", field.ID, "iteration", i)
				return nil, ctx.Err()
			default:
			}
			k.logger.DebugContext(ctx, "Parsing repeated item", "field_id", field.ID, "iteration", i+1, "total_count", count)
			itemField := field
			itemField.Repeat = ""
			itemField.RepeatExpr = ""
			item, err := k.parseField(ctx, itemField, pCtx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					k.logger.WarnContext(ctx, "EOF reached while parsing repeated item", "field_id", field.ID, "iteration", i+1)
					break
				}
				return nil, fmt.Errorf("parsing repeated item %d for field '%s': %w", i+1, field.ID, err)
			}
			items = append(items, item)
		}
	} else if count == -1 {
		itemNum := 0
		for {
			itemNum++
			k.logger.DebugContext(ctx, "Parsing EOS-repeated item", "field_id", field.ID, "item_num", itemNum)
			itemField := field
			itemField.Repeat = ""
			itemField.RepeatExpr = ""
			item, err := k.parseField(ctx, itemField, pCtx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("error parsing repeated item: %w", err)
			}
			items = append(items, item)
			select {
			case <-ctx.Done():
				k.logger.InfoContext(ctx, "Parsing EOS-repeated field cancelled", "field_id", field.ID, "items_parsed", len(items))
				return nil, ctx.Err()
			default:
			}
		}
	}

	// Add items to result
	itemValues := make([]any, len(items))
	for i, item := range items {
		itemValues[i] = item
	}
	result.Value = itemValues
	k.logger.DebugContext(ctx, "Finished parsing repeated field", "field_id", field.ID, "num_items", len(items))
	return result, nil
}

// parseContentsField handles fields with fixed contents
func (k *KaitaiInterpreter) parseContentsField(ctx context.Context, field SequenceItem, pCtx *ParseContext) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Parsing contents field", "field_id", field.ID)
	result := &ParsedData{
		Type: field.Type,
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Determine contents
	var expected []byte

	switch v := field.Contents.(type) {
	case []any:
		// Array of byte values
		expected = make([]byte, len(v))
		for i, b := range v {
			if val, ok := b.(float64); ok {
				expected[i] = byte(val)
			} else {
				return nil, fmt.Errorf("invalid content byte value for field '%s': %v", field.ID, b)
			}
		}
	case string:
		expected = []byte(v)
	default:
		return nil, fmt.Errorf("unsupported contents type for field '%s': %T", field.ID, v)
	}
	k.logger.DebugContext(ctx, "Expected contents for field", "field_id", field.ID, "expected_bytes", fmt.Sprintf("%x", expected))

	// Read actual bytes
	actual, err := pCtx.IO.ReadBytes(len(expected))
	if err != nil {
		return nil, fmt.Errorf("reading content bytes for field '%s': %w", field.ID, err)
	}
	k.logger.DebugContext(ctx, "Actual contents read for field", "field_id", field.ID, "actual_bytes", fmt.Sprintf("%x", actual))

	// Validate contents
	if !bytes.Equal(actual, expected) {
		k.logger.ErrorContext(ctx, "Content validation failed", "field_id", field.ID, "expected", fmt.Sprintf("%x", expected), "actual", fmt.Sprintf("%x", actual))
		return nil, fmt.Errorf("content validation failed for field '%s', expected %x, got %x", field.ID, expected, actual)
	}

	result.Value = actual
	k.logger.DebugContext(ctx, "Contents field parsed successfully", "field_id", field.ID)
	return result, nil
}

// parseStringField handles string fields
func (k *KaitaiInterpreter) parseStringField(ctx context.Context, field SequenceItem, pCtx *ParseContext, size int) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Parsing string field", "field_id", field.ID, "type", field.Type, "size", size, "encoding_field", field.Encoding, "size_eos", field.SizeEOS)
	result := &ParsedData{
		Type: field.Type,
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Determine encoding
	encoding := field.Encoding
	if encoding == "" {
		encoding = k.schema.Meta.Encoding
	}
	if encoding == "" {
		encoding = "UTF-8" // Default encoding
	}
	k.logger.DebugContext(ctx, "Using encoding for string field", "field_id", field.ID, "encoding", encoding)

	var strBytes []byte
	var err error

	if field.Type == "strz" {
		// Zero-terminated string
		strBytes, err = pCtx.IO.ReadBytesTerm(0, false, true, true)
		k.logger.DebugContext(ctx, "Read zero-terminated string", "field_id", field.ID, "bytes_read", len(strBytes), "error", err)
	} else if size > 0 {
		// Fixed-size string
		strBytes, err = pCtx.IO.ReadBytes(size)
		k.logger.DebugContext(ctx, "Read fixed-size string", "field_id", field.ID, "size", size, "bytes_read", len(strBytes), "error", err)
	} else if field.SizeEOS {
		// Read until end of stream
		k.logger.DebugContext(ctx, "Reading string until EOS", "field_id", field.ID)
		// Use ReadStrEOS directly if the encoding is UTF-8
		if strings.ToUpper(encoding) == "UTF-8" || strings.ToUpper(encoding) == "UTF8" {
			str, err := pCtx.IO.ReadStrEOS(encoding) // Corrected: Use pCtx.IO
			if err != nil {                          // pCtx.IO
				return nil, fmt.Errorf("reading string until EOS for field '%s': %w", field.ID, err) // ctx.IO -> pCtx.IO
			}
			result.Value = str
			return result, nil
		}

		// Otherwise, use position and seek
		isEof, err := pCtx.IO.EOF()
		if err != nil {
			return nil, fmt.Errorf("checking EOF for field '%s': %w", field.ID, err)
		}
		if isEof {
			result.Value = ""
			return result, nil
		}

		pos, err := pCtx.IO.Pos()
		if err != nil {
			return nil, fmt.Errorf("getting current position for field '%s': %w", field.ID, err)
		}
		stream := pCtx.IO
		endPos, err := stream.Size()
		if err != nil {
			return nil, fmt.Errorf("getting stream size for field '%s': %w", field.ID, err)
		}
		size := endPos - pos
		strBytes, err = pCtx.IO.ReadBytes(int(size))
		if err != nil {
			return nil, fmt.Errorf("reading string bytes until EOS for field '%s': %w", field.ID, err)
		}
	} else {
		return nil, fmt.Errorf("cannot determine string size for field '%s'", field.ID)
	}

	if err != nil { // This check might be redundant if errors are handled above
		return nil, fmt.Errorf("reading string bytes for field '%s': %w", field.ID, err)
	}

	// Process encoding - use CEL's bytesToStr function
	program, err := k.expressionPool.GetExpression("bytesToStr(input)")
	if err != nil {
		return nil, fmt.Errorf("failed to compile bytesToStr expression: %w", err)
	}

	val, err := k.expressionPool.EvaluateExpression(program, map[string]any{
		"input": strBytes,
	})
	if err != nil {
		// If CEL processing fails, fall back to manual decoding
		var str string

		k.logger.WarnContext(ctx, "CEL bytesToStr failed, falling back to manual decoding", "field_id", field.ID, "cel_error", err)
		// Use Kaitai's BytesToStr where possible
		if strings.ToUpper(encoding) == "ASCII" || strings.ToUpper(encoding) == "UTF-8" || strings.ToUpper(encoding) == "UTF8" {
			str, err = kaitai.BytesToStr(strBytes, nil)
			if err != nil {
				return nil, fmt.Errorf("decoding string for field '%s' with encoding '%s': %w", field.ID, encoding, err)
			}

		} else {
			//  proper transcoding for other encodings
			switch strings.ToUpper(encoding) {
			case "UTF-16LE", "UTF16LE":
				decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
				utf8Str, _, err := transform.String(decoder, string(strBytes))
				if err != nil {
					return nil, fmt.Errorf("decoding UTF-16LE for field '%s': %w", field.ID, err)
				}
				str = utf8Str
			case "UTF-16BE", "UTF16BE":
				decoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
				utf8Str, _, err := transform.String(decoder, string(strBytes))
				if err != nil {
					return nil, fmt.Errorf("decoding UTF-16BE for field '%s': %w", field.ID, err)
				}
				str = utf8Str
			default:
				return nil, fmt.Errorf("unsupported encoding '%s' for field '%s'", encoding, field.ID)
			}
		}
		result.Value = str
	} else {
		result.Value = val
	}
	k.logger.DebugContext(ctx, "Parsed string field", "field_id", field.ID, "value_len", len(result.Value.(string)))
	return result, nil
}

// parseBytesField handles bytes fields
func (k *KaitaiInterpreter) parseBytesField(ctx context.Context, field SequenceItem, pCtx *ParseContext, size int) (*ParsedData, error) {
	k.logger.DebugContext(ctx, "Parsing bytes field", "field_id", field.ID, "size", size, "size_eos", field.SizeEOS)

	result := &ParsedData{
		Type: field.Type,
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var bytesData []byte
	var err error

	if size > 0 {
		// Fixed-size bytes
		bytesData, err = pCtx.IO.ReadBytes(size)
	} else if field.SizeEOS {
		// Read until end of stream
		bytesData, err = pCtx.IO.ReadBytesFull()
	} else {
		return nil, fmt.Errorf("cannot determine bytes size for field '%s'", field.ID)
	}

	if err != nil {
		return nil, fmt.Errorf("reading bytes for field '%s': %w", field.ID, err)
	}

	result.Value = bytesData
	k.logger.DebugContext(ctx, "Parsed bytes field", "field_id", field.ID, "bytes_len", len(bytesData))
	return result, nil
}

// evaluateInstance calculates an instance field
func (k *KaitaiInterpreter) evaluateInstance(inst InstanceDef, pCtx *ParseContext) (*ParsedData, error) {
	// TODO: This method should also accept context.Context
	// k.logger.DebugContext(goCtx, "Evaluating instance", "instance_name", "TODO_get_name", "instance_value_expr", inst.Value)
	goCtx := context.Background() // Placeholder, should ideally be passed in
	// Evaluate the instance expression using CEL
	value, err := k.evaluateExpression(goCtx, inst.Value, pCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate instance expression: %w", err)
	}

	result := &ParsedData{
		Type:  inst.Type,
		Value: value,
	}

	// Handle type conversion if needed
	if inst.Type != "" {
		// Here we could implement type conversion based on inst.Type
		// For now, we just set the type
	}

	return result, nil
}

// evaluateExpression evaluates a Kaitai expression using CEL
func (k *KaitaiInterpreter) evaluateExpression(ctx context.Context, kaitaiExpr string, pCtx *ParseContext) (any, error) {
	k.logger.DebugContext(ctx, "Evaluating CEL expression", "kaitai_expr", kaitaiExpr)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Get or compile expression
	program, err := k.expressionPool.GetExpression(kaitaiExpr)
	if err != nil {
		return nil, fmt.Errorf("compiling expression '%s': %w", kaitaiExpr, err)
	}

	// Create activation from context
	activation, err := pCtx.AsActivation()
	if err != nil {
		return nil, fmt.Errorf("creating activation for expression '%s': %w", kaitaiExpr, err)
	}

	// Evaluate expression
	result, _, err := program.Eval(activation)
	if err != nil {
		return nil, fmt.Errorf("evaluating expression '%s': %w", kaitaiExpr, err)
	}
	k.logger.DebugContext(ctx, "CEL expression evaluated successfully", "kaitai_expr", kaitaiExpr, "result", result.Value())
	return result.Value(), nil
}

// parsedDataToMap converts ParsedData to a map suitable for JSON serialization
func ParsedDataToMap(data *ParsedData) any {
	if data == nil {
		return nil
	}

	if data.IsArray {
		// Handle array type
		if arr, ok := data.Value.([]any); ok {
			result := make([]any, len(arr))
			for i, v := range arr {
				if pd, ok := v.(*ParsedData); ok {
					result[i] = ParsedDataToMap(pd)
				} else {
					result[i] = v
				}
			}
			return result
		}
		return data.Value
	}

	if len(data.Children) == 0 {
		// Just return the value for primitive types
		return data.Value
	}

	// Convert struct type with children
	result := make(map[string]any)

	// Add value field if it exists and isn't zero/empty
	if data.Value != nil {
		result["_value"] = data.Value
	}

	// Add all children
	for name, child := range data.Children {
		result[name] = ParsedDataToMap(child)
	}

	return result
}

// Helper for checking boolean values
func isTrue(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != ""
	}
	return true
}
