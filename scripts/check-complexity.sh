#!/usr/bin/env bash
set -e

THRESHOLD=${1:-6}

# Check if gocyclo is available
if ! command -v gocyclo &> /dev/null; then
    echo "Installing gocyclo..."
    go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
fi

# Run gocyclo, excluding contrib/ directory
# The -over flag shows functions above the threshold
VIOLATIONS=$(gocyclo -over "$THRESHOLD" . 2>/dev/null | grep -v 'contrib/' || true)

if [ -n "$VIOLATIONS" ]; then
    echo "ERROR: Functions exceeding CC $THRESHOLD:"
    echo "$VIOLATIONS"
    echo ""
    echo "Total violations: $(echo "$VIOLATIONS" | wc -l)"
    exit 1
fi

echo "SUCCESS: All functions have CC <= $THRESHOLD"
