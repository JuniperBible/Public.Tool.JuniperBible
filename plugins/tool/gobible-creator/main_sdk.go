//go:build sdk

// Plugin tool-gobible-creator provides GoBible format creation for feature phones.
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/internal/safefile"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TranscriptEvent extends the base event with gobible-creator-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	InputFile string `json:"input_file,omitempty"`
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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-gobible-creator <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-gobible-creator",
		Info: ipc.ToolInfo{
			Name:        "tool-gobible-creator",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "GoBible format creation for feature phones",
			Profiles: []ipc.ProfileInfo{
				{ID: "create", Description: "Create GoBible JAR from OSIS/ThML"},
				{ID: "info", Description: "Display collection info"},
				{ID: "validate", Description: "Validate source files before conversion"},
			},
			Requires: []string{"java", "gobiblecreator.jar"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"create":   profileCreate,
			"info":     profileInfo,
			"validate": profileValidate,
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

func profileCreate(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	collection := req.Args["collection"]
	if collection == "" {
		return fmt.Errorf("collection file required")
	}

	// SEC-007: Validate and sanitize collection path to prevent command injection
	absCollection, err := filepath.Abs(collection)
	if err != nil {
		return fmt.Errorf("invalid collection path: %w", err)
	}

	fileInfo, err := os.Stat(absCollection)
	if err != nil {
		return fmt.Errorf("collection file not accessible: %w", err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("collection path must be a file, not a directory")
	}

	cleanPath := filepath.Clean(absCollection)
	if cleanPath != absCollection {
		return fmt.Errorf("collection path contains path traversal or invalid characters")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "create_start"},
		InputFile:       cleanPath,
	})

	jarPath := findGoBibleCreator()
	if jarPath == "" {
		return fmt.Errorf("gobiblecreator.jar not found")
	}

	absJarPath, err := filepath.Abs(jarPath)
	if err != nil {
		return fmt.Errorf("invalid jar path: %w", err)
	}
	if _, err := os.Stat(absJarPath); err != nil {
		return fmt.Errorf("jar file not accessible: %w", err)
	}

	// Run GoBibleCreator with validated absolute paths
	cmd := exec.Command("java", "-jar", absJarPath, cleanPath)
	cmd.Dir = req.OutDir
	output, err := cmd.CombinedOutput()

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "create_end",
			Data: map[string]interface{}{
				"output":     string(output),
				"jar_path":   absJarPath,
				"collection": cleanPath,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("gobiblecreator failed: %w: %s", err, output)
	}
	return nil
}

func profileInfo(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	absInput, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}
	cleanInput := filepath.Clean(absInput)
	if cleanInput != absInput {
		return fmt.Errorf("input path contains path traversal or invalid characters")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "info_start"},
		InputFile:       cleanInput,
	})

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

	infoPath := filepath.Join(req.OutDir, "info.json")
	infoData, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(infoPath, infoData, 0644)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "info_end",
			Data:  info,
		},
	})

	return nil
}

func profileValidate(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	input := req.Args["input"]
	if input == "" {
		return fmt.Errorf("input required")
	}

	absInput, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}
	cleanInput := filepath.Clean(absInput)
	if cleanInput != absInput {
		return fmt.Errorf("input path contains path traversal or invalid characters")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "validate_start"},
		InputFile:       cleanInput,
	})

	data, err := safefile.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	var osis OSIS
	if err := xml.Unmarshal(data, &osis); err != nil {
		transcript.WriteEvent(TranscriptEvent{
			TranscriptEvent: ipc.TranscriptEvent{
				Event: "validate_end",
				Data: map[string]interface{}{
					"valid": false,
					"error": err.Error(),
				},
			},
		})
		return fmt.Errorf("invalid OSIS: %w", err)
	}

	valid := true
	var issues []string

	if osis.OsisText.OsisIDWork == "" {
		valid = false
		issues = append(issues, "missing osisIDWork attribute")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "validate_end",
			Data: map[string]interface{}{
				"valid":  valid,
				"issues": issues,
			},
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
	if jar := os.Getenv("GOBIBLE_JAR"); jar != "" {
		if _, err := os.Stat(jar); err == nil {
			return jar
		}
	}
	return ""
}
