package kaitaistruct

import (
	"gopkg.in/yaml.v3"
)

// KaitaiSchema represents a parsed KSY schema file
type KaitaiSchema struct {
	Meta      Meta                   `yaml:"meta"`
	Seq       []SequenceItem         `yaml:"seq"`
	Types     map[string]Type        `yaml:"types"`
	Instances map[string]InstanceDef `yaml:"instances"`
	Enums     map[string]EnumDef     `yaml:"enums"`
	Doc       string                 `yaml:"doc"`
	DocRef    string                 `yaml:"doc-ref"`
	Params    []ParameterDef         `yaml:"params"`
	RootType  string                 `yaml:"-"` // Not in the schema, but set by the processor
}

// Meta contains metadata about the KSY schema
type Meta struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Application string   `yaml:"application"`
	FileExt     []string `yaml:"file-ext"`
	License     string   `yaml:"license"`
	Ks          string   `yaml:"ks-version"`
	Endian      string   `yaml:"endian"`
	BitEndian   string   `yaml:"bit-endian"`
	Encoding    string   `yaml:"encoding"`
	Imports     []string `yaml:"imports"`
}

// SequenceItem represents a field in the binary structure
type SequenceItem struct {
	ID          string        `yaml:"id"`
	Type        any           `yaml:"type"` // Can be string or switch-on object
	Value       string        `yaml:"value,omitempty"`
	Enum        string        `yaml:"enum,omitempty"`
	Repeat      string        `yaml:"repeat,omitempty"`
	RepeatExpr  string        `yaml:"repeat-expr,omitempty"`
	RepeatUntil string        `yaml:"repeat-until,omitempty"`
	Size        any           `yaml:"size,omitempty"`
	SizeEOS     bool          `yaml:"size-eos,omitempty"`
	IfExpr      string        `yaml:"if,omitempty"`
	Process     string        `yaml:"process,omitempty"`
	Contents    any           `yaml:"contents,omitempty"`
	Terminator  any           `yaml:"terminator,omitempty"`
	Include     any           `yaml:"include,omitempty"`
	Consume     any           `yaml:"consume,omitempty"`
	Encoding    string        `yaml:"encoding,omitempty"`
	PadRight    any           `yaml:"pad-right,omitempty"`
	Doc         string        `yaml:"doc,omitempty"`
	DocRef      string        `yaml:"doc-ref,omitempty"`
	Switch      any           `yaml:"switch,omitempty"`
	Valid       *ValidationDef `yaml:"valid,omitempty"`
}

// Type defines a custom type in the KSY schema
type Type struct {
	Seq       []SequenceItem         `yaml:"seq"`
	Types     map[string]*Type       `yaml:"types"`
	Instances map[string]InstanceDef `yaml:"instances"`
	Params    []ParameterDef         `yaml:"params"`
	Doc       string                 `yaml:"doc"`
	DocRef    string                 `yaml:"doc-ref"`
}

// InstanceDef defines an instance (calculated field) in the KSY schema
type InstanceDef struct {
	Value      string `yaml:"value"`
	Type       string `yaml:"type,omitempty"`
	Repeat     string `yaml:"repeat,omitempty"`
	RepeatExpr string `yaml:"repeat-expr,omitempty"`
	IfExpr     string `yaml:"if,omitempty"`
	Encoding   string `yaml:"encoding,omitempty"`
	Doc        string `yaml:"doc,omitempty"`
	DocRef     string `yaml:"doc-ref,omitempty"`
}

// EnumDef defines an enumeration in the KSY schema
type EnumDef map[any]string

// ParameterDef defines a parameter in the KSY schema
type ParameterDef struct {
	ID     string `yaml:"id"`
	Type   string `yaml:"type"`
	Doc    string `yaml:"doc,omitempty"`
	DocRef string `yaml:"doc-ref,omitempty"`
}

// SwitchType defines a switch type in the KSY schema
type SwitchType struct {
	SwitchOn string         `yaml:"switch-on"`
	Cases    map[string]any `yaml:"cases"`
}

