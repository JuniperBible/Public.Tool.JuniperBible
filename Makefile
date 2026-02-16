# Juniper Bible Makefile (restructured targets + legacy aliases)
# Build outputs go to bin/
#
# Naming convention:
#   build-*  Build targets
#   test-*   Test targets
#   docs-*   Documentation targets
#   dist-*   Distribution targets
#   dev-*    Development utilities
#   fmt, lint, clean, help  Core utilities

SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c

# Paths
BIN          := bin
BIN_PLUGINS  := $(BIN)/plugins
BIN_FORMATS  := $(BIN_PLUGINS)/format
BIN_TOOLS    := $(BIN_PLUGINS)/tool
BIN_META     := $(BIN_PLUGINS)/meta
DIST         := dist
BUILD        := build

# Go
GO      := go
GOFLAGS :=

# Version for distribution
VERSION := 0.1.0

# All supported platforms (pure Go builds for all)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

# CGO-capable platforms
CGO_PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Legacy juniper source location
JUNIPER_LEGACY_SRC := contrib/tool/juniper/src

# Sample modules for data generation
SAMPLE_MODULES := KJV ASV DRC LXX OEB OSMHB SBLGNT Tyndale Vulgate WEB Geneva1599

# Format plugins to build
FORMAT_PLUGINS := file zip dir tar sword sword-pure osis usfm usx zefania theword json sqlite markdown html epub esword mysword dbl accordance flex gobible logos morphgnt odf onlinebible oshb pdb rtf sblgnt sfm tei txt xml

# Tool plugins to build
TOOL_PLUGINS := libsword pandoc calibre usfm2osis sqlite libxml2 unrtf gobible-creator repoman hugo

# =============================================================================
# Default / Common
# =============================================================================

.PHONY: help
help:
	@echo "Juniper Bible Makefile"
	@echo ""
	@echo "Build outputs go to: bin/"
	@echo ""
	@echo "COMMON"
	@echo "  make build                  Build main CLI (default dev build)"
	@echo "  make test                   Run quick unit tests (short mode)"
	@echo "  make all                    Build everything + run tests"
	@echo "  make clean                  Remove build artifacts"
	@echo "  make help                   Show this help"
	@echo ""
	@echo "BUILD (binaries)"
	@echo "  make build-cli              Build capsule CLI -> bin/"
	@echo "  make build-cgo              Build capsule with CGO SQLite -> bin/"
	@echo "  make build-web              Build web UI server (disabled, use capsule web)"
	@echo "  make build-api              Build REST API server (disabled, use capsule api)"
	@echo "  make build-juniper          Build juniper.sword -> bin/"
	@echo "  make build-standalone       Build all standalone binaries -> bin/"
	@echo ""
	@echo "BUILD (plugins)"
	@echo "  make build-plugins          Build all plugins -> bin/plugins/"
	@echo "  make build-formats          Build format plugins -> bin/plugins/format/"
	@echo "  make build-tools            Build tool plugins -> bin/plugins/tool/"
	@echo "  make build-meta             Build meta plugins -> bin/plugins/meta/"
	@echo "  make build-repoman          Build repoman tool plugin -> bin/"
	@echo ""
	@echo "LEGACY (CGO reference)"
	@echo "  make build-legacy           Build legacy juniper (CGO) -> bin/"
	@echo "  make build-legacy-extract   Build legacy extract tool -> bin/"
	@echo "  make build-legacy-all       Build all legacy binaries"
	@echo ""
	@echo "TEST"
	@echo "  make test                   Quick tests (short)"
	@echo "  make test-full              All tests (may take 10+ min)"
	@echo "  make test-v                 Verbose tests"
	@echo "  make test-coverage          Coverage report"
	@echo "  make test-cgo               Tests with CGO SQLite"
	@echo "  make test-integration       Integration tests"
	@echo "  make test-functional        Functional tests (CLI workflows)"
	@echo "  make test-sqlite-divergence Compare SQLite driver outputs"
	@echo ""
	@echo "TEST (juniper suites)"
	@echo "  make test-juniper           Juniper sample tests (base + samples)"
	@echo "  make test-juniper-base      Base tests only"
	@echo "  make test-juniper-samples   11-sample IR + round-trip tests"
	@echo "  make test-juniper-home      Comprehensive: all ~/.sword modules"
	@echo "  make test-juniper-all       Everything juniper (includes comprehensive)"
	@echo "  make test-workflow          Full workflow tests on sample capsules"
	@echo ""
	@echo "DOCS"
	@echo "  make docs                   Generate documentation"
	@echo "  make docs-verify            Verify docs are up to date"
	@echo ""
	@echo "DIST / RELEASE"
	@echo "  make dist                   Build all distributions (purego + cgo)"
	@echo "  make dist-purego            Pure Go distributions (all platforms)"
	@echo "  make dist-cgo               CGO distribution (current platform)"
	@echo "  make dist-local             Distributions (current platform only)"
	@echo "  make dist-list              List distribution files"
	@echo "  make dist-clean             Clean distribution artifacts"
	@echo ""
	@echo "DEV / UTIL"
	@echo "  make fmt                    Format Go code"
	@echo "  make lint                   Lint (vet + golangci-lint)"
	@echo "  make dev-deps               Install dev dependencies"
	@echo "  make dev-nix-shell          Enter Nix shell with test tools"
	@echo "  make selfcheck              Run self-check on example"
	@echo "  make verify                 Verify example capsule"
	@echo "  make plugins-list           List available plugins"
	@echo "  make check-stray            Check for stray binaries in root"
	@echo "  make verify-standalone      Verify all standalone plugins build"
	@echo "  make lint-wrappers          Ensure wrappers remain thin (<20 lines)"
	@echo ""
	@echo "SAMPLE DATA"
	@echo "  make sample-data            Generate sample capsules from ~/.sword"
	@echo "  make sample-data-clean      Clean sample data"
	@echo "  make sample-data-test       Test sample data workflow"
	@echo ""
	@echo "See docs/MIGRATION.md for old target name mappings."

