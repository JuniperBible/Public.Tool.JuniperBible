# API Documentation

This directory contains OpenAPI/Swagger documentation for the Juniper Bible REST API.

## Files

- **openapi.yaml** - OpenAPI 3.0 specification for the REST API

## Viewing the Documentation

### Online Viewers

You can view the OpenAPI specification using online tools:

1. **Swagger Editor**: https://editor.swagger.io/
   - File > Import File > Select `openapi.yaml`

2. **Redoc**: https://redocly.github.io/redoc/
   - Paste the raw YAML URL or file contents

### Local Viewing

#### Option 1: Swagger UI (Docker)

```bash
docker run -p 8081:8080 -e SWAGGER_JSON=/docs/openapi.yaml \
  -v $(pwd)/docs/api:/docs swaggerapi/swagger-ui
```

Then open http://localhost:8081

#### Option 2: Redoc (Docker)

```bash
docker run -p 8081:80 -e SPEC_URL=/specs/openapi.yaml \
  -v $(pwd)/docs/api:/usr/share/nginx/html/specs redocly/redoc
```

Then open http://localhost:8081

#### Option 3: VS Code Extension

Install the "OpenAPI (Swagger) Editor" extension and open `openapi.yaml`

## API Overview

The Juniper Bible API provides **11 endpoints** across 6 categories:

### Information & Health
- `GET /` - API information
- `GET /health` - Health check

### Capsule Management
- `GET /capsules` - List all capsules
- `POST /capsules` - Upload a new capsule
- `GET /capsules/{id}` - Get capsule details
- `DELETE /capsules/{id}` - Delete a capsule

### Conversion

- `POST /convert` - Synchronous conversion (not yet implemented)

### Asynchronous Jobs
- `POST /jobs` - Create conversion job
- `GET /jobs/{id}` - Get job status
- `DELETE /jobs/{id}` - Cancel job

### Metadata

- `GET /plugins` - List available plugins
- `GET /formats` - List supported formats

### Real-Time Updates

- `WS /ws` - WebSocket connection for progress updates

## Authentication

The API supports optional API key authentication:

```bash
# Generate a secure API key
export CAPSULE_API_KEY=$(openssl rand -base64 32)

# Use in requests
curl -H "X-API-Key: $CAPSULE_API_KEY" http://localhost:8080/capsules
```

When authentication is enabled, all endpoints except `/` and `/health` require the `X-API-Key` header.

## Example Usage

### List Capsules

```bash
curl http://localhost:8080/capsules
```

### Upload a Capsule

```bash
curl -X POST -F "file=@bible.tar.xz" http://localhost:8080/capsules
```

### Get Capsule Details

```bash
curl http://localhost:8080/capsules/bible.tar.xz
```

### Create Conversion Job

```bash
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source": "bible.tar.xz",
    "target_format": "json",
    "options": {
      "include_notes": true
    }
  }'
```

### Check Job Status

```bash
curl http://localhost:8080/jobs/{job-id}
```

### WebSocket Connection

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log(`${msg.operation}: ${msg.progress}% - ${msg.message}`);
};
```

## Response Format

All API responses follow a consistent structure:

```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "timestamp": "2026-01-10T12:34:56Z",
    "total": 42
  }
}
```

Error responses:

```json
{
  "success": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "Resource not found"
  },
  "meta": {
    "timestamp": "2026-01-10T12:34:56Z"
  }
}
```

## Supported Formats

The API supports 13 Bible formats with varying loss classifications:

| Format | Loss Class | Extract | Emit | Description |
|--------|------------|---------|------|-------------|
| OSIS | L0 | Yes | Yes | Open Scripture Information Standard |
| USFM | L0 | Yes | Yes | Unified Standard Format Markers |
| USX | L0 | Yes | Yes | Unified Scripture XML |
| Zefania | L0 | Yes | Yes | Zefania Bible format |
| TheWord | L0 | Yes | Yes | TheWord Bible software |
| JSON | L0 | Yes | Yes | JSON structure |
| HTML | L1 | Yes | Yes | Static HTML site |
| SQLite | L1 | Yes | Yes | SQLite database |
| e-Sword | L1 | Yes | Yes | e-Sword format |
| EPUB | L1 | No | Yes | EPUB3 ebook |
| Markdown | L1 | No | Yes | Hugo-compatible Markdown |
| SWORD | L2 | Yes | Yes | SWORD module format |
| Plain Text | L3 | Yes | Yes | Plain text (verse per line) |

### Loss Classes

- **L0**: Lossless (perfect round-trip)
- **L1**: Minor loss (formatting/styling)
- **L2**: Moderate loss (some structural elements)
- **L3**: High loss (text only)

## Security

The API includes multiple security features:

- **Authentication**: Optional API key authentication
- **Rate Limiting**: Configurable requests per minute with burst support
- **CORS**: Configurable allowed origins
- **TLS/HTTPS**: Optional TLS support
- **File Validation**: Magic byte validation and size limits (1GB max)
- **Path Sanitization**: Protection against path traversal attacks
- **Constant-Time Comparison**: Prevents timing attacks on API keys

## Development

To regenerate or update the OpenAPI specification:

1. Edit the source in `/tmp/worktree-api-openapi/internal/api/handlers.go`
2. Update `docs/api/openapi.yaml` to reflect changes
3. Validate with: `docker run --rm -v $(pwd):/spec redocly/cli lint /spec/docs/api/openapi.yaml`
