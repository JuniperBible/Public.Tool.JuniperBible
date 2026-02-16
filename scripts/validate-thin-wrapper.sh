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

# Check 1: Plugin directory exists
if [ -d "$PLUGIN_DIR" ]; then
    success "Plugin directory exists: $PLUGIN_DIR"
    ((PASSED++))
else
    fail "Plugin directory not found: $PLUGIN_DIR"
    ((FAILED++))
fi

# Check 2: main.go exists
if [ -f "${PLUGIN_DIR}/main.go" ]; then
    success "main.go exists"
    ((PASSED++))
else
    fail "main.go not found"
    ((FAILED++))
    exit 1
fi

# Check 3: Has standalone build tag
if grep -q "^//go:build standalone" "${PLUGIN_DIR}/main.go"; then
    success "Has //go:build standalone tag"
    ((PASSED++))
else
    fail "Missing //go:build standalone tag"
    ((FAILED++))
fi

# Check 4: Line count (should be ~5-20 lines)
LINE_COUNT=$(wc -l < "${PLUGIN_DIR}/main.go")
if [ "$LINE_COUNT" -le 20 ]; then
    success "Line count is acceptable: ${LINE_COUNT} lines (≤20)"
    ((PASSED++))
elif [ "$LINE_COUNT" -le 30 ]; then
    warn "Line count is high: ${LINE_COUNT} lines (target ≤20)"
    ((WARNED++))
else
    fail "Line count too high: ${LINE_COUNT} lines (should be ≤20)"
    ((FAILED++))
fi

# Check 5: Imports canonical format package
if grep -q "github.com/FocuswithJustin/JuniperBible/core/formats/${FORMAT_NAME_LOWER}" "${PLUGIN_DIR}/main.go"; then
    success "Imports canonical format package"
    ((PASSED++))
else
    fail "Does not import canonical format package"
    ((FAILED++))
fi

# Check 6: Imports SDK format package
if grep -q "github.com/FocuswithJustin/JuniperBible/plugins/sdk/format" "${PLUGIN_DIR}/main.go"; then
    success "Imports SDK format package"
    ((PASSED++))
else
    fail "Does not import SDK format package"
    ((FAILED++))
fi

# Check 7: Calls format.Run()
if grep -q "format.Run(" "${PLUGIN_DIR}/main.go"; then
    success "Calls format.Run()"
    ((PASSED++))
else
    fail "Does not call format.Run()"
    ((FAILED++))
fi

# Check 8: Passes Config variable
if grep -q "${FORMAT_NAME_LOWER}.Config" "${PLUGIN_DIR}/main.go"; then
    success "Passes Config variable"
    ((PASSED++))
else
    fail "Does not pass Config variable"
    ((FAILED++))
fi

# Check 9: Canonical directory exists
if [ -d "$CANONICAL_DIR" ]; then
    success "Canonical directory exists: $CANONICAL_DIR"
    ((PASSED++))
else
    fail "Canonical directory not found: $CANONICAL_DIR"
    ((FAILED++))
fi

# Check 10: Canonical format.go exists
if [ -f "${CANONICAL_DIR}/format.go" ]; then
    success "Canonical format.go exists"
    ((PASSED++))
else
    fail "Canonical format.go not found"
    ((FAILED++))
fi

# Check 11: Config variable exported in canonical
if [ -f "${CANONICAL_DIR}/format.go" ] && grep -q "var Config = &format.Config" "${CANONICAL_DIR}/format.go"; then
    success "Config variable exported in canonical package"
    ((PASSED++))
else
    fail "Config variable not found in canonical package"
    ((FAILED++))
fi

# Check 12: Compiles with standalone tag
if go build -tags standalone -o /dev/null "${PLUGIN_DIR}/main.go" 2>/dev/null; then
    success "Compiles with standalone tag"
    ((PASSED++))
else
    fail "Does not compile with standalone tag"
    ((FAILED++))
fi

# Check 13: No duplicated IPC types in wrapper
DUPLICATES=0
for TYPE in "IPCRequest" "IPCResponse" "DetectResult" "IngestResult" "Corpus"; do
    if grep -q "type $TYPE" "${PLUGIN_DIR}/main.go"; then
        ((DUPLICATES++))
    fi
done
if [ "$DUPLICATES" -eq 0 ]; then
    success "No duplicated IPC types in wrapper"
    ((PASSED++))
else
    fail "Found $DUPLICATES duplicated IPC type definitions"
    ((FAILED++))
fi

# Check 14: No handler functions in wrapper
HANDLERS=0
for HANDLER in "handleDetect" "handleIngest" "handleEnumerate" "handleExtractIR" "handleEmitNative"; do
    if grep -q "func $HANDLER" "${PLUGIN_DIR}/main.go"; then
        ((HANDLERS++))
    fi
done
if [ "$HANDLERS" -eq 0 ]; then
    success "No handler functions in wrapper (delegated to SDK)"
    ((PASSED++))
else
    fail "Found $HANDLERS handler functions (should be in canonical package)"
    ((FAILED++))
fi

# Summary
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
