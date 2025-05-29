# Kaitai Test Coverage Analysis

## Current Status
- **Current Coverage**: 72.9% (was 71.0%, improved by +1.9%)
- **Target Coverage**: 85% (+12.1% improvement needed)
- **Official Test Cases Available**: 188 test cases
- **Our Current Test Files**: 12 test files

## Missing Test Categories (High Impact for Coverage)

### 1. **Validation Tests** (28 test cases) 🔥 **HIGHEST PRIORITY**
**Impact**: Data validation is core parser functionality - likely 15-20% coverage impact
**Examples**:
- `valid_fail_eq_int` - Integer equality validation
- `valid_fail_range_bytes` - Byte range validation  
- `valid_fail_contents` - Fixed content validation
- `valid_fail_expr` - Expression-based validation
- `valid_fail_repeat_*` - Array validation patterns

**Implementation Strategy**: Add comprehensive validation test suite covering:
- Range validations (min/max for integers, floats, strings, bytes)
- Equality validations (exact value matching)
- Content validations (magic numbers, fixed sequences)
- Conditional validations (if-based validation rules)
- Array element validations (repeat + validation)

### 2. **Expression Tests** (17 test cases) 🔥 **HIGH PRIORITY**  
**Impact**: Expression evaluation is heavily used - likely 8-12% coverage impact
**Missing Areas**:
- `expr_str_ops` - String operations (.reverse, .substring, .to_i)
- `expr_sizeof_*` - Size calculation functions
- `expr_calc_array_ops` - Array operations and calculations
- `expr_bytes_ops` - Byte array operations
- `expr_io_*` - I/O position and EOF handling

**Current Gap**: We have basic expression tests but missing advanced string/array/size operations.

### 3. **Enum Tests** (15 test cases) 🔥 **HIGH PRIORITY**
**Impact**: Enum handling is fundamental - likely 6-10% coverage impact  
**Missing Areas**:
- `enum_deep` - Nested enum structures
- `enum_import_*` - Cross-file enum imports
- `enum_to_i` - Enum-to-integer conversion
- `enum_if` - Conditional enum usage
- `enum_invalid` - Invalid enum value handling

**Current Gap**: We have basic enum support but missing complex scenarios.

### 4. **Repeat/Array Tests** (8 test cases) 🟡 **MEDIUM PRIORITY**
**Impact**: Array parsing is common - likely 4-6% coverage impact
**Missing Areas**:
- `repeat_until_*` - Conditional array termination
- `repeat_eos_*` - Arrays until end-of-stream
- `repeat_n_*` - Fixed-count arrays with complex elements

### 5. **Navigation Tests** (7 test cases) 🟡 **MEDIUM PRIORITY**  
**Impact**: Parent/root navigation - likely 3-5% coverage impact
**Missing Areas**:
- `nav_parent*` - Parent object navigation
- `nav_root*` - Root object navigation
- Parent-child relationship handling

### 6. **Import Tests** (8 test cases) 🟡 **MEDIUM PRIORITY**
**Impact**: Cross-file imports - likely 3-5% coverage impact  
**Missing Areas**:
- `imports_*` - Various import scenarios
- Cross-schema type references
- Relative vs absolute imports

### 7. **Parameter Tests** (9 test cases) 🟡 **MEDIUM PRIORITY**
**Impact**: Parameterized types - likely 3-4% coverage impact
**Missing Areas**:
- `params_def*` - Parameter definitions
- `params_pass_*` - Parameter passing
- `params_call*` - Parameter invocation

## Recommended Implementation Order

### Phase 1: Validation Tests (Target: +8% coverage)
```go
// Add to kaitai_suite_extended_test.go or new validation_test.go
func TestValidation_IntegerRange(t *testing.T) { ... }
func TestValidation_ByteContents(t *testing.T) { ... }
func TestValidation_ExpressionBased(t *testing.T) { ... }
func TestValidation_ArrayElements(t *testing.T) { ... }
```

### Phase 2: String Expression Tests (Target: +3% coverage)
```go
func TestStringOperations_Reverse(t *testing.T) { ... }
func TestStringOperations_Substring(t *testing.T) { ... }
func TestStringOperations_ToInteger(t *testing.T) { ... }
```

### Phase 3: Advanced Enum Tests (Target: +2% coverage)  
```go
func TestEnums_DeepNesting(t *testing.T) { ... }
func TestEnums_CrossFileImports(t *testing.T) { ... }
func TestEnums_InvalidValues(t *testing.T) { ... }
```

## Test Data Files Needed
Key binary test files from `test/src/`:
- `fixed_struct.bin` - For validation tests
- `term_strz.bin` - For string operation tests  
- `enum_0.bin` - For enum tests
- `nav.bin` - For navigation tests
- `repeat_until_s4.bin` - For repeat tests

## Code Areas Likely Needing Enhancement

### 1. Validation Implementation
- `pkg/kaitaistruct/parser.go` - Add validation logic after field parsing
- Likely missing: Range checks, content validation, expression-based validation

### 2. String Operations  
- `internal/cel/stringFunctions.go` - Add missing .reverse, .substring, .to_i methods
- `internal/cel/ASTTransformer.go` - Handle method call transformations

### 3. Enum Edge Cases
- `pkg/kaitaicel/kaitai-cel-enum-types.go` - Handle invalid values, nested enums
- Import resolution for cross-file enums

### 4. Size Functions
- Missing `sizeof<type>` and advanced `_sizeof` implementations
- Array size calculations

## Expected Coverage Improvement
- **Phase 1 (Validation)**: 72.9% → 81% (+8.1%)
- **Phase 2 (Expressions)**: 81% → 84% (+3.0%) 
- **Phase 3 (Enums)**: 84% → 86% (+2.0%)
- **Total**: 72.9% → 86% (+13.1%) - **Exceeds 85% target**

## Implementation Notes
- Focus on **parser.go** validation logic - this is likely the biggest coverage gap
- String operations need CEL function enhancements
- Use existing binary test data from official Kaitai test suite
- Prioritize tests that exercise error conditions and edge cases
- Validation tests will likely reveal missing error handling code paths