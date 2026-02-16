//go:build sdk

// Plugin format-example demonstrates all plugin features using the SDK pattern.
// This is a NOOP plugin for documentation purposes only - it won't process real files.
//
// This example shows:
// - Using the format SDK for simplified plugin development
// - All required functions: Detect, Parse (ingest), Enumerate, Emit
// - IR (Intermediate Representation) support
// - Error handling with proper SDK patterns
//
// To create your own plugin, use this as a reference and replace the noop
// implementations with your actual format parsing logic.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

const (
	// PluginName identifies this plugin
	PluginName = "example"

	// NoopMode disables actual file processing - set to false for a real plugin
	NoopMode = true
)

func main() {
	format.Run(&format.Config{
		Name:       "format-example",
		Extensions: []string{".example"},
		Detect:     detectWrapper,
		Parse:      parseWrapper,
		Emit:       emitWrapper,
		Enumerate:  enumerateWrapper,
	})
}

// detectWrapper wraps detect to match SDK signature: func(path string) (*ipc.DetectResult, error)
func detectWrapper(path string) (*ipc.DetectResult, error) {
	detected, format, err := detect(path)
	if err != nil {
		return nil, err
	}
	return &ipc.DetectResult{
		Detected: detected,
		Format:   format,
	}, nil
}

// parseWrapper wraps parse to match SDK signature: func(path string) (*ir.Corpus, error)
func parseWrapper(path string) (*ir.Corpus, error) {
	// Use empty string for outputDir since SDK handles output directory management
	_, _, corpus, err := parse(path, "")
	return corpus, err
}

// emitWrapper wraps emit to match SDK signature: func(corpus *ir.Corpus, outputDir string) (string, error)
func emitWrapper(corpus *ir.Corpus, outputDir string) (string, error) {
	// Use empty string for formatVariant as default
	return emit(corpus, outputDir, "")
}

// enumerateWrapper wraps enumerate to match SDK signature: func(path string) (*ipc.EnumerateResult, error)
func enumerateWrapper(path string) (*ipc.EnumerateResult, error) {
	ptrEntries, err := enumerate(path)
	if err != nil {
		return nil, err
	}
	// Convert []*ipc.EnumerateEntry to []ipc.EnumerateEntry
	entries := make([]ipc.EnumerateEntry, len(ptrEntries))
	for i, e := range ptrEntries {
		if e != nil {
			entries[i] = *e
		}
	}
	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}

// detect determines if this plugin can handle the given file.
//
// Returns:
//   - detected: bool - Whether this plugin can handle the input
//   - format: string - Format name if detected
//   - reason: string - Human-readable explanation
func detect(path string) (bool, string, error) {
	if NoopMode {
		// In noop mode, always return false to prevent accidental use
		return false, "", fmt.Errorf("noop plugin - for documentation only")
	}

	// Real implementation would:
	// 1. Check if path exists and is a file/directory
	// 2. Check file extension (e.g., .example)
	// 3. Read file header or content to verify format
	// 4. Return true if format matches

	info, err := os.Stat(path)
	if err != nil {
		return false, "", fmt.Errorf("cannot stat: %v", err)
	}

	if info.IsDir() {
		return false, "", fmt.Errorf("path is a directory")
	}

	ext := filepath.Ext(path)
	if ext != ".example" {
		return false, "", fmt.Errorf("not an .example file")
	}

	// In a real plugin, you would verify the content here
	// data, err := os.ReadFile(path)
	// if err != nil { return false, "", err }
	// if !hasExpectedFormat(data) { return false, "", err }

	return true, PluginName, nil
}

