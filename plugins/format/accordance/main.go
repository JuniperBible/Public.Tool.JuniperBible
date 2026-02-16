//go:build !sdk

// Plugin format-accordance handles Accordance Mac Bible format.
// Accordance uses a proprietary SQLite-based format with custom structure.
//
// IR Support:
// - extract-ir: Reads Accordance format to IR (L2)
// - emit-native: Converts IR to Accordance-compatible format (L2)
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	req, err := ipc.ReadRequest()
	if err != nil {
		ipc.RespondErrorf("failed to decode request: %v", err)
		return
	}

	switch req.Command {
	case "detect":
		handleDetect(req.Args)
	case "ingest":
		handleIngest(req.Args)
	case "enumerate":
		handleEnumerate(req.Args)
	case "extract-ir":
		handleExtractIR(req.Args)
	case "emit-native":
		handleEmitNative(req.Args)
	default:
		ipc.RespondErrorf("unknown command: %s", req.Command)
	}
}

func handleDetect(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	// Accordance uses .amod extension
	if ext == ".amod" || ext == ".accordance" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "Accordance",
			Reason:   "Accordance file extension detected",
		})
		return
	}

	// Check if it's SQLite with Accordance structure
	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not an Accordance file",
		})
		return
	}
	defer db.Close()

	// Check for Accordance-specific tables
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'Acc%' LIMIT 1").Scan(&tableName)
	if err == nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: true,
			Format:   "Accordance",
			Reason:   "Accordance database structure detected",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: false,
		Reason:   "no Accordance structure found",
	})
}

func handleIngest(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	blobDir := filepath.Join(outputDir, hashHex[:2])
	if err := os.MkdirAll(blobDir, 0755); err != nil {
		ipc.RespondErrorf("failed to create blob dir: %v", err)
		return
	}

	blobPath := filepath.Join(blobDir, hashHex)
	if err := os.WriteFile(blobPath, data, 0644); err != nil {
		ipc.RespondErrorf("failed to write blob: %v", err)
		return
	}

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	ipc.MustRespond(&ipc.IngestResult{
		ArtifactID: artifactID,
		BlobSHA256: hashHex,
		SizeBytes:  int64(len(data)),
		Metadata: map[string]string{
			"format": "Accordance",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		ipc.RespondErrorf("failed to stat: %v", err)
		return
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "Accordance"},
			},
		},
	})
}

func handleExtractIR(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read file: %v", err)
		return
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "Accordance",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L2",
		Attributes:   make(map[string]string),
	}

	// Store raw for round-trip
	corpus.Attributes["_accordance_raw"] = hex.EncodeToString(data)

	// Try to extract content from SQLite database
	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err == nil {
		defer db.Close()
		corpus.Documents = extractAccordanceContent(db, artifactID)
	}

	// If no documents extracted, create minimal structure
	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ipc.Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		ipc.RespondErrorf("failed to serialize IR: %v", err)
		return
	}

	irPath := filepath.Join(outputDir, corpus.ID+".ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		ipc.RespondErrorf("failed to write IR: %v", err)
		return
	}
	ipc.MustRespond(&ipc.ExtractIRResult{
		IRPath:    irPath,
		LossClass: "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "Accordance",
			TargetFormat: "IR",
			LossClass:    "L2",
			Warnings:     []string{"Accordance format has proprietary structure - limited extraction"},
		},
	})
}

func extractAccordanceContent(db *sql.DB, artifactID string) []*ipc.Document {
	doc := &ipc.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	// Try common table names
	tables := []string{"verses", "content", "text", "AccVerses", "AccContent"}
	for _, table := range tables {
		rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 100", table))
		if err != nil {
			continue
		}
		defer rows.Close()

		cols, _ := rows.Columns()
		if len(cols) > 0 {
			sequence := 0
			for rows.Next() {
				values := make([]interface{}, len(cols))
				valuePtrs := make([]interface{}, len(cols))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					continue
				}

				for _, v := range values {
					if text, ok := v.(string); ok && len(text) > 10 {
						sequence++
						hash := sha256.Sum256([]byte(text))

						cb := &ipc.ContentBlock{
							ID:       fmt.Sprintf("cb-%d", sequence),
							Sequence: sequence,
							Text:     text,
							Hash:     hex.EncodeToString(hash[:]),
						}
						doc.ContentBlocks = append(doc.ContentBlocks, cb)
						break
					}
				}
			}
		}
	}

	return []*ipc.Document{doc}
}

func handleEmitNative(args map[string]interface{}) {
	irPath, ok := args["ir_path"].(string)
	if !ok {
		ipc.RespondError("ir_path argument required")
		return
	}

	outputDir, ok := args["output_dir"].(string)
	if !ok {
		ipc.RespondError("output_dir argument required")
		return
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		ipc.RespondErrorf("failed to read IR file: %v", err)
		return
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		ipc.RespondErrorf("failed to parse IR: %v", err)
		return
	}

	outputPath := filepath.Join(outputDir, corpus.ID+".amod")

	// Check for raw Accordance for round-trip
	if raw, ok := corpus.Attributes["_accordance_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0644); err != nil {
				ipc.RespondErrorf("failed to write Accordance: %v", err)
				return
			}
			ipc.MustRespond(&ipc.EmitNativeResult{
				OutputPath: outputPath,
				Format:     "Accordance",
				LossClass:  "L0",
				LossReport: &ipc.LossReport{
					SourceFormat: "IR",
					TargetFormat: "Accordance",
					LossClass:    "L0",
				},
			})
			return
		}
	}

	// Generate Accordance-compatible SQLite from IR
	db, err := sql.Open(sqliteDriver, outputPath)
	if err != nil {
		ipc.RespondErrorf("failed to create database: %v", err)
		return
	}
	defer db.Close()

	// Create Accordance-style tables
	_, err = db.Exec(`
		CREATE TABLE AccMetadata (
			key TEXT PRIMARY KEY,
			value TEXT
		);
		CREATE TABLE AccVerses (
			id INTEGER PRIMARY KEY,
			book TEXT,
			chapter INTEGER,
			verse INTEGER,
			text TEXT
		);
	`)
	if err != nil {
		ipc.RespondErrorf("failed to create tables: %v", err)
		return
	}

	// Insert metadata
	db.Exec("INSERT INTO AccMetadata VALUES ('title', ?)", corpus.Title)
	db.Exec("INSERT INTO AccMetadata VALUES ('language', ?)", corpus.Language)

	// Insert content
	for _, doc := range corpus.Documents {
		for _, cb := range doc.ContentBlocks {
			chapter := 1
			verse := cb.Sequence
			if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
				if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
					chapter = ref.Chapter
					verse = ref.Verse
				}
			}
			db.Exec("INSERT INTO AccVerses (book, chapter, verse, text) VALUES (?, ?, ?, ?)",
				doc.ID, chapter, verse, cb.Text)
		}
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "Accordance",
		LossClass:  "L2",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "Accordance",
			LossClass:    "L2",
			Warnings:     []string{"Generated Accordance-compatible format - not native Accordance"},
		},
	})
}
