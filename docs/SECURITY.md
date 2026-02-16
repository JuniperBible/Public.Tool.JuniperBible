# Security Documentation

This document describes the security model, architecture, known limitations, and deployment recommendations for Juniper Bible.

## Table of Contents

- [Security Model](#security-model)
- [Plugin Execution Environment](#plugin-execution-environment)
- [File System Access Controls](#file-system-access-controls)
- [API Security](#api-security)
- [Known Limitations](#known-limitations)
- [Deployment Recommendations](#deployment-recommendations)
- [Reporting Security Issues](#reporting-security-issues)

## Security Model

Juniper Bible follows a **defense-in-depth** security model with multiple layers of protection:

### 1. Plugin Isolation

**Architecture**: Plugins are separate executables that communicate with the host via JSON over stdin/stdout (IPC).

**Isolation Mechanisms**:

- **Process Boundary**: Each plugin runs as a separate OS process with its own memory space
- **No Shared State**: Plugins cannot directly access host memory or internal state
- **Controlled IPC**: All communication goes through validated JSON messages
- **Timeout Protection**: Plugin execution has configurable timeouts (default: 60 seconds)
- **Embedded-First**: External plugins are disabled by default; only embedded (compiled-in) plugins are used

**Plugin Types**:

- **Embedded Plugins**: Compiled directly into the binary, cannot be modified without rebuilding
- **External Plugins**: Loadable from filesystem (disabled by default, requires `--plugins-external` flag)

### 2. Pure Go Implementation

**Self-Contained Architecture**:

- Built with `CGO_ENABLED=0` - no C dependencies
- No external tool requirements (libsword, pandoc, calibre, libxml2, unrtf replaced with pure Go)
- Reduces attack surface by eliminating external dependencies

### 3. Deterministic Execution

**Nix VM Harness** (for reference tools):
- Standardized environment: `TZ=UTC`, `LC_ALL=C.UTF-8`
- Reproducible builds and executions
- Isolated from host system

## Plugin Execution Environment

### External Plugin Security

When external plugins are enabled (`--plugins-external`):

1. **Plugin Loading**:
   - Plugins must have a valid `plugin.json` manifest
   - Manifest validation checks: `plugin_id`, `version`, `kind`, `entrypoint`
   - Plugin binaries must be executable files in the plugin directory

2. **Execution Flow**:
   ```
   Host → JSON Request (stdin) → Plugin Process → JSON Response (stdout) → Host
   ```

3. **Plugin Capabilities**:
   - Plugins receive file paths as arguments
   - Plugins can read/write files in specified output directories
   - Plugins have no special privileges beyond the user running capsule
   - Plugins run with the same permissions as the capsule process

4. **Timeout and Cancellation**:
   - Default timeout: 60 seconds
   - Uses `context.WithTimeout` for proper cancellation
   - Process is killed if timeout is exceeded

### Plugin Sandbox Boundaries

**What Plugins CAN Do**:
- Read files passed as arguments
- Write to designated output directories
- Execute within their process memory space
- Return structured JSON responses

**What Plugins CANNOT Do**:
- Access host process memory
- Modify host state directly
- Execute indefinitely (timeout enforced)
- Access files outside provided paths (unless user permissions allow)

**Security Note**: Plugins run with the **same user permissions** as the capsule process. There is no OS-level sandboxing (no chroot, namespaces, or containers). Plugin security relies on:
1. Process isolation
2. Controlled IPC
3. User's filesystem permissions
4. Disabled-by-default external plugins

## File System Access Controls

### Path Traversal Protection

**Web UI Handlers** (`internal/web/handlers.go`):
- Implements `sanitizePath()` function for all user-supplied paths
- Validates paths using `filepath.Clean()` and absolute path resolution
- Rejects paths containing `..` after cleaning
- Ensures resolved paths remain within base directory using prefix checking

**API Handlers** (`internal/api/handlers.go`) - **FIXED**:

- Added path sanitization for file uploads (`createCapsuleHandler`)
- Added path sanitization for capsule access (`getCapsuleHandler`, `deleteCapsuleHandler`)
- Uses `filepath.Base()` to extract filename only, preventing directory traversal
- Rejects special paths (`.`, `..`) and paths containing `..`

**Archive Extraction** (`internal/juniper/repoman/repoman.go`):
- Validates extracted file paths to prevent zip-slip vulnerabilities
- Uses `filepath.Clean()` and prefix checking
- Ensures extraction targets remain within destination directory

### File Access Patterns

**Input Validation**:

- User-supplied paths are cleaned with `filepath.Clean()`
- Relative paths are resolved to absolute paths
- Directory traversal sequences (`../`) are detected and rejected

**Output Directories**:
- Created with `0755` permissions
- Files written with `0644` permissions
- No world-writable directories or files

## API Security

### HTTP Security Headers

Applied via `SecurityHeadersMiddleware` (`internal/server/common.go`):

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
```

**Note**: CSP uses `'unsafe-inline'` for scripts and styles due to embedded templates. This is acceptable for local/trusted deployments but should be hardened for public-facing deployments.

### CORS Configuration

**Current State**: CORS middleware exists (`CORSMiddleware`) but allows all origins (`Access-Control-Allow-Origin: *`).

**Recommendation**: For production deployments, restrict CORS to trusted origins:
```go
w.Header().Set("Access-Control-Allow-Origin", "https://trusted-domain.com")
```

### Input Validation

**API Endpoints**:
- JSON payload validation
- File upload size limits (100MB max)
- Filename sanitization (path traversal protection)
- ID parameter sanitization
- HTTP method restrictions

**Web Handlers**:
- Form input sanitization
- Path parameter validation
- Query parameter filtering

### Rate Limiting

**Current State**: No built-in rate limiting.

**Recommendation**: For production deployments, add rate limiting middleware to prevent abuse:
- Per-IP request limits
- Upload size/frequency limits
- API endpoint throttling

## Known Limitations

### 1. No OS-Level Sandboxing

**Issue**: External plugins run with the same OS-level permissions as the capsule process.

**Impact**: A malicious plugin could:
- Read any file the user can read
- Write any file the user can write
- Execute system commands
- Access network resources

**Mitigation**:
- External plugins are **disabled by default**
- Only use trusted plugins
- Run capsule with minimal user permissions
- Review plugin source code before enabling external plugins
- Consider running in a container/VM for additional isolation

### 2. Limited CSP (Content Security Policy)

**Issue**: Web UI requires `'unsafe-inline'` for scripts and styles.

**Impact**: Potential XSS vulnerabilities if user input is not properly sanitized.

**Mitigation**:

- All user input is HTML-escaped using `html/template`
- Web UI is designed for local/trusted use
- For public deployment, implement nonce-based CSP

### 3. No Authentication/Authorization

**Issue**: Web UI and API have no built-in authentication.

**Impact**: Anyone with network access can use the service.

**Mitigation**:
- Bind to `localhost` only (`--port 8080` binds to `127.0.0.1`)
- Use a reverse proxy (nginx, caddy) with authentication for remote access
- Run in a trusted network environment
- Consider adding API keys or basic auth for production use

### 4. File Upload Risks

**Issue**: API accepts arbitrary file uploads (up to 100MB).

**Impact**:
- Disk space exhaustion
- Processing of malicious files

**Mitigation**:
- File size limits enforced (100MB)
- Files stored in designated capsules directory
- Filename sanitization prevents path traversal
- Consider adding file type validation
- Monitor disk usage

### 5. SQLite Injection

**Issue**: While the code uses parameterized queries in most places, dynamic SQL construction exists in some plugins.

**Impact**: Potential SQL injection in plugin implementations.

**Mitigation**:
- Core code uses `core/sqlite` package with parameterized queries
- Plugin developers should follow secure coding practices
- Review plugin SQL queries for proper parameterization

## Deployment Recommendations

### Local Development / Trusted Environment

**Minimal Configuration**:
```bash
capsule web --port 8080 --capsules ./capsules
```

**Security Considerations**:
- Binds to localhost only
- No external plugin loading
- Suitable for single-user development

### Multi-User / Untrusted Environment

**Recommended Configuration**:
```bash
# Run in container with limited permissions
docker run --read-only --tmpfs /tmp \
  -v ./capsules:/capsules:rw \
  -p 127.0.0.1:8080:8080 \
  juniper-bible web --port 8080 --capsules /capsules

# Use reverse proxy for authentication
# nginx with basic auth or OAuth2 proxy
```

**Additional Hardening**:
1. **Containerization**: Run in Docker/Podman with:
   - Read-only root filesystem
   - Dropped capabilities
   - No network access (if not needed)
   - Resource limits (CPU, memory)

2. **User Permissions**:
   - Create dedicated user account
   - Restrict file system access
   - Use principle of least privilege

3. **Network Security**:
   - Bind to localhost only
   - Use reverse proxy with TLS
   - Implement authentication (basic auth, OAuth2, mTLS)
   - Add rate limiting

4. **Monitoring**:
   - Log all API requests
   - Monitor file upload sizes
   - Track plugin execution times
   - Alert on anomalies

5. **Dependency Management**:
   - Keep forked dependencies in sync with upstream (see [FORKED_DEPENDENCIES.md](FORKED_DEPENDENCIES.md))
   - Run `govulncheck` regularly to detect vulnerabilities
   - Subscribe to security advisories for all dependencies
   - Follow the monthly maintenance schedule for fork updates

### Public Internet Deployment (NOT RECOMMENDED)

**Warning**: Juniper Bible is designed for local/trusted environments. Public internet deployment requires significant additional hardening.

**Required Hardening** (minimum):

1. Strong authentication (OAuth2, mTLS)
2. Rate limiting and DDoS protection
3. Input validation and sanitization review
4. Regular security audits
5. Intrusion detection/prevention
6. Dedicated security review of all plugins
7. Container isolation with AppArmor/SELinux
8. Network segmentation
9. Regular updates and patching
10. Security monitoring and incident response plan

## Security Best Practices

### For Developers

1. **Never trust user input**: Always sanitize paths, filenames, and parameters
2. **Use parameterized queries**: Avoid dynamic SQL construction
3. **Validate JSON**: Check types and ranges before using IPC data
4. **Handle errors securely**: Don't leak sensitive information in error messages
5. **Minimize privileges**: Request only necessary permissions
6. **Review dependencies**: Keep Go modules updated, audit for vulnerabilities

### For Plugin Developers

1. **Validate all inputs**: Check paths, verify file existence, validate formats
2. **Use safe file operations**: Check for path traversal, use absolute paths
3. **Handle timeouts gracefully**: Respond to context cancellation
4. **Return structured errors**: Use JSON error responses, not panics
5. **Document security requirements**: Note any special permissions needed
6. **Avoid command injection**: Don't execute shell commands with user input
7. **Test with malicious input**: Fuzz test your plugin with edge cases

### For System Administrators

1. **Principle of least privilege**: Run capsule with minimal user permissions
2. **Network isolation**: Bind to localhost, use firewall rules
3. **Regular updates**: Keep capsule and dependencies current
4. **Monitor logs**: Watch for suspicious activity
5. **Backup regularly**: Protect against data loss
6. **Test disaster recovery**: Ensure backups are restorable
7. **Document deployment**: Maintain security configuration documentation

## Reporting Security Issues

**DO NOT** open public GitHub issues for security vulnerabilities.

**Instead**:
1. Email security concerns to the maintainer (see README for contact)
2. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if available)
3. Allow reasonable time for fix before public disclosure
4. Credit will be given for responsible disclosure

**Response Timeline**:
- Acknowledgment: Within 48 hours
- Initial assessment: Within 7 days
- Fix timeline: Depends on severity (critical: days, high: weeks, medium: months)

## Security Audit History

### 2026-01-07: Dependency Audit

**Audit Date**: 2026-01-07
**Conducted By**: Automated security scan via `govulncheck`
**Go Version**: 1.25.4

#### Vulnerabilities Found

**High Severity - Go Standard Library**:

1. **GO-2025-4175**: Improper application of excluded DNS name constraints when verifying wildcard names in `crypto/x509`
   - **Status**: Requires Go 1.25.5+ to fix
   - **Affected**: `crypto/x509@go1.25.4`
   - **Fixed in**: `crypto/x509@go1.25.5`
   - **Mitigation**: Upgrade to Go 1.25.5 when available

2. **GO-2025-4155**: Excessive resource consumption when printing error string for host certificate validation in `crypto/x509`
   - **Status**: Requires Go 1.25.5+ to fix
   - **Affected**: `crypto/x509@go1.25.4`
   - **Fixed in**: `crypto/x509@go1.25.5`
   - **Mitigation**: Upgrade to Go 1.25.5 when available

**Third-Party Dependencies** (Resolved):

3. **GO-2025-3595**: Incorrect Neutralization of Input During Web Page Generation in `golang.org/x/net`
   - **Status**: FIXED
   - **Resolution**: Updated to `v0.48.0`

4. **GO-2025-3503**: HTTP Proxy bypass using IPv6 Zone IDs in `golang.org/x/net`
   - **Status**: FIXED
   - **Resolution**: Updated to `v0.48.0`

#### Dependencies Updated

- `golang.org/x/net`: v0.33.0 → v0.48.0 (Security fix)
- `golang.org/x/sys`: v0.28.0 → v0.39.0
- `golang.org/x/text`: v0.21.0 → v0.32.0
- `github.com/klauspost/cpuid/v2`: v2.0.12 → v2.3.0
- `modernc.org/sqlite`: v1.34.5 → v1.42.2 (Pure Go SQLite)

### 2026-01-07: Initial Security Audit

**Findings**:

1. **CRITICAL - Path Traversal in API**: File upload and capsule access endpoints did not sanitize filenames/IDs
   - **Status**: FIXED
   - **Fix**: Added `filepath.Base()` and `..` detection in `createCapsuleHandler`, `getCapsuleHandler`, `deleteCapsuleHandler`

2. **MEDIUM - No Rate Limiting**: API has no rate limiting
   - **Status**: DOCUMENTED
   - **Recommendation**: Add rate limiting middleware for production

3. **MEDIUM - No Authentication**: Web/API have no auth
   - **Status**: DOCUMENTED (by design)
   - **Recommendation**: Use reverse proxy with auth for untrusted environments

4. **LOW - Permissive CORS**: Allows all origins
   - **Status**: DOCUMENTED
   - **Recommendation**: Restrict origins for production

5. **INFO - External Plugins**: Plugin security model documented
   - **Status**: DOCUMENTED
   - **Recommendation**: Keep external plugins disabled by default

**Verified Secure**:
- ✅ Web handlers use `sanitizePath()` with proper path traversal protection
- ✅ Archive extraction validates paths (zip-slip protection)
- ✅ Plugin IPC uses structured JSON (no command injection)
- ✅ SQL queries use parameterized queries in core code
- ✅ Security headers applied to all responses
- ✅ External plugins disabled by default
- ✅ Plugin execution has timeouts

## Conclusion

Juniper Bible is designed for **local/trusted environments**. The security model assumes:
- Single user or trusted users
- Local network access
- Trusted plugins
- No malicious input

For production deployments, additional hardening is required (authentication, rate limiting, containerization, monitoring).

The most critical security control is **disabled-by-default external plugins**. Only enable external plugins from trusted sources after code review.

---

**Last Updated**: 2026-01-07
**Version**: 1.0
**Audit Performed By**: Internal Security Analysis
