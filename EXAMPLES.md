# Examples and Usage Patterns

This document provides practical examples of how to use the Kaitai Struct Benthos plugin in various scenarios.

## Basic Usage

### Parsing a Binary File

```yaml
# config.yaml
input:
  file:
    paths: ["/path/to/data/*.bin"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "/path/to/schemas/my_format.ksy"
        is_parser: true

output:
  file:
    path: "/path/to/output/data_${!count("files")}.json"
    codec: json
```

Run:
```bash
benthos --plugin-dir=. -c config.yaml
```

### Serializing JSON to Binary

```yaml
# serialize_config.yaml
input:
  file:
    paths: ["/path/to/data/*.json"]
    codec: json

pipeline:
  processors:
    - kaitai:
        schema_path: "/path/to/schemas/my_format.ksy"
        is_parser: false

output:
  file:
    path: "/path/to/output/data_${!count("files")}.bin"
    codec: raw
```

Run:
```bash
benthos --plugin-dir=. -c serialize_config.yaml
```

## Advanced Examples

### Processing Network Packets

This example shows how to use the plugin to parse and analyze network packet captures.

```yaml
# network_packets.yaml
input:
  file:
    paths: ["/path/to/packets/*.pcap"]
    codec: raw

pipeline:
  processors:
    # First parse the PCAP container
    - kaitai:
        schema_path: "/path/to/schemas/pcap.ksy"
        is_parser: true
    
    # Extract packet payloads and process each one
    - mapping: |
        root = this
        # Extract TCP packets only
        root.tcp_packets = this.packets.filter(p -> 
          p.network.protocol == 6
        ).map_each(p -> {
          packet = p
          # Add metadata
          packet.timestamp = p.timestamp
          packet.src_ip = p.network.src_ip
          packet.dst_ip = p.network.dst_ip
          packet.src_port = p.transport.src_port
          packet.dst_port = p.transport.dst_port
          packet.payload = p.transport.payload
          return packet
        })
        root

    # Convert packets to array for batch processing
    - split:
        items_path: tcp_packets
    
    # Parse HTTP if port matches
    - branch:
        processors:
          - kaitai:
              schema_path: "/path/to/schemas/http_request.ksy"
              is_parser: true
        condition:
          jmespath: "dst_port == 80 || dst_port == 443"

    # Parse DNS if port matches
    - branch:
        processors:
          - kaitai:
              schema_path: "/path/to/schemas/dns_packet.ksy"
              is_parser: true
        condition:
          jmespath: "dst_port == 53"

output:
  file:
    path: "/path/to/output/packet_${!count("files")}.json"
    codec: json
```

### IoT Sensor Data Pipeline

This example demonstrates processing IoT sensor data with the plugin.

```yaml
# iot_pipeline.yaml
input:
  mqtt:
    urls: ["tcp://broker.example.com:1883"]
    topics: ["sensors/+/data"]
    qos: 1
    client_id: benthos_processor

pipeline:
  processors:
    # Parse binary sensor data
    - kaitai:
        schema_path: "/path/to/schemas/sensor_data.ksy"
        is_parser: true
    
    # Add metadata and process readings
    - mapping: |
        root = this
        root.processed_timestamp = now()
        root.device_id = this.header.device_id
        
        # Process temperature readings
        root.temperature_readings = this.readings
          .filter(r -> r.sensor_id == 0)
          .map_each(r -> {
            reading = r
            reading.value_fahrenheit = r.value * 1.8 + 32
            reading.is_warning = r.value > 30 # High temperature warning
            return reading
          })
        
        # Process humidity readings  
        root.humidity_readings = this.readings
          .filter(r -> r.sensor_id == 1)
          .map_each(r -> {
            reading = r
            reading.is_warning = r.value > 80 # High humidity warning
            return reading
          })
        
        # Calculate averages
        if root.temperature_readings.length() > 0 {
          temp_values = root.temperature_readings.map_each(r -> r.value)
          root.avg_temperature = temp_values.sum() / temp_values.length()
        }
        
        if root.humidity_readings.length() > 0 {
          humid_values = root.humidity_readings.map_each(r -> r.value)
          root.avg_humidity = humid_values.sum() / humid_values.length()
        }
        
        # Set alert flag
        root.has_warnings = this.readings.exists(r -> r.is_warning)
        
        root

    # Route warnings to a different output
    - switch:
        - output_index: 1
          condition:
            jmespath: "has_warnings == true"

output:
  - elasticsearch:
      urls: ["http://localhost:9200"]
      index: sensors
      id: ${! this.device_id + "_" + timestamp_unix() }
  
  # Warning alerts go to a different destination
  - kafka:
      addresses: ["kafka:9092"]
      topic: sensor_warnings
```

### File Format Converter

This example shows how to use the plugin to convert between binary formats.

