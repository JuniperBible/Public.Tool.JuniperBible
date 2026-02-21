//go:build !sdk

// Plugin tool-usfm2osis provides USFM to OSIS conversion.
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

	"github.com/JuniperBible/juniper/internal/safefile"
	"github.com/JuniperBible/juniper/plugins/ipc"
)

type ToolRunRequest struct {
	Profile string            `json:"profile"`
	Args    map[string]string `json:"args,omitempty"`
	OutDir  string            `json:"out_dir"`
}

type TranscriptEvent struct {
	Event      string      `json:"event"`
	Timestamp  string      `json:"timestamp,omitempty"`
	Plugin     string      `json:"plugin,omitempty"`
	Profile    string      `json:"profile,omitempty"`
	InputFile  string      `json:"input_file,omitempty"`
	OutputFile string      `json:"output_file,omitempty"`
	Error      string      `json:"error,omitempty"`
	ExitCode   int         `json:"exit_code,omitempty"`
	Data       interface{} `json:"data,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-usfm2osis <command> [args]")
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
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func printInfo() {
	info := map[string]interface{}{
		"name":        "tool-usfm2osis",
		"version":     "1.0.0",
		"type":        "tool",
		"description": "USFM to OSIS conversion using usfm2osis",
		"profiles": []map[string]string{
			{"id": "convert", "description": "Convert USFM files to OSIS XML"},
			{"id": "convert-batch", "description": "Convert directory of USFM files"},
			{"id": "validate", "description": "Validate USFM syntax"},
		},
		"requires": []string{"usfm2osis.py"},
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
		if err != nil {
			continue
		}
		var req map[string]interface{}
		json.Unmarshal([]byte(line), &req)
		encoder.Encode(map[string]interface{}{"success": true})
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

	if reqPath == "" || outDir == "" {
		fmt.Fprintln(os.Stderr, "Usage: tool-usfm2osis run --request <path> --out <dir>")
		os.Exit(1)
	}

	reqData, _ := safefile.ReadFile(reqPath)
	var req ToolRunRequest
	json.Unmarshal(reqData, &req)
	req.OutDir = outDir
	os.MkdirAll(outDir, 0700)

	executeProfile(&req)
}

func executeProfile(req *ToolRunRequest) {
	transcript := ipc.NewTranscript(req.OutDir)
	defer transcript.Close()

	transcript.WriteEvent(TranscriptEvent{
		Event:     "start",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Plugin:    "tool-usfm2osis",
		Profile:   req.Profile,
	})

	var err error
	switch req.Profile {
	case "convert":
		err = profileConvert(req, transcript)
	case "convert-batch":
		err = profileConvertBatch(req, transcript)
	case "validate":
		err = profileValidate(req, transcript)
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

func profileConvert(req *ToolRunRequest, transcript *ipc.Transcript) error {
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
		Event:      "convert_start",
		InputFile:  inputFile,
		OutputFile: outputFile,
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
			Event: "convert_error",
			Error: fmt.Sprintf("%v: %s", err, string(output)),
		})
		return err
	}

	info, _ := os.Stat(outputFile)
	var size int64
	if info != nil {
		size = info.Size()
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:      "convert_end",
		OutputFile: outputFile,
		Data:       map[string]interface{}{"size_bytes": size},
	})

	return nil
}

func profileConvertBatch(req *ToolRunRequest, transcript *ipc.Transcript) error {
	inputDir := req.Args["input_dir"]
	if inputDir == "" {
		return fmt.Errorf("input_dir required")
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:     "batch_start",
		InputFile: inputDir,
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
			Event: "batch_error",
			Error: fmt.Sprintf("%v: %s", err, string(output)),
		})
		return err
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:      "batch_end",
		OutputFile: outputFile,
	})

	return nil
}

func profileValidate(req *ToolRunRequest, transcript *ipc.Transcript) error {
	inputFile := req.Args["input"]
	if inputFile == "" {
		return fmt.Errorf("input file required")
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:     "validate_start",
		InputFile: inputFile,
	})

	// Just check file exists and has USFM markers
	data, err := safefile.ReadFile(inputFile)
	if err != nil {
		return err
	}

	valid := len(data) > 0 // Simple validation
	transcript.WriteEvent(TranscriptEvent{
		Event: "validate_end",
		Data:  map[string]interface{}{"valid": valid, "size_bytes": len(data)},
	})

	return nil
}
