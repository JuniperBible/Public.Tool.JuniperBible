# Juniper Bible: Byte-for-Byte + Bug-for-Bug Preservation System

> **Note**: This is a historical design document describing the original vision for the project. The current implementation has evolved significantly:
> - External tool dependencies (libsword, pandoc, calibre, etc.) have been replaced with pure Go implementations
> - The NixOS VM execution model described here is no longer the primary approach
> - The project now builds with `CGO_ENABLED=0` and has no external CLI tool requirements
> - See README.md for current architecture details and the actual implementation state

> A full, production-ready system plan including: project charter, architecture, repository layout, plugin contracts, deterministic NixOS VM runner, CLI contract, storage format, schemas, and TDD/CI strategy.

---

## Modular Distribution Architecture

Test data and third-party dependencies are distributed as separate Go modules for flexible licensing and reduced repository size:

### Repository Branches

| Branch | Contents | Size | Purpose |
|--------|----------|------|---------|
| `main` | Production code | ~5 MB | Main project source |
| `development` | Development code | ~5 MB | Active development |
| `test-data` | Sample Bible modules, test fixtures | ~67 MB | Testing data |
| `test-contrib` | Third-party tool references, legacy code | ~16 MB | Third-party licensing |

### Importing as Go Modules

```go
// Import test data
import "github.com/FocuswithJustin/mimicry/test-data/data"

kjvPath := data.SampleModule("kjv")
fixturePath := data.Fixture("sample.txt")

// Import tool references
import "github.com/FocuswithJustin/mimicry/test-contrib/tools"

juniperSrc := tools.JuniperSrc()
licensePath := tools.ToolLicense("pandoc")
```

### Git Worktrees for Development

For parallel development across branches:

```bash
# Create worktrees for test branches
git worktree add -b test-data ../mimicry-worktrees/test-data development
git worktree add -b test-contrib ../mimicry-worktrees/test-contrib development

# List worktrees
git worktree list
```

---

## 0. One-Line Definition

A content-addressed capsule that stores original bytes verbatim and produces deterministic behavior transcripts by running reference tools (libSWORD and others) inside a pinned NixOS VM, with self-check plans that power TDD/CI.

---

## 1. Project Charter

### Goals

- **Byte preservation (absolute):** Any ingested input can be exported back byte-for-byte identical (verified by SHA-256 + BLAKE3)
- **Behavior preservation (authoritative):** For supported formats/tools, produce a deterministic transcript of how the reference tool behaves (bug-for-bug)
- **Plugin architecture:**
  - Format plugins handle detection/ingest/enumeration of input artifacts without modifying bytes
  - Tool plugins run reference tools in a deterministic engine and produce transcripts + derived artifacts
- **Self-check capability (built-in):** "IR↔native" is implemented as RoundTrip Plans that can be executed to produce SelfCheck Reports used as golden baselines in TDD/CI
- **Deterministic execution environment:** Reference tools run inside a NixOS VM built from a pinned flake.lock, offline, pinned locale/timezone, fixed mount paths

### Non-Negotiables

- **Byte-for-byte export always possible:** Identity export re-emits stored original blobs
- **No hidden semantic validation rules:** Correctness is proven via hashes and transcripts from pinned engines, not ad-hoc validators
- **Tool behavior duplication uses the tool:** Bug compatibility means calling the reference toolchain, not reimplementing it

### Out of Scope

- Guaranteed byte-identical regeneration from semantic IR for every format. You may generate derived "native" artifacts, but byte-identity is guaranteed only when exporting stored blobs.

---

## 2. Core Concepts

### Artifact

Immutable record pointing to a content-addressed blob and metadata:

- `kind`: zip, tar, sword-module, sword-conf, sword-data, osis, usfm, pdf, unknown, etc.
- `hashes`: sha256 + optional blake3
- `blob path` in capsule

### Capsule

Portable container (tar.xz or zip) holding:

- `manifest.json`
- `blobs/sha256/<2>/<sha256>`
- Optional `blobs/blake3/<2>/<blake3>.json` pointer files
- Transcripts and derived artifacts stored as blobs, referenced by manifest

