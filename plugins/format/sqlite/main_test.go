//go:build !sdk

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// createTestDB creates a minimal Capsule SQLite Bible database for testing.
func createTestDB(t *testing.T, path string) {
	t.Helper()

	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE meta (id TEXT, title TEXT, language TEXT, description TEXT, version TEXT);
		CREATE TABLE books (id TEXT, name TEXT, book_order INTEGER);
		CREATE TABLE verses (id TEXT, book TEXT, chapter INTEGER, verse INTEGER, text TEXT);
		INSERT INTO meta VALUES ('test', 'Test Bible', 'en', 'A test Bible', '1.0.0');
		INSERT INTO books VALUES ('Gen', 'Genesis', 1);
		INSERT INTO verses VALUES ('Gen.1.1', 'Gen', 1, 1, 'In the beginning God created the heaven and the earth.');
		INSERT INTO verses VALUES ('Gen.1.2', 'Gen', 1, 2, 'And the earth was without form, and void.');
		INSERT INTO verses VALUES ('Gen.1.3', 'Gen', 1, 3, 'And God said, Let there be light: and there was light.');
	`)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
}

// createInvalidSQLiteDB creates a SQLite file without the verses table
func createInvalidSQLiteDB(t *testing.T, path string) {
	t.Helper()

	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, data TEXT);`)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
}

// TestHandleDetect tests the detect handler directly
func TestHandleDetect(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		args         map[string]interface{}
		wantDetected bool
		wantFormat   string
		wantReason   string
	}{
		{
			name: "valid SQLite Bible database",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				createTestDB(t, dbPath)
				return dbPath
			},
			args:         nil, // will be set in test
			wantDetected: true,
			wantFormat:   "SQLite",
			wantReason:   "Capsule SQLite Bible format detected",
		},
		{
			name: "SQLite file without verses table",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "invalid.db")
				createInvalidSQLiteDB(t, dbPath)
				return dbPath
			},
			wantDetected: false,
			wantReason:   "no 'verses' table found",
		},
		{
			name: "non-SQLite file with .db extension",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				txtPath := filepath.Join(tmpDir, "test.db")
				os.WriteFile(txtPath, []byte("This is not a SQLite file"), 0644)
				return txtPath
			},
			wantDetected: false,
			wantReason:   "no 'verses' table found",
		},
		{
			name: "text file without SQLite extension",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				txtPath := filepath.Join(tmpDir, "test.txt")
				os.WriteFile(txtPath, []byte("Hello world"), 0644)
				return txtPath
			},
			wantDetected: false,
			wantReason:   "not a SQLite file extension",
		},
		{
			name: "directory instead of file",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dirPath := filepath.Join(tmpDir, "test.db")
				os.MkdirAll(dirPath, 0755)
				return dirPath
			},
			wantDetected: false,
			wantReason:   "path is a directory",
		},
		{
			name: "non-existent file",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/to/file.db"
			},
			wantDetected: false,
			wantReason:   "cannot stat",
		},
		{
			name: ".sqlite extension",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.sqlite")
				createTestDB(t, dbPath)
				return dbPath
			},
			wantDetected: true,
			wantFormat:   "SQLite",
		},
		{
			name: ".sqlite3 extension",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.sqlite3")
				createTestDB(t, dbPath)
				return dbPath
			},
			wantDetected: true,
			wantFormat:   "SQLite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			args := tt.args
			if args == nil {
				args = map[string]interface{}{"path": path}
			}

			handleDetect(args)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)

			var resp ipc.Response
			if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			result, ok := resp.Result.(*ipc.DetectResult)
			if !ok {
				// Try to convert from map
				if m, ok := resp.Result.(map[string]interface{}); ok {
					result = &ipc.DetectResult{
						Detected: m["detected"].(bool),
					}
					if format, ok := m["format"].(string); ok {
						result.Format = format
					}
					if reason, ok := m["reason"].(string); ok {
						result.Reason = reason
					}
				} else {
					t.Fatalf("result is not ipc.DetectResult: %T", resp.Result)
				}
			}

			if result.Detected != tt.wantDetected {
				t.Errorf("detected = %v, want %v (reason: %s)", result.Detected, tt.wantDetected, result.Reason)
			}
			if tt.wantFormat != "" && result.Format != tt.wantFormat {
				t.Errorf("format = %v, want %v", result.Format, tt.wantFormat)
			}
			if tt.wantReason != "" && !strings.Contains(result.Reason, tt.wantReason) {
				t.Errorf("reason = %q, want to contain %q", result.Reason, tt.wantReason)
			}
		})
	}
}

