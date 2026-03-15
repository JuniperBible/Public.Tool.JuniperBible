# CAS Migration, Test Codegen, Complexity — Implementation Plan

## Task 1: CAS Migration to Private.Lib.Veronica — COMPLETED

Removed the built-in filesystem CAS and replaced it with a dependency on
Private.Lib.Veronica.

### Changes

- **Removed** built-in filesystem CAS files: `store.go`, `blake3.go`,
  `store_test.go`, `blake3_test.go`.
- **Created** `hash.go` with pure `Hash()`, `Blake3Hash()`, `isValidHash()`,
  `ErrBlobNotFound`, and `ErrInvalidHash`.
- **Updated** `iface.go` with `HashResult` struct; removed `Store` compile
  check.
- **Updated** `veronica.go` with `blake3Pointer` struct.
- **Updated** `capsule.go`: removed `cas.NewStore` fallback; Veronica CAS is
  now required.
- **Updated** `cmd/capsule/main.go`: `initVeronica` now fatals on failure;
  `capsuleOpts` is a var for test overriding.
- **Updated** all test files to use a mock `VeronicaCAS` across `core/capsule`,
  `core/runner`, `core/selfcheck`, `cmd/capsule`, and `internal/tools`.

---

## Task 2: Cyclomatic Complexity ≤ 11 — COMPLETED

All functions were already at cyclomatic complexity 11 or below.

### Changes

- **Updated** Makefile threshold from 6 to 11.
- **Verified** every function satisfies CC ≤ 11.

---

## Task 3: Test Migration to Go Code Generation — COMPLETED

Migrated hand-written format tests to a code-generated approach driven by a
JSON spec file.

### Deliverables

| File | Purpose |
|------|---------|
| `tools/testgen/main.go` | Code generator that reads `testspec.json` and emits `format_generated_test.go` files. |
| `core/formats/*/testspec.json` | Declarative test specification for each of the 41 format packages. |
| `core/formats/generate.go` | Contains the `go:generate` directive that invokes `testgen`. |
| `core/formats/*/format_generated_test.go` | Generated test files (41 total). |

### Steps completed

1. Created `tools/testgen/main.go` using `text/template` and `go/format`.
2. Created `testspec.json` for all 41 format packages (13 active, 28 skip all).
3. Created `core/formats/generate.go` with `//go:generate go run ../../tools/testgen`.
4. Ran `go generate` — produced 41 `format_generated_test.go` files.
5. Verified all generated tests pass.
6. Marked `core/formats/testing/suite.go` as deprecated in its package doc.
