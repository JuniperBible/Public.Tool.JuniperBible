# System End State

## Executive Summary

We are building a **reproducible conversion lab**, not "just a converter."

The system:

- Stores original files byte-for-byte
- Runs reference tools in a deterministic VM
- Records exactly how those tools behave
- Uses that record as automated tests

If our code disagrees with the reference behavior, tests fail. No arguing about edge cases.

---

## Non-Negotiable Rules

### 1. Never Mutate Original Bytes

- Inputs are stored once in content-addressed storage
- Exporting "back to native" means re-emitting those exact bytes

### 2. All Correctness Comes from Transcripts

- If libSWORD says a verse renders a certain way, that is the truth
- Our code must match transcripts, not expectations or docs

### 3. No Hidden Logic

- Any normalization or cleanup must be a visible plugin step
- If it affects output, it's recorded and hashed

### 4. Determinism or It's a Bug

- Same input + same engine = same hashes
- If it's flaky, we fix the environment, not the tests

---

## Implementation Phases

### Phase 1: Foundation (No Conversion Yet)

- Content-addressed blob store (SHA-256 primary, BLAKE3 secondary)
- Capsule pack/unpack
- Manifest generation + schema validation
- Identity export (prove byte-for-byte round-trip)

**Success criterion:** `ingest -> export(ID)` produces identical hashes for all test files.

### Phase 2: Deterministic Execution Harness

- Nix flake that builds the engine VM
- Host-side VM runner that:
  - Mounts `/work/in` and `/work/out`
  - Executes a tool plugin
  - Captures transcript + outputs
- Enforce TZ/LC_ALL/LANG, no network

**Success criterion:** Running the same tool twice produces identical transcript hashes.

### Phase 3: Plugin System

- Plugin loader + contract enforcement
- Format plugins: file, dir, zip, tar, SWORD enumerator (conf + data files)
- Tool plugin: `tools.libsword` (emits transcript JSONL and content blobs)

**Success criterion:** libSWORD output is fully captured and replayable.

### Phase 4: Self-Check Engine (The Payoff)

- Implement RoundTripPlan execution
- Implement SelfCheckReport generation
- Add default plans: `identity-bytes`, `libsword-behavior-identity`

**Success criterion:** `capsule selfcheck` produces a stable pass/fail artifact usable in CI.

---

## How to Think While Coding

### Treat the VM Like a Compiler Toolchain

- Pin it
- Version it
- Never "just update" it

### Treat Transcripts Like Gold

- They are the oracle
- They are the spec
- They replace documentation

### Treat Conversion Code as Guilty Until Proven Innocent

- Every change must pass transcript comparison
- If behavior changes, you must explain why and update goldens deliberately

---

## Definition of Done

A feature is done when:

- Its outputs are blobs with hashes
- Its behavior is captured in a transcript
- There is a self-check plan that verifies it
- CI compares hashes, not text diffs

If any of those are missing, it's not done.

---

## Common Traps to Avoid

- Do not parse and re-emit native formats unless explicitly required
- Do not assume ordering, encoding, or canonicalization
- Do not bake normalization into core logic
- Do not rely on "looks correct" testing

---

## Why This Architecture Matters for Velocity

Once this is in place:

- Adding a new format = write a plugin, not rewrite the system
- Reverse-engineering a tool = run it, record it, match it
- Bugs become diffs with hashes and exact locations
- Refactors are safe because behavior is locked down

This reduces long-term risk and speeds up development after the initial setup.

---

## The One Sentence to Remember

> We don't guess what conversion tools do—we measure them, freeze them, and make our code match.

That's the standard this system enforces.
