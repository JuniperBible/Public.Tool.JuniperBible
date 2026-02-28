#!/usr/bin/env bash
# Fetch SWORD modules for comprehensive testing
# Clean room approach: all modules stored in vendor_external/sword_modules/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SWORD_DIR="${SCRIPT_DIR}/sword_modules"
CACHE_DIR="${SWORD_DIR}/.cache"

# CrossWire FTP server
CROSSWIRE_FTP="ftp.crosswire.org"
CROSSWIRE_PATH="/pub/sword/packages/rawzip"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Module sets for tiered testing
QUICK_MODULES=(
    "KJV"
    "Tyndale"
    "Geneva1599"
    "DRC"
    "Vulgate"
)

COMPREHENSIVE_MODULES=(
    # Historic English
    "KJV" "KJVA" "KJV2006" "AKJV"
    "Tyndale" "Geneva1599" "Coverdale" "Matthew" "Great" "Bishops"
    "DRC" "Webster" "RWebster"

    # Modern English
    "ASV" "WEB" "WEBBE" "WEBMe" "BBE" "Darby" "YLT"
    "ISV" "NET" "NHEB" "NHEBJE" "NHEBME"

    # Original Languages
    "SBLGNT" "TR" "Byz" "WHNU" "Tischendorf"
    "OSMHB" "WLC" "Aleppo" "OSHB"
    "LXX" "LXXup" "ABGk"

    # Latin
    "Vulgate" "VulgClementine" "VulgHetworded" "VulgSistworded"
    "Clementine"

    # German
    "GerLut1545" "Luther1545" "GerElb1871" "GerSch" "GerMenge"

    # French
    "FreBBB" "FreCrl" "FreJND" "FreMartin" "FreOltramare" "FrePGR"

    # Spanish
    "SpaRV1909" "SpaRV1865" "SpaSEV" "SpaVNT"

    # Other Languages
    "ItaDio" "ItaRive" # Italian
    "PorAR" "PorBLH" # Portuguese
    "RusSynodal" "RusVZh" # Russian
    "PolGdanska" # Polish
    "CzeBKR" "CzeKMS" # Czech
    "HunKar" # Hungarian
    "RomCor" # Romanian
    "FinPR" # Finnish
    "SweSVE" # Swedish
    "NorSMB" # Norwegian
    "DutSVV" # Dutch

    # Commentaries
    "MHC" "MHCC" "JFB" "TSK" "Barnes" "Clarke"
    "Geneva" "Wesley" "Gill" "Poole" "Henry"

    # Dictionaries/Lexicons
    "StrongsGreek" "StrongsHebrew"
    "BDB" "Thayer" "TWOT"
    "Easton" "ISBE" "Naves" "Torrey"
)

# Ensure directories exist
mkdir -p "${SWORD_DIR}"/{mods.d,modules}
mkdir -p "${CACHE_DIR}"

fetch_mods_index() {
    log_info "Fetching module index from CrossWire..."
    local index_url="ftp://${CROSSWIRE_FTP}/pub/sword/raw/mods.d.tar.gz"
    local index_file="${CACHE_DIR}/mods.d.tar.gz"

    if [[ -f "${index_file}" ]] && [[ $(find "${index_file}" -mtime -1 2>/dev/null) ]]; then
        log_info "Using cached module index (less than 1 day old)"
    else
        curl -# -o "${index_file}" "${index_url}" || {
            log_error "Failed to download module index"
            return 1
        }
    fi

    # Extract to get module list
    tar -tzf "${index_file}" 2>/dev/null | grep '\.conf$' | sed 's|.*/||; s|\.conf$||' | sort -u > "${SWORD_DIR}/available_modules.txt"
    log_info "Found $(wc -l < "${SWORD_DIR}/available_modules.txt") available modules"
}

