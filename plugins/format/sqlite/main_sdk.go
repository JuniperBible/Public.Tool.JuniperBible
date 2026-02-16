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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	format.Run(&format.Config{
		Name:       "SQLite",
		Extensions: []string{".db", ".sqlite", ".sqlite3"},
		Detect:     detect,
		Parse:      parse,
		Emit:       emit,
		Enumerate:  enumerate,
	})
}

func detect(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot stat: %v", err),
		}, nil
	}

	if info.IsDir() {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "path is a directory",
		}, nil
	}

	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not a SQLite file extension",
		}, nil
	}

	// Try to open as SQLite
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   fmt.Sprintf("cannot open as SQLite: %v", err),
		}, nil
	}
	defer db.Close()

	// Check for our schema (verses table with book, chapter, verse, text columns)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='verses'").Scan(&count)
	if err != nil || count == 0 {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "no 'verses' table found",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "SQLite",
		Reason:   "Capsule SQLite Bible format detected",
	}, nil
}

func parse(path string) (*ir.Corpus, error) {
	// Compute source hash
	sourceData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source: %w", err)
	}
	sourceHash := sha256.Sum256(sourceData)

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
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
		return nil, fmt.Errorf("failed to query verses: %w", err)
	}
	defer rows.Close()

	bookDocs := make(map[string]*ir.Document)
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
			doc = &ir.Document{
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

		cb := &ir.ContentBlock{
			ID:       fmt.Sprintf("cb-%d", sequence),
			Sequence: sequence,
			Text:     text,
			Hash:     hex.EncodeToString(hash[:]),
			Anchors: []*ir.Anchor{
				{
					ID:       fmt.Sprintf("a-%d-0", sequence),
					Position: 0,
					Spans: []*ir.Span{
						{
							ID:            fmt.Sprintf("s-%s", osisID),
							Type:          "VERSE",
							StartAnchorID: fmt.Sprintf("a-%d-0", sequence),
							Ref: &ir.Ref{
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

	return corpus, nil
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".db")

	// Create new SQLite database
	db, err := sqlite.Open(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
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
		return "", fmt.Errorf("failed to create schema: %w", err)
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

	return outputPath, nil
}

func enumerate(path string) (*ipc.EnumerateResult, error) {
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
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

	return &ipc.EnumerateResult{
		Entries: entries,
	}, nil
}
