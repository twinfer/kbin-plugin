package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/redpanda-data/benthos/v4/public/service"
	kst "github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
	"gopkg.in/yaml.v3"
)

// KaitaiProcessor is a Benthos processor that uses Kaitai Struct
// to parse and serialize binary data without code generation.
type KaitaiProcessor struct {
	config       KaitaiConfig
	schemaMap    sync.Map // Cache for parsed schemas
	logger       *service.Logger
	mParsed      *service.MetricCounter
	mSerialized  *service.MetricCounter
	mErrors      *service.MetricCounter
	mCacheHits   *service.MetricCounter
	mCacheMisses *service.MetricCounter
}

// KaitaiConfig contains configuration parameters for the Kaitai processor.
type KaitaiConfig struct {
	SchemaPath string `json:"schema_path" yaml:"schema_path"`
	IsParser   bool   `json:"is_parser" yaml:"is_parser"`
	RootType   string `json:"root_type" yaml:"root_type"`
}

func init() {
	// Register the processor with Benthos
	err := service.RegisterProcessor(
		"kaitai",
		kaitaiProcessorConfig(),
		func(conf *service.ParsedConfig, mgr *service.Resources) (service.Processor, error) {
			return newKaitaiProcessorFromConfig(conf, mgr)
		},
	)
	if err != nil {
		panic(err)
	}
}

// kaitaiProcessorConfig returns a config spec for a kaitai processor.
func kaitaiProcessorConfig() *service.ConfigSpec {
	return service.NewConfigSpec().
		Summary("Parses or serializes binary data using Kaitai Struct definitions without code generation.").
		Description("This processor uses Kaitai Struct to parse binary data into JSON or serialize JSON back to binary according to KSY schema definitions.").
		Field(service.NewStringField("schema_path").
			Description("Path to the Kaitai Struct (.ksy) schema file.").
			Example("./schemas/my_format.ksy")).
		Field(service.NewBoolField("is_parser").
			Description("Whether this processor parses binary to JSON (true) or serializes JSON to binary (false).").
			Default(true)).
		Field(service.NewStringField("root_type").
			Description("The root type name from the KSY file to use when parsing or serializing. Leave empty to use the default root type.").
			Default("")).
		Version("0.1.0")
}

// newKaitaiProcessorFromConfig creates a new KaitaiProcessor from a parsed config.
func newKaitaiProcessorFromConfig(conf *service.ParsedConfig, mgr *service.Resources) (*KaitaiProcessor, error) {
	schemaPath, err := conf.FieldString("schema_path")
	if err != nil {
		return nil, err
	}

	isParser, err := conf.FieldBool("is_parser")
	if err != nil {
		return nil, err
	}

	rootType, err := conf.FieldString("root_type")
	if err != nil {
		return nil, err
	}

	config := KaitaiConfig{
		SchemaPath: schemaPath,
		IsParser:   isParser,
		RootType:   rootType,
	}

	// Check if schema file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("schema file not found at path: %s", schemaPath)
	}

	logger := mgr.Logger()
	metrics := mgr.Metrics()

	return &KaitaiProcessor{
		config:       config,
		logger:       logger,
		mParsed:      metrics.NewCounter("kaitai_parsed_messages"),
		mSerialized:  metrics.NewCounter("kaitai_serialized_messages"),
		mErrors:      metrics.NewCounter("kaitai_processing_errors"),
		mCacheHits:   metrics.NewCounter("kaitai_schema_cache_hits"),
		mCacheMisses: metrics.NewCounter("kaitai_schema_cache_misses"),
	}, nil
}

// Process applies Kaitai parsing or serialization to a message.
func (k *KaitaiProcessor) Process(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	if k.config.IsParser {
		return k.parseBinary(ctx, msg)
	}
	return k.serializeToBinary(ctx, msg)
}

