#!/usr/bin/env bash
# Ensure standalone wrappers remain thin (<20 lines)
set -e

echo "Checking wrapper file sizes..."
echo ""

FAILED=0
PASSED=0
TOTAL=0
MAX_LINES=20

for main in plugins/format-*/main.go; do
    if [ ! -f "$main" ]; then
        continue
    fi

    TOTAL=$((TOTAL + 1))
    lines=$(wc -l < "$main")
    plugin=$(dirname "$main" | xargs basename)

    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "✗ $main has $lines lines (max $MAX_LINES)"
        FAILED=$((FAILED + 1))
    else
        echo "✓ $main has $lines lines"
        PASSED=$((PASSED + 1))
    fi
done

echo ""
echo "=========================================="
echo "Results: $PASSED/$TOTAL passed"
echo "=========================================="

if [ $FAILED -gt 0 ]; then
    echo "ERROR: $FAILED wrappers exceed $MAX_LINES line limit"
    exit 1
fi

echo "All wrappers are appropriately thin!"
exit 0
