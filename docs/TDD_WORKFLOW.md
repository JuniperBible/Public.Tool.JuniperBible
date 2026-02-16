# TDD Workflow for Contributors

This guide explains how to work with Juniper Bible's test-driven development approach.

---

## Core Principle

> Tests compare **hashes**, not text.

If a hash changes, you must explain why. No exceptions.

---

## Test-First Requirements

Before implementing any feature:

1. **Write the test first** - Define expected hashes before writing code
2. **Run the test** - Verify it fails for the right reason
3. **Implement the feature** - Make the test pass
4. **Verify hashes are stable** - Run the test multiple times

### Example Workflow

```bash
# 1. Write test with expected hash (will fail initially)
func TestMyFeature(t *testing.T) {
    expectedHash := "abc123..."  // Expected output hash

    // Run the operation
    result := myFeature(input)

    // Compare hashes
    if result.Hash != expectedHash {
        t.Errorf("hash mismatch: got %s, want %s", result.Hash, expectedHash)
    }
}

# 2. Run the test (should fail)
go test -run TestMyFeature -v

# 3. Implement the feature

# 4. Run the test again (should pass)
go test -run TestMyFeature -v

# 5. Run multiple times to verify determinism
for i in {1..5}; do go test -run TestMyFeature; done
```

---

## Hash Comparison Testing

### What Gets Hashed

- **Blob content** - SHA-256 of raw bytes
- **Transcripts** - SHA-256 of transcript.jsonl content
- **SelfCheck reports** - SHA-256 of report JSON

### Hash Comparison Pattern

```go
func TestBlobHash(t *testing.T) {
    // Create test data
    data := []byte("test content")

    // Store in CAS
    store, _ := cas.NewStore(tempDir)
    hash, _ := store.Store(data)

    // Verify hash matches expected
    expected := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
    if hash != expected {
        t.Errorf("hash mismatch: got %s, want %s", hash, expected)
    }
}
```

### Transcript Hash Testing

```go
func TestTranscriptHash(t *testing.T) {
    // Run tool and capture transcript
    transcript := runTool(input)

    // Hash the transcript
    hash := sha256.Sum256(transcript)
    hashHex := hex.EncodeToString(hash[:])

    // Compare to golden
    golden := "expected_transcript_hash..."
    if hashHex != golden {
        t.Errorf("transcript hash mismatch: got %s, want %s", hashHex, golden)
    }
}
```

---

## Golden File Management

### What Are Goldens?

Goldens are stored hashes that represent known-good outputs. They serve as the baseline for regression testing.

### Golden Storage

```
testdata/
  goldens/
    sample.txt.sha256      # Expected hash for sample.txt
    transcript.sha256      # Expected transcript hash
    report.sha256          # Expected selfcheck report hash
```

### Golden Format

Each golden file contains a single SHA-256 hash:

```
9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
```

### Using Goldens in Tests

```go
func TestAgainstGolden(t *testing.T) {
    // Read golden hash
    goldenBytes, _ := os.ReadFile("testdata/goldens/sample.txt.sha256")
    golden := strings.TrimSpace(string(goldenBytes))

    // Compute actual hash
    data, _ := os.ReadFile("testdata/fixtures/inputs/sample.txt")
    hash := sha256.Sum256(data)
    actual := hex.EncodeToString(hash[:])

    // Compare
    if actual != golden {
        t.Errorf("golden mismatch: got %s, want %s", actual, golden)
    }
}
```

---

## How to Update Goldens

### When to Update

Update goldens **only** when:

1. You intentionally changed behavior
2. You can explain exactly what changed and why
3. The change has been reviewed

### Never Update Goldens When

- Tests are "just failing"
- You don't understand why the hash changed
- The change was unintentional

### Update Process

```bash
# 1. Verify the new behavior is correct
./capsule selfcheck /path/to/capsule

# 2. Examine the difference
./capsule compare run-old run-new

# 3. If correct, save the new golden
./capsule golden save /path/to/capsule --run run-id --out testdata/goldens/

# 4. Document the change in your commit message
git commit -m "Update golden: <explanation of what changed and why>"
```