.PHONY: all build clean dirs
all: build build-plugins test build-juniper build-legacy docs

build: build-cli

clean:
	rm -rf "$(BIN)" "$(DIST)" "$(BUILD)"
	# Remove any stray binaries from project root
	rm -f capsule capsule-web juniper.sword
	rm -f capsule-juniper capsule-repoman
	rm -f capsule-juniper-legacy capsule-juniper-legacy-extract
	rm -f sword-pure juniper-sword juniper-esword juniper-hugo juniper-repoman
	rm -f libsword pandoc calibre usfm2osis sqlite libxml2 unrtf gobible-creator zip tar
	# Remove test artifacts
	rm -f *.out *.test coverage.html
	rm -f tools/docgen/docgen tools/migrate-ipc/migrate-ipc
	find . -name "*.out" -type f -delete 2>/dev/null || true
	find . -name "*.test" -type f -delete 2>/dev/null || true
	find . -name "coverage.html" -type f -delete 2>/dev/null || true
	# Remove plugin binaries from source directories
	find plugins -name "format-*" -type f -executable -delete 2>/dev/null || true
	find plugins -name "tool-*" -type f -executable -delete 2>/dev/null || true
	find plugins -name "juniper-*" -type f -executable -delete 2>/dev/null || true
	find plugins -name "sword-pure" -type f -executable -delete 2>/dev/null || true

dirs:
	@mkdir -p "$(BIN)" "$(DIST)" "$(BUILD)"

# =============================================================================
# Build: Binaries
# =============================================================================

.PHONY: build-cli build-cgo build-web build-api build-juniper build-meta build-repoman build-standalone

build-cli: dirs
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o "$(BIN)/capsule" ./cmd/capsule

build-cgo: dirs
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) -tags cgo_sqlite -o "$(BIN)/capsule" ./cmd/capsule

# NOTE: Web and API servers temporarily disabled pending decision to deprecate or improve
# The main capsule binary now has integrated `capsule web` and `capsule api` commands
build-web: dirs
	@echo "Web server build disabled - use 'capsule web' command instead"

build-api: dirs
	@echo "API server build disabled - use 'capsule api' command instead"

build-juniper: dirs
	@echo "Building juniper.sword..."
	cd plugins/format/sword-pure && CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o "$(CURDIR)/$(BIN)/juniper.sword" .
	@echo "Built: $(BIN)/juniper.sword"
	@echo ""
	@echo "Usage:"
	@echo "  ./$(BIN)/juniper.sword list              List Bible modules in ~/.sword"
	@echo "  ./$(BIN)/juniper.sword ingest            Interactive module ingestion"
	@echo "  ./$(BIN)/juniper.sword ingest KJV DRC    Ingest specific modules"
	@echo "  ./$(BIN)/juniper.sword help              Show all options"

