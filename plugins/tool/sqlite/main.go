//go:build !sdk

// Plugin tool-sqlite provides SQLite database operations.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	Database  string      `json:"database,omitempty"`
	Query     string      `json:"query,omitempty"`
	Error     string      `json:"error,omitempty"`
	ExitCode  int         `json:"exit_code,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-sqlite <command> [args]")
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
		"name":        "tool-sqlite",
		"version":     "1.0.0",
		"type":        "tool",
		"description": "SQLite database operations",
		"profiles": []map[string]string{
			{"id": "query", "description": "Execute SQL query and return results"},
			{"id": "export-csv", "description": "Export query results to CSV"},
			{"id": "schema", "description": "Show database schema"},
			{"id": "tables", "description": "List all tables"},
		},
		"requires": []string{"sqlite3"},
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
	os.MkdirAll(outDir, 0700)
	executeProfile(&req)
}

func executeProfile(req *ToolRunRequest) {
	transcript := ipc.NewTranscript(req.OutDir)
	defer transcript.Close()

	transcript.WriteEvent(TranscriptEvent{
		Event:     "start",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Plugin:    "tool-sqlite",
		Profile:   req.Profile,
	})

	var err error
	switch req.Profile {
	case "query":
		err = profileQuery(req, transcript)
	case "export-csv":
		err = profileExportCSV(req, transcript)
	case "schema":
		err = profileSchema(req, transcript)
	case "tables":
		err = profileTables(req, transcript)
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

func profileQuery(req *ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	sql := req.Args["sql"]
	if db == "" || sql == "" {
		return fmt.Errorf("database and sql required")
	}

	transcript.WriteEvent(TranscriptEvent{
		Event:    "query_start",
		Database: db,
		Query:    sql,
	})

	cmd := exec.Command("sqlite3", "-header", "-json", db, sql)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("sqlite3 failed: %w", err)
	}

	resultFile := filepath.Join(req.OutDir, "result.json")
	os.WriteFile(resultFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		Event: "query_end",
		Data:  map[string]interface{}{"result_file": resultFile, "rows": strings.Count(string(output), "\n")},
	})

	return nil
}

func profileExportCSV(req *ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	sql := req.Args["sql"]
	if db == "" || sql == "" {
		return fmt.Errorf("database and sql required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "export_start", Database: db})

	cmd := exec.Command("sqlite3", "-header", "-csv", db, sql)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("sqlite3 failed: %w", err)
	}

	csvFile := filepath.Join(req.OutDir, "export.csv")
	os.WriteFile(csvFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		Event: "export_end",
		Data:  map[string]interface{}{"csv_file": csvFile},
	})

	return nil
}

func profileSchema(req *ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	if db == "" {
		return fmt.Errorf("database required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "schema_start", Database: db})

	cmd := exec.Command("sqlite3", db, ".schema")
	output, _ := cmd.Output()

	schemaFile := filepath.Join(req.OutDir, "schema.sql")
	os.WriteFile(schemaFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		Event: "schema_end",
		Data:  map[string]interface{}{"schema_file": schemaFile},
	})

	return nil
}

func profileTables(req *ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	if db == "" {
		return fmt.Errorf("database required")
	}

	transcript.WriteEvent(TranscriptEvent{Event: "tables_start", Database: db})

	cmd := exec.Command("sqlite3", db, ".tables")
	output, _ := cmd.Output()

	tables := strings.Fields(string(output))

	tablesFile := filepath.Join(req.OutDir, "tables.json")
	data, _ := json.MarshalIndent(map[string]interface{}{
		"tables": tables,
		"count":  len(tables),
	}, "", "  ")
	os.WriteFile(tablesFile, data, 0600)

	transcript.WriteEvent(TranscriptEvent{
		Event: "tables_end",
		Data:  map[string]interface{}{"tables": tables, "count": len(tables)},
	})

	return nil
}
