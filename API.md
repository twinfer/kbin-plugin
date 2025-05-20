# API Documentation

This document provides detailed information about the API for the Kaitai Struct Benthos plugin.

## Table of Contents

- [Core Types](#core-types)
- [Processor API](#processor-api)
- [Interpreter API](#interpreter-api)
- [Serializer API](#serializer-api)
- [Expression API](#expression-api)
- [Schema API](#schema-api)

## Core Types

### KaitaiProcessor

The main Benthos processor implementation.

```go
type KaitaiProcessor struct {
    config    KaitaiConfig
    schemaMap sync.Map // Cache for parsed schemas
}

type KaitaiConfig struct {
    SchemaPath string `json:"schema_path" yaml:"schema_path"`
    IsParser   bool   `json:"is_parser" yaml:"is_parser"`
    RootType   string `json:"root_type" yaml:"root_type"`
}
```

**Methods:**

- `Process(ctx context.Context, msg *service.Message) (service.MessageBatch, error)`: Process a Benthos message
- `Close(ctx context.Context) error`: Clean up resources

### ParsedData

Represents the result of parsing binary data.

```go
type ParsedData struct {
    Value    interface{}
    Children map[string]*ParsedData
    Type     string
    IsArray  bool
}
```

### KaitaiInterpreter

Dynamic interpreter for Kaitai Struct schemas.

```go
type KaitaiInterpreter struct {
    schema         *KaitaiSchema
    expressionPool *ExpressionPool
    typeStack      []string
    valueStack     []*ParseContext
}

type ParseContext struct {
    Value    interface{}
    Parent   *ParseContext
    Root     *ParseContext
    IO       *kaitai.Stream
    Children map[string]interface{}
}
```

### KaitaiSerializer

Serializes structured data to binary format.

```go
type KaitaiSerializer struct {
    schema         *KaitaiSchema
    expressionPool *ExpressionPool
    valueStack     []*SerializeContext
}

type SerializeContext struct {
    Value    interface{}
    Parent   *SerializeContext
    Root     *SerializeContext
    Children map[string]interface{}
}
```

### KaitaiSchema

Represents a parsed KSY schema file.

```go
type KaitaiSchema struct {
    Meta      Meta                    `yaml:"meta"`
    Seq       []SequenceItem          `yaml:"seq"`
    Types     map[string]Type         `yaml:"types"`
    Instances map[string]InstanceDef  `yaml:"instances"`
    Enums     map[string]EnumDef      `yaml:"enums"`
    Doc       string                  `yaml:"doc"`
    DocRef    string                  `yaml:"doc-ref"`
    Params    []ParameterDef          `yaml:"params"`
}
```

## Processor API

### Registration

```go
func init() {
    // Register the processor with Benthos
    err := service.RegisterProcessor(
        "kaitai",
        kaitaiProcessorConfig(),
        func(conf *service.ParsedConfig, mgr *service.Resources) (service.Processor, error) {
            return newKaitaiProcessorFromConfig(conf)
        },
    )
    if err != nil {
        panic(err)
    }
}
```

### Configuration Spec

```go
func kaitaiProcessorConfig() *service.ConfigSpec {
    return service.NewConfigSpec().
        Summary("Parses or serializes binary data using Kaitai Struct definitions without code generation.").
        Field(service.NewStringField("schema_path").
            Description("Path to the Kaitai Struct (.ksy) schema file.").
            Example("./schemas/my_format.ksy")).
        Field(service.NewBoolField("is_parser").
            Description("Whether this processor parses binary to JSON (true) or serializes JSON to binary (false).").
            Default(true)).
        Field(service.NewStringField("root_type").
            Description("The root type name from the KSY file.").
            Default("")).
        Version("0.1.0")
}
```

### Processing Messages

```go
func (k *KaitaiProcessor) Process(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
    if k.config.IsParser {
        return k.parseBinary(ctx, msg)
    }
    return k.serializeToBinary(ctx, msg)
}
```

## Interpreter API

### Creating an Interpreter

```go
func NewKaitaiInterpreter(schema *KaitaiSchema) *KaitaiInterpreter {
    return &KaitaiInterpreter{
        schema:         schema,
        expressionPool: NewExpressionPool(),
        typeStack:      make([]string, 0),
        valueStack:     make([]*ParseContext, 0),
    }
}
```

### Parsing Binary Data

```go
func (k *KaitaiInterpreter) Parse(stream *kaitai.Stream) (*ParsedData, error) {
    // Create root context
    rootCtx := &ParseContext{
        Children: make(map[string]interface{}),
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
    
    // Process instances if any
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
```

### Parsing Types and Fields

```go
func (k *KaitaiInterpreter) parseType(typeName string, stream *kaitai.Stream) (*ParsedData, error) {
    // Implementation details...
}

func (k *KaitaiInterpreter) parseField(field SequenceItem, ctx *ParseContext) (*ParsedData, error) {
    // Implementation details...
}

func (k *KaitaiInterpreter) parseRepeatedField(field SequenceItem, ctx *ParseContext, size int) (*ParsedData, error) {
    // Implementation details...
}
```

## Serializer API

### Creating a Serializer

```go
func NewKaitaiSerializer(schema *KaitaiSchema) *KaitaiSerializer {
    return &KaitaiSerializer{
        schema:         schema,
        expressionPool: NewExpressionPool(),
        valueStack:     make([]*SerializeContext, 0),
    }
}
```

### Serializing Structured Data

```go
func (k *KaitaiSerializer) Serialize(data interface{}) ([]byte, error) {
    buf := new(bytes.Buffer)
    
    // Create root context
    rootCtx := &SerializeContext{
        Value:    data,
        Children: make(map[string]interface{}),
    }
    rootCtx.Root = rootCtx
    
    // Push root context
    k.valueStack = append(k.valueStack, rootCtx)
    defer func() {
        // Pop root context when done
        k.valueStack = k.valueStack[:len(k.valueStack)-1]
    }()
    
    // Get data map
    dataMap, ok := data.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("expected map[string]interface{}, got %T", data)
    }
    
    // Fill context
    for key, val := range dataMap {
        rootCtx.Children[key] = val
    }
    
    // Determine endianness
    var endian binary.ByteOrder
    if k.schema.Meta.Endian == "le" {
        endian = binary.LittleEndian
    } else {
        endian = binary.BigEndian
    }
    
    // Serialize according to root type
    rootType := k.schema.Meta.ID
    if k.schema.RootType != "" {
        rootType = k.schema.RootType
    }
    
    // Serialize root sequence
    if err := k.serializeType(rootType, dataMap, buf, endian); err != nil {
        return nil, fmt.Errorf("failed serializing root type '%s': %w", rootType, err)
    }
    
    return buf.Bytes(), nil
}
```

### Serializing Types and Fields

```go
func (k *KaitaiSerializer) serializeType(typeName string, data interface{}, buf *bytes.Buffer, endian binary.ByteOrder) error {
    // Implementation details...
}

func (k *KaitaiSerializer) serializeField(field SequenceItem, data map[string]interface{}, buf *bytes.Buffer, endian binary.ByteOrder) error {
    // Implementation details...
}

func (k *KaitaiSerializer) serializeRepeatedField(field SequenceItem, value interface{}, buf *bytes.Buffer, endian binary.ByteOrder) error {
    // Implementation details...
}
```

## Expression API

### Expression Pool

```go
type ExpressionPool struct {
    expressions map[string]*govaluate.EvaluableExpression
}

func NewExpressionPool() *ExpressionPool {
    return &ExpressionPool{
        expressions: make(map[string]*govaluate.EvaluableExpression),
    }
}
```

### Evaluating Expressions

```go
func (k *KaitaiInterpreter) evaluateExpression(expr string, ctx *ParseContext) (interface{}, error) {
    // Get or compile expression
    evalExpr, err := k.expressionPool.GetExpression(expr)
    if err != nil {
        return nil, fmt.Errorf("failed to compile expression: %w", err)
    }
    
    // Create parameter map
    params := make(map[string]interface{})
    
    // Add current context values
    if ctx.Children != nil {
        for name, val := range ctx.Children {
            params[name] = val
        }
    }
    
    // Add special variables
    params["_root"] = ctx.Root.Children
    if ctx.Parent != nil {
        params["_parent"] = ctx.Parent.Children
    }
    params["_io"] = ctx.IO
    
    // Evaluate expression
    result, err := evalExpr.Evaluate(params)
    if err != nil {
        return nil, fmt.Errorf("failed to evaluate expression: %w", err)
    }
    
    return result, nil
}
```

### Expression Transformation

```go
func transformKaitaiExpression(expr string) string {
    // Replace ternary operator
    ternaryPattern := regexp.MustCompile(`(.*?)\s*\?\s*(.*?)\s*:\s*(.*)`)
    for ternaryPattern.MatchString(expr) {
        expr = ternaryPattern.ReplaceAllString(expr, "ternary($1, $2, $3)")
    }
    
    // Replace bitshift operators
    expr = regexp.MustCompile(`(.*?)\s*<<\s*(\d+)`).ReplaceAllString(expr, "bitShiftLeft($1, $2)")
    expr = regexp.MustCompile(`(.*?)\s*>>\s*(\d+)`).ReplaceAllString(expr, "bitShiftRight($1, $2)")
    
    // Replace bitwise operators
    expr = regexp.MustCompile(`(.*?)\s*\&\s*(.*)`).ReplaceAllString(expr, "bitAnd($1, $2)")
    expr = regexp.MustCompile(`(.*?)\s*\|\s*(.*)`).ReplaceAllString(expr, "bitOr($1, $2)")
    expr = regexp.MustCompile(`(.*?)\s*\^\s*(.*)`).ReplaceAllString(expr, "bitXor($1, $2)")
    
    // Replace enum to_i function
    expr = regexp.MustCompile(`([\w\.]+)\.to_i\(\)`).ReplaceAllString(expr, "enumToInt($1)")
    
    // Replace to_s function
    expr = regexp.MustCompile(`([\w\.]+)\.to_s\(\)`).ReplaceAllString(expr, "toString($1)")
    
    // Replace length function
    expr = regexp.MustCompile(`([\w\.]+)\.length`).ReplaceAllString(expr, "length($1)")
    
    // Replace reverse function
    expr = regexp.MustCompile(`([\w\.]+)\.reverse`).ReplaceAllString(expr, "reverse($1)")
    
    return expr
}
```

## Schema API

### Schema Types

```go
type KaitaiSchema struct {
    Meta      Meta                    `yaml:"meta"`
    Seq       []SequenceItem          `yaml:"seq"`
    Types     map[string]Type         `yaml:"types"`
    Instances map[string]InstanceDef  `yaml:"instances"`
    Enums     map[string]EnumDef      `yaml:"enums"`
    Doc       string                  `yaml:"doc"`
    DocRef    string                  `yaml:"doc-ref"`
    Params    []ParameterDef          `yaml:"params"`
}

type Meta struct {
    ID          string   `yaml:"id"`
    Title       string   `yaml:"title"`
    Application string   `yaml:"application"`
    FileExt     []string `yaml:"file-ext"`
    License     string   `yaml:"license"`
    Ks          string   `yaml:"ks-version"`
    Endian      string   `yaml:"endian"`
    Encoding    string   `yaml:"encoding"`
    Imports     []string `yaml:"imports"`
}

type SequenceItem struct {
    ID           string      `yaml:"id"`
    Type         string      `yaml:"type"`
    Repeat       string      `yaml:"repeat,omitempty"`
    RepeatExpr   string      `yaml:"repeat-expr,omitempty"`
    RepeatUntil  string      `yaml:"repeat-until,omitempty"`
    Size         interface{} `yaml:"size,omitempty"`
    SizeEOS      bool        `yaml:"size-eos,omitempty"`
    IfExpr       string      `yaml:"if,omitempty"`
    Process      string      `yaml:"process,omitempty"`
    Contents     interface{} `yaml:"contents,omitempty"`
    Terminator   interface{} `yaml:"terminator,omitempty"`
    Include      interface{} `yaml:"include,omitempty"`
    Consume      interface{} `yaml:"consume,omitempty"`
    Encoding     string      `yaml:"encoding,omitempty"`
    PadRight     interface{} `yaml:"pad-right,omitempty"`
    Doc          string      `yaml:"doc,omitempty"`
    DocRef       string      `yaml:"doc-ref,omitempty"`
}
```

### Loading Schema

```go
func (k *KaitaiProcessor) loadSchema(path string) (*KaitaiSchema, error) {
    // Check schema cache first
    if cachedSchema, ok := k.schemaMap.Load(path); ok {
        return cachedSchema.(*KaitaiSchema), nil
    }

    // Read schema file
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read schema file: %w", err)
    }

    // Parse YAML
    schema := &KaitaiSchema{}
    if err := yaml.Unmarshal(data, schema); err != nil {
        return nil, fmt.Errorf("failed to parse schema YAML: %w", err)
    }

    // Store in cache
    k.schemaMap.Store(path, schema)

    return schema, nil
}
```