// TestHandleExtractIR tests the extract-ir handler directly
func TestHandleExtractIR(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) (dbPath, outputDir string)
		wantErr       bool
		wantLossClass string
		validateFunc  func(t *testing.T, corpus *ipc.Corpus)
	}{
		{
			name: "extract valid Bible database",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				createTestDB(t, dbPath)
				outputDir := filepath.Join(tmpDir, "output")
				os.MkdirAll(outputDir, 0755)
				return dbPath, outputDir
			},
			wantLossClass: "L1",
			validateFunc: func(t *testing.T, corpus *ipc.Corpus) {
				if corpus.Title != "Test Bible" {
					t.Errorf("title = %q, want %q", corpus.Title, "Test Bible")
				}
				if corpus.Language != "en" {
					t.Errorf("language = %q, want %q", corpus.Language, "en")
				}
				if corpus.Description != "A test Bible" {
					t.Errorf("description = %q, want %q", corpus.Description, "A test Bible")
				}
				if len(corpus.Documents) != 1 {
					t.Fatalf("documents count = %d, want 1", len(corpus.Documents))
				}
				doc := corpus.Documents[0]
				if doc.ID != "Gen" {
					t.Errorf("document ID = %q, want %q", doc.ID, "Gen")
				}
				if len(doc.ContentBlocks) != 3 {
					t.Errorf("content blocks = %d, want 3", len(doc.ContentBlocks))
				}
				// Verify first verse
				cb := doc.ContentBlocks[0]
				if cb.Text != "In the beginning God created the heaven and the earth." {
					t.Errorf("unexpected verse text: %q", cb.Text)
				}
				// Verify anchors and spans
				if len(cb.Anchors) != 1 {
					t.Errorf("anchors count = %d, want 1", len(cb.Anchors))
				}
				if len(cb.Anchors[0].Spans) != 1 {
					t.Errorf("spans count = %d, want 1", len(cb.Anchors[0].Spans))
				}
				span := cb.Anchors[0].Spans[0]
				if span.Type != "VERSE" {
					t.Errorf("span type = %q, want %q", span.Type, "VERSE")
				}
				if span.Ref.Book != "Gen" || span.Ref.Chapter != 1 || span.Ref.Verse != 1 {
					t.Errorf("ref = %v:%d:%d, want Gen:1:1", span.Ref.Book, span.Ref.Chapter, span.Ref.Verse)
				}
			},
		},
		{
			name: "database without metadata table",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "nometa.db")
				db, _ := sqlite.Open(dbPath)
				defer db.Close()
				db.Exec(`
					CREATE TABLE verses (id TEXT, book TEXT, chapter INTEGER, verse INTEGER, text TEXT);
					INSERT INTO verses VALUES ('Gen.1.1', 'Gen', 1, 1, 'In the beginning.');
				`)
				outputDir := filepath.Join(tmpDir, "output")
				os.MkdirAll(outputDir, 0755)
				return dbPath, outputDir
			},
			wantLossClass: "L1",
			validateFunc: func(t *testing.T, corpus *ipc.Corpus) {
				if corpus.Title != "" {
					t.Errorf("expected empty title, got %q", corpus.Title)
				}
				if len(corpus.Documents) != 1 {
					t.Errorf("documents count = %d, want 1", len(corpus.Documents))
				}
			},
		},
		{
			name: "multiple books",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "multibook.db")
				db, _ := sqlite.Open(dbPath)
				defer db.Close()
				db.Exec(`
					CREATE TABLE verses (id TEXT, book TEXT, chapter INTEGER, verse INTEGER, text TEXT);
					INSERT INTO verses VALUES ('Gen.1.1', 'Gen', 1, 1, 'Genesis text.');
					INSERT INTO verses VALUES ('Exod.1.1', 'Exod', 1, 1, 'Exodus text.');
					INSERT INTO verses VALUES ('Exod.1.2', 'Exod', 1, 2, 'Second verse.');
				`)
				outputDir := filepath.Join(tmpDir, "output")
				os.MkdirAll(outputDir, 0755)
				return dbPath, outputDir
			},
			wantLossClass: "L1",
			validateFunc: func(t *testing.T, corpus *ipc.Corpus) {
				if len(corpus.Documents) != 2 {
					t.Fatalf("documents count = %d, want 2", len(corpus.Documents))
				}
				// Find books by ID (order may vary)
				var genDoc, exodDoc *ipc.Document
				for _, doc := range corpus.Documents {
					if doc.ID == "Gen" {
						genDoc = doc
					} else if doc.ID == "Exod" {
						exodDoc = doc
					}
				}
				if genDoc == nil {
					t.Error("Genesis document not found")
				}
				if exodDoc == nil {
					t.Error("Exodus document not found")
				}
				if exodDoc != nil && len(exodDoc.ContentBlocks) != 2 {
					t.Errorf("Exodus verses = %d, want 2", len(exodDoc.ContentBlocks))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath, outputDir := tt.setup(t)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handleExtractIR(map[string]interface{}{
				"path":       dbPath,
				"output_dir": outputDir,
			})

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)

			var resp ipc.Response
			if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if tt.wantErr && resp.Status == "ok" {
				t.Error("expected error, got ok")
				return
			}
			if !tt.wantErr && resp.Status != "ok" {
				t.Errorf("expected ok, got error: %s", resp.Error)
				return
			}

			if tt.wantErr {
				return
			}

			result := resp.Result.(map[string]interface{})
			if result["loss_class"] != tt.wantLossClass {
				t.Errorf("loss_class = %v, want %v", result["loss_class"], tt.wantLossClass)
			}

			irPath := result["ir_path"].(string)
			data, err := os.ReadFile(irPath)
			if err != nil {
				t.Fatalf("failed to read IR: %v", err)
			}

			var corpus ipc.Corpus
			if err := json.Unmarshal(data, &corpus); err != nil {
				t.Fatalf("failed to parse IR: %v", err)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, &corpus)
			}
		})
	}
}

