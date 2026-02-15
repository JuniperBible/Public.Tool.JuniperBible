#!/bin/bash
# Parity test harness for SDK migration
# Compares golden test outputs against current plugin behavior

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TESTDATA_DIR="$PROJECT_ROOT/plugins/ipc/testdata"
TEMP_DIR=$(mktemp -d)

trap "rm -rf $TEMP_DIR" EXIT

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

passed=0
failed=0
skipped=0

log_pass() {
    echo -e "${GREEN}PASS${NC}: $1"
    ((passed++))
}

log_fail() {
    echo -e "${RED}FAIL${NC}: $1"
    ((failed++))
}

log_skip() {
    echo -e "${YELLOW}SKIP${NC}: $1"
    ((skipped++))
}

# Normalize JSON for comparison
# Removes timestamps, UUIDs, temp paths, and sorts keys
normalize_json() {
    local file="$1"
    if command -v jq &> /dev/null; then
        jq -S '
            walk(
                if type == "string" then
                    # Replace timestamps
                    gsub("[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})"; "<TIMESTAMP>") |
                    # Replace UUIDs
                    gsub("[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"; "<UUID>") |
                    # Replace temp paths
                    gsub("/tmp/[^\"]+"; "<TEMP_PATH>") |
                    gsub("/var/folders/[^\"]+"; "<TEMP_PATH>")
                else
                    .
                end
            )
        ' "$file" 2>/dev/null || cat "$file"
    else
        cat "$file"
    fi
}

# Compare two JSON files after normalization
compare_json() {
    local expected="$1"
    local actual="$2"

    local norm_expected=$(normalize_json "$expected")
    local norm_actual=$(normalize_json "$actual")

    if [ "$norm_expected" = "$norm_actual" ]; then
        return 0
    else
        echo "Expected:"
        echo "$norm_expected" | head -20
        echo "---"
        echo "Actual:"
        echo "$norm_actual" | head -20
        return 1
    fi
}

# Test a plugin command
test_plugin() {
    local plugin_binary="$1"
    local request_file="$2"
    local expected_file="$3"
    local test_name="$4"

    if [ ! -x "$plugin_binary" ]; then
        log_skip "$test_name (binary not found: $plugin_binary)"
        return
    fi

    if [ ! -f "$request_file" ]; then
        log_skip "$test_name (request file not found: $request_file)"
        return
    fi

    if [ ! -f "$expected_file" ]; then
        log_skip "$test_name (expected file not found: $expected_file)"
        return
    fi

    local actual_file="$TEMP_DIR/actual.json"

    # Run plugin with request
    cat "$request_file" | "$plugin_binary" > "$actual_file" 2>/dev/null

    if compare_json "$expected_file" "$actual_file"; then
        log_pass "$test_name"
    else
        log_fail "$test_name"
    fi
}

# Run golden tests for a specific plugin
run_golden_tests() {
    local plugin_name="$1"
    local plugin_binary="$PROJECT_ROOT/plugins/format-$plugin_name/format-$plugin_name"

    echo ""
    echo "Testing plugin: $plugin_name"
    echo "----------------------------------------"

    # Test detect command
    if [ -f "$TESTDATA_DIR/detect_request.json" ]; then
        test_plugin "$plugin_binary" \
            "$TESTDATA_DIR/detect_request.json" \
            "$TESTDATA_DIR/detect_response_true.json" \
            "$plugin_name:detect"
    fi

    # Add more command tests as golden files are created
}

# Run all parity tests
run_all_tests() {
    echo "JuniperBible SDK Parity Tests"
    echo "========================================"
    echo "Project root: $PROJECT_ROOT"
    echo "Test data: $TESTDATA_DIR"
    echo ""

    # Test each format plugin that has been migrated
    for plugin_dir in "$PROJECT_ROOT/plugins/format-"*/; do
        if [ -d "$plugin_dir" ]; then
            plugin_name=$(basename "$plugin_dir" | sed 's/format-//')
            # Only test if plugin has SDK config (indicating migration)
            if grep -q "sdk/format" "$plugin_dir"*.go 2>/dev/null; then
                run_golden_tests "$plugin_name"
            fi
        fi
    done

    echo ""
    echo "========================================"
    echo "Results: ${GREEN}$passed passed${NC}, ${RED}$failed failed${NC}, ${YELLOW}$skipped skipped${NC}"

    if [ $failed -gt 0 ]; then
        exit 1
    fi
}

# Generate golden outputs from current plugin behavior
generate_golden() {
    local plugin_name="$1"
    local plugin_binary="$PROJECT_ROOT/plugins/format-$plugin_name/format-$plugin_name"
    local output_dir="$TESTDATA_DIR/$plugin_name"

    if [ ! -x "$plugin_binary" ]; then
        echo "Plugin binary not found: $plugin_binary"
        exit 1
    fi

    mkdir -p "$output_dir"

    echo "Generating golden outputs for: $plugin_name"

    # Generate detect response
    if [ -f "$TESTDATA_DIR/detect_request.json" ]; then
        cat "$TESTDATA_DIR/detect_request.json" | "$plugin_binary" > "$output_dir/detect_response.json"
        echo "  Created: detect_response.json"
    fi

    # Add more command generations as needed

    echo "Golden outputs saved to: $output_dir"
}

# Main
case "${1:-}" in
    generate)
        if [ -z "${2:-}" ]; then
            echo "Usage: $0 generate <plugin-name>"
            exit 1
        fi
        generate_golden "$2"
        ;;
    test|"")
        run_all_tests
        ;;
    *)
        echo "Usage: $0 [test|generate <plugin-name>]"
        exit 1
        ;;
esac