fetch_module() {
    local module_id="$1"
    local module_lower=$(echo "$module_id" | tr '[:upper:]' '[:lower:]')
    local module_dir="${SWORD_DIR}/modules/${module_lower}"
    local conf_file="${SWORD_DIR}/mods.d/${module_lower}.conf"

    # Check if already downloaded
    if [[ -f "${conf_file}" ]] && [[ -d "${module_dir}" ]]; then
        log_info "Module ${module_id} already present, skipping"
        return 0
    fi

    log_info "Fetching module: ${module_id}"

    # Try multiple URL patterns
    local urls=(
        "ftp://${CROSSWIRE_FTP}/pub/sword/packages/rawzip/${module_id}.zip"
        "ftp://${CROSSWIRE_FTP}/pub/sword/packages/rawzip/${module_lower}.zip"
        "ftp://${CROSSWIRE_FTP}/pub/sword/raw/mods.d/${module_lower}.conf"
    )

    local zip_file="${CACHE_DIR}/${module_id}.zip"
    local downloaded=false

    for url in "${urls[@]}"; do
        if curl -s --fail -o "${zip_file}" "${url}" 2>/dev/null; then
            downloaded=true
            break
        fi
    done

    if [[ "${downloaded}" != "true" ]]; then
        log_warn "Could not download ${module_id} - may not be available"
        return 1
    fi

    # Extract module
    if [[ -f "${zip_file}" ]]; then
        unzip -q -o "${zip_file}" -d "${SWORD_DIR}/" 2>/dev/null || {
            log_warn "Failed to extract ${module_id}"
            return 1
        }
        log_info "Extracted ${module_id}"
    fi

    return 0
}

fetch_module_set() {
    local set_name="$1"
    shift
    local modules=("$@")

    log_info "Fetching ${set_name} module set (${#modules[@]} modules)..."

    local success=0
    local failed=0

    for module in "${modules[@]}"; do
        if fetch_module "${module}"; then
            ((success++))
        else
            ((failed++))
        fi
    done

    log_info "${set_name} complete: ${success} succeeded, ${failed} failed"
}

generate_checksums() {
    log_info "Generating checksums..."
    cd "${SWORD_DIR}"
    find . -type f \( -name "*.bzz" -o -name "*.bzs" -o -name "*.bzv" -o -name "*.conf" \) \
        -exec sha256sum {} \; > checksums.sha256
    log_info "Checksums written to ${SWORD_DIR}/checksums.sha256"
}

# Parse arguments
SET="quick"
while [[ $# -gt 0 ]]; do
    case $1 in
        --quick)
            SET="quick"
            shift
            ;;
        --comprehensive)
            SET="comprehensive"
            shift
            ;;
        --all)
            SET="all"
            shift
            ;;
        --module)
            fetch_module "$2"
            exit $?
            ;;
        --help|-h)
            echo "Usage: $0 [--quick|--comprehensive|--all] [--module MODULE_ID]"
            echo ""
            echo "Options:"
            echo "  --quick          Fetch 5 essential modules (default)"
            echo "  --comprehensive  Fetch 100+ modules for thorough testing"
            echo "  --all            Fetch all available modules"
            echo "  --module ID      Fetch a specific module by ID"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Main execution
log_info "SWORD Module Fetcher - Clean Room Setup"
log_info "Target directory: ${SWORD_DIR}"

fetch_mods_index

case "${SET}" in
    quick)
        fetch_module_set "Quick" "${QUICK_MODULES[@]}"
        ;;
    comprehensive)
        fetch_module_set "Comprehensive" "${COMPREHENSIVE_MODULES[@]}"
        ;;
    all)
        log_info "Fetching ALL available modules..."
        while IFS= read -r module; do
            fetch_module "${module}" || true
        done < "${SWORD_DIR}/available_modules.txt"
        ;;
esac

generate_checksums

log_info "Done! Modules stored in ${SWORD_DIR}"
log_info "To use in tests: export SWORD_PATH=${SWORD_DIR}"
