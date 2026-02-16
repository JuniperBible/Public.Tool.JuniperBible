//go:build !sdk

// Plugin format-example demonstrates all plugin features.
// This is a NOOP plugin for documentation purposes only - it won't process real files.
//
// This example shows:
// - IPC protocol (JSON stdin/stdout communication)
// - All required commands: detect, ingest, enumerate, extract-ir, emit-native
// - Using the plugins/ipc package for common operations
// - Error handling and response formatting
// - IR (Intermediate Representation) support
// - Content-addressed storage integration
//
// To create your own plugin, use this as a reference and replace the noop
// implementations with your actual format parsing logic.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

const (
	// PluginName identifies this plugin
	PluginName = "example"

	// NoopMode disables actual file processing - set to false for a real plugin
	NoopMode = true
)

func main() {
	// Step 1: Read the IPC request from stdin
	// The host sends a JSON object with "command" and "args" fields
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to read request: %v", err)
		return
	}

	// Step 2: Route to the appropriate command handler
	// Format plugins must implement these commands:
	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

// handleDetect determines if this plugin can handle the given file.
//
// Input args:
//   - path: string (required) - Path to file or directory to check
//
// Output:
//   - detected: bool - Whether this plugin can handle the input
//   - format: string - Format name if detected
//   - reason: string - Human-readable explanation
//
// Example request:
//   {"command": "detect", "args": {"path": "/path/to/file.example"}}
//
// Example response:
//   {"status": "ok", "result": {"detected": true, "format": "example", "reason": "..."}}
func handleDetect(args map[string]interface{}) {
	// Extract the required "path" argument using the ipc helper
	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	if NoopMode {
		// In noop mode, always return false to prevent accidental use
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "noop plugin - for documentation only",
		})
		return
	}

	// Real implementation would:
	// 1. Check if path exists and is a file/directory
	// 2. Check file extension (e.g., .example)
	// 3. Read file header or content to verify format
	// 4. Return true if format matches

	// Example using the ipc.HandleDetect helper for simple cases:
	// ipc.HandleDetect(args, []string{".example"}, []string{"EXAMPLE"}, "example")

	// Example manual implementation:
	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		})
		return
	}

	ext := filepath.Ext(path)
	if ext != ".example" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not an .example file",
		})
		return
	}

	// In a real plugin, you would verify the content here
	// data, err := os.ReadFile(path)
	// if err != nil { ... }
	// if !hasExpectedFormat(data) { ... }

	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   PluginName,
		Reason:   "example format detected",
	})
}

// handleIngest stores the file bytes verbatim in content-addressed storage.
//
// Input args:
//   - path: string (required) - Path to file to ingest
//   - output_dir: string (required) - Directory for content-addressed storage
//
// Output:
//   - artifact_id: string - Identifier derived from filename
//   - blob_sha256: string - SHA-256 hash of file contents
//   - size_bytes: int64 - File size in bytes
//   - metadata: map[string]string - Additional metadata
//
// The file is stored in: output_dir/<hash[:2]>/<hash>
//
// Example request:
//   {"command": "ingest", "args": {"path": "/tmp/file.example", "output_dir": "/tmp/blobs"}}
//
// Example response:
//   {"status": "ok", "result": {"artifact_id": "file", "blob_sha256": "abc123...", ...}}
func handleIngest(args map[string]interface{}) {
	if NoopMode {
		ipc.RespondError("noop plugin - ingest not supported")
		return
	}

	// Extract path and output_dir using the ipc helper
	path, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// For simple file-based formats, use the ipc.HandleIngest helper:
	// ipc.HandleIngest(args, PluginName)

	// Manual implementation:
	// 1. Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	// 2. Store in content-addressed storage
	// The StoreBlob helper creates the directory structure and returns the hash
	hashHex, err := ipc.StoreBlob(outputDir, data)
	if err != nil {
		ipc.RespondErrorf("failed to store blob: %v", err)
		return
	}

	// 3. Generate artifact ID from filename
	artifactID := ipc.ArtifactIDFromPath(path)

	// 4. Return the result
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format":    PluginName,
			"extension": filepath.Ext(path),
		},
	})
}

// handleEnumerate lists the components within an archive or container.
//
// Input args:
//   - path: string (required) - Path to file/directory to enumerate
//
// Output:
//   - entries: []EnumerateEntry - List of files/components
//
// Each entry contains:
//   - path: string - Relative path within archive
//   - size_bytes: int64 - Size in bytes
//   - is_dir: bool - Whether this is a directory
//   - mod_time: string (optional) - Modification time
//   - metadata: map[string]string (optional) - Additional metadata
//
// Example request:
//   {"command": "enumerate", "args": {"path": "/path/to/archive.example"}}
//
// Example response:
//   {"status": "ok", "result": {"entries": [{"path": "file1.txt", "size_bytes": 100, ...}]}}
func handleEnumerate(args map[string]interface{}) {
	if NoopMode {
		ipc.RespondError("noop plugin - enumerate not supported")
		return
	}

	path, err := ipc.StringArg(args, "path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// For single-file formats, use the ipc helper:
	// ipc.HandleEnumerateSingleFile(args, PluginName)

	// For archive formats, enumerate all files:
	// Real implementation would:
	// 1. Open the archive/container
	// 2. List all entries
	// 3. Return metadata for each entry

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}

	// Example: Single file
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata: map[string]string{
					"format": PluginName,
				},
			},
		},
	})

	// Example: Multiple files in archive
	// entries := []ipc.EnumerateEntry{
	//     {Path: "book1.txt", SizeBytes: 1024, IsDir: false},
	//     {Path: "book2.txt", SizeBytes: 2048, IsDir: false},
	//     {Path: "metadata/", SizeBytes: 0, IsDir: true},
	// }
	// ipc.MustRespond(&ipc.EnumerateResult{Entries: entries})
}