// TestHandleEmitNative tests the emit-native handler directly
func TestHandleEmitNative(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (irPath, outputDir string)
		validateFunc func(t *testing.T, dbPath string)
		wantErr      bool
	}{
		{
			name: "emit simple Bible",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				corpus := &ipc.Corpus{
					ID:          "test",
					Version:     "1.0.0",
					ModuleType:  "BIBLE",
					Title:       "Test Bible",
					Language:    "en",
					Description: "A test",
					Documents: []*ipc.Document{
						{
							ID:    "Gen",
							Title: "Genesis",
							Order: 1,
							ContentBlocks: []*ipc.ContentBlock{
								{
									ID:       "cb-1",
									Sequence: 1,
									Text:     "In the beginning.",
									Anchors: []*ipc.Anchor{
										{
											ID:       "a-1-0",
											Position: 0,
											Spans: []*ipc.Span{
												{
													ID:            "s-Gen.1.1",
													Type:          "VERSE",
													StartAnchorID: "a-1-0",
													Ref: &ipc.Ref{
														Book:    "Gen",
														Chapter: 1,
														Verse:   1,
														OSISID:  "Gen.1.1",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}
				irData, _ := json.MarshalIndent(corpus, "", "  ")
				irPath := filepath.Join(tmpDir, "test.ir.json")
				os.WriteFile(irPath, irData, 0644)
				outputDir := filepath.Join(tmpDir, "output")
				os.MkdirAll(outputDir, 0755)
				return irPath, outputDir
			},
			validateFunc: func(t *testing.T, dbPath string) {
				db, err := sqlite.OpenReadOnly(dbPath)
				if err != nil {
					t.Fatalf("failed to open output DB: %v", err)
				}
				defer db.Close()

				var count int
				db.QueryRow("SELECT COUNT(*) FROM verses").Scan(&count)
				if count != 1 {
					t.Errorf("verse count = %d, want 1", count)
				}

				var text string
				db.QueryRow("SELECT text FROM verses WHERE book='Gen' AND chapter=1 AND verse=1").Scan(&text)
				if text != "In the beginning." {
					t.Errorf("verse text = %q, want %q", text, "In the beginning.")
				}

				// Check metadata
				var title string
				db.QueryRow("SELECT title FROM meta LIMIT 1").Scan(&title)
				if title != "Test Bible" {
					t.Errorf("title = %q, want %q", title, "Test Bible")
				}

				// Check books table
				var bookName string
				db.QueryRow("SELECT name FROM books WHERE id='Gen'").Scan(&bookName)
				if bookName != "Genesis" {
					t.Errorf("book name = %q, want %q", bookName, "Genesis")
				}
			},
		},
		{
			name: "emit multiple books",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				corpus := &ipc.Corpus{
					ID:         "multi",
					Version:    "1.0.0",
					ModuleType: "BIBLE",
					Documents: []*ipc.Document{
						{
							ID:    "Gen",
							Title: "Genesis",
							Order: 1,
							ContentBlocks: []*ipc.ContentBlock{
								{
									ID:       "cb-1",
									Sequence: 1,
									Text:     "Genesis 1:1",
									Anchors: []*ipc.Anchor{
										{
											ID: "a-1-0",
											Spans: []*ipc.Span{
												{
													ID:   "s-Gen.1.1",
													Type: "VERSE",
													Ref:  &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 1, OSISID: "Gen.1.1"},
												},
											},
										},
									},
								},
							},
						},
						{
							ID:    "Exod",
							Title: "Exodus",
							Order: 2,
							ContentBlocks: []*ipc.ContentBlock{
								{
									ID:       "cb-2",
									Sequence: 2,
									Text:     "Exodus 1:1",
									Anchors: []*ipc.Anchor{
										{
											ID: "a-2-0",
											Spans: []*ipc.Span{
												{
													ID:   "s-Exod.1.1",
													Type: "VERSE",
													Ref:  &ipc.Ref{Book: "Exod", Chapter: 1, Verse: 1, OSISID: "Exod.1.1"},
												},
											},
										},
									},
								},
							},
						},
					},
				}
				irData, _ := json.MarshalIndent(corpus, "", "  ")
				irPath := filepath.Join(tmpDir, "multi.ir.json")
				os.WriteFile(irPath, irData, 0644)
				outputDir := filepath.Join(tmpDir, "output")
				os.MkdirAll(outputDir, 0755)
				return irPath, outputDir
			},
			validateFunc: func(t *testing.T, dbPath string) {
				db, err := sqlite.OpenReadOnly(dbPath)
				if err != nil {
					t.Fatalf("failed to open output DB: %v", err)
				}
				defer db.Close()

				var count int
				db.QueryRow("SELECT COUNT(*) FROM verses").Scan(&count)
				if count != 2 {
					t.Errorf("verse count = %d, want 2", count)
				}

				db.QueryRow("SELECT COUNT(*) FROM books").Scan(&count)
				if count != 2 {
					t.Errorf("book count = %d, want 2", count)
				}
			},
		},
		{
			name: "content block without verse span",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				corpus := &ipc.Corpus{
					ID:         "noverse",
					Version:    "1.0.0",
					ModuleType: "BIBLE",
					Documents: []*ipc.Document{
						{
							ID:    "Gen",
							Title: "Genesis",
							Order: 1,
							ContentBlocks: []*ipc.ContentBlock{
								{
									ID:       "cb-1",
									Sequence: 1,
									Text:     "Some text",
									Anchors: []*ipc.Anchor{
										{
											ID: "a-1-0",
											Spans: []*ipc.Span{
												{
													ID:   "s-1",
													Type: "HEADING", // Not a VERSE
												},
											},
										},
									},
								},
							},
						},
					},
				}
				irData, _ := json.MarshalIndent(corpus, "", "  ")
				irPath := filepath.Join(tmpDir, "noverse.ir.json")
				os.WriteFile(irPath, irData, 0644)
				outputDir := filepath.Join(tmpDir, "output")
				os.MkdirAll(outputDir, 0755)
				return irPath, outputDir
			},
			validateFunc: func(t *testing.T, dbPath string) {
				db, err := sqlite.OpenReadOnly(dbPath)
				if err != nil {
					t.Fatalf("failed to open output DB: %v", err)
				}
				defer db.Close()

				var count int
				db.QueryRow("SELECT COUNT(*) FROM verses").Scan(&count)
				if count != 0 {
					t.Errorf("verse count = %d, want 0 (no VERSE spans)", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			irPath, outputDir := tt.setup(t)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handleEmitNative(map[string]interface{}{
				"ir_path":    irPath,
				"output_dir": outputDir,
			})

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)

			var resp ipc.Response
			if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if tt.wantErr && resp.Status == "ok" {
				t.Error("expected error, got ok")
				return
			}
			if !tt.wantErr && resp.Status != "ok" {
				t.Errorf("expected ok, got error: %s", resp.Error)
				return
			}

			if tt.wantErr {
				return
			}

			result := resp.Result.(map[string]interface{})
			dbPath := result["output_path"].(string)

			if tt.validateFunc != nil {
				tt.validateFunc(t, dbPath)
			}
		})
	}
}

// TestHandleIngest tests the ingest handler directly
func TestHandleIngest(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (path, outputDir string)
		validateFunc func(t *testing.T, result map[string]interface{}, outputDir string)
		wantErr      bool
	}{
		{
			name: "ingest valid database",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				createTestDB(t, dbPath)
				outputDir := filepath.Join(tmpDir, "blobs")
				os.MkdirAll(outputDir, 0755)
				return dbPath, outputDir
			},
			validateFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				artifactID := result["artifact_id"].(string)
				if artifactID != "test" {
					t.Errorf("artifact_id = %q, want %q", artifactID, "test")
				}

				blobHash := result["blob_sha256"].(string)
				if len(blobHash) != 64 {
					t.Errorf("blob hash length = %d, want 64", len(blobHash))
				}

				blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
				if _, err := os.Stat(blobPath); os.IsNotExist(err) {
					t.Error("blob file not created")
				}

				metadata := result["metadata"].(map[string]interface{})
				if metadata["format"] != "SQLite" {
					t.Errorf("metadata format = %v, want SQLite", metadata["format"])
				}

				sizeBytes := int64(result["size_bytes"].(float64))
				if sizeBytes <= 0 {
					t.Errorf("size_bytes = %d, want > 0", sizeBytes)
				}
			},
		},
		{
			name: "ingest with complex filename",
			setup: func(t *testing.T) (string, string) {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "my-bible-v2.sqlite3")
				createTestDB(t, dbPath)
				outputDir := filepath.Join(tmpDir, "blobs")
				os.MkdirAll(outputDir, 0755)
				return dbPath, outputDir
			},
			validateFunc: func(t *testing.T, result map[string]interface{}, outputDir string) {
				artifactID := result["artifact_id"].(string)
				if artifactID != "my-bible-v2" {
					t.Errorf("artifact_id = %q, want %q", artifactID, "my-bible-v2")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, outputDir := tt.setup(t)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handleIngest(map[string]interface{}{
				"path":       path,
				"output_dir": outputDir,
			})

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)

			var resp ipc.Response
			if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if tt.wantErr && resp.Status == "ok" {
				t.Error("expected error, got ok")
				return
			}
			if !tt.wantErr && resp.Status != "ok" {
				t.Errorf("expected ok, got error: %s", resp.Error)
				return
			}

			if tt.wantErr {
				return
			}

			result := resp.Result.(map[string]interface{})
			if tt.validateFunc != nil {
				tt.validateFunc(t, result, outputDir)
			}
		})
	}
}

// TestHandleEnumerate tests the enumerate handler directly
func TestHandleEnumerate(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		validateFunc func(t *testing.T, entries []interface{})
		wantErr      bool
	}{
		{
			name: "enumerate Bible database",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				createTestDB(t, dbPath)
				return dbPath
			},
			validateFunc: func(t *testing.T, entries []interface{}) {
				if len(entries) < 3 {
					t.Errorf("entries count = %d, want at least 3", len(entries))
				}

				// Find the verses table
				foundVerses := false
				for _, e := range entries {
					entry := e.(map[string]interface{})
					if entry["path"] == "verses" {
						foundVerses = true
						metadata := entry["metadata"].(map[string]interface{})
						if metadata["type"] != "table" {
							t.Errorf("type = %v, want table", metadata["type"])
						}
						rowCount := metadata["row_count"].(string)
						if rowCount != "3" {
							t.Errorf("row_count = %v, want 3", rowCount)
						}
					}
				}
				if !foundVerses {
					t.Error("verses table not found in enumeration")
				}
			},
		},
		{
			name: "enumerate empty database",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "empty.db")
				db, _ := sqlite.Open(dbPath)
				db.Close()
				return dbPath
			},
			validateFunc: func(t *testing.T, entries []interface{}) {
				// Empty database should return an empty array (no tables)
				if len(entries) != 0 {
					t.Errorf("expected 0 entries for empty database, got %d", len(entries))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handleEnumerate(map[string]interface{}{
				"path": path,
			})

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)

			var resp ipc.Response
			if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if tt.wantErr && resp.Status == "ok" {
				t.Error("expected error, got ok")
				return
			}
			if !tt.wantErr && resp.Status != "ok" {
				t.Errorf("expected ok, got error: %s", resp.Error)
				return
			}

			if tt.wantErr {
				return
			}

			result := resp.Result.(map[string]interface{})
			var entries []interface{}
			if result["entries"] != nil {
				entries = result["entries"].([]interface{})
			}
			if tt.validateFunc != nil {
				tt.validateFunc(t, entries)
			}
		})
	}
}