```yaml
# format_converter.yaml
input:
  stdin:
    codec: raw

pipeline:
  processors:
    # Parse the source format
    - kaitai:
        schema_path: "${INPUT_FORMAT_SCHEMA}"
        is_parser: true
    
    # Transform data if needed
    - mapping: |
        root = this
        # Map fields from source to target format
        target = {}
        
        # Image conversion example (PNG to BMP)
        target.header = {
          "signature": "BM",
          "file_size": this.len_file + 54,  # BMP header size + data
          "reserved": 0,
          "data_offset": 54
        }
        
        target.dib_header = {
          "header_size": 40,
          "width": this.ihdr.width,
          "height": this.ihdr.height,
          "planes": 1,
          "bits_per_pixel": 24,
          "compression": 0,
          "image_size": this.ihdr.width * this.ihdr.height * 3,
          "x_pixels_per_meter": 2835,
          "y_pixels_per_meter": 2835,
          "colors_used": 0,
          "important_colors": 0
        }
        
        # Convert pixel data (simplified)
        target.pixel_data = this.idat.pixel_data
        
        root = target
    
    # Serialize to the target format
    - kaitai:
        schema_path: "${OUTPUT_FORMAT_SCHEMA}"
        is_parser: false

output:
  stdout:
    codec: raw
```

Run:
```bash
INPUT_FORMAT_SCHEMA=/path/to/schemas/png.ksy OUTPUT_FORMAT_SCHEMA=/path/to/schemas/bmp.ksy \
benthos --plugin-dir=. -c format_converter.yaml < input.png > output.bmp
```

## Integration Patterns

### Kafka Stream Processing

This example shows how to use the plugin in a Kafka-based streaming pipeline.

```yaml
# kafka_stream.yaml
input:
  kafka:
    addresses: ["kafka:9092"]
    topics: ["binary_data"]
    consumer_group: "kaitai_processors"
    batching:
      count: 100
      period: 1s

pipeline:
  processors:
    # Parse binary messages
    - kaitai:
        schema_path: "/path/to/schemas/message_format.ksy"
        is_parser: true
    
    # Process and transform messages
    - mapping: |
        root = this
        # Transformation logic...
        root
    
    # Route to different outputs based on message type
    - switch:
        - output_index: 0
          condition:
            jmespath: "header.type == 'A'"
        - output_index: 1
          condition:
            jmespath: "header.type == 'B'"
        - output_index: 2  # Default

output:
  - kafka:
      addresses: ["kafka:9092"]
      topic: processed_type_a
  - kafka:
      addresses: ["kafka:9092"]
      topic: processed_type_b
  - kafka:
      addresses: ["kafka:9092"]
      topic: processed_other
```

### HTTP API with Binary Request/Response

This example shows how to use the plugin to create an HTTP API that accepts and returns binary data.

```yaml
# http_api.yaml
input:
  http_server:
    path: /api/binary
    allowed_methods: [POST]
    timeout: 5s

pipeline:
  processors:
    # Parse the binary request
    - kaitai:
        schema_path: "/path/to/schemas/request_format.ksy"
        is_parser: true
    
    # Process the request
    - mapping: |
        root = this
        
        # Extract request parameters
        command = this.header.command
        parameters = this.payload.parameters
        
        # Process command
        response = {}
        response.header = {
          "version": 1,
          "request_id": this.header.request_id,
          "timestamp": timestamp_unix()
        }
        
        # Handle different commands
        if command == "get_status" {
          response.payload = {
            "status": "ok",
            "system_time": timestamp_unix(),
            "values": [1, 2, 3, 4]
          }
        } else if command == "set_config" {
          # Process configuration change
          response.payload = {
            "status": "ok",
            "config_applied": true
          }
        } else {
          response.payload = {
            "status": "error",
            "error_code": 1,
            "message": "Unknown command"
          }
        }
        
        root = response
    
    # Serialize the response back to binary
    - kaitai:
        schema_path: "/path/to/schemas/response_format.ksy"
        is_parser: false

output:
  http_server:
    codec: raw
```

## Custom Processing Examples

### Parsing with Expressions

This example demonstrates using expressions in your Kaitai schema and how they're handled by the plugin.

```yaml
# Example schema with expressions
meta:
  id: custom_format
  endian: le

seq:
  - id: header_size
    type: u2
  - id: flags
    type: u2
  - id: header
    size: header_size - 4  # Expression: subtract the size of header_size and flags
  - id: num_entries
    type: u4
  - id: entries
    type: entry
    repeat: expr
    repeat-expr: num_entries
  - id: has_footer
    type: u1
  - id: footer
    type: footer
    if: has_footer != 0  # Conditional field

types:
  entry:
    seq:
      - id: key_length
        type: u2
      - id: key
        type: str
        size: key_length
        encoding: UTF-8
      - id: value_length
        type: u2
      - id: value
        type: str
        size: value_length
        encoding: UTF-8
    instances:
      key_value_pair:  # Calculated instance
        value: key + ": " + value

  footer:
    seq:
      - id: checksum
        type: u4
      - id: reserved
        size: 8
    instances:
      is_valid:  # Boolean instance using bitwise operation
        value: (checksum & 0xF0F0F0F0) == 0x10101010
```

