#!/usr/bin/env bash
# Juniper Bible Functional Test Suite
# Tests all major CLI and workflow functionality
#
# Usage: ./tests/functional_test.sh [--quick] [--verbose]
#
# Options:
#   --quick    Run only essential tests (skip long-running tests)
#   --verbose  Show detailed output
#
# Exit codes:
#   0 - All tests passed
#   1 - One or more tests failed

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_DIR="/tmp/capsule-functional-test-$$"
CAPSULE_BIN="$PROJECT_ROOT/bin/capsule"
CAPSULE_WEB_BIN="$PROJECT_ROOT/bin/capsule-web"
SAMPLE_DATA="$PROJECT_ROOT/contrib/sample-data"

# Test modules (KJV, DRC, Vulgate, ASV, Geneva as specified)
TEST_MODULES=("kjv" "drc" "vulgate" "asv" "geneva1599")

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Options
QUICK_MODE=false
VERBOSE=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        --quick)
            QUICK_MODE=true
            ;;
        --verbose)
            VERBOSE=true
            ;;
    esac
done

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "\n${GREEN}=== TEST: $1 ===${NC}"
}

pass_test() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++)) || true
}

fail_test() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++)) || true
}

skip_test() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
    ((TESTS_SKIPPED++)) || true
}

cleanup() {
    log_info "Cleaning up test directory..."
    rm -rf "$TEST_DIR"
    # Kill any background web server
    pkill -f "capsule-web.*8899" 2>/dev/null || true
}

trap cleanup EXIT

# Guard helpers - return 1 (causing caller to return) when condition not met

# skip_if_quick <label>
# Skips the named test and returns 1 when QUICK_MODE is true.
skip_if_quick() {
    $QUICK_MODE || return 0
    skip_test "$1"
    return 1
}

# require_dir <path> <label>
# Skips the named test and returns 1 when the directory does not exist.
require_dir() {
    [[ -d "$1" ]] && return 0
    skip_test "$2"
    return 1
}

# require_file <path> <label>
# Skips the named test and returns 1 when the file does not exist.
require_file() {
    [[ -f "$1" ]] && return 0
    skip_test "$2"
    return 1
}

# verbose_dump <output>
# Prints output only when VERBOSE is true.
verbose_dump() {
    $VERBOSE && echo "$1" || true
}

# mod_id <mod>
# Prints the canonical module identifier for a lower-case module directory name.
mod_id() {
    case "$1" in
        kjv)       echo "KJV" ;;
        drc)       echo "DRC" ;;
        vulgate)   echo "Vulgate" ;;
        asv)       echo "ASV" ;;
        geneva1599) echo "Geneva1599" ;;
        *)         echo "$1" | tr '[:lower:]' '[:upper:]' ;;
    esac
}

# run_go_test <pkg_pattern> <tail_lines> <label>
# Runs go test, checks for passing output, and reports the result.
run_go_test() {
    local pkg="$1" lines="$2" label="$3"
    local output
    output=$(go test "$pkg" -short 2>&1 | tail -"$lines")
    local pass_count fail_count
    pass_count=$(echo "$output" | grep -c "^ok" || true)
    fail_count=$(echo "$output" | grep -c "^FAIL" || true)
    if [[ $fail_count -eq 0 ]]; then
        pass_test "$label ($pass_count packages)"
    else
        fail_test "$label - $fail_count packages failed"
        verbose_dump "$output"
    fi
}

# Setup
setup() {
    log_info "Setting up test environment..."
    mkdir -p "$TEST_DIR"
    cd "$PROJECT_ROOT"

    # Build binaries if needed
    if [[ ! -f "$CAPSULE_BIN" ]]; then
        log_info "Building capsule CLI..."
        CGO_ENABLED=0 go build -o "$CAPSULE_BIN" ./cmd/capsule
    fi

    if [[ ! -f "$CAPSULE_WEB_BIN" ]]; then
        log_info "Building capsule-web..."
        go build -o "$CAPSULE_WEB_BIN" ./cmd/capsule-web
    fi

    # Build sword-pure plugin if needed
    if [[ ! -f "$PROJECT_ROOT/plugins/format/sword-pure/format-sword-pure" ]]; then
        log_info "Building sword-pure plugin..."
        (cd "$PROJECT_ROOT/plugins/format/sword-pure" && go build -o format-sword-pure .)
    fi
}

