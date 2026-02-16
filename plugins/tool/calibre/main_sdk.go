// Plugin tool-calibre provides e-book format conversions using Calibre.
//
// This is a TOOL plugin (not a format plugin). It runs Calibre tools as
// reference implementations to produce deterministic transcripts.
//
// Profiles:
//   - convert: Convert between e-book formats
//   - create-epub: Create EPUB with metadata
//   - epub-metadata: Extract or set EPUB metadata
//   - list-formats: List supported formats
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with calibre-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputFile  string `json:"input_file,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
	FromFormat string `json:"from_format,omitempty"`
	ToFormat   string `json:"to_format,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-calibre <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-calibre",
		Info: ipc.ToolInfo{
			Name:        "tool-calibre",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "E-book format conversions using Calibre",
			Profiles: []ipc.ProfileInfo{
				{ID: "convert", Description: "Convert between e-book formats"},
				{ID: "create-epub", Description: "Create EPUB with metadata"},
				{ID: "epub-metadata", Description: "Extract or set EPUB metadata"},
				{ID: "list-formats", Description: "List supported formats"},
			},
			Requires: []string{"ebook-convert", "ebook-meta"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"convert":       profileConvert,
			"create-epub":   profileCreateEpub,
			"epub-metadata": profileEpubMetadata,
			"list-formats":  profileListFormats,
		},
	}

	switch os.Args[1] {
	case "info":
		ipc.PrintToolInfo(config.Info)
	case "run":
		reqPath, outDir := ipc.ParseToolFlags()
		req := ipc.LoadToolRequest(reqPath, outDir)
		ipc.ExecuteWithTranscript(req, config)
	case "ipc":
		ipc.RunStandardToolIPC(config)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func profileConvert(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputFile := req.Args["input"]
	if inputFile == "" {
		return fmt.Errorf("input file required for convert")
	}

	toFormat := req.Args["to"]
	if toFormat == "" {
		return fmt.Errorf("output format (to) required for convert")
	}

	// Determine output filename
	outputFile := req.Args["output"]
	if outputFile == "" {
		base := filepath.Base(inputFile)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		outputFile = filepath.Join(req.OutDir, name+"."+toFormat)
	} else if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(req.OutDir, outputFile)
	}

	// Detect input format from extension
	inputExt := strings.TrimPrefix(filepath.Ext(inputFile), ".")

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		InputFile:       inputFile,
		OutputFile:      outputFile,
		FromFormat:      inputExt,
		ToFormat:        toFormat,
	})

	// Build ebook-convert command
	args := []string{inputFile, outputFile}

	// Add optional parameters
	if title := req.Args["title"]; title != "" {
		args = append(args, "--title", title)
	}
	if author := req.Args["author"]; author != "" {
		args = append(args, "--authors", author)
	}

	cmd := exec.Command("ebook-convert", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "convert_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
			InputFile: inputFile,
			ToFormat:  toFormat,
		})
		return fmt.Errorf("ebook-convert failed: %w: %s", err, string(output))
	}

	// Get output file info
	info, _ := os.Stat(outputFile)
	var sizeBytes int64
	if info != nil {
		sizeBytes = info.Size()
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "convert_end",
			Data: map[string]interface{}{
				"size_bytes": sizeBytes,
			},
		},
		OutputFile: outputFile,
		ToFormat:   toFormat,
	})

	return nil
}

func profileCreateEpub(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputFile := req.Args["input"]
	if inputFile == "" {
		return fmt.Errorf("input file required for create-epub")
	}

	// Determine output filename
	outputFile := req.Args["output"]
	if outputFile == "" {
		base := filepath.Base(inputFile)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		outputFile = filepath.Join(req.OutDir, name+".epub")
	} else if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(req.OutDir, outputFile)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "epub_start"},
		InputFile:       inputFile,
		OutputFile:      outputFile,
	})

	// Build ebook-convert command
	args := []string{inputFile, outputFile}

	// Add metadata options
	if title := req.Args["title"]; title != "" {
		args = append(args, "--title", title)
	}
	if author := req.Args["author"]; author != "" {
		args = append(args, "--authors", author)
	}
	if cover := req.Args["cover"]; cover != "" {
		args = append(args, "--cover", cover)
	}
	if language := req.Args["language"]; language != "" {
		args = append(args, "--language", language)
	}

	cmd := exec.Command("ebook-convert", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "epub_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
			InputFile: inputFile,
		})
		return fmt.Errorf("ebook-convert failed: %w: %s", err, string(output))
	}

	// Get output file info
	info, _ := os.Stat(outputFile)
	var sizeBytes int64
	if info != nil {
		sizeBytes = info.Size()
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "epub_end",
			Data: map[string]interface{}{
				"size_bytes": sizeBytes,
			},
		},
		OutputFile: outputFile,
	})

	return nil
}

func profileEpubMetadata(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputFile := req.Args["input"]
	if inputFile == "" {
		return fmt.Errorf("input file required for epub-metadata")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "metadata_start"},
		InputFile:       inputFile,
	})

	// Use ebook-meta to get metadata
	cmd := exec.Command("ebook-meta", inputFile)
	output, err := cmd.Output()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "metadata_error",
				Error: err.Error(),
			},
			InputFile: inputFile,
		})
		return fmt.Errorf("ebook-meta failed: %w", err)
	}

	// Parse metadata from output
	metadata := parseEbookMetaOutput(string(output))

	// Write metadata to file
	metadataFile := filepath.Join(req.OutDir, "metadata.json")
	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(metadataFile, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "metadata_end",
			Data:  metadata,
		},
		OutputFile: metadataFile,
	})

	return nil
}

func parseEbookMetaOutput(output string) map[string]string {
	metadata := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if key != "" && value != "" {
				metadata[key] = value
			}
		}
	}
	return metadata
}

func profileListFormats(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(ipc.TranscriptEvent{
		Event: "list_formats_start",
	})

	// Common Calibre input formats
	inputFormats := []string{
		"azw", "azw3", "azw4", "cbz", "cbr", "cb7", "cbc", "chm",
		"djvu", "docx", "epub", "fb2", "fbz", "html", "htmlz",
		"lit", "lrf", "mobi", "odt", "pdf", "prc", "pdb", "pml",
		"rb", "rtf", "snb", "tcr", "txt", "txtz",
	}

	// Common Calibre output formats
	outputFormats := []string{
		"azw3", "docx", "epub", "fb2", "htmlz", "lit", "lrf",
		"mobi", "oeb", "pdb", "pdf", "pml", "rb", "rtf", "snb",
		"tcr", "txt", "txtz", "zip",
	}

	// Write formats to file
	formatsFile := filepath.Join(req.OutDir, "formats.json")
	formatsData := map[string]interface{}{
		"input_formats":  inputFormats,
		"output_formats": outputFormats,
		"input_count":    len(inputFormats),
		"output_count":   len(outputFormats),
	}
	formatsJSON, _ := json.MarshalIndent(formatsData, "", "  ")
	if err := os.WriteFile(formatsFile, formatsJSON, 0644); err != nil {
		return fmt.Errorf("failed to write formats: %w", err)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "list_formats_end",
			Data:  formatsData,
		},
		OutputFile: formatsFile,
	})

	return nil
}
