package kaitaistruct

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"maps"

	"github.com/google/cel-go/cel"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	internalCel "github.com/twinfer/kbin-plugin/internal/cel"
	"github.com/twinfer/kbin-plugin/pkg/kaitaicel"
)

// KaitaiSerializer provides functionality to serialize objects according to Kaitai schema
type KaitaiSerializer struct {
	schema          *KaitaiSchema
	expressionPool  *internalCel.ExpressionPool
	typeStack       []string // Stack of type names being processed for hierarchical resolution
	logger          *slog.Logger
}

// SerializeContext holds the current state during serialization
type SerializeContext struct {
	Value    any
	Parent   *SerializeContext
	Root     *SerializeContext
	Writer   *kaitai.Writer
	Children map[string]any
}

// NewKaitaiSerializer creates a new serializer for a given schema
func NewKaitaiSerializer(schema *KaitaiSchema, logger *slog.Logger) (*KaitaiSerializer, error) {
	pool, err := internalCel.NewExpressionPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create expression pool: %w", err)
	}

	log := logger
	if log == nil {
		log = slog.Default()
	}

	return &KaitaiSerializer{
		schema:          schema,
		expressionPool:  pool,
		logger:          log,
	}, nil
}

// AsActivation creates a CEL activation from the serialization context
func (ctx *SerializeContext) AsActivation() (cel.Activation, error) {
	vars := make(map[string]any)

	// Add current context values
	if ctx.Children != nil {
		maps.Copy(vars, ctx.Children)
	}

	// Add special variables
	vars["_writer"] = ctx.Writer
	if ctx.Root != nil {
		vars["_root"] = ctx.Root.Value
	}
	if ctx.Parent != nil {
		vars["_parent"] = ctx.Parent.Value
	}

	return cel.NewActivation(vars)
}

// Serialize serializes an object according to the schema
func (k *KaitaiSerializer) Serialize(ctx context.Context, data map[string]any) ([]byte, error) {
	k.logger.DebugContext(ctx, "Starting Kaitai serialization", "root_type_meta", k.schema.Meta.ID, "root_type_schema", k.schema.RootType)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Create a buffer to write to
	buf := bytes.NewBuffer(nil)
	writer := kaitai.NewWriter(buf)

	// Create root context
	rootCtx := &SerializeContext{
		Value:    data,
		Children: data,
		Writer:   writer,
	}
	rootCtx.Root = rootCtx

	// Get root type
	rootType := k.schema.Meta.ID
	if k.schema.RootType != "" {
		rootType = k.schema.RootType
	}

	// Serialize according to root type
	if err := k.serializeType(ctx, rootType, data, rootCtx); err != nil {
		return nil, fmt.Errorf("failed serializing root type '%s': %w", rootType, err)
	}
	k.logger.DebugContext(ctx, "Finished Kaitai serialization")
	return buf.Bytes(), nil
}

// serializeType serializes data according to a specified type
func (k *KaitaiSerializer) serializeType(goCtx context.Context, typeName string, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing type", "type_name", typeName)
	select {
	case <-goCtx.Done():
		return goCtx.Err()
	default:
	}
	// Check if it's a switch type
	if strings.Contains(typeName, "switch-on:") {
		k.logger.DebugContext(goCtx, "Handling switch type defined in type name", "type_name_switch", typeName)
		return k.serializeAdHocSwitchType(goCtx, typeName, data, sCtx)
	}

	// Look for the type in schema first (root type or custom types)
	if typeName == k.schema.Meta.ID {
		// Serialize root level fields
		dataMap, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("expected map for root type, got %T", data)
		}
		
		// Create context for the root type
		fieldCtx := &SerializeContext{
			Value:    dataMap,
			Children: dataMap,
			Writer:   sCtx.Writer,
			Parent:   sCtx,
			Root:     sCtx.Root,
		}
		
		// Pre-evaluate root-level instances
		if k.schema.Instances != nil {
			k.logger.DebugContext(goCtx, "Pre-evaluating root-level instances", "instance_count", len(k.schema.Instances))
			err := k.evaluateInstancesWithDependencies(goCtx, k.schema.Instances, fieldCtx)
			if err != nil {
				k.logger.WarnContext(goCtx, "Some root-level instances could not be evaluated due to dependencies", "error", err)
			}
		}
		
		// Use helper to serialize sequence
		return k.serializeSequence(goCtx, typeName, k.schema.Seq, dataMap, fieldCtx)
	}

	// Check if it's a built-in type (only for primitive type names)
	if k.isBuiltinTypeName(typeName) {
		if handled, err := k.serializeBuiltinType(goCtx, typeName, data, sCtx.Writer); handled {
			if err != nil {
				k.logger.ErrorContext(goCtx, "Error serializing built-in type", "type_name", typeName, "error", err)
			}
			return err
		}
	}

	// Try to find the type in the schema types using hierarchical resolution
	typePtr, found := k.resolveTypeInHierarchy(typeName)
	if !found {
		k.logger.ErrorContext(goCtx, "Unknown type for serialization", "type_name", typeName)
		return fmt.Errorf("unknown type: %s", typeName)
	}
	typeObj := *typePtr

	// Serialize type object
	dataMap, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("expected map for type %s, got %T", typeName, data)
	}

	// Create context for this type
	fieldCtx := &SerializeContext{
		Value:    dataMap,
		Children: dataMap,
		Writer:   sCtx.Writer,
		Parent:   sCtx,
		Root:     sCtx.Root,
	}

	// Push type to stack for hierarchical resolution
	k.typeStack = append(k.typeStack, typeName)
	defer func() {
		// Pop type from stack when done
		k.typeStack = k.typeStack[:len(k.typeStack)-1]
	}()

	// Pre-evaluate instances for this type and add them to the fieldCtx.Children
	// This makes them available for expressions in seq items (if, size, repeat-expr, etc.)
	if typeObj.Instances != nil {
		k.logger.DebugContext(goCtx, "Pre-evaluating instances for type", "type_name", typeName, "instance_count", len(typeObj.Instances))
		err := k.evaluateInstancesWithDependencies(goCtx, typeObj.Instances, fieldCtx)
		if err != nil {
			k.logger.WarnContext(goCtx, "Some instances could not be evaluated due to dependencies", "type_name", typeName, "error", err)
		}
	}

	// Use helper to serialize sequence
	if err := k.serializeSequence(goCtx, typeName, typeObj.Seq, dataMap, fieldCtx); err != nil {
		k.logger.ErrorContext(goCtx, "Error serializing sequence for type", "type_name", typeName, "error", err)

		return err
	}

	k.logger.DebugContext(goCtx, "Finished serializing type", "type_name", typeName)
	return nil
}

