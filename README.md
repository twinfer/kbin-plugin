# Kaitai Struct Benthos Plugin

A dynamic interpreter for [Kaitai Struct](https://kaitai.io/) schemas as a [Benthos](https://www.benthos.dev/) plugin, enabling binary parsing and serialization without code generation.

## Overview

This plugin allows Benthos users to process binary data using Kaitai Struct schema definitions (`.ksy` files) without requiring the compilation step or generated code. The plugin dynamically interprets `.ksy` schemas at runtime to parse binary data into JSON or serialize JSON back to binary format.

### Key Features

- **Dynamic Schema Interpretation**: Parse KSY files directly and build parsing/serialization logic at runtime
- **No Code Generation**: Works without requiring the Kaitai Struct compiler or generated code
- **Expression Support**: Full support for Kaitai's expression language including:
  - Bitwise operations: `&`, `|`, `^`, `<<`, `>>`
  - Ternary conditionals: `cond ? val1 : val2`
  - Parent/root references: `_parent.field`, `_root.header.version`
  - Built-in functions: `to_s()`, `length`, `reverse()`
  - Enum handling: `enum_value.to_i()`
- **Seamless Integration**: Works as a standard Benthos processor
- **Binary ↔ JSON Conversion**: Supports both parsing and serialization

## Installation

### Prerequisites

- Go 1.19 or later
- Benthos 4.0 or later

### Building from Source

```bash
# Clone the repository
git clone https://github.com/yourorg/benthos-kaitai-plugin.git
cd benthos-kaitai-plugin

# Build the plugin
go build -o benthos-kaitai ./cmd/benthos-kaitai

# Run Benthos with the plugin
benthos --plugin-dir=. -c your_config.yaml
```

## Usage

### Configuration

Add the `kaitai` processor to your Benthos configuration:

```yaml
pipeline:
  processors:
    - kaitai:
        schema_path: "./schemas/sensor_data.ksy"  # Path to your schema file
        is_parser: true  # Set to true for binary→JSON, false for JSON→binary
        root_type: ""    # Optional, defaults to schema's main type
```

### Example: Parsing Binary Data

```yaml
input:
  file:
    paths: ["./sensor_readings.bin"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "./schemas/sensor_data.ksy"
        is_parser: true

    # Optional: Add other processors to work with the parsed data
    - mapping: |
        root.processed_time = now()
        root.readings = this.readings.map_each(reading -> {
          reading.value_celsius = reading.value
          reading.value_fahrenheit = reading.value * 1.8 + 32
          return reading
        })
        root

output:
  kafka:
    addresses: ["kafka:9092"]
    topic: processed_readings
```

### Example: Serializing JSON to Binary

```yaml
input:
  http_server:
    path: /sensor
    codec: json

pipeline:
  processors:
    - kaitai:
        schema_path: "./schemas/sensor_data.ksy"
        is_parser: false  # Serialize mode

output:
  file:
    path: "./output/data_${!timestamp_unix_nano}.bin"
    codec: raw
```

## Supported Kaitai Features

The plugin supports most Kaitai Struct features:

- ✅ Basic types (integers, floats, strings, bytes)
- ✅ Compound types (structs, arrays)
- ✅ Enums and switch cases
- ✅ Repeat expressions and repeat-until
- ✅ Size-prefixed fields
- ✅ Conditional fields (if expressions)
- ✅ Instance expressions
- ✅ Endianness control
- ✅ Recursion and nested types
- ✅ Process blocks (for common encodings)
- ✅ Fixed contents validation
- ✅ Zero-terminated strings
- ✅ Size-based value reading

## Performance Considerations

- **Schema Caching**: Schemas are parsed once and cached for reuse
- **Expression Compilation**: Expressions are compiled once and reused
- **Memory Usage**: The plugin is designed for efficient memory usage in streaming contexts
- **Error Handling**: Robust error reporting with clear context

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
