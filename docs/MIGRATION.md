# Makefile Target Migration Guide

This document maps old Makefile target names to their new canonical equivalents.

**Old target names have been removed.** If you use an old name, Make will fail with "No rule to make target". Update your scripts and habits to use the new names.

## Why the Change

The Makefile was restructured to follow a consistent naming convention:

- `build-*` for build targets
- `test-*` for test targets
- `docs-*` for documentation targets
- `dist-*` for distribution targets
- `dev-*` for development utilities
- `fmt`, `lint`, `clean`, `help` for core utilities

This makes targets discoverable by naming alone and reduces guesswork.

## Migration Table

### Build Targets

| Old Command | New Command | Notes |
|-------------|-------------|-------|
| `make web` | `make build-web` | Currently disabled; use `capsule web` |
| `make api` | `make build-api` | Currently disabled; use `capsule api` |
| `make plugins` | `make build-plugins` | |
| `make plugins-format` | `make build-formats` | |
| `make plugins-tool` | `make build-tools` | |
| `make juniper` | `make build-juniper` | |
| `make juniper-meta` | `make build-meta` | |
| `make repoman` | `make build-repoman` | |
| `make standalone` | `make build-standalone` | |
| `make juniper-legacy` | `make build-legacy` | |
| `make juniper-legacy-extract` | `make build-legacy-extract` | |
| `make juniper-legacy-all` | `make build-legacy-all` | |
| `make format-cmd` | `make build-formats` | |
| `make tools-cmd` | `make build-tools` | |

### Test Targets

| Old Command | New Command | Notes |
|-------------|-------------|-------|
| `make integration` | `make test-integration` | |
| `make juniper-test` | `make test-juniper` | |
| `make juniper-test-base` | `make test-juniper-base` | |
| `make juniper-test-samples` | `make test-juniper-samples` | |
| `make juniper-test-comprehensive` | `make test-juniper-home` | Tests all `~/.sword` modules |
| `make juniper-test-all` | `make test-juniper-all` | |

### Utility Targets

| Old Command | New Command | Notes |
|-------------|-------------|-------|
| `make list-plugins` | `make plugins-list` | |
| `make nix-test-shell` | `make dev-nix-shell` | |

## Unchanged Targets

These targets keep their original names (they already follow the convention or are common shortcuts):

- `make build` - Build main CLI
- `make test` - Quick unit tests
- `make all` - Build everything + run tests
- `make clean` - Remove artifacts
- `make help` - Show help
- `make fmt` - Format code
- `make lint` - Run linting
- `make docs` - Generate documentation
- `make docs-verify` - Verify docs are current
- `make dist` - Build all distributions
- `make dist-*` - Distribution variants

## CI/CD Updates

Update your CI pipelines to use the new names:

```yaml
# Before

- run: make plugins
- run: make juniper-test-comprehensive

# After

- run: make build-plugins
- run: make test-juniper-home
```

## Questions?

Run `make help` for the complete list of available targets.