// serializeAdHocSwitchType handles serialization of switch types defined in the type name string.
func (k *KaitaiSerializer) serializeAdHocSwitchType(goCtx context.Context, typeName string, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing ad-hoc switch type", "type_name_switch", typeName)
	select {
	case <-goCtx.Done():
		return goCtx.Err()
	default:
	}
	// Extract the expression part after "switch-on:"
	expressionPart := strings.TrimPrefix(typeName, "switch-on:")
	if expressionPart == typeName || expressionPart == "" { // Check if TrimPrefix did anything or if expr is empty
		return fmt.Errorf("invalid switch type format: %s", typeName)
	}
	switchExpr := strings.TrimSpace(expressionPart)

	// Evaluate switch expression
	switchValue, err := k.evaluateExpression(goCtx, switchExpr, sCtx)
	if err != nil {
		return fmt.Errorf("evaluating ad-hoc switch expression '%s': %w", switchExpr, err)
	}

	// Determine actual type based on switch value
	// For this ad-hoc format, the expression must evaluate to the type name string.
	actualType, ok := switchValue.(string)
	if !ok {
		k.logger.ErrorContext(goCtx, "Ad-hoc switch expression did not evaluate to a string type name", "type_name_switch", typeName, "switch_expr", switchExpr, "evaluated_value_type", fmt.Sprintf("%T", switchValue))
		return fmt.Errorf("ad-hoc switch expression '%s' for type '%s' did not evaluate to a string type name, got %T", switchExpr, typeName, switchValue)
	}

	k.logger.DebugContext(goCtx, "Ad-hoc switch resolved", "original_type_switch", typeName, "switch_expr", switchExpr, "switch_value", switchValue, "resolved_type", actualType)

	// Serialize using actual type
	return k.serializeType(goCtx, actualType, data, sCtx)
}

// serializeSequence processes a list of sequence items for a given type.
func (k *KaitaiSerializer) serializeSequence(goCtx context.Context, typeName string, sequence []SequenceItem, dataMap map[string]any, typeCtx *SerializeContext) error {
	// At this point, typeCtx.Children should contain both the fields from dataMap
	// and any pre-evaluated instances for the current 'typeName'.
	// If instances were not pre-evaluated in serializeType, they would need to be handled here
	// with care for dependencies.

	for _, seq := range sequence {
		// Handle switch types
		if seq.Type == "switch" {
			k.logger.DebugContext(goCtx, "Handling switch type for field", "field_id", seq.ID, "type_name", typeName)
			// Resolve switch type
			actualType, err := k.resolveSwitchTypeForSerialization(goCtx, seq.Switch, typeCtx)
			if err != nil {
				return fmt.Errorf("error resolving switch type for field '%s': %w", seq.ID, err)
			}

			// Override type for this field
			seqCopy := seq
			seqCopy.Type = actualType

			fieldData, dataOk := dataMap[seq.ID]
			if !dataOk && actualType != "" {
				k.logger.WarnContext(goCtx, "Data for switch field not found, but type resolved", "field_id", seq.ID, "resolved_type", actualType)
			}
			if err := k.serializeField(goCtx, seqCopy, fieldData, typeCtx); err != nil {
				return fmt.Errorf("serializing switch field '%s' (resolved as '%s') in type '%s': %w", seq.ID, actualType, typeName, err)
			}
			continue
		}

		// Handle normal field
		fieldData, dataOk := dataMap[seq.ID]
		if !dataOk {
			if seq.IfExpr == "" {
				k.logger.WarnContext(goCtx, "Data for non-conditional field not found in input map", "field_id", seq.ID, "type_name", typeName)
			} else {
				k.logger.DebugContext(goCtx, "Data for conditional field not found, will be handled by 'if' expr", "field_id", seq.ID, "type_name", typeName)
			}
		}
		if err := k.serializeField(goCtx, seq, fieldData, typeCtx); err != nil {
			return fmt.Errorf("error serializing field '%s' in type '%s': %w", seq.ID, typeName, err)
		}
	}
	return nil
}

