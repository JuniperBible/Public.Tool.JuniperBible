package mybible

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func emitCreateSchema(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("failed to create verses table: %w", err)
	}
	db.Exec("CREATE INDEX book_number_index ON verses (book_number)")
	db.Exec("CREATE INDEX chapter_index ON verses (chapter)")
	db.Exec("CREATE INDEX verse_index ON verses (verse)")

	if _, err := db.Exec("CREATE TABLE info (name TEXT NOT NULL, value TEXT NOT NULL)"); err != nil {
		return fmt.Errorf("failed to create info table: %w", err)
	}
	return nil
}

func emitInsertMetadata(db *sql.DB, corpus *ir.Corpus) {
	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}
	db.Exec("INSERT INTO info (name, value) VALUES ('description', ?)", title)
	if corpus.Description != "" {
		db.Exec("INSERT INTO info (name, value) VALUES ('detailed_info', ?)", corpus.Description)
	}
	if corpus.Language != "" {
		db.Exec("INSERT INTO info (name, value) VALUES ('language', ?)", corpus.Language)
	}
	if v, ok := corpus.Attributes["version"]; ok {
		db.Exec("INSERT INTO info (name, value) VALUES ('version', ?)", v)
	}
	for k, v := range corpus.Attributes {
		if k != "version" {
			db.Exec("INSERT INTO info (name, value) VALUES (?, ?)", k, v)
		}
	}
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".SQLite3")

	db, err := sqlite.Open(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	if err := emitCreateSchema(db); err != nil {
		return "", err
	}

	if err := emitBibleNative(db, corpus); err != nil {
		return "", fmt.Errorf("failed to emit content: %w", err)
	}

	emitInsertMetadata(db, corpus)

	return outputPath, nil
}