// TestErrorHandling tests error paths via IPC
func TestErrorHandling(t *testing.T) {
	t.Run("detect with missing path via IPC", func(t *testing.T) {
		// This test would cause os.Exit if tested directly, so we skip it
		// Error handling is tested through the IPC interface in other tests
		t.Skip("Error paths that call os.Exit must be tested via subprocess")
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		// This would also require subprocess testing
		t.Skip("Invalid JSON input must be tested via subprocess")
	})
}

// TestRespond tests the ipc.MustRespond function
func TestRespond(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ipc.MustRespond(&ipc.DetectResult{
		Detected: true,
		Format:   "SQLite",
		Reason:   "test",
	})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
}

// TestUnknownCommand tests handling of unknown commands
func TestUnknownCommand(t *testing.T) {
	req := ipc.Request{
		Command: "unknown-command",
		Args:    map[string]interface{}{},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "error" {
		t.Errorf("expected error status for unknown command, got %s", resp.Status)
	}
	if !strings.Contains(resp.Error, "unknown command") {
		t.Errorf("expected 'unknown command' error, got: %s", resp.Error)
	}
}

// TestSQLiteDetect tests the detect command via IPC.
func TestSQLiteDetect(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	createTestDB(t, dbPath)

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": dbPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] != true {
		t.Error("expected detected to be true")
	}
	if result["format"] != "SQLite" {
		t.Errorf("expected format SQLite, got %v", result["format"])
	}
}

