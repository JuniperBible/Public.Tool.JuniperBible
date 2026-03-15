# Juniper Bible

**Byte-for-byte preservation. Bug-for-bug behavior. Deterministic forever.**

**Juniper Bible** (a.k.a. **JuniperBible**) is a **Bible Reader, Retrieval, Archival, Transmittal, and Conversion** system for Bible and religious text formats (SWORD, OSIS, USFM, GenBook, archives, and more).

It is **not a normal converter**.

It is intended to be a **forensic harness** that captures how reference tools *actually behave* (including quirks) and used that behavior for **automated, hash-based tests**.

> **Rule:** If our code disagrees with the reference tool, **our code is wrong**.

Juniper Bible exists to make these workflows portable, testable, and reproducible—without rewriting history or "fixing" the data.

Juniper Bible was originally created and is currently maintained by Justin Michael Weeks to power content and format workflows for focuswithjustin.com. While building that pipeline, it became clear that although many excellent tools and libraries exist, the ecosystem often falls short in one or more practical areas: portability, clear documentation, long-term maintainability, modern reproducible builds, or truly open availability. Some projects depend on aging stacks, some are hard to deploy broadly, and others are proprietary or no longer actively maintained. In a few cases, this project exists for the most honorable engineering reason of all: I wanted to scratch an itch in Go and offer another solid option.

And in that work, we are reminded of something deeper. The talents we have are not self-made; we can create nothing except from the materials the Creator has given us. They are gifts. In applying our craft, we are humbled by the drive to explore and create that seems embedded in who we are, as though it were part of what it means to be made in the image of our Creator. Humanity keeps trying to close the fracture we caused, wandering back toward love and goodness through sin, repair, and creation. Somewhere in the process, we realize how small we are and how immense the mercy is. Existence itself is a gift beyond measuring, and it is awe-inspiring that the Author of existence would still provide us the means to understand, to seek, and to approach salvation and achieve everlasting life, which we so greatly do not deserve. As part of the glorious story of deliverance and salvation, we as makers and heirs of creation get to participate in the glorious textual transmission of the word of our Lord and Saviour.  We do this through the witness of a sinful creature, saved by an unalterable and eternal word.  We do so, because it is the will of the father, that all humanity who ask shall be saved by and through his Word.

---

## Quick Start

```bash
# Build
make build

# Run tests
make test

# Convert SWORD modules to capsule format
juniper capsule create --module KJV

# List available formats
juniper formats list
```

## Documentation

Full documentation has been consolidated in the [Private.Org.JuniperBible.Doc](https://github.com/JuniperBible/Private.Org.JuniperBible.Doc) repository:

| Document | Location in Org Docs |
|----------|---------------------|
| Architecture | `docs/architecture/juniper.md` |
| Charter | `docs/charters/juniper-charter.md` |
| Quick Start | `docs/juniper/QUICK_START.md` |
| File Formats | `docs/juniper/FILE_FORMATS.md` |
| Plugin Development | `docs/juniper/PLUGIN_DEVELOPMENT.md` |
| Plugin IPC Protocol | `docs/juniper/plugins/IPC_PROTOCOL.md` |
| Plugin SDK | `docs/juniper/plugins/SDK_README.md` |
| SQLite Backend | `docs/juniper/sqlite/` |
| Build Modes | `docs/juniper/BUILD_MODES.md` |
| Versification | `docs/juniper/VERSIFICATION.md` |
| Developer Notes | `docs/juniper/DEVELOPER_NOTES.md` |
| Scripts | `docs/juniper/SCRIPTS.md` |
| Changelog | `docs/juniper/CHANGELOG.md` |
| Testing | `docs/testing/juniper.md` |
| Security | `docs/security/juniper.md` |
| WebSocket Security | `docs/security/juniper-websocket.md` |
| API (REST) | `docs/api/juniper-rest.md` |
| API (CLI) | `docs/api/juniper-cli.md` |
| API (OpenAPI) | `docs/api/juniper-openapi/` |
| Archive | `docs/archive/juniper-attic/` |

## License

See: [`LICENSE`](LICENSE)
