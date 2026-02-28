#!/usr/bin/env bash
# Build libsword from source for CGo reference testing
# Clean room approach: library built and stored in vendor_external/libsword/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIBSWORD_DIR="${SCRIPT_DIR}/libsword"
BUILD_DIR="${LIBSWORD_DIR}/build"
SRC_DIR="${LIBSWORD_DIR}/src"

# SWORD library version to build
SWORD_VERSION="1.9.0"
SWORD_URL="https://crosswire.org/ftpmirror/pub/sword/source/v1.9/sword-${SWORD_VERSION}.tar.gz"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_dependencies() {
    log_info "Checking build dependencies..."

    local missing=()

    command -v cmake >/dev/null 2>&1 || missing+=("cmake")
    command -v make >/dev/null 2>&1 || missing+=("make")
    command -v g++ >/dev/null 2>&1 || missing+=("g++")
    command -v curl >/dev/null 2>&1 || missing+=("curl")

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing dependencies: ${missing[*]}"
        echo "Install with:"
        echo "  Ubuntu/Debian: sudo apt-get install ${missing[*]} libz-dev libcurl4-openssl-dev libicu-dev"
        echo "  Fedora/RHEL: sudo dnf install ${missing[*]} zlib-devel libcurl-devel libicu-devel"
        echo "  macOS: brew install ${missing[*]}"
        exit 1
    fi

    log_info "All build dependencies present"
}

download_source() {
    log_info "Downloading SWORD library source v${SWORD_VERSION}..."

    mkdir -p "${SRC_DIR}"
    local tarball="${SRC_DIR}/sword-${SWORD_VERSION}.tar.gz"

    if [[ -f "${tarball}" ]]; then
        log_info "Source tarball already downloaded"
    else
        curl -L -o "${tarball}" "${SWORD_URL}" || {
            log_error "Failed to download SWORD source"
            exit 1
        }
    fi

    # Verify checksum (SWORD 1.9.0 SHA256)
    local expected_sha256="5f89a25c1d14c9f9bbc7cc45da7faa97ed82fd79ece0a08ac35b4fc98c97ebe1"
    local actual_sha256=$(sha256sum "${tarball}" | cut -d' ' -f1)

    if [[ "${actual_sha256}" != "${expected_sha256}" ]]; then
        log_warn "Checksum mismatch (may be different version):"
        log_warn "  Expected: ${expected_sha256}"
        log_warn "  Got:      ${actual_sha256}"
    fi

    # Extract
    log_info "Extracting source..."
    tar -xzf "${tarball}" -C "${SRC_DIR}" --strip-components=1
}

build_libsword() {
    log_info "Building libsword..."

    mkdir -p "${BUILD_DIR}"
    cd "${BUILD_DIR}"

    # Configure with CMake
    cmake "${SRC_DIR}" \
        -DCMAKE_INSTALL_PREFIX="${LIBSWORD_DIR}/install" \
        -DCMAKE_BUILD_TYPE=Release \
        -DSWORD_BUILD_UTILS=ON \
        -DSWORD_BUILD_TESTS=OFF \
        -DSWORD_BINDINGS=OFF \
        -DWITH_CLUCENE=OFF \
        -DWITH_ICU=ON \
        -DWITH_CURL=ON \
        -DWITH_ZLIB=ON

    # Build
    make -j$(nproc 2>/dev/null || echo 4)

    # Install to local directory
    make install

    log_info "libsword built successfully"
}

copy_headers() {
    log_info "Copying headers for CGo..."

    local include_dir="${LIBSWORD_DIR}/include"
    mkdir -p "${include_dir}"

    # Copy essential headers
    cp -r "${LIBSWORD_DIR}/install/include/sword/"* "${include_dir}/" 2>/dev/null || \
    cp -r "${SRC_DIR}/include/"* "${include_dir}/" || {
        log_error "Failed to copy headers"
        exit 1
    }

    log_info "Headers copied to ${include_dir}"
}

copy_diatheke() {
    log_info "Copying diatheke binary..."

    local diatheke_src="${LIBSWORD_DIR}/install/bin/diatheke"
    local diatheke_dst="${SCRIPT_DIR}/diatheke/diatheke"

    mkdir -p "${SCRIPT_DIR}/diatheke"

    if [[ -f "${diatheke_src}" ]]; then
        cp "${diatheke_src}" "${diatheke_dst}"
        chmod +x "${diatheke_dst}"
        log_info "diatheke copied to ${diatheke_dst}"
    else
        log_warn "diatheke binary not found in build output"
    fi
}

generate_cgo_bindings() {
    log_info "Generating CGo binding helper..."

    cat > "${LIBSWORD_DIR}/cgo_flags.go" << 'EOF'
// +build cgo,libsword

package libsword

/*
#cgo CXXFLAGS: -I${SRCDIR}/include
#cgo LDFLAGS: -L${SRCDIR}/install/lib -lsword -lstdc++
*/
import "C"
EOF

    log_info "CGo flags helper created"
}

write_usage_instructions() {
    cat > "${LIBSWORD_DIR}/USAGE.md" << EOF
# libsword - Clean Room Build

Version: ${SWORD_VERSION}
Built: $(date -Iseconds)

## Directory Structure

\`\`\`
libsword/
├── include/           # Header files for CGo
├── install/
│   ├── bin/          # diatheke and other utilities
│   ├── lib/          # libsword.so / libsword.a
│   └── share/        # Data files
├── build/            # CMake build directory
└── src/              # Source code
\`\`\`

## Using with CGo

Set environment variables:

\`\`\`bash
export CGO_CXXFLAGS="-I${LIBSWORD_DIR}/include"
export CGO_LDFLAGS="-L${LIBSWORD_DIR}/install/lib -lsword"
export LD_LIBRARY_PATH="${LIBSWORD_DIR}/install/lib:\${LD_LIBRARY_PATH}"
\`\`\`

Build with CGo enabled:

\`\`\`bash
CGO_ENABLED=1 go build -tags libsword ./...
\`\`\`

## Using diatheke

\`\`\`bash
export PATH="${LIBSWORD_DIR}/install/bin:\${PATH}"
export SWORD_PATH="${SCRIPT_DIR}/sword_modules"

diatheke -b KJV -k "John 3:16"
\`\`\`

## Verification

\`\`\`bash
# Check library
ldd ${LIBSWORD_DIR}/install/lib/libsword.so

# Check diatheke
${LIBSWORD_DIR}/install/bin/diatheke -v
\`\`\`
EOF

    log_info "Usage instructions written to ${LIBSWORD_DIR}/USAGE.md"
}

generate_checksums() {
    log_info "Generating checksums..."
    cd "${LIBSWORD_DIR}"
    find install -type f \( -name "*.so*" -o -name "*.a" -o -name "diatheke" \) \
        -exec sha256sum {} \; > checksums.sha256
    log_info "Checksums written to ${LIBSWORD_DIR}/checksums.sha256"
}

# Main execution
log_info "libsword Builder - Clean Room Setup"
log_info "Target directory: ${LIBSWORD_DIR}"
log_info "SWORD version: ${SWORD_VERSION}"

check_dependencies
download_source
build_libsword
copy_headers
copy_diatheke
generate_cgo_bindings
write_usage_instructions
generate_checksums

log_info "Done! libsword built in ${LIBSWORD_DIR}"
log_info "See ${LIBSWORD_DIR}/USAGE.md for usage instructions"