// handleExtractIR converts the native format to Intermediate Representation (IR).
//
// Input args:
//   - path: string (required) - Path to source file
//   - output_dir: string (required) - Directory for output files
//
// Output:
//   - ir_path: string - Path to IR JSON file (for large corpuses)
//   - ir: object - Inline IR data (for small corpuses)
//   - loss_class: string - L0-L4 classification
//   - loss_report: LossReport (optional) - Detailed loss information
//
// Loss classes:
//   - L0: Byte-for-byte round-trip (lossless)
//   - L1: Semantically lossless (formatting may differ)
//   - L2: Minor loss (some metadata/structure)
//   - L3: Significant loss (text preserved, markup lost)
//   - L4: Text-only (minimal preservation)
//
// Example request:
//   {"command": "extract-ir", "args": {"path": "/tmp/bible.example", "output_dir": "/tmp/ir"}}
//
// Example response:
//   {"status": "ok", "result": {"ir_path": "/tmp/ir/bible.json", "loss_class": "L0"}}
func handleExtractIR(args map[string]interface{}) {
	if NoopMode {
		ipc.RespondError("noop plugin - extract-ir not supported")
		return
	}

	_, outputDir, err := ipc.PathAndOutputDir(args)
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// Real implementation would:
	// 1. Parse the native format (from path)
	// 2. Build an IR Corpus structure
	// 3. Write IR to JSON file
	// 4. Return the path and loss classification

	// Example IR structure for a Bible:
	corpus := &ipc.Corpus{
		ID:            "example-bible",
		Version:       "1.0",
		ModuleType:    "bible",
		Versification: "KJV",
		Language:      "en",
		Title:         "Example Bible",
		Description:   "An example Bible for demonstration",
		Publisher:     "Example Publisher",
		SourceFormat:  PluginName,
		Documents:     []*ipc.Document{
			// Each book is a Document
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					// Each verse is a ContentBlock
					{
						ID:       "Gen.1.1",
						Sequence: 1,
						Text:     "In the beginning God created the heaven and the earth.",
						Anchors: []*ipc.Anchor{
							// Anchors mark positions where spans can attach
							{
								ID:       "Gen.1.1.a0",
								Position: 0,
								Spans: []*ipc.Span{
									// Spans represent markup (verse markers, formatting, etc)
									{
										ID:            "Gen.1.1.verse",
										Type:          "verse",
										StartAnchorID: "Gen.1.1.a0",
										Ref: &ipc.Ref{
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

	// Write IR to file
	irPath := filepath.Join(outputDir, "corpus.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to marshal IR: %v", err)
		return
	}

	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}

	// Return the result
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L1", // Semantically lossless
		LossReport: &ipc.LossReport{
			SourceFormat: PluginName,
			TargetFormat: "ir",
			LossClass:    "L1",
			Warnings: []string{
				"Example warning: some formatting may differ",
			},
		},
	})
}

// handleEmitNative converts Intermediate Representation (IR) to native format.
//
// Input args:
//   - ir_path: string (required) - Path to IR JSON file
//   - output_dir: string (required) - Directory for output files
//   - format: string (optional) - Specific format variant to emit
//
// Output:
//   - output_path: string - Path to generated native file
//   - format: string - Output format name
//   - loss_class: string - L0-L4 classification
//   - loss_report: LossReport (optional) - Detailed loss information
//
// Example request:
//   {"command": "emit-native", "args": {"ir_path": "/tmp/ir/corpus.json", "output_dir": "/tmp/out"}}
//
// Example response:
//   {"status": "ok", "result": {"output_path": "/tmp/out/bible.example", "format": "example", ...}}
func handleEmitNative(args map[string]interface{}) {
	if NoopMode {
		ipc.RespondError("noop plugin - emit-native not supported")
		return
	}

	// Extract required arguments
	irPath, err := ipc.StringArg(args, "ir_path")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	outputDir, err := ipc.StringArg(args, "output_dir")
	if err != nil {
		ipc.RespondError(err.Error())
		return
	}

	// Optional format variant
	format := ipc.StringArgOr(args, "format", PluginName)

	// Real implementation would:
	// 1. Read and parse the IR JSON file
	// 2. Convert IR to native format
	// 3. Write the output file
	// 4. Return the path and loss classification

	// Read IR
	irData, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

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

	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		ipc.RespondErrorf("failed to write output: %v", err)
		return
	}

	// Return the result
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     format,
		LossClass:  "L1", // Semantically lossless
		LossReport: &ipc.LossReport{
			SourceFormat: "ir",
			TargetFormat: format,
			LossClass:    "L1",
			Warnings: []string{
				"Example warning: some IR features may not round-trip exactly",
			},
		},
	})
}
