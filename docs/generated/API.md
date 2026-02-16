# API Reference

This document provides an overview of the main packages and types in Juniper Bible.

## REST API

The Juniper Bible REST API provides HTTP endpoints for managing capsules, converting formats, and querying plugins.

### Authentication

The API supports optional API key authentication. When enabled, all requests must include a valid API key.

**Configuration:**

- `Auth.Enabled`: Enable/disable authentication (default: false)
- `Auth.APIKeys`: List of valid API keys

**Authentication Header:**

```http
Authorization: Bearer your-api-key-here
```

**Authentication Failed:**

When authentication is enabled and no valid API key is provided:

```http
HTTP/1.1 401 Unauthorized

{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid or missing API key"
  }
}
```

**Example Configuration:**

```go
config := api.Config{
    Port: 8080,
    Auth: api.AuthConfig{
        Enabled: true,
        APIKeys: []string{"secret-key-1", "secret-key-2"},
    },
}
api.Start(config)
```

### Rate Limiting

The API implements token bucket rate limiting per client IP address. Rate limits are configured when starting the server.

**Configuration:**

- `RateLimitRequests`: Maximum requests per minute (0 = disabled)
- `RateLimitBurst`: Maximum burst size (number of tokens in bucket)

**Rate Limit Headers:**

Every API response includes rate limit information:

```http
X-RateLimit-Limit: 60          # Requests allowed per minute
X-RateLimit-Remaining: 45       # Requests remaining in current window
X-RateLimit-Reset: 1704672000   # Unix timestamp when limit resets
```

**Rate Limit Exceeded:**

When rate limit is exceeded, the API returns:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1704672000

{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Try again in 30 seconds."
  }
}
```

**IP Detection:**

The rate limiter identifies clients by IP address, checking headers in order:

1. `X-Forwarded-For` (proxy/load balancer)
2. `X-Real-IP` (reverse proxy)
3. `RemoteAddr` (direct connection)

**Example Configuration:**

```go
config := api.Config{
    Port:              8080,
    RateLimitRequests: 60,   // 60 requests/minute
    RateLimitBurst:    10,   // Allow bursts of 10
}
api.Start(config)
```

### TLS Configuration

The API supports optional TLS/HTTPS encryption for secure communication.

**Configuration:**

- `TLS.Enabled`: Enable/disable TLS (default: false)
- `TLS.CertFile`: Path to TLS certificate file
- `TLS.KeyFile`: Path to TLS private key file

**Example Configuration:**

```go
config := api.Config{
    Port: 8443,
    TLS: api.TLSConfig{
        Enabled:  true,
        CertFile: "/path/to/cert.pem",
        KeyFile:  "/path/to/key.pem",
    },
}
api.Start(config)
```

### WebSocket Real-Time Updates

The API provides a WebSocket endpoint for real-time progress updates on long-running operations.

**Endpoint:** `WS /ws`

**Message Types:**

The WebSocket sends JSON messages with different event types:

```json
{
  "type": "progress",
  "job_id": "upload",
  "stage": "validating",
  "message": "Validating file type",
  "percent": 30
}
```

```json
{
  "type": "complete",
  "job_id": "upload",
  "message": "Upload completed successfully",
  "result": {...}
}
```

```json
{
  "type": "error",
  "job_id": "upload",
  "message": "Upload failed: invalid file"
}
```

**Event Types:**

- `progress` - Operation in progress with percentage complete
- `complete` - Operation completed successfully
- `error` - Operation failed with error message

### Background Jobs

The API supports submitting long-running operations as background jobs.

**Submit Job:** `POST /jobs`

Request body:
```json
{
  "type": "convert",
  "params": {
    "source": "bible.osis",
    "target_format": "usfm"
  }
}
```

Response:
```json
{
  "success": true,
  "data": {
    "job_id": "job-123",
    "status": "queued"
  }
}
```

**Get Job Status:** `GET /jobs/:id`

Response:
```json
{
  "success": true,
  "data": {
    "job_id": "job-123",
    "status": "running",
    "progress": 45,
    "message": "Converting format..."
  }
}
```

**Cancel Job:** `DELETE /jobs/:id`

Response:
```json
{
  "success": true,
  "data": {
    "message": "Job cancelled"
  }
}
```

**Job Statuses:**

- `queued` - Job is waiting to be processed
- `running` - Job is currently executing
- `completed` - Job finished successfully
- `failed` - Job encountered an error
- `cancelled` - Job was cancelled by user

### Endpoints

- `GET /` - API information
- `GET /health` - Health check
- `GET /capsules` - List all capsules
- `POST /capsules` - Upload a capsule
- `GET /capsules/:id` - Get capsule details
- `DELETE /capsules/:id` - Delete a capsule
- `POST /convert` - Convert between formats
- `GET /plugins` - List available plugins
- `GET /formats` - List supported formats
- `WS /ws` - WebSocket connection for real-time updates
- `POST /jobs` - Submit background job
- `GET /jobs/:id` - Get job status
- `DELETE /jobs/:id` - Cancel job

## Package Overview

| Package | Description |
|---------|-------------|
| `core/ir` | Intermediate Representation for Bible text |
| `core/capsule` | Capsule archive handling |
| `core/cas` | Content-addressed storage |
| `core/plugins` | Plugin loader and IPC |
| `core/runner` | Deterministic execution harness |
| `core/selfcheck` | Round-trip verification |
| `core/docgen` | Documentation generation |
| `core/xml` | Pure Go XML parsing with XPath support |
| `core/rtf` | Pure Go RTF parser with HTML/text/LaTeX output |
| `core/epub` | Pure Go EPUB3 creation and parsing |
| `core/gobible` | Pure Go GoBible JAR creation |

## core/ir - Intermediate Representation

The IR package provides types for representing Bible text in a format-agnostic way.

### Corpus

```go
type Corpus struct {
    ID            string            `json:"id"`
    Version       string            `json:"version"`
    Title         string            `json:"title"`
    Language      string            `json:"language"`
    Versification string            `json:"versification,omitempty"`
    ModuleType    ModuleType        `json:"module_type"`
    Documents     []*Document       `json:"documents"`
    MappingTables []*MappingTable   `json:"mapping_tables,omitempty"`
    CrossReferences []*CrossReference `json:"cross_references,omitempty"`
    SourceHash    string            `json:"source_hash,omitempty"`
    LossClass     LossClass         `json:"loss_class,omitempty"`
}
```

A Corpus represents a complete Bible or biblical text collection.

### Document

```go
type Document struct {
    ID            string          `json:"id"`
    Title         string          `json:"title,omitempty"`
    Order         int             `json:"order"`
    CanonicalRef  *Ref            `json:"canonical_ref,omitempty"`
    ContentBlocks []*ContentBlock `json:"content_blocks,omitempty"`
    Annotations   []*Annotation   `json:"annotations,omitempty"`
}
```

A Document represents a book of the Bible.

### ContentBlock

```go
type ContentBlock struct {
    ID       string    `json:"id"`
    Sequence int       `json:"sequence"`
    Text     string    `json:"text"`
    Tokens   []*Token  `json:"tokens,omitempty"`
    Anchors  []*Anchor `json:"anchors,omitempty"`
    Hash     string    `json:"hash,omitempty"`
}
```

A ContentBlock represents a unit of text (verse, paragraph, etc.).

### Ref

```go
type Ref struct {
    Book      string `json:"book"`
    Chapter   int    `json:"chapter"`
    Verse     int    `json:"verse"`
    VerseEnd  int    `json:"verse_end,omitempty"`
    OSISID    string `json:"osis_id,omitempty"`
    SubVerse  string `json:"sub_verse,omitempty"`
}
```

A Ref represents a Bible reference (e.g., Gen.1.1).

### Key Functions

```go
// Parse a reference string
func ParseRef(s string) (*Ref, error)