build-meta: dirs
	@echo "Building juniper meta plugin..."
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o "$(BIN)/capsule-juniper" ./plugins/meta/juniper
	@echo "Built: $(BIN)/capsule-juniper"

build-repoman: dirs
	@echo "Building repoman tool plugin..."
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o "$(BIN)/capsule-repoman" ./plugins/tool/repoman
	@echo "Built: $(BIN)/capsule-repoman"

build-standalone: build-juniper build-meta build-repoman
	@echo ""
	@echo "All standalone binaries built in $(BIN)/"
	@echo "  - juniper.sword (SWORD format plugin)"
	@echo "  - capsule-juniper (meta plugin CLI)"
	@echo "  - capsule-repoman (repository manager)"

# =============================================================================
# Build: Plugins
# =============================================================================

.PHONY: build-plugins build-formats build-tools

build-plugins: build-formats build-tools

build-formats: dirs
	@echo "Building format plugins to $(BIN_FORMATS)/..."
	@mkdir -p "$(BIN_FORMATS)"
	@for plugin in $(FORMAT_PLUGINS); do \
		if [ -d "plugins/format/$$plugin" ]; then \
			mkdir -p "$(BIN_FORMATS)/$$plugin"; \
			if [ -f "plugins/format/$$plugin/go.mod" ]; then \
				(cd "plugins/format/$$plugin" && $(GO) build $(GOFLAGS) -o "$(CURDIR)/$(BIN_FORMATS)/$$plugin/format-$$plugin" . 2>/dev/null) || true; \
			else \
				$(GO) build $(GOFLAGS) -o "$(BIN_FORMATS)/$$plugin/format-$$plugin" "./plugins/format/$$plugin" 2>/dev/null || true; \
			fi; \
			if [ -f "plugins/format/$$plugin/plugin.json" ]; then \
				cp "plugins/format/$$plugin/plugin.json" "$(BIN_FORMATS)/$$plugin/"; \
			fi; \
		fi; \
	done

build-tools: dirs
	@echo "Building tool plugins to $(BIN_TOOLS)/..."
	@mkdir -p "$(BIN_TOOLS)"
	@for plugin in $(TOOL_PLUGINS); do \
		if [ -d "plugins/tool/$$plugin" ]; then \
			mkdir -p "$(BIN_TOOLS)/$$plugin"; \
			if [ -f "plugins/tool/$$plugin/go.mod" ]; then \
				(cd "plugins/tool/$$plugin" && $(GO) build $(GOFLAGS) -o "$(CURDIR)/$(BIN_TOOLS)/$$plugin/tool-$$plugin" . 2>/dev/null) || true; \
			else \
				$(GO) build $(GOFLAGS) -o "$(BIN_TOOLS)/$$plugin/tool-$$plugin" "./plugins/tool/$$plugin" 2>/dev/null || true; \
			fi; \
			if [ -f "plugins/tool/$$plugin/plugin.json" ]; then \
				cp "plugins/tool/$$plugin/plugin.json" "$(BIN_TOOLS)/$$plugin/"; \
			fi; \
		fi; \
	done

# =============================================================================
# Build: Legacy (CGO reference)
# =============================================================================

.PHONY: build-legacy build-legacy-extract build-legacy-all

build-legacy: dirs
	@echo "Building capsule-juniper-legacy from contrib reference..."
	@echo "Note: This builds the OLD buggy implementation for comparison testing"
	@if [ ! -d "$(JUNIPER_LEGACY_SRC)" ]; then \
		echo "ERROR: Reference juniper source not found at $(JUNIPER_LEGACY_SRC)"; \
		exit 1; \
	fi
	cd "$(JUNIPER_LEGACY_SRC)" && CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o "$(CURDIR)/$(BIN)/capsule-juniper-legacy" ./cmd/juniper
	@echo "Built: $(BIN)/capsule-juniper-legacy"

build-legacy-extract: dirs
	@echo "Building capsule-juniper-legacy-extract from contrib reference..."
	@if [ ! -d "$(JUNIPER_LEGACY_SRC)" ]; then \
		echo "ERROR: Reference juniper source not found at $(JUNIPER_LEGACY_SRC)"; \
		exit 1; \
	fi
	cd "$(JUNIPER_LEGACY_SRC)" && CGO_ENABLED=1 $(GO) build $(GOFLAGS) -o "$(CURDIR)/$(BIN)/capsule-juniper-legacy-extract" ./cmd/extract
	@echo "Built: $(BIN)/capsule-juniper-legacy-extract"

