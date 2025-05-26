# Benthos Plugin for Kaitai Struct

This Benthos plugin allows you to parse and serialize binary data using Kaitai Struct definitions without requiring pre-compiled parsers. It dynamically interprets `.ksy` schema files to process data streams.

## Features

*   **Dynamic Parsing & Serialization**: Process binary data based on Kaitai Struct (`.ksy`) files at runtime. No code generation step is needed.
*   **Kaitai Struct Compliant**: Aims to support the full Kaitai Struct specification.
*   **Common Expression Language (CEL) Support**: Leverages CEL for evaluating conditional logic, instance fields, and other dynamic aspects defined in your `.ksy` files.
*   **Custom Kaitai Types (`kaitaicel`)**: Implements Kaitai-specific data types within the CEL environment for accurate type handling.
*   **Benthos Integration**: Seamlessly integrates as a Benthos processor for use in data pipelines.

## Installation

To use this plugin, you need to build it and ensure it's available in your Benthos environment.

```shell
# Ensure you have Go installed (version 1.20+)
# Clone the repository (if you haven't already)
# git clone <repository-url>
# cd <repository-directory>

# Build the plugin (this might vary based on your Benthos setup)
# For example, if you are building a custom Benthos binary:
# go build -o benthos_custom cmd/kbin-plugin/main.go

# Alternatively, you might be using Benthos plugin mechanisms that
# involve placing the compiled plugin in a specific directory.
# Please refer to the Benthos documentation for plugin management.
```

*Note: Detailed build and deployment instructions will depend on your specific Benthos setup (custom binary, Docker image, etc.).*

## Configuration

Here's an example of how to configure the `kaitai` processor in your Benthos pipeline configuration:

```yaml
pipeline:
  processors:
    - kaitai:
        schema_path: "./schemas/my_format.ksy" # Path to your Kaitai Struct (.ksy) file
        is_parser: true                        # true for parsing binary to JSON, false for serializing JSON to binary
        root_type: "my_main_type"              # Optional: Specify the root type from the KSY schema. If empty, uses the 'id' from the schema's 'meta' section.
```

**Configuration Options:**

*   `schema_path` (string): **Required.** The file path to your Kaitai Struct definition (`.ksy`) file.
*   `is_parser` (bool): **Optional.** Defaults to `true`.
    *   If `true`, the processor will parse incoming binary messages into structured data (typically JSON).
    *   If `false`, the processor will serialize incoming structured data (JSON) into binary messages.
*   `root_type` (string): **Optional.** The name of the root type within your KSY schema to use for parsing or serialization. If left empty, the plugin will use the main type specified in the `meta.id` field of your KSY file.

## Usage Examples

### Parsing Binary Data to JSON

Suppose you have a `network_packet.ksy` file defining the structure of a network packet and you want to parse raw packet data from a Kafka topic.

```yaml
input:
  kafka:
    addresses: [ "localhost:9092" ]
    topics: [ "raw_packets" ]
    consumer_group: "benthos_consumers"

pipeline:
  processors:
    - kaitai:
        schema_path: "./configs/kaitai/network_packet.ksy"
        is_parser: true
        # root_type: "ethernet_frame" # Optional, if your KSY meta.id is not ethernet_frame

output:
  stdout: {}
```

This configuration will:
1.  Read raw binary messages from the `raw_packets` Kafka topic.
2.  Use the `network_packet.ksy` schema to parse each message.
3.  Output the resulting JSON structure to stdout.

### Serializing JSON Data to Binary

Imagine you have JSON messages representing commands that need to be serialized into a specific binary format defined by `command_protocol.ksy`.

```yaml
input:
  http_server:
    address: "0.0.0.0:4195"
    path: "/submit_command"

pipeline:
  processors:
    - kaitai:
        schema_path: "./configs/kaitai/command_protocol.ksy"
        is_parser: false
        root_type: "command"

output:
  gcp_pubsub:
    project: "my-gcp-project"
    topic: "binary_commands"
```

This configuration will:
1.  Receive JSON messages via an HTTP POST request to `/submit_command`.
2.  Use the `command_protocol.ksy` (with `command` as the root type) to serialize the JSON into binary.
3.  Send the resulting binary message to the `binary_commands` GCP Pub/Sub topic.