// Align corpora by verse
func AlignByVerse(corpora []*Corpus) (*ParallelCorpus, error)

// Map references between versification systems
func (mt *MappingTable) MapRef(ref *Ref) *Ref

// Apply versification to entire corpus
func (mt *MappingTable) ApplyToCorpus(corpus *Corpus) (*Corpus, *LossReport, error)
```

### Loss Classes

| Class | Description |
|-------|-------------|
| `L0` | Lossless - byte-identical round-trip |
| `L1` | Semantically lossless - content preserved |
| `L2` | Minor loss - some metadata lost |
| `L3` | Significant loss - text preserved, markup lost |
| `L4` | Major loss - content may be affected |

## core/capsule - Archive Handling

### Capsule

```go
type Capsule struct {
    Path     string
    Manifest *Manifest
}

// Create a new capsule
func Create(path string) (*Capsule, error)

// Open an existing capsule
func Open(path string) (*Capsule, error)

// Pack a capsule
func (c *Capsule) Pack(outputPath string) error

// Unpack a capsule
func Unpack(capsulePath, outputDir string) error
```

### Manifest

```go
type Manifest struct {
    CapsuleVersion string      `json:"capsule_version"`
    CreatedAt      time.Time   `json:"created_at"`
    Tool           *ToolInfo   `json:"tool,omitempty"`
    Blobs          []*BlobInfo `json:"blobs"`
    Artifacts      []*Artifact `json:"artifacts"`
    Runs           []*Run      `json:"runs,omitempty"`
    IRExtractions  []*IRRecord `json:"ir_extractions,omitempty"`
}
```

### Key Functions

```go
// Export an artifact
func (c *Capsule) Export(artifactID, outputPath string, mode ExportMode) error

// Export with format conversion via IR
func (c *Capsule) ExportDerived(artifactID string, opts *DerivedExportOptions) (*DerivedExportResult, error)

// Verify capsule integrity
func (c *Capsule) Verify() error

// Store IR in capsule
func (c *Capsule) StoreIR(corpus *ir.Corpus) (*IRRecord, error)

