#!/usr/bin/env bash
# Show the status of Phase 3 thin wrapper migration.
# Lists all format plugins and their migration status.

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo "================================"
echo "Phase 3: Thin Wrapper Migration Status"
echo "================================"
echo ""

TOTAL=0
MIGRATED=0
NOT_MIGRATED=0
MISSING_CANONICAL=0

# Header
printf "%-20s %-15s %-10s %-50s\n" "Format" "Status" "Lines" "Notes"
echo "--------------------------------------------------------------------------------"

# Check each format plugin
for PLUGIN_DIR in plugins/format-*/; do
    [ -d "$PLUGIN_DIR" ] || continue

    # Skip template directory
    if [[ "$PLUGIN_DIR" == "plugins/format-template/" ]]; then
        continue
    fi

    FORMAT_NAME=$(basename "$PLUGIN_DIR" | sed 's/^format-//')
    CANONICAL_DIR="core/formats/${FORMAT_NAME}"
    MAIN_FILE="${PLUGIN_DIR}main.go"

    TOTAL=$((TOTAL + 1))

    # Check if it's a thin wrapper
    if [ -f "$MAIN_FILE" ]; then
        LINE_COUNT=$(wc -l < "$MAIN_FILE" 2>/dev/null | tr -d '[:space:]' || echo "0")
        HAS_BUILD_TAG=$(grep -c "^//go:build standalone" "$MAIN_FILE" 2>/dev/null | tr -d '[:space:]' || echo "0")
        CALLS_SDK=$(grep -c "format.Run(" "$MAIN_FILE" 2>/dev/null | tr -d '[:space:]' || echo "0")

        if [ "$HAS_BUILD_TAG" -gt 0 ] && [ "$CALLS_SDK" -gt 0 ] && [ "$LINE_COUNT" -le 20 ]; then
            # It's a thin wrapper!
            printf "${GREEN}%-20s %-15s %-10s %-50s${NC}\n" \
                "$FORMAT_NAME" "✓ Migrated" "${LINE_COUNT} lines" "Thin wrapper complete"
            MIGRATED=$((MIGRATED + 1))
        elif [ "$LINE_COUNT" -le 30 ] && [ "$CALLS_SDK" -gt 0 ]; then
            # Close to being a thin wrapper
            printf "${YELLOW}%-20s %-15s %-10s %-50s${NC}\n" \
                "$FORMAT_NAME" "⚠ Partial" "${LINE_COUNT} lines" "Uses SDK but has extra code"
            NOT_MIGRATED=$((NOT_MIGRATED + 1))
        else
            # Still using old implementation
            if [ ! -d "$CANONICAL_DIR" ]; then
                printf "${RED}%-20s %-15s %-10s %-50s${NC}\n" \
                    "$FORMAT_NAME" "✗ Not Started" "${LINE_COUNT} lines" "Missing canonical (Phase 2 needed)"
                MISSING_CANONICAL=$((MISSING_CANONICAL + 1))
            else
                printf "${RED}%-20s %-15s %-10s %-50s${NC}\n" \
                    "$FORMAT_NAME" "✗ Not Migrated" "${LINE_COUNT} lines" "Canonical exists, needs conversion"
            fi
            NOT_MIGRATED=$((NOT_MIGRATED + 1))
        fi
    else
        printf "${RED}%-20s %-15s %-10s %-50s${NC}\n" \
            "$FORMAT_NAME" "✗ Missing" "N/A" "main.go not found"
        ((NOT_MIGRATED++))
    fi
done

echo "--------------------------------------------------------------------------------"
echo ""
echo "Summary:"
echo -e "  Total formats:         ${BLUE}${TOTAL}${NC}"
echo -e "  Migrated (thin):       ${GREEN}${MIGRATED}${NC}"
echo -e "  Not migrated:          ${RED}${NOT_MIGRATED}${NC}"
if [ "$MISSING_CANONICAL" -gt 0 ]; then
    echo -e "  Missing canonical:     ${YELLOW}${MISSING_CANONICAL}${NC} (Phase 2 needed first)"
fi
echo ""

# Calculate percentage
if [ "$TOTAL" -gt 0 ]; then
    PERCENT=$((MIGRATED * 100 / TOTAL))
    echo -e "Progress: ${CYAN}${PERCENT}%${NC} complete"

    # Progress bar
    BAR_LENGTH=50
    FILLED=$((PERCENT * BAR_LENGTH / 100))
    EMPTY=$((BAR_LENGTH - FILLED))

    printf "["
    printf "%${FILLED}s" | tr ' ' '='
    printf "%${EMPTY}s" | tr ' ' '-'
    printf "] ${PERCENT}%%\n"
fi

echo ""
echo "Next steps:"
if [ "$MISSING_CANONICAL" -gt 0 ]; then
    echo "  1. Complete Phase 2 for formats missing canonical implementations"
    echo "  2. Run: ./scripts/convert-to-thin-wrapper.sh <format-name>"
else
    echo "  1. Run: ./scripts/convert-to-thin-wrapper.sh <format-name>"
fi
echo "  2. Validate: ./scripts/validate-thin-wrapper.sh <format-name>"
echo "  3. Test: go test core/formats/<format-name>/..."
echo ""
echo "See docs/THIN_WRAPPER_MIGRATION.md for details."
