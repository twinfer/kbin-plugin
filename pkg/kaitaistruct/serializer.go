package kaitaistruct

import (
	"bytes"
	"fmt"
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
func NewKaitaiSerializer(schema *KaitaiSchema) (*KaitaiSerializer, error) {
	pool, err := internalCel.NewExpressionPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create expression pool: %w", err)
	}

	return &KaitaiSerializer{
		schema:          schema,
		expressionPool:  pool,
		processRegistry: NewProcessRegistry(),
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
func (k *KaitaiSerializer) Serialize(data map[string]any) ([]byte, error) {
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
	if err := k.serializeType(rootType, data, rootCtx); err != nil {
		return nil, fmt.Errorf("failed serializing root type '%s': %w", rootType, err)
	}

	return buf.Bytes(), nil
}

// serializeType serializes data according to a specified type
func (k *KaitaiSerializer) serializeType(typeName string, data any, ctx *SerializeContext) error {
	// Check if it's a switch type
	if strings.Contains(typeName, "switch-on:") {
		return k.serializeSwitchType(typeName, data, ctx)
	}

	// Check if it's a built-in type
	if handled, err := k.serializeBuiltinType(typeName, data, ctx.Writer); handled {
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
			if err := k.serializeField(seq, dataMap[seq.ID], ctx); err != nil {
				return fmt.Errorf("error serializing field '%s': %w", seq.ID, err)
			}
		}

		return nil
	}

	// Try to find the type in the schema types
	typeObj, found := k.schema.Types[typeName]
	if !found {
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
		Writer:   ctx.Writer,
		Parent:   ctx,
		Root:     ctx.Root,
	}

	// Serialize sequence fields
	for _, seq := range typeObj.Seq {
		// Handle switch types
		if seq.Type == "switch" {
			// Resolve switch type
			switchSelector, err := NewSwitchTypeSelector(seq.Switch, k.schema)
			if err != nil {
				return fmt.Errorf("error creating switch selector for field '%s': %w", seq.ID, err)
			}

			// Create a dummy ParseContext from SerializeContext for switchSelector
			// parseCtx := &ParseContext{
			// 	Value:    fieldCtx.Value,
			// 	Parent:   nil, // Set as needed if parent context is required
			// 	Root:     nil, // Set as needed if root context is required
			// 	Children: fieldCtx.Children,
			// }
			// actualType, err := switchSelector.ResolveType(parseCtx, nil) // We don't need the interpreter here

			actualType, err := switchSelector.ResolveType(nil, nil) // We don't need the interpreter here
			if err != nil {
				return fmt.Errorf("error resolving switch type for field '%s': %w", seq.ID, err)
			}

			// Override type for this field
			seqCopy := seq
			seqCopy.Type = actualType

			// Serialize field with resolved type
			if err := k.serializeField(seqCopy, dataMap[seq.ID], fieldCtx); err != nil {
				return fmt.Errorf("error serializing switch field '%s' with type '%s': %w", seq.ID, actualType, err)
			}
			continue
		}

		// Handle normal field
		if err := k.serializeField(seq, dataMap[seq.ID], fieldCtx); err != nil {
			return fmt.Errorf("error serializing field '%s' in type '%s': %w", seq.ID, typeName, err)
		}
	}

	return nil
}

// serializeSwitchType handles serialization of switch types
func (k *KaitaiSerializer) serializeSwitchType(typeName string, data any, ctx *SerializeContext) error {
	// Parse switch type format: switch-on:expression:default_type
	parts := strings.Split(typeName, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid switch type format: %s", typeName)
	}

	// Evaluate switch expression
	switchValue, err := k.evaluateExpression(parts[1], ctx)
	if err != nil {
		return fmt.Errorf("failed to evaluate switch expression: %w", err)
	}

	// Determine actual type based on switch value
	// TODO: Implement proper switch case mapping
	var actualType string
	if len(parts) > 2 {
		actualType = parts[2] // Use default type for now
	} else {
		return fmt.Errorf("no matching case for switch value: %v", switchValue)
	}

	// Serialize using actual type
	return k.serializeType(actualType, data, ctx)
}

// serializeBuiltinType handles serialization of built-in types
func (k *KaitaiSerializer) serializeBuiltinType(typeName string, data any, writer *kaitai.Writer) (bool, error) {
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
			return k.serializeBuiltinType(newType, data, writer)
		}
	}

	return false, nil
}

