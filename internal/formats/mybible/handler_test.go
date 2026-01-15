//go:build cgo

package mybible

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// createTestDatabase creates a minimal test MyBible database.
func createTestDatabase(t *testing.T, path string) {
	t.Helper()

	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer db.Close()

	// Create verses table
	if _, err := db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("failed to create verses table: %v", err)
	}

	// Create info table
	if _, err := db.Exec("CREATE TABLE info (name TEXT NOT NULL, value TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create info table: %v", err)
	}

	// Insert sample verses (Genesis 1:1-3)
	verses := []struct {
		bookNum, chapter, verse int
		text                    string
	}{
		{1, 1, 1, "In the beginning God created the heaven and the earth."},
		{1, 1, 2, "And the earth was without form, and void; and darkness was upon the face of the deep."},
		{1, 1, 3, "And God said, Let there be light: and there was light."},
		{40, 1, 1, "The book of the generation of Jesus Christ, the son of David, the son of Abraham."},
	}

	for _, v := range verses {
		if _, err := db.Exec("INSERT INTO verses (book_number, chapter, verse, text) VALUES (?, ?, ?, ?)",
			v.bookNum, v.chapter, v.verse, v.text); err != nil {
			t.Fatalf("failed to insert verse: %v", err)
		}
	}

	// Insert metadata
	db.Exec("INSERT INTO info (name, value) VALUES ('description', 'Test Bible')")
	db.Exec("INSERT INTO info (name, value) VALUES ('detailed_info', 'A test Bible for unit tests')")
	db.Exec("INSERT INTO info (name, value) VALUES ('language', 'en')")
	db.Exec("INSERT INTO info (name, value) VALUES ('version', '1.0')")
}

// TestParser_NewParser tests creating a parser.
func TestParser_NewParser(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	if parser.db == nil {
		t.Error("parser.db is nil")
	}
	if parser.filePath != dbPath {
		t.Errorf("parser.filePath = %s, want %s", parser.filePath, dbPath)
	}
}