### Engine

Reproducible execution environment:

- NixOS VM, pinned flake.lock hash
- Pinned locale/timezone
- No network
- Fixed mount points `/work/in` and `/work/out`

### Transcript

Deterministic JSONL event stream emitted by a tool plugin, where each event references derived payloads by hash.

### RoundTrip Plan

Declarative pipeline definition: ingest → tool runs → exports → checks. Executing a plan yields a SelfCheck Report.

### SelfCheck Report

Machine-readable deterministic pass/fail artifact (also stored as a blob) used as the basis for TDD/CI goldens.

---

## 3. Capsule File Format (On-Disk Layout)

### Container Type

- **Default:** `capsule.tar.xz` (streamable, good compression)
- **Alternate:** `capsule.zip` (random access, Windows-friendly)

### Required Layout

```
capsule/
  manifest.json
  blobs/
    sha256/<first2>/<sha256>                 # blob bytes
    blake3/<first2>/<blake3>.json            # optional pointer to sha256 (tiny file)
```

### Blob Addressing Rules

- **Primary store key:** SHA-256 hex
- **Optional parallel lookup:** BLAKE3 hex via pointer file:
  - `blobs/blake3/ab/<blake3>.json` contents: `{"sha256":"<sha256>"}`
  - This avoids duplicating blob bytes under both hashes

---

## 4. Deterministic Engine (NixOS VM)

### Determinism Requirements

Inside VM runs:

- `TZ=UTC`
- `LC_ALL=C.UTF-8`
- `LANG=C.UTF-8`
- Fixed paths: `/work/in` and `/work/out`
- No network
- Absolute tool paths from Nix store (no PATH drift)
- Avoid timestamps in outputs; if tools emit them, normalize via a normalization plugin step

### Execution Contract

**Host prepares `/work/in` containing:**

- `request.json` (tool run request)
- `tool` (plugin runner executable)
- Staged input files if needed (or a mounted read-only blob staging directory)

**VM writes to `/work/out`:**

- `transcript.jsonl`
- `outputs/` (optional)
- `stdout`, `stderr`

Host ingests these outputs into capsule blobs and records them in manifest.

---

## 5. Plugin Architecture

### Plugin Types

**Format plugin (kind=format):**

- Detect input
- Ingest bytes verbatim
- Enumerate components (optional): e.g., zip → files, sword module → .conf + data files
- Must not mutate original bytes

**Tool plugin (kind=tool):**

- Provides an EngineSpec (which VM/toolchain to use)
- Runs reference tool deterministically in VM
- Produces transcript JSONL and derived artifacts as blobs

### Plugin Packaging

Plugins are executables + manifest:

```
plugins/<name>/
  plugin.json
  bin/<entrypoint>
```

### Plugin Discovery

Default search paths:

- `./plugins/**/plugin.json`
- `$CAPSULE_PLUGIN_PATH/**/plugin.json`

### Plugin Manifest Format

`plugin.json`:

```json
{
  "plugin_id": "tools.libsword",
  "version": "0.1.0",
  "kind": "tool",
  "entrypoint": "bin/libsword-plugin",
  "capabilities": {
    "inputs": ["artifact.kind:sword-module", "artifact.kind:zip", "artifact.kind:dir"],
    "profiles": ["raw", "osis->html", "thml->html", "gbf->html"]
  }
}
```

### IPC Protocol (Language-Agnostic)

Plugins communicate via JSON over stdin/stdout:

- Capsule core invokes plugin with a subcommand and passes JSON

**Tool plugin required commands:**

- `engine-spec` → returns EngineSpec JSON
- `run` with `--request request.json --out <dir>` → writes transcript + outputs

**Format plugin required commands:**

- `detect`
- `ingest`
- `enumerate`

---

## 6. CLI Contract

### Commands

