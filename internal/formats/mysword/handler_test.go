//go:build cgo

package mysword

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func TestParserNewParser(t *testing.T) {
	// Create a test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create Books table
	_, err = db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatalf("failed to create Books table: %v", err)
	}

	// Create info table
	_, err = db.Exec("CREATE TABLE info (name TEXT, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert test data
	_, err = db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (1, 1, 1, 'In the beginning God created the heaven and the earth.')")
	if err != nil {
		t.Fatalf("failed to insert test verse: %v", err)
	}

	_, err = db.Exec("INSERT INTO info (name, value) VALUES ('description', 'Test Bible')")
	if err != nil {
		t.Fatalf("failed to insert info: %v", err)
	}

	db.Close()

	// Test NewParser
	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	if parser.db == nil {
		t.Error("parser.db is nil")
	}

	if parser.filePath != dbPath {
		t.Errorf("parser.filePath = %q, want %q", parser.filePath, dbPath)
	}
}

func TestParserGetMetadata(t *testing.T) {
	// Create a test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create info table
	_, err = db.Exec("CREATE TABLE info (name TEXT, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert metadata
	_, err = db.Exec("INSERT INTO info (name, value) VALUES ('description', 'Test Bible')")
	if err != nil {
		t.Fatalf("failed to insert info: %v", err)
	}

	_, err = db.Exec("INSERT INTO info (name, value) VALUES ('version', '1.0')")
	if err != nil {
		t.Fatalf("failed to insert version: %v", err)
	}

	db.Close()

	// Test GetMetadata
	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	desc := parser.GetMetadata("description")
	if desc != "Test Bible" {
		t.Errorf("GetMetadata('description') = %q, want %q", desc, "Test Bible")
	}

	version := parser.GetMetadata("version")
	if version != "1.0" {
		t.Errorf("GetMetadata('version') = %q, want %q", version, "1.0")
	}

	missing := parser.GetMetadata("missing")
	if missing != "" {
		t.Errorf("GetMetadata('missing') = %q, want empty string", missing)
	}
}

func TestParserGetAllVerses(t *testing.T) {
	// Create a test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create Books table
	_, err = db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatalf("failed to create Books table: %v", err)
	}

	// Insert test verses
	verses := []struct {
		book    int
		chapter int
		verse   int
		text    string
	}{
		{1, 1, 1, "In the beginning God created the heaven and the earth."},
		{1, 1, 2, "And the earth was without form, and void."},
		{40, 1, 1, "The book of the generation of Jesus Christ."},
	}

	for _, v := range verses {
		_, err = db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
			v.book, v.chapter, v.verse, v.text)
		if err != nil {
			t.Fatalf("failed to insert verse: %v", err)
		}
	}

	db.Close()

	// Test GetAllVerses
	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	allVerses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses failed: %v", err)
	}

	if len(allVerses) != 3 {
		t.Errorf("GetAllVerses returned %d verses, want 3", len(allVerses))
	}

	// Check first verse
	if allVerses[0].Book != 1 || allVerses[0].Chapter != 1 || allVerses[0].Verse != 1 {
		t.Errorf("First verse reference = %d:%d:%d, want 1:1:1",
			allVerses[0].Book, allVerses[0].Chapter, allVerses[0].Verse)
	}

	if allVerses[0].Text != "In the beginning God created the heaven and the earth." {
		t.Errorf("First verse text = %q, want %q",
			allVerses[0].Text, "In the beginning God created the heaven and the earth.")
	}
}

func TestParserGetAllVersesWithHTML(t *testing.T) {
	// Create a test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create Books table
	_, err = db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatalf("failed to create Books table: %v", err)
	}

	// Insert verse with HTML
	_, err = db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
		1, 1, 1, "In the <i>beginning</i> God <b>created</b> the heaven.")
	if err != nil {
		t.Fatalf("failed to insert verse: %v", err)
	}

	db.Close()

	// Test GetAllVerses
	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	allVerses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses failed: %v", err)
	}

	if len(allVerses) != 1 {
		t.Fatalf("GetAllVerses returned %d verses, want 1", len(allVerses))
	}

	// HTML should be stripped
	expected := "In the beginning God created the heaven."
	if allVerses[0].Text != expected {
		t.Errorf("Verse text = %q, want %q", allVerses[0].Text, expected)
	}
}