### Using the CLI

```bash
# Save a new golden hash
./capsule golden save /path/to/capsule.tar.xz --run run-1 --out testdata/goldens/run-1.sha256

# Check against existing golden
./capsule golden check /path/to/capsule.tar.xz --run run-1 --golden testdata/goldens/run-1.sha256
```

---

## Test Categories

### Unit Tests

Test individual functions in isolation:

```bash
go test ./core/cas/...
go test ./core/capsule/...
```

### Integration Tests

Test full workflows:

```bash
go test ./core/selfcheck/...
```

### CLI Tests

Test the complete CLI:

```bash
./capsule test testdata/fixtures
```

### Sample Data Integration Tests

Test against real Bible modules from contrib/sample-data/:

```bash
go test ./tests/integration/... -v
```

These tests verify:

- Capsule existence and integrity
- Artifact hash regression (detects unintended changes)
- Module data structure (conf files, data directories)
- Round-trip fidelity (100% byte preservation)

---

## Debugging Test Failures

### Step 1: Identify the Changed Hash

```bash
go test -v 2>&1 | grep "mismatch"
```

### Step 2: Find What Produced It

Look at the test output to identify which step failed:

```
hash mismatch: got abc123... want def456...
    at: TestPackAndUnpack
    step: comparing unpacked blob
```

### Step 3: Compare Artifacts

```bash
# For transcripts
./capsule compare run-a run-b

# For blobs
diff <(xxd /tmp/actual.bin) <(xxd /tmp/expected.bin)
```

### Step 4: Fix the Root Cause

- If environment issue: fix the harness
- If code bug: fix the code
- If intentional change: update golden with explanation

---

## Running the Full Test Suite

```bash
# Run all Go tests
go test ./... -v

# Run CLI tests
./capsule test testdata/fixtures

# Run everything (what CI does)
go build ./... && go test ./... && ./capsule test testdata/fixtures
```

---

## CI Integration

### What CI Checks

1. All Go tests pass
2. All code builds
3. Plugins build successfully
4. Round-trip verification passes
5. Golden hashes match

### CI Workflow

```yaml

- name: Run unit tests
  run: go test ./... -v

- name: Build CLI and plugins
  run: |
    go build -o capsule ./cmd/capsule
    for p in file zip dir tar sword osis usfm; do
      go build -o plugins/format/$p/format-$p ./plugins/format/$p
    done
    go build -o plugins/tool/libsword/tool-libsword ./plugins/tool/libsword

- name: Run capsule test
  run: ./capsule test testdata/fixtures

- name: Verify round-trip
  run: |
    ./capsule ingest testdata/fixtures/inputs/sample.txt --out /tmp/test.capsule.tar.xz
    ./capsule verify /tmp/test.capsule.tar.xz
    ./capsule selfcheck /tmp/test.capsule.tar.xz
    ./capsule export /tmp/test.capsule.tar.xz --artifact sample --out /tmp/exported.txt
    diff testdata/fixtures/inputs/sample.txt /tmp/exported.txt
```

---

## Common Mistakes

### Mistake: Comparing Text Instead of Hashes

```go
// Wrong
if string(output) != expectedText {
    t.Error("text mismatch")
}

// Right
if hashOf(output) != expectedHash {
    t.Error("hash mismatch")
}
```

### Mistake: Non-Deterministic Tests

```go
// Wrong - timestamps cause different hashes
output := fmt.Sprintf("created at %s", time.Now())

// Right - use deterministic values
output := fmt.Sprintf("created at %s", "2026-01-01T00:00:00Z")
```

### Mistake: Updating Goldens Without Explanation

```bash
# Wrong
cp new_hash.txt golden.txt
git commit -m "fix tests"

# Right
git commit -m "Update golden: Changed X to Y because Z"
```

---

## Summary

1. **Write tests first** with expected hashes
2. **Compare hashes**, not text
3. **Update goldens deliberately** with explanations
4. **Fix the harness** when tests are flaky
5. **The transcript is the spec** - trust it over documentation