// TestParser_GetMetadata tests metadata retrieval.
func TestParser_GetMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"description", "description", "Test Bible"},
		{"detailed_info", "detailed_info", "A test Bible for unit tests"},
		{"language", "language", "en"},
		{"version", "version", "1.0"},
		{"nonexistent", "nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.GetMetadata(tt.key)
			if got != tt.expected {
				t.Errorf("GetMetadata(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

// TestParser_GetAllVerses tests verse retrieval.
func TestParser_GetAllVerses(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}
	defer parser.Close()

	verses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses failed: %v", err)
	}

	if len(verses) != 4 {
		t.Fatalf("GetAllVerses returned %d verses, want 4", len(verses))
	}

	// Check first verse
	if verses[0].BookNumber != 1 {
		t.Errorf("verse[0].BookNumber = %d, want 1", verses[0].BookNumber)
	}
	if verses[0].Chapter != 1 {
		t.Errorf("verse[0].Chapter = %d, want 1", verses[0].Chapter)
	}
	if verses[0].Verse != 1 {
		t.Errorf("verse[0].Verse = %d, want 1", verses[0].Verse)
	}
	if !strings.Contains(verses[0].Text, "In the beginning") {
		t.Errorf("verse[0].Text = %q, want to contain 'In the beginning'", verses[0].Text)
	}
}

// TestParser_GetAllVerses_HTMLStripping tests HTML tag removal.
func TestParser_GetAllVerses_HTMLStripping(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Create verses table
	db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`)

	// Insert verse with HTML
	db.Exec("INSERT INTO verses (book_number, chapter, verse, text) VALUES (?, ?, ?, ?)",
		1, 1, 1, "In the <i>beginning</i> God <b>created</b> the heaven.")
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
		t.Fatalf("GetAllVerses returned %d verses, want 1", len(verses))
	}

	// Check HTML was stripped
	if strings.Contains(verses[0].Text, "<") || strings.Contains(verses[0].Text, ">") {
		t.Errorf("verse text still contains HTML tags: %q", verses[0].Text)
	}
	if !strings.Contains(verses[0].Text, "beginning") || !strings.Contains(verses[0].Text, "created") {
		t.Errorf("verse text missing content after HTML stripping: %q", verses[0].Text)
	}
}

// TestParser_Close tests closing the parser.
func TestParser_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	parser, err := NewParser(dbPath)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}

	err = parser.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second close should be safe
	err = parser.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// TestHandler_Detect tests detection of MyBible files.
func TestHandler_Detect(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	handler := &Handler{}
	result, err := handler.Detect(dbPath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.Detected {
		t.Errorf("Detect returned false, want true. Reason: %s", result.Reason)
	}
	if result.Format != "mybible" {
		t.Errorf("Detect format = %s, want mybible", result.Format)
	}
}

// TestHandler_Detect_WrongExtension tests detection with wrong extension.
func TestHandler_Detect_WrongExtension(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(dbPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.Detect(dbPath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Detect returned true for .txt file, want false")
	}
}

// TestHandler_Detect_Directory tests detection with directory.
func TestHandler_Detect_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	handler := &Handler{}
	result, err := handler.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.Detected {
		t.Error("Detect returned true for directory, want false")
	}
	if !strings.Contains(result.Reason, "directory") {
		t.Errorf("Detect reason should mention directory, got: %s", result.Reason)
	}
}

// TestHandler_Ingest tests ingesting a MyBible file.
func TestHandler_Ingest(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	outputDir := filepath.Join(tmpDir, "output")

	handler := &Handler{}
	result, err := handler.Ingest(dbPath, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("ArtifactID = %s, want test", result.ArtifactID)
	}
	if result.BlobSHA256 == "" {
		t.Error("BlobSHA256 is empty")
	}
	if result.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want > 0", result.SizeBytes)
	}
	if result.Metadata["format"] != "mybible" {
		t.Errorf("Metadata[format] = %s, want mybible", result.Metadata["format"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

// TestHandler_Enumerate tests enumerating a MyBible file.
func TestHandler_Enumerate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	handler := &Handler{}
	result, err := handler.Enumerate(dbPath)
	if err != nil {
		t.Fatalf("Enumerate failed: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Enumerate returned %d entries, want 1", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.SQLite3" {
		t.Errorf("Entry path = %s, want test.SQLite3", entry.Path)
	}
	if entry.IsDir {
		t.Error("Entry IsDir = true, want false")
	}
}

// TestHandler_ExtractIR tests IR extraction.
func TestHandler_ExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.SQLite3")
	createTestDatabase(t, dbPath)

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.ExtractIR(dbPath, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	if result.IRPath == "" {
		t.Error("IRPath is empty")
	}
	if result.LossClass != "L1" {
		t.Errorf("LossClass = %s, want L1", result.LossClass)
	}

	// Verify IR file exists
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Error("IR file does not exist")
	}

	// Read and verify IR content
	data, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if corpus.ID != "test" {
		t.Errorf("corpus.ID = %s, want test", corpus.ID)
	}
	if corpus.ModuleType != "BIBLE" {
		t.Errorf("corpus.ModuleType = %s, want BIBLE", corpus.ModuleType)
	}
	if corpus.SourceFormat != "mybible" {
		t.Errorf("corpus.SourceFormat = %s, want mybible", corpus.SourceFormat)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("corpus.Title = %s, want Test Bible", corpus.Title)
	}
	if corpus.Language != "en" {
		t.Errorf("corpus.Language = %s, want en", corpus.Language)
	}

	// Verify documents
	if len(corpus.Documents) < 2 {
		t.Fatalf("corpus.Documents length = %d, want at least 2", len(corpus.Documents))
	}

	// Check Genesis document
	genDoc := corpus.Documents[0]
	if genDoc.ID != "Gen" {
		t.Errorf("first document ID = %s, want Gen", genDoc.ID)
	}
	if genDoc.Order != 1 {
		t.Errorf("first document Order = %d, want 1", genDoc.Order)
	}
	if len(genDoc.ContentBlocks) != 3 {
		t.Errorf("Genesis has %d content blocks, want 3", len(genDoc.ContentBlocks))
	}

	// Check first verse
	cb := genDoc.ContentBlocks[0]
	if !strings.Contains(cb.Text, "In the beginning") {
		t.Errorf("first verse text = %q, want to contain 'In the beginning'", cb.Text)
	}
	if len(cb.Anchors) == 0 {
		t.Fatal("first verse has no anchors")
	}
	if len(cb.Anchors[0].Spans) == 0 {
		t.Fatal("first verse has no spans")
	}

	span := cb.Anchors[0].Spans[0]
	if span.Type != "VERSE" {
		t.Errorf("span.Type = %s, want VERSE", span.Type)
	}
	if span.Ref == nil {
		t.Fatal("span.Ref is nil")
	}
	if span.Ref.Book != "Gen" {
		t.Errorf("span.Ref.Book = %s, want Gen", span.Ref.Book)
	}
	if span.Ref.Chapter != 1 {
		t.Errorf("span.Ref.Chapter = %d, want 1", span.Ref.Chapter)
	}
	if span.Ref.Verse != 1 {
		t.Errorf("span.Ref.Verse = %d, want 1", span.Ref.Verse)
	}
}

// TestHandler_EmitNative tests native format emission.
func TestHandler_EmitNative(t *testing.T) {
	tmpDir := t.TempDir()

	// First create a database and extract IR
	dbPath := filepath.Join(tmpDir, "original.SQLite3")
	createTestDatabase(t, dbPath)

	irOutputDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(irOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	extractResult, err := handler.ExtractIR(dbPath, irOutputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Now emit native format
	emitOutputDir := filepath.Join(tmpDir, "emit")
	if err := os.MkdirAll(emitOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	emitResult, err := handler.EmitNative(extractResult.IRPath, emitOutputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	if emitResult.OutputPath == "" {
		t.Error("OutputPath is empty")
	}
	if emitResult.Format != "mybible" {
		t.Errorf("Format = %s, want mybible", emitResult.Format)
	}
	if emitResult.LossClass != "L1" {
		t.Errorf("LossClass = %s, want L1", emitResult.LossClass)
	}

	// Verify output file exists
	if _, err := os.Stat(emitResult.OutputPath); os.IsNotExist(err) {
		t.Error("Output file does not exist")
	}

	// Verify output is a valid SQLite database
	db, err := sqlite.OpenReadOnly(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to open emitted database: %v", err)
	}
	defer db.Close()

	// Check verses table exists and has data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM verses").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query verses: %v", err)
	}
	if count != 4 {
		t.Errorf("verses count = %d, want 4", count)
	}

	// Check info table exists and has metadata
	rows, err := db.Query("SELECT name, value FROM info WHERE name = 'description'")
	if err != nil {
		t.Fatalf("failed to query info: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Error("info table has no description")
	}

	var name, value string
	if err := rows.Scan(&name, &value); err != nil {
		t.Fatalf("failed to scan info: %v", err)
	}
	if value != "Test Bible" {
		t.Errorf("description = %s, want Test Bible", value)
	}
}

// TestHandler_RoundTrip tests extracting IR and emitting back to native format.
func TestHandler_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create original database
	originalPath := filepath.Join(tmpDir, "original.SQLite3")
	createTestDatabase(t, originalPath)

	handler := &Handler{}

	// Extract to IR
	irOutputDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(irOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	extractResult, err := handler.ExtractIR(originalPath, irOutputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Emit back to native
	emitOutputDir := filepath.Join(tmpDir, "emit")
	if err := os.MkdirAll(emitOutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	emitResult, err := handler.EmitNative(extractResult.IRPath, emitOutputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Compare original and emitted databases
	originalDB, err := sqlite.OpenReadOnly(originalPath)
	if err != nil {
		t.Fatalf("failed to open original database: %v", err)
	}
	defer originalDB.Close()

	emittedDB, err := sqlite.OpenReadOnly(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to open emitted database: %v", err)
	}
	defer emittedDB.Close()

	// Compare verse counts
	var originalCount, emittedCount int
	originalDB.QueryRow("SELECT COUNT(*) FROM verses").Scan(&originalCount)
	emittedDB.QueryRow("SELECT COUNT(*) FROM verses").Scan(&emittedCount)

	if originalCount != emittedCount {
		t.Errorf("verse count mismatch: original=%d, emitted=%d", originalCount, emittedCount)
	}

	// Compare first verse text
	var originalText, emittedText string
	originalDB.QueryRow("SELECT text FROM verses WHERE book_number=1 AND chapter=1 AND verse=1").Scan(&originalText)
	emittedDB.QueryRow("SELECT text FROM verses WHERE book_number=1 AND chapter=1 AND verse=1").Scan(&emittedText)

	if originalText != emittedText {
		t.Errorf("verse text mismatch:\noriginal: %q\nemitted:  %q", originalText, emittedText)
	}

	// Compare metadata
	var originalDesc, emittedDesc string
	originalDB.QueryRow("SELECT value FROM info WHERE name='description'").Scan(&originalDesc)
	emittedDB.QueryRow("SELECT value FROM info WHERE name='description'").Scan(&emittedDesc)

	if originalDesc != emittedDesc {
		t.Errorf("description mismatch: original=%q, emitted=%q", originalDesc, emittedDesc)
	}
}

// TestBookNumToOSIS tests book number to OSIS conversion.
func TestBookNumToOSIS(t *testing.T) {
	tests := []struct {
		bookNum  int
		expected string
	}{
		{1, "Gen"},
		{2, "Exod"},
		{19, "Ps"},
		{40, "Matt"},
		{66, "Rev"},
		{999, "Book999"}, // Unknown book
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := bookNumToOSIS(tt.bookNum)
			if got != tt.expected {
				t.Errorf("bookNumToOSIS(%d) = %s, want %s", tt.bookNum, got, tt.expected)
			}
		})
	}
}

// TestOSISToBookNum tests OSIS to book number conversion.
func TestOSISToBookNum(t *testing.T) {
	tests := []struct {
		osis     string
		expected int
	}{
		{"Gen", 1},
		{"Exod", 2},
		{"Ps", 19},
		{"Matt", 40},
		{"Rev", 66},
		{"Unknown", 0}, // Unknown book
	}

	for _, tt := range tests {
		t.Run(tt.osis, func(t *testing.T) {
			got := osisToBookNum(tt.osis)
			if got != tt.expected {
				t.Errorf("osisToBookNum(%s) = %d, want %d", tt.osis, got, tt.expected)
			}
		})
	}
}

// TestStripHTML tests the HTML stripping function.
func TestStripHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no html", "plain text", "plain text"},
		{"italic", "text with <i>italic</i> words", "text with italic words"},
		{"bold", "text with <b>bold</b> words", "text with bold words"},
		{"multiple tags", "<i>italic</i> and <b>bold</b>", "italic and bold"},
		{"nested tags", "<b><i>nested</i></b>", "nested"},
		{"empty tags", "text <i></i> more", "text  more"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.expected {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestHandler_ExtractIR_EmptyDatabase tests IR extraction from empty database.
func TestHandler_ExtractIR_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.SQLite3")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	// Create tables but insert no data
	db.Exec(`CREATE TABLE verses (
		book_number INTEGER NOT NULL,
		chapter INTEGER NOT NULL,
		verse INTEGER NOT NULL,
		text TEXT NOT NULL
	)`)
	db.Exec("CREATE TABLE info (name TEXT NOT NULL, value TEXT NOT NULL)")
	db.Close()

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	result, err := handler.ExtractIR(dbPath, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Read and verify IR content
	data, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	if len(corpus.Documents) != 0 {
		t.Errorf("corpus has %d documents, want 0", len(corpus.Documents))
	}
}

// TestHandler_EmitNative_InvalidIRPath tests emission with invalid IR path.
func TestHandler_EmitNative_InvalidIRPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.EmitNative("/nonexistent/ir.json", outputDir)
	if err == nil {
		t.Error("Expected error for invalid IR path")
	}
}

// TestHandler_EmitNative_InvalidJSON tests emission with invalid JSON.
func TestHandler_EmitNative_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	irPath := filepath.Join(tmpDir, "invalid.ir.json")
	if err := os.WriteFile(irPath, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{}
	_, err := handler.EmitNative(irPath, outputDir)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}
