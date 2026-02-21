//go:build !sdk

// Package main implements a Hugo JSON output generator plugin.
// Generates Hugo-compatible JSON data files from Bible modules.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
)

// PluginInfo contains plugin metadata.
type PluginInfo struct {
	PluginID    string `json:"plugin_id"`
	Version     string `json:"version"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "info":
			printInfo()
			return
		case "ipc":
			runIPC()
			return
		}
	}

	// Default: run IPC mode (read from stdin)
	runIPC()
}

func printInfo() {
	info := PluginInfo{
		PluginID:    "tool.hugo",
		Version:     "0.1.0",
		Kind:        "tool",
		Description: "Hugo JSON output generator for Bible modules",
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(info)
}

func runIPC() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req ipc.Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(fmt.Sprintf("invalid JSON: %v", err))
			continue
		}

		handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		sendError(fmt.Sprintf("stdin read error: %v", err))
	}
}

func handleRequest(req *ipc.Request) {
	switch req.Command {
	case "generate":
		handleGenerate(req)
	default:
		sendError(fmt.Sprintf("unknown command: %s", req.Command))
	}
}

func handleGenerate(req *ipc.Request) {
	inputPath, _ := req.Args["input"].(string)
	outputPath, _ := req.Args["output"].(string)
	format, _ := req.Args["format"].(string)

	if inputPath == "" {
		sendError("missing required argument: input")
		return
	}
	if outputPath == "" {
		sendError("missing required argument: output")
		return
	}

	opts := &GenerateOptions{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Format:     format,
	}

	result, err := Generate(opts)
	if err != nil {
		sendError(fmt.Sprintf("generation failed: %v", err))
		return
	}

	sendResult(result)
}

func sendResult(result interface{}) {
	resp := ipc.Response{
		Status: "ok",
		Result: result,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}

func sendError(msg string) {
	resp := ipc.Response{
		Status: "error",
		Error:  msg,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
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