// parse reads the native format file and converts it to IR.
//
// Input:
//   - path: Path to source file
//   - outputDir: Directory for output files
//
// Returns:
//   - artifactID: Identifier derived from filename
//   - blobHash: SHA-256 hash of file contents
//   - corpus: IR Corpus structure
func parse(path string, outputDir string) (artifactID string, blobHash string, corpus *ir.Corpus, err error) {
	if NoopMode {
		return "", "", nil, fmt.Errorf("noop plugin - parse not supported")
	}

	// 1. Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read file: %v", err)
	}

	// 2. Store in content-addressed storage
	hashHex, err := ipc.StoreBlob(outputDir, data)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to store blob: %v", err)
	}

	// 3. Generate artifact ID from filename
	artifactID = ipc.ArtifactIDFromPath(path)

	// 4. Parse the file and build IR Corpus
	// Real implementation would:
	// - Parse the native format from 'data'
	// - Build an IR Corpus structure
	// - Handle all markup, verses, formatting, etc.

	// Example IR structure for a Bible:
	corpus = &ir.Corpus{
		ID:            artifactID,
		Version:       "1.0",
		ModuleType:    "bible",
		Versification: "KJV",
		Language:      "en",
		Title:         "Example Bible",
		Description:   "An example Bible for demonstration",
		Publisher:     "Example Publisher",
		SourceFormat:  PluginName,
		Documents: []*ir.Document{
			// Each book is a Document
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					// Each verse is a ContentBlock
					{
						ID:       "Gen.1.1",
						Sequence: 1,
						Text:     "In the beginning God created the heaven and the earth.",
						Anchors: []*ir.Anchor{
							// Anchors mark positions where spans can attach
							{
								ID:       "Gen.1.1.a0",
								Position: 0,
								Spans: []*ir.Span{
									// Spans represent markup (verse markers, formatting, etc)
									{
										ID:            "Gen.1.1.verse",
										Type:          "verse",
										StartAnchorID: "Gen.1.1.a0",
										Ref: &ir.Ref{
											Book:    "Gen",
											Chapter: 1,
											Verse:   1,
											OSISID:  "Gen.1.1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return artifactID, hashHex, corpus, nil
}

// enumerate lists the components within an archive or container.
//
// Input:
//   - path: Path to file/directory to enumerate
//
// Returns:
//   - entries: List of files/components
//
// Each entry contains:
//   - path: string - Relative path within archive
//   - size_bytes: int64 - Size in bytes
//   - is_dir: bool - Whether this is a directory
//   - metadata: map[string]string (optional) - Additional metadata
func enumerate(path string) ([]*ipc.EnumerateEntry, error) {
	if NoopMode {
		return nil, fmt.Errorf("noop plugin - enumerate not supported")
	}

	// For archive formats, enumerate all files:
	// Real implementation would:
	// 1. Open the archive/container
	// 2. List all entries
	// 3. Return metadata for each entry

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %v", err)
	}

	// Example: Single file
	entries := []*ipc.EnumerateEntry{
		{
			Path:      filepath.Base(path),
			SizeBytes: info.Size(),
			IsDir:     false,
			Metadata: map[string]string{
				"format": PluginName,
			},
		},
	}

	// Example: Multiple files in archive
	// entries := []*ipc.EnumerateEntry{
	//     {Path: "book1.txt", SizeBytes: 1024, IsDir: false},
	//     {Path: "book2.txt", SizeBytes: 2048, IsDir: false},
	//     {Path: "metadata/", SizeBytes: 0, IsDir: true},
	// }

	return entries, nil
}

// emit converts Intermediate Representation (IR) to native format.
//
// Input:
//   - corpus: IR Corpus structure
//   - outputDir: Directory for output files
//   - formatVariant: Specific format variant to emit (optional)
//
// Returns:
//   - outputPath: Path to generated native file
func emit(corpus *ir.Corpus, outputDir string, formatVariant string) (string, error) {
	if NoopMode {
		return "", fmt.Errorf("noop plugin - emit not supported")
	}

	// Real implementation would:
	// 1. Convert IR to native format
	// 2. Write the output file
	// 3. Return the path

	// Convert to native format
	// In a real plugin, this would involve:
	// - Iterating through corpus.Documents
	// - Reconstructing the native file structure
	// - Handling spans and anchors to reconstruct markup
	// - Applying format-specific encoding rules

	outputPath := filepath.Join(outputDir, corpus.ID+".example")

	// Example: Simple text output
	output := fmt.Sprintf("# %s\n\n", corpus.Title)
	for _, doc := range corpus.Documents {
		output += fmt.Sprintf("## %s\n\n", doc.Title)
		for _, block := range doc.ContentBlocks {
			output += fmt.Sprintf("%s\n", block.Text)
		}
	}

	if err := os.WriteFile(outputPath, []byte(output), 0600); err != nil {
		return "", fmt.Errorf("failed to write output: %v", err)
	}

	return outputPath, nil
}
