//go:build !sdk

// Plugin tool-libxml2 provides XML validation and transformation using libxml2.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

type ToolRunRequest struct {
	Profile string            `json:"profile"`
	Args    map[string]string `json:"args,omitempty"`
	OutDir  string            `json:"out_dir"`
}

type TranscriptEvent struct {
	Event     string      `json:"event"`
	Timestamp string      `json:"timestamp,omitempty"`
	Plugin    string      `json:"plugin,omitempty"`
	Profile   string      `json:"profile,omitempty"`
	InputFile string      `json:"input_file,omitempty"`
	Error     string      `json:"error,omitempty"`
	ExitCode  int         `json:"exit_code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}

	switch os.Args[1] {
	case "info":
		printInfo()
	case "run":
		runTool()
	case "ipc":
		runIPC()
	default:
		os.Exit(1)
	}
}

func printInfo() {
	info := map[string]interface{}{
		"name":        "tool-libxml2",
		"version":     "1.0.0",
		"type":        "tool",
		"description": "XML validation and transformation using libxml2",
		"profiles": []map[string]string{
			{"id": "validate", "description": "Validate XML against schema"},
			{"id": "xpath", "description": "Extract content using XPath"},
			{"id": "xslt", "description": "Transform XML using XSLT"},
			{"id": "format", "description": "Format/pretty-print XML"},
		},
		"requires": []string{"xmllint", "xsltproc"},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(info)
}

func runIPC() {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		encoder.Encode(map[string]interface{}{"success": true})
		_ = line
	}
}

func runTool() {
	var reqPath, outDir string
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--request":
			if i+1 < len(os.Args) {
				reqPath = os.Args[i+1]
				i++
			}
		case "--out":
			if i+1 < len(os.Args) {
				outDir = os.Args[i+1]
				i++
			}
		}
	}

	reqData, _ := safefile.ReadFile(reqPath)
	var req ToolRunRequest
	json.Unmarshal(reqData, &req)
	req.OutDir = outDir
	os.MkdirAll(outDir, 0755)
	executeProfile(&req)
}

func executeProfile(req *ToolRunRequest) {
	transcript := ipc.NewTranscript(req.OutDir)
	defer transcript.Close()

	transcript.WriteEvent(TranscriptEvent{
		Event:     "start",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Plugin:    "tool-libxml2",
		Profile:   req.Profile,
	})

	var err error
	switch req.Profile {
	case "validate":
		err = profileValidate(req, transcript)
	case "xpath":
		err = profileXPath(req, transcript)
	case "xslt":
		err = profileXSLT(req, transcript)
	case "format":
		err = profileFormat(req, transcript)
	default:
		err = fmt.Errorf("unknown profile: %s", req.Profile)
	}

	exitCode := 0
	if err != nil {
		exitCode = 1
		transcript.WriteEvent(TranscriptEvent{Event: "error", Error: err.Error()})
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:     "end",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ExitCode:  exitCode,
	})
}

func profileValidate(req *ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "validate_start", InputFile: input})

	args := []string{"--noout"}
	if schema := req.Args["schema"]; schema != "" {
		args = append(args, "--schema", schema)
	}
	args = append(args, input)

	cmd := exec.Command("xmllint", args...)
	output, err := cmd.CombinedOutput()

	valid := err == nil
	transcript.WriteEvent(TranscriptEvent{
		Event: "validate_end",
		Data: map[string]interface{}{
			"valid":  valid,
			"output": string(output),
		},
	})

	if !valid {
		return fmt.Errorf("validation failed: %s", output)
	}
	return nil
}

func profileXPath(req *ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	xpath := req.Args["xpath"]
	if input == "" || xpath == "" {
		return fmt.Errorf("input and xpath required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "xpath_start", InputFile: input})

	cmd := exec.Command("xmllint", "--xpath", xpath, input)
	output, err := cmd.Output()

	resultFile := filepath.Join(req.OutDir, "xpath_result.txt")
	os.WriteFile(resultFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		Event: "xpath_end",
		Data:  map[string]interface{}{"result_file": resultFile, "result_size": len(output)},
	})

	if err != nil {
		return fmt.Errorf("xpath failed: %w", err)
	}
	return nil
}

func profileXSLT(req *ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	stylesheet := req.Args["stylesheet"]
	if input == "" || stylesheet == "" {
		return fmt.Errorf("input and stylesheet required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "xslt_start", InputFile: input})

	outputFile := filepath.Join(req.OutDir, "transformed.xml")
	cmd := exec.Command("xsltproc", "-o", outputFile, stylesheet, input)
	output, err := cmd.CombinedOutput()

	if err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event: "xslt_error",
			Error: fmt.Sprintf("%v: %s", err, output),
		})
		return err
	}

	info, _ := os.Stat(outputFile)
	var size int64
	if info != nil {
		size = info.Size()
	}

	transcript.WriteEvent(TranscriptEvent{
		Event: "xslt_end",
		Data:  map[string]interface{}{"output_file": outputFile, "size": size},
	})

	return nil
}

func profileFormat(req *ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "format_start", InputFile: input})

	cmd := exec.Command("xmllint", "--format", input)
	output, err := cmd.Output()

	outputFile := filepath.Join(req.OutDir, "formatted.xml")
	os.WriteFile(outputFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		Event: "format_end",
		Data:  map[string]interface{}{"output_file": outputFile, "size": len(output)},
	})

	if err != nil {
		return fmt.Errorf("format failed: %w", err)
	}
	return nil
}