func TestDetectModuleType(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"bible.mybible", "bible"},
		{"commentary.commentaries.mybible", "commentary"},
		{"dict.dictionary.mybible", "dictionary"},
		{"BIBLE.MYBIBLE", "bible"},
		{"test.txt", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := DetectModuleType(tt.filename)
			if got != tt.want {
				t.Errorf("DetectModuleType(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no HTML",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "simple tags",
			input: "<b>bold</b> and <i>italic</i>",
			want:  "bold and italic",
		},
		{
			name:  "nested tags",
			input: "<b>bold <i>and italic</i></b>",
			want:  "bold and italic",
		},
		{
			name:  "self-closing tags",
			input: "text<br/>more text",
			want:  "textmore text",
		},
		{
			name:  "incomplete tag",
			input: "text <incomplete",
			want:  "text <incomplete",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "only tags",
			input: "<b></b>",
			want:  "",
		},
		{
			name:  "multiple consecutive tags",
			input: "<b><i>text</i></b>",
			want:  "text",
		},
		{
			name:  "whitespace",
			input: "  <b>text</b>  ",
			want:  "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandlerExtractIR(t *testing.T) {
	// Create a test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create Books table
	_, err = db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatalf("failed to create Books table: %v", err)
	}

	// Create info table
	_, err = db.Exec("CREATE TABLE info (name TEXT, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert test data
	_, err = db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (1, 1, 1, 'In the beginning God created the heaven and the earth.')")
	if err != nil {
		t.Fatalf("failed to insert test verse: %v", err)
	}

	_, err = db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (1, 1, 2, 'And the earth was without form, and void.')")
	if err != nil {
		t.Fatalf("failed to insert test verse: %v", err)
	}

	_, err = db.Exec("INSERT INTO info (name, value) VALUES ('description', 'Test Bible')")
	if err != nil {
		t.Fatalf("failed to insert info: %v", err)
	}

	_, err = db.Exec("INSERT INTO info (name, value) VALUES ('version', '1.0')")
	if err != nil {
		t.Fatalf("failed to insert version: %v", err)
	}

	db.Close()

	// Test ExtractIR
	handler := &Handler{}
	outputDir := t.TempDir()

	result, err := handler.ExtractIR(dbPath, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Check result
	if result.LossClass != "L1" {
		t.Errorf("LossClass = %q, want L1", result.LossClass)
	}

	if result.IRPath == "" {
		t.Error("IRPath is empty")
	}

	// Read and validate IR file
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	// Validate corpus
	if corpus.ID != "test" {
		t.Errorf("Corpus.ID = %q, want %q", corpus.ID, "test")
	}

	if corpus.ModuleType != "BIBLE" {
		t.Errorf("Corpus.ModuleType = %q, want BIBLE", corpus.ModuleType)
	}

	if corpus.Title != "Test Bible" {
		t.Errorf("Corpus.Title = %q, want %q", corpus.Title, "Test Bible")
	}

	if len(corpus.Documents) != 1 {
		t.Fatalf("len(Corpus.Documents) = %d, want 1", len(corpus.Documents))
	}

	doc := corpus.Documents[0]
	if doc.ID != "Gen" {
		t.Errorf("Document.ID = %q, want Gen", doc.ID)
	}

	if len(doc.ContentBlocks) != 2 {
		t.Fatalf("len(Document.ContentBlocks) = %d, want 2", len(doc.ContentBlocks))
	}

	// Check first verse
	cb := doc.ContentBlocks[0]
	if cb.Text != "In the beginning God created the heaven and the earth." {
		t.Errorf("ContentBlock.Text = %q, want %q",
			cb.Text, "In the beginning God created the heaven and the earth.")
	}

	if len(cb.Anchors) != 1 {
		t.Fatalf("len(ContentBlock.Anchors) = %d, want 1", len(cb.Anchors))
	}

	anchor := cb.Anchors[0]
	if len(anchor.Spans) != 1 {
		t.Fatalf("len(Anchor.Spans) = %d, want 1", len(anchor.Spans))
	}

	span := anchor.Spans[0]
	if span.Type != "VERSE" {
		t.Errorf("Span.Type = %q, want VERSE", span.Type)
	}

	if span.Ref == nil {
		t.Fatal("Span.Ref is nil")
	}

	if span.Ref.Book != "Gen" {
		t.Errorf("Span.Ref.Book = %q, want Gen", span.Ref.Book)
	}

	if span.Ref.Chapter != 1 {
		t.Errorf("Span.Ref.Chapter = %d, want 1", span.Ref.Chapter)
	}

	if span.Ref.Verse != 1 {
		t.Errorf("Span.Ref.Verse = %d, want 1", span.Ref.Verse)
	}
}

func TestHandlerEmitNative(t *testing.T) {
	// Create a test IR corpus
	corpus := &ipc.Corpus{
		ID:           "test-emit",
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		Title:        "Test Emit Bible",
		Description:  "Test description",
		Language:     "en",
		SourceFormat: "MySword",
		LossClass:    "L1",
		Attributes: map[string]string{
			"version": "1.0",
		},
		Documents: []*ipc.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				Attributes: map[string]string{
					"book_num": "1",
				},
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning God created the heaven and the earth.",
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

	// Serialize to JSON
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to serialize corpus: %v", err)
	}

	// Write IR file
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "test-emit.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	// Test EmitNative
	handler := &Handler{}
	outputDir := t.TempDir()

	result, err := handler.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Check result
	if result.Format != "MySword" {
		t.Errorf("Format = %q, want MySword", result.Format)
	}

	if result.LossClass != "L1" {
		t.Errorf("LossClass = %q, want L1", result.LossClass)
	}

	if result.OutputPath == "" {
		t.Error("OutputPath is empty")
	}

	// Verify output file exists
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Errorf("Output file does not exist: %s", result.OutputPath)
	}

	// Open and verify database
	db, err := sqlite.OpenReadOnly(result.OutputPath)
	if err != nil {
		t.Fatalf("failed to open output database: %v", err)
	}
	defer db.Close()

	// Check Books table
	var book, chapter, verse int
	var scripture string
	err = db.QueryRow("SELECT Book, Chapter, Verse, Scripture FROM Books WHERE Book = 1 AND Chapter = 1 AND Verse = 1").Scan(&book, &chapter, &verse, &scripture)
	if err != nil {
		t.Fatalf("failed to query Books table: %v", err)
	}

	if book != 1 || chapter != 1 || verse != 1 {
		t.Errorf("Book reference = %d:%d:%d, want 1:1:1", book, chapter, verse)
	}

	if scripture != "In the beginning God created the heaven and the earth." {
		t.Errorf("Scripture = %q, want %q", scripture, "In the beginning God created the heaven and the earth.")
	}

	// Check info table
	var name, value string
	err = db.QueryRow("SELECT name, value FROM info WHERE name = 'description'").Scan(&name, &value)
	if err != nil {
		t.Fatalf("failed to query info table: %v", err)
	}

	if value != "Test Emit Bible" {
		t.Errorf("info.description = %q, want %q", value, "Test Emit Bible")
	}
}

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.mysword" {
		t.Errorf("PluginID = %q, want format.mysword", m.PluginID)
	}
	if m.Kind != "format" {
		t.Errorf("Kind = %q, want format", m.Kind)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want 1.0.0", m.Version)
	}
}