// TestSQLiteDetectNonSQLite tests detect command on non-SQLite file.
func TestSQLiteDetectNonSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("Hello world"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": txtPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["detected"] == true {
		t.Error("expected detected to be false for non-SQLite file")
	}
}

// TestSQLiteExtractIR tests the extract-ir command.
func TestSQLiteExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	createTestDB(t, dbPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       dbPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["loss_class"] != "L1" {
		t.Errorf("expected loss_class L1, got %v", result["loss_class"])
	}

	irPath, ok := result["ir_path"].(string)
	if !ok {
		t.Fatal("ir_path is not a string")
	}

	irData, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("expected title Test Bible, got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 3 {
		t.Errorf("expected 3 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestSQLiteEmitNative tests the emit-native command.
func TestSQLiteEmitNative(t *testing.T) {
	tmpDir := t.TempDir()

	corpus := ipc.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
		Language:   "en",
		Documents: []*ipc.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning.",
						Anchors: []*ipc.Anchor{
							{
								ID:       "a-1-0",
								Position: 0,
								Spans: []*ipc.Span{
									{
										ID:            "s-Gen.1.1",
										Type:          "VERSE",
										StartAnchorID: "a-1-0",
										Ref: &ipc.Ref{
											Book:    "Gen",
											Chapter: 1,
											Verse:   1,
											OSISID:  "Gen.1.1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(&corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	irPath := filepath.Join(tmpDir, "test.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if result["format"] != "SQLite" {
		t.Errorf("expected format SQLite, got %v", result["format"])
	}

	dbPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	// Verify the output database
	db, err := sqlite.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("failed to open output database: %v", err)
	}
	defer db.Close()

	var count int
	db.QueryRow("SELECT COUNT(*) FROM verses").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 verse, got %d", count)
	}
}

// TestSQLiteRoundTrip tests L1 semantic round-trip.
func TestSQLiteRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "original.db")
	createTestDB(t, dbPath)

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       dbPath,
			"output_dir": irDir,
		},
	}

	extractResp := executePlugin(t, &extractReq)
	if extractResp.Status != "ok" {
		t.Fatalf("extract-ir failed: %s", extractResp.Error)
	}

	extractResult := extractResp.Result.(map[string]interface{})
	irPath := extractResult["ir_path"].(string)

	// Emit native
	emitReq := ipc.Request{
		Command: "emit-native",
		Args: map[string]interface{}{
			"ir_path":    irPath,
			"output_dir": outDir,
		},
	}

	emitResp := executePlugin(t, &emitReq)
	if emitResp.Status != "ok" {
		t.Fatalf("emit-native failed: %s", emitResp.Error)
	}

	emitResult := emitResp.Result.(map[string]interface{})
	outputPath := emitResult["output_path"].(string)

	// Compare verse content (L1 - semantic comparison)
	origDB, err := sqlite.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("failed to open original: %v", err)
	}
	defer origDB.Close()

	outDB, err := sqlite.OpenReadOnly(outputPath)
	if err != nil {
		t.Fatalf("failed to open output: %v", err)
	}
	defer outDB.Close()

	// Compare verse counts
	var origCount, outCount int
	origDB.QueryRow("SELECT COUNT(*) FROM verses").Scan(&origCount)
	outDB.QueryRow("SELECT COUNT(*) FROM verses").Scan(&outCount)

	if origCount != outCount {
		t.Errorf("verse count mismatch: original %d, output %d", origCount, outCount)
	}

	// Compare verse text
	rows, _ := origDB.Query("SELECT book, chapter, verse, text FROM verses ORDER BY book, chapter, verse")
	defer rows.Close()

	for rows.Next() {
		var book, text string
		var chapter, verse int
		rows.Scan(&book, &chapter, &verse, &text)

		var outText string
		outDB.QueryRow("SELECT text FROM verses WHERE book=? AND chapter=? AND verse=?", book, chapter, verse).Scan(&outText)

		if text != outText {
			t.Errorf("text mismatch at %s.%d.%d: %q vs %q", book, chapter, verse, text, outText)
		}
	}
}

// TestSQLiteIngest tests the ingest command.
func TestSQLiteIngest(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	createTestDB(t, dbPath)

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       dbPath,
			"output_dir": outputDir,
		},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	blobHash, ok := result["blob_sha256"].(string)
	if !ok {
		t.Fatal("blob_sha256 is not a string")
	}

	blobPath := filepath.Join(outputDir, blobHash[:2], blobHash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("blob file was not created")
	}
}