# Test 1: Juniper List
test_juniper_list() {
    log_test "Juniper List - Sample Modules"

    for mod in "${TEST_MODULES[@]}"; do
        local mod_path="$SAMPLE_DATA/$mod"
        require_dir "$mod_path" "juniper list $mod - directory not found" || continue
        local output
        output=$("$CAPSULE_BIN" juniper list "$mod_path" 2>&1)
        if echo "$output" | grep -qi "modules"; then
            pass_test "juniper list $mod"
        else
            fail_test "juniper list $mod - unexpected output"
            verbose_dump "$output"
        fi
    done
}

# Test 2: Juniper Ingest
test_juniper_ingest() {
    log_test "Juniper Ingest - Create Capsules"

    for mod in "${TEST_MODULES[@]}"; do
        local mod_path="$SAMPLE_DATA/$mod"
        require_dir "$mod_path" "juniper ingest $mod - directory not found" || continue
        local mod_upper
        mod_upper=$(mod_id "$mod")
        local capsule_file="$TEST_DIR/$mod_upper.capsule.tar.gz"
        local output
        output=$("$CAPSULE_BIN" juniper ingest --path "$mod_path" -o "$TEST_DIR" "$mod_upper" 2>&1)
        if [[ ! -f "$capsule_file" ]]; then
            fail_test "juniper ingest $mod_upper - capsule not created"
            verbose_dump "$output"
            continue
        fi
        local size
        size=$(stat -f%z "$capsule_file" 2>/dev/null || stat -c%s "$capsule_file" 2>/dev/null)
        if [[ $size -gt 0 ]]; then
            pass_test "juniper ingest $mod_upper (${size} bytes)"
        else
            fail_test "juniper ingest $mod_upper - empty capsule"
        fi
    done
}

# Test 3: Capsule Ingest (proper capsule format)
test_capsule_ingest() {
    log_test "Capsule Ingest - Create Proper Capsules"

    local conf_file="$SAMPLE_DATA/kjv/mods.d/kjv.conf"
    require_file "$conf_file" "capsule ingest - kjv.conf not found" || return 0
    local output
    output=$("$CAPSULE_BIN" capsule ingest "$conf_file" --out "$TEST_DIR/kjv-proper.capsule.tar.gz" 2>&1)
    if [[ -f "$TEST_DIR/kjv-proper.capsule.tar.gz" ]]; then
        pass_test "capsule ingest kjv.conf"
    else
        fail_test "capsule ingest kjv.conf - capsule not created"
        verbose_dump "$output"
    fi
}

# Test 4: Capsule Verify
test_capsule_verify() {
    log_test "Capsule Verify"

    require_file "$TEST_DIR/kjv-proper.capsule.tar.gz" "capsule verify - capsule not found" || return 0
    local output
    output=$("$CAPSULE_BIN" capsule verify "$TEST_DIR/kjv-proper.capsule.tar.gz" 2>&1)
    if echo "$output" | grep -qi "passed\|ok"; then
        pass_test "capsule verify"
    else
        fail_test "capsule verify - verification failed"
        verbose_dump "$output"
    fi
}

# Test 5: Capsule Export
test_capsule_export() {
    log_test "Capsule Export"

    require_file "$TEST_DIR/kjv-proper.capsule.tar.gz" "capsule export - capsule not found" || return 0
    local output
    output=$("$CAPSULE_BIN" capsule export "$TEST_DIR/kjv-proper.capsule.tar.gz" --artifact kjv --out "$TEST_DIR/kjv-exported.conf" 2>&1)
    if [[ ! -f "$TEST_DIR/kjv-exported.conf" ]]; then
        fail_test "capsule export - file not created"
        verbose_dump "$output"
        return 0
    fi
    if grep -q "KJV" "$TEST_DIR/kjv-exported.conf"; then
        pass_test "capsule export"
    else
        fail_test "capsule export - content mismatch"
    fi
}

# Test 6: Format Detect
test_format_detect() {
    log_test "Format Detect"

    local kjv_path="$SAMPLE_DATA/kjv"
    require_dir "$kjv_path" "format detect - kjv not found" || return 0
    local output
    output=$("$CAPSULE_BIN" format detect "$kjv_path" 2>&1)
    if echo "$output" | grep -qi "sword\|MATCH"; then
        pass_test "format detect kjv"
    else
        fail_test "format detect kjv - SWORD not detected"
        verbose_dump "$output"
    fi
}

