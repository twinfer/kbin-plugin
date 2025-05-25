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
	Encoding    string   `yaml:"encoding"`
	Imports     []string `yaml:"imports"`
}

// SequenceItem represents a field in the binary structure
type SequenceItem struct {
	ID          string `yaml:"id"`
	Type        string `yaml:"type"`
	Enum        string `yaml:"enum,omitempty"`
	Repeat      string `yaml:"repeat,omitempty"`
	RepeatExpr  string `yaml:"repeat-expr,omitempty"`
	RepeatUntil string `yaml:"repeat-until,omitempty"`
	Size        any    `yaml:"size,omitempty"`
	SizeEOS     bool   `yaml:"size-eos,omitempty"`
	IfExpr      string `yaml:"if,omitempty"`
	Process     string `yaml:"process,omitempty"`
	Contents    any    `yaml:"contents,omitempty"`
	Terminator  any    `yaml:"terminator,omitempty"`
	Include     any    `yaml:"include,omitempty"`
	Consume     any    `yaml:"consume,omitempty"`
	Encoding    string `yaml:"encoding,omitempty"`
	PadRight    any    `yaml:"pad-right,omitempty"`
	Doc         string `yaml:"doc,omitempty"`
	DocRef      string `yaml:"doc-ref,omitempty"`
	Switch      any    `yaml:"switch,omitempty"`
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

// NewKaitaiSchemaFromYAML parses a Kaitai Struct YAML schema into a KaitaiSchema struct.
func NewKaitaiSchemaFromYAML(data []byte) (*KaitaiSchema, error) {
	var schema KaitaiSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}
