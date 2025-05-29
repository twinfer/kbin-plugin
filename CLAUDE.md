# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build
```bash
# Build the plugin binary
go build -o kbin-benthos-plugin ./cmd/kbin-plugin

# Ensure dependencies are correct
go mod tidy
```

### Test
```bash
# Run all tests
go test ./...

# Run tests for specific packages with verbose output
go test ./pkg/kaitaistruct -v
go test ./internal/cel -v
go test ./pkg/kaitaicel -v

# Run tests with coverage
go test -cover ./...
```

### Static Analysis
```bash
# Run go vet (will fail on scripts/kaitai-testgen-simple.go - this is a known issue)
go vet ./...

# Format code
go fmt ./...
```

## High-Level Architecture

This is a Benthos plugin that dynamically interprets Kaitai Struct (.ksy) schemas to parse and serialize binary data without requiring pre-compilation.

### Core Components

1. **KaitaiProcessor** (`cmd/kbin-plugin/main.go`)
   - Benthos processor interface implementation
   - Manages schema caching (main and framing schemas)
   - Handles message batch processing and metrics
   - Entry point for parse/serialize operations

2. **KaitaiInterpreter** (`pkg/kaitaistruct/parser.go`)
   - Core parsing/serialization engine
   - Recursively traverses schema structure
   - Manages ParseContext for state tracking
   - Integrates with CEL for expression evaluation

3. **KaitaiSchema** (`pkg/kaitaistruct/schema.go`)
   - Go representation of parsed .ksy YAML files
   - Defines types, sequences, instances, and metadata

4. **CEL Integration** (`internal/cel/` and `pkg/kaitaicel/`)
   - Custom CEL environment with Kaitai-specific functions
   - KaitaiCEL types implement Kaitai data types (KaitaiInt, KaitaiString, etc.)
   - Expression pool caches compiled CEL programs for performance

### Key Design Patterns

- **Dynamic Interpretation**: No code generation; schemas are interpreted at runtime
- **CEL for Expressions**: All conditional logic, calculations, and dynamic fields use CEL
- **Type Safety**: Custom KaitaiCEL types preserve both raw bytes and interpreted values
- **Caching**: Schemas and compiled expressions are cached for performance

### Package Structure

- `cmd/kbin-plugin/`: Main entry point and Benthos processor
- `pkg/kaitaistruct/`: Core parsing/serialization logic
- `pkg/kaitaicel/`: Kaitai-specific CEL types (integers, strings, enums, bitfields)
- `internal/cel/`: CEL environment setup and custom functions
- `pkg/expression/`: Expression parser for legacy support
- `testutil/`: Test utilities
- `test/`: Kaitai test format files (.ksy) and test data

## Important Notes

- The project uses Go 1.24+ with modern features like generics
- There's a known build issue with `scripts/kaitai-testgen-simple.go` (fmt.Sprintf argument mismatch)
- Tests extensively use Kaitai Struct's official test formats for validation
- The plugin supports both parsing (binary to JSON) and serialization (JSON to binary)

## Kaitai Struct Specification Reference

### Type System
- **Primitive Types**: `u1`, `u2`, `u4`, `u8` (unsigned), `s1`, `s2`, `s4`, `s8` (signed), `f4`, `f8` (floats)
- **String Types**: `str` (with encoding), `strz` (zero-terminated)
- **Bytes Type**: `bytes` (raw byte arrays)
- **Bit Types**: `b1`, `b2`, ..., `b64` (bit-level fields)
- **User-Defined Types**: Custom types defined in `types:` section
- **Type Defaulting**: Fields with `size` but no `type` default to `bytes` (not string)

### Field Attributes
- `type`: Data type specification (required for non-bytes fields)
- `size`: Fixed size in bytes, or expression for variable size
- `size-eos`: Read until end of stream
- `repeat`: Array repetition (`expr`, `eos`, `until`)
- `repeat-expr`: Expression for repeat count
- `repeat-until`: Condition for stopping repetition
- `terminator`: Byte value that terminates parsing
- `include`: Whether to include terminator in result
- `if`: Conditional parsing expression
- `process`: Data processing (e.g., `zlib`, `xor(key)`)
- `encoding`: String encoding (e.g., `UTF-8`, `ASCII`)
- `contents`: Fixed content validation
- `value`: Computed field (instance)

### Expressions & Built-in Functions
- **Arithmetic**: `+`, `-`, `*`, `/`, `%` (modulo)
- **Comparison**: `==`, `!=`, `<`, `>`, `<=`, `>=`
- **Logical**: `and`, `or`, `not`
- **Bitwise**: `&`, `|`, `^`, `<<`, `>>`
- **Ternary**: `condition ? true_val : false_val`
- **String Methods**: `.length`, `.reverse`, `.substring(start, end)`, `.to_i(base)`
- **Array Methods**: `.length`, `.[index]`
- **Type Conversion**: `.to_s(encoding)`, `.to_i`
- **IO Access**: `_io.pos`, `_io.size`, `_io.eof`
- **Size Functions**: `_sizeof`, `sizeof<type>`

