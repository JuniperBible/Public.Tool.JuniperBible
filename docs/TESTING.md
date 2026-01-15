# Testing Guide

This document describes Juniper Bible's testing strategy, including the testing pyramid, test categories, and best practices.

---

## Testing Pyramid

Juniper Bible follows the testing pyramid approach for comprehensive and efficient testing:

```
          /\
         /  \        End-to-End / Runner Tests
        /    \       (Few, slow, highest confidence)
       /------\
      /        \     Integration Tests
     /          \    (Some, medium speed)
    /------------\
   /              \  Unit Tests
  /                \ (Many, fast, coverage metrics)
 /------------------\
```

### Layer Summary

| Layer | Count | Speed | Purpose | Tools |
|-------|-------|-------|---------|-------|
| **Unit** | Many | Fast (<1s) | Coverage, isolated logic | `go test` |
| **Integration** | Some | Medium (1-30s) | System interactions, real tools | `go test ./tests/integration/...` |
| **E2E / Runner** | Few | Slow (30s+) | Full workflow validation | Runner framework, CI/CD |

---

## Test Categories

### 1. Unit Tests

Unit tests are fast, isolated tests that verify individual functions and modules.

**Location:** `*_test.go` files alongside source code

**Purpose:**
- Verify individual function behavior
- Provide coverage metrics (target: 80%+)
- Fast feedback during development
- Test edge cases and error paths

**Run:**
```bash
# Run all unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test ./internal/formats/swordpure/...
```

**Example:**
```go
func TestParseRef(t *testing.T) {
    ref, err := ParseRef("Gen 1:1")
    if err != nil {
        t.Fatalf("ParseRef failed: %v", err)
    }
    if ref.Book != "Gen" {
        t.Errorf("expected book 'Gen', got %q", ref.Book)
    }
}
```

### 2. Integration Tests

Integration tests verify that components work together correctly with real dependencies.

**Location:** `tests/integration/`

**Purpose:**
- Test real tool interactions (SWORD tools, SQLite, etc.)
- Verify CLI commands work end-to-end
- Test with real file formats and data
- Catch integration issues

**Run:**
```bash
# Run all integration tests
go test ./tests/integration/... -v

# Run specific integration tests
go test ./tests/integration/... -run TestSWORD

# Skip if tools not available
go test ./tests/integration/... -short
```

**Example:**
```go
func TestCapsuleSwordList(t *testing.T) {
    swordPath := createTestSwordInstallation(t)
    defer os.RemoveAll(swordPath)

    stdout, _, exitCode := runCapsuleSword(t, "juniper", "list", swordPath)
    if exitCode != 0 {
        t.Skipf("juniper list command failed")
    }

    if !strings.Contains(stdout, "TestMod") {
        t.Errorf("expected module in output")
    }
}
```

### 3. End-to-End / Runner Tests

Runner tests execute complete workflows in a controlled, reproducible environment.

**Location:** `core/runner/` and CI/CD pipelines

**Purpose:**
- Full workflow validation
- Reproducible execution environment
- Behavioral regression testing
- Hash-based verification

**Run:**
```bash
# Via make
make integration

# Via capsule test
./capsule test testdata/fixtures
```

---

## Coverage Guidelines

### Coverage Targets

| Package Type | Target |
|--------------|--------|
| Core packages (`core/`) | 80%+ |
| Format handlers (`internal/formats/`) | 85%+ |
| CLI functions | Testable core logic: 90%+ |
| Plugins | 70%+ |

### Checking Coverage

```bash
# Overall coverage
go test -cover ./...

# Detailed function coverage
go test -coverprofile=/tmp/coverage.out ./...
go tool cover -func=/tmp/coverage.out

# HTML coverage report
go tool cover -html=/tmp/coverage.out -o coverage.html
```

### CLI Function Testing Strategy

CLI functions that use `os.Args`, `os.Exit`, and stdio are refactored for testability:

```go
// Wrapper function (not directly testable)
func cmdList() {
    if err := runListCmd(os.Args, os.Stdout, os.Stderr); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

// Testable core function (100% testable)
func runListCmd(args []string, stdout, stderr io.Writer) error {
    // All logic here
}
```

This pattern allows:
- 100% coverage of `runListCmd`
- 0% coverage of `cmdList` wrapper (expected)
- Full testing of all code paths via dependency injection

---

## Test Organization

### File Naming

```
package_name.go      # Source file
package_name_test.go # Unit tests for that file
```

### Test Function Naming

