#!/usr/bin/env bash
# Build all standalone plugins and verify they work
set -e

echo "Verifying standalone plugin builds..."
echo ""

FAILED=0
PASSED=0
TOTAL=0

for plugin in plugins/format-*/; do
    if [ ! -d "$plugin" ]; then
        continue
    fi

    name=$(basename "$plugin")
    TOTAL=$((TOTAL + 1))

    echo "[$TOTAL] Building $name..."

    # Build with standalone tag
    if go build -tags standalone -o "/tmp/$name" "./$plugin" 2>&1; then
        echo "  ✓ Build successful"

        # Test basic IPC functionality (detect command)
        if echo '{"command":"detect","args":{"path":"README.md"}}' | "/tmp/$name" >/dev/null 2>&1; then
            echo "  ✓ IPC test passed"
            PASSED=$((PASSED + 1))
        else
            echo "  ✗ IPC test failed"
            FAILED=$((FAILED + 1))
        fi

        # Clean up binary
        rm -f "/tmp/$name"
    else
        echo "  ✗ Build failed"
        FAILED=$((FAILED + 1))
    fi
    echo ""
done

echo "=========================================="
echo "Results: $PASSED/$TOTAL passed"
echo "=========================================="

if [ $FAILED -gt 0 ]; then
    echo "ERROR: $FAILED plugins failed verification"
    exit 1
fi

echo "All standalone plugins verified successfully!"
exit 0
