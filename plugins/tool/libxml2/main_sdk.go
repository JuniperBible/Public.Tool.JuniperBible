//go:build sdk

// Plugin tool-libxml2 provides XML validation and transformation using libxml2.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with libxml2-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputFile string `json:"input_file,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-libxml2 <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-libxml2",
		Info: ipc.ToolInfo{
			Name:        "tool-libxml2",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "XML validation and transformation using libxml2",
			Profiles: []ipc.ProfileInfo{
				{ID: "validate", Description: "Validate XML against schema"},
				{ID: "xpath", Description: "Extract content using XPath"},
				{ID: "xslt", Description: "Transform XML using XSLT"},
				{ID: "format", Description: "Format/pretty-print XML"},
			},
			Requires: []string{"xmllint", "xsltproc"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"validate": profileValidate,
			"xpath":    profileXPath,
			"xslt":     profileXSLT,
			"format":   profileFormat,
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

func profileValidate(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "validate_start"},
		InputFile:       input,
	})

	args := []string{"--noout"}
	if schema := req.Args["schema"]; schema != "" {
		args = append(args, "--schema", schema)
	}
	args = append(args, input)

	cmd := exec.Command("xmllint", args...)
	output, err := cmd.CombinedOutput()

	valid := err == nil

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "validate_end",
			Data: map[string]interface{}{
				"valid":  valid,
				"output": string(output),
			},
		},
	})

	if !valid {
		return fmt.Errorf("validation failed: %s", output)
	}
	return nil
}

func profileXPath(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	xpath := req.Args["xpath"]
	if input == "" || xpath == "" {
		return fmt.Errorf("input and xpath required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "xpath_start"},
		InputFile:       input,
	})

	cmd := exec.Command("xmllint", "--xpath", xpath, input)
	output, err := cmd.Output()

	resultFile := filepath.Join(req.OutDir, "xpath_result.txt")
	os.WriteFile(resultFile, output, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "xpath_end",
			Data:  map[string]interface{}{"result_file": resultFile, "result_size": len(output)},
		},
	})

	if err != nil {
		return fmt.Errorf("xpath failed: %w", err)
	}
	return nil
}

func profileXSLT(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	stylesheet := req.Args["stylesheet"]
	if input == "" || stylesheet == "" {
		return fmt.Errorf("input and stylesheet required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "xslt_start"},
		InputFile:       input,
	})

	outputFile := filepath.Join(req.OutDir, "transformed.xml")
	cmd := exec.Command("xsltproc", "-o", outputFile, stylesheet, input)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "xslt_error",
				Error: fmt.Sprintf("%v: %s", err, output),
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
			Event: "xslt_end",
			Data:  map[string]interface{}{"output_file": outputFile, "size": size},
		},
	})

	return nil
}

func profileFormat(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "format_start"},
		InputFile:       input,
	})

	cmd := exec.Command("xmllint", "--format", input)
	output, err := cmd.Output()

	outputFile := filepath.Join(req.OutDir, "formatted.xml")
	os.WriteFile(outputFile, output, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "format_end",
			Data:  map[string]interface{}{"output_file": outputFile, "size": len(output)},
		},
	})

	if err != nil {
		return fmt.Errorf("format failed: %w", err)
	}
	return nil
}