| Command | Description |
|---------|-------------|
| `capsule ingest <path> --out <capsule.tar.xz>` | Uses format plugins for detection and ingestion; stores every file as an Artifact + blob |
| `capsule enumerate <capsule> --artifact <id>` | Optional unpacking/enumeration producing additional verbatim artifacts |
| `capsule run <capsule> --tool <tool-plugin-id> --profile <profile> --inputs <artifact-id>...` | Runs tool plugin in deterministic VM; stores transcript + derived artifacts |
| `capsule export <capsule> --artifact <id> --mode IDENTITY --out <path>` | Writes original bytes back out (byte-for-byte) |
| `capsule verify <capsule>` | Validates schemas and re-hashes blobs |
| `capsule selfcheck <capsule> --plan <plan-id> --targets <artifact-id>...` | Executes RoundTripPlan and writes SelfCheckReport blob |
| `capsule test <fixtures-dir> --goldens <goldens-dir> --plan <plan-id>` | TDD/CI driver: build capsule from fixtures, run selfcheck, compare expected hashes |

---

## 7. Reference Behavior: libSWORD Tool Plugin

### Inputs

SWORD module directory or archive artifact (zip/tar) that contains `mods.d/*.conf` and `modules/...`

### Profiles

- `raw` (as tool returns)
- `osis->html`
- `thml->html`
- `gbf->html`

### Transcript Events

- `ENGINE_INFO`
- `MODULE_DISCOVERED`
- `KEY_ENUM` (keys list stored as blob)
- `ENTRY_RENDERED` (rendered entry stored as blob per key/profile or chunked bundle)
- `WARN`, `ERROR`

### Output Scaling

For large corpora, avoid one file per verse. Prefer:

- Keys list as one blob (text)
- Rendered outputs as chunked JSONL or content-addressed bundles:
  - `render.jsonl` lines: `{module, key, profile, sha256, bytes}`
  - Actual rendered bytes stored as blobs or as compressed chunk blobs

---

## 8. Test Strategy (TDD/CI)

### Goldens Are Hashes, Not Huge Files

A "golden" is typically:

- Transcript blob SHA-256, and/or
- SelfCheckReport blob SHA-256

### Required Test Classes

**Byte Identity Test (universal):**

1. Ingest artifact A
2. Export IDENTITY → bytes B
3. Assert `sha256(B) == sha256(A)` and `blake3(B) == blake3(A)`

**Behavior Identity Test (tool-defined):**

1. Run tool transcript on original
2. Export IDENTITY
3. Run tool transcript on exported
4. Assert transcript hashes match (or per-event payload hashes match)

**Regression Drift Test:**

- Compare new transcript hash to committed golden hash for the pinned engine

### SelfCheck Is the Standard Test Output

All integration tests should call `capsule selfcheck` and compare report hashes.

---

## 9. Schemas

### 9.1 Capsule Manifest JSON Schema

Save as `schemas/capsule.manifest.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.org/capsule.manifest.schema.json",
  "title": "Capsule Manifest",
  "type": "object",
  "additionalProperties": false,
  "required": ["capsule_version", "created_at", "tool", "blobs", "artifacts", "runs"],
  "properties": {
    "capsule_version": { "type": "string", "pattern": "^1\\.[0-9]+\\.[0-9]+$" },
    "created_at": { "type": "string", "format": "date-time" },
    "tool": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "version"],
      "properties": {
        "name": { "type": "string" },
        "version": { "type": "string" },
        "git_rev": { "type": "string" },
        "attributes": { "$ref": "#/$defs/Attributes" }
      }
    },
    "blobs": {
      "type": "object",
      "additionalProperties": false,
      "required": ["by_sha256"],
      "properties": {
        "by_sha256": {
          "type": "object",
          "propertyNames": { "$ref": "#/$defs/Sha256Hex" },
          "additionalProperties": { "$ref": "#/$defs/BlobRecord" }
        },
        "by_blake3": {
          "type": "object",
          "propertyNames": { "$ref": "#/$defs/Blake3Hex" },
          "additionalProperties": { "$ref": "#/$defs/BlobRecord" }
        }
      }
    },
    "artifacts": {
      "type": "object",
      "propertyNames": { "$ref": "#/$defs/ID" },
      "additionalProperties": { "$ref": "#/$defs/Artifact" }
    },
    "runs": {
      "type": "object",
      "propertyNames": { "$ref": "#/$defs/ID" },
      "additionalProperties": { "$ref": "#/$defs/Run" }
    },
    "roundtrip_plans": {
      "type": "object",
      "propertyNames": { "$ref": "#/$defs/ID" },
      "additionalProperties": { "$ref": "#/$defs/RoundTripPlan" }
    },
    "self_checks": {
      "type": "object",
      "propertyNames": { "$ref": "#/$defs/ID" },
      "additionalProperties": { "$ref": "#/$defs/SelfCheckRecord" }
    },
    "exports": {
      "type": "object",
      "propertyNames": { "$ref": "#/$defs/ID" },
      "additionalProperties": { "$ref": "#/$defs/Export" }
    },
    "attributes": { "$ref": "#/$defs/Attributes" }
  }
}
```

