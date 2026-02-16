//go:build !sdk

// Plugin format-sqlite handles SQLite Bible database format.
// Provides queryable database structure for programmatic access.
//
// IR Support:
// - extract-ir: Reads SQLite Bible database to IR (L1)
// - emit-native: Converts IR to SQLite database (L1)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
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

	info, err := os.Stat(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		})
		return
	}

	if info.IsDir() {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		})
		return
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "not a SQLite file extension",
		})
		return
	}

	// Try to open as SQLite
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as SQLite: %v", err),
		})
		return
	}
	defer db.Close()

	// Check for our schema (verses table with book, chapter, verse, text columns)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='verses'").Scan(&count)
	if err != nil || count == 0 {
		ipc.MustRespond(&ipc.DetectResult{
			Detected: false,
			Reason:   "no 'verses' table found",
		})
		return
	}
	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "SQLite",
		Reason:   "Capsule SQLite Bible format detected",
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
			"format": "SQLite",
		},
	})
}

func handleEnumerate(args map[string]interface{}) {
	path, ok := args["path"].(string)
	if !ok {
		ipc.RespondError("path argument required")
		return
	}

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.RespondErrorf("failed to open database: %v", err)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		ipc.RespondErrorf("failed to query tables: %v", err)
		return
	}
	defer rows.Close()

	var entries []ipc.EnumerateEntry
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		var count int64
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName)).Scan(&count)

		entries = append(entries, ipc.EnumerateEntry{
			Path:      tableName,
			SizeBytes: count,
			IsDir:     false,
			Metadata: map[string]string{
				"type":      "table",
				"row_count": fmt.Sprintf("%d", count),
			},
		})
	}
	ipc.MustRespond(&ipc.EnumerateResult{
		Entries: entries,
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

	// Compute source hash
	sourceData, err := os.ReadFile(path)
	if err != nil {
		ipc.RespondErrorf("failed to read source: %v", err)
		return
	}
	sourceHash := sha256.Sum256(sourceData)

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		ipc.RespondErrorf("failed to open database: %v", err)
		return
	}
	defer db.Close()

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ipc.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "SQLite",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L1",
		Attributes:   make(map[string]string),
	}

	// Try to get metadata
	var title, language, description string
	row := db.QueryRow("SELECT title, language, description FROM meta LIMIT 1")
	if err := row.Scan(&title, &language, &description); err == nil {
		corpus.Title = title
		corpus.Language = language
		corpus.Description = description
	}

	// Read verses
	rows, err := db.Query("SELECT book, chapter, verse, text FROM verses ORDER BY book, chapter, verse")
	if err != nil {
		ipc.RespondErrorf("failed to query verses: %v", err)
		return
	}
	defer rows.Close()

	bookDocs := make(map[string]*ipc.Document)
	sequence := 0

	for rows.Next() {
		var book string
		var chapter, verse int
		var text string
		if err := rows.Scan(&book, &chapter, &verse, &text); err != nil {
			continue
		}

		doc, ok := bookDocs[book]
		if !ok {
			doc = &ipc.Document{
				ID:    book,
				Title: book,
				Order: len(bookDocs) + 1,
			}
			bookDocs[book] = doc
			corpus.Documents = append(corpus.Documents, doc)
		}

		sequence++
		hash := sha256.Sum256([]byte(text))
		osisID := fmt.Sprintf("%s.%d.%d", book, chapter, verse)

		cb := &ipc.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ipc.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ipc.Span{
						{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ipc.Ref{
								Book:    book,
								Chapter: chapter,
								Verse:   verse,
								OSISID:  osisID,
							},
						},
					},
				},
			},
		}
		doc.ContentBlocks = append(doc.ContentBlocks, cb)
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
		LossClass: "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "SQLite",
			TargetFormat: "IR",
			LossClass:    "L1",
		},
	})
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

	outputPath := filepath.Join(outputDir, corpus.ID+".db")

	// Create new SQLite database
	db, err := sqlite.Open(outputPath)
	if err != nil {
		ipc.RespondErrorf("failed to create database: %v", err)
		return
	}
	defer db.Close()

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS meta (
			id TEXT PRIMARY KEY,
			title TEXT,
			language TEXT,
			description TEXT,
			version TEXT
		);
		CREATE TABLE IF NOT EXISTS books (
			id TEXT PRIMARY KEY,
			name TEXT,
			book_order INTEGER
		);
		CREATE TABLE IF NOT EXISTS verses (
			id TEXT PRIMARY KEY,
			book TEXT,
			chapter INTEGER,
			verse INTEGER,
			text TEXT,
			FOREIGN KEY (book) REFERENCES books(id)
		);
		CREATE INDEX IF NOT EXISTS idx_verses_ref ON verses(book, chapter, verse);
	`)
	if err != nil {
		ipc.RespondErrorf("failed to create schema: %v", err)
		return
	}

	// Insert metadata
	db.Exec("INSERT INTO meta (id, title, language, description, version) VALUES (?, ?, ?, ?, ?)",
		corpus.ID, corpus.Title, corpus.Language, corpus.Description, corpus.Version)

	// Insert books and verses
	for _, doc := range corpus.Documents {
		db.Exec("INSERT INTO books (id, name, book_order) VALUES (?, ?, ?)",
			doc.ID, doc.Title, doc.Order)

		for _, cb := range doc.ContentBlocks {
			for _, anchor := range cb.Anchors {
				for _, span := range anchor.Spans {
					if span.Ref != nil && span.Type == "VERSE" {
						db.Exec("INSERT INTO verses (id, book, chapter, verse, text) VALUES (?, ?, ?, ?, ?)",
							span.Ref.OSISID, doc.ID, span.Ref.Chapter, span.Ref.Verse, cb.Text)
					}
				}
			}
		}
	}
	ipc.MustRespond(&ipc.EmitNativeResult{
		OutputPath: outputPath,
		Format:     "SQLite",
		LossClass:  "L1",
		LossReport: &ipc.LossReport{
			SourceFormat: "IR",
			TargetFormat: "SQLite",
			LossClass:    "L1",
		},
	})
}