```go
func TestFunctionName(t *testing.T)           // Basic test
func TestFunctionName_EdgeCase(t *testing.T)  // Specific scenario
func TestFunctionName_Error(t *testing.T)     // Error path
```

### Table-Driven Tests

```go
func TestParseRef(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Reference
        wantErr bool
    }{
        {"simple", "Gen 1:1", Reference{Book: "Gen"}, false},
        {"invalid", "Invalid", Reference{}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseRef(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseRef() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && got.Book != tt.want.Book {
                t.Errorf("ParseRef() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## Hash-Based Testing

Juniper Bible uses hash-based testing for determinism:

### Principle

> Tests compare **hashes**, not text.

### Why Hashes?

1. **Determinism:** Same input must produce same hash
2. **Efficiency:** Comparing 64 bytes vs. megabytes of output
3. **Regression detection:** Any change is immediately visible
4. **No "looks right" judgment:** Either matches or doesn't

### Golden Files

Golden files store expected hashes:

```
testdata/goldens/
  kjv-genesis.sha256     # Expected hash for KJV Genesis export
  transcript-001.sha256  # Expected transcript hash
```

### Updating Goldens

**Only update when:**
- Intentional behavior change
- Change is understood and documented
- Change has been reviewed

**Never update when:**
- "Tests are failing"
- You don't understand why
- To make CI pass

```bash
# Save new golden
./capsule golden save capsule.tar.xz --run run-1 --out goldens/run-1.sha256

# Check against golden
./capsule golden check capsule.tar.xz --run run-1 --golden goldens/run-1.sha256
```

---

## Running Tests

### Quick Commands

```bash
# All unit tests
make test

# With coverage
make test-coverage

# Integration tests
go test ./tests/integration/... -v

# Specific package
go test ./internal/formats/swordpure/... -v

# Single test
go test -run TestParseRef ./internal/formats/swordpure/
```

### CI Commands

```bash
# What CI runs
go build ./...
go test ./...
go test ./tests/integration/... -short
./capsule test testdata/fixtures
```

### Coverage Report

```bash
# Generate and view coverage
make test-coverage
open coverage.html
```

---

## Test Dependencies

### Required for Unit Tests

- Go 1.21+
- nix-shell (recommended)

### Required for Integration Tests

May require external tools (tests skip gracefully if unavailable):

| Tool | Package | Purpose |
|------|---------|---------|
| diatheke | sword-utils | SWORD module queries |
| mod2imp | sword-utils | Module export |
| sqlite3 | sqlite | Database operations |
| xmllint | libxml2 | XML validation |
| pandoc | pandoc | Document conversion |

### nix-shell Environment

```bash
# Enter reproducible environment with all tools
nix-shell

# Now all tools are available
diatheke -b KJV -k "Gen 1:1"
```

---

## Best Practices

### Do

- Write tests before implementing features
- Use table-driven tests for multiple cases
- Test error paths explicitly
- Keep unit tests fast (<100ms each)
- Use `t.Helper()` in test helpers
- Clean up temp files with `defer`

### Don't

- Compare text output directly (use hashes)
- Depend on execution order
- Use `time.Sleep()` in tests
- Skip tests without reason
- Update goldens without understanding changes

### Test Helpers

```go
func createMockModule(t *testing.T) string {
    t.Helper() // Marks this as helper for better error reporting

    tmpDir, err := os.MkdirTemp("", "test-*")
    if err != nil {
        t.Fatalf("failed to create temp dir: %v", err)
    }
    t.Cleanup(func() { os.RemoveAll(tmpDir) }) // Auto cleanup

    // Setup mock data...
    return tmpDir
}
```

---

## Debugging Test Failures

### 1. Run with Verbose Output

```bash
go test -v -run TestFailingTest ./package/...
```

### 2. Check Specific Output

```bash
go test -v 2>&1 | grep -A5 "FAIL"
```

### 3. Use Test Logging

```go
t.Logf("intermediate value: %v", value)
```

### 4. Compare Hash Mismatches

```bash
# Get expected vs actual
diff <(echo "$expected") <(echo "$actual")
```

### 5. Run Single Test Repeatedly

```bash
for i in {1..10}; do go test -run TestFlaky; done
```

---

## See Also

- [TDD_WORKFLOW.md](TDD_WORKFLOW.md) - Test-driven development guide
- [DEVELOPER_NOTES.md](DEVELOPER_NOTES.md) - Developer setup
- [DESIGN_NOTES.md](DESIGN_NOTES.md) - Architecture decisions
