// Plugin tool-pandoc provides document format conversions using Pandoc.
//
// This is a TOOL plugin (not a format plugin). It runs Pandoc as a reference
// implementation to produce deterministic transcripts.
//
// Profiles:
//   - convert: Convert between document formats
//   - create-epub: Create EPUB with metadata and cover
//   - extract-metadata: Extract document metadata as JSON
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

// TranscriptEvent extends the base event with pandoc-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputFile  string `json:"input_file,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
	FromFormat string `json:"from_format,omitempty"`
	ToFormat   string `json:"to_format,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-pandoc <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-pandoc",
		Info: ipc.ToolInfo{
			Name:        "tool-pandoc",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "Document format conversions using Pandoc",
			Profiles: []ipc.ProfileInfo{
				{ID: "convert", Description: "Convert between document formats"},
				{ID: "create-epub", Description: "Create EPUB with metadata and cover"},
				{ID: "extract-metadata", Description: "Extract document metadata as JSON"},
				{ID: "list-formats", Description: "List supported formats"},
			},
			Requires: []string{"pandoc"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"convert":          profileConvert,
			"create-epub":      profileCreateEPUB,
			"extract-metadata": profileExtractMetadata,
			"list-formats":     profileListFormats,
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

	outputFormat := req.Args["to"]
	if outputFormat == "" {
		return fmt.Errorf("output format (--to) required for convert")
	}

	inputFormat := req.Args["from"]

	// Determine output filename
	outputFile := req.Args["output"]
	if outputFile == "" {
		base := filepath.Base(inputFile)
		ext := filepath.Ext(base)
		name := base[:len(base)-len(ext)]
		outputFile = filepath.Join(req.OutDir, name+"."+outputFormat)
	} else if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(req.OutDir, outputFile)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		InputFile:       inputFile,
		OutputFile:      outputFile,
		FromFormat:      inputFormat,
		ToFormat:        outputFormat,
	})

	// Build pandoc command
	args := []string{}

	if inputFormat != "" {
		args = append(args, "-f", inputFormat)
	}
	args = append(args, "-t", outputFormat)
	args = append(args, "-o", outputFile)
	args = append(args, inputFile)

	cmd := exec.Command("pandoc", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "convert_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
			InputFile: inputFile,
			ToFormat:  outputFormat,
		})
		return fmt.Errorf("pandoc failed: %w: %s", err, string(output))
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
				"size_bytes":    sizeBytes,
				"pandoc_output": string(output),
			},
		},
		OutputFile: outputFile,
		ToFormat:   outputFormat,
	})

	return nil
}

func profileCreateEPUB(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
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

	// Build pandoc command for EPUB
	args := []string{"-o", outputFile}

	// Optional metadata file
	if metadataFile := req.Args["metadata"]; metadataFile != "" {
		args = append(args, "--metadata-file="+metadataFile)
	}

	// Optional cover image
	if coverImage := req.Args["cover"]; coverImage != "" {
		args = append(args, "--epub-cover-image="+coverImage)
	}

	// Optional CSS
	if css := req.Args["css"]; css != "" {
		args = append(args, "--css="+css)
	}

	// Optional title
	if title := req.Args["title"]; title != "" {
		args = append(args, "--metadata", "title="+title)
	}

	// Optional author
	if author := req.Args["author"]; author != "" {
		args = append(args, "--metadata", "author="+author)
	}

	args = append(args, inputFile)

	cmd := exec.Command("pandoc", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "epub_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
			InputFile: inputFile,
		})
		return fmt.Errorf("pandoc epub failed: %w: %s", err, string(output))
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

func profileExtractMetadata(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputFile := req.Args["input"]
	if inputFile == "" {
		return fmt.Errorf("input file required for extract-metadata")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "metadata_start"},
		InputFile:       inputFile,
	})

	// Use pandoc to extract metadata as JSON
	cmd := exec.Command("pandoc", "-t", "json", inputFile)
	output, err := cmd.Output()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "metadata_error",
				Error: err.Error(),
			},
			InputFile: inputFile,
		})
		return fmt.Errorf("pandoc metadata extraction failed: %w", err)
	}

	// Parse the JSON to extract just metadata
	var doc map[string]interface{}
	if err := json.Unmarshal(output, &doc); err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "metadata_error",
				Error: "failed to parse pandoc JSON output",
			},
		})
		return err
	}

	metadata := doc["meta"]
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

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

func profileListFormats(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	transcript.WriteEvent(ipc.TranscriptEvent{
		Event: "list_formats_start",
	})

	// Get input formats
	inputCmd := exec.Command("pandoc", "--list-input-formats")
	inputOutput, err := inputCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list input formats: %w", err)
	}

	inputFormats := strings.Split(strings.TrimSpace(string(inputOutput)), "\n")

	// Get output formats
	outputCmd := exec.Command("pandoc", "--list-output-formats")
	outputOutput, err := outputCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list output formats: %w", err)
	}

	outputFormats := strings.Split(strings.TrimSpace(string(outputOutput)), "\n")

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