*(Full schema definitions omitted for brevity—see original for complete `$defs`)*

### 9.2 Transcript Event JSON Schema

Save as `schemas/transcript.event.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.org/transcript.event.schema.json",
  "title": "Tool Transcript Event",
  "type": "object",
  "additionalProperties": false,
  "required": ["t", "seq"],
  "properties": {
    "t": { "type": "string" },
    "seq": { "type": "integer", "minimum": 0 },
    "engine_id": { "type": "string" },
    "plugin_id": { "type": "string" },
    "plugin_version": { "type": "string" },
    "module": { "type": "string" },
    "key": { "type": "string" },
    "profile": { "type": "string" },
    "sha256": { "$ref": "#/$defs/Sha256Hex" },
    "blake3": { "$ref": "#/$defs/Blake3Hex" },
    "bytes": { "type": "integer", "minimum": 0 },
    "message": { "type": "string" },
    "attributes": { "$ref": "#/$defs/Attributes" }
  }
}
```

### 9.3 SelfCheck Report JSON Schema

Save as `schemas/selfcheck.report.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.org/selfcheck.report.schema.json",
  "title": "SelfCheck Report",
  "type": "object",
  "additionalProperties": false,
  "required": ["report_version", "created_at", "plan_id", "engine", "results", "status"],
  "properties": {
    "report_version": { "type": "string", "pattern": "^1\\.[0-9]+\\.[0-9]+$" },
    "created_at": { "type": "string", "format": "date-time" },
    "plan_id": { "type": "string" },
    "engine": {
      "type": "object",
      "additionalProperties": false,
      "required": ["engine_id", "flake_lock_sha256"],
      "properties": {
        "engine_id": { "type": "string" },
        "flake_lock_sha256": { "type": "string", "pattern": "^[a-f0-9]{64}$" },
        "derivations": { "type": "array", "items": { "type": "string" } },
        "env": { "type": "object" }
      }
    },
    "results": {
      "type": "array",
      "minItems": 1,
      "items": { "$ref": "#/$defs/CheckResult" }
    },
    "status": { "type": "string", "enum": ["pass", "fail"] },
    "attributes": { "type": "object" }
  }
}
```

---

## 10. Nix Flake Skeleton

Save as `nix/flake.nix`:

