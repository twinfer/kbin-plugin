package kaitaistruct

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	typeStack       []string         // Stack of type names being processed
	valueStack      []*ParseContext  // Stack of parent values for expression evaluation
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
func NewKaitaiInterpreter(schema *KaitaiSchema) (*KaitaiInterpreter, error) {
	// Create expression pool with our enhanced CEL environment
	pool, err := internalCel.NewExpressionPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create expression pool: %w", err)
	}

	return &KaitaiInterpreter{
		schema:          schema,
		expressionPool:  pool,
		typeStack:       make([]string, 0),
		valueStack:      make([]*ParseContext, 0),
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
func (k *KaitaiInterpreter) Parse(stream *kaitai.Stream) (*ParsedData, error) {
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
	result, err := k.parseType(rootType, stream)
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
		for name, inst := range k.schema.Instances {
			val, err := k.evaluateInstance(inst, rootCtx)
			if err != nil {
				return nil, fmt.Errorf("failed evaluating instance '%s': %w", name, err)
			}
			result.Children[name] = val
		}
	}

	return result, nil
}

// parseType parses a Kaitai type from the stream
func (k *KaitaiInterpreter) parseType(typeName string, stream *kaitai.Stream) (*ParsedData, error) {
	// Check for circular dependency
	// This is a simple check to prevent infinite loops in case of circular references
	if slices.Contains(k.typeStack, typeName) {
		return nil, fmt.Errorf("circular type dependency detected: %s", typeName)
	}

	// Push current type to stack
	k.typeStack = append(k.typeStack, typeName)
	defer func() {
		// Pop current type from stack when done
		k.typeStack = k.typeStack[:len(k.typeStack)-1]
	}()

	// Create result structure
	result := &ParsedData{
		Children: make(map[string]*ParsedData),
		Type:     typeName,
	}

	// Check if it's a built-in type
	if parsedData, handled, err := k.parseBuiltinType(typeName, stream); handled {
		if err != nil {
			return nil, err
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
		ctx := &ParseContext{
			Children: make(map[string]any),
			IO:       stream,
			Parent:   k.valueStack[len(k.valueStack)-1],
			Root:     k.valueStack[0].Root,
		}

		// Evaluate switch expression using CEL
		switchValue, err := k.evaluateExpression(parts[1], ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate switch expression: %w", err)
		}

		// Determine actual type based on switch value
		var actualType string
		// TODO: Implement switch cases mapping
		// For now, use default type if specified
		if len(parts) > 2 {
			actualType = parts[2]
		} else {
			return nil, fmt.Errorf("no matching case for switch value: %v", switchValue)
		}

		// Parse using actual type
		return k.parseType(actualType, stream)
	}

	// Look for the type in schema
	var typeObj Type
	var found bool

	if typeName == k.schema.Meta.ID {
		// Parse root level sequence
		ctx := &ParseContext{
			Children: make(map[string]any),
			IO:       stream,
			Parent:   k.valueStack[len(k.valueStack)-1],
			Root:     k.valueStack[0].Root,
		}

		// Parse sequence fields
		for _, seq := range k.schema.Seq {
			field, err := k.parseField(seq, ctx)
			if err != nil {
				return nil, fmt.Errorf("error parsing field '%s': %w", seq.ID, err)
			}
			if field != nil { // Only add if not nil
				result.Children[seq.ID] = field    // Store the ParsedData
				ctx.Children[seq.ID] = field.Value // Store the primitive for expressions
			}
		}

		return result, nil
	} else if typeObj, found = k.schema.Types[typeName]; found {
		// Create context for this type
		ctx := &ParseContext{
			Children: make(map[string]any),
			IO:       stream,
			Parent:   k.valueStack[len(k.valueStack)-1],
			Root:     k.valueStack[0].Root,
		}

		// Push context
		k.valueStack = append(k.valueStack, ctx)
		defer func() {
			// Pop context when done
			k.valueStack = k.valueStack[:len(k.valueStack)-1]
		}()

		// Parse sequence fields
		for _, seq := range typeObj.Seq {
			// Check if this is a switch type
			if seq.Type == "switch" {
				// Handle switch case
				switchSelector, err := NewSwitchTypeSelector(seq.Switch, k.schema)
				if err != nil {
					return nil, fmt.Errorf("error creating switch selector for field '%s': %w", seq.ID, err)
				}

				// Resolve actual type using CEL for switch expressions
				actualType, err := switchSelector.ResolveType(ctx, k)
				if err != nil {
					return nil, fmt.Errorf("error resolving switch type for field '%s': %w", seq.ID, err)
				}

				// Override type for this field
				seqCopy := seq
				seqCopy.Type = actualType

				// Parse field with resolved type
				field, err := k.parseField(seqCopy, ctx)
				if err != nil {
					return nil, fmt.Errorf("error parsing switch field '%s' with type '%s': %w",
						seq.ID, actualType, err)
				}

				result.Children[seq.ID] = field    // Store the ParsedData
				ctx.Children[seq.ID] = field.Value // Store the primitive for expressions
				continue
			}

			// Parse regular field
			field, err := k.parseField(seq, ctx)
			if err != nil {
				return nil, fmt.Errorf("error parsing field '%s' in type '%s': %w",
					seq.ID, typeName, err)
			}
			if field != nil { // Only add if not nil
				result.Children[seq.ID] = field    // Store the ParsedData
				ctx.Children[seq.ID] = field.Value // Store the primitive for expressions
			}
		}

		// Process instances if any
		if typeObj.Instances != nil {
			for name, inst := range typeObj.Instances {
				val, err := k.evaluateInstance(inst, ctx)
				if err != nil {
					return nil, fmt.Errorf("failed evaluating instance '%s' in type '%s': %w",
						name, typeName, err)
				}
				result.Children[name] = val
			}
		}

		return result, nil
	}

	return nil, fmt.Errorf("unknown type: %s", typeName)
}

// parseBuiltinType handles built-in Kaitai types
func (k *KaitaiInterpreter) parseBuiltinType(typeName string, stream *kaitai.Stream) (*ParsedData, bool, error) {
	result := &ParsedData{
		Type:     typeName,
		Children: make(map[string]*ParsedData),
	}

	// Process built-in types
	switch typeName {
	case "u1":
		val, err := stream.ReadU1()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint8(val)
		return result, true, nil
	case "u2le":
		val, err := stream.ReadU2le()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint16(val)
		return result, true, nil
	case "u4le":
		val, err := stream.ReadU4le()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint32(val)
		return result, true, nil
	case "u8le":
		val, err := stream.ReadU8le()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint64(val)
		return result, true, nil
	case "u2be":
		val, err := stream.ReadU2be()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint16(val)
		return result, true, nil
	case "u4be":
		val, err := stream.ReadU4be()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint32(val)
		return result, true, nil
	case "u8be":
		val, err := stream.ReadU8be()
		if err != nil {
			return nil, true, err
		}
		result.Value = uint64(val)
		return result, true, nil
	case "s1":
		val, err := stream.ReadS1()
		if err != nil {
			return nil, true, err
		}
		result.Value = int8(val)
		return result, true, nil
	case "s2le":
		val, err := stream.ReadS2le()
		if err != nil {
			return nil, true, err
		}
		result.Value = int16(val)
		return result, true, nil
	case "s4le":
		val, err := stream.ReadS4le()
		if err != nil {
			return nil, true, err
		}
		result.Value = int32(val)
		return result, true, nil
	case "s8le":
		val, err := stream.ReadS8le()
		if err != nil {
			return nil, true, err
		}
		result.Value = int64(val)
		return result, true, nil
	case "s2be":
		val, err := stream.ReadS2be()
		if err != nil {
			return nil, true, err
		}
		result.Value = int16(val)
		return result, true, nil
	case "s4be":
		val, err := stream.ReadS4be()
		if err != nil {
			return nil, true, err
		}
		result.Value = int32(val)
		return result, true, nil
	case "s8be":
		val, err := stream.ReadS8be()
		if err != nil {
			return nil, true, err
		}
		result.Value = int64(val)
		return result, true, nil
	case "f4le":
		val, err := stream.ReadF4le()
		if err != nil {
			return nil, true, err
		}
		result.Value = float32(val)
		return result, true, nil
	case "f8le":
		val, err := stream.ReadF8le()
		if err != nil {
			return nil, true, err
		}
		result.Value = float64(val)
		return result, true, nil
	case "f4be":
		val, err := stream.ReadF4be()
		if err != nil {
			return nil, true, err
		}
		result.Value = float32(val)
		return result, true, nil
	case "f8be":
		val, err := stream.ReadF8be()
		if err != nil {
			return nil, true, err
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
			return k.parseBuiltinType(newType, stream)
		}
	}

	// Not a built-in type
	return nil, false, nil
}

// parseField parses a field from the sequence
func (k *KaitaiInterpreter) parseField(field SequenceItem, ctx *ParseContext) (*ParsedData, error) {
	// Check if field has a condition using CEL
	if field.IfExpr != "" {
		result, err := k.evaluateExpression(field.IfExpr, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate if condition: %w", err)
		}

		// Skip field if condition is false
		if !isTrue(result) {
			return nil, nil
		}
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
			result, err := k.evaluateExpression(v, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate size expression: %w", err)
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
				return nil, fmt.Errorf("size expression result is not a number: %v (type %T)", result, result)
			}
		default:
			return nil, fmt.Errorf("unsupported size type: %T", v)
		}
	}

	// Handle repeat attribute
	if field.Repeat != "" {
		return k.parseRepeatedField(field, ctx, size)
	}

	// Handle contents attribute
	if field.Contents != nil {
		return k.parseContentsField(field, ctx)
	}

	// Parse string type
	if field.Type == "str" || field.Type == "strz" {
		return k.parseStringField(field, ctx, size)
	}

	// Parse bytes type
	if field.Type == "bytes" {
		return k.parseBytesField(field, ctx, size)
	}

	// Read data based on size
	var fieldData []byte
	var subStream *kaitai.Stream
	var err error

	if size > 0 {
		// Read sized data
		fieldData, err = ctx.IO.ReadBytes(size)
		if err != nil {
			return nil, fmt.Errorf("failed to read %d bytes for field: %w", size, err)
		}

		// Create substream
		subStream = kaitai.NewStream(bytes.NewReader(fieldData))
	} else {
		// Use current stream directly
		subStream = ctx.IO
	}

	// Apply process if specified
	if field.Process != "" && size > 0 {
		// Process the field data using CEL for process functions
		processedData, err := k.processDataWithCEL(fieldData, field.Process)
		if err != nil {
			return nil, fmt.Errorf("failed to process field data: %w", err)
		}

		// Create new substream with processed data
		subStream = kaitai.NewStream(bytes.NewReader(processedData))
	}

	// Parse using the appropriate stream
	return k.parseType(field.Type, subStream)
}

