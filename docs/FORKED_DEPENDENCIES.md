# Forked Dependencies Documentation

This document describes the forked dependencies used in the Juniper Bible project, explains why each fork was necessary, and outlines the process for keeping them in sync with upstream.

## Table of Contents

- [Overview](#overview)
- [Forked Dependencies](#forked-dependencies)
  - [alecthomas/kong](#alecthomaskong)
  - [alecthomas/participle](#alecthomesparticiple)
- [Security Update Process](#security-update-process)
- [Monitoring Upstream Changes](#monitoring-upstream-changes)
- [Fork Maintenance Schedule](#fork-maintenance-schedule)

## Overview

Juniper Bible uses forked versions of two dependencies to support specific functionality or bug fixes not yet available in upstream. While forks provide necessary features, they introduce a security maintenance burden as they may miss upstream security updates.

**Security Notice**: All forked dependencies must be regularly synchronized with upstream to incorporate security patches and bug fixes.

## Forked Dependencies

### alecthomas/kong

**Upstream Repository**: https://github.com/alecthomas/kong
**Fork Repository**: https://github.com/FocuswithJustin/kong
**Version**: v1.13.0
**Last Sync Date**: 2026-01-09

#### Why Forked

The kong CLI framework fork includes the following modifications:

1. **Custom Resolver Support**: Enhanced resolver interface for dynamic configuration loading
2. **Environment Variable Handling**: Improved environment variable prefix handling for nested structures
3. **Help Text Formatting**: Customized help text rendering for better UX in Juniper Bible CLI

#### Differences from Upstream

- **Files Modified**:
  - `resolver.go`: Added support for custom configuration sources
  - `context.go`: Enhanced context handling for nested commands
  - `help.go`: Customized help text formatting

- **Behavior Changes**:
  - Supports loading configuration from multiple sources (env, file, defaults)
  - Better integration with structured config types (api.Config, web.Config)
  - Preserves all upstream functionality and tests

#### Keeping in Sync

1. **Monitor upstream releases**: https://github.com/alecthomas/kong/releases
2. **Check for security advisories**: https://github.com/alecthomas/kong/security/advisories
3. **Merge upstream changes monthly**:
   ```bash
   cd $GOPATH/src/github.com/FocuswithJustin/kong
   git remote add upstream https://github.com/alecthomas/kong.git
   git fetch upstream
   git merge upstream/master
   # Resolve conflicts, ensuring custom changes are preserved
   git push origin master
   ```
4. **Update go.mod version** if fork changes
5. **Run tests** to ensure compatibility: `make test`

#### Upstream Contribution Status

**Status**: Not contributed upstream (waiting for design review)

**Reason**: The custom resolver changes are specific to Juniper Bible's configuration model. Upstream maintainer feedback needed before proposing a pull request.

**Future Plan**:
- Extract generic resolver interface improvements
- Propose upstream PR with broader use case
- If accepted, remove fork and use upstream version

---

### alecthomas/participle

**Upstream Repository**: https://github.com/alecthomas/participle
**Fork Repository**: https://github.com/FocuswithJustin/participle
**Version**: v2.1.4 (upstream equivalent: v2.1.1+)
**Last Sync Date**: 2026-01-09

#### Why Forked

The participle parser library fork includes the following modifications:

1. **USFM Parser Support**: Custom lexer extensions for USFM marker parsing
2. **Position Tracking**: Enhanced source position tracking for error reporting
3. **Unicode Handling**: Improved UTF-8 byte-order mark (BOM) handling

#### Differences from Upstream

- **Files Modified**:
  - `lexer/text_scanner.go`: Enhanced Unicode BOM handling
  - `lexer/peek.go`: Added custom token lookahead for USFM
  - `parse.go`: Improved error position reporting

- **Behavior Changes**:
  - Handles USFM markers like `\id`, `\c`, `\v` correctly
  - Better error messages with line/column positions
  - UTF-8 BOM detection and stripping
  - Preserves all upstream functionality and tests

#### Keeping in Sync

1. **Monitor upstream releases**: https://github.com/alecthomas/participle/releases
2. **Check for security advisories**: https://github.com/alecthomas/participle/security/advisories
3. **Merge upstream changes monthly**:
   ```bash
   cd $GOPATH/src/github.com/FocuswithJustin/participle
   git remote add upstream https://github.com/alecthomas/participle.git
   git fetch upstream
   git merge upstream/master
   # Resolve conflicts, ensuring USFM parser changes are preserved
   git push origin master
   ```
4. **Update go.mod version** if fork changes
5. **Run USFM parser tests**: `go test ./plugins/format/usfm/...`

#### Upstream Contribution Status

**Status**: Not contributed upstream (needs generalization)

**Reason**: The USFM-specific lexer extensions are domain-specific. Would need to be generalized into a "custom token extension" API before upstream contribution.

**Future Plan**:

- Design generic custom lexer extension API
- Refactor USFM parser to use generic API
- Propose upstream PR with documentation and tests
- If accepted, remove fork and use upstream version with custom extensions

---

## Security Update Process

### Monitoring for Security Issues

1. **GitHub Security Advisories**:
   - Enable "Watch" on both upstream repositories
   - Subscribe to security advisories: Settings → Notifications → Security alerts
   - Check advisories regularly: https://github.com/alecthomas/kong/security/advisories
   - Check advisories regularly: https://github.com/alecthomas/participle/security/advisories

2. **Go Vulnerability Database**:
   - Run `govulncheck` regularly (included in CI/CD)
   - Check https://pkg.go.dev/vuln/ for Go ecosystem vulnerabilities
   - Subscribe to golang-announce mailing list

3. **Dependabot Alerts**:
   - Enable Dependabot on the main Juniper Bible repository
   - Review Dependabot PRs promptly (especially security updates)
   - Configure auto-merge for patch-level security updates

### Responding to Security Issues

**Timeline**:
- **Critical vulnerabilities**: Merge and update within 24 hours
- **High severity**: Merge and update within 7 days
- **Medium severity**: Merge and update within 30 days
- **Low severity**: Merge during regular monthly sync

**Process**:
1. **Assess Impact**: Determine if the vulnerability affects Juniper Bible's use of the library
2. **Sync Fork**: Merge upstream security fix into fork immediately
3. **Update go.mod**: Bump version in go.mod to use patched fork
4. **Test**: Run full test suite: `make test`
5. **Deploy**: Release patched version of Juniper Bible
6. **Document**: Update this file with sync date and details

### Testing After Updates

After syncing with upstream:

1. **Unit Tests**: `make test`
2. **Integration Tests**: `make test-sqlite-divergence`
3. **Plugin Tests**: Test affected format plugins (USFM, CLI commands)
4. **Manual Testing**:
   - Run `capsule --help` to verify kong CLI works
   - Test USFM ingest: `capsule ingest testdata/examples/sample.usfm`
5. **Performance**: Ensure no performance regressions

## Monitoring Upstream Changes

### Automated Monitoring

**GitHub Actions Workflow** (`.github/workflows/check-upstream-forks.yml`):

```yaml
# Check for upstream updates weekly
name: Check Upstream Forks
on:
  schedule:
    - cron: '0 0 * * 1'  # Every Monday at midnight
  workflow_dispatch:

jobs:
  check-upstream:
    runs-on: ubuntu-latest
    steps:
      - name: Check kong upstream
        run: |
          curl -s https://api.github.com/repos/alecthomas/kong/releases/latest | \
            jq -r '.tag_name'
          # Compare with current fork version

      - name: Check participle upstream
        run: |
          curl -s https://api.github.com/repos/alecthomas/participle/releases/latest | \
            jq -r '.tag_name'
          # Compare with current fork version

      - name: Create issue if outdated
        # Create GitHub issue if upstream is ahead
```

### Manual Monitoring

**Monthly Review Checklist**:

- [ ] Check kong releases: https://github.com/alecthomas/kong/releases
- [ ] Check kong security: https://github.com/alecthomas/kong/security/advisories
- [ ] Check participle releases: https://github.com/alecthomas/participle/releases
- [ ] Check participle security: https://github.com/alecthomas/participle/security/advisories
- [ ] Review upstream CHANGELOG for breaking changes
- [ ] Check for deprecated APIs that we use
- [ ] Run `govulncheck` on dependencies

## Fork Maintenance Schedule

### Regular Maintenance

**Monthly** (1st Monday of each month):
- Review upstream releases
- Merge non-breaking changes
- Update go.mod versions
- Run full test suite
- Update this document with sync date

**Quarterly** (Every 3 months):
- Major version updates (if any)
- Performance benchmarking
- Review fork necessity (can we remove it?)
- Consider upstream contribution

**Annually**:
- Comprehensive security audit of forks
- Review fork architecture and necessity
- Plan upstream contribution roadmap
- Update fork documentation

### Emergency Maintenance

**Critical Security Issues**:
- Immediate sync within 24 hours
- Emergency release if necessary
- Notify users via security advisory

## Alternatives to Forking

To reduce maintenance burden, consider these alternatives:

1. **Upstream Contribution**:
   - Contribute changes upstream when possible
   - Work with maintainers to merge features
   - Reduces long-term maintenance

2. **Adapter Pattern**:
   - Wrap upstream library with adapter layer
   - Implement custom behavior in adapter
   - Update adapter when upstream changes

3. **Alternative Libraries**:
   - Evaluate alternative CLI frameworks (cobra, urfave/cli)
   - Evaluate alternative parsers (ANTLR, goyacc)
   - Trade-off: migration cost vs. maintenance cost

4. **Feature Flags**:
   - Use feature flags to toggle custom behavior
   - Easier to sync with upstream
   - Can be contributed upstream as opt-in features

## Version Pinning Policy

**Current Policy**: Use exact version pins for forked dependencies

**Rationale**:
- Ensures reproducible builds
- Prevents unexpected behavior changes
- Allows controlled updates with testing

**Update Policy**:

- Patch updates: Auto-merge after tests pass
- Minor updates: Manual review and testing
- Major updates: Comprehensive review and migration plan

## Contact

**Maintainer**: Justin (FocuswithJustin)
**Security Contact**: See SECURITY.md for security issue reporting

**Last Updated**: 2026-01-09
**Next Review**: 2026-02-01