### Endianness
- **Global**: Set in `meta.endian` (`be` or `le`)
- **Per-Type**: Append `be` or `le` to type name (e.g., `u4le`, `s2be`)
- **Bit Endianness**: Set in `meta.bit-endian`
- **Default**: Big-endian if not specified

### Structure Organization
- `meta`: Schema metadata (id, endian, encoding, etc.)
- `seq`: Sequence of fields (main data structure)
- `types`: User-defined type definitions
- `instances`: Computed fields and lazy evaluation
- `enums`: Enumeration definitions
- `doc`: Documentation strings

### Common Patterns
- **Length-Prefixed Data**: Use size expression referencing previous field
- **Conditional Fields**: Use `if` expressions for optional data
- **Switch Types**: Use `switch-on` for polymorphic parsing
- **Substreams**: Process data chunks independently
- **Validation**: Use `contents` for magic numbers/signatures

## CEL Integration Lessons Learned

### Array Type Conversion Strategy
**Problem**: Array literals like `[65, 67, 75]` need different handling based on context:
- For byte comparisons: should become `[]byte` for CEL `types.Bytes` compatibility
- For general array operations: should remain `[]int64` for type safety

**Solution**: Naming-based heuristic in `evaluateInstance` (`pkg/kaitaistruct/parser.go:1647`):
```go
// Only convert to bytes if:
// 1. All values are in byte range (0-255)
// 2. Instance name doesn't suggest it should stay as integers (like "int_array")
shouldConvertToBytes := allInts && allValidBytes && !strings.Contains(strings.ToLower(instanceName), "int")
```

### CEL Function Overload Conflicts
**Problem**: CEL's standard library includes built-in functions that can conflict with custom overloads.

**Key Findings**:
- CEL has built-in `size()` function for collections
- Adding custom `size` function with `cel.MemberOverload` causes signature collisions
- Error: `"function declaration merge failed: overload signature collision in function size: list_size collides with custom_overload"`

**Solutions**:
- Use unique function names (`array_first`, `array_last`, `array_min`, `array_max`) to avoid conflicts
- Leverage CEL's built-in functions when possible instead of reimplementing

### Size Property vs Function Ambiguity
**Critical Issue**: Kaitai's `.size` syntax has dual meanings:
1. **Property access**: `_io.size` → access "size" key in map (returns stored value like `4`)
2. **Collection size**: `array.size` → get length of collection (returns count like `7`)

**Problem**: Using uniform approach breaks one use case:
- `size(map)` returns number of keys (1) instead of value of "size" property (4)
- Property access `array.size` fails with "unsupported index type 'string' in list"

**Solution**: Context-aware AST transformation (`internal/cel/ASTTransformer.go:271-305`):
```go
case "size":
    // Handle .size based on context:
    // - For _io or expressions that contain "_io": use property access
    // - For everything else: use size() function
    isIoContext := false
    
    // Check if this is direct _io access
    if _, isDirectIo := node.Value.(*expr.Io); isDirectIo {
        isIoContext = true
    } else if attr, isAttr := node.Value.(*expr.Attr); isAttr && attr.Name == "_io" {
        isIoContext = true  
    } else if id, isId := node.Value.(*expr.Id); isId && strings.Contains(id.Name, "_io") {
        isIoContext = true
    }
    
    if isIoContext {
        // For _io contexts, use property access: obj._io.size
        return obj + ".size"
    } else {
        // For arrays, use CEL function: size(array)
        return "size(" + obj + ")"
    }
```

### String Conversion for Byte Arrays
**Enhancement**: Added `types.Bytes` support in `to_s()` function (`internal/cel/stringFunctions.go:36-38`):
```go
case types.Bytes:
    // Convert CEL bytes directly to string
    return types.String(string(v))
```

This enables expressions like `byte_array.to_s("ASCII")` to work correctly.

### Binary Data Processing Architecture
**Problem**: Need for binary data transformations (XOR, rotation, compression) in Kaitai parsing.

**Key Findings**:
- CEL's built-in functions are insufficient for binary processing:
  - ❌ No bitwise XOR operator (`^` not supported)
  - ❌ No bytes indexing or manipulation functions
  - ❌ No compression/decompression capabilities
  - ❌ No bit rotation operations

**Solution**: Custom CEL functions in `internal/cel/kaitaiAPI.go`:
- `processXOR(data, key)` - XOR transformation using Kaitai runtime
- `processRotateLeft(data, amount)` / `processRotateRight(data, amount)` - Bit rotation
- `processZlib(data)` - Zlib decompression
- `processZlibCompress(data)` - Zlib compression (for serialization)
- Direct integration with `github.com/kaitai-io/kaitai_struct_go_runtime/kaitai` for battle-tested implementations