// processDataWithCEL processes data using CEL expressions
func (k *KaitaiInterpreter) processDataWithCEL(data []byte, processSpec string) ([]byte, error) {
	// Parse the process spec (e.g., "xor(0x5F)")
	parts := strings.Split(processSpec, "(")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid process specification: %s", processSpec)
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
		return nil, fmt.Errorf("unknown process function: %s", processFn)
	}

	// Get the CEL program
	program, err := k.expressionPool.GetExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile process expression: %w", err)
	}

	// Evaluate with input data as parameter
	result, err := k.expressionPool.EvaluateExpression(program, map[string]any{
		"input": data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate process expression: %w", err)
	}

	// Convert result back to byte array
	if bytesResult, ok := result.([]byte); ok {
		return bytesResult, nil
	}
	return nil, fmt.Errorf("process result is not a byte array: %T", result)
}

// parseRepeatedField handles repeating fields
func (k *KaitaiInterpreter) parseRepeatedField(field SequenceItem, ctx *ParseContext, size int) (*ParsedData, error) {
	result := &ParsedData{
		Type:    field.Type,
		IsArray: true,
		Value:   make([]any, 0),
	}

	// Determine repeat count
	var count int

	if field.RepeatExpr != "" {
		// Evaluate repeat expression using CEL
		expr, err := k.evaluateExpression(field.RepeatExpr, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate repeat expression: %w", err)
		}

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
			return nil, fmt.Errorf("repeat expression result is not a number: %v (type %T)", expr, expr)
		}
	} else if field.Repeat == "eos" {
		// Repeat until end of stream
		count = -1
	} else {
		return nil, fmt.Errorf("unsupported repeat type: %s", field.Repeat)
	}

	// Read items
	items := make([]*ParsedData, 0)

	if count > 0 {
		for range count {
			itemField := field
			itemField.Repeat = ""
			itemField.RepeatExpr = ""
			item, err := k.parseField(itemField, ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("error parsing repeated item: %w", err)
			}
			items = append(items, item)
		}
	} else if count == -1 {
		for {
			itemField := field
			itemField.Repeat = ""
			itemField.RepeatExpr = ""
			item, err := k.parseField(itemField, ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("error parsing repeated item: %w", err)
			}
			items = append(items, item)
		}
	}

	// Add items to result
	itemValues := make([]any, len(items))
	for i, item := range items {
		itemValues[i] = item
	}
	result.Value = itemValues

	return result, nil
}

