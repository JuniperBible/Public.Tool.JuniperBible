# WebSocket Security Implementation

This document describes the WebSocket security measures implemented in the Mimicry API server.

## Overview

The WebSocket implementation now includes comprehensive security measures to protect against common WebSocket vulnerabilities:

1. **Origin Validation** - Prevents Cross-Site WebSocket Hijacking (CSWSH)
2. **Authentication** - Requires API key validation before upgrade
3. **Message Rate Limiting** - Prevents DoS attacks via message flooding
4. **Message Size Limits** - Prevents memory exhaustion attacks

## Security Features

### 1. Origin Header Validation

The WebSocket handler validates the `Origin` header to prevent unauthorized cross-origin connections.

**Configuration:**
```go
config := WebSocketSecurityConfig{
    AllowedOrigins: []string{
        "https://example.com",
        "https://app.example.com",
        "*.example.com",  // Wildcard for subdomains
    },
}
```

**Security Benefits:**

- Prevents Cross-Site WebSocket Hijacking (CSWSH)
- Ensures only authorized domains can establish WebSocket connections
- Supports exact matching and subdomain wildcards

**Note:** Using `"*"` allows all origins and is NOT recommended for production.

### 2. Authentication

WebSocket connections can require authentication before upgrade.

**Configuration:**
```go
config := WebSocketSecurityConfig{
    RequireAuth: true,
    AuthConfig: AuthConfig{
        Enabled: true,
        APIKey:  "your-secure-api-key-here",
    },
}
```

**Authentication Methods:**

- **Header-based:** `X-API-Key: your-api-key`
- **Query parameter:** `ws://host/path?api_key=your-api-key`

Query parameter authentication is provided as a fallback for WebSocket clients that cannot easily set custom headers.

**Security Benefits:**

- Ensures only authenticated clients can connect
- Uses constant-time comparison to prevent timing attacks
- Supports both header and query parameter authentication

### 3. Message Rate Limiting

Per-client message rate limiting prevents DoS attacks.

**Configuration:**
```go
config := WebSocketSecurityConfig{
    MaxMessageRate: 10,  // 10 messages per second
}
```

**Implementation:**

- Token bucket algorithm with burst capacity (2x the rate)
- Per-client tracking with automatic cleanup
- Automatic connection closure on rate limit violation

**Security Benefits:**

- Prevents DoS attacks via message flooding
- Fair resource allocation across clients
- Automatic cleanup of idle rate limiters

### 4. Message Size Limits

Maximum message size prevents memory exhaustion.

**Configuration:**
```go
config := WebSocketSecurityConfig{
    MaxMessageSize: 4096,  // 4KB max
}
```

**Security Benefits:**

- Prevents memory exhaustion attacks
- Protects server resources
- Enforced at the WebSocket protocol level

## Usage

### Secure WebSocket Handler

Use `SecureWebSocketHandler` for production deployments:

```go
package main

import (
    "github.com/FocuswithJustin/mimicry/internal/api"
)

func main() {
    // Initialize hub
    hub := api.NewHub()
    go hub.Run()

    // Initialize rate limiter
    rateLimiter := api.NewWebSocketRateLimiter()

    // Configure security
    config := api.WebSocketSecurityConfig{
        AllowedOrigins: []string{"https://your-domain.com"},
        MaxMessageRate: 10,
        MaxMessageSize: 4096,
        RequireAuth:    true,
        AuthConfig: api.AuthConfig{
            Enabled: true,
            APIKey:  "your-secure-api-key",
        },
    }

    // Create secure handler
    handler := api.SecureWebSocketHandler(hub, config, rateLimiter)

    // Register route
    http.HandleFunc("/ws", handler)
}
```

### Default Configuration

For quick setup with secure defaults:

```go
config := api.DefaultWebSocketSecurityConfig()
// Override as needed
config.AllowedOrigins = []string{"https://your-domain.com"}
config.RequireAuth = true
config.AuthConfig = api.AuthConfig{
    Enabled: true,
    APIKey:  "your-secure-api-key",
}
```

## Migration from Insecure Handler

The existing `handleWebSocket` function is deprecated and lacks security measures.

**Before (Insecure):**
```go
http.HandleFunc("/ws", handleWebSocket)
```

**After (Secure):**
```go
hub := api.NewHub()
go hub.Run()

rateLimiter := api.NewWebSocketRateLimiter()
config := api.DefaultWebSocketSecurityConfig()
config.AllowedOrigins = []string{"https://your-domain.com"}

handler := api.SecureWebSocketHandler(hub, config, rateLimiter)
http.HandleFunc("/ws", handler)
```

## Security Considerations

### Production Deployment

1. **Never use wildcard origins (`"*"`) in production**
   - Specify exact allowed origins
   - Use subdomain wildcards sparingly

2. **Always enable authentication in production**
   ```go
   config.RequireAuth = true
   ```

3. **Use environment variables for API keys**
   ```go
   config.AuthConfig.APIKey = os.Getenv("WEBSOCKET_API_KEY")
   ```

4. **Set appropriate rate limits**
   - Consider your application's message patterns
   - Default: 10 messages/second with 2x burst

5. **Set appropriate message size limits**
   - Consider your largest legitimate message
   - Default: 4KB (4096 bytes)

### Monitoring

The security implementation logs important events:

- Connection attempts with invalid origins
- Authentication failures
- Rate limit violations
- Message size violations

Monitor these logs for potential security issues.

### Testing

The implementation includes comprehensive tests:

```bash
cd internal/api
go test -v -run TestWebSocketSecurity
go test -v -run TestSecureWebSocketHandler
```

## API Reference

### Types

```go
type WebSocketSecurityConfig struct {
    AllowedOrigins []string
    MaxMessageRate int
    MaxMessageSize int64
    RequireAuth    bool
    AuthConfig     AuthConfig
}

type WebSocketRateLimiter struct {
    // Per-client rate limiting
}
```

### Functions

```go
// DefaultWebSocketSecurityConfig returns secure default configuration
func DefaultWebSocketSecurityConfig() WebSocketSecurityConfig

// SecureWebSocketHandler creates a secure WebSocket handler
func SecureWebSocketHandler(hub *Hub, config WebSocketSecurityConfig,
    rateLimiter *WebSocketRateLimiter) http.HandlerFunc

// NewWebSocketRateLimiter creates a new rate limiter
func NewWebSocketRateLimiter() *WebSocketRateLimiter
```

## Security Vulnerabilities Addressed

| Vulnerability | Mitigation |
|--------------|------------|
| Cross-Site WebSocket Hijacking (CSWSH) | Origin validation |
| Unauthorized access | Authentication requirement |
| Denial of Service (message flooding) | Per-client rate limiting |
| Memory exhaustion | Message size limits |
| Timing attacks on authentication | Constant-time comparison |

## References

- [RFC 6455 - The WebSocket Protocol](https://tools.ietf.org/html/rfc6455)
- [OWASP WebSocket Security](https://owasp.org/www-community/vulnerabilities/WebSocket)
- [Gorilla WebSocket Security](https://github.com/gorilla/websocket/blob/master/README.md#origin-considerations)