// serializeBuiltinType handles serialization of built-in types using kaitaicel
func (k *KaitaiSerializer) serializeBuiltinType(goCtx context.Context, typeName string, data any, writer *kaitai.Writer) (bool, error) {
	k.logger.DebugContext(goCtx, "Serializing built-in type with kaitaicel", "type_name", typeName)

	// Handle type-specific endianness if not specified (but only for multi-byte types)
	actualTypeName := typeName
	if (strings.HasPrefix(typeName, "u") || strings.HasPrefix(typeName, "s") || strings.HasPrefix(typeName, "f")) &&
		typeName != "u1" && typeName != "s1" { // 1-byte types don't have endianness
		if !strings.HasSuffix(typeName, "le") && !strings.HasSuffix(typeName, "be") {
			endian := k.schema.Meta.Endian
			if endian == "" {
				endian = "be" // Default big-endian if not specified
			}
			actualTypeName = typeName + endian
			k.logger.DebugContext(goCtx, "Applying endianness to built-in type", "original_type", typeName, "new_type_with_endian", actualTypeName)
		}
	}

	// Use kaitaicel centralized factory to create the type
	kaitaiType, err := kaitaicel.NewKaitaiTypeFromValue(data, actualTypeName)
	if err != nil {
		// Type not handled by kaitaicel, return false to let other handlers try
		if actualTypeName == "str" || actualTypeName == "strz" || actualTypeName == "bytes" {
			return false, nil
		}
		return true, fmt.Errorf("failed to create kaitai type for '%s': %w", actualTypeName, err)
	}

	// Get binary data from kaitai type using Serialize
	binaryData := kaitaiType.Serialize()

	// Write to the writer
	if err := writer.WriteBytes(binaryData); err != nil {
		return true, fmt.Errorf("failed to write bytes for type '%s': %w", actualTypeName, err)
	}

	k.logger.DebugContext(goCtx, "Successfully serialized built-in type with kaitaicel", "type_name", actualTypeName, "bytes_written", len(binaryData))
	return true, nil
}

// isBuiltinTypeName checks if a type name represents a builtin primitive type
func (k *KaitaiSerializer) isBuiltinTypeName(typeName string) bool {
	// Remove endian suffix to check base type
	baseType := typeName
	if strings.HasSuffix(typeName, "le") || strings.HasSuffix(typeName, "be") {
		baseType = typeName[:len(typeName)-2]
	}

	switch baseType {
	case "u1", "u2", "u4", "u8", "s1", "s2", "s4", "s8", "f4", "f8":
		return true
	case "str", "strz", "bytes":
		return true
	default:
		// Check for bit field types (b1, b2, b3, etc.)
		bitTypeRegex := regexp.MustCompile(`^b(\d+)$`)
		if bitTypeRegex.MatchString(baseType) {
			return true
		}
		return false
	}
}

// serializeField serializes a single field
func (k *KaitaiSerializer) serializeField(goCtx context.Context, field SequenceItem, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing field", "field_id", field.ID, "field_type", field.Type)
	select {
	case <-goCtx.Done():
		return goCtx.Err()
	default:
	}
	// Check if field has a condition
	if field.IfExpr != "" {
		k.logger.DebugContext(goCtx, "Evaluating if condition for field", "field_id", field.ID, "if_expr", field.IfExpr)
		result, err := k.evaluateExpression(goCtx, field.IfExpr, sCtx)
		if err != nil {
			return fmt.Errorf("evaluating if condition for field '%s' ('%s'): %w", field.ID, field.IfExpr, err)
		}

		// Skip field if condition is false
		if !isTrue(result) {
			return nil
		}
	}
	// If data is nil here and the field is not conditional (or condition was true),
	// subsequent type-specific serialization might fail. This is generally okay as
	// it implies missing mandatory data.

	// Handle repeat attribute
	if field.Repeat != "" {
		return k.serializeRepeatedField(goCtx, field, data, sCtx)
	}

	// Handle enum fields - extract numeric value from enum object
	if field.Enum != "" {
		return k.serializeEnumField(goCtx, field, data, sCtx)
	}

	// Handle contents attribute
	if field.Contents != nil {
		return k.serializeContentsField(goCtx, field, sCtx)
	}

	// Handle process first (before type-specific handling)
	if field.Process != "" {
		return k.serializeProcessedField(goCtx, field, data, sCtx)
	}

	// Handle string and bytes fields
	if field.Type == "str" || field.Type == "strz" {
		return k.serializeStringField(goCtx, field, data, sCtx)
	}

	if field.Type == "bytes" {
		return k.serializeBytesField(goCtx, field, data, sCtx)
	}

	// Explicitly handle fields defined with type: switch
	if field.Type == "switch" {
		k.logger.DebugContext(goCtx, "Field is a switch type, resolving actual type for serialization", "field_id", field.ID)
		if field.Switch == nil {
			// This case should ideally be caught by schema validation earlier,
			// but good to have a check here.
			return fmt.Errorf("field '%s' is of type 'switch' but has no 'switch-on' definition", field.ID)
		}
		// sCtx is the context of the parent type containing this switch field.
		// The switch-on expression will be evaluated against sCtx.
		actualType, err := k.resolveSwitchTypeForSerialization(goCtx, field.Switch, sCtx)
		if err != nil {
			return fmt.Errorf("resolving switch type for field '%s': %w", field.ID, err)
		}
		k.logger.DebugContext(goCtx, "Switch field resolved for serialization", "field_id", field.ID, "original_type", field.Type, "resolved_type", actualType)
		// Now serialize using the resolved actualType.
		return k.serializeType(goCtx, actualType, data, sCtx)
	}

	k.logger.DebugContext(goCtx, "Recursively serializing field's defined type", "field_id", field.ID, "defined_type", field.Type)

	// Default: serialize as a type
	return k.serializeType(goCtx, getTypeAsString(field.Type), data, sCtx)
}

