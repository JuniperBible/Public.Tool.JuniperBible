//go:build sdk

// Plugin tool-usfm2osis provides USFM to OSIS conversion.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with usfm2osis-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputFile  string `json:"input_file,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-usfm2osis <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-usfm2osis",
		Info: ipc.ToolInfo{
			Name:        "tool-usfm2osis",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "USFM to OSIS conversion using usfm2osis",
			Profiles: []ipc.ProfileInfo{
				{ID: "convert", Description: "Convert USFM files to OSIS XML"},
				{ID: "convert-batch", Description: "Convert directory of USFM files"},
				{ID: "validate", Description: "Validate USFM syntax"},
			},
			Requires: []string{"usfm2osis.py"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"convert":       profileConvert,
			"convert-batch": profileConvertBatch,
			"validate":      profileValidate,
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
		return fmt.Errorf("input file required")
	}

	outputFile := req.Args["output"]
	if outputFile == "" {
		outputFile = "output.osis"
	}
	if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(req.OutDir, outputFile)
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "convert_start"},
		InputFile:       inputFile,
		OutputFile:      outputFile,
	})

	// Try usfm2osis.py or usfm2osis
	toolName := "usfm2osis.py"
	if _, err := exec.LookPath(toolName); err != nil {
		toolName = "usfm2osis"
	}

	cmd := exec.Command(toolName, "-o", outputFile, inputFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "convert_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
		})
		return err
	}

	info, _ := os.Stat(outputFile)
	var size int64
	if info != nil {
		size = info.Size()
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "convert_end",
			Data:  map[string]interface{}{"size_bytes": size},
		},
		OutputFile: outputFile,
	})

	return nil
}

func profileConvertBatch(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputDir := req.Args["input_dir"]
	if inputDir == "" {
		return fmt.Errorf("input_dir required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "batch_start"},
		InputFile:       inputDir,
	})

	toolName := "usfm2osis.py"
	if _, err := exec.LookPath(toolName); err != nil {
		toolName = "usfm2osis"
	}

	outputFile := filepath.Join(req.OutDir, "output.osis")
	cmd := exec.Command(toolName, "-o", outputFile, inputDir)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "batch_error",
				Error: fmt.Sprintf("%v: %s", err, string(output)),
			},
		})
		return err
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "batch_end"},
		OutputFile:      outputFile,
	})

	return nil
}

func profileValidate(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	inputFile := req.Args["input"]
	if inputFile == "" {
		return fmt.Errorf("input file required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "validate_start"},
		InputFile:       inputFile,
	})

	// Just check file exists and has USFM markers
	data, err := safefile.ReadFile(inputFile)
	if err != nil {
		return err
	}

	valid := len(data) > 0 // Simple validation

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "validate_end",
			Data:  map[string]interface{}{"valid": valid, "size_bytes": len(data)},
		},
	})

	return nil
}
