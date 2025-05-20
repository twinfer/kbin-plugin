#!/usr/bin/env bash
# Integration tests for Kaitai Struct Benthos plugin

set -e

echo "Running integration tests..."

# Base directory for test data and results
BASE_DIR="testdata/formats"
RESULTS_DIR="test/results"
mkdir -p "${RESULTS_DIR}"

# Make sure Benthos is available
if ! command -v rpk &> /dev/null; then
    echo "Benthos not found, installing..."
    curl -Lsf https://sh.benthos.dev | bash
fi

# Make sure the plugin is built
if [ ! -f "kbin-plugin" ]; then
    echo "Building plugin..."
    go build -o kbin-plugin ./cmd/kbin-plugin
fi

# Test parsing for each format
for format_dir in ${BASE_DIR}/*; do
    format=$(basename "${format_dir}")
    schema_file="${format_dir}/${format}.ksy"
    
    if [ ! -f "${schema_file}" ]; then
        continue
    fi
    
    echo "Testing ${format}..."
    
    # Find all binary samples
    for sample_file in "${format_dir}"/samples/*; do
        if [[ "${sample_file}" == *.json ]]; then
            continue
        fi
        
        sample_name=$(basename "${sample_file}")
        output_file="${RESULTS_DIR}/${format}_${sample_name}.json"
        
        echo "  Processing ${sample_name}..."
        
        # Create a temporary Benthos config
        config_file="${RESULTS_DIR}/${format}_${sample_name}.yaml"
        cat > "${config_file}" << EOF
input:
  file:
    paths: ["${sample_file}"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "${schema_file}"
        is_parser: true

output:
  file:
    path: "${output_file}"
    codec: json
EOF
        
        # Run Benthos with the config
        rpk --plugin-dir=. -c "${config_file}" &
        BENTHOS_PID=$!
        
        # Wait a bit for processing to complete
        sleep 2
        
        # Kill Benthos
        kill ${BENTHOS_PID} || true
        
        # Check the output file exists
        if [ -f "${output_file}" ]; then
            echo "  ✅ Successfully processed ${sample_name}"
            
            # Test roundtrip (parsing -> serialization -> parsing)
            roundtrip_config="${RESULTS_DIR}/${format}_${sample_name}_roundtrip.yaml"
            roundtrip_bin="${RESULTS_DIR}/${format}_${sample_name}.bin"
            roundtrip_json="${RESULTS_DIR}/${format}_${sample_name}_roundtrip.json"
            
            # Step 1: Serialize JSON to binary
            cat > "${roundtrip_config}" << EOF
input:
  file:
    paths: ["${output_file}"]
    codec: json

pipeline:
  processors:
    - kaitai:
        schema_path: "${schema_file}"
        is_parser: false

output:
  file:
    path: "${roundtrip_bin}"
    codec: raw
EOF
            
            rpk --plugin-dir=. -c "${roundtrip_config}" &
            BENTHOS_PID=$!
            sleep 2
            kill ${BENTHOS_PID} || true
            
            # Step 2: Parse binary back to JSON
            if [ -f "${roundtrip_bin}" ]; then
                cat > "${roundtrip_config}" << EOF
input:
  file:
    paths: ["${roundtrip_bin}"]
    codec: raw

pipeline:
  processors:
    - kaitai:
        schema_path: "${schema_file}"
        is_parser: true

output:
  file:
    path: "${roundtrip_json}"
    codec: json
EOF
                
                rpk --plugin-dir=. -c "${roundtrip_config}" &
                BENTHOS_PID=$!
                sleep 2
                kill ${BENTHOS_PID} || true
                
                if [ -f "${roundtrip_json}" ]; then
                    echo "  ✅ Successful roundtrip test for ${sample_name}"
                else
                    echo "  ❌ Failed to complete roundtrip test (parse step) for ${sample_name}"
                fi
            else
                echo "  ❌ Failed to complete roundtrip test (serialize step) for ${sample_name}"
            fi
        else
            echo "  ❌ Failed to process ${sample_name}"
        fi
    done
done

echo "Integration tests completed!"