// parseBinary parses binary data into a JSON structure using Kaitai Struct.
func (k *KaitaiProcessor) parseBinary(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	k.logger.Debug("Parsing binary data with Kaitai Struct")

	// Get binary data from message
	binData, err := msg.AsBytes()
	if err != nil {
		k.logger.Errorf("Failed to get binary data from message: %v", err)
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("failed to get binary data from message: %w", err))
		return service.MessageBatch{msg}, nil
	}

	if len(binData) == 0 {
		k.logger.Warn("Empty binary data provided")
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("empty binary data provided"))
		return service.MessageBatch{msg}, nil
	}

	// Load schema
	schema, err := k.loadSchema(k.config.SchemaPath)
	if err != nil {
		k.logger.Errorf("Failed to load schema: %v", err)
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("failed to load schema: %w", err))
		return service.MessageBatch{msg}, nil
	}

	// Create Kaitai stream for parsing
	stream := kaitai.NewStream(bytes.NewReader(binData))

	// Create interpreter and parse data
	interpreter, _ := kst.NewKaitaiInterpreter(schema)
	parsedData, err := interpreter.Parse(stream)
	if err != nil {
		k.logger.Errorf("Failed to parse binary data of size %d bytes: %v", len(binData), err)
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("failed to parse binary data of size %d bytes: %w", len(binData), err))
		return service.MessageBatch{msg}, nil
	}

	// Convert parsed data to a map/JSON structure
	result := kst.ParsedDataToMap(parsedData)

	k.logger.Debugf("Successfully parsed %d bytes of binary data", len(binData))
	k.mParsed.Incr(1)

	// Create new message with parsed data
	newMsg := service.NewMessage(nil)
	newMsg.SetStructured(result)

	// Copy metadata from original message
	msg.MetaWalk(func(key, value string) error {
		newMsg.MetaSet(key, value)
		return nil
	})

	return service.MessageBatch{newMsg}, nil
}

// serializeToBinary serializes a JSON structure to binary using Kaitai Struct.
func (k *KaitaiProcessor) serializeToBinary(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	k.logger.Debug("Serializing structured data to binary with Kaitai Struct")

	// Get structured data from message
	structData, err := msg.AsStructured()
	if err != nil {
		k.logger.Errorf("Failed to get structured data from message: %v", err)
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("failed to get structured data from message: %w", err))
		return service.MessageBatch{msg}, nil
	}

	// Load schema
	schema, err := k.loadSchema(k.config.SchemaPath)
	if err != nil {
		k.logger.Errorf("Failed to load schema: %v", err)
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("failed to load schema: %w", err))
		return service.MessageBatch{msg}, nil
	}

	// Create serializer and serialize data
	serializer, _ := kst.NewKaitaiSerializer(schema)
	binData, err := serializer.Serialize(structData)
	if err != nil {
		k.logger.Errorf("Failed to serialize data: %v", err)
		k.mErrors.Incr(1)
		msg.SetError(fmt.Errorf("failed to serialize data: %w", err))
		return service.MessageBatch{msg}, nil
	}

	k.logger.Debugf("Successfully serialized data to %d bytes of binary data", len(binData))
	k.mSerialized.Incr(1)

	newMsg := service.NewMessage(binData)

	// Copy metadata from original message
	msg.MetaWalk(func(key, value string) error {
		newMsg.MetaSet(key, value)
		return nil
	})

	return service.MessageBatch{newMsg}, nil
}

// loadSchema loads and parses a KSY schema file.
func (k *KaitaiProcessor) loadSchema(path string) (*kst.KaitaiSchema, error) {
	// Check schema cache first
	if cachedSchema, ok := k.schemaMap.Load(path); ok {
		k.logger.Tracef("Schema cache hit for path: %s", path)
		k.mCacheHits.Incr(1)
		return cachedSchema.(*kst.KaitaiSchema), nil
	}

	k.logger.Debugf("Loading schema from path: %s", path)
	k.mCacheMisses.Incr(1)

	// Read schema file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// Parse YAML
	schema := &kst.KaitaiSchema{}
	if err := yaml.Unmarshal(data, schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema YAML: %w", err)
	}

	// Store in cache
	k.schemaMap.Store(path, schema)
	k.logger.Debugf("Loaded and cached schema from: %s", path)

	return schema, nil
}

// Close the processor resources
func (k *KaitaiProcessor) Close(ctx context.Context) error {
	k.logger.Debug("Closing Kaitai processor and clearing schema cache")
	k.schemaMap = sync.Map{} // Clear the cache
	return nil
}
