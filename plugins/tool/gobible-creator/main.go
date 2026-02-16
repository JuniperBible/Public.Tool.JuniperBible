//go:build !sdk

// Plugin tool-gobible-creator provides GoBible format creation for feature phones.
package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
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
		"name":        "tool-gobible-creator",
		"version":     "1.0.0",
		"type":        "tool",
		"description": "GoBible format creation for feature phones",
		"profiles": []map[string]string{
			{"id": "create", "description": "Create GoBible JAR from OSIS/ThML"},
			{"id": "info", "description": "Display collection info"},
			{"id": "validate", "description": "Validate source files before conversion"},
		},
		"requires": []string{"java", "gobiblecreator.jar"},
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
		Plugin:    "tool-gobible-creator",
		Profile:   req.Profile,
	})

	var err error
	switch req.Profile {
	case "create":
		err = profileCreate(req, transcript)
	case "info":
		err = profileInfo(req, transcript)
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

// OSIS structures for parsing
type OSIS struct {
	XMLName  xml.Name `xml:"osis"`
	OsisText OsisText `xml:"osisText"`
}

type OsisText struct {
	OsisIDWork string `xml:"osisIDWork,attr"`
	Lang       string `xml:"lang,attr"`
	Header     Header `xml:"header"`
}

type Header struct {
	Work Work `xml:"work"`
}

type Work struct {
	OsisWork string `xml:"osisWork,attr"`
}

func profileCreate(req *ToolRunRequest, transcript *ipc.Transcript) error {
	collection := req.Args["collection"]
	if collection == "" {
		return fmt.Errorf("collection file required")
	}

	// SEC-007: Validate and sanitize collection path to prevent command injection
	// Convert to absolute path and validate it exists
	absCollection, err := filepath.Abs(collection)
	if err != nil {
		return fmt.Errorf("invalid collection path: %w", err)
	}

	// Verify the file exists and is a regular file
	fileInfo, err := os.Stat(absCollection)
	if err != nil {
		return fmt.Errorf("collection file not accessible: %w", err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("collection path must be a file, not a directory")
	}

	// Ensure path doesn't contain dangerous characters or path traversal attempts
	cleanPath := filepath.Clean(absCollection)
	if cleanPath != absCollection {
		return fmt.Errorf("collection path contains path traversal or invalid characters")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "create_start", InputFile: cleanPath})

	// Find GoBibleCreator.jar
	jarPath := findGoBibleCreator()
	if jarPath == "" {
		return fmt.Errorf("gobiblecreator.jar not found")
	}

	// Validate jar path as well
	absJarPath, err := filepath.Abs(jarPath)
	if err != nil {
		return fmt.Errorf("invalid jar path: %w", err)
	}
	if _, err := os.Stat(absJarPath); err != nil {
		return fmt.Errorf("jar file not accessible: %w", err)
	}

	outputName := req.Args["output"]
	if outputName == "" {
		outputName = "output.jar"
	}

	// Run GoBibleCreator with validated absolute paths
	cmd := exec.Command("java", "-jar", absJarPath, cleanPath)
	cmd.Dir = req.OutDir
	output, err := cmd.CombinedOutput()

	transcript.WriteEvent(TranscriptEvent{
		Event: "create_end",
		Data: map[string]interface{}{
			"output":     string(output),
			"jar_path":   absJarPath,
			"collection": cleanPath,
		},
	})

	if err != nil {
		return fmt.Errorf("gobiblecreator failed: %w: %s", err, output)
	}
	return nil
}

func profileInfo(req *ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	// Validate and sanitize input path
	absInput, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}
	cleanInput := filepath.Clean(absInput)
	if cleanInput != absInput {
		return fmt.Errorf("input path contains path traversal or invalid characters")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "info_start", InputFile: cleanInput})

	// Parse OSIS file to extract info
	data, err := safefile.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	var osis OSIS
	if err := xml.Unmarshal(data, &osis); err != nil {
		return fmt.Errorf("failed to parse OSIS: %w", err)
	}

	info := map[string]interface{}{
		"work_id":  osis.OsisText.OsisIDWork,
		"language": osis.OsisText.Lang,
		"format":   "OSIS",
	}

	// Write info to file
	infoPath := filepath.Join(req.OutDir, "info.json")
	infoData, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(infoPath, infoData, 0644)

	transcript.WriteEvent(TranscriptEvent{
		Event: "info_end",
		Data:  info,
	})

	return nil
}

func profileValidate(req *ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	// Validate and sanitize input path
	absInput, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}
	cleanInput := filepath.Clean(absInput)
	if cleanInput != absInput {
		return fmt.Errorf("input path contains path traversal or invalid characters")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "validate_start", InputFile: cleanInput})

	// Parse OSIS file to validate
	data, err := safefile.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	var osis OSIS
	if err := xml.Unmarshal(data, &osis); err != nil {
		transcript.WriteEvent(TranscriptEvent{
			Event: "validate_end",
			Data: map[string]interface{}{
				"valid": false,
				"error": err.Error(),
			},
		})
		return fmt.Errorf("invalid OSIS: %w", err)
	}

	// Basic validation
	valid := true
	var issues []string

	if osis.OsisText.OsisIDWork == "" {
		valid = false
		issues = append(issues, "missing osisIDWork attribute")
	}

	transcript.WriteEvent(TranscriptEvent{
		Event: "validate_end",
		Data: map[string]interface{}{
			"valid":  valid,
			"issues": issues,
		},
	})

	if !valid {
		return fmt.Errorf("validation failed: %v", issues)
	}
	return nil
}

func findGoBibleCreator() string {
	paths := []string{
		"/usr/share/java/gobiblecreator.jar",
		"/usr/share/java/GoBibleCreator.jar",
		"/opt/gobiblecreator/GoBibleCreator.jar",
		"/opt/gobible/GoBibleCreator.jar",
		"gobiblecreator.jar",
		"GoBibleCreator.jar",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Check GOBIBLE_JAR env var
	if jar := os.Getenv("GOBIBLE_JAR"); jar != "" {
		if _, err := os.Stat(jar); err == nil {
			return jar
		}
	}
	return ""
}
