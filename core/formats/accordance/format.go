// Package accordance provides canonical Accordance Mac Bible format support.
// Accordance uses a proprietary SQLite-based format with custom structure.
//
// IR Support:
// - extract-ir: Reads Accordance format to IR (L2)
// - emit-native: Converts IR to Accordance-compatible format (L2)
package accordance

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// Config defines the Accordance format plugin.
var Config = &format.Config{
	PluginID:   "format.accordance",
	Name:       "Accordance",
	Extensions: []string{".amod", ".accordance"},
	Detect:     detectAccordance,
	Parse:      parseAccordance,
	Emit:       emitAccordance,
	Enumerate:  enumerateAccordance,
}

// sqliteDriver is set at build time via build tags
const sqliteDriver = "sqlite"

func detectAccordance(path string) (*ipc.DetectResult, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".amod" || ext == ".accordance" {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "Accordance",
			Reason:   "Accordance file extension detected",
		}, nil
	}

	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err != nil {
		return &ipc.DetectResult{
			Detected: false,
			Reason:   "not an Accordance file",
		}, nil
	}
	defer db.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'Acc%' LIMIT 1").Scan(&tableName)
	if err == nil {
		return &ipc.DetectResult{
			Detected: true,
			Format:   "Accordance",
			Reason:   "Accordance database structure detected",
		}, nil
	}

	return &ipc.DetectResult{
		Detected: false,
		Reason:   "no Accordance structure found",
	}, nil
}

func parseAccordance(path string) (*ir.Corpus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	sourceHash := sha256.Sum256(data)
	artifactID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	corpus := &ir.Corpus{
		ID:           artifactID,
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		SourceFormat: "Accordance",
		SourceHash:   hex.EncodeToString(sourceHash[:]),
		LossClass:    "L2",
		Attributes:   make(map[string]string),
	}

	corpus.Attributes["_accordance_raw"] = hex.EncodeToString(data)

	db, err := sql.Open(sqliteDriver, path+"?mode=ro")
	if err == nil {
		defer db.Close()
		corpus.Documents = extractAccordanceContent(db, artifactID)
	}

	if len(corpus.Documents) == 0 {
		corpus.Documents = []*ir.Document{
			{
				ID:    artifactID,
				Title: artifactID,
				Order: 1,
			},
		}
	}

	return corpus, nil
}

func extractAccordanceContent(db *sql.DB, artifactID string) []*ir.Document {
	doc := &ir.Document{
		ID:    artifactID,
		Title: artifactID,
		Order: 1,
	}

	tables := []string{"verses", "content", "text", "AccVerses", "AccContent"}
	sequence := 0
	for _, table := range tables {
		sequence = extractFromTable(db, doc, table, sequence)
	}

	return []*ir.Document{doc}
}

func extractFromTable(db *sql.DB, doc *ir.Document, table string, sequence int) int {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 100", table))
	if err != nil {
		return sequence
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	if len(cols) == 0 {
		return sequence
	}

	for rows.Next() {
		if text := extractTextFromRow(rows, cols); text != "" {
			sequence++
			doc.ContentBlocks = append(doc.ContentBlocks, createContentBlock(sequence, text))
		}
	}
	return sequence
}

func extractTextFromRow(rows *sql.Rows, cols []string) string {
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return ""
	}

	for _, v := range values {
		if text, ok := v.(string); ok && len(text) > 10 {
			return text
		}
	}
	return ""
}

func createContentBlock(sequence int, text string) *ir.ContentBlock {
	hash := sha256.Sum256([]byte(text))
	return &ir.ContentBlock{
		ID:       fmt.Sprintf("cb-%d", sequence),
		Sequence: sequence,
		Text:     text,
		Hash:     hex.EncodeToString(hash[:]),
	}
}

func writeRawAccordance(corpus *ir.Corpus, outputPath string) (bool, error) {
	raw, ok := corpus.Attributes["_accordance_raw"]
	if !ok || raw == "" {
		return false, nil
	}
	rawData, err := hex.DecodeString(raw)
	if err != nil {
		return false, nil
	}
	if err := os.WriteFile(outputPath, rawData, 0600); err != nil {
		return false, fmt.Errorf("failed to write Accordance: %w", err)
	}
	return true, nil
}

func createAccordanceTables(db *sql.DB) error {
	_, err := db.Exec(`
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
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

func resolveChapterVerse(cb *ir.ContentBlock) (int, int) {
	if len(cb.Anchors) > 0 && len(cb.Anchors[0].Spans) > 0 {
		if ref := cb.Anchors[0].Spans[0].Ref; ref != nil {
			return ref.Chapter, ref.Verse
		}
	}
	return 1, cb.Sequence
}

func insertContentBlocks(db *sql.DB, docs []*ir.Document) {
	for _, doc := range docs {
		for _, cb := range doc.ContentBlocks {
			chapter, verse := resolveChapterVerse(cb)
			db.Exec("INSERT INTO AccVerses (book, chapter, verse, text) VALUES (?, ?, ?, ?)",
				doc.ID, chapter, verse, cb.Text)
		}
	}
}

func emitAccordance(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".amod")

	if written, err := writeRawAccordance(corpus, outputPath); written || err != nil {
		return outputPath, err
	}

	db, err := sql.Open(sqliteDriver, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	if err := createAccordanceTables(db); err != nil {
		return "", err
	}

	db.Exec("INSERT INTO AccMetadata VALUES ('title', ?)", corpus.Title)
	db.Exec("INSERT INTO AccMetadata VALUES ('language', ?)", corpus.Language)

	insertContentBlocks(db, corpus.Documents)

	return outputPath, nil
}

func enumerateAccordance(path string) (*ipc.EnumerateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{
			{
				Path:      filepath.Base(path),
				SizeBytes: info.Size(),
				IsDir:     false,
				Metadata:  map[string]string{"format": "Accordance"},
			},
		},
	}, nil
}
