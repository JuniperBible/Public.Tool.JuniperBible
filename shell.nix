{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # Go development
    go
    gopls
    gotools
    go-tools
    delve

    # CGO dependencies (for SQLite-based plugins and legacy juniper build)
    sqlite.dev  # Development headers for go-sqlite3
    sqlite.out  # Runtime library
    pkg-config
    gcc

    # Reference tools
    sword

    # Build and test utilities
    gnumake
    git
    jq
    cloc

    # Documentation
    pandoc

    # Archive handling
    zip
    unzip
    gnutar
    xz

    # XML tools (for format plugins)
    libxml2
    libxslt

    # E-book tools (for calibre plugin)
    calibre

    # Network tools (for repoman testing)
    curl
    wget
  ];

  # CGO configuration for sqlite
  CGO_ENABLED = "1";

  shellHook = ''
    export TZ=UTC
    export LC_ALL=C.UTF-8
    export LANG=C.UTF-8

    # Go module configuration
    export GOFLAGS="-mod=readonly"

    # CGO configuration for go-sqlite3
    # pkg-config handles the include/lib paths automatically
    export CGO_CFLAGS="$(pkg-config --cflags sqlite3 2>/dev/null || true)"
    export CGO_LDFLAGS="$(pkg-config --libs sqlite3 2>/dev/null || true)"

    # Runtime library path for SQLite (needed for test binaries)
    SQLITE_LIB_PATH="$(pkg-config --variable=libdir sqlite3 2>/dev/null || true)"
    if [ -n "$SQLITE_LIB_PATH" ]; then
      export LD_LIBRARY_PATH="$SQLITE_LIB_PATH:$LD_LIBRARY_PATH"
    fi

    # Clear Go build cache on first entry to pick up new CGO settings
    if [ ! -f /tmp/.juniper-nix-cache-cleared ]; then
      go clean -cache 2>/dev/null || true
      touch /tmp/.juniper-nix-cache-cleared
    fi

    echo ""
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║              Juniper Bible Development Environment              ║"
    echo "╠═══════════════════════════════════════════════════════════════╣"
    echo "║                                                               ║"
    echo "║  Build:                                                       ║"
    echo "║    make build          - Build capsule CLI                    ║"
    echo "║    make plugins        - Build all plugins                    ║"
    echo "║    make all            - Build everything and test            ║"
    echo "║                                                               ║"
    echo "║  Test:                                                        ║"
    echo "║    make test           - Run all tests                        ║"
    echo "║    make test-v         - Run tests with verbose output        ║"
    echo "║    make integration    - Run integration tests                ║"
    echo "║                                                               ║"
    echo "║  Juniper Plugins (113 tests):                                 ║"
    echo "║    cd plugins/juniper && go test ./...                        ║"
    echo "║                                                               ║"
    echo "║  Legacy Juniper (comparison testing):                         ║"
    echo "║    make juniper-legacy     - Build legacy CLI                 ║"
    echo "║    make juniper-legacy-all - Build all legacy binaries        ║"
    echo "║                                                               ║"
    echo "║  Documentation:                                               ║"
    echo "║    make docs           - Generate documentation               ║"
    echo "║                                                               ║"
    echo "║  SWORD tools available:                                       ║"
    echo "║    diatheke -b KJV -k \"Gen 1:1\"                               ║"
    echo "║                                                               ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo ""
  '';
}
