//go:build sdk

// Package main implements a Hugo JSON output generator plugin.
// Generates Hugo-compatible JSON data files from Bible modules.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with hugo-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputPath  string `json:"input_path,omitempty"`
	OutputPath string `json:"output_path,omitempty"`
}

// GenerateOptions contains options for Hugo generation.
type GenerateOptions struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"` // "sword", "esword", or auto-detect
}

// GenerateResult contains the result of Hugo generation.
type GenerateResult struct {
	BiblesGenerated int      `json:"bibles_generated"`
	ChaptersWritten int      `json:"chapters_written"`
	OutputFiles     []string `json:"output_files"`
}

// BibleIndex represents the bibles.json index file.
type BibleIndex struct {
	Bibles []BibleMetadata `json:"bibles"`
}

// BibleMetadata contains metadata for a single Bible.
type BibleMetadata struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Abbreviation string   `json:"abbreviation"`
	Language     string   `json:"language"`
	Description  string   `json:"description"`
	Copyright    string   `json:"copyright"`
	Books        []string `json:"books"`
}

// ChapterData represents a single chapter's data.
type ChapterData struct {
	BibleID string      `json:"bible_id"`
	Book    string      `json:"book"`
	Chapter int         `json:"chapter"`
	Verses  []VerseData `json:"verses"`
}

// VerseData represents a single verse.
type VerseData struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-hugo <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-hugo",
		Info: ipc.ToolInfo{
			Name:        "tool-hugo",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "Hugo JSON output generator for Bible modules",
			Profiles: []ipc.ProfileInfo{
				{ID: "generate", Description: "Generate Hugo JSON data files from Bible modules"},
			},
			Requires: []string{},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"generate": profileGenerate,
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

func profileGenerate(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputPath := req.Args["input"]
	if inputPath == "" {
		return fmt.Errorf("missing required argument: input")
	}

	outputPath := req.Args["output"]
	if outputPath == "" {
		return fmt.Errorf("missing required argument: output")
	}

	format := req.Args["format"]

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "generate_start"},
		InputPath:       inputPath,
		OutputPath:      outputPath,
	})

	opts := &GenerateOptions{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Format:     format,
	}

	result, err := Generate(opts)
	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "generate_error",
				Error: err.Error(),
			},
		})
		return fmt.Errorf("generation failed: %w", err)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "generate_end",
			Data: map[string]interface{}{
				"bibles_generated": result.BiblesGenerated,
				"chapters_written": result.ChaptersWritten,
				"output_files":     result.OutputFiles,
			},
		},
		OutputPath: outputPath,
	})

	return nil
}