// ValidationDef defines validation rules for a field
type ValidationDef struct {
	// Simple validation (direct value comparison)
	Value any `yaml:"-"` // Set when validation is a simple value like "valid: 123"
	
	// Complex validation rules
	Expr   string `yaml:"expr,omitempty"`   // Expression-based validation
	Min    any    `yaml:"min,omitempty"`    // Minimum value
	Max    any    `yaml:"max,omitempty"`    // Maximum value
	AnyOf  []any  `yaml:"any-of,omitempty"` // List of allowed values
	InEnum bool   `yaml:"in-enum,omitempty"` // Must be valid enum value
}

// UnmarshalYAML implements custom YAML unmarshaling for ValidationDef
// This handles both simple values (valid: 123) and complex objects (valid: {min: 5, max: 10})
func (v *ValidationDef) UnmarshalYAML(value *yaml.Node) error {
	// If it's a simple scalar value, store it directly
	if value.Kind == yaml.ScalarNode {
		// Try to decode as different types
		var intVal int64
		var floatVal float64
		var strVal string
		var boolVal bool
		
		if err := value.Decode(&intVal); err == nil {
			v.Value = intVal
			return nil
		}
		if err := value.Decode(&floatVal); err == nil {
			v.Value = floatVal
			return nil
		}
		if err := value.Decode(&boolVal); err == nil {
			v.Value = boolVal
			return nil
		}
		if err := value.Decode(&strVal); err == nil {
			v.Value = strVal
			return nil
		}
		return nil
	}
	
	// If it's a sequence (array), store it directly as Value
	if value.Kind == yaml.SequenceNode {
		var seqVal []any
		if err := value.Decode(&seqVal); err == nil {
			v.Value = seqVal
			return nil
		}
	}
	
	// If it's a complex object, decode normally
	type validationAlias ValidationDef
	var alias validationAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*v = ValidationDef(alias)
	return nil
}

// NewKaitaiSchemaFromYAML parses a Kaitai Struct YAML schema into a KaitaiSchema struct.
func NewKaitaiSchemaFromYAML(data []byte) (*KaitaiSchema, error) {
	var schema KaitaiSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

// CalculateTypeSize calculates the size of a custom type in bytes
func (s *KaitaiSchema) CalculateTypeSize(typeName string) int64 {
	// Look up the type in our types map
	typeRef, exists := s.Types[typeName]
	if !exists {
		return 0
	}

	var totalSize int64

	// Calculate the size of all sequence fields
	for _, field := range typeRef.Seq {
		fieldSize := s.calculateFieldSize(field)
		totalSize += fieldSize
	}

	return totalSize
}

// calculateFieldSize calculates the size of a single field
func (s *KaitaiSchema) calculateFieldSize(field SequenceItem) int64 {
	// If field has an explicit size, use it
	if field.Size != nil {
		switch size := field.Size.(type) {
		case int:
			return int64(size)
		case int64:
			return size
		case string:
			// For expressions, we can't easily calculate without context
			// Return 0 for now - this would need CEL evaluation
			return 0
		}
	}

	// If field has a type, calculate based on type
	if field.Type != nil {
		switch fieldType := field.Type.(type) {
		case string:
			return s.getBuiltinTypeSize(fieldType)
		default:
			// Complex type specification
			return 0
		}
	}

	return 0
}

// getBuiltinTypeSize returns the size of built-in Kaitai types
func (s *KaitaiSchema) getBuiltinTypeSize(typeName string) int64 {
	switch typeName {
	case "u1", "s1":
		return 1
	case "u2", "s2", "u2le", "u2be", "s2le", "s2be":
		return 2
	case "u4", "s4", "u4le", "u4be", "s4le", "s4be":
		return 4
	case "u8", "s8", "u8le", "u8be", "s8le", "s8be":
		return 8
	case "f4", "f4le", "f4be":
		return 4
	case "f8", "f8le", "f8be":
		return 8
	default:
		// Check if it's a custom type
		return s.CalculateTypeSize(typeName)
	}
}