// parseContentsField handles fields with fixed contents
func (k *KaitaiInterpreter) parseContentsField(field SequenceItem, ctx *ParseContext) (*ParsedData, error) {
	result := &ParsedData{
		Type: field.Type,
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
				return nil, fmt.Errorf("invalid content byte value: %v", b)
			}
		}
	case string:
		expected = []byte(v)
	default:
		return nil, fmt.Errorf("unsupported contents type: %T", v)
	}

	// Read actual bytes
	actual, err := ctx.IO.ReadBytes(len(expected))
	if err != nil {
		return nil, fmt.Errorf("failed to read content bytes: %w", err)
	}

	// Validate contents
	if !bytes.Equal(actual, expected) {
		return nil, fmt.Errorf("content validation failed, expected %v, got %v", expected, actual)
	}

	result.Value = actual
	return result, nil
}

// parseStringField handles string fields
func (k *KaitaiInterpreter) parseStringField(field SequenceItem, ctx *ParseContext, size int) (*ParsedData, error) {
	result := &ParsedData{
		Type: field.Type,
	}

	// Determine encoding
	encoding := field.Encoding
	if encoding == "" {
		encoding = k.schema.Meta.Encoding
	}
	if encoding == "" {
		encoding = "UTF-8" // Default encoding
	}

	var strBytes []byte
	var err error

	if field.Type == "strz" {
		// Zero-terminated string
		strBytes, err = ctx.IO.ReadBytesTerm(0, false, true, true)
	} else if size > 0 {
		// Fixed-size string
		strBytes, err = ctx.IO.ReadBytes(size)
	} else if field.SizeEOS {
		// Read until end of stream
		// Use ReadStrEOS directly if the encoding is UTF-8
		if strings.ToUpper(encoding) == "UTF-8" || strings.ToUpper(encoding) == "UTF8" {
			str, err := ctx.IO.ReadStrEOS(encoding)
			if err != nil {
				return nil, fmt.Errorf("failed to read string: %w", err)
			}
			result.Value = str
			return result, nil
		}

		// Otherwise, use position and seek
		isEof, err := ctx.IO.EOF()
		if err != nil {
			return nil, fmt.Errorf("failed to check EOF: %w", err)
		}
		if isEof {
			result.Value = ""
			return result, nil
		}

		pos, err := ctx.IO.Pos()
		if err != nil {
			return nil, fmt.Errorf("failed to get current position: %w", err)
		}
		stream := ctx.IO
		endPos, err := stream.Size()
		if err != nil {
			return nil, fmt.Errorf("failed to get stream size: %w", err)
		}
		size := endPos - pos
		strBytes, err = ctx.IO.ReadBytes(int(size))
		if err != nil {
			return nil, fmt.Errorf("failed to read string bytes: %w", err)
		}
	} else {
		return nil, fmt.Errorf("cannot determine string size")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read string bytes: %w", err)
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

		// Use Kaitai's BytesToStr where possible
		if strings.ToUpper(encoding) == "ASCII" || strings.ToUpper(encoding) == "UTF-8" || strings.ToUpper(encoding) == "UTF8" {
			str, err = kaitai.BytesToStr(strBytes, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to decode string: %w", err)
			}
		} else {
			//  proper transcoding for other encodings
			switch strings.ToUpper(encoding) {
			case "UTF-16LE", "UTF16LE":
				decoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
				utf8Str, _, err := transform.String(decoder, string(strBytes))
				if err != nil {
					return nil, fmt.Errorf("failed to decode UTF-16LE: %w", err)
				}
				str = utf8Str
			case "UTF-16BE", "UTF16BE":
				decoder := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()
				utf8Str, _, err := transform.String(decoder, string(strBytes))
				if err != nil {
					return nil, fmt.Errorf("failed to decode UTF-16BE: %w", err)
				}
				str = utf8Str
			default:
				return nil, fmt.Errorf("unsupported encoding: %s", encoding)
			}
		}
		result.Value = str
	} else {
		result.Value = val
	}

	return result, nil
}

// parseBytesField handles bytes fields
func (k *KaitaiInterpreter) parseBytesField(field SequenceItem, ctx *ParseContext, size int) (*ParsedData, error) {
	result := &ParsedData{
		Type: field.Type,
	}

	var bytesData []byte
	var err error

	if size > 0 {
		// Fixed-size bytes
		bytesData, err = ctx.IO.ReadBytes(size)
	} else if field.SizeEOS {
		// Read until end of stream
		bytesData, err = ctx.IO.ReadBytesFull()
	} else {
		return nil, fmt.Errorf("cannot determine bytes size")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read bytes: %w", err)
	}

	result.Value = bytesData
	return result, nil
}

// evaluateInstance calculates an instance field
func (k *KaitaiInterpreter) evaluateInstance(inst InstanceDef, ctx *ParseContext) (*ParsedData, error) {
	// Evaluate the instance expression using CEL
	value, err := k.evaluateExpression(inst.Value, ctx)
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
func (k *KaitaiInterpreter) evaluateExpression(kaitaiExpr string, ctx *ParseContext) (any, error) {
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
