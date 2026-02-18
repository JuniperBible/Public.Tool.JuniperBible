#!/usr/bin/env bash
# Validate that a format plugin has been properly converted to a thin wrapper.
#
# Usage: ./scripts/validate-thin-wrapper.sh <format-name>
# Example: ./scripts/validate-thin-wrapper.sh json

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

success() {
    echo -e "${GREEN}[PASS]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

fail() {
    echo -e "${RED}[FAIL]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*"
    exit 1
}

# Check arguments
if [ $# -ne 1 ]; then
    error "Usage: $0 <format-name>
Example: $0 json"
fi

FORMAT_NAME="$1"
FORMAT_NAME_LOWER=$(echo "$FORMAT_NAME" | tr '[:upper:]' '[:lower:]')
PLUGIN_DIR="plugins/format-${FORMAT_NAME_LOWER}"
CANONICAL_DIR="core/formats/${FORMAT_NAME_LOWER}"

info "Validating thin wrapper for format: ${FORMAT_NAME_LOWER}"
echo ""

PASSED=0
FAILED=0
WARNED=0

# ---------------------------------------------------------------------------
# Unified check runner
# Accepts a pass message, fail message, and a command string to evaluate.
# Sets the global PASSED/FAILED counters accordingly.
# ---------------------------------------------------------------------------
run_check() {
    local pass_msg="$1"
    local fail_msg="$2"
    local cmd="$3"

    if eval "$cmd"; then
        success "$pass_msg"
        ((PASSED++))
    else
        fail "$fail_msg"
        ((FAILED++))
    fi
}

# ---------------------------------------------------------------------------
# Special check: line count (three-way branch kept in its own function so the
# main flow remains a simple loop with no nested conditionals).
# ---------------------------------------------------------------------------
check_line_count() {
    local file="$1"
    local count
    count=$(wc -l < "$file")

    if [ "$count" -le 20 ]; then
        success "Line count is acceptable: ${count} lines (<=20)"
        ((PASSED++))
    elif [ "$count" -le 30 ]; then
        warn "Line count is high: ${count} lines (target <=20)"
        ((WARNED++))
    else
        fail "Line count too high: ${count} lines (should be <=20)"
        ((FAILED++))
    fi
}

# ---------------------------------------------------------------------------
# Special check: count occurrences of a list of tokens in a file.
# Reports pass when count is zero; fail otherwise.
# ---------------------------------------------------------------------------
check_absent_tokens() {
    local file="$1"
    local pass_msg="$2"
    local fail_msg_prefix="$3"
    shift 3
    local tokens=("$@")
    local count=0

    for token in "${tokens[@]}"; do
        if grep -q "$token" "$file"; then
            ((count++))
        fi
    done

    if [ "$count" -eq 0 ]; then
        success "$pass_msg"
        ((PASSED++))
    else
        fail "${fail_msg_prefix}${count}"
        ((FAILED++))
    fi
}

# ---------------------------------------------------------------------------
# Simple boolean checks expressed as parallel arrays:
#   CHECKS_CMD   – the shell command evaluated by run_check
#   CHECKS_PASS  – message printed on success
#   CHECKS_FAIL  – message printed on failure
#   CHECKS_FATAL – '1' means exit immediately on failure (like original check 2)
# ---------------------------------------------------------------------------
CHECKS_CMD=(
    "[ -d '${PLUGIN_DIR}' ]"
    "[ -f '${PLUGIN_DIR}/main.go' ]"
    "grep -q '^//go:build standalone' '${PLUGIN_DIR}/main.go'"
    "grep -q 'github.com/FocuswithJustin/JuniperBible/core/formats/${FORMAT_NAME_LOWER}' '${PLUGIN_DIR}/main.go'"
    "grep -q 'github.com/FocuswithJustin/JuniperBible/plugins/sdk/format' '${PLUGIN_DIR}/main.go'"
    "grep -q 'format.Run(' '${PLUGIN_DIR}/main.go'"
    "grep -q '${FORMAT_NAME_LOWER}.Config' '${PLUGIN_DIR}/main.go'"
    "[ -d '${CANONICAL_DIR}' ]"
    "[ -f '${CANONICAL_DIR}/format.go' ]"
    "[ -f '${CANONICAL_DIR}/format.go' ] && grep -q 'var Config = &format.Config' '${CANONICAL_DIR}/format.go'"
    "go build -tags standalone -o /dev/null '${PLUGIN_DIR}/main.go' 2>/dev/null"
)

CHECKS_PASS=(
    "Plugin directory exists: ${PLUGIN_DIR}"
    "main.go exists"
    "Has //go:build standalone tag"
    "Imports canonical format package"
    "Imports SDK format package"
    "Calls format.Run()"
    "Passes Config variable"
    "Canonical directory exists: ${CANONICAL_DIR}"
    "Canonical format.go exists"
    "Config variable exported in canonical package"
    "Compiles with standalone tag"
)

CHECKS_FAIL=(
    "Plugin directory not found: ${PLUGIN_DIR}"
    "main.go not found"
    "Missing //go:build standalone tag"
    "Does not import canonical format package"
    "Does not import SDK format package"
    "Does not call format.Run()"
    "Does not pass Config variable"
    "Canonical directory not found: ${CANONICAL_DIR}"
    "Canonical format.go not found"
    "Config variable not found in canonical package"
    "Does not compile with standalone tag"
)

# Index of the check whose failure causes an immediate exit (0-based; check 2
# in the original script was the main.go existence check, now index 1).
FATAL_CHECK_INDEX=1

# ---------------------------------------------------------------------------
# Run all simple boolean checks
# ---------------------------------------------------------------------------
for i in "${!CHECKS_CMD[@]}"; do
    run_check "${CHECKS_PASS[$i]}" "${CHECKS_FAIL[$i]}" "${CHECKS_CMD[$i]}"

    if [ "$i" -eq "$FATAL_CHECK_INDEX" ] && [ "$FAILED" -gt 0 ]; then
        exit 1
    fi
done

# ---------------------------------------------------------------------------
# Line count check (three-way: pass / warn / fail)
# ---------------------------------------------------------------------------
check_line_count "${PLUGIN_DIR}/main.go"

# ---------------------------------------------------------------------------
# Absence checks: IPC type duplication and handler functions
# ---------------------------------------------------------------------------
check_absent_tokens \
    "${PLUGIN_DIR}/main.go" \
    "No duplicated IPC types in wrapper" \
    "Found duplicated IPC type definitions: " \
    "type IPCRequest" "type IPCResponse" "type DetectResult" "type IngestResult" "type Corpus"

check_absent_tokens \
    "${PLUGIN_DIR}/main.go" \
    "No handler functions in wrapper (delegated to SDK)" \
    "Found handler functions (should be in canonical package): " \
    "func handleDetect" "func handleIngest" "func handleEnumerate" "func handleExtractIR" "func handleEmitNative"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "================================"
echo "Validation Summary"
echo "================================"
echo -e "Format: ${BLUE}${FORMAT_NAME_LOWER}${NC}"
echo -e "Passed: ${GREEN}${PASSED}${NC}"
if [ "$WARNED" -gt 0 ]; then
    echo -e "Warnings: ${YELLOW}${WARNED}${NC}"
fi
if [ "$FAILED" -gt 0 ]; then
    echo -e "Failed: ${RED}${FAILED}${NC}"
fi
echo ""

if [ "$FAILED" -eq 0 ]; then
    success "All checks passed! Format is properly converted to thin wrapper."
    exit 0
else
    fail "Validation failed with $FAILED errors"
    echo ""
    echo "To fix issues:"
    echo "  1. Review the failed checks above"
    echo "  2. Ensure Phase 2 is complete: core/formats/${FORMAT_NAME_LOWER}/"
    echo "  3. Re-run conversion: ./scripts/convert-to-thin-wrapper.sh ${FORMAT_NAME_LOWER}"
    echo "  4. See docs/THIN_WRAPPER_MIGRATION.md for details"
    exit 1
fi