Benthos configuration to use this schema:

```yaml
# expressions_example.yaml
input:
  file:
    paths: ["/path/to/data/*.bin"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "/path/to/schemas/custom_format.ksy"
        is_parser: true
    
    # Use the parsed data
    - mapping: |
        root = this
        
        # Access fields parsed with expressions
        header_data = this.header
        
        # Access the entry array created by repeat-expr
        all_entries = this.entries
        
        # Access calculated instances
        key_value_pairs = this.entries.map_each(e -> e.key_value_pair)
        
        # Use conditional fields
        if this.has_footer != 0 {
          is_valid = this.footer.is_valid
          root.checksum_valid = is_valid
        }
        
        root

output:
  stdout:
    codec: json
```

### Custom Processing Extensions

This example shows how to use the plugin with custom processing types.

```yaml
# Example schema with process attribute
meta:
  id: encrypted_data
  endian: be

seq:
  - id: magic
    contents: [0xED, 0xCF]
  - id: version
    type: u1
  - id: key_id
    type: u2
  - id: iv
    size: 16
  - id: encrypted_size
    type: u4
  - id: encrypted_data
    size: encrypted_size
    process: xor(key_id)  # Apply XOR decoding with key_id as the key
  - id: checksum
    type: u4
```

The plugin implements process handlers for:
- `xor` - XOR decoding with a key
- `zlib` - Zlib decompression
- `rol` / `ror` - Rotate bits left/right

Benthos configuration:

```yaml
# process_example.yaml
input:
  file:
    paths: ["/path/to/encrypted_data.bin"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "/path/to/schemas/encrypted_data.ksy"
        is_parser: true
    
    # Access the decrypted data
    - mapping: |
        root = this
        
        # The plugin automatically handled the XOR decryption process
        decrypted_json = json_from_bytes(this.encrypted_data)
        root.decrypted_content = decrypted_json
        
        root

output:
  stdout:
    codec: json
```

### Using Enums and Switch Types

This example demonstrates using enums and switch types in your Kaitai schema.

```yaml
# Example schema with enums and switch
meta:
  id: message_format
  endian: le

seq:
  - id: message_type
    type: u1
    enum: message_types
  - id: length
    type: u2
  - id: body
    type: message_body
    size: length

types:
  message_body:
    seq:
      - id: data
        type:
          switch-on: _parent.message_type
          cases:
            'message_types::text': text_message
            'message_types::image': image_message
            'message_types::audio': audio_message
            _: unknown_message

  text_message:
    seq:
      - id: encoding
        type: u1
        enum: text_encodings
      - id: text
        type: str
        size-eos: true
        encoding: UTF-8

  image_message:
    seq:
      - id: image_type
        type: u1
        enum: image_types
      - id: width
        type: u2
      - id: height
        type: u2
      - id: image_data
        size-eos: true

  audio_message:
    seq:
      - id: sample_rate
        type: u4
      - id: channels
        type: u1
      - id: audio_data
        size-eos: true

  unknown_message:
    seq:
      - id: raw_data
        size-eos: true

enums:
  message_types:
    0: text
    1: image
    2: audio
    
  text_encodings:
    0: ascii
    1: utf8
    2: utf16

  image_types:
    0: png
    1: jpeg
    2: gif
```

Benthos configuration:

```yaml
# enum_switch_example.yaml
input:
  file:
    paths: ["/path/to/messages/*.bin"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "/path/to/schemas/message_format.ksy"
        is_parser: true
    
    # Process based on message type
    - mapping: |
        root = this
        
        message_type = this.message_type.to_s()
        root.message_type_name = message_type
        
        if message_type == "text" {
          encoding = this.body.data.encoding.to_s()
          root.text_content = this.body.data.text
          root.encoding_name = encoding
        } else if message_type == "image" {
          image_type = this.body.data.image_type.to_s()
          root.image = {
            "type": image_type,
            "width": this.body.data.width,
            "height": this.body.data.height,
            "size": this.body.data.image_data.length()
          }
        } else if message_type == "audio" {
          root.audio = {
            "sample_rate": this.body.data.sample_rate,
            "channels": this.body.data.channels,
            "duration_ms": this.body.data.audio_data.length() * 1000 / 
                           (this.body.data.sample_rate * this.body.data.channels * 2)
          }
        }
        
        root

output:
  elasticsearch:
    urls: ["http://localhost:9200"]
    index: messages
    id: ${! uuid_v4() }
```

These examples demonstrate the flexibility and power of the Kaitai Struct Benthos plugin in various real-world scenarios.