# Test 7: Plugins List
test_plugins_list() {
    log_test "Plugins List"

    local output
    output=$("$CAPSULE_BIN" plugins list 2>&1)
    local format_count
    format_count=$(echo "$output" | grep -c "format\." || true)

    if [[ $format_count -ge 30 ]]; then
        pass_test "plugins list ($format_count format plugins)"
    else
        fail_test "plugins list - expected 30+ format plugins, got $format_count"
        verbose_dump "$output"
    fi
}

# Test 8: Web UI Tests (via go test)
test_webui() {
    log_test "Web UI Tests"

    skip_if_quick "Web UI tests - skipped in quick mode" || return 0
    local output
    output=$(go test ./cmd/capsule-web/ -v -run "Test" 2>&1 | tail -20)
    if echo "$output" | grep -q "PASS"; then
        local pass_count
        pass_count=$(echo "$output" | grep -c "PASS" || true)
        pass_test "Web UI tests ($pass_count passed)"
    else
        fail_test "Web UI tests - some tests failed"
        verbose_dump "$output"
    fi
}

# Test 9: Web UI HTTP endpoints
test_webui_endpoints() {
    log_test "Web UI HTTP Endpoints"

    skip_if_quick "Web UI HTTP tests - skipped in quick mode" || return 0

    # Start web server
    "$CAPSULE_WEB_BIN" -capsules "$SAMPLE_DATA/capsules" -port 8899 &
    local web_pid=$!
    sleep 2

    # Test endpoints
    local endpoints=("/" "/plugins" "/convert" "/tools" "/sword")
    for endpoint in "${endpoints[@]}"; do
        local response
        response=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8899$endpoint" 2>/dev/null || echo "000")
        if [[ "$response" == "200" ]]; then
            pass_test "HTTP GET $endpoint"
        else
            fail_test "HTTP GET $endpoint - status $response"
        fi
    done

    # Kill web server
    kill $web_pid 2>/dev/null || true
}

# Test 10: Go Unit Tests (quick)
test_go_unit() {
    log_test "Go Unit Tests"

    if $QUICK_MODE; then
        run_go_test "./core/..." 10 "core package tests (quick)"
    else
        run_go_test "./..." 30 "All Go tests"
    fi
}

# Test 11: MySword Plugin
test_mysword_plugin() {
    log_test "MySword Plugin"

    local output
    output=$(go test ./plugins/format/mysword/ -v 2>&1 | tail -15)
    if echo "$output" | grep -q "PASS"; then
        local pass_count
        pass_count=$(echo "$output" | grep -c "PASS" || true)
        pass_test "MySword plugin tests ($pass_count passed)"
    else
        fail_test "MySword plugin tests failed"
        verbose_dump "$output"
    fi
}

# Test 12: CGO Comparison Tests (long running)
test_cgo_comparison() {
    log_test "CGO Comparison Tests"

    skip_if_quick "CGO comparison tests - skipped in quick mode" || return 0
    command -v diatheke &>/dev/null || { skip_test "CGO comparison tests - diatheke not installed"; return 0; }

    local output
    output=$(go test ./plugins/format/sword-pure/ -run CGOComparison -v -timeout 10m 2>&1 | tail -20)
    if echo "$output" | grep -q "PASS"; then
        pass_test "CGO comparison tests"
    else
        fail_test "CGO comparison tests failed"
        verbose_dump "$output"
    fi
}

# Main
main() {
    echo "========================================"
    echo "Juniper Bible Functional Test Suite"
    echo "========================================"
    echo "Project: $PROJECT_ROOT"
    echo "Test Dir: $TEST_DIR"
    echo "Quick Mode: $QUICK_MODE"
    echo "Verbose: $VERBOSE"
    echo ""

    setup

    # Run tests
    test_juniper_list
    test_juniper_ingest
    test_capsule_ingest
    test_capsule_verify
    test_capsule_export
    test_format_detect
    test_plugins_list
    test_webui
    test_webui_endpoints
    test_go_unit
    test_mysword_plugin
    test_cgo_comparison

    # Summary
    echo ""
    echo "========================================"
    echo "Test Summary"
    echo "========================================"
    echo -e "${GREEN}Passed:${NC}  $TESTS_PASSED"
    echo -e "${RED}Failed:${NC}  $TESTS_FAILED"
    echo -e "${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
    echo ""

    if [[ $TESTS_FAILED -gt 0 ]]; then
        log_error "Some tests failed!"
        exit 1
    else
        log_info "All tests passed!"
        exit 0
    fi
}

main "$@"
