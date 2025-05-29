// Package kbin provides a high-level API for parsing and serializing binary data
// using Kaitai Struct format specifications.
//
// This package simplifies the use of Kaitai Struct in Go applications by providing
// easy-to-use functions that handle the complexity of schema loading, parsing,
// and serialization.
//
// Basic usage:
//
//	// Parse binary data to a map
//	data, err := kbin.ParseBinary(binaryData, "path/to/schema.ksy")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Serialize to JSON
//	jsonData, err := kbin.SerializeToJSON(binaryData, "path/to/schema.ksy")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Convert JSON back to binary
//	binaryData, err := kbin.SerializeFromJSON(jsonData, "path/to/schema.ksy")
//	if err != nil {
//	    log.Fatal(err)
//	}
package kbin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
)

// Parser wraps the Kaitai functionality with caching and configuration
type Parser struct {
	schemaCache  map[string]*kaitaistruct.KaitaiSchema
	cacheMutex   sync.RWMutex
	logger       *slog.Logger
	options      options
}

// options holds configuration for the parser
type options struct {
	rootType       string
	logger         *slog.Logger
	enableCaching  bool
	cacheTimeout   time.Duration
	importPaths    []string
	debugMode      bool
}

// Option is a function that configures parser options
type Option func(*options)

// WithRootType sets the root type to parse (defaults to the schema ID)
func WithRootType(rootType string) Option {
	return func(o *options) {
		o.rootType = rootType
	}
}

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithCaching enables schema caching with the specified timeout
func WithCaching(timeout time.Duration) Option {
	return func(o *options) {
		o.enableCaching = true
		o.cacheTimeout = timeout
	}
}

// WithImportPaths adds additional paths to search for imported schemas
func WithImportPaths(paths ...string) Option {
	return func(o *options) {
		o.importPaths = append(o.importPaths, paths...)
	}
}

// WithDebugMode enables debug logging
func WithDebugMode(enabled bool) Option {
	return func(o *options) {
		o.debugMode = enabled
	}
}

// defaultOptions returns the default configuration
func defaultOptions() options {
	return options{
		logger:        slog.Default(),
		enableCaching: true,
		cacheTimeout:  5 * time.Minute,
		importPaths:   []string{},
		debugMode:     false,
	}
}

// Global parser instance for convenience functions
var globalParser *Parser
var globalParserOnce sync.Once

// getGlobalParser returns a singleton parser instance
func getGlobalParser() *Parser {
	globalParserOnce.Do(func() {
		globalParser = NewParser()
	})
	return globalParser
}

// NewParser creates a new parser instance with the given options
func NewParser(opts ...Option) *Parser {
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	if options.debugMode {
		options.logger = options.logger.With("debug", true)
	}

	return &Parser{
		schemaCache: make(map[string]*kaitaistruct.KaitaiSchema),
		logger:      options.logger,
		options:     options,
	}
}

// ParseBinary parses binary data using the specified Kaitai schema
func ParseBinary(data []byte, schemaPath string, opts ...Option) (map[string]any, error) {
	parser := getGlobalParser()
	return parser.ParseBinary(context.Background(), data, schemaPath, opts...)
}

// ParseBinaryWithContext parses binary data using the specified Kaitai schema with a context
func ParseBinaryWithContext(ctx context.Context, data []byte, schemaPath string, opts ...Option) (map[string]any, error) {
	parser := getGlobalParser()
	return parser.ParseBinary(ctx, data, schemaPath, opts...)
}

// SerializeToJSON parses binary data and converts it to JSON
func SerializeToJSON(data []byte, schemaPath string, opts ...Option) ([]byte, error) {
	parser := getGlobalParser()
	return parser.SerializeToJSON(context.Background(), data, schemaPath, opts...)
}

// SerializeToJSONWithContext parses binary data and converts it to JSON with a context
func SerializeToJSONWithContext(ctx context.Context, data []byte, schemaPath string, opts ...Option) ([]byte, error) {
	parser := getGlobalParser()
	return parser.SerializeToJSON(ctx, data, schemaPath, opts...)
}

// SerializeFromJSON converts JSON data back to binary format
func SerializeFromJSON(jsonData []byte, schemaPath string, opts ...Option) ([]byte, error) {
	parser := getGlobalParser()
	return parser.SerializeFromJSON(context.Background(), jsonData, schemaPath, opts...)
}

// SerializeFromJSONWithContext converts JSON data back to binary format with a context
func SerializeFromJSONWithContext(ctx context.Context, jsonData []byte, schemaPath string, opts ...Option) ([]byte, error) {
	parser := getGlobalParser()
	return parser.SerializeFromJSON(ctx, jsonData, schemaPath, opts...)
}