// TestIPCErrorPaths tests error handling via IPC
func TestIPCErrorPaths(t *testing.T) {
	t.Run("detect with missing path", func(t *testing.T) {
		req := ipc.Request{
			Command: "detect",
			Args:    map[string]interface{}{},
		}
		resp := executePlugin(t, &req)
		if resp.Status != "error" {
			t.Errorf("expected error, got %s", resp.Status)
		}
		if !strings.Contains(resp.Error, "path argument required") {
			t.Errorf("unexpected error: %s", resp.Error)
		}
	})

	t.Run("ingest with missing arguments", func(t *testing.T) {
		req := ipc.Request{
			Command: "ingest",
			Args:    map[string]interface{}{"path": "/tmp/test.db"},
		}
		resp := executePlugin(t, &req)
		if resp.Status != "error" {
			t.Errorf("expected error, got %s", resp.Status)
		}
	})

	t.Run("extract-ir with non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		req := ipc.Request{
			Command: "extract-ir",
			Args: map[string]interface{}{
				"path":       "/nonexistent/file.db",
				"output_dir": tmpDir,
			},
		}
		resp := executePlugin(t, &req)
		if resp.Status != "error" {
			t.Errorf("expected error, got %s", resp.Status)
		}
	})

	t.Run("emit-native with missing ir_path", func(t *testing.T) {
		tmpDir := t.TempDir()
		req := ipc.Request{
			Command: "emit-native",
			Args: map[string]interface{}{
				"output_dir": tmpDir,
			},
		}
		resp := executePlugin(t, &req)
		if resp.Status != "error" {
			t.Errorf("expected error, got %s", resp.Status)
		}
	})

	t.Run("emit-native with invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		irPath := filepath.Join(tmpDir, "invalid.json")
		os.WriteFile(irPath, []byte("{invalid json}"), 0644)

		req := ipc.Request{
			Command: "emit-native",
			Args: map[string]interface{}{
				"ir_path":    irPath,
				"output_dir": tmpDir,
			},
		}
		resp := executePlugin(t, &req)
		if resp.Status != "error" {
			t.Errorf("expected error, got %s", resp.Status)
		}
	})

	t.Run("enumerate with missing path", func(t *testing.T) {
		req := ipc.Request{
			Command: "enumerate",
			Args:    map[string]interface{}{},
		}
		resp := executePlugin(t, &req)
		if resp.Status != "error" {
			t.Errorf("expected error, got %s", resp.Status)
		}
	})
}