// serializeRepeatedField handles serialization of repeated fields
func (k *KaitaiSerializer) serializeRepeatedField(goCtx context.Context, field SequenceItem, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing repeated field", "field_id", field.ID, "repeat_type", field.Repeat, "repeat_expr", field.RepeatExpr)
	items, ok := data.([]any)
	if !ok {
		return fmt.Errorf("expected array for repeated field '%s', got %T", field.ID, data)
	}

	// Determine repeat count
	var expectedCount int
	if field.RepeatExpr != "" {
		// Evaluate repeat expression
		k.logger.DebugContext(goCtx, "Evaluating repeat expression", "field_id", field.ID, "expr", field.RepeatExpr, "available_vars", fmt.Sprintf("%v", maps.Keys(sCtx.Children)))
		result, err := k.evaluateExpression(goCtx, field.RepeatExpr, sCtx)
		if err != nil {
			return fmt.Errorf("evaluating repeat expression for field '%s' ('%s'): %w", field.ID, field.RepeatExpr, err)
		}
		k.logger.DebugContext(goCtx, "Evaluated repeat expression for count", "field_id", field.ID, "expr_result", result)

		// Convert to int
		switch v := result.(type) {
		case int:
			expectedCount = v
		case int64:
			expectedCount = int(v)
		case float64:
			expectedCount = int(v)
		case uint8:
			expectedCount = int(v)
		case uint16:
			expectedCount = int(v)
		case uint32:
			expectedCount = int(v)
		case uint64:
			expectedCount = int(v)
		default:
			k.logger.ErrorContext(goCtx, "Repeat expression result is not a number", "field_id", field.ID, "result_type", fmt.Sprintf("%T", result))
			return fmt.Errorf("repeat expression for field '%s' ('%s') result is not a number: %v", field.ID, field.RepeatExpr, result)
		}

		// Validate count
		if len(items) != expectedCount {
			k.logger.ErrorContext(goCtx, "Array length mismatch for repeated field", "field_id", field.ID, "actual_len", len(items), "expected_count", expectedCount)
			return fmt.Errorf("array length %d for field '%s' doesn't match expected repeat count %d from expression '%s'", len(items), field.ID, expectedCount, field.RepeatExpr)
		}
	}
	k.logger.DebugContext(goCtx, "Determined item count for repeated field", "field_id", field.ID, "count", len(items))

	// Serialize each item
	for i, item := range items {
		select {
		case <-goCtx.Done():
			k.logger.InfoContext(goCtx, "Serialization of repeated field cancelled", "field_id", field.ID, "item_index", i)
			return goCtx.Err()
		default:
		}
		k.logger.DebugContext(goCtx, "Serializing repeated item", "field_id", field.ID, "item_index", i)
		itemField := field
		itemField.Repeat = ""
		itemField.RepeatExpr = ""

		if err := k.serializeField(goCtx, itemField, item, sCtx); err != nil {
			return fmt.Errorf("serializing item %d of repeated field '%s': %w", i, field.ID, err)
		}
	}
	k.logger.DebugContext(goCtx, "Finished serializing repeated field", "field_id", field.ID, "item_count", len(items))

	return nil
}

// serializeContentsField handles serialization of fields with fixed content
func (k *KaitaiSerializer) serializeContentsField(goCtx context.Context, field SequenceItem, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing contents field", "field_id", field.ID)
	select {
	case <-goCtx.Done():
		return goCtx.Err()
	default:
	}
	var expected []byte

	switch v := field.Contents.(type) {
	case []any:
		// Array of byte values
		expected = make([]byte, len(v))
		for i, b := range v {
			if val, ok := b.(float64); ok {
				expected[i] = byte(val)
			} else {
				return fmt.Errorf("invalid content byte value for field '%s': %v", field.ID, b)
			}
		}
	case string:
		expected = []byte(v)
	default:
		return fmt.Errorf("unsupported contents type for field '%s': %T", field.ID, v)
	}
	k.logger.DebugContext(goCtx, "Expected contents for field", "field_id", field.ID, "expected_bytes", fmt.Sprintf("%x", expected))

	// Write the fixed content
	if err := sCtx.Writer.WriteBytes(expected); err != nil {
		return fmt.Errorf("writing content bytes for field '%s': %w", field.ID, err)
	}
	k.logger.DebugContext(goCtx, "Contents field serialized successfully", "field_id", field.ID)

	return nil
}

