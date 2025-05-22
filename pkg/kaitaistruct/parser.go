package kaitaistruct

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"maps"
	"slices"

	"github.com/google/cel-go/cel"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	internalCel "github.com/twinfer/kbin-plugin/internal/cel"
	"github.com/twinfer/kbin-plugin/testutil"
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
			val, err := k.evaluateInstance(ctx, inst, rootCtx) // Pass ctx
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
		// Extract the expression part after "switch-on:"
		expressionPart := strings.TrimPrefix(typeName, "switch-on:")
		if expressionPart == typeName || expressionPart == "" { // Check if TrimPrefix did anything or if expr is empty
			return nil, fmt.Errorf("invalid switch type format: %s", typeName)
		}
		switchExpr := strings.TrimSpace(expressionPart)

		// The ad-hoc switch expression should be evaluated in the context of the
		// type that *contains* this ad-hoc switch field. This context is the one
		// at the top of the valueStack, which represents the type currently being parsed.
		if len(k.valueStack) == 0 {
			return nil, fmt.Errorf("internal error: valueStack is empty when evaluating ad-hoc switch for type '%s'", typeName)
		}
		currentTypeEvalCtx := k.valueStack[len(k.valueStack)-1]
		k.logger.DebugContext(ctx, "Context for ad-hoc switch evaluation",
			"type_name", typeName,
			"switch_expr", switchExpr,
			"currentTypeEvalCtx_children", fmt.Sprintf("%#v", currentTypeEvalCtx.Children))
		if currentTypeEvalCtx == nil {
			return nil, fmt.Errorf("internal error: valueStack is empty or has nil context for ad-hoc switch in type '%s'", typeName)
		}

		// Evaluate switch expression using CEL, passing the Go context `ctx`
		switchValue, err := k.evaluateExpression(ctx, switchExpr, currentTypeEvalCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate switch expression '%s' for type '%s': %w", switchExpr, typeName, err)
		}

		// Determine actual type based on switch value
		actualType, ok := switchValue.(string)
		if !ok {
			k.logger.ErrorContext(ctx, "Ad-hoc switch expression did not evaluate to a string type name", "type_name", typeName, "switch_on_expr", switchExpr, "evaluated_value_type", fmt.Sprintf("%T", switchValue))
			return nil, fmt.Errorf("ad-hoc switch expression '%s' did not evaluate to a string type name, got %T", switchExpr, switchValue)
		}

		k.logger.DebugContext(ctx, "Ad-hoc switch resolved", "original_type", typeName, "switch_on_expr", switchExpr, "switch_value", switchValue, "resolved_type", actualType)
		// Parse using actual type, passing the Go context `ctx`
		return k.parseType(ctx, actualType, stream)
	}

	// Look for the type in the schema
	var typeObj Type
	var found bool

	if typeName == k.schema.Meta.ID {
		// This is the root type being parsed directly (not as a field of another type)
		// Its parent is effectively nil in terms of Kaitai's _parent, and its root is itself.
		// The rootCtx is already on the valueStack.
		if len(k.valueStack) == 0 { // Should not happen if Parse set up rootCtx
			return nil, fmt.Errorf("internal error: valueStack empty when parsing root type sequence for '%s'", typeName)
		}
		currentRootCtx := k.valueStack[0] // This is the root ParseContext
		// Parse root level sequence
		evalCtx := &ParseContext{
			Children: make(map[string]any),
			IO:       stream,
			Parent:   nil, // Root level has no _parent in the Kaitai sense of a containing user type
			Root:     currentRootCtx,
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
			var parsedFieldData *ParsedData // Renamed to avoid conflict
			var parseErr error
			k.logger.DebugContext(ctx, "Processing seq item in type", "type_name", typeName, "seq_id", seq.ID, "seq_type_literal", seq.Type, "is_switch", seq.Type == "switch")

			// parseField will handle all field types, including resolving "switch" and "switch-on:"
			parsedFieldData, parseErr = k.parseField(ctx, seq, typeEvalCtx)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing field '%s' in type '%s': %w", seq.ID, typeName, parseErr)
			}

			if parsedFieldData != nil { // Only add if not nil (e.g., conditional field was skipped)
				result.Children[seq.ID] = parsedFieldData            // Store the ParsedData
				typeEvalCtx.Children[seq.ID] = parsedFieldData.Value // Store the primitive for expressions in typeEvalCtx
			}
		}

		// Process instances if any
		if typeObj.Instances != nil {
			k.logger.DebugContext(ctx, "Processing instances for type", "type_name", typeName, "instance_count", len(typeObj.Instances))

			instancesToProcess := make(map[string]InstanceDef)
			for name, inst := range typeObj.Instances {
				instancesToProcess[name] = inst
				// Eagerly add instance definition to context so it can be referenced if needed,
				// though its value will be evaluated. This is more for introspection.
				// typeEvalCtx.Children[name] = inst // This might be too early or confusing for CEL
			}

			maxPasses := len(instancesToProcess) + 2 // Allow a couple of extra passes for dependencies
			processedInLastPass := -1

			for pass := 0; pass < maxPasses && len(instancesToProcess) > 0 && processedInLastPass != 0; pass++ {
				k.logger.DebugContext(ctx, "Instance evaluation pass", "type_name", typeName, "pass_num", pass+1, "remaining_instances", len(instancesToProcess))
				processedInThisPass := 0
				successfullyProcessedThisPass := make(map[string]bool)

				for name, inst := range instancesToProcess {
					val, err := k.evaluateInstance(ctx, inst, typeEvalCtx)
					if err != nil {
						// If error is due to missing attribute, it might be a dependency not yet evaluated.
						// A more sophisticated check could inspect the error for "no such attribute".
						// For now, we log and hope a subsequent pass resolves it.
						// If the error is persistent (not a dependency issue), it will eventually fail.
						k.logger.DebugContext(ctx, "Instance evaluation attempt failed (may retry)", "type_name", typeName, "instance_name", name, "pass", pass+1, "error", err)

						// A more sophisticated check could inspect the error for "no such attribute".
					} else {
						k.logger.DebugContext(ctx, "Instance evaluated successfully", "type_name", typeName, "instance_name", name, "value", val.Value)
						result.Children[name] = val // Store the ParsedData for the final result
						if val != nil {
							typeEvalCtx.Children[name] = val.Value // Add to current eval context
						}
						successfullyProcessedThisPass[name] = true
						processedInThisPass++
					}
				}
				for name := range successfullyProcessedThisPass {
					delete(instancesToProcess, name)
				}
				processedInLastPass = processedInThisPass
			}
			if len(instancesToProcess) > 0 {
				remainingInstanceNames := make([]string, 0, len(instancesToProcess))
				for name := range instancesToProcess {
					remainingInstanceNames = append(remainingInstanceNames, name)
				}
				return nil, fmt.Errorf("failed to evaluate all instances for type '%s' after %d passes; remaining: %v. Check for circular dependencies or unresolvable expressions.", typeName, maxPasses-1, strings.Join(remainingInstanceNames, ", "))
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

	// If the field's type is an ad-hoc switch string (e.g., "switch-on:expr"),
	// resolve the actual type here using pCtx for expression evaluation.
	actualFieldType := field.Type

	if strings.HasPrefix(field.Type, "switch-on:") {
		expressionPart := strings.TrimPrefix(field.Type, "switch-on:")
		if expressionPart == field.Type || expressionPart == "" {
			return nil, fmt.Errorf("invalid ad-hoc switch type format for field '%s': %s", field.ID, field.Type)
		}
		switchExpr := strings.TrimSpace(expressionPart)
		k.logger.DebugContext(ctx, "Field has ad-hoc switch type, evaluating expression", "field_id", field.ID, "switch_expr", switchExpr, "pCtx_children", fmt.Sprintf("%#v", pCtx.Children))

		switchValue, err := k.evaluateExpression(ctx, switchExpr, pCtx) // Use pCtx here
		if err != nil {
			return nil, fmt.Errorf("evaluating ad-hoc switch expression '%s' for field '%s': %w", switchExpr, field.ID, err)
		}

		resolvedTypeName, ok := switchValue.(string)
		if !ok {
			return nil, fmt.Errorf("ad-hoc switch expression '%s' for field '%s' did not evaluate to a string type name, got %T", switchExpr, field.ID, switchValue)
		}
		actualFieldType = resolvedTypeName
		k.logger.DebugContext(ctx, "Ad-hoc switch field type resolved", "field_id", field.ID, "original_type", field.Type, "resolved_type", actualFieldType)
	} else if field.Type == "switch" {
		// Handle fields explicitly defined with type: switch
		k.logger.DebugContext(ctx, "Field is an explicit switch type, resolving actual type", "field_id", field.ID)
		if field.Switch == nil {
			return nil, fmt.Errorf("field '%s' is of type 'switch' but has no 'switch-on' definition", field.ID)
		}
		switchSelector, err := NewSwitchTypeSelector(field.Switch, k.schema)
		if err != nil {
			return nil, fmt.Errorf("creating switch selector for field '%s': %w", field.ID, err)
		}
		// pCtx is the context of the parent type containing this switch field
		resolvedType, err := switchSelector.ResolveType(ctx, pCtx, k)
		if err != nil {
			return nil, fmt.Errorf("resolving switch type for field '%s': %w", field.ID, err)
		}
		actualFieldType = resolvedType
		k.logger.DebugContext(ctx, "Explicit switch field type resolved", "field_id", field.ID, "original_type", field.Type, "resolved_type", actualFieldType)
	}

	k.logger.DebugContext(ctx, "Recursively parsing field type", "field_id", field.ID, "field_type_to_parse", actualFieldType)
	// Parse using the appropriate stream
	return k.parseType(ctx, actualFieldType, subStream)
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
	program, err := k.expressionPool.GetExpression(fmt.Sprintf("bytesToStr(input, %q)", encoding)) // Pass encoding to CEL
	if err != nil {
		return nil, fmt.Errorf("failed to compile bytesToStr expression: %w", err)
	}

	val, err := k.expressionPool.EvaluateExpression(program, map[string]any{
		"input": strBytes,
	})
	// If CEL evaluation fails, it returns an error. We no longer have a manual fallback here.
	if err != nil {
		return nil, fmt.Errorf("decoding string for field '%s' with encoding '%s' via CEL: %w", field.ID, encoding, err)
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
func (k *KaitaiInterpreter) evaluateInstance(goCtx context.Context, inst InstanceDef, pCtx *ParseContext) (*ParsedData, error) {
	k.logger.DebugContext(goCtx, "Evaluating instance", "instance_value_expr", inst.Value) // TODO: Add instance name to log if available
	// Evaluate the instance expression using CEL
	value, err := k.evaluateExpression(goCtx, inst.Value, pCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate instance expression: %w", err)
	}

	result := &ParsedData{
		Type:  inst.Type,
		Value: value,
	}
	k.logger.DebugContext(goCtx, "Instance value from CEL", "instance_name", inst.DocRef, "value", value, "value_type", fmt.Sprintf("%T", value))

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
		result[testutil.ToPascalCase(name)] = ParsedDataToMap(child) // Convert key to PascalCase

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