```nix
{
  description = "Juniper Bible deterministic engine VM (NixOS)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable"; # pinned by flake.lock
  };

  outputs = { self, nixpkgs }:
  let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };
  in {
    packages.${system}.engine-tools = pkgs.buildEnv {
      name = "engine-tools";
      paths = [
        pkgs.bash
        pkgs.coreutils
        pkgs.findutils
        pkgs.gnugrep
        pkgs.gawk
        pkgs.jq
        pkgs.python3
        pkgs.zip
        pkgs.unzip
        pkgs.gnutar
        pkgs.xz
        pkgs.openssl
        pkgs.sword  # Reference tool
      ];
    };

    nixosConfigurations.engine-vm = nixpkgs.lib.nixosSystem {
      inherit system;
      modules = [
        ({ pkgs, ... }: {
          networking.networkmanager.enable = false;
          networking.useDHCP = false;
          networking.firewall.enable = true;

          time.timeZone = "UTC";
          i18n.defaultLocale = "C.UTF-8";

          environment.variables = {
            TZ = "UTC";
            LC_ALL = "C.UTF-8";
            LANG = "C.UTF-8";
          };

          users.users.runner = {
            isNormalUser = true;
            extraGroups = [ "wheel" ];
            password = "";
          };
          security.sudo.wheelNeedsPassword = false;

          environment.systemPackages = [
            self.packages.${system}.engine-tools
          ];

          # Host provides these mounts (9p shown; virtiofs also fine)
          fileSystems."/work/in" = {
            device = "workin";
            fsType = "9p";
            options = [ "trans=virtio" "version=9p2000.L" "msize=104857600" "cache=loose" ];
          };
          fileSystems."/work/out" = {
            device = "workout";
            fsType = "9p";
            options = [ "trans=virtio" "version=9p2000.L" "msize=104857600" "cache=loose" ];
          };

          systemd.services.capsule-runner = {
            description = "Juniper Bible Runner";
            wantedBy = [ "multi-user.target" ];
            serviceConfig = {
              Type = "simple";
              User = "runner";
              WorkingDirectory = "/work/in";
              ExecStart = "/bin/sh /work/in/runner.sh";
              Restart = "no";
            };
          };

          system.stateVersion = "24.11";
        })
      ];
    };
  };
}
```

### Runner Script

Placed by host into `/work/in/runner.sh`:

```bash
#!/bin/sh
set -eu

export TZ=UTC
export LC_ALL=C.UTF-8
export LANG=C.UTF-8
umask 022

IN=/work/in
OUT=/work/out

mkdir -p "$OUT"

REQ="$IN/request.json"
[ -f "$REQ" ] || { echo "missing request.json" >&2; exit 2; }

TOOL="$IN/tool"
[ -x "$TOOL" ] || { echo "missing tool executable at /work/in/tool" >&2; exit 2; }

"$TOOL" run --request "$REQ" --out "$OUT"
```

---

## 11. Built-in RoundTrip Plans

Include at least these plans in new capsules (or ship as defaults in the CLI):

### plan_identity_bytes

- **Steps:** EXPORT identity
- **Checks:** BYTE_EQUAL between original artifact and export result

### plan_sword_libsword_behavior_identity

- **Steps:**
  1. RUN_TOOL libsword on original input (profile osis->html)
  2. EXPORT identity
  3. RUN_TOOL libsword on exported bytes (same profile)
- **Checks:** TRANSCRIPT_EQUAL between run_a and run_b

This is your "IR↔native self-check": your IR base is the capsule, your "to native and back" is identity export, and behavior equivalence is proven via pinned tool transcripts.

---

## 12. Security/Integrity Requirements

- All blobs verified by SHA-256 during `verify` and before export
- VM execution is offline; no host secrets mounted
- Treat capsules as untrusted: never auto-execute plugin binaries from inside capsules
- Plugins are installed from trusted paths; maintain allowlist of plugin IDs in production

---

## 13. Implementation Milestones

1. Blob store + capsule pack/unpack + manifest generation + verify
2. Format plugins: generic-file, dir, zip, tar, sword enumerator
3. VM runner (host-side) + Nix flake engine build + deterministic run harness
4. Tool plugin: tools.libsword transcript generation
5. Selfcheck engine: execute RoundTripPlan → SelfCheckReport blob
6. CI harness: fixtures + golden transcript hash comparisons

---

## 14. Definition of Done

A developer delivers:

- `capsule ingest` produces capsule with correct blob addressing + manifest
- `capsule export --mode IDENTITY` produces byte-identical output
- `capsule run --tool tools.libsword` produces deterministic transcript
- `capsule selfcheck --plan plan_sword_libsword_behavior_identity` produces a stable SelfCheckReport, and CI compares its hash to goldens

That's the whole system.