// serializeStringField handles serialization of string fields
func (k *KaitaiSerializer) serializeStringField(goCtx context.Context, field SequenceItem, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing string field", "field_id", field.ID, "encoding_from_field", field.Encoding)
	select {
	case <-goCtx.Done():
		return goCtx.Err()
	default:
	}

	str, ok := data.(string)
	if !ok {
		return fmt.Errorf("expected string for field '%s', got %T", field.ID, data)
	}

	// Determine encoding
	encoding := field.Encoding
	if encoding == "" {
		encoding = k.schema.Meta.Encoding
	}
	if encoding == "" {
		encoding = "UTF-8" // Default encoding
	}
	k.logger.DebugContext(goCtx, "Using encoding for string field", "field_id", field.ID, "encoding", encoding)
	// Convert string to bytes based on encoding
	var strBytes []byte
	// var err error

	if strings.ToUpper(encoding) == "ASCII" || strings.ToUpper(encoding) == "UTF-8" || strings.ToUpper(encoding) == "UTF8" {
		strBytes = []byte(str)
	} else {
		// Handle other encodings using CEL
		k.logger.DebugContext(goCtx, "Encoding string using CEL 'encodeString'", "field_id", field.ID, "encoding", encoding)
		program, err := k.expressionPool.GetExpression("encodeString(input, encoding)")
		if err != nil {
			return fmt.Errorf("compiling encodeString expression for field '%s': %w", field.ID, err)
		}

		result, err := k.expressionPool.EvaluateExpression(program, map[string]any{
			"input":    str,
			"encoding": encoding,
		})
		if err != nil {
			return fmt.Errorf("encoding string for field '%s' with encoding '%s': %w", field.ID, encoding, err)
		}

		bytesResult, ok := result.([]byte)
		if !ok {
			return fmt.Errorf("encodeString for field '%s' didn't return bytes: %T", field.ID, result)
		}
		strBytes = bytesResult
	}

	// Handle size attribute
	if field.Size != nil {
		var size int
		switch v := field.Size.(type) {
		case int:
			size = v
		case float64:
			size = int(v)
		case string:
			// Size is an expression
			k.logger.DebugContext(goCtx, "Evaluating size expression for string field", "field_id", field.ID, "size_expr", v)
			result, err := k.evaluateExpression(goCtx, v, sCtx)
			if err != nil {
				return fmt.Errorf("evaluating size expression for string field '%s' ('%s'): %w", field.ID, v, err)
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
				k.logger.ErrorContext(goCtx, "Size expression for string field result is not a number", "field_id", field.ID, "size_expr", v, "result_type", fmt.Sprintf("%T", result))
				return fmt.Errorf("size expression for string field '%s' ('%s') result is not a number: %v", field.ID, v, result)
			}
		default:
			return fmt.Errorf("unsupported size type for string field '%s': %T", field.ID, v)
		}
		k.logger.DebugContext(goCtx, "Determined size for string field", "field_id", field.ID, "size", size)

		// Validate or adjust length
		if len(strBytes) > size {
			strBytes = strBytes[:size] // Truncate
		} else if len(strBytes) < size {
			// Pad with zeros
			padding := make([]byte, size-len(strBytes))
			strBytes = append(strBytes, padding...)
		}
	}

	// Write string bytes
	if field.Type == "strz" {
		// Add null terminator
		strBytes = append(strBytes, 0)
	}

	if err := sCtx.Writer.WriteBytes(strBytes); err != nil {
		return fmt.Errorf("writing string bytes for field '%s': %w", field.ID, err)
	}
	k.logger.DebugContext(goCtx, "String field serialized", "field_id", field.ID, "bytes_written", len(strBytes))

	return nil
}

// serializeBytesField handles serialization of bytes fields
func (k *KaitaiSerializer) serializeBytesField(goCtx context.Context, field SequenceItem, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing bytes field", "field_id", field.ID)
	select {
	case <-goCtx.Done():
		return goCtx.Err()
	default:
	}

	var bytesData []byte

	switch v := data.(type) {
	case []byte:
		bytesData = v
	case string:
		k.logger.DebugContext(goCtx, "Converting string data to bytes for bytes field", "field_id", field.ID)
		bytesData = []byte(v)
	default:
		return fmt.Errorf("expected bytes or string for field '%s', got %T", field.ID, data)

	}

	// Handle size attribute
	if field.Size != nil {
		var size int
		switch v := field.Size.(type) {
		case int:
			size = v
		case float64:
			size = int(v)
		case string:
			// Size is an expression
			k.logger.DebugContext(goCtx, "Evaluating size expression for bytes field", "field_id", field.ID, "size_expr", v)
			result, err := k.evaluateExpression(goCtx, v, sCtx)
			if err != nil {
				return fmt.Errorf("evaluating size expression for bytes field '%s' ('%s'): %w", field.ID, v, err)
			}

			// Convert to int
			switch r := result.(type) {
			case int:
				size = r
			case int64:
				size = int(r)
			case float64:
				size = int(r)
			default:
				k.logger.ErrorContext(goCtx, "Size expression for bytes field result is not a number", "field_id", field.ID, "size_expr", v, "result_type", fmt.Sprintf("%T", result))
				return fmt.Errorf("size expression for bytes field '%s' ('%s') result is not a number: %v", field.ID, v, result)
			}
		default:
			return fmt.Errorf("unsupported size type for bytes field '%s': %T", field.ID, v)
		}
		k.logger.DebugContext(goCtx, "Determined size for bytes field", "field_id", field.ID, "size", size)

		// Validate or adjust length
		if len(bytesData) > size {
			bytesData = bytesData[:size] // Truncate
		} else if len(bytesData) < size {
			// Pad with zeros
			padding := make([]byte, size-len(bytesData))
			bytesData = append(bytesData, padding...)
		}
	}

	// Write bytes
	if err := sCtx.Writer.WriteBytes(bytesData); err != nil {
		return fmt.Errorf("writing bytes for field '%s': %w", field.ID, err)
	}
	k.logger.DebugContext(goCtx, "Bytes field serialized", "field_id", field.ID, "bytes_written", len(bytesData))

	return nil
}

