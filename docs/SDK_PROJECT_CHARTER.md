# JuniperBible Plugin SDK Migration - Project Charter

## Project Name
JuniperBible Plugin SDK Migration

## Project Summary
Introduce a Plugin SDK layer between existing IPC protocol and plugin implementations to eliminate ~68% of duplicated boilerplate code across 94 plugins/handlers while maintaining full backward compatibility.

## Problem Statement
The current plugin architecture requires each of 94 plugins to implement repetitive boilerplate for:
- Command dispatch (~30 lines per plugin)
- Detection logic (~40 lines per plugin)
- Ingest/blob storage (~50 lines per plugin)
- IR read/write (~30 lines per plugin)
- Error handling (~20 lines per plugin)

This results in ~56% code duplication (~31,450 lines of similar code), increasing maintenance burden and bug surface area.

## Objectives
1. Reduce plugin code by 68% (~21,650 lines eliminated, net ~800 SDK added)
2. Maintain 100% backward compatibility with existing IPC protocol
3. Create clear, documented SDK API for plugin authors
4. Enable new plugin creation in <10 minutes

## Success Criteria
- All 94 plugins pass parity tests (identical behavior before/after)
- Line count reduction ≥50% per plugin
- Zero breaking changes to IPC wire protocol
- SDK API documented with examples
- **Each PR keeps CI green; no PR may merge with failing parity harness for its touched plugins**

## Scope

### In Scope
- SDK package creation (`plugins/sdk/`)
- Migration of 42 format plugins
- Migration of 10 tool plugins
- Migration of 41 internal handlers (via adapter; see Phase 4)
- IPC protocol documentation
- Golden tests for backward compatibility
- Optional codegen scaffolding tool

### Out of Scope
- Changes to IPC wire protocol
- New plugin features (separate project)
- Performance optimizations (separate project)
- UI/CLI changes

## Stakeholders
- Plugin authors (primary beneficiary)
- Core maintainers (SDK ownership)
- CI/CD systems (must remain green throughout)

## Constraints
- IPC protocol must not change
- Existing tests must pass unchanged
- **No runtime dependency on code generation**: Generated files are committed to the repo; runtime does not import the generator module
- **CI Gate**: `go test ./...` must pass from a clean checkout without running `make gen`
- Migration PRs must not modify SDK API

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SDK API instability during migration | Merge conflicts, rework | Freeze SDK in migration PRs |
| Subtle behavior changes | Silent bugs | Parity test harness with structural diff |
| Golden test flakiness | CI failures | Normalization of timestamps/paths/UUIDs/OS paths |
| Internal handler breakage | Core functionality loss | Wrap-first adapter strategy |

## Non-Negotiable Rules
1. **IPC is the wire protocol** - transport, serialization, framing. Keep it boring.
2. **SDK is the developer API** - types, helpers, ergonomics. Thin wrapper over IPC.
3. **Codegen is scaffolding only** - one-time generation. Runtime must not require `go generate`.
4. **One semantic source of truth** - `plugins/ipc/protocol.go` defines canonical types. SDK re-exports or embeds these types directly; it never defines parallel structs.
5. **Plugin authors never construct IPC envelopes directly** - they return `ipc.*Result` or SDK wrappers that produce them.

## Governance

### SDK API Review
Any change to `plugins/sdk/` requires approval from the "SDK owner" (or CODEOWNERS entry).
This makes the "freeze SDK during migrations" rule enforceable.

## Timeline (PR-based)
1. PR 1: IPC PROTOCOL.md + golden tests
2. PR 2: SDK runtime + txt migration (reference implementation)
3. PR 3-6: Plugin batch migrations (11 parallel workers)
4. PR 7: Internal handler migrations
5. PR 8: Optional codegen (after SDK stabilizes)
6. PR 9: Final documentation

## Estimated Impact

| Component | Current | After SDK | Reduction |
|-----------|---------|-----------|-----------|
| 42 format plugins | ~19,250 lines | ~4,200 lines | 78% (~15,050 saved) |
| 41 internal handlers | ~9,000 lines | ~4,100 lines | 54% (~4,900 saved) |
| 10 tool plugins | ~3,200 lines | ~1,500 lines | 53% (~1,700 saved) |
| **Total** | ~31,450 lines | ~9,800 lines + ~800 SDK | **~68% (~21,650 net saved)** |

## Approval
- [ ] Project sponsor approval
- [ ] Technical lead review
- [ ] CI/CD verification plan confirmed

## Related Documents
- [Full Implementation Plan](../.claude/plans/frolicking-shimmying-comet.md)
- [Plugin Development Guide](PLUGIN_DEVELOPMENT.md)
- [IPC Protocol Specification](../plugins/ipc/PROTOCOL.md) (to be created)
