package kaitaistruct

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	internalCel "github.com/twinfer/kbin-plugin/internal/cel"
)

// KaitaiSerializer provides functionality to serialize objects according to Kaitai schema
type KaitaiSerializer struct {
	schema          *KaitaiSchema
	expressionPool  *internalCel.ExpressionPool
	processRegistry *ProcessRegistry
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
		processRegistry: NewProcessRegistry(),
		logger:          log,
	}, nil
}

// AsActivation creates a CEL activation from the serialization context
func (ctx *SerializeContext) AsActivation() (cel.Activation, error) {
	vars := make(map[string]any)

	// Add current context values
	if ctx.Children != nil {
		for k, v := range ctx.Children {
			vars[k] = v
		}
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

	// Check if it's a built-in type
	if handled, err := k.serializeBuiltinType(goCtx, typeName, data, sCtx.Writer); handled {
		if err != nil {
			k.logger.ErrorContext(goCtx, "Error serializing built-in type", "type_name", typeName, "error", err)
		}
		return err
	}

	// Look for the type in schema
	if typeName == k.schema.Meta.ID {
		// Serialize root level fields
		dataMap, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("expected map for root type, got %T", data)
		}

		// Serialize sequence fields
		for _, seq := range k.schema.Seq {
			if err := k.serializeField(goCtx, seq, dataMap[seq.ID], sCtx); err != nil {
				return fmt.Errorf("serializing field '%s' in root type '%s': %w", seq.ID, typeName, err)
			}
		}

		return nil
	}

	// Try to find the type in the schema types
	typeObj, found := k.schema.Types[typeName]
	if !found {
		k.logger.ErrorContext(goCtx, "Unknown type for serialization", "type_name", typeName)
		return fmt.Errorf("unknown type: %s", typeName)
	}

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

	// Serialize sequence fields
	for _, seq := range typeObj.Seq {
		// Handle switch types
		if seq.Type == "switch" {
			k.logger.DebugContext(goCtx, "Handling switch type for field", "field_id", seq.ID, "type_name", typeName)
			// Resolve switch type
			actualType, err := k.resolveSwitchTypeForSerialization(goCtx, seq.Switch, fieldCtx) // Pass fieldCtx which contains the necessary data for expression evaluation
			if err != nil {
				return fmt.Errorf("error resolving switch type for field '%s': %w", seq.ID, err)
			}

			// Override type for this field
			seqCopy := seq
			seqCopy.Type = actualType

			// Serialize field with resolved type
			fieldData, dataOk := dataMap[seq.ID]
			if !dataOk && actualType != "" { // Allow empty if resolved type is effectively "skip" or not present
				// This might be an error or expected, depending on schema. For now, log and continue if actualType is empty.
				k.logger.WarnContext(goCtx, "Data for switch field not found, but type resolved", "field_id", seq.ID, "resolved_type", actualType)
			}
			if err := k.serializeField(goCtx, seqCopy, fieldData, fieldCtx); err != nil {
				return fmt.Errorf("serializing switch field '%s' (resolved as '%s') in type '%s': %w", seq.ID, actualType, typeName, err)
			}
			continue
		}

		// Handle normal field
		fieldData, dataOk := dataMap[seq.ID]
		if !dataOk {
			// If field has an 'if' condition, it might be legitimately absent.
			// Otherwise, this is likely an error or an optional field not provided.
			if seq.IfExpr == "" { // Only warn if not conditional
				k.logger.WarnContext(goCtx, "Data for non-conditional field not found in input map", "field_id", seq.ID, "type_name", typeName)
			} else {
				k.logger.DebugContext(goCtx, "Data for conditional field not found, will be handled by 'if' expr", "field_id", seq.ID, "type_name", typeName)
			}
			// `serializeField` will handle the `if` condition. If data is nil and `if` is false, it's skipped.
			// If `if` is true and data is nil, `serializeField` might error depending on the field type.
		}
		if err := k.serializeField(goCtx, seq, fieldData, fieldCtx); err != nil {
			return fmt.Errorf("error serializing field '%s' in type '%s': %w", seq.ID, typeName, err)
		}
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
	// Parse switch type format: switch-on:expression:default_type
	parts := strings.Split(typeName, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid switch type format: %s", typeName)
	}

	// Evaluate switch expression
	switchExpr := parts[1]
	switchValue, err := k.evaluateExpression(goCtx, switchExpr, sCtx)
	if err != nil {
		return fmt.Errorf("evaluating ad-hoc switch expression '%s': %w", switchExpr, err)
	}

	// Determine actual type based on switch value
	// TODO: Implement proper switch case mapping
	var actualType string
	if len(parts) > 2 {
		actualType = parts[2] // Use default type from the typeName string
	} else {
		// This ad-hoc format doesn't have explicit cases in the typeName string itself.
		// It relies on the expression evaluating to a type name or using the default.
		// If the expression itself is supposed to yield the type name:
		if svStr, ok := switchValue.(string); ok {
			actualType = svStr
		} else {
			k.logger.ErrorContext(goCtx, "No default type in ad-hoc switch and expression did not yield type name", "type_name_switch", typeName, "switch_expr", switchExpr, "switch_value", switchValue)
			return fmt.Errorf("no default type in ad-hoc switch '%s' and expression '%s' (value: %v) did not yield a type name", typeName, switchExpr, switchValue)
		}
	}
	k.logger.DebugContext(goCtx, "Ad-hoc switch resolved", "original_type_switch", typeName, "switch_expr", switchExpr, "switch_value", switchValue, "resolved_type", actualType)

	// Serialize using actual type
	return k.serializeType(goCtx, actualType, data, sCtx)
}

// serializeBuiltinType handles serialization of built-in types
func (k *KaitaiSerializer) serializeBuiltinType(goCtx context.Context, typeName string, data any, writer *kaitai.Writer) (bool, error) {
	k.logger.DebugContext(goCtx, "Serializing built-in type", "type_name", typeName)
	// Handle standard types
	switch typeName {
	case "u1":
		val, err := toUint8(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU1(val)

	case "u2le":
		val, err := toUint16(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU2le(val)

	case "u4le":
		val, err := toUint32(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU4le(val)

	case "u8le":
		val, err := toUint64(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU8le(val)

	case "u2be":
		val, err := toUint16(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU2be(val)

	case "u4be":
		val, err := toUint32(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU4be(val)

	case "u8be":
		val, err := toUint64(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteU8be(val)

	case "s1":
		val, err := toInt8(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS1(val)

	case "s2le":
		val, err := toInt16(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS2le(val)

	case "s4le":
		val, err := toInt32(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS4le(val)

	case "s8le":
		val, err := toInt64(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS8le(val)

	case "s2be":
		val, err := toInt16(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS2be(val)

	case "s4be":
		val, err := toInt32(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS4be(val)

	case "s8be":
		val, err := toInt64(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteS8be(val)

	case "f4le":
		val, err := toFloat32(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteF4le(val)

	case "f8le":
		val, err := toFloat64(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteF8le(val)

	case "f4be":
		val, err := toFloat32(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteF4be(val)

	case "f8be":
		val, err := toFloat64(data)
		if err != nil {
			return true, err
		}
		return true, writer.WriteF8be(val)

	case "str", "strz":
		// Handled in serializeField
		return false, nil
	}

	// Handle type-specific endianness
	if strings.HasPrefix(typeName, "u") || strings.HasPrefix(typeName, "s") ||
		strings.HasPrefix(typeName, "f") {
		if !strings.HasSuffix(typeName, "le") && !strings.HasSuffix(typeName, "be") {
			endian := k.schema.Meta.Endian
			if endian == "" {
				endian = "be" // Default big-endian if not specified
			}
			newType := typeName + endian
			k.logger.DebugContext(goCtx, "Applying endianness to built-in type", "original_type", typeName, "new_type_with_endian", newType)
			return k.serializeBuiltinType(goCtx, newType, data, writer)
		}
	}
	k.logger.DebugContext(goCtx, "Type not a recognized built-in for direct serialization", "type_name", typeName)
	return false, nil
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

	// Handle contents attribute
	if field.Contents != nil {
		return k.serializeContentsField(goCtx, field, sCtx)
	}

	// Handle string and bytes fields
	if field.Type == "str" || field.Type == "strz" {
		return k.serializeStringField(goCtx, field, data, sCtx)
	}

	if field.Type == "bytes" {
		return k.serializeBytesField(goCtx, field, data, sCtx)
	}

	// Handle process
	if field.Process != "" {
		return k.serializeProcessedField(goCtx, field, data, sCtx)
	}

	k.logger.DebugContext(goCtx, "Recursively serializing field's defined type", "field_id", field.ID, "defined_type", field.Type)

	// Default: serialize as a type
	return k.serializeType(goCtx, field.Type, data, sCtx)
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

	// Parse the process spec (e.g., "xor(0x5F)")
	// This parsing is simple; more complex specs might need a proper parser.
	parts := strings.Split(processSpec, "(")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid process specification: %s", processSpec)
	}

	processFn := parts[0]
	paramStr := strings.TrimRight(parts[1], ")")

	// Create CEL expression for reverse process
	var expr string
	switch processFn {
	case "xor":
		// XOR is its own inverse
		expr = fmt.Sprintf("processXOR(input, %s)", paramStr)
	case "zlib":
		// For serialization, if stored data is zlib-decompressed, we need to zlib-compress.
		// Assuming a CEL function `processZlibCompress(input)` exists.
		expr = "processZlibCompress(input)" // Placeholder: ensure this CEL function exists
		k.logger.WarnContext(goCtx, "Zlib reverse process (compression) relies on 'processZlibCompress' CEL function", "process_spec", processSpec)
		// If no such function, this will fail at GetExpression or EvaluateExpression.
		// return nil, fmt.Errorf("zlib compression (reverse process) not fully implemented for serialization via CEL yet")
	case "rotate":
		// If original process is rotate_left(N), reverse is rotate_right(N) or rotate_left(-N).
		expr = fmt.Sprintf("processRotateRight(input, %s)", paramStr)
	default:
		return nil, fmt.Errorf("unknown process function for reverse: %s", processFn)
	}
	k.logger.DebugContext(goCtx, "Constructed CEL expression for reverse process", "cel_expr", expr)

	// Compile and evaluate CEL expression
	program, err := k.expressionPool.GetExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("compiling reverse process expression '%s': %w", expr, err)
	}

	// Create activation. Note: sCtx here is the context of the field *being processed*,
	// which might be different from the context where the process parameters (like XOR key) are defined.
	// If process parameters depend on other fields, sCtx should be the parent context of the field.
	// For now, assuming process parameters are literals or simple.

	result, err := k.expressionPool.EvaluateExpression(program, map[string]any{
		"input": data,
		// Potentially add other variables from sCtx if paramStr could be an expression
	})
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

// evaluateExpression evaluates a Kaitai expression using CEL
func (k *KaitaiSerializer) evaluateExpression(goCtx context.Context, kaitaiExpr string, sCtx *SerializeContext) (any, error) {
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
		return nil, fmt.Errorf("evaluating expression '%s': %w", kaitaiExpr, err)
	}
	k.logger.DebugContext(goCtx, "CEL expression evaluated successfully for serialization", "kaitai_expr", kaitaiExpr, "result", result.Value(), "result_type", fmt.Sprintf("%T", result.Value()))

	return result.Value(), nil
}

// resolveSwitchTypeForSerialization resolves the actual type for a field based on a switch-on expression.
func (k *KaitaiSerializer) resolveSwitchTypeForSerialization(goCtx context.Context, switchType any, sCtx *SerializeContext) (string, error) {
	spec, err := NewSwitchTypeSelector(switchType, k.schema)
	if err != nil {
		return "", fmt.Errorf("failed to create switch type selector: %w", err)
	}
	if spec == nil || spec.switchOn == "" {
		return "", fmt.Errorf("switch specification is nil")
	}
	k.logger.DebugContext(goCtx, "Resolving switch type for serialization", "switch_on_expr", spec.switchOn)

	switchOnVal, err := k.evaluateExpression(goCtx, spec.switchOn, sCtx)
	if err != nil {
		return "", fmt.Errorf("evaluating switch-on expression '%s': %w", spec.switchOn, err)
	}
	k.logger.DebugContext(goCtx, "Switch-on expression evaluated", "switch_on_expr", spec.switchOn, "value", switchOnVal)

	// If spec.cases is nil, it means the switch-on expression itself should evaluate to the type name.
	if spec.cases == nil {
		typeNameResult, ok := switchOnVal.(string)
		if !ok {
			return "", fmt.Errorf("switch-on expression '%s' was expected to directly yield a type name (string), but got type %T (value: %v)", spec.switchOn, switchOnVal, switchOnVal)
		}
		k.logger.DebugContext(goCtx, "Switch-on expression directly resolved to type", "switch_on_expr", spec.switchOn, "resolved_type", typeNameResult)
		return typeNameResult, nil
	}

	// Convert switchOnVal to string for map lookup.
	// Kaitai KSY usually implies string keys for cases, but values can be numbers.
	// CEL evaluation might return int, bool, string. We need a consistent key.
	var switchKey string
	switch v := switchOnVal.(type) {
	case string:
		switchKey = v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		switchKey = fmt.Sprintf("%d", v) // Convert numbers to string
	case bool:
		switchKey = fmt.Sprintf("%t", v) // Convert bools to "true" or "false"
	case float32, float64:
		// Kaitai cases usually don't use floats, but handle just in case
		switchKey = fmt.Sprintf("%g", v)
	default:
		// If it's another type, try a generic string conversion. This might be error-prone.
		k.logger.WarnContext(goCtx, "Switch-on value is of an unexpected type, attempting generic string conversion", "type", fmt.Sprintf("%T", v))
		switchKey = fmt.Sprintf("%v", v)
	}

	if actualType, ok := spec.cases[switchKey]; ok {
		k.logger.DebugContext(goCtx, "Switch case matched", "switch_key", switchKey, "resolved_type", actualType)
		return actualType, nil
	}

	// Check for default case "_"
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

// Type conversion helpers
func toUint8(data any) (byte, error) {
	switch v := data.(type) {
	case uint8:
		return v, nil
	case int:
		return byte(v), nil
	case int64:
		return byte(v), nil
	case float64:
		return byte(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint8", data)
	}
}

func toUint16(data any) (uint16, error) {
	switch v := data.(type) {
	case uint16:
		return v, nil
	case int:
		return uint16(v), nil
	case int64:
		return uint16(v), nil
	case float64:
		return uint16(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint16", data)
	}
}

func toUint32(data any) (uint32, error) {
	switch v := data.(type) {
	case uint32:
		return v, nil
	case int:
		return uint32(v), nil
	case int64:
		return uint32(v), nil
	case float64:
		return uint32(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint32", data)
	}
}

func toUint64(data any) (uint64, error) {
	switch v := data.(type) {
	case uint64:
		return v, nil
	case int:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", data)
	}
}

func toInt8(data any) (int8, error) {
	switch v := data.(type) {
	case int8:
		return v, nil
	case int:
		return int8(v), nil
	case int64:
		return int8(v), nil
	case float64:
		return int8(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int8", data)
	}
}

func toInt16(data any) (int16, error) {
	switch v := data.(type) {
	case int16:
		return v, nil
	case int:
		return int16(v), nil
	case int64:
		return int16(v), nil
	case float64:
		return int16(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int16", data)
	}
}

func toInt32(data any) (int32, error) {
	switch v := data.(type) {
	case int32:
		return v, nil
	case int:
		return int32(v), nil
	case int64:
		return int32(v), nil
	case float64:
		return int32(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int32", data)
	}
}

func toInt64(data any) (int64, error) {
	switch v := data.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", data)
	}
}

func toFloat32(data any) (float32, error) {
	switch v := data.(type) {
	case float32:
		return v, nil
	case float64:
		return float32(v), nil
	case int:
		return float32(v), nil
	case int64:
		return float32(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float32", data)
	}
}

func toFloat64(data any) (float64, error) {
	switch v := data.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", data)
	}
}

// getWriterBuffer accesses the buffer from a kaitai.Writer
func getWriterBuffer(writer *kaitai.Writer) ([]byte, error) {
	if buf, ok := writer.Writer.(*bytes.Buffer); ok {
		return buf.Bytes(), nil
	}
	return nil, fmt.Errorf("writer doesn't support buffer access")
}
