#!/usr/bin/env bash
# Convert a standalone format plugin to a thin wrapper that calls canonical core/formats package.
#
# Usage: ./scripts/convert-to-thin-wrapper.sh <format-name>
# Example: ./scripts/convert-to-thin-wrapper.sh json

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
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
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
BACKUP_DIR="${PLUGIN_DIR}/main.go.backup.$(date +%Y%m%d_%H%M%S)"

# Validate format name
if [[ ! "$FORMAT_NAME_LOWER" =~ ^[a-z0-9_-]+$ ]]; then
    error "Invalid format name: $FORMAT_NAME. Must contain only lowercase letters, numbers, hyphens, and underscores."
fi

info "Converting format: ${FORMAT_NAME_LOWER}"

# Check if plugin directory exists
if [ ! -d "$PLUGIN_DIR" ]; then
    error "Plugin directory does not exist: $PLUGIN_DIR"
fi

# Check if canonical implementation exists
if [ ! -d "$CANONICAL_DIR" ]; then
    error "Canonical format implementation not found: $CANONICAL_DIR
Please complete Phase 2 for this format before converting to thin wrapper."
fi

# Check if canonical format.go exists
if [ ! -f "${CANONICAL_DIR}/format.go" ]; then
    error "Canonical format.go not found: ${CANONICAL_DIR}/format.go"
fi

# Check if Config is exported in canonical package
if ! grep -q "var Config = &format.Config" "${CANONICAL_DIR}/format.go"; then
    error "Config variable not found in ${CANONICAL_DIR}/format.go
The canonical package must export a Config variable."
fi

info "Found canonical implementation at: $CANONICAL_DIR"

# Check if main.go exists
if [ ! -f "${PLUGIN_DIR}/main.go" ]; then
    error "Plugin main.go not found: ${PLUGIN_DIR}/main.go"
fi

# Create backup
info "Creating backup: $BACKUP_DIR"
cp "${PLUGIN_DIR}/main.go" "$BACKUP_DIR"
success "Backup created"

# Get the current line count for comparison
OLD_LINES=$(wc -l < "${PLUGIN_DIR}/main.go")

# Create the thin wrapper
info "Creating thin wrapper..."
cat > "${PLUGIN_DIR}/main.go" <<EOF
//go:build standalone

package main

import (
	"github.com/FocuswithJustin/JuniperBible/core/formats/${FORMAT_NAME_LOWER}"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func main() {
	format.Run(${FORMAT_NAME_LOWER}.Config)
}
EOF

NEW_LINES=$(wc -l < "${PLUGIN_DIR}/main.go")
success "Thin wrapper created: ${PLUGIN_DIR}/main.go"
info "Line count: ${OLD_LINES} -> ${NEW_LINES} (reduced by $((OLD_LINES - NEW_LINES)) lines, $(( (OLD_LINES - NEW_LINES) * 100 / OLD_LINES ))% reduction)"

# Validate the wrapper compiles with standalone build tag
info "Validating compilation with standalone build tag..."
if go build -tags standalone -o /tmp/format-${FORMAT_NAME_LOWER}-test "${PLUGIN_DIR}/main.go" 2>&1; then
    success "Compilation successful"
    rm -f /tmp/format-${FORMAT_NAME_LOWER}-test
else
    error "Compilation failed! Restoring backup..."
    cp "$BACKUP_DIR" "${PLUGIN_DIR}/main.go"
    error "Wrapper compilation failed. Backup restored."
fi

# Test that the plugin responds to basic IPC commands
info "Testing basic IPC functionality..."
if echo '{"command":"detect","args":{"path":"nonexistent.txt"}}' | /tmp/format-${FORMAT_NAME_LOWER}-test 2>&1 | grep -q '"status"'; then
    success "IPC test passed"
else
    warn "IPC test did not return expected JSON response (this may be normal for some formats)"
fi

# Summary
echo ""
success "Conversion complete!"
info "Summary:"
info "  - Format: ${FORMAT_NAME_LOWER}"
info "  - Plugin: ${PLUGIN_DIR}/main.go"
info "  - Canonical: ${CANONICAL_DIR}/format.go"
info "  - Backup: $BACKUP_DIR"
info "  - Old size: ${OLD_LINES} lines"
info "  - New size: ${NEW_LINES} lines"
info "  - Reduction: $((OLD_LINES - NEW_LINES)) lines ($((100 - (NEW_LINES * 100 / OLD_LINES)))%)"

echo ""
info "Next steps:"
info "  1. Run tests: go test ${PLUGIN_DIR}/..."
info "  2. Test standalone plugin manually"
info "  3. If all tests pass, commit changes"
info "  4. To rollback: cp $BACKUP_DIR ${PLUGIN_DIR}/main.go"