// serializeField serializes a single field
func (k *KaitaiSerializer) serializeField(field SequenceItem, data any, ctx *SerializeContext) error {
	// Check if field has a condition
	if field.IfExpr != "" {
		result, err := k.evaluateExpression(field.IfExpr, ctx)
		if err != nil {
			return fmt.Errorf("failed to evaluate if condition: %w", err)
		}

		// Skip field if condition is false
		if !isTrue(result) {
			return nil
		}
	}

	// Handle repeat attribute
	if field.Repeat != "" {
		return k.serializeRepeatedField(field, data, ctx)
	}

	// Handle contents attribute
	if field.Contents != nil {
		return k.serializeContentsField(field, ctx)
	}

	// Handle string and bytes fields
	if field.Type == "str" || field.Type == "strz" {
		return k.serializeStringField(field, data, ctx)
	}

	if field.Type == "bytes" {
		return k.serializeBytesField(field, data, ctx)
	}

	// Handle process
	if field.Process != "" {
		return k.serializeProcessedField(field, data, ctx)
	}

	// Default: serialize as a type
	return k.serializeType(field.Type, data, ctx)
}

// serializeRepeatedField handles serialization of repeated fields
func (k *KaitaiSerializer) serializeRepeatedField(field SequenceItem, data any, ctx *SerializeContext) error {
	items, ok := data.([]any)
	if !ok {
		return fmt.Errorf("expected array for repeated field, got %T", data)
	}

	// Determine repeat count
	var expectedCount int
	if field.RepeatExpr != "" {
		// Evaluate repeat expression
		result, err := k.evaluateExpression(field.RepeatExpr, ctx)
		if err != nil {
			return fmt.Errorf("failed to evaluate repeat expression: %w", err)
		}

		// Convert to int
		switch v := result.(type) {
		case int:
			expectedCount = v
		case int64:
			expectedCount = int(v)
		case float64:
			expectedCount = int(v)
		default:
			return fmt.Errorf("repeat expression result is not a number: %v", result)
		}

		// Validate count
		if len(items) != expectedCount {
			return fmt.Errorf("array length %d doesn't match expected repeat count %d", len(items), expectedCount)
		}
	}

	// Serialize each item
	for _, item := range items {
		itemField := field
		itemField.Repeat = ""
		itemField.RepeatExpr = ""

		if err := k.serializeField(itemField, item, ctx); err != nil {
			return fmt.Errorf("error serializing repeated item: %w", err)
		}
	}

	return nil
}

// serializeContentsField handles serialization of fields with fixed content
func (k *KaitaiSerializer) serializeContentsField(field SequenceItem, ctx *SerializeContext) error {
	var expected []byte

	switch v := field.Contents.(type) {
	case []any:
		// Array of byte values
		expected = make([]byte, len(v))
		for i, b := range v {
			if val, ok := b.(float64); ok {
				expected[i] = byte(val)
			} else {
				return fmt.Errorf("invalid content byte value: %v", b)
			}
		}
	case string:
		expected = []byte(v)
	default:
		return fmt.Errorf("unsupported contents type: %T", v)
	}

	// Write the fixed content
	if err := ctx.Writer.WriteBytes(expected); err != nil {
		return fmt.Errorf("failed to write content bytes: %w", err)
	}

	return nil
}

// serializeStringField handles serialization of string fields
func (k *KaitaiSerializer) serializeStringField(field SequenceItem, data any, ctx *SerializeContext) error {
	str, ok := data.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", data)
	}

	// Determine encoding
	encoding := field.Encoding
	if encoding == "" {
		encoding = k.schema.Meta.Encoding
	}
	if encoding == "" {
		encoding = "UTF-8" // Default encoding
	}

	// Convert string to bytes based on encoding
	var strBytes []byte
	// var err error

	if strings.ToUpper(encoding) == "ASCII" || strings.ToUpper(encoding) == "UTF-8" || strings.ToUpper(encoding) == "UTF8" {
		strBytes = []byte(str)
	} else {
		// Handle other encodings using CEL
		program, err := k.expressionPool.GetExpression("encodeString(input, encoding)")
		if err != nil {
			return fmt.Errorf("failed to compile encodeString expression: %w", err)
		}

		result, err := k.expressionPool.EvaluateExpression(program, map[string]any{
			"input":    str,
			"encoding": encoding,
		})
		if err != nil {
			return fmt.Errorf("failed to encode string: %w", err)
		}

		bytesResult, ok := result.([]byte)
		if !ok {
			return fmt.Errorf("encodeString didn't return bytes: %T", result)
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
			result, err := k.evaluateExpression(v, ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate size expression: %w", err)
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
				return fmt.Errorf("size expression result is not a number: %v", result)
			}
		default:
			return fmt.Errorf("unsupported size type: %T", v)
		}

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

	if err := ctx.Writer.WriteBytes(strBytes); err != nil {
		return fmt.Errorf("failed to write string bytes: %w", err)
	}

	return nil
}