// serializeProcessedField handles serialization of processed fields
func (k *KaitaiSerializer) serializeProcessedField(goCtx context.Context, field SequenceItem, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing processed field", "field_id", field.ID, "process_spec", field.Process)
	// First, serialize to a buffer
	fieldCtx := &SerializeContext{
		Value:    data,
		Children: sCtx.Children,                          // Children from the parent context of the field being processed
		Parent:   sCtx.Parent,                            // Parent context
		Root:     sCtx.Root,                              // Root context
		Writer:   kaitai.NewWriter(bytes.NewBuffer(nil)), // Temporary writer
	}

	// Serialize the field without processing
	fieldCopy := field
	fieldCopy.Process = "" // Temporarily remove process to get raw serialized form
	k.logger.DebugContext(goCtx, "Serializing field to temp buffer before reverse processing", "field_id", field.ID)
	if err := k.serializeField(goCtx, fieldCopy, data, fieldCtx); err != nil {
		return fmt.Errorf("serializing field '%s' to temp buffer before processing: %w", field.ID, err)
	}

	// Get the serialized bytes

	serialized, err := getWriterBuffer(fieldCtx.Writer)
	if err != nil {
		return fmt.Errorf("failed to get writer buffer: %w", err)
	}
	k.logger.DebugContext(goCtx, "Got raw serialized bytes for processed field", "field_id", field.ID, "raw_bytes_len", len(serialized))

	// Reverse the process
	processed, err := k.reverseProcess(goCtx, serialized, field.Process, sCtx) // Pass sCtx for expression evaluation within reverseProcess
	if err != nil {
		return fmt.Errorf("reversing process for field '%s' (spec: '%s'): %w", field.ID, field.Process, err)
	}
	k.logger.DebugContext(goCtx, "Reverse processed data for field", "field_id", field.ID, "processed_bytes_len", len(processed))

	// Write the processed data
	if err := sCtx.Writer.WriteBytes(processed); err != nil {
		return fmt.Errorf("writing processed field '%s': %w", field.ID, err)
	}
	k.logger.DebugContext(goCtx, "Processed field serialized successfully", "field_id", field.ID)

	return nil
}

// reverseProcess applies the reverse of a process function
func (k *KaitaiSerializer) reverseProcess(goCtx context.Context, data []byte, processSpec string, sCtx *SerializeContext) ([]byte, error) {
	k.logger.DebugContext(goCtx, "Reversing process", "process_spec", processSpec, "input_data_len", len(data))
	select {
	case <-goCtx.Done():
		return nil, goCtx.Err()
	default:
	}

	// Parse the process spec (e.g., "xor(0x5F)" or "zlib")
	var processFn, paramStr string
	if strings.Contains(processSpec, "(") {
		parts := strings.Split(processSpec, "(")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid process specification format: '%s'", processSpec)
		}
		processFn = parts[0]
		paramStr = strings.TrimRight(parts[1], ")")
	} else {
		// No parentheses - just the function name
		processFn = processSpec
		paramStr = ""
	}

	// Create CEL expression for reverse process
	var expr string
	switch processFn {
	case "xor":
		// XOR is its own inverse
		expr = fmt.Sprintf("processXOR(data, %s)", paramStr)
	case "zlib":
		// For serialization, if stored data is zlib-decompressed, we need to zlib-compress.
		expr = "processZlibCompress(data)"
	case "rotate", "rol":
		// If original process is rotate_left(N), reverse is rotate_right(N)
		expr = fmt.Sprintf("processRotateRight(data, %s)", paramStr)
	case "ror":
		// If original process is rotate_right(N), reverse is rotate_left(N)
		expr = fmt.Sprintf("processRotateLeft(data, %s)", paramStr)
	default:
		return nil, fmt.Errorf("unknown process function for reverse: %s", processFn)
	}
	k.logger.DebugContext(goCtx, "Constructed CEL expression for reverse process", "cel_expr", expr)

	// Compile and evaluate CEL expression
	program, err := k.expressionPool.GetExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("compiling reverse process expression '%s': %w", expr, err)
	}

	// Create a complete activation map similar to what the parser does
	evalMap := make(map[string]any)
	if sCtx.Children != nil {
		maps.Copy(evalMap, sCtx.Children)
	}
	if sCtx.Parent != nil {
		evalMap["_parent"] = sCtx.Parent.Children // Expose parent's children map
	}
	if sCtx.Root != nil {
		evalMap["_root"] = sCtx.Root.Children // Expose root's children map
	}
	// Note: _io is not available in serialization context
	evalMap["data"] = data // Add the data to be processed as 'data'

	result, err := k.expressionPool.EvaluateExpression(program, evalMap)
	if err != nil {
		return nil, fmt.Errorf("evaluating reverse process expression '%s': %w", expr, err)
	}

	// Convert result to bytes
	if bytesResult, ok := result.([]byte); ok {
		k.logger.DebugContext(goCtx, "Reverse process successful", "process_spec", processSpec, "output_data_len", len(bytesResult))
		return bytesResult, nil
	}
	k.logger.ErrorContext(goCtx, "Reverse process result is not bytes", "process_spec", processSpec, "result_type", fmt.Sprintf("%T", result))
	return nil, fmt.Errorf("reverse process expression '%s' result is not bytes: %T", expr, result)
}

// evaluateInstancesWithDependencies evaluates instances using multi-pass approach like the parser
func (k *KaitaiSerializer) evaluateInstancesWithDependencies(goCtx context.Context, instances map[string]InstanceDef, sCtx *SerializeContext) error {
	if len(instances) == 0 {
		return nil
	}
	
	instancesToProcess := make(map[string]InstanceDef)
	maps.Copy(instancesToProcess, instances)
	
	maxPasses := len(instancesToProcess) + 2 // Allow a couple of extra passes for dependencies
	processedInLastPass := -1
	
	for pass := 0; pass < maxPasses && len(instancesToProcess) > 0 && processedInLastPass != 0; pass++ {
		k.logger.DebugContext(goCtx, "Instance evaluation pass", "pass_num", pass+1, "remaining_instances", len(instancesToProcess))
		processedInThisPass := 0
		successfullyProcessedThisPass := make(map[string]bool)
		
		for name, inst := range instancesToProcess {
			k.logger.DebugContext(goCtx, "Attempting to evaluate instance", "instance_name", name, "pass", pass+1, "available_variables", fmt.Sprintf("%v", maps.Keys(sCtx.Children)), "instance_expr", inst.Value)
			
			val, err := k.evaluateExpression(goCtx, inst.Value, sCtx)
			if err != nil {
				k.logger.DebugContext(goCtx, "Instance evaluation attempt failed (may retry)", "instance_name", name, "pass", pass+1, "error", err)
			} else {
				k.logger.DebugContext(goCtx, "Instance evaluated successfully", "instance_name", name, "value", val)
				sCtx.Children[name] = val
				successfullyProcessedThisPass[name] = true
				processedInThisPass++
			}
		}
		
		// Remove successfully processed instances
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
		return fmt.Errorf("failed to evaluate all instances after %d passes; remaining: %v. Check for circular dependencies or unresolvable expressions", maxPasses-1, strings.Join(remainingInstanceNames, ", "))
	}
	
	return nil
}



