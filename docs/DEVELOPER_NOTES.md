# Developer Guide

## What You're Actually Building

You are not building a Bible converter.

You are building a **forensic harness** that:

- Stores files exactly as they came in
- Runs the real conversion tools in a locked box
- Records what those tools actually do
- Turns that record into tests

If our code disagrees with the reference behavior, **our code is wrong**.

---

## The Three Rules You Must Not Break

### 1. Never Touch Original Bytes

- When a file is ingested, it is stored verbatim
- Exporting "back to native" means re-emitting those exact bytes
- You never "fix," "normalize," or "clean" input files
- **If you feel the urge to do that, stop**

### 2. Truth Comes from Transcripts, Not Assumptions

- The transcript produced by the reference tool is the spec
- If libSWORD outputs something surprising, that surprise is the rule
- Docs, intuition, and "what seems right" do not matter

### 3. Determinism Is Mandatory

- Same input + same engine must produce the same hashes
- If something is flaky, it's a bug in the environment or harness
- Fix the harness, don't loosen the tests

---

## Working with Modular Branches

Test data and third-party packages are in separate branches for flexible licensing:

### Branch Structure

| Branch | Purpose | Import Path |
|--------|---------|-------------|
| `test-data` | Sample Bibles, fixtures | `github.com/FocuswithJustin/mimicry/test-data/data` |
| `test-contrib` | Tool references, legacy code | `github.com/FocuswithJustin/mimicry/test-contrib/tools` |

### Using Git Worktrees

For parallel development across branches:

```bash
# Create worktrees
git worktree add ../mimicry-worktrees/test-data test-data
git worktree add ../mimicry-worktrees/test-contrib test-contrib

# Work in each worktree independently
cd ../mimicry-worktrees/test-data
# make changes, commit, push

# List worktrees
git worktree list

# Remove a worktree
git worktree remove ../mimicry-worktrees/test-data
```

### Local Development with Replace Directives

For local testing before pushing:

```go
// go.mod
replace github.com/FocuswithJustin/mimicry/test-data => ../mimicry-worktrees/test-data
replace github.com/FocuswithJustin/mimicry/test-contrib => ../mimicry-worktrees/test-contrib
```

---

## Plugin SDK

The Plugin SDK provides a simplified development path for plugins by handling common infrastructure tasks automatically.

### What the SDK Provides

- **Simplified Development**: Focus on plugin logic rather than boilerplate
- **Command Routing**: Automatic dispatch to appropriate plugin methods
- **Error Handling**: Standardized error reporting and recovery
- **Lifecycle Management**: Initialization, cleanup, and state management

### SDK Adoption

Currently **87 plugins** have SDK versions, allowing developers to choose between:

- Direct plugin implementation for maximum control
- SDK-based implementation for faster development

### Building with SDK Support

```bash
# Build with SDK mode enabled
go build -tags=sdk
```

### References

- SDK implementation: `plugins/sdk/`
- Development guide: `docs/PLUGIN_DEVELOPMENT.md`

---

## How to Work Day-to-Day

### When Adding Support for a Format

Write a format plugin that only:

- Detects the format
- Ingests bytes
- Optionally enumerates components

Do not parse or reinterpret content unless explicitly told.

**Success =** bytes are preserved and addressable by hash.

### When Duplicating a Tool's Behavior

1. Write a tool plugin
2. Run the tool inside the deterministic VM
3. Capture:
   - `transcript.jsonl`
   - stdout/stderr (as blobs)
   - any outputs (as blobs)
4. Hash everything

**Success =** running the tool twice produces identical transcript hashes.

### When Writing Conversion Logic

- Assume your code is wrong until proven otherwise
- Compare your output against the transcript
- Fix one divergence at a time
- If tests fail, do not update goldens casually

---

## How Tests Work

Tests do not compare text files.

Tests compare **hashes**:

- Blob hashes
- Transcript hashes
- Self-check report hashes

If a hash changes, you explain why.

---

## How to Debug Failures

When a test fails:

1. Look at which hash changed
2. Identify which step produced it
3. Inspect the transcript event for that step
4. Fix the code or environment until hashes match again

No guessing.

---

## What "Done" Looks Like

A feature is done only if:

- Its outputs are stored as blobs
- Its behavior is recorded in a transcript
- A self-check plan verifies it
- CI compares its hashes to goldens

If any of those are missing, the feature is incomplete.

---

## What Not to Do

- Don't regenerate native files and expect byte equality
- Don't hardcode ordering, encoding, or formatting assumptions
- Don't add "helpful fixes" to inputs
- Don't loosen tests because they're annoying

---

## Why This Makes Your Life Easier

Once this system exists:

- You don't argue about edge cases
- You don't chase mysterious regressions
- You don't guess how a tool behaves

You run it, record it, and match it.

---

## The One Sentence to Remember

> If the transcript says it behaves that way, that's how it behaves.

That's the contract you're coding to.