// TestSQLiteEnumerate tests the enumerate command.
func TestSQLiteEnumerate(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	createTestDB(t, dbPath)

	req := ipc.Request{
		Command: "enumerate",
		Args:    map[string]interface{}{"path": dbPath},
	}

	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s: %s", resp.Status, resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	entries, ok := result["entries"].([]interface{})
	if !ok {
		t.Fatal("entries is not an array")
	}

	if len(entries) < 3 {
		t.Errorf("expected at least 3 tables, got %d", len(entries))
	}

	// Find verses table
	foundVerses := false
	for _, e := range entries {
		entry := e.(map[string]interface{})
		if entry["path"] == "verses" {
			foundVerses = true
			metadata := entry["metadata"].(map[string]interface{})
			if metadata["row_count"] != "3" {
				t.Errorf("expected 3 verses, got %v", metadata["row_count"])
			}
		}
	}

	if !foundVerses {
		t.Error("verses table not found in enumeration")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-sqlite"
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
		if err := buildCmd.Run(); err != nil {
			t.Fatalf("failed to build plugin: %v", err)
		}
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	cmd := exec.Command(pluginPath)
	cmd.Stdin = bytes.NewReader(reqData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			var resp ipc.Response
			if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
				return &resp
			}
		}
		t.Fatalf("plugin execution failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp ipc.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\noutput: %s", err, stdout.String())
	}

	return &resp
}