build-legacy-all: build-legacy build-legacy-extract
	@echo "All legacy juniper binaries built in $(BIN)/"

# =============================================================================
# Tests
# =============================================================================

.PHONY: test test-full test-v test-coverage test-cgo test-integration test-functional test-functional-quick test-dist test-dist-quick test-sqlite-divergence ci

test:
	CGO_ENABLED=0 $(GO) test -short ./...

ci: test lint-wrappers verify-standalone
	@echo ""
	@echo "=========================================="
	@echo "CI checks passed successfully!"
	@echo "=========================================="

test-full:
	CGO_ENABLED=0 $(GO) test -timeout 10m ./...

test-v:
	$(GO) test -v ./...

test-coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-cgo:
	CGO_ENABLED=1 $(GO) test -tags cgo_sqlite ./...

test-integration: build build-plugins
	./$(BIN)/capsule test testdata/fixtures

test-functional:
	./tests/functional_test.sh

test-functional-quick:
	./tests/functional_test.sh --quick

test-dist:
	./tests/dist_test.sh

test-dist-quick:
	./tests/dist_test.sh --skip-build

test-sqlite-divergence:
	@echo "Testing SQLite divergence (pure Go)..."
	@CGO_ENABLED=0 $(GO) test ./core/sqlite/... -v -run Divergence 2>&1 | tee /tmp/sqlite-purego.txt
	@echo ""
	@echo "Testing SQLite divergence (CGO)..."
	@CGO_ENABLED=1 $(GO) test -tags cgo_sqlite ./core/sqlite/... -v -run Divergence 2>&1 | tee /tmp/sqlite-cgo.txt
	@echo ""
	@echo "Comparing hashes..."
	@grep "Divergence hash:" /tmp/sqlite-purego.txt > /tmp/hash1.txt
	@grep "Divergence hash:" /tmp/sqlite-cgo.txt > /tmp/hash2.txt
	@if diff -q /tmp/hash1.txt /tmp/hash2.txt > /dev/null; then \
		echo "SUCCESS: Both drivers produce identical results"; \
		cat /tmp/hash1.txt; \
	else \
		echo "FAILURE: Drivers have diverged!"; \
		echo "Pure Go:"; cat /tmp/hash1.txt; \
		echo "CGO:"; cat /tmp/hash2.txt; \
		exit 1; \
	fi

# =============================================================================
# Tests: Juniper Suites
# =============================================================================

.PHONY: test-juniper test-juniper-base test-juniper-samples test-juniper-home test-juniper-all test-workflow

test-juniper-base:
	@echo "Running juniper base tests (11 sample Bibles)..."
	cd plugins/format/sword-pure && CGO_ENABLED=0 $(GO) test -v -run "TestBase" .

test-juniper-samples:
	@echo "Running juniper sample Bible tests (11 Bibles, parallel)..."
	cd plugins/format/sword-pure && CGO_ENABLED=0 $(GO) test -v -timeout 5m -run "TestExtractIRSampleBibles|TestZTextRoundTripAllSampleBibles" .

test-juniper-home:
	@echo "Running juniper comprehensive tests (ALL ~/.sword modules)..."
	@echo "Warning: This tests ALL installed modules and may take a long time."
	cd plugins/format/sword-pure && SWORD_TEST_ALL=1 CGO_ENABLED=0 $(GO) test -v -timeout 30m -run "TestComprehensive|TestZTextRoundTripAllSampleBibles" .

test-juniper: test-juniper-base test-juniper-samples
	@echo ""
	@echo "Sample tests complete. Run 'make test-juniper-home' for full ~/.sword testing."

test-juniper-all: test-juniper-base
	@echo "Running comprehensive juniper tests (all modules)..."
	cd plugins/format/sword-pure && SWORD_TEST_ALL=1 CGO_ENABLED=0 $(GO) test -v -timeout 60m ./...