// evaluateExpression evaluates a Kaitai expression using CEL
func (k *KaitaiSerializer) evaluateExpression(goCtx context.Context, kaitaiExpr string, sCtx *SerializeContext) (any, error) {
	return k.evaluateExpressionWithGuard(goCtx, kaitaiExpr, sCtx, make(map[string]bool))
}

// evaluateExpressionWithGuard evaluates a Kaitai expression with recursion guard
func (k *KaitaiSerializer) evaluateExpressionWithGuard(goCtx context.Context, kaitaiExpr string, sCtx *SerializeContext, evaluating map[string]bool) (any, error) {
	k.logger.DebugContext(goCtx, "Evaluating CEL expression for serialization", "kaitai_expr", kaitaiExpr)
	select {
	case <-goCtx.Done():
		return nil, goCtx.Err()
	default:
	}
	// Get or compile expression
	program, err := k.expressionPool.GetExpression(kaitaiExpr)
	if err != nil {
		return nil, fmt.Errorf("compiling expression '%s': %w", kaitaiExpr, err)
	}

	// Create activation from context
	activation, err := sCtx.AsActivation()
	if err != nil {
		return nil, fmt.Errorf("failed to create activation for serialization: %w", err)
	}

	// Evaluate expression
	result, _, err := program.Eval(activation)
	if err != nil {
		// Check if this is a "no such attribute" error that might be resolved by evaluating an instance on-demand
		errStr := err.Error()
		if strings.Contains(errStr, "no such attribute(s):") {
			// Extract the missing attribute name from the error
			// Error format: "no such attribute(s): attr_name"
			parts := strings.Split(errStr, "no such attribute(s): ")
			if len(parts) > 1 {
				missingAttr := strings.TrimSpace(parts[1])
				k.logger.DebugContext(goCtx, "Attempting on-demand instance evaluation", "missing_attr", missingAttr, "kaitai_expr", kaitaiExpr)
				
				// Check root-level instances first
				if k.schema.Instances != nil {
					if inst, exists := k.schema.Instances[missingAttr]; exists {
						// Check if we're already evaluating this instance to prevent recursion
						if !evaluating[missingAttr] {
							evaluating[missingAttr] = true
							instanceResult, instanceErr := k.evaluateExpressionWithGuard(goCtx, inst.Value, sCtx, evaluating)
							delete(evaluating, missingAttr)
							
							if instanceErr == nil {
								k.logger.DebugContext(goCtx, "Successfully evaluated root instance on-demand", "instance_name", missingAttr, "value", instanceResult)
								sCtx.Children[missingAttr] = instanceResult
								
								// Recreate activation with the new instance and retry
								activation, activationErr := sCtx.AsActivation()
								if activationErr == nil {
									retryResult, _, retryErr := program.Eval(activation)
									if retryErr == nil {
										k.logger.DebugContext(goCtx, "CEL expression evaluated successfully after on-demand instance", "kaitai_expr", kaitaiExpr, "result", retryResult.Value())
										return retryResult.Value(), nil
									}
								}
							} else {
								k.logger.DebugContext(goCtx, "Failed to evaluate root instance on-demand", "instance_name", missingAttr, "error", instanceErr)
							}
						} else {
							k.logger.DebugContext(goCtx, "Skipping on-demand instance due to recursion guard", "instance_name", missingAttr, "kaitai_expr", kaitaiExpr)
						}
					}
				}
			}
		}
		
		return nil, fmt.Errorf("evaluating expression '%s': %w", kaitaiExpr, err)
	}
	k.logger.DebugContext(goCtx, "CEL expression evaluated successfully for serialization", "kaitai_expr", kaitaiExpr, "result", result.Value(), "result_type", fmt.Sprintf("%T", result.Value()))

	return result.Value(), nil
}

