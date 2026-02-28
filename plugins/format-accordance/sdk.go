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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/sqlite"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

func runSDK() {
	if err := format.Run(&format.Config{
		Name:       "Accordance",
		Extensions: []string{".amod", ".accordance"},
		Detect:     detectAccordance,
		Parse:      parseAccordance,
		Emit:       emitAccordance,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func detectAccordance(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".amod" || ext == ".accordance" {
		return &ipc.DetectResult{Detected: true, Format: "Accordance", Reason: "Accordance file extension detected"}, nil
	}

	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: "not an Accordance file"}, nil
	}
	defer db.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'Acc%' LIMIT 1").Scan(&tableName)
	if err == nil {
		return &ipc.DetectResult{Detected: true, Format: "Accordance", Reason: "Accordance database structure detected"}, nil
	}

	return &ipc.DetectResult{Detected: false, Reason: "no Accordance structure found"}, nil
}

func parseAccordance(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := ir.NewCorpus(artifactID, "BIBLE", "")
	corpus.SourceFormat = "Accordance"
	corpus.SourceHash = hex.EncodeToString(sourceHash[:])
	corpus.LossClass = "L2"
	corpus.Attributes = map[string]string{"_accordance_raw": hex.EncodeToString(data)}

	db, err := sqlite.OpenReadOnly(path)
	if err == nil {
		defer db.Close()
		corpus.Documents = extractAccordanceContent(db, artifactID)
	}

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{ir.NewDocument(artifactID, artifactID, 1)}
	}

	return corpus, nil
}

func extractAccordanceContent(db *sql.DB, artifactID string) []*ir.Document {
	doc := ir.NewDocument(artifactID, artifactID, 1)

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

						cb := &ir.ContentBlock{
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

	return []*ir.Document{doc}
}

func emitAccordance(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".amod")

	// Check for raw Accordance for round-trip
	if raw, ok := corpus.Attributes["_accordance_raw"]; ok && raw != "" {
		rawData, err := hex.DecodeString(raw)
		if err == nil {
			if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
				return "", fmt.Errorf("failed to write Accordance: %w", err)
			}
			return outputPath, nil
		}
	}

	// Generate Accordance-compatible SQLite from IR
	db, err := sqlite.Open(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE AccMetadata (key TEXT PRIMARY KEY, value TEXT);
		CREATE TABLE AccVerses (id INTEGER PRIMARY KEY, book TEXT, chapter INTEGER, verse INTEGER, text TEXT);
	`)
	if err != nil {
		return "", fmt.Errorf("failed to create tables: %w", err)
	}

	db.Exec("INSERT INTO AccMetadata VALUES ('title', ?)", corpus.Title)
	db.Exec("INSERT INTO AccMetadata VALUES ('language', ?)", corpus.Language)

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

	return outputPath, nil
}
