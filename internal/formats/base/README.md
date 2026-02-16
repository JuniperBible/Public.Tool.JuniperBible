# Format Handler Base Package

This package provides common functionality for format handlers to reduce code duplication.

## Overview

After analyzing multiple format handlers (OSIS, USFM, SWORD, Zefania, JSON, TXT), several patterns emerged:

### Common Patterns in `Detect()`:

1. Check if path exists with `os.Stat(path)`
2. Reject directories
3. Check file extensions
4. Optionally read and validate file content
5. Return `*plugins.DetectResult`

### Common Patterns in `Ingest()`:

1. Read file with `os.ReadFile(path)`
2. Compute SHA256 hash
3. Create blob directory: `outputDir/hash[:2]`
4. Write blob file
5. Extract artifact ID (from filename or content)
6. Return `*plugins.IngestResult` with metadata

### Common Patterns in `Enumerate()`:

1. Call `os.Stat(path)`
2. Return single-entry result for files
3. Use base filename and file size

## Usage Examples

### Simple File Detection

For formats that only need extension checking:

```go
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
    return base.DetectFile(path, base.DetectConfig{
        Extensions: []string{".txt", ".text"},
        FormatName: "TXT",
    })
}
```

### Content-Based Detection

For formats that need to check file content:

```go
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
    return base.DetectFile(path, base.DetectConfig{
        Extensions:     []string{".usfm", ".sfm", ".ptx"},
        FormatName:     "USFM",
        ContentMarkers: []string{"\\id ", "\\c ", "\\v "},
    })
}
```

### Custom Detection Logic

For formats requiring custom validation:

```go
func (h *Handler) Detect(path string) (*plugins.DetectResult, error) {
    return base.DetectFile(path, base.DetectConfig{
        Extensions:   []string{".osis", ".xml"},
        FormatName:   "OSIS",
        CheckContent: true,
        CustomValidator: func(path string, data []byte) (bool, string, error) {
            var doc OSISDoc
            if err := xml.Unmarshal(data, &doc); err == nil && doc.OsisText.OsisIDWork != "" {
                return true, "Valid OSIS XML structure", nil
            }
            return false, "", nil
        },
    })
}
```

### Simple Ingestion

For formats that use filename as artifact ID:

```go
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
    return base.IngestFile(path, outputDir, base.IngestConfig{
        FormatName: "TXT",
    })
}
```

### Ingestion with Custom Artifact ID

For formats that extract artifact ID from content:

```go
func (h *Handler) Ingest(path, outputDir string) (*plugins.IngestResult, error) {
    return base.IngestFile(path, outputDir, base.IngestConfig{
        FormatName: "USFM",
        ArtifactIDExtractor: func(path string, data []byte) string {
            content := string(data)
            if idx := strings.Index(content, "\\id "); idx >= 0 {
                endIdx := strings.IndexAny(content[idx+4:], " \n\r")
                if endIdx > 0 {
                    return strings.TrimSpace(content[idx+4 : idx+4+endIdx])
                }
            }
            return filepath.Base(path)
        },
    })
}
```

### Simple Enumeration

```go
func (h *Handler) Enumerate(path string) (*plugins.EnumerateResult, error) {
    return base.EnumerateFile(path, map[string]string{
        "format": "TXT",
    })
}
```

### Helper Functions

For ExtractIR and EmitNative operations:

```go
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
    // Read file
    fileInfo, err := base.ReadFileInfo(path)
    if err != nil {
        return nil, err
    }

    // Format-specific parsing
    corpus, err := parseToIR(fileInfo.Data)
    if err != nil {
        return nil, fmt.Errorf("failed to parse: %w", err)
    }

    // Serialize IR
    irData, err := json.MarshalIndent(corpus, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("failed to serialize IR: %w", err)
    }

    // Write output
    irPath, err := base.WriteOutput(outputDir, corpus.ID+".ir.json", irData)
    if err != nil {
        return nil, err
    }

    return &plugins.ExtractIRResult{
        IRPath:    irPath,
        LossClass: string(corpus.LossClass),
    }, nil
}
```

### Unsupported Operations

For formats that don't support certain operations:

```go
func (h *Handler) ExtractIR(path, outputDir string) (*plugins.ExtractIRResult, error) {
    return nil, base.UnsupportedOperationError("IR extraction", "SWORD")
}

func (h *Handler) EmitNative(irPath, outputDir string) (*plugins.EmitNativeResult, error) {
    return nil, base.UnsupportedOperationError("native emission", "SWORD")
}
```

## Benefits

1. **Reduced Duplication**: Common patterns are abstracted into reusable functions
2. **Consistency**: All handlers follow the same patterns for common operations
3. **Maintainability**: Bug fixes and improvements in one place benefit all handlers
4. **Flexibility**: Handlers can still provide custom logic where needed
5. **Clarity**: Handler code focuses on format-specific logic

## What Remains Format-Specific

The base package handles the mechanical operations but leaves format-specific logic to handlers:

- Format detection heuristics (content markers, validation)
- Artifact ID extraction from content
- IR extraction/parsing logic
- Native emission/generation logic
- Format-specific metadata

This balance allows handlers to focus on what makes their format unique while leveraging common infrastructure.