### Serializer Process Improvements
**Enhancements made to `pkg/kaitaistruct/serializer.go`**:

1. **Complete Process Support**: Added support for all process functions including parameterless ones:
   - Fixed parsing to handle both `xor(0xff)` and `zlib` formats
   - Added support for `rol`/`ror` aliases matching the parser

2. **Zlib Compression**: Implemented missing `processZlibCompress` CEL function for serialization:
   - Custom `compressZlib()` helper function since Kaitai runtime only has decompression
   - Proper error handling and round-trip compatibility

3. **Improved Context Handling**: Enhanced reverse process activation to match parser behavior:
   - Full context propagation including `_parent` and `_root` references
   - Better support for field-based parameters in process functions

4. **Round-Trip Verification**: Added comprehensive tests for serialize → parse cycles:
   - All process functions (XOR, rotate, zlib) now work correctly in both directions
   - Proper validation of data integrity through complete round-trips

### Process Integration Critical Bug Fix
**Problem**: Process functions (like `xor`, `zlib`, `rotate`) were not being applied to `size-eos` fields.

**Root Cause**: Process application logic was split between two code paths:
1. **Main field parsing** (`pkg/kaitaistruct/parser.go:926`): Applied process only when `size > 0`
2. **Specialized field parsers** (`parseBytesField`, `parseStringField`): Never applied process

**Issue**: `size-eos` fields have `size = 0` initially, so they take the specialized parser path which skipped process application entirely.

**Solution**: Added process handling to `parseBytesField` (`pkg/kaitaistruct/parser.go:1580-1588`):
```go
// Apply process if specified
if field.Process != "" {
    k.logger.DebugContext(ctx, "Processing bytes field data", "field_id", field.ID, "process_spec", field.Process)
    processedData, err := k.processDataWithCEL(ctx, bytesData, field.Process, pCtx)
    if err != nil {
        return nil, fmt.Errorf("processing bytes field '%s' data with spec '%s': %w", field.ID, field.Process, err)
    }
    bytesData = processedData
}
```

**Testing Strategy**: Use actual Kaitai test suite data from `test/src/*.bin` files. Find binary file names in Go test files:
```go
// Pattern in test/go/*_test.go files:
f, err := os.Open("../../src/process_xor_1.bin")
```

## Test Coverage Improvements

### Extended Test Suite (`pkg/kaitaistruct/kaitai_suite_extended_test.go`)
**Current Coverage**: 71.0% of statements (+0.5% improvement)

**Added Tests**:
1. **Process Function Tests**: XOR (constant & field-based), rotate left/right, zlib compression
2. **Enum Handling Tests**: Basic enum functionality with proper map structure validation
3. **Round-Trip Tests**: Complete parse → serialize → parse validation for basic structures
4. **Integration Tests**: Real test data from Kaitai official test suite

**Round-Trip Validation**:
- ✅ Basic data structures (integers, bytes, length-prefixed fields)
- ✅ Multi-field layouts with different endianness
- ✅ Complex nested structures
- ✅ **Process functions**: XOR, rotate, zlib with all field types including `size-eos`

## Issues to Address

1. **ExprSizeofType**: sizeof<type> syntax not implemented
2. ~~**ExprBytesCmp**: Byte array vs int array comparison type mismatch~~ ✅ **FIXED**
3. ~~**ExprIoTernary**: Complex ternary expressions with I/O access~~ ✅ **FIXED**
4. ~~**CEL Process Integration**: Process functions not applied to size-eos fields~~ ✅ **FIXED**  
5. ~~**Serializer Process Bug**: Process functions not applied correctly to fields in serialization~~ ✅ **FIXED**
6. **ExprStrOps**: String methods (reverse, substring, to_i) not implemented

### Serializer Process Function Bug Fix ✅ **FIXED**
**Problem**: Round-trip tests revealed that process functions (XOR, rotate, zlib) were not being applied correctly during serialization.

**Root Cause**: Order of checks in `serializeField()` function (lines 387-399):
```go
// ❌ BEFORE: Type-specific handling happened before process handling
if field.Type == "bytes" {
    return k.serializeBytesField(goCtx, field, data, sCtx)  // Bypassed process
}
if field.Process != "" {
    return k.serializeProcessedField(goCtx, field, data, sCtx)  // Never reached
}
```

**Solution**: Reordered checks to prioritize process handling:
```go  
// ✅ AFTER: Process handling happens first
if field.Process != "" {
    return k.serializeProcessedField(goCtx, field, data, sCtx)
}
if field.Type == "bytes" {
    return k.serializeBytesField(goCtx, field, data, sCtx)
}
```

**Validation**: All round-trip tests now pass:
- ✅ XOR with constants and field values
- ✅ Rotate left and right operations  
- ✅ Zlib compression/decompression
- ✅ Complete parse → serialize → parse integrity