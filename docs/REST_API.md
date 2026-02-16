# REST API Documentation

The Juniper Bible REST API provides HTTP access to capsule management, format conversion, and plugin information.

## Quick Start

Start the API server:

```bash
capsule api --port 8080 --capsules-dir ./data/capsules
```

With authentication enabled:

```bash
# Generate a secure API key
export CAPSULE_API_KEY=$(openssl rand -base64 32)

# Start server with authentication
capsule api --port 8080 --capsules-dir ./data/capsules --auth --api-key "$CAPSULE_API_KEY"

# Make authenticated requests
curl -H "X-API-Key: $CAPSULE_API_KEY" http://localhost:8080/capsules
```

## Authentication

API key authentication is optional and can be enabled via configuration.

### Configuration

Authentication is controlled by two settings:

- `--auth`: Enable API key authentication (default: disabled)
- `--api-key`: The API key to require (required if `--auth` is enabled)

### Environment Variables

```bash
export CAPSULE_API_KEY="your-secret-api-key-here"
capsule api --auth --api-key "$CAPSULE_API_KEY"
```

### Security Requirements

- API keys must be at least 16 characters long
- Keys are case-sensitive
- Use a cryptographically secure random generator (e.g., `openssl rand -base64 32`)

### Making Authenticated Requests

Include the API key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-secret-api-key-here" \
     http://localhost:8080/capsules
```

### Public Endpoints

The following endpoints are always accessible without authentication:

- `GET /` - API information
- `GET /health` - Health check

All other endpoints require authentication when auth is enabled.

### Error Responses

**Missing API Key:**
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing X-API-Key header"
  },
  "meta": {
    "timestamp": "2025-01-07T12:00:00Z"
  }
}
```

**Invalid API Key:**
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid API key"
  },
  "meta": {
    "timestamp": "2025-01-07T12:00:00Z"
  }
}
```

## Base URL

All API endpoints are relative to the server base URL:

```
http://localhost:8080
```

## Response Format

All responses follow a standard JSON structure:

**Success Response:**
```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "timestamp": "2025-01-07T12:00:00Z",
    "total": 10
  }
}
```

**Error Response:**
```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  },
  "meta": {
    "timestamp": "2025-01-07T12:00:00Z"
  }
}
```

## Endpoints

### GET /

Get API information.

**Authentication:** Not required

**Response:**
```json
{
  "success": true,
  "data": {
    "name": "Juniper Bible API",
    "version": "0.2.0",
    "docs": "/api/docs",
    "endpoints": [
      "GET /health",
      "GET /capsules",
      "POST /capsules",
      "GET /capsules/:id",
      "DELETE /capsules/:id",
      "POST /convert",
      "GET /plugins",
      "GET /formats"
    ]
  }
}
```

### GET /health

Health check endpoint.

**Authentication:** Not required

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "version": "0.2.0",
    "uptime": "1h23m45s",
    "capsules": 5,
    "plugins": 33
  }
}
```

### GET /capsules

List all capsules.

**Authentication:** Required (if enabled)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "kjv.tar.xz",
      "name": "kjv.tar.xz",
      "path": "kjv.tar.xz",
      "size": 1234567,
      "format": "tar.xz",
      "created_at": "2025-01-07T12:00:00Z"
    }
  ],
  "meta": {
    "total": 1,
    "timestamp": "2025-01-07T12:00:00Z"
  }
}
```

### POST /capsules

Upload a new capsule.

**Authentication:** Required (if enabled)

**Request:**

- Content-Type: `multipart/form-data`
- Field: `file` (the capsule file to upload)

**Example:**
```bash
curl -X POST \
  -H "X-API-Key: your-api-key" \
  -F "file=@kjv.tar.xz" \
  http://localhost:8080/capsules
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "kjv.tar.xz",
    "name": "kjv.tar.xz",
    "path": "kjv.tar.xz",
    "size": 1234567,
    "format": "tar.xz"
  }
}
```

### GET /capsules/:id

Get capsule details.

**Authentication:** Required (if enabled)

**Example:**
```bash
curl -H "X-API-Key: your-api-key" \
  http://localhost:8080/capsules/kjv.tar.xz
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "kjv.tar.xz",
    "name": "kjv.tar.xz",
    "path": "kjv.tar.xz",
    "size": 1234567,
    "format": "tar.xz",
    "manifest": {
      "version": "1.0",
      "module_type": "bible",
      "title": "King James Version",
      "language": "en"
    },
    "artifacts": [
      {
        "id": "kjv.osis",
        "name": "kjv.osis",
        "size": 5432100
      }
    ]
  }
}
```

### DELETE /capsules/:id

Delete a capsule.

**Authentication:** Required (if enabled)

**Example:**
```bash
curl -X DELETE \
  -H "X-API-Key: your-api-key" \
  http://localhost:8080/capsules/kjv.tar.xz
```

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "Capsule deleted"
  }
}
```

### GET /plugins

List all available plugins.

**Authentication:** Required (if enabled)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "name": "osis",
      "type": "format",
      "description": "Format plugin for osis"
    },
    {
      "name": "usfm",
      "type": "format",
      "description": "Format plugin for usfm"
    }
  ],
  "meta": {
    "total": 2,
    "timestamp": "2025-01-07T12:00:00Z"
  }
}
```

### GET /formats

List all supported formats.

**Authentication:** Required (if enabled)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "osis",
      "name": "OSIS XML",
      "extensions": [".osis", ".xml"],
      "loss_class": "L0",
      "description": "Open Scripture Information Standard",
      "can_extract": true,
      "can_emit": true
    }
  ],
  "meta": {
    "total": 13,
    "timestamp": "2025-01-07T12:00:00Z"
  }
}
```

### POST /convert

Convert between formats.

**Authentication:** Required (if enabled)

**Status:** Not yet implemented (returns 501)

**Request:**
```json
{
  "source": "kjv.osis",
  "target_format": "usfm",
  "options": {}
}
```

**Response:**
```json
{
  "success": false,
  "error": {
    "code": "NOT_IMPLEMENTED",
    "message": "Conversion from kjv.osis to usfm not yet implemented via API. Use the CLI."
  }
}
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid API key |
| `NOT_FOUND` | 404 | Resource not found |
| `BAD_REQUEST` | 400 | Invalid request |
| `INVALID_JSON` | 400 | Malformed JSON body |
| `MISSING_PARAMS` | 400 | Required parameters missing |
| `MISSING_FILE` | 400 | File upload missing |
| `MISSING_ID` | 400 | Resource ID missing |
| `METHOD_NOT_ALLOWED` | 405 | HTTP method not allowed |
| `SAVE_FAILED` | 500 | Failed to save file |
| `DELETE_FAILED` | 500 | Failed to delete resource |
| `NOT_IMPLEMENTED` | 501 | Feature not implemented |
| `INVALID_CONFIG` | 500 | Server configuration error |

## CORS

The API includes CORS headers for cross-origin requests:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, X-API-Key
```

## Best Practices

### API Key Security

1. **Generate strong keys:** Use cryptographically secure random generators
   ```bash
   openssl rand -base64 32
   ```

2. **Store securely:** Never commit API keys to version control
   ```bash
   # Use environment variables
   export CAPSULE_API_KEY="..."

   # Or a secrets file (excluded from git)
   echo "CAPSULE_API_KEY=..." > .env
   source .env
   ```

3. **Rotate regularly:** Change API keys periodically

4. **Use HTTPS:** In production, always use TLS/SSL encryption

### Rate Limiting

Currently not implemented. Consider implementing rate limiting for production deployments.

### Monitoring

Monitor the following metrics:

- Request rate per endpoint
- Authentication failures
- Response times (logged as `[TIME]` or `[SLOW]` in server logs)
- Error rates by code

## Example Workflows

### Upload and Query Capsule

```bash
# Set API key
export CAPSULE_API_KEY="your-secret-key"

# Upload capsule
curl -X POST \
  -H "X-API-Key: $CAPSULE_API_KEY" \
  -F "file=@kjv.tar.xz" \
  http://localhost:8080/capsules

# List all capsules
curl -H "X-API-Key: $CAPSULE_API_KEY" \
  http://localhost:8080/capsules

# Get capsule details
curl -H "X-API-Key: $CAPSULE_API_KEY" \
  http://localhost:8080/capsules/kjv.tar.xz

# Delete capsule
curl -X DELETE \
  -H "X-API-Key: $CAPSULE_API_KEY" \
  http://localhost:8080/capsules/kjv.tar.xz
```

### Query Available Formats

```bash
# List all supported formats
curl -H "X-API-Key: $CAPSULE_API_KEY" \
  http://localhost:8080/formats

# List all plugins
curl -H "X-API-Key: $CAPSULE_API_KEY" \
  http://localhost:8080/plugins
```

## Related Documentation

- [API.md](generated/API.md) - Code API reference
- [CLI_REFERENCE.md](generated/CLI_REFERENCE.md) - Command-line interface
- [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) - Plugin development guide