func TestRegister(t *testing.T) {
	// Register should not panic when called multiple times
	Register()
}

func TestHandlerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	t.Run("mybible file", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "test.mybible")
		// Just create a simple file with .mybible extension
		if err := os.WriteFile(dbPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := h.Detect(dbPath)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if !result.Detected {
			t.Error("Expected .mybible file to be detected")
		}
		if result.Format != "mysword" {
			t.Errorf("Format = %q, want mysword", result.Format)
		}
	})

	t.Run("non-mybible extension", func(t *testing.T) {
		txtPath := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(txtPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := h.Detect(txtPath)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Expected non-mybible file to not be detected")
		}
		if result.Reason != "not a .mybible file" {
			t.Errorf("Reason = %q, want 'not a .mybible file'", result.Reason)
		}
	})

	t.Run("directory", func(t *testing.T) {
		result, err := h.Detect(tmpDir)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Expected directory to not be detected")
		}
		if result.Reason != "path is a directory" {
			t.Errorf("Reason = %q, want 'path is a directory'", result.Reason)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result, err := h.Detect(filepath.Join(tmpDir, "nonexistent.mybible"))
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if result.Detected {
			t.Error("Expected nonexistent file to not be detected")
		}
	})
}

func TestHandlerIngest(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	t.Run("success", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "test.mybible")
		db, err := sqlite.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
		db.Close()

		outputDir := filepath.Join(tmpDir, "output")

		result, err := h.Ingest(dbPath, outputDir)
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}

		if result.ArtifactID != "test" {
			t.Errorf("ArtifactID = %q, want test", result.ArtifactID)
		}
		if result.BlobSHA256 == "" {
			t.Error("BlobSHA256 should not be empty")
		}
		if result.Metadata["format"] != "mysword" {
			t.Errorf("Metadata format = %q, want mysword", result.Metadata["format"])
		}

		// Verify blob was written
		blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
		if _, err := os.Stat(blobPath); os.IsNotExist(err) {
			t.Error("Blob file was not created")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := h.Ingest(filepath.Join(tmpDir, "nonexistent.mybible"), tmpDir)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("non-writable output", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "test2.mybible")
		db, _ := sqlite.Open(dbPath)
		db.Close()

		_, err := h.Ingest(dbPath, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestHandlerEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	t.Run("success", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "test.mybible")
		content := []byte("test content for size")
		if err := os.WriteFile(dbPath, content, 0644); err != nil {
			t.Fatal(err)
		}

		result, err := h.Enumerate(dbPath)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
		}

		entry := result.Entries[0]
		if entry.Path != "test.mybible" {
			t.Errorf("Path = %q, want test.mybible", entry.Path)
		}
		if entry.SizeBytes != int64(len(content)) {
			t.Errorf("SizeBytes = %d, want %d", entry.SizeBytes, len(content))
		}
		if entry.IsDir {
			t.Error("IsDir should be false")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := h.Enumerate(filepath.Join(tmpDir, "nonexistent.mybible"))
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestHandlerEmitNativeCommentary(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create commentary corpus
	corpus := &ipc.Corpus{
		ID:           "test-commentary",
		Version:      "1.0.0",
		ModuleType:   "COMMENTARY",
		Title:        "Test Commentary",
		SourceFormat: "MySword",
		LossClass:    "L1",
		Attributes:   make(map[string]string),
		Documents: []*ipc.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "This is a commentary on Genesis 1:1",
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
	irPath := filepath.Join(tmpDir, "test-commentary.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Check extension
	if !strings.HasSuffix(result.OutputPath, ".commentaries.mybible") {
		t.Errorf("OutputPath = %q, should end with .commentaries.mybible", result.OutputPath)
	}

	// Verify database has commentaries table
	db, err := sqlite.OpenReadOnly(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer db.Close()

	var text string
	err = db.QueryRow("SELECT text FROM commentaries WHERE book_number = 1 AND chapter_number_from = 1 AND verse_number_from = 1").Scan(&text)
	if err != nil {
		t.Fatalf("Failed to query commentaries: %v", err)
	}

	if text != "This is a commentary on Genesis 1:1" {
		t.Errorf("Commentary text = %q, want expected text", text)
	}
}

func TestHandlerEmitNativeDictionary(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create dictionary corpus
	corpus := &ipc.Corpus{
		ID:           "test-dictionary",
		Version:      "1.0.0",
		ModuleType:   "DICTIONARY",
		Title:        "Test Dictionary",
		SourceFormat: "MySword",
		LossClass:    "L1",
		Attributes:   make(map[string]string),
		Documents: []*ipc.Document{
			{
				ID:    "entries",
				Title: "Entries",
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:         "cb-1",
						Sequence:   1,
						Text:       "Definition of grace",
						Attributes: map[string]interface{}{"topic": "grace"},
					},
					{
						ID:         "cb-2",
						Sequence:   2,
						Text:       "Definition of faith",
						Attributes: map[string]interface{}{"topic": "faith"},
					},
				},
			},
		},
	}

	irData, _ := json.MarshalIndent(corpus, "", "  ")
	irPath := filepath.Join(tmpDir, "test-dictionary.ir.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := h.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Check extension
	if !strings.HasSuffix(result.OutputPath, ".dictionary.mybible") {
		t.Errorf("OutputPath = %q, should end with .dictionary.mybible", result.OutputPath)
	}

	// Verify database has dictionary table
	db, err := sqlite.OpenReadOnly(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to open output: %v", err)
	}
	defer db.Close()

	var topic, definition string
	err = db.QueryRow("SELECT topic, definition FROM dictionary WHERE topic = 'grace'").Scan(&topic, &definition)
	if err != nil {
		t.Fatalf("Failed to query dictionary: %v", err)
	}

	if definition != "Definition of grace" {
		t.Errorf("Dictionary definition = %q, want expected text", definition)
	}
}

func TestHandlerExtractIRErrors(t *testing.T) {
	h := &Handler{}

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := h.ExtractIR("/nonexistent/path.mybible", t.TempDir())
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("non-writable output", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.mybible")

		db, _ := sqlite.Open(dbPath)
		db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
		db.Exec("INSERT INTO Books VALUES (1, 1, 1, 'Test verse')")
		db.Close()

		_, err := h.ExtractIR(dbPath, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestHandlerEmitNativeErrors(t *testing.T) {
	h := &Handler{}

	t.Run("nonexistent IR file", func(t *testing.T) {
		_, err := h.EmitNative("/nonexistent/ir.json", t.TempDir())
		if err == nil {
			t.Error("Expected error for nonexistent IR file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		irPath := filepath.Join(tmpDir, "invalid.json")
		if err := os.WriteFile(irPath, []byte("not valid json"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := h.EmitNative(irPath, tmpDir)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})
}

func TestVersesToIRUnknownBook(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, _ := sqlite.Open(dbPath)
	db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	// Insert verse with unknown book number (99) - books 1-66 are canonical
	// The versesToIR only adds documents for books 1-66, so book 99 won't be added
	db.Exec("INSERT INTO Books VALUES (99, 1, 1, 'Unknown book verse')")
	db.Close()

	parser, _ := NewParser(dbPath)
	defer parser.Close()

	verses, _ := parser.GetAllVerses()
	h := &Handler{}
	corpus, _ := h.versesToIR(dbPath, verses, parser)

	// Book 99 is outside the canonical 66 books range, so it won't be included
	// in the final corpus (versesToIR only iterates 1-66)
	// Test that versesToIR handles this gracefully without error
	if corpus.ID == "" {
		t.Error("Expected corpus ID to be set")
	}
	if corpus.ModuleType != "BIBLE" {
		t.Errorf("ModuleType = %q, want BIBLE", corpus.ModuleType)
	}
}

func TestEmitBibleNativeWithOSISID(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	// Create corpus with OSIS ID but no book_num attribute
	corpus := &ipc.Corpus{
		ID:           "test-osis",
		Version:      "1.0.0",
		ModuleType:   "BIBLE",
		Title:        "Test OSIS ID",
		SourceFormat: "MySword",
		LossClass:    "L1",
		Attributes:   make(map[string]string),
		Documents: []*ipc.Document{
			{
				ID:         "Gen", // OSIS ID without book_num attribute
				Title:      "Genesis",
				Order:      1,
				Attributes: map[string]string{}, // No book_num
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "cb-1",
						Sequence: 1,
						Text:     "In the beginning",
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
	irPath := filepath.Join(tmpDir, "test-osis.ir.json")
	os.WriteFile(irPath, irData, 0644)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	result, err := h.EmitNative(irPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Verify database
	db, _ := sqlite.OpenReadOnly(result.OutputPath)
	defer db.Close()

	var book int
	err = db.QueryRow("SELECT Book FROM Books WHERE Chapter = 1 AND Verse = 1").Scan(&book)
	if err != nil {
		t.Fatalf("Failed to query Books: %v", err)
	}

	// Should have converted "Gen" to book number 1
	if book != 1 {
		t.Errorf("Book number = %d, want 1", book)
	}
}

func TestParserGetAllVersesAlternateTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, _ := sqlite.Open(dbPath)
	// Create "Bible" table instead of "Books"
	db.Exec("CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	db.Exec("INSERT INTO Bible VALUES (1, 1, 1, 'First verse')")
	db.Close()

	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses failed: %v", err)
	}

	if len(verses) != 1 {
		t.Errorf("Expected 1 verse, got %d", len(verses))
	}
}

func TestHandlerRoundTrip(t *testing.T) {
	// Create a test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.mybible")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create Books table
	_, err = db.Exec("CREATE TABLE Books (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatalf("failed to create Books table: %v", err)
	}

	// Create info table
	_, err = db.Exec("CREATE TABLE info (name TEXT, value TEXT)")
	if err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert test verses
	testVerses := []struct {
		book    int
		chapter int
		verse   int
		text    string
	}{
		{1, 1, 1, "In the beginning God created the heaven and the earth."},
		{1, 1, 2, "And the earth was without form, and void."},
		{1, 1, 3, "And God said, Let there be light: and there was light."},
	}

	for _, v := range testVerses {
		_, err = db.Exec("INSERT INTO Books (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
			v.book, v.chapter, v.verse, v.text)
		if err != nil {
			t.Fatalf("failed to insert verse: %v", err)
		}
	}

	_, err = db.Exec("INSERT INTO info (name, value) VALUES ('description', 'Round Trip Test')")
	if err != nil {
		t.Fatalf("failed to insert info: %v", err)
	}

	db.Close()

	// Extract IR
	handler := &Handler{}
	irDir := t.TempDir()

	extractResult, err := handler.ExtractIR(dbPath, irDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Emit Native
	outputDir := t.TempDir()
	emitResult, err := handler.EmitNative(extractResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Open emitted database
	db2, err := sqlite.OpenReadOnly(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to open emitted database: %v", err)
	}
	defer db2.Close()

	// Verify all verses are present
	for _, tv := range testVerses {
		var scripture string
		err = db2.QueryRow("SELECT Scripture FROM Books WHERE Book = ? AND Chapter = ? AND Verse = ?",
			tv.book, tv.chapter, tv.verse).Scan(&scripture)
		if err != nil {
			t.Errorf("failed to find verse %d:%d:%d: %v", tv.book, tv.chapter, tv.verse, err)
			continue
		}

		if scripture != tv.text {
			t.Errorf("verse %d:%d:%d text = %q, want %q", tv.book, tv.chapter, tv.verse, scripture, tv.text)
		}
	}

	// Verify metadata
	var value string
	err = db2.QueryRow("SELECT value FROM info WHERE name = 'description'").Scan(&value)
	if err != nil {
		t.Fatalf("failed to query info table: %v", err)
	}

	if value != "Round Trip Test" {
		t.Errorf("info.description = %q, want %q", value, "Round Trip Test")
	}
}