// Load IR from capsule
func (c *Capsule) LoadIR(recordID string) (*ir.Corpus, error)
```

## core/cas - Content-Addressed Storage

```go
type Store struct {
    BasePath string
}

// Create a new store
func NewStore(basePath string) *Store

// Store a blob and return its hash
func (s *Store) Store(data []byte) (string, error)

// Retrieve a blob by hash
func (s *Store) Get(hash string) ([]byte, error)

// Check if a blob exists
func (s *Store) Has(hash string) bool

// Compute SHA-256 hash
func Hash(data []byte) string

// Compute BLAKE3 hash
func HashBlake3(data []byte) string
```

## core/plugins - Plugin System

### Plugin

```go
type Plugin struct {
    ID          string       `json:"plugin_id"`
    Version     string       `json:"version"`
    Kind        string       `json:"kind"` // "format" or "tool"
    Entrypoint  string       `json:"entrypoint"`
    Description string       `json:"description,omitempty"`
    Extensions  []string     `json:"extensions,omitempty"`
    LossClass   string       `json:"loss_class,omitempty"`
    IRSupport   *IRSupport   `json:"ir_support,omitempty"`
    Profiles    []Profile    `json:"profiles,omitempty"`
}
```

### Loader

```go
type Loader struct {
    PluginDir string
    Plugins   map[string]*Plugin
}

// Create a new loader
func NewLoader(pluginDir string) *Loader

// Discover and load all plugins
func (l *Loader) LoadAll() error

// Get a plugin by ID
func (l *Loader) Get(id string) *Plugin

// Detect format of a file
func (l *Loader) Detect(filePath string) (*Plugin, error)
```

### IPC Protocol

```go
// Execute a plugin command
func Execute(plugin *Plugin, command string, input interface{}) (interface{}, error)

// Extract IR from a file
func ExtractIR(plugin *Plugin, filePath string) (*ir.Corpus, *LossReport, error)

// Emit native format from IR
func EmitNative(plugin *Plugin, corpus *ir.Corpus, outputPath string) (*LossReport, error)
```

## core/runner - Execution Harness

### NixExecutor

```go
type NixExecutor struct {
    WorkDir string
}

// Create a new Nix executor
func NewNixExecutor(workDir string) *NixExecutor

// Run a tool with deterministic environment
func (e *NixExecutor) Run(tool, profile string, args map[string]string) (*Transcript, error)
```

### Transcript

```go
type Transcript struct {
    Events []*TranscriptEvent `json:"events"`
    Hash   string             `json:"hash"`
}

type TranscriptEvent struct {
    Timestamp time.Time              `json:"timestamp"`
    Type      string                 `json:"type"`
    Data      map[string]interface{} `json:"data"`
}
```

## core/selfcheck - Verification

### Plan

```go
type Plan struct {
    ID          string  `json:"id"`
    Description string  `json:"description"`
    Steps       []*Step `json:"steps"`
    Checks      []*Check `json:"checks"`
}
```

### Executor

```go
type Executor struct {
    Capsule *capsule.Capsule
}

// Execute a verification plan
func (e *Executor) Execute(plan *Plan) (*Report, error)
```

### LossBudget

```go
type LossBudget struct {
    MaxLossClass LossClass
    AllowedLoss  map[string]LossClass
}

// Check if loss is within budget
func (b *LossBudget) IsWithinBudget(report *LossReport) bool
```

## core/docgen - Documentation Generation

```go
type Generator struct {
    PluginDir string
    OutputDir string
}

// Create a new generator
func NewGenerator(pluginDir, outputDir string) *Generator

// Load all plugins for documentation
func (g *Generator) LoadPlugins() ([]PluginManifest, error)

// Generate all documentation
func (g *Generator) GenerateAll() error
```

## CLI Commands

See [CLI_REFERENCE.md](generated/CLI_REFERENCE.md) for complete CLI documentation.

### Core Commands

```bash
capsule ingest <path> --out <capsule.tar.xz>
capsule export <capsule> --artifact <id> --out <path>
capsule verify <capsule>
capsule selfcheck <capsule>
```

### IR Commands

```bash
capsule extract-ir <path> --format <format> --out <ir.json>
capsule emit-native <ir.json> --format <format> --out <path>
capsule convert <path> --to <format> --out <path>
capsule ir-info <ir.json>
```

### Plugin Commands

```bash
capsule plugins
capsule detect <path>
capsule enumerate <path>
capsule run <tool> <profile> [--input <path>]
```

### Documentation Commands

```bash
capsule docgen plugins
capsule docgen formats
capsule docgen cli
capsule docgen all [--output <dir>]
```

## Related Documentation

- [IR_IMPLEMENTATION.md](IR_IMPLEMENTATION.md) - Detailed IR system documentation
- [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md) - How to create plugins
- [VERSIFICATION.md](VERSIFICATION.md) - Versification systems
- [CROSSREF.md](CROSSREF.md) - Cross-reference types
- [PARALLEL.md](PARALLEL.md) - Parallel corpus alignment