// ParseBinary parses binary data using the specified Kaitai schema
func (p *Parser) ParseBinary(ctx context.Context, data []byte, schemaPath string, opts ...Option) (map[string]any, error) {
	// Apply any additional options
	options := p.options
	for _, opt := range opts {
		opt(&options)
	}

	// Load the schema
	schema, err := p.loadSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("loading schema: %w", err)
	}

	// Determine root type
	rootType := options.rootType
	if rootType == "" {
		rootType = schema.Meta.ID
	}

	// Create an interpreter for this schema
	interpreter, err := kaitaistruct.NewKaitaiInterpreter(schema, p.logger)
	if err != nil {
		return nil, fmt.Errorf("creating interpreter: %w", err)
	}

	// Create a stream from the data
	stream := kaitai.NewStream(bytes.NewReader(data))

	// Parse the data
	result, err := interpreter.Parse(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("parsing data: %w", err)
	}

	// Convert ParsedData to map
	resultMap := p.convertParsedDataToMap(result)
	return resultMap, nil
}

// SerializeToJSON parses binary data and converts it to JSON
func (p *Parser) SerializeToJSON(ctx context.Context, data []byte, schemaPath string, opts ...Option) ([]byte, error) {
	// Parse the binary data first
	result, err := p.ParseBinary(ctx, data, schemaPath, opts...)
	if err != nil {
		return nil, err
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling to JSON: %w", err)
	}

	return jsonData, nil
}

// SerializeFromJSON converts JSON data back to binary format
func (p *Parser) SerializeFromJSON(ctx context.Context, jsonData []byte, schemaPath string, opts ...Option) ([]byte, error) {
	// Apply any additional options
	options := p.options
	for _, opt := range opts {
		opt(&options)
	}

	// Load the schema
	schema, err := p.loadSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("loading schema: %w", err)
	}

	// Determine root type
	rootType := options.rootType
	if rootType == "" {
		rootType = schema.Meta.ID
	}

	// Parse JSON into a map
	var data map[string]any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	// Create a serializer
	serializer, err := kaitaistruct.NewKaitaiSerializer(schema, p.logger)
	if err != nil {
		return nil, fmt.Errorf("creating serializer: %w", err)
	}

	// Set the root type on the schema
	schema.RootType = rootType
	
	// Serialize to binary
	result, err := serializer.Serialize(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("serializing data: %w", err)
	}

	return result, nil
}

// loadSchema loads a schema from disk with caching support
func (p *Parser) loadSchema(schemaPath string) (*kaitaistruct.KaitaiSchema, error) {
	// Check cache first if enabled
	if p.options.enableCaching {
		p.cacheMutex.RLock()
		cached, exists := p.schemaCache[schemaPath]
		p.cacheMutex.RUnlock()
		if exists {
			return cached, nil
		}
	}

	// Load the schema from disk
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("reading schema file: %w", err)
	}

	// Parse the schema
	schema, err := kaitaistruct.NewKaitaiSchemaFromYAML(data)
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}

	// Note: Import paths would need to be handled by the interpreter when parsing imports
	// The schema itself doesn't have an ImportPaths field

	// Cache the schema if enabled
	if p.options.enableCaching {
		p.cacheMutex.Lock()
		p.schemaCache[schemaPath] = schema
		p.cacheMutex.Unlock()
	}

	return schema, nil
}

// ClearCache clears the schema cache
func (p *Parser) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.schemaCache = make(map[string]*kaitaistruct.KaitaiSchema)
}

// convertParsedDataToMap converts ParsedData structure to a map
func (p *Parser) convertParsedDataToMap(pd *kaitaistruct.ParsedData) map[string]any {
	if pd == nil {
		return nil
	}

	result := make(map[string]any)
	
	// If the parsed data has a direct value (for primitive types), return it wrapped in a map
	if pd.Value != nil && pd.Children == nil {
		result["value"] = p.convertToGoTypes(pd.Value)
		return result
	}

	// Process all children
	for name, child := range pd.Children {
		// Skip internal fields
		if name == "_io" || name == "_parent" || name == "_root" {
			continue
		}

		if child.Value != nil {
			result[name] = p.convertToGoTypes(child.Value)
		} else if child.Children != nil {
			// For nested structures, recurse
			result[name] = p.convertParsedDataToMap(child)
		}
	}

	return result
}

// convertToGoTypes recursively converts Kaitai types to standard Go types
func (p *Parser) convertToGoTypes(v any) any {
	// Check if it's a Kaitai type first to avoid infinite recursion
	if kaitaiType, ok := v.(interface{ Value() any }); ok {
		// Get the underlying value and stop recursion if it's the same object
		underlying := kaitaiType.Value()
		if underlying == v {
			// This can happen with some types, just return the value as-is
			return v
		}
		return p.convertToGoTypes(underlying)
	}

	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			// Skip internal fields
			if k == "_io" || k == "_parent" || k == "_root" {
				continue
			}
			result[k] = p.convertToGoTypes(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = p.convertToGoTypes(item)
		}
		return result
	default:
		return val
	}
}

// ValidateSchema validates a Kaitai schema file without parsing any data
func ValidateSchema(schemaPath string) error {
	parser := getGlobalParser()
	return parser.ValidateSchema(schemaPath)
}

// ValidateSchema validates a Kaitai schema file without parsing any data
func (p *Parser) ValidateSchema(schemaPath string) error {
	_, err := p.loadSchema(schemaPath)
	return err
}