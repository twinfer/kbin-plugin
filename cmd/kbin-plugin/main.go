package main

import (
	"bytes"
	"context"
	"errors" // Import for errors.Is
	"fmt"
	"io" // Import for io.EOF and io.ErrUnexpectedEOF
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/redpanda-data/benthos/v4/public/service"
	kcel "github.com/twinfer/kbin-plugin/pkg/kaitaicel" // Import the kaitaicel package
	kst "github.com/twinfer/kbin-plugin/pkg/kaitaistruct"
	"gopkg.in/yaml.v3"
)

// benthosLogHandler wraps a Benthos logger to implement slog.Handler
type benthosLogHandler struct {
	logger *service.Logger
	attrs  []slog.Attr
	groups []string
}

// newBenthosLogHandler creates a new slog handler that forwards logs to Benthos
func newBenthosLogHandler(logger *service.Logger) slog.Handler {
	return &benthosLogHandler{
		logger: logger,
		attrs:  make([]slog.Attr, 0),
		groups: make([]string, 0),
	}
}

// Enabled implements slog.Handler
func (h *benthosLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Benthos logger doesn't expose its level, so we'll assume all levels are enabled
	// and let Benthos handle the filtering
	return true
}

// Handle implements slog.Handler
func (h *benthosLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Build the message with attributes
	msg := r.Message
	
	// Add attributes from the record
	fields := make(map[string]interface{})
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()
		return true
	})
	
	// Add pre-existing attributes
	for _, attr := range h.attrs {
		fields[attr.Key] = attr.Value.Any()
	}
	
	// Apply group prefixes
	if len(h.groups) > 0 {
		groupedFields := fields
		for i := len(h.groups) - 1; i >= 0; i-- {
			groupedFields = map[string]interface{}{
				h.groups[i]: groupedFields,
			}
		}
		fields = groupedFields
	}
	
	// Convert slog level to Benthos logging methods
	switch r.Level {
	case slog.LevelDebug:
		h.logger.With(fields).Debugf(msg)
	case slog.LevelInfo:
		h.logger.With(fields).Infof(msg)
	case slog.LevelWarn:
		h.logger.With(fields).Warnf(msg)
	case slog.LevelError:
		h.logger.With(fields).Errorf(msg)
	default:
		// For custom levels, use Info as default
		h.logger.With(fields).Infof(msg)
	}
	
	return nil
}

// WithAttrs implements slog.Handler
func (h *benthosLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := &benthosLogHandler{
		logger: h.logger,
		attrs:  make([]slog.Attr, len(h.attrs)+len(attrs)),
		groups: make([]string, len(h.groups)),
	}
	copy(newHandler.attrs, h.attrs)
	copy(newHandler.attrs[len(h.attrs):], attrs)
	copy(newHandler.groups, h.groups)
	return newHandler
}

// WithGroup implements slog.Handler
func (h *benthosLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newHandler := &benthosLogHandler{
		logger: h.logger,
		attrs:  make([]slog.Attr, len(h.attrs)),
		groups: make([]string, len(h.groups)+1),
	}
	copy(newHandler.attrs, h.attrs)
	copy(newHandler.groups, h.groups)
	newHandler.groups[len(h.groups)] = name
	return newHandler
}

// SystemTime is an interface for getting the current time.
var SystemTime SystemTimeInterface = systemTime{} // Default to real system time

// SystemTimeInterface defines the methods required for time operations.
type SystemTimeInterface interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

// systemTime is a concrete implementation of SystemTimeInterface using the standard library.
type systemTime struct{}

func (systemTime) Now() time.Time                  { return time.Now() }
func (systemTime) Since(t time.Time) time.Duration { return time.Since(t) }

// bufferPool provides a pool of byte buffers to reduce allocations
var bufferPool = sync.Pool{
	New: func() any {
		// Start with 4KB buffers
		buf := make([]byte, 4096)
		return &buf
	},
}

// getBuffer gets a buffer from the pool
func getBuffer(size int) []byte {
	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr
	if cap(buf) < size {
		// If the buffer is too small, allocate a new one
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}
	return buf
}

// putBuffer returns a buffer to the pool
func putBuffer(buf []byte) {
	// Only return buffers that aren't too large (to avoid memory bloat)
	if cap(buf) <= 1024*1024 { // 1MB max
		bufPtr := &buf
		bufferPool.Put(bufPtr)
	}
}

type KaitaiProcessor struct {
	config             KaitaiConfig
	schemaCache        sync.Map // Cache for main data schemas
	framingSchemaCache sync.Map // Cache for framing schemas
	logger             *service.Logger

	// Metrics
	mParsedTotal                *service.MetricCounter // Total successfully parsed output messages (payloads)
	mSerializedTotal            *service.MetricCounter // Total successfully serialized output messages
	mErrorsTotal                *service.MetricCounter // General processing errors not covered by more specific metrics
	mSchemaCacheHits            *service.MetricCounter
	mSchemaCacheMisses          *service.MetricCounter
	mFramingSchemaCacheHits     *service.MetricCounter
	mFramingSchemaCacheMisses   *service.MetricCounter
	mFramesProcessed            *service.MetricCounter
	mFrameParsingErrors         *service.MetricCounter
	mPayloadParsingErrors       *service.MetricCounter
	mPayloadSerializationErrors *service.MetricCounter
	mBytesProcessed             *service.MetricCounter
	mFrameProcDuration          *service.MetricTimer
}

// KaitaiConfig contains configuration parameters for the Kaitai processor
type KaitaiConfig struct {
	SchemaPath string `json:"schema_path" yaml:"schema_path"` // Path to the main KSY schema
	IsParser   bool   `json:"is_parser" yaml:"is_parser"`     // True for parsing binary to JSON, false for serializing JSON to binary
	RootType   string `json:"root_type" yaml:"root_type"`

	// Framing configuration (optional)
	FramingSchemaPath  string `json:"framing_schema_path,omitempty" yaml:"framing_schema_path,omitempty"`
	FramingRootType    string `json:"framing_root_type,omitempty" yaml:"framing_root_type,omitempty"`
	FramingDataFieldID string `json:"framing_data_field_id,omitempty" yaml:"framing_data_field_id,omitempty"`
	// MaxBufferSize int `json:"max_buffer_size,omitempty" yaml:"max_buffer_size,omitempty"` // Defer for now based on simplified framing
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
		Field(service.NewStringField("framing_schema_path").
			Description("Optional: Path to a KSY schema file used to define message/frame boundaries. If used, the processor will first parse frames using this schema, then parse the extracted payload using the main 'schema_path'.").
			Default("").Optional()).
		Field(service.NewStringField("framing_root_type").
			Description("Optional: The root type in the 'framing_schema_path' to parse for frame extraction. Defaults to the meta.id of the framing schema if left empty.").
			Default("").Optional()).
		Field(service.NewStringField("framing_data_field_id").
			Description("Required if 'framing_schema_path' is set. The ID of the field within the parsed framing structure that contains the actual data payload to be processed by the main schema.").
			Default("").Optional()).
		// Field(service.NewIntField("max_buffer_size").
		//     Description("Optional: Maximum size of the internal buffer for holding partial frame data, in bytes. Only used if framing is active.").
		//     Default(1024*1024*10).Optional()), // Example: 10MB, deferring for now
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

	framingSchemaPath, err := conf.FieldString("framing_schema_path")
	if err != nil {
		return nil, err
	}
	framingRootType, err := conf.FieldString("framing_root_type")
	if err != nil {
		return nil, err
	}
	framingDataFieldID, err := conf.FieldString("framing_data_field_id")
	if err != nil {
		return nil, err
	}

	// Validation for configuration
	if schemaPath == "" {
		return nil, fmt.Errorf("schema_path is required")
	}
	
	// Validation for framing
	if framingSchemaPath != "" && framingDataFieldID == "" {
		return nil, fmt.Errorf("framing_data_field_id is required when framing_schema_path is set")
	}
	
	// Validate that framing is only used with parser mode
	if framingSchemaPath != "" && !isParser {
		return nil, fmt.Errorf("framing is only supported in parser mode (is_parser: true)")
	}

	config := KaitaiConfig{
		SchemaPath:         schemaPath,
		IsParser:           isParser,
		RootType:           rootType,
		FramingSchemaPath:  framingSchemaPath,
		FramingRootType:    framingRootType,
		FramingDataFieldID: framingDataFieldID,
	}

	// Check if schema file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("schema file not found at path: %s", schemaPath)
	} else if err != nil {
		return nil, fmt.Errorf("error accessing schema file: %w", err)
	}
	
	if framingSchemaPath != "" {
		if _, err := os.Stat(framingSchemaPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("framing schema file not found at path: %s", framingSchemaPath)
		} else if err != nil {
			return nil, fmt.Errorf("error accessing framing schema file: %w", err)
		}
	}

	logger := mgr.Logger()
	metrics := mgr.Metrics()

	processorMode := "serializer"
	if config.IsParser {
		processorMode = "parser"
	}
	framingActive := config.FramingSchemaPath != "" && config.IsParser

	logger.Infof("Kaitai processor configured. Mode: %s, Schema: %s, RootType: %s, Framing active: %t",
		processorMode, config.SchemaPath, config.RootType, framingActive)
	if framingActive {
		logger.Infof("Framing config: Schema: %s, RootType: %s, DataFieldID: %s",
			config.FramingSchemaPath, config.FramingRootType, config.FramingDataFieldID)
	}

	kp := &KaitaiProcessor{
		config:                      config,
		logger:                      logger,
		mParsedTotal:                metrics.NewCounter("kaitai_parsed_total"),
		mSerializedTotal:            metrics.NewCounter("kaitai_serialized_total"),
		mErrorsTotal:                metrics.NewCounter("kaitai_errors_total"),
		mSchemaCacheHits:            metrics.NewCounter("kaitai_schema_cache_hits_total"),
		mSchemaCacheMisses:          metrics.NewCounter("kaitai_schema_cache_misses_total"),
		mFramingSchemaCacheHits:     metrics.NewCounter("kaitai_framing_schema_cache_hits_total"),
		mFramingSchemaCacheMisses:   metrics.NewCounter("kaitai_framing_schema_cache_misses_total"),
		mFramesProcessed:            metrics.NewCounter("kaitai_frames_processed_total"),
		mFrameParsingErrors:         metrics.NewCounter("kaitai_frame_parsing_errors_total"),
		mPayloadParsingErrors:       metrics.NewCounter("kaitai_payload_parsing_errors_total"),
		mPayloadSerializationErrors: metrics.NewCounter("kaitai_payload_serialization_errors_total"),
		mBytesProcessed:             metrics.NewCounter("kaitai_bytes_processed_total"),
		mFrameProcDuration:          metrics.NewTimer("kaitai_frame_processing_duration_seconds"),
		// schemaCache and framingSchemaCache are zero-value sync.Map and ready to use
	}
	return kp, nil
}

// Process applies Kaitai parsing or serialization to a message.
func (k *KaitaiProcessor) Process(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	if k.config.IsParser {
		k.logger.Debugf("Entering PARSER mode")
		if k.config.FramingSchemaPath != "" {
			k.logger.Debugf("Framing is ENABLED")
			return k.parseFramedBinary(ctx, msg)
		}
		k.logger.Debugf("Framing is DISABLED")
		return k.parseBinary(ctx, msg)
	}
	k.logger.Debugf("Entering SERIALIZER mode")
	return k.serializeToBinary(ctx, msg)
}

// parseFramedBinary handles parsing of messages that require KSY-defined framing.
// It extracts multiple frames from a single input message, parses each frame's payload,
// and returns a batch of messages, one for each successfully parsed payload.
func (k *KaitaiProcessor) parseFramedBinary(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	// Timing for individual frame processing is done inside the loop.

	inputData, err := msg.AsBytes()
	if err != nil {
		k.logger.Errorf("Failed to get binary data from message for framing: %v", err)
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to get binary data for framing: %w", err))
		return service.MessageBatch{msg}, nil
	}

	if len(inputData) == 0 {
		k.logger.Warnf("Empty binary data provided for framed parsing")
		return service.MessageBatch{}, nil
	}
	k.logger.With("message_len", len(inputData)).Debugf("Entering framed parsing for message")

	framingSchema, err := k.loadFramingSchema(k.config.FramingSchemaPath)
	if err != nil {
		// Error logged by loadFramingSchema
		k.mErrorsTotal.Incr(1) // General error as it's schema loading
		msg.SetError(fmt.Errorf("failed to load framing schema '%s': %w", k.config.FramingSchemaPath, err))
		return service.MessageBatch{msg}, nil
	}
	effectiveFramingRootType := k.config.FramingRootType
	if effectiveFramingRootType == "" && framingSchema.Meta.ID != "" {
		effectiveFramingRootType = framingSchema.Meta.ID
	}
	k.logger.With("path", k.config.FramingSchemaPath, "root_type", effectiveFramingRootType).Debugf("Using framing schema")

	dataSchema, err := k.loadDataSchema(k.config.SchemaPath)
	if err != nil {
		// Error logged by loadDataSchema
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to load data schema '%s': %w", k.config.SchemaPath, err))
		return service.MessageBatch{msg}, nil
	}

	inputStream := kaitai.NewStream(bytes.NewReader(inputData))
	streamSize, err := inputStream.Size()
	if err != nil {
		k.logger.Errorf("Failed to get initial stream size: %v", err)
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to get initial stream size: %w", err))
		return service.MessageBatch{msg}, nil
	}

	outputMessages := service.MessageBatch{}
	var mainFramingError error // To store the first critical framing error

	// Create slog.Logger instances using Benthos logger
	frameInterpreterSlog := slog.New(newBenthosLogHandler(k.logger)).With("component", "frame_interpreter")
	dataInterpreterSlog := slog.New(newBenthosLogHandler(k.logger)).With("component", "data_interpreter")

	for {
		currentPos, errLoopPos := inputStream.Pos()
		if errLoopPos != nil {
			k.logger.Errorf("Failed to get stream position in loop: %v", errLoopPos)
			mainFramingError = fmt.Errorf("stream error (pos in loop): %w", errLoopPos)
			k.mErrorsTotal.Incr(1)
			break
		}

		if !(currentPos < streamSize) {
			break // End of stream
		}

		frameStartPos := currentPos // Position at the start of this frame attempt
		bufferRemaining := streamSize - frameStartPos
		k.logger.With("stream_pos", frameStartPos, "buffer_remaining", bufferRemaining).Debugf("Attempting to parse frame")

		// Ensure RootType is set for the schema if specified in config
		// Create a copy of the schema to avoid modifying the cached version
		currentFramingSchema := *framingSchema
		if k.config.FramingRootType != "" {
			currentFramingSchema.RootType = k.config.FramingRootType
		}

		frameInterpreter, err := kst.NewKaitaiInterpreter(&currentFramingSchema, frameInterpreterSlog)
		if err != nil {
			k.logger.Errorf("Failed to create frame interpreter: %v", err)
			mainFramingError = fmt.Errorf("failed to create frame interpreter: %w", err)
			k.mErrorsTotal.Incr(1)
			break
		}

		frameParseStartTime := SystemTime.Now() // Start timing for this frame attempt
		frameContainerPd, frameErr := frameInterpreter.Parse(ctx, inputStream)

		// Helper to check for various EOF conditions
		isEofError := func(e error) bool {
			if e == nil {
				return false
			}
			_, isKaitaiCustomEof := e.(kaitai.EndOfStreamError)
			return isKaitaiCustomEof || errors.Is(e, io.EOF) || errors.Is(e, io.ErrUnexpectedEOF)
		}
		if frameErr != nil {
			if isEofError(frameErr) {
				k.logger.With("stream_pos", frameStartPos).Warnf("Partial frame data at end of message: %v", frameErr)
				mainFramingError = frameErr // This isn't necessarily a frame parsing error for metrics if it's just EOF
				break
			}
			k.logger.With("stream_pos", frameStartPos).Errorf("Error parsing frame container: %v", frameErr)
			k.mFrameParsingErrors.Incr(1)
			k.mErrorsTotal.Incr(1) // Also count as general error
			// Store the first critical framing error
			if mainFramingError == nil {
				mainFramingError = fmt.Errorf("failed to parse frame at pos %d: %w", frameStartPos, frameErr)
			}

			posAfterError, errSeekPos := inputStream.Pos()
			if errSeekPos != nil {
				k.logger.Errorf("Failed to get stream position after frame parse error (before seek): %v", errSeekPos)
				if mainFramingError == nil {
					mainFramingError = fmt.Errorf("stream error (pos after frame_err): %w", errSeekPos)
				}
				k.mErrorsTotal.Incr(1)
				break
			}

			if posAfterError < streamSize {
				_, errSeek := inputStream.Seek(posAfterError, 1)
				if errSeek != nil {
					k.logger.Errorf("Failed to seek stream after frame parse error: %v", errSeek)
					if mainFramingError == nil {
						mainFramingError = fmt.Errorf("stream error (seek after frame_err): %w", errSeek)
					}
					k.mErrorsTotal.Incr(1)
					break
				}
				newPosLog, _ := inputStream.Pos() // For logging, best effort
				k.logger.With("new_pos", newPosLog).Debugf("Advanced stream by 1 byte due to frame parse error")
				continue
			}
			break // Break if at EOF after error
		}

		// Successfully parsed a frame container
		posAfterFrameParse, errPosSuccess := inputStream.Pos()
		if errPosSuccess != nil {
			k.logger.Errorf("Failed to get stream position after successful frame parse: %v", errPosSuccess)
			mainFramingError = fmt.Errorf("stream error (pos after frame success): %w", errPosSuccess)
			k.mErrorsTotal.Incr(1)
			break
		}
		k.logger.With("framing_type", frameContainerPd.Type, "stream_pos_after_frame_parse", posAfterFrameParse).Debugf("Frame container parsed successfully")
		frameContainerMap := kst.ParsedDataToMap(frameContainerPd)

		// Ensure frameContainerMap is a map before accessing
		frameMap, ok := frameContainerMap.(map[string]any)
		if !ok {
			k.logger.With("frame_type", frameContainerPd.Type, "parsed_type", fmt.Sprintf("%T", frameContainerMap)).Errorf("Parsed frame container is not a map")
			k.mFrameParsingErrors.Incr(1) // Error in frame structure/contract
			k.mErrorsTotal.Incr(1)
			mainFramingError = fmt.Errorf("parsed frame container is not a map, got %T", frameContainerMap)
			break // Cannot proceed if the frame structure is unexpected
		}

		// Use original field name to match the map keys from ParsedDataToMap (now preserves snake_case)
		payloadRaw, found := frameMap[k.config.FramingDataFieldID]
		k.mFramesProcessed.Incr(1)
		if !found {
			k.logger.With("field_id", k.config.FramingDataFieldID, "frame_type", frameContainerPd.Type).Errorf("Framing data field ID not found in parsed frame")
			k.mFrameParsingErrors.Incr(1) // Error in frame structure/contract
			k.mErrorsTotal.Incr(1)
			mainFramingError = fmt.Errorf("framing data field '%s' not found", k.config.FramingDataFieldID)
			// Attempt to advance stream if stuck
			stuckPosCheck, errStuck := inputStream.Pos()
			if errStuck != nil {
				k.logger.Errorf("Stream error (pos for stuck check): %v", errStuck)
				k.mErrorsTotal.Incr(1)
				break
			}
			if stuckPosCheck <= frameStartPos && stuckPosCheck < streamSize {
				if _, errSeek := inputStream.Seek(stuckPosCheck, 1); errSeek != nil {
					k.logger.Errorf("Stream error (seek for stuck check): %v", errSeek)
					k.mErrorsTotal.Incr(1)
					break
				}
				continue
			} else if stuckPosCheck <= frameStartPos {
				break // Stuck
			}
			continue // Or break, depending on desired strictness
		}

		payloadBytes, ok := payloadRaw.([]byte)
		if !ok {
			// Attempt to convert if it's a KaitaiCEL type holding bytes

			if kcBytes, isKcBytes := payloadRaw.(kcel.KaitaiBytes); isKcBytes {
				payloadBytes = kcBytes.RawBytes()
			} else if kcString, isKcString := payloadRaw.(kcel.KaitaiString); isKcString { // if data field is a string
				payloadBytes = []byte(kcString.Value().(string))
			} else {
				k.logger.With("field_id", k.config.FramingDataFieldID, "type", fmt.Sprintf("%T", payloadRaw), "frame_type", frameContainerPd.Type).Errorf("Framing data field is not []byte or kaitaicel.KaitaiBytes/KaitaiString")
				k.mFrameParsingErrors.Incr(1) // Error in frame structure/contract
				k.mErrorsTotal.Incr(1)
				if mainFramingError == nil {
					mainFramingError = fmt.Errorf("framing data field '%s' is not bytes (type: %T)", k.config.FramingDataFieldID, payloadRaw)
				}
				stuckPosCheck, errStuck := inputStream.Pos()
				if errStuck != nil {
					k.logger.Errorf("Stream error (pos for stuck check type): %v", errStuck)
					k.mErrorsTotal.Incr(1)
					break
				}
				if stuckPosCheck <= frameStartPos && stuckPosCheck < streamSize {
					if _, errSeek := inputStream.Seek(stuckPosCheck, 1); errSeek != nil {
						k.logger.Errorf("Stream error (seek for stuck check type): %v", errSeek)
						k.mErrorsTotal.Incr(1)
						break
					}
					continue
				} else if stuckPosCheck <= frameStartPos {
					break
				}
				continue
			}
		}
		k.logger.With("payload_len", len(payloadBytes)).Debugf("Payload extracted for data parsing")

		// Always create a message, even for empty payloads
		if len(payloadBytes) > 0 {
			payloadStream := kaitai.NewStream(bytes.NewReader(payloadBytes))
			// Ensure RootType is set for the data schema if specified in config
			currentDataSchema := *dataSchema // Create a copy
			effectiveDataRootType := k.config.RootType
			if effectiveDataRootType == "" && currentDataSchema.Meta.ID != "" {
				effectiveDataRootType = currentDataSchema.Meta.ID
			}
			if k.config.RootType != "" { // Use configured if present
				currentDataSchema.RootType = k.config.RootType
			}
			k.logger.With("path", k.config.SchemaPath, "root_type", effectiveDataRootType).Debugf("Using data schema for payload")

			dataInterpreter, err := kst.NewKaitaiInterpreter(&currentDataSchema, dataInterpreterSlog)
			if err != nil { // Handle error from NewKaitaiInterpreter
				k.logger.Errorf("Failed to create data interpreter for payload: %v", err)
				k.mPayloadParsingErrors.Incr(1)
				k.mErrorsTotal.Incr(1)
				// Create a new message to carry the error for this specific frame's payload
				errorMsg := service.NewMessage(nil)
				errorMsg.SetError(fmt.Errorf("failed to create data interpreter for payload: %w", err))
				outputMessages = append(outputMessages, errorMsg)
				mainFramingError = fmt.Errorf("failed to create data interpreter: %w", err)
				k.mErrorsTotal.Incr(1)
				break
			}

			parsedPayloadPd, payloadErr := dataInterpreter.Parse(ctx, payloadStream)
			newMsg := service.NewMessage(nil)

			if payloadErr != nil {
				k.logger.With("frame_start_pos", frameStartPos).Errorf("Error parsing payload data for frame: %v", payloadErr)
				k.mPayloadParsingErrors.Incr(1)
				k.mErrorsTotal.Incr(1)
				newMsg.SetError(fmt.Errorf("failed to parse payload: %w", payloadErr))
			}

			if parsedPayloadPd != nil {
				k.mBytesProcessed.Incr(int64(len(payloadBytes)))                                 // Count successfully processed payload bytes
				k.mFrameProcDuration.Timing(SystemTime.Since(frameParseStartTime).Nanoseconds()) // Timing for the entire frame processing (framing + payload)
				resultMap := kst.ParsedDataToMap(parsedPayloadPd)
				newMsg.SetStructured(resultMap)
			} else if payloadErr == nil {
				newMsg.SetStructured(map[string]any{})
			}

			msg.MetaWalk(func(key, value string) error {
				newMsg.MetaSet(key, value)
				return nil
			})
			// Set plugin-specific metadata
			newMsg.MetaSet("kaitai_schema_path", k.config.SchemaPath)
			if effectiveDataRootType == "" && dataSchema != nil { // Recalculate effectiveDataRootType if it was empty and schema is available
				effectiveDataRootType = dataSchema.Meta.ID
			}
			newMsg.MetaSet("kaitai_root_type", effectiveDataRootType)
			newMsg.MetaSet("kaitai_frame_schema_path", k.config.FramingSchemaPath)
			if effectiveFramingRootType == "" && framingSchema != nil { // Recalculate effectiveFramingRootType
				effectiveFramingRootType = framingSchema.Meta.ID
			}
			newMsg.MetaSet("kaitai_frame_root_type", effectiveFramingRootType)

			outputMessages = append(outputMessages, newMsg)
			k.mParsedTotal.Incr(1)
			k.logger.With("frame_start_pos", frameStartPos).Debugf("Frame payload successfully parsed and message created")
		} else { // payloadBytes is empty
			k.logger.Debugf("Extracted empty payload, creating empty message for this frame")

			// Create a message with empty structure for zero-length payloads
			newMsg := service.NewMessage(nil)
			newMsg.SetStructured(map[string]any{})

			// Copy metadata from original message
			msg.MetaWalk(func(key, value string) error {
				newMsg.MetaSet(key, value)
				return nil
			})

			// Set plugin-specific metadata
			newMsg.MetaSet("kaitai_schema_path", k.config.SchemaPath)
			effectiveDataRootType := k.config.RootType
			if effectiveDataRootType == "" && dataSchema != nil {
				effectiveDataRootType = dataSchema.Meta.ID
			}
			newMsg.MetaSet("kaitai_root_type", effectiveDataRootType)
			newMsg.MetaSet("kaitai_frame_schema_path", k.config.FramingSchemaPath)
			effectiveFramingRootType := k.config.FramingRootType
			if effectiveFramingRootType == "" && framingSchema != nil {
				effectiveFramingRootType = framingSchema.Meta.ID
			}
			newMsg.MetaSet("kaitai_frame_root_type", effectiveFramingRootType)

			outputMessages = append(outputMessages, newMsg)
			k.mParsedTotal.Incr(1)
			k.logger.With("frame_start_pos", frameStartPos).Debugf("Empty payload message created")

			// Time frame container processing (we're here because frame parsing was successful)
			k.mFrameProcDuration.Timing(SystemTime.Since(frameParseStartTime).Nanoseconds())
		}

		// Infinite loop prevention
		posAfterLoopIteration, errInfLoop := inputStream.Pos()
		if errInfLoop != nil {
			k.logger.Errorf("Failed to get stream position for infinite loop check: %v", errInfLoop)
			if mainFramingError == nil {
				mainFramingError = fmt.Errorf("stream error (pos for infinite loop check): %w", errInfLoop)
			}
			k.mErrorsTotal.Incr(1)
			break
		}

		if posAfterLoopIteration <= frameStartPos {
			if posAfterLoopIteration < streamSize {
				k.logger.With("stream_pos", posAfterLoopIteration+1).Warnf("Advanced stream by 1 byte due to lack of progress in frame parsing")
				_, errSeekInf := inputStream.Seek(posAfterLoopIteration, 1)
				if errSeekInf != nil {
					k.logger.Errorf("Failed to seek stream during infinite loop prevention: %v", errSeekInf)
					if mainFramingError == nil {
						mainFramingError = fmt.Errorf("stream error (seek for infinite loop prevention): %w", errSeekInf)
					}
					k.mErrorsTotal.Incr(1)
					break
				}
			} else {
				k.logger.With("pos", posAfterLoopIteration).Debugf("Stream position did not advance but EOF reached")
				break
			}
		}
	}

	if len(outputMessages) == 0 && mainFramingError != nil {
		// If no messages were successfully processed and a framing error occurred,
		// propagate the error on the original message.
		k.logger.Errorf("No frames processed due to framing error: %v", mainFramingError)
		msg.SetError(mainFramingError) // Attach error to original message
		return service.MessageBatch{msg}, nil
	}

	finalPos, errLogPos := inputStream.Pos()
	bytesRemainingAfterLoop := int64(0)
	if errLogPos == nil {
		bytesRemainingAfterLoop = streamSize - finalPos
	} else {
		k.logger.Warnf("Could not get final stream position for logging: %v", errLogPos)
		// bytesRemainingAfterLoop remains 0 or could be set to -1 to indicate error
	}
	k.logger.With("frames_extracted", len(outputMessages), "bytes_remaining_in_msg_buffer", bytesRemainingAfterLoop).Debugf("Finished attempting to extract frames from message")

	// Re-define or ensure isEofError is accessible here if it was a very local closure.
	// For simplicity, assuming it's defined in a scope accessible here or re-declared if necessary.
	isEofErrorCheckForFinal := func(e error) bool { // Duplicating for clarity if scope was an issue.
		if e == nil {
			return false
		}
		_, isKaitaiCustomEof := e.(kaitai.EndOfStreamError)
		return isKaitaiCustomEof || errors.Is(e, io.EOF) || errors.Is(e, io.ErrUnexpectedEOF)
	}
	if mainFramingError != nil && isEofErrorCheckForFinal(mainFramingError) && len(outputMessages) > 0 {
		k.logger.Infof("Finished processing message with some frames parsed and a partial frame at the end (EOF).")
		return outputMessages, nil
	}

	return outputMessages, nil
}

// parseBinary parses binary data into a JSON structure using Kaitai Struct.
func (k *KaitaiProcessor) parseBinary(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	k.logger.Debugf("Parsing non-framed binary data with Kaitai Struct")

	// Get binary data from message
	binData, err := msg.AsBytes()
	if err != nil {
		k.logger.Errorf("Failed to get binary data from message for non-framed parsing: %v", err)
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to get binary data from message: %w", err))
		return service.MessageBatch{msg}, nil
	}

	if len(binData) == 0 {
		k.logger.Warnf("Empty binary data provided for non-framed parsing")
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("empty binary data provided"))
		return service.MessageBatch{msg}, nil
	}
	startTime := SystemTime.Now()

	// Load schema
	schema, err := k.loadDataSchema(k.config.SchemaPath)
	if err != nil {
		// Error logged by loadDataSchema
		k.mErrorsTotal.Incr(1) // General error as it's schema loading
		msg.SetError(fmt.Errorf("failed to load schema: %w", err))
		return service.MessageBatch{msg}, nil
	}

	// Create Kaitai stream for parsing
	stream := kaitai.NewStream(bytes.NewReader(binData))

	// Create interpreter and parse data
	// Create slog.Logger instance using Benthos logger
	interpreterSlog := slog.New(newBenthosLogHandler(k.logger)).With("component", "data_interpreter")
	interpreter, err := kst.NewKaitaiInterpreter(schema, interpreterSlog)
	if err != nil {
		k.logger.Errorf("Failed to create Kaitai interpreter for single parse: %v", err)
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to create interpreter: %w", err))
		return service.MessageBatch{msg}, nil
	}

	parsedDataPd, parseErr := interpreter.Parse(ctx, stream) // Use ctx from function signature
	if parseErr != nil {
		k.logger.With("data_size", len(binData)).Errorf("Failed to parse non-framed binary data: %v", err)
		k.mPayloadParsingErrors.Incr(1) // Specific error for payload parsing
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to parse binary data (size: %d bytes): %w", len(binData), err))
		return service.MessageBatch{msg}, nil
	}
	k.mBytesProcessed.Incr(int64(len(binData)))
	k.mFrameProcDuration.Timing(SystemTime.Since(startTime).Nanoseconds()) // Treat as one "frame"

	// Convert parsed data to a map/JSON structure
	result := kst.ParsedDataToMap(parsedDataPd)

	k.logger.With("data_size", len(binData)).Debugf("Successfully parsed non-framed binary data")
	k.mParsedTotal.Incr(1)

	// Create new message with parsed data
	newMsg := service.NewMessage(nil)
	newMsg.SetStructured(result)

	// Copy metadata from original message
	msg.MetaWalk(func(key, value string) error {
		newMsg.MetaSet(key, value)
		return nil
	})

	// Set plugin-specific metadata
	newMsg.MetaSet("kaitai_schema_path", k.config.SchemaPath)
	effectiveDataRootType := k.config.RootType
	if effectiveDataRootType == "" && schema != nil { // schema is the loaded dataSchema in this function
		effectiveDataRootType = schema.Meta.ID
	}
	newMsg.MetaSet("kaitai_root_type", effectiveDataRootType)

	return service.MessageBatch{newMsg}, nil
}

// serializeToBinary serializes a JSON structure to binary using Kaitai Struct.
func (k *KaitaiProcessor) serializeToBinary(ctx context.Context, msg *service.Message) (service.MessageBatch, error) {
	k.logger.Debugf("Serializing structured data to binary with Kaitai Struct")

	// Get structured data from message
	structData, err := msg.AsStructured()
	if err != nil {
		k.logger.Errorf("Failed to get structured data from message: %v", err)
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to get structured data from message: %w", err))
		return service.MessageBatch{msg}, nil
	}

	dataMap, ok := structData.(map[string]any)
	if !ok {
		// Log that the data is not the expected map type before returning an error.
		k.logger.With("data_type", fmt.Sprintf("%T", structData)).Errorf("Structured data for serialization is not a map[string]any, got %T", structData)
		k.mErrorsTotal.Incr(1) // Count as a general error as it's an input format issue for serialization
		msg.SetError(fmt.Errorf("structured data for serialization is not a map[string]any, got %T", structData))
		return service.MessageBatch{msg}, fmt.Errorf("structured data for serialization is not a map[string]any, got %T", structData)
	}
	k.logger.With("data_type", fmt.Sprintf("%T", structData), "num_keys", len(dataMap)).Debugf("Input data for serialization")

	// Load schema
	schema, err := k.loadDataSchema(k.config.SchemaPath)
	if err != nil {
		// Error logged by loadDataSchema
		k.mErrorsTotal.Incr(1) // General error as it's schema loading
		msg.SetError(fmt.Errorf("failed to load schema: %w", err))
		return service.MessageBatch{msg}, nil
	}

	// Create serializer and serialize data
	// Create slog.Logger instance using Benthos logger
	serializerSlog := slog.New(newBenthosLogHandler(k.logger)).With("component", "data_serializer")
	serializer, err := kst.NewKaitaiSerializer(schema, serializerSlog)
	if err != nil {
		k.logger.Errorf("Failed to create Kaitai serializer: %v", err)
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to create serializer: %w", err))
		return service.MessageBatch{msg}, nil
	}

	// Ensure structData is map[string]any for serialization
	dataMap, ok = structData.(map[string]any)
	if !ok {
		return service.MessageBatch{msg}, fmt.Errorf("structured data is not a map[string]any, got %T", structData)
	}
	binData, err := serializer.Serialize(ctx, dataMap) // Use ctx from function signature
	if err != nil {
		k.logger.Errorf("Failed to serialize data: %v", err)
		k.mPayloadSerializationErrors.Incr(1) // Specific error for payload serialization
		k.mErrorsTotal.Incr(1)
		msg.SetError(fmt.Errorf("failed to serialize data: %w", err))
		return service.MessageBatch{msg}, nil
	}

	k.logger.With("output_size_bytes", len(binData)).Debugf("Successfully serialized data")
	k.mSerializedTotal.Incr(1)

	newMsg := service.NewMessage(binData)

	// Copy metadata from original message
	msg.MetaWalk(func(key, value string) error {
		newMsg.MetaSet(key, value)
		return nil
	})

	return service.MessageBatch{newMsg}, nil
}

// loadDataSchema loads the main data schema using the internal helper.
func (k *KaitaiProcessor) loadDataSchema(path string) (*kst.KaitaiSchema, error) {
	return k.loadSchemaInternal(path, &k.schemaCache, *k.mSchemaCacheHits, *k.mSchemaCacheMisses, "data")
}

// loadFramingSchema loads the framing schema using the internal helper.
func (k *KaitaiProcessor) loadFramingSchema(path string) (*kst.KaitaiSchema, error) {
	return k.loadSchemaInternal(path, &k.framingSchemaCache, *k.mFramingSchemaCacheHits, *k.mFramingSchemaCacheMisses, "framing")
}

// loadSchemaInternal loads and parses a KSY schema file using the specified cache and metrics.
func (k *KaitaiProcessor) loadSchemaInternal(path string, cache *sync.Map, mHits, mMisses service.MetricCounter, schemaType string) (*kst.KaitaiSchema, error) {
	if path == "" {
		return nil, fmt.Errorf("%s schema path is empty", schemaType)
	}
	// Check schema cache first
	if cachedSchema, ok := cache.Load(path); ok {
		k.logger.With("path", path).Tracef("%s schema cache hit", schemaType)
		mHits.Incr(1)
		return cachedSchema.(*kst.KaitaiSchema), nil
	}

	k.logger.With("path", path).Debugf("Loading %s schema from file", schemaType)
	mMisses.Incr(1)

	// Read schema file
	data, err := os.ReadFile(path)
	if err != nil {
		k.logger.With("path", path).Errorf("Failed to read schema file: %v", err)
		return nil, fmt.Errorf("failed to read schema file '%s': %w", path, err)
	}

	// Parse YAML
	schema := &kst.KaitaiSchema{}
	if err := yaml.Unmarshal(data, schema); err != nil {
		k.logger.With("path", path).Errorf("Failed to parse schema YAML: %v", err)
		return nil, fmt.Errorf("failed to parse schema YAML from '%s': %w", path, err)
	}

	// Store in cache
	cache.Store(path, schema)
	k.logger.With("path", path).Debugf("Loaded and cached schema successfully")

	return schema, nil
}

// Close the processor resources
func (k *KaitaiProcessor) Close(ctx context.Context) error {
	k.logger.Debugf("Closing Kaitai processor and clearing schema cache")
	k.schemaCache = sync.Map{}        // Clear the main data schema cache
	k.framingSchemaCache = sync.Map{} // Clear the framing schema cache
	return nil
}

func main() {
	service.RunCLI(context.Background())
}