// resolveSwitchTypeForSerialization resolves the actual type for a field based on a switch-on expression.
func (k *KaitaiSerializer) resolveSwitchTypeForSerialization(goCtx context.Context, switchType any, sCtx *SerializeContext) (string, error) {
	// Check if switchType is a direct expression string
	if switchOnExprStr, isExprString := switchType.(string); isExprString {
		k.logger.DebugContext(goCtx, "Resolving switch type from direct expression string", "switch_on_expr", switchOnExprStr)
		switchOnVal, err := k.evaluateExpression(goCtx, switchOnExprStr, sCtx)
		if err != nil {
			return "", fmt.Errorf("evaluating switch-on expression string '%s': %w", switchOnExprStr, err)
		}
		typeNameResult, ok := switchOnVal.(string)
		if !ok {
			return "", fmt.Errorf("switch-on expression string '%s' was expected to directly yield a type name (string), but got type %T (value: %v)", switchOnExprStr, switchOnVal, switchOnVal)
		}
		k.logger.DebugContext(goCtx, "Switch-on expression string directly resolved to type", "switch_on_expr", switchOnExprStr, "resolved_type", typeNameResult)
		return typeNameResult, nil
	} else {
		// Proceed with map-based switch definition
		spec, err := NewSwitchTypeSelector(switchType, k.schema)
		if err != nil {
			return "", fmt.Errorf("failed to create switch type selector: %w", err)
		}
		if spec == nil || spec.switchOn == "" {
			return "", fmt.Errorf("switch specification is nil or switch-on expression is empty")
		}
		k.logger.DebugContext(goCtx, "Resolving switch type for serialization from map definition", "switch_on_expr", spec.switchOn)

		switchOnVal, err := k.evaluateExpression(goCtx, spec.switchOn, sCtx)
		if err != nil {
			return "", fmt.Errorf("evaluating switch-on expression '%s': %w", spec.switchOn, err)
		}
		k.logger.DebugContext(goCtx, "Switch-on expression evaluated", "switch_on_expr", spec.switchOn, "value", switchOnVal)

		// If spec.cases is nil (should not happen if NewSwitchTypeSelector parsed a map with cases),
		// this indicates an issue or a very specific KSY structure not yet fully handled.
		// However, NewSwitchTypeSelector would likely error if 'cases' is missing from a map.
		// For safety, we can check, but the primary path here is for map-based cases.
		if spec.cases == nil {
			return "", fmt.Errorf("internal error: switch specification parsed as map but cases are nil for switch-on '%s'", spec.switchOn)
		}

		var switchKey string
		switch v := switchOnVal.(type) {
		case string:
			switchKey = v
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			switchKey = fmt.Sprintf("%d", v)
		case bool:
			switchKey = fmt.Sprintf("%t", v)
		case float32, float64:
			switchKey = fmt.Sprintf("%g", v)
		default:
			k.logger.WarnContext(goCtx, "Switch-on value is of an unexpected type for map case lookup, attempting generic string conversion", "type", fmt.Sprintf("%T", v))
			switchKey = fmt.Sprintf("%v", v)
		}

		if actualType, ok := spec.cases[switchKey]; ok {
			k.logger.DebugContext(goCtx, "Switch case matched", "switch_key", switchKey, "resolved_type", actualType)
			return actualType, nil
		}

		if defaultType, ok := spec.cases["_"]; ok {
			k.logger.DebugContext(goCtx, "Switch case not matched, using default", "switch_key", switchKey, "default_type", defaultType, "switch_on_expr", spec.switchOn, "evaluated_value", switchOnVal)
			return defaultType, nil
		}

		k.logger.ErrorContext(goCtx, "No case matched for switch-on value and no default case provided",
			"switch_on_expr", spec.switchOn, "evaluated_value", switchOnVal,
			"string_key_used", switchKey, "available_cases", fmt.Sprintf("%v", spec.cases))
		return "", fmt.Errorf("no case matching switch value '%v' (key: '%s') for expression '%s' and no default '_' case was found",
			switchOnVal, switchKey, spec.switchOn)
	}
}

// resolveTypeInHierarchy resolves a type name by searching through nested type scopes
func (k *KaitaiSerializer) resolveTypeInHierarchy(typeName string) (*Type, bool) {
	// Try to resolve in current nested type context first
	// Walk up the type stack to find the type in nested scopes
	for i := len(k.typeStack) - 1; i >= 0; i-- {
		currentTypeName := k.typeStack[i]
		if currentType, found := k.schema.Types[currentTypeName]; found {
			// Check if the type has nested types
			if currentType.Types != nil {
				if nestedType, found := currentType.Types[typeName]; found {
					return nestedType, true
				}
			}
		}
	}

	// Fall back to global type lookup
	if globalType, found := k.schema.Types[typeName]; found {
		return &globalType, true
	}

	return nil, false
}

// serializeEnumField handles serialization of enum fields
func (k *KaitaiSerializer) serializeEnumField(goCtx context.Context, field SequenceItem, data any, sCtx *SerializeContext) error {
	k.logger.DebugContext(goCtx, "Serializing enum field", "field_id", field.ID, "enum_name", field.Enum, "field_type", field.Type)
	
	// Handle different enum data formats
	var enumValue any
	
	switch v := data.(type) {
	case map[string]any:
		// Enum object format: {"name": "cat", "value": 4, "valid": true}
		if val, exists := v["value"]; exists {
			enumValue = val
			k.logger.DebugContext(goCtx, "Extracted enum value from object", "field_id", field.ID, "enum_value", enumValue)
		} else {
			return fmt.Errorf("enum object for field '%s' missing 'value' field", field.ID)
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// Direct numeric value
		enumValue = v
		k.logger.DebugContext(goCtx, "Using direct numeric value for enum", "field_id", field.ID, "enum_value", enumValue)
	case float32, float64:
		// Float that should be an integer
		enumValue = v
		k.logger.DebugContext(goCtx, "Using numeric value for enum", "field_id", field.ID, "enum_value", enumValue)
	default:
		return fmt.Errorf("unsupported enum data format for field '%s': %T", field.ID, data)
	}
	
	// Now serialize the numeric value using the field's base type
	return k.serializeType(goCtx, getTypeAsString(field.Type), enumValue, sCtx)
}

// Type conversion helpers

// getWriterBuffer accesses the buffer from a kaitai.Writer
func getWriterBuffer(writer *kaitai.Writer) ([]byte, error) {
	if buf, ok := writer.Writer.(*bytes.Buffer); ok {
		return buf.Bytes(), nil
	}
	return nil, fmt.Errorf("writer doesn't support buffer access")
}
