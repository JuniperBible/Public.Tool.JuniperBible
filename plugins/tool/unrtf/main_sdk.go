// Plugin tool-unrtf provides RTF to other format conversions using unrtf.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with unrtf-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputFile string `json:"input_file,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-unrtf <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-unrtf",
		Info: ipc.ToolInfo{
			Name:        "tool-unrtf",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "RTF to other format conversions using unrtf",
			Profiles: []ipc.ProfileInfo{
				{ID: "to-html", Description: "Convert RTF to HTML"},
				{ID: "to-text", Description: "Convert RTF to plain text"},
				{ID: "to-latex", Description: "Convert RTF to LaTeX"},
			},
			Requires: []string{"unrtf"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"to-html":  profileToHTML,
			"to-text":  profileToText,
			"to-latex": profileToLatex,
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

func profileToHTML(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		InputFile:       input,
	})

	cmd := exec.Command("unrtf", "--html", input)
	output, err := cmd.Output()

	outputFile := filepath.Join(req.OutDir, "output.html")
	os.WriteFile(outputFile, output, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "convert_end",
			Data:  map[string]interface{}{"output_file": outputFile, "size": len(output)},
		},
	})

	if err != nil {
		return fmt.Errorf("unrtf failed: %w", err)
	}
	return nil
}

func profileToText(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		InputFile:       input,
	})

	cmd := exec.Command("unrtf", "--text", input)
	output, err := cmd.Output()

	outputFile := filepath.Join(req.OutDir, "output.txt")
	os.WriteFile(outputFile, output, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "convert_end",
			Data:  map[string]interface{}{"output_file": outputFile, "size": len(output)},
		},
	})

	if err != nil {
		return fmt.Errorf("unrtf failed: %w", err)
	}
	return nil
}

func profileToLatex(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		InputFile:       input,
	})

	cmd := exec.Command("unrtf", "--latex", input)
	output, err := cmd.Output()

	outputFile := filepath.Join(req.OutDir, "output.tex")
	os.WriteFile(outputFile, output, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "convert_end",
			Data:  map[string]interface{}{"output_file": outputFile, "size": len(output)},
		},
	})

	if err != nil {
		return fmt.Errorf("unrtf failed: %w", err)
	}
	return nil
}
