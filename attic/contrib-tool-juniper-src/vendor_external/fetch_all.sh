#!/usr/bin/env bash
# Master script to fetch all external dependencies for clean room testing
# This ensures complete reproducibility and isolation from system dependencies

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_section() { echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"; echo -e "${BLUE}  $1${NC}"; echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}\n"; }

print_banner() {
    echo -e "${GREEN}"
    cat << 'EOF'
       _             _
      | |_   _ _ __ (_)_ __   ___ _ __
   _  | | | | | '_ \| | '_ \ / _ \ '__|
  | |_| | |_| | | | | | |_) |  __/ |
   \___/ \__,_|_| |_|_| .__/ \___|_|
                      |_|
  Clean Room Dependency Fetcher
EOF
    echo -e "${NC}"
}

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Fetch all external dependencies for comprehensive testing."
    echo ""
    echo "Options:"
    echo "  --all              Fetch everything (default)"
    echo "  --quick            Fetch minimal set for quick testing"
    echo "  --comprehensive    Fetch comprehensive module set"
    echo "  --sword            Fetch SWORD modules only"
    echo "  --esword           Create e-Sword test databases only"
    echo "  --libsword         Build libsword from source only"
    echo "  --spdx             Fetch SPDX license data only"
    echo "  --versifications   Generate versification files only"
    echo "  --verify           Verify existing dependencies"
    echo "  --clean            Remove all fetched dependencies"
    echo "  -h, --help         Show this help"
    echo ""
    echo "Examples:"
    echo "  $0 --quick         # Minimal setup for CI"
    echo "  $0 --comprehensive # Full test coverage"
    echo "  $0 --all           # Everything including libsword build"
}

verify_dependencies() {
    log_section "Verifying Dependencies"

    local all_ok=true

    # Check SWORD modules
    if [[ -f "${SCRIPT_DIR}/sword_modules/checksums.sha256" ]]; then
        log_info "Verifying SWORD modules..."
        if (cd "${SCRIPT_DIR}/sword_modules" && sha256sum -c checksums.sha256 --quiet 2>/dev/null); then
            log_info "SWORD modules: OK"
        else
            log_warn "SWORD modules: CHECKSUM MISMATCH"
            all_ok=false
        fi
    else
        log_warn "SWORD modules: NOT FOUND"
        all_ok=false
    fi

    # Check e-Sword modules
    if [[ -f "${SCRIPT_DIR}/esword_modules/checksums.sha256" ]]; then
        log_info "Verifying e-Sword modules..."
        if (cd "${SCRIPT_DIR}/esword_modules" && sha256sum -c checksums.sha256 --quiet 2>/dev/null); then
            log_info "e-Sword modules: OK"
        else
            log_warn "e-Sword modules: CHECKSUM MISMATCH"
            all_ok=false
        fi
    else
        log_warn "e-Sword modules: NOT FOUND"
        all_ok=false
    fi

    # Check SPDX data
    if [[ -f "${SCRIPT_DIR}/spdx/checksums.sha256" ]]; then
        log_info "Verifying SPDX data..."
        if (cd "${SCRIPT_DIR}/spdx" && sha256sum -c checksums.sha256 --quiet 2>/dev/null); then
            log_info "SPDX data: OK"
        else
            log_warn "SPDX data: CHECKSUM MISMATCH"
            all_ok=false
        fi
    else
        log_warn "SPDX data: NOT FOUND"
        all_ok=false
    fi

    # Check versifications
    if [[ -f "${SCRIPT_DIR}/versifications/checksums.sha256" ]]; then
        log_info "Verifying versifications..."
        if (cd "${SCRIPT_DIR}/versifications" && sha256sum -c checksums.sha256 --quiet 2>/dev/null); then
            log_info "Versifications: OK"
        else
            log_warn "Versifications: CHECKSUM MISMATCH"
            all_ok=false
        fi
    else
        log_warn "Versifications: NOT FOUND"
        all_ok=false
    fi

    # Check libsword
    if [[ -f "${SCRIPT_DIR}/libsword/install/lib/libsword.so" ]] || [[ -f "${SCRIPT_DIR}/libsword/install/lib/libsword.dylib" ]]; then
        log_info "libsword: OK"
    else
        log_warn "libsword: NOT BUILT"
    fi

    # Check diatheke
    if [[ -x "${SCRIPT_DIR}/diatheke/diatheke" ]]; then
        log_info "diatheke: OK"
    else
        log_warn "diatheke: NOT FOUND"
    fi

    if [[ "${all_ok}" == "true" ]]; then
        log_info "All dependencies verified successfully!"
        return 0
    else
        log_warn "Some dependencies are missing or have mismatched checksums"
        return 1
    fi
}

clean_all() {
    log_section "Cleaning All Dependencies"

    log_info "Removing sword_modules..."
    rm -rf "${SCRIPT_DIR}/sword_modules"

    log_info "Removing esword_modules..."
    rm -rf "${SCRIPT_DIR}/esword_modules"

    log_info "Removing libsword..."
    rm -rf "${SCRIPT_DIR}/libsword"

    log_info "Removing diatheke..."
    rm -rf "${SCRIPT_DIR}/diatheke"

    log_info "Removing spdx..."
    rm -rf "${SCRIPT_DIR}/spdx"

    log_info "Removing versifications..."
    rm -rf "${SCRIPT_DIR}/versifications"

    log_info "Removing testdata..."
    rm -rf "${SCRIPT_DIR}/testdata"

    log_info "Clean complete!"
}

