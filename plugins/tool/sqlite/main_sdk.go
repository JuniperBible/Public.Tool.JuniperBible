//go:build sdk

// Plugin tool-sqlite provides SQLite database operations.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/juniper/plugins/ipc"
)

// TranscriptEvent extends the base event with sqlite-specific fields.
type TranscriptEvent struct {
	ipc.TranscriptEvent
	Database string `json:"database,omitempty"`
	Query    string `json:"query,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tool-sqlite <command> [args]")
		os.Exit(1)
	}

	config := &ipc.ToolConfig{
		PluginName: "tool-sqlite",
		Info: ipc.ToolInfo{
			Name:        "tool-sqlite",
			Version:     "1.0.0",
			Type:        "tool",
			Description: "SQLite database operations",
			Profiles: []ipc.ProfileInfo{
				{ID: "query", Description: "Execute SQL query and return results"},
				{ID: "export-csv", Description: "Export query results to CSV"},
				{ID: "schema", Description: "Show database schema"},
				{ID: "tables", Description: "List all tables"},
			},
			Requires: []string{"sqlite3"},
		},
		Profiles: map[string]ipc.ProfileHandler{
			"query":      profileQuery,
			"export-csv": profileExportCSV,
			"schema":     profileSchema,
			"tables":     profileTables,
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

func profileQuery(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	sql := req.Args["sql"]
	if db == "" || sql == "" {
		return fmt.Errorf("database and sql required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "query_start"},
		Database:        db,
		Query:           sql,
	})

	cmd := exec.Command("sqlite3", "-header", "-json", db, sql)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("sqlite3 failed: %w", err)
	}

	resultFile := filepath.Join(req.OutDir, "result.json")
	os.WriteFile(resultFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "query_end",
			Data:  map[string]interface{}{"result_file": resultFile, "rows": strings.Count(string(output), "\n")},
		},
	})

	return nil
}

func profileExportCSV(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	sql := req.Args["sql"]
	if db == "" || sql == "" {
		return fmt.Errorf("database and sql required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "export_start"},
		Database:        db,
	})

	cmd := exec.Command("sqlite3", "-header", "-csv", db, sql)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("sqlite3 failed: %w", err)
	}

	csvFile := filepath.Join(req.OutDir, "export.csv")
	os.WriteFile(csvFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "export_end",
			Data:  map[string]interface{}{"csv_file": csvFile},
		},
	})

	return nil
}

func profileSchema(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	if db == "" {
		return fmt.Errorf("database required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "schema_start"},
		Database:        db,
	})

	cmd := exec.Command("sqlite3", db, ".schema")
	output, _ := cmd.Output()

	schemaFile := filepath.Join(req.OutDir, "schema.sql")
	os.WriteFile(schemaFile, output, 0600)

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "schema_end",
			Data:  map[string]interface{}{"schema_file": schemaFile},
		},
	})

	return nil
}

func profileTables(req *ipc.ToolRunRequest, transcript *ipc.Transcript) error {
	db := req.Args["database"]
	if db == "" {
		return fmt.Errorf("database required")
	}

	transcript.WriteEvent(TranscriptEvent{
		TranscriptEvent: ipc.TranscriptEvent{Event: "tables_start"},
		Database:        db,
	})

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
		TranscriptEvent: ipc.TranscriptEvent{
			Event: "tables_end",
			Data:  map[string]interface{}{"tables": tables, "count": len(tables)},
		},
	})

	return nil
}