// serializeBytesField handles serialization of bytes fields
func (k *KaitaiSerializer) serializeBytesField(field SequenceItem, data any, ctx *SerializeContext) error {
	var bytesData []byte

	switch v := data.(type) {
	case []byte:
		bytesData = v
	case string:
		bytesData = []byte(v)
	default:
		return fmt.Errorf("expected bytes or string, got %T", data)
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
			result, err := k.evaluateExpression(v, ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate size expression: %w", err)
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
				return fmt.Errorf("size expression result is not a number: %v", result)
			}
		default:
			return fmt.Errorf("unsupported size type: %T", v)
		}

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
	if err := ctx.Writer.WriteBytes(bytesData); err != nil {
		return fmt.Errorf("failed to write bytes: %w", err)
	}

	return nil
}

// serializeProcessedField handles serialization of processed fields
func (k *KaitaiSerializer) serializeProcessedField(field SequenceItem, data any, ctx *SerializeContext) error {
	// First, serialize to a buffer
	fieldCtx := &SerializeContext{
		Value:    data,
		Children: ctx.Children,
		Parent:   ctx.Parent,
		Root:     ctx.Root,
		Writer:   kaitai.NewWriter(bytes.NewBuffer(nil)), // Temporary writer
	}

	// Serialize the field without processing
	fieldCopy := field
	fieldCopy.Process = ""
	if err := k.serializeField(fieldCopy, data, fieldCtx); err != nil {
		return fmt.Errorf("failed to serialize field before processing: %w", err)
	}

	// Get the serialized bytes

	serialized, err := getWriterBuffer(fieldCtx.Writer)
	if err != nil {
		return fmt.Errorf("failed to get writer buffer: %w", err)
	}

	// Reverse the process
	processed, err := k.reverseProcess(serialized, field.Process)
	if err != nil {
		return fmt.Errorf("failed to reverse process field data: %w", err)
	}

	// Write the processed data
	if err := ctx.Writer.WriteBytes(processed); err != nil {
		return fmt.Errorf("failed to write processed field: %w", err)
	}

	return nil
}

// reverseProcess applies the reverse of a process function
func (k *KaitaiSerializer) reverseProcess(data []byte, processSpec string) ([]byte, error) {
	// Parse the process spec (e.g., "xor(0x5F)")
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
		return nil, fmt.Errorf("zlib compression not implemented for serialization")
	case "rotate":
		// For rotate, we need to negate the amount
		expr = fmt.Sprintf("processRotateRight(input, %s)", paramStr)
	default:
		return nil, fmt.Errorf("unknown process function: %s", processFn)
	}

	// Compile and evaluate CEL expression
	program, err := k.expressionPool.GetExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile process expression: %w", err)
	}

	result, err := k.expressionPool.EvaluateExpression(program, map[string]any{
		"input": data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate process expression: %w", err)
	}

	// Convert result to bytes
	if bytesResult, ok := result.([]byte); ok {
		return bytesResult, nil
	}
	return nil, fmt.Errorf("process result is not bytes: %T", result)
}

// evaluateExpression evaluates a Kaitai expression using CEL
func (k *KaitaiSerializer) evaluateExpression(kaitaiExpr string, ctx *SerializeContext) (any, error) {
	// Get or compile expression
	program, err := k.expressionPool.GetExpression(kaitaiExpr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", err)
	}

	// Create activation from context
	activation, err := ctx.AsActivation()
	if err != nil {
		return nil, fmt.Errorf("failed to create activation: %w", err)
	}

	// Evaluate expression
	result, _, err := program.Eval(activation)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return result.Value(), nil
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