generate_env_script() {
    log_section "Generating Environment Setup Script"

    cat > "${SCRIPT_DIR}/env.sh" << EOF
#!/usr/bin/env bash
# Source this file to set up the clean room environment
# Usage: source ${SCRIPT_DIR}/env.sh

export JUNIPER_VENDOR="${SCRIPT_DIR}"
export SWORD_PATH="${SCRIPT_DIR}/sword_modules"
export ESWORD_PATH="${SCRIPT_DIR}/esword_modules"
export SPDX_PATH="${SCRIPT_DIR}/spdx"
export VERSIFICATION_PATH="${SCRIPT_DIR}/versifications"

# libsword (if built)
if [[ -d "${SCRIPT_DIR}/libsword/install" ]]; then
    export CGO_CXXFLAGS="-I${SCRIPT_DIR}/libsword/include"
    export CGO_LDFLAGS="-L${SCRIPT_DIR}/libsword/install/lib -lsword"
    export LD_LIBRARY_PATH="${SCRIPT_DIR}/libsword/install/lib:\${LD_LIBRARY_PATH:-}"
    export DYLD_LIBRARY_PATH="${SCRIPT_DIR}/libsword/install/lib:\${DYLD_LIBRARY_PATH:-}"
fi

# diatheke (if available)
if [[ -x "${SCRIPT_DIR}/diatheke/diatheke" ]]; then
    export PATH="${SCRIPT_DIR}/diatheke:\${PATH}"
fi

echo "Clean room environment configured:"
echo "  SWORD_PATH=\${SWORD_PATH}"
echo "  ESWORD_PATH=\${ESWORD_PATH}"
echo "  SPDX_PATH=\${SPDX_PATH}"
echo "  VERSIFICATION_PATH=\${VERSIFICATION_PATH}"
EOF

    chmod +x "${SCRIPT_DIR}/env.sh"
    log_info "Generated env.sh - source this file to set up environment"
}

generate_master_checksums() {
    log_section "Generating Master Checksums"

    cd "${SCRIPT_DIR}"
    {
        echo "# Master checksums for all external dependencies"
        echo "# Generated: $(date -Iseconds)"
        echo ""

        for dir in sword_modules esword_modules spdx versifications; do
            if [[ -f "${dir}/checksums.sha256" ]]; then
                echo "# ${dir}"
                cat "${dir}/checksums.sha256"
                echo ""
            fi
        done
    } > master_checksums.sha256

    log_info "Generated master_checksums.sha256"
}

# Parse arguments
MODE="all"
SWORD_SET="quick"

while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            MODE="all"
            SWORD_SET="comprehensive"
            shift
            ;;
        --quick)
            MODE="all"
            SWORD_SET="quick"
            shift
            ;;
        --comprehensive)
            MODE="all"
            SWORD_SET="comprehensive"
            shift
            ;;
        --sword)
            MODE="sword"
            shift
            ;;
        --esword)
            MODE="esword"
            shift
            ;;
        --libsword)
            MODE="libsword"
            shift
            ;;
        --spdx)
            MODE="spdx"
            shift
            ;;
        --versifications)
            MODE="versifications"
            shift
            ;;
        --verify)
            verify_dependencies
            exit $?
            ;;
        --clean)
            clean_all
            exit 0
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main execution
print_banner

log_info "Clean Room Dependency Fetcher"
log_info "Mode: ${MODE}"
log_info "SWORD set: ${SWORD_SET}"
log_info "Target: ${SCRIPT_DIR}"

case "${MODE}" in
    all)
        log_section "Fetching SPDX License Data"
        "${SCRIPT_DIR}/fetch_spdx.sh"

        log_section "Generating Versification Files"
        "${SCRIPT_DIR}/fetch_versifications.sh"

        log_section "Creating e-Sword Test Databases"
        "${SCRIPT_DIR}/fetch_esword_modules.sh"

        log_section "Fetching SWORD Modules"
        "${SCRIPT_DIR}/fetch_sword_modules.sh" "--${SWORD_SET}"

        # libsword is optional - only build if dependencies are available
        if command -v cmake >/dev/null 2>&1 && command -v make >/dev/null 2>&1; then
            log_section "Building libsword (optional)"
            "${SCRIPT_DIR}/fetch_libsword.sh" || log_warn "libsword build failed (optional)"
        else
            log_warn "Skipping libsword build (cmake/make not available)"
        fi
        ;;
    sword)
        "${SCRIPT_DIR}/fetch_sword_modules.sh" "--${SWORD_SET}"
        ;;
    esword)
        "${SCRIPT_DIR}/fetch_esword_modules.sh"
        ;;
    libsword)
        "${SCRIPT_DIR}/fetch_libsword.sh"
        ;;
    spdx)
        "${SCRIPT_DIR}/fetch_spdx.sh"
        ;;
    versifications)
        "${SCRIPT_DIR}/fetch_versifications.sh"
        ;;
esac

generate_env_script
generate_master_checksums

log_section "Summary"
log_info "External dependencies fetched successfully!"
log_info ""
log_info "To use these dependencies, run:"
log_info "  source ${SCRIPT_DIR}/env.sh"
log_info ""
log_info "To verify integrity:"
log_info "  $0 --verify"
log_info ""
log_info "Directory sizes:"
du -sh "${SCRIPT_DIR}"/*/ 2>/dev/null | grep -v '.cache' || true