test-workflow: build build-juniper
	@echo "Running full workflow tests on sample capsules..."
	@PASSED=0; FAILED=0; TOTAL=0; \
	for capsule in contrib/sample-data/capsules/*.tar.gz; do \
		if [ -f "$$capsule" ]; then \
			TOTAL=$$((TOTAL + 1)); \
			NAME=$$(basename "$$capsule"); \
			echo ""; \
			echo "Testing: $$NAME"; \
			if ./$(BIN)/capsule capsule verify "$$capsule" >/dev/null 2>&1 && \
			   tar -tzf "$$capsule" 2>/dev/null | grep -q "ir/" && \
			   ./$(BIN)/capsule capsule selfcheck "$$capsule" >/dev/null 2>&1; then \
				echo "  PASSED"; \
				PASSED=$$((PASSED + 1)); \
			else \
				echo "  FAILED"; \
				FAILED=$$((FAILED + 1)); \
			fi; \
		fi; \
	done; \
	echo ""; \
	echo "==========================================="; \
	echo "Workflow test results: $$PASSED/$$TOTAL passed"; \
	if [ $$FAILED -gt 0 ]; then \
		echo "$$FAILED tests FAILED"; \
		exit 1; \
	fi

# =============================================================================
# Documentation
# =============================================================================

.PHONY: docs docs-verify docs-api

docs: build build-plugins
	@echo "Generating documentation..."
	@mkdir -p docs/generated
	./$(BIN)/capsule dev docgen all --output docs/generated
	@echo "Documentation generated in docs/generated/"

docs-verify: docs
	@echo "Checking if documentation is up to date..."
	@if git diff --quiet docs/generated/; then \
		echo "Documentation is up to date."; \
	else \
		echo "ERROR: Documentation is out of date. Run 'make docs' and commit."; \
		git diff --stat docs/generated/; \
		exit 1; \
	fi

docs-api:
	@echo "API documentation generation not yet implemented"
	@echo "See docs/IR_IMPLEMENTATION.md for IR API"
	@echo "See docs/PLUGIN_DEVELOPMENT.md for plugin API"

# =============================================================================
# Distribution
# =============================================================================

.PHONY: dist dist-purego dist-cgo dist-local dist-list dist-clean
.PHONY: dist-platform-purego dist-platform-cgo dist-build-purego dist-build-cgo dist-package

dist: dist-clean dist-purego dist-cgo
	@echo ""
	@echo "All distributions built in $(DIST)/"
	@echo ""
	@echo "Pure Go builds (all platforms):"
	@ls -1 "$(DIST)"/*-purego.tar.gz 2>/dev/null || echo "  (none)"
	@echo ""
	@echo "CGO builds (linux, darwin):"
	@ls -1 "$(DIST)"/*-cgo.tar.gz 2>/dev/null || echo "  (none)"

dist-purego:
	@echo "Building pure-Go distributions for all platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo "$$platform" | cut -d'/' -f1); \
		GOARCH=$$(echo "$$platform" | cut -d'/' -f2); \
		$(MAKE) dist-platform-purego GOOS="$$GOOS" GOARCH="$$GOARCH"; \
	done

dist-cgo:
	@echo "Building CGO distribution for current platform..."
	@CURRENT_OS=$$(go env GOOS); \
	CURRENT_ARCH=$$(go env GOARCH); \
	PLATFORM="$$CURRENT_OS/$$CURRENT_ARCH"; \
	if echo "$(CGO_PLATFORMS)" | grep -q "$$PLATFORM"; then \
		$(MAKE) dist-platform-cgo GOOS="$$CURRENT_OS" GOARCH="$$CURRENT_ARCH"; \
	else \
		echo "  Skipping CGO build for $$PLATFORM (not in CGO_PLATFORMS)"; \
	fi

dist-local:
	@echo "Building distributions for current platform..."
	@CURRENT_OS=$$(go env GOOS); \
	CURRENT_ARCH=$$(go env GOARCH); \
	$(MAKE) dist-platform-purego GOOS="$$CURRENT_OS" GOARCH="$$CURRENT_ARCH"; \
	PLATFORM="$$CURRENT_OS/$$CURRENT_ARCH"; \
	if echo "$(CGO_PLATFORMS)" | grep -q "$$PLATFORM"; then \
		$(MAKE) dist-platform-cgo GOOS="$$CURRENT_OS" GOARCH="$$CURRENT_ARCH"; \
	fi

dist-list:
	@echo "Distribution files in $(DIST)/:"
	@ls -lah "$(DIST)"/*.tar.gz "$(DIST)"/*.tar.xz "$(DIST)"/*.zip 2>/dev/null || echo "No distribution files found. Run 'make dist' first."

dist-clean:
	rm -rf "$(DIST)" "$(BUILD)"
	mkdir -p "$(DIST)" "$(BUILD)"

# Internal distribution targets
dist-platform-purego:
	@if [ -z "$(GOOS)" ] || [ -z "$(GOARCH)" ]; then \
		echo "ERROR: GOOS and GOARCH must be set"; \
		exit 1; \
	fi
	@echo ""
	@echo "=========================================="
	@echo "Building pure-Go for $(GOOS)/$(GOARCH)..."
	@echo "=========================================="
	@$(MAKE) dist-build-purego GOOS=$(GOOS) GOARCH=$(GOARCH)
	@$(MAKE) dist-package GOOS=$(GOOS) GOARCH=$(GOARCH) VARIANT=purego

dist-platform-cgo:
	@if [ -z "$(GOOS)" ] || [ -z "$(GOARCH)" ]; then \
		echo "ERROR: GOOS and GOARCH must be set"; \
		exit 1; \
	fi
	@echo ""
	@echo "=========================================="
	@echo "Building CGO for $(GOOS)/$(GOARCH)..."
	@echo "=========================================="
	@$(MAKE) dist-build-cgo GOOS=$(GOOS) GOARCH=$(GOARCH)
	@$(MAKE) dist-package GOOS=$(GOOS) GOARCH=$(GOARCH) VARIANT=cgo

dist-build-purego:
	@DIST_BUILD="$(BUILD)/$(GOOS)-$(GOARCH)-purego"; \
	mkdir -p "$$DIST_BUILD/bin" "$$DIST_BUILD/plugins/example/noop"; \
	EXT=""; \
	if [ "$(GOOS)" = "windows" ]; then EXT=".exe"; fi; \
	echo "  Building main binaries (pure Go, modernc.org/sqlite)..."; \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o "$$DIST_BUILD/bin/capsule$$EXT" ./cmd/capsule; \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o "$$DIST_BUILD/bin/juniper.sword$$EXT" ./plugins/format/sword-pure; \
	echo "  Building noop placeholder plugin..."; \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o "$$DIST_BUILD/plugins/example/noop/example-noop$$EXT" ./plugins/example/noop; \
	cp plugins/example/noop/plugin.json "$$DIST_BUILD/plugins/example/noop/"; \
	cp plugins/example/noop/README.md "$$DIST_BUILD/plugins/example/noop/"

dist-build-cgo:
	@DIST_BUILD="$(BUILD)/$(GOOS)-$(GOARCH)-cgo"; \
	mkdir -p "$$DIST_BUILD/bin" "$$DIST_BUILD/plugins/example/noop"; \
	EXT=""; \
	if [ "$(GOOS)" = "windows" ]; then EXT=".exe"; fi; \
	echo "  Building main binaries (CGO, mattn/go-sqlite3)..."; \
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -tags cgo_sqlite -o "$$DIST_BUILD/bin/capsule$$EXT" ./cmd/capsule; \
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -tags cgo_sqlite -o "$$DIST_BUILD/bin/juniper.sword$$EXT" ./plugins/format/sword-pure; \
	echo "  Building noop placeholder plugin..."; \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o "$$DIST_BUILD/plugins/example/noop/example-noop$$EXT" ./plugins/example/noop; \
	cp plugins/example/noop/plugin.json "$$DIST_BUILD/plugins/example/noop/"; \
	cp plugins/example/noop/README.md "$$DIST_BUILD/plugins/example/noop/"

dist-package:
	@if [ -z "$(VARIANT)" ]; then \
		echo "ERROR: VARIANT must be set (purego or cgo)"; \
		exit 1; \
	fi
	@DIST_BUILD="$(BUILD)/$(GOOS)-$(GOARCH)-$(VARIANT)"; \
	DIST_NAME="juniper-bible-$(VERSION)-$(GOOS)-$(GOARCH)-$(VARIANT)"; \
	DIST_PKG="$(DIST)/$$DIST_NAME"; \
	mkdir -p "$$DIST_PKG"; \
	echo "  Packaging $$DIST_NAME..."; \
	cp -r "$$DIST_BUILD/bin" "$$DIST_PKG/"; \
	cp -r "$$DIST_BUILD/plugins" "$$DIST_PKG/"; \
	mkdir -p "$$DIST_PKG/capsules"; \
	if [ -d "contrib/sample-data/capsules" ]; then \
		cp -r contrib/sample-data/capsules/* "$$DIST_PKG/capsules/" 2>/dev/null || true; \
	fi; \
	cp README "$$DIST_PKG/" 2>/dev/null || true; \
	cp LICENSE "$$DIST_PKG/" 2>/dev/null || true; \
	cp CHANGELOG "$$DIST_PKG/" 2>/dev/null || true; \
	echo "  Creating archives..."; \
	(cd "$(DIST)" && tar -czf "$$DIST_NAME.tar.gz" "$$DIST_NAME"); \
	(cd "$(DIST)" && tar -cJf "$$DIST_NAME.tar.xz" "$$DIST_NAME"); \
	(cd "$(DIST)" && zip -rq "$$DIST_NAME.zip" "$$DIST_NAME"); \
	rm -rf "$$DIST_PKG"; \
	echo "  Created: $$DIST_NAME.tar.gz, $$DIST_NAME.tar.xz, $$DIST_NAME.zip"

# =============================================================================
# Dev / Utilities
# =============================================================================

.PHONY: fmt lint dev-deps dev-nix-shell selfcheck verify plugins-list check-stray verify-standalone lint-wrappers

fmt:
	gofmt -w .

lint:
	@echo "Running go vet (excluding core/ir participle grammar files)..."
	@$(GO) vet $$($(GO) list ./... | grep -v '/core/ir$$')
	@echo "Running go vet on core/ir without structtag check..."
	@$(GO) vet -structtag=false ./core/ir/... 2>/dev/null || true
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

dev-deps:
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "Consider installing golangci-lint for additional linting"

dev-nix-shell:
	@echo "Entering Nix shell with integration test tools..."
	@echo "Available tools: diatheke, ebook-convert, pandoc, xmllint, sqlite3, unrtf"
	nix-shell tests/integration/shell.nix

selfcheck: build
	./$(BIN)/capsule selfcheck testdata/examples/sample.capsule.tar.xz

verify: build
	./$(BIN)/capsule verify testdata/examples/sample.capsule.tar.xz

plugins-list: build build-plugins
	./$(BIN)/capsule plugins

check-stray:
	@STRAY=$$(find . -maxdepth 1 -type f -executable ! -name "*.sh" ! -name "*.py" 2>/dev/null); \
	if [ -n "$$STRAY" ]; then \
		echo "ERROR: Found stray binaries in project root:"; \
		echo "$$STRAY"; \
		echo ""; \
		echo "All binaries should be in bin/, build/, or dist/"; \
		echo "Run 'make clean' to remove them"; \
		exit 1; \
	else \
		echo "OK: No stray binaries in project root"; \
	fi

verify-standalone:
	@./scripts/verify-standalone-builds.sh

lint-wrappers:
	@./scripts/lint-wrappers.sh

# =============================================================================
# Sample Data Generation
# =============================================================================

.PHONY: sample-data sample-data-clean sample-data-test

sample-data: build-juniper
	@echo "Generating sample capsules with IR..."
	@mkdir -p contrib/sample-data/capsules
	@for mod in $(SAMPLE_MODULES); do \
		if [ -d "$$HOME/.sword/modules" ]; then \
			./$(BIN)/juniper.sword ingest "$$mod" --output contrib/sample-data/capsules/ 2>/dev/null || \
			echo "  Skipping $$mod (not found in ~/.sword)"; \
		fi; \
	done
	@echo "Sample capsules generated in contrib/sample-data/capsules/"
	@ls -la contrib/sample-data/capsules/*.tar.gz 2>/dev/null || echo "No capsules found"

sample-data-clean:
	rm -rf contrib/sample-data/capsules/*.tar.gz

sample-data-test: build build-juniper
	@echo "Testing sample data workflow..."
	@CAPSULE=$$(ls contrib/sample-data/capsules/*.tar.gz 2>/dev/null | head -1); \
	if [ -z "$$CAPSULE" ]; then \
		echo "ERROR: No sample capsules found. Run 'make sample-data' first."; \
		exit 1; \
	fi; \
	echo "Testing with: $$CAPSULE"; \
	echo "  1. Verify capsule..."; \
	./$(BIN)/capsule capsule verify "$$CAPSULE" || exit 1; \
	echo "  2. Check IR exists..."; \
	tar -tzf "$$CAPSULE" | grep -q "ir/" && echo "     IR found" || (echo "     ERROR: No IR in capsule"; exit 1); \
	echo "  3. Selfcheck..."; \
	./$(BIN)/capsule capsule selfcheck "$$CAPSULE" || exit 1; \
	echo "  Sample data workflow test PASSED"

