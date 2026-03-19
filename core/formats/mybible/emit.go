package mybible

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/sqlite"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
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
	db.Exec("CREATE INDEX idx_verses_bcv ON verses (book_number, chapter, verse)")

	if _, err := db.Exec("CREATE TABLE info (name TEXT NOT NULL, value TEXT NOT NULL)"); err != nil {
		return fmt.Errorf("failed to create info table: %w", err)
	}
	return nil
}

func emitInsertMetadataTx(tx *sql.Tx, corpus *ir.Corpus) error {
	title := corpus.Title
	if title == "" {
		title = corpus.ID
	}
	if _, err := tx.Exec("INSERT INTO info (name, value) VALUES ('description', ?)", title); err != nil {
		return fmt.Errorf("insert metadata description: %w", err)
	}
	if corpus.Description != "" {
		if _, err := tx.Exec("INSERT INTO info (name, value) VALUES ('detailed_info', ?)", corpus.Description); err != nil {
			return fmt.Errorf("insert metadata detailed_info: %w", err)
		}
	}
	if corpus.Language != "" {
		if _, err := tx.Exec("INSERT INTO info (name, value) VALUES ('language', ?)", corpus.Language); err != nil {
			return fmt.Errorf("insert metadata language: %w", err)
		}
	}
	if v, ok := corpus.Attributes["version"]; ok {
		if _, err := tx.Exec("INSERT INTO info (name, value) VALUES ('version', ?)", v); err != nil {
			return fmt.Errorf("insert metadata version: %w", err)
		}
	}
	for k, v := range corpus.Attributes {
		if k != "version" {
			if _, err := tx.Exec("INSERT INTO info (name, value) VALUES (?, ?)", k, v); err != nil {
				return fmt.Errorf("insert metadata %s: %w", k, err)
			}
		}
	}
	return nil
}

func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, corpus.ID+".SQLite3")

	db, err := sqlite.Open(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create database: %w", err)
	}
	defer db.Close()

	if err := sqlite.ConfigureForBulkWrite(db); err != nil {
		return "", err
	}

	if err := emitCreateSchema(db); err != nil {
		return "", err
	}

	if err := sqlite.WithTransaction(db, func(tx *sql.Tx) error {
		if err := emitBibleNativeTx(tx, corpus); err != nil {
			return err
		}
		return emitInsertMetadataTx(tx, corpus)
	}); err != nil {
		return "", fmt.Errorf("failed to emit content: %w", err)
	}

	if err := sqlite.Optimize(db); err != nil {
		return "", err
	}

	return outputPath, nil
}
