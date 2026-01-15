//go:build cgo

package esword

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// TestBibleParser tests the BibleParser methods
func TestBibleParser(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bblx")

	// Create a test Bible database
	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create Bible table
	_, err = db.Exec(`CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		t.Fatal(err)
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
		{40, 1, 1, "The book of the generation of Jesus Christ."},
	}

	for _, v := range testVerses {
		_, err := db.Exec("INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
			v.book, v.chapter, v.verse, v.text)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create Details table
	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)",
		"Test Bible", "TST", "Test Information", "1.0", 0)
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	// Test NewBibleParser
	parser, err := NewBibleParser(testFile)
	if err != nil {
		t.Fatalf("NewBibleParser failed: %v", err)
	}
	defer parser.Close()

	// Test GetMetadata
	metadata := parser.GetMetadata()
	if metadata == nil {
		t.Fatal("GetMetadata returned nil")
	}
	if metadata.Title != "Test Bible" {
		t.Errorf("Expected title 'Test Bible', got %s", metadata.Title)
	}
	if metadata.Abbreviation != "TST" {
		t.Errorf("Expected abbreviation 'TST', got %s", metadata.Abbreviation)
	}

	// Test GetVerse
	verse, err := parser.GetVerse(1, 1, 1)
	if err != nil {
		t.Fatalf("GetVerse failed: %v", err)
	}
	if verse.Book != 1 || verse.Chapter != 1 || verse.Verse != 1 {
		t.Errorf("GetVerse returned wrong verse: %d:%d:%d", verse.Book, verse.Chapter, verse.Verse)
	}
	if verse.Scripture != "In the beginning God created the heaven and the earth." {
		t.Errorf("GetVerse returned wrong text: %s", verse.Scripture)
	}

	// Test GetChapter
	verses, err := parser.GetChapter(1, 1)
	if err != nil {
		t.Fatalf("GetChapter failed: %v", err)
	}
	if len(verses) != 2 {
		t.Errorf("Expected 2 verses in chapter, got %d", len(verses))
	}

	// Test GetBook
	verses, err = parser.GetBook(1)
	if err != nil {
		t.Fatalf("GetBook failed: %v", err)
	}
	if len(verses) != 2 {
		t.Errorf("Expected 2 verses in book 1, got %d", len(verses))
	}

	// Test GetAllVerses
	allVerses, err := parser.GetAllVerses()
	if err != nil {
		t.Fatalf("GetAllVerses failed: %v", err)
	}
	if len(allVerses) != 3 {
		t.Errorf("Expected 3 total verses, got %d", len(allVerses))
	}

	// Test GetChapterCount
	count, err := parser.GetChapterCount(1)
	if err != nil {
		t.Fatalf("GetChapterCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 chapter in book 1, got %d", count)
	}

	// Test GetVerseCount
	count, err = parser.GetVerseCount(1, 1)
	if err != nil {
		t.Fatalf("GetVerseCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 verses in chapter 1:1, got %d", count)
	}
}

// TestCommentaryParser tests the CommentaryParser methods
func TestCommentaryParser(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cmtx")

	// Create a test Commentary database
	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create Commentary table
	_, err = db.Exec(`CREATE TABLE Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert test entries
	_, err = db.Exec("INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)",
		1, 1, 1, 1, 1, "Commentary on Genesis 1:1")
	if err != nil {
		t.Fatal(err)
	}

	// Create Details table
	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER, RightToLeft INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)",
		"Test Commentary", "TCMT", "Test Commentary Info", 1, 0)
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	// Test NewCommentaryParser
	parser, err := NewCommentaryParser(testFile)
	if err != nil {
		t.Fatalf("NewCommentaryParser failed: %v", err)
	}
	defer parser.Close()

	// Test GetEntry
	entry, err := parser.GetEntry(1, 1, 1)
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if entry.Book != 1 || entry.ChapterStart != 1 || entry.VerseStart != 1 {
		t.Errorf("GetEntry returned wrong entry: %d:%d:%d", entry.Book, entry.ChapterStart, entry.VerseStart)
	}
	if entry.Comments != "Commentary on Genesis 1:1" {
		t.Errorf("GetEntry returned wrong comments: %s", entry.Comments)
	}

	// Test GetChapter
	entries := parser.GetChapter(1, 1)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry in chapter, got %d", len(entries))
	}

	// Test GetBook
	entries = parser.GetBook(1)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry in book, got %d", len(entries))
	}

	// Test ListBooks
	books := parser.ListBooks()
	if len(books) != 1 {
		t.Errorf("Expected 1 book, got %d", len(books))
	}

	// Test ModuleInfo
	info := parser.ModuleInfo()
	if info.Title != "Test Commentary" {
		t.Errorf("Expected title 'Test Commentary', got %s", info.Title)
	}
	if info.EntryCount != 1 {
		t.Errorf("Expected 1 entry, got %d", info.EntryCount)
	}

	// Test HasOT
	if !parser.HasOT() {
		t.Error("Expected HasOT to be true")
	}

	// Test HasNT
	if parser.HasNT() {
		t.Error("Expected HasNT to be false")
	}
}

// TestDictionaryParser tests the DictionaryParser methods
func TestDictionaryParser(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dctx")

	// Create a test Dictionary database
	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create Dictionary table
	_, err = db.Exec(`CREATE TABLE Dictionary (Topic TEXT, Definition TEXT)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert test entries
	testEntries := []struct {
		topic      string
		definition string
	}{
		{"Abel", "Second son of Adam"},
		{"Abraham", "Father of many nations"},
		{"Covenant", "An agreement between God and man"},
	}

	for _, e := range testEntries {
		_, err := db.Exec("INSERT INTO Dictionary (Topic, Definition) VALUES (?, ?)",
			e.topic, e.definition)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create Details table
	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO Details (Title, Abbreviation, Information, Version) VALUES (?, ?, ?, ?)",
		"Test Dictionary", "TDICT", "Test Dictionary Info", 1)
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	// Test NewDictionaryParser
	parser, err := NewDictionaryParser(testFile)
	if err != nil {
		t.Fatalf("NewDictionaryParser failed: %v", err)
	}
	defer parser.Close()

	// Test GetEntry
	entry, err := parser.GetEntry("Abel")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if entry.Topic != "Abel" {
		t.Errorf("Expected topic 'Abel', got %s", entry.Topic)
	}
	if entry.Definition != "Second son of Adam" {
		t.Errorf("Expected definition 'Second son of Adam', got %s", entry.Definition)
	}

	// Test ListTopics
	topics := parser.ListTopics()
	if len(topics) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(topics))
	}

	// Test ListTopicsSorted
	sortedTopics := parser.ListTopicsSorted()
	if len(sortedTopics) != 3 {
		t.Errorf("Expected 3 sorted topics, got %d", len(sortedTopics))
	}
	if sortedTopics[0] != "Abel" {
		t.Errorf("Expected first topic 'Abel', got %s", sortedTopics[0])
	}

	// Test SearchTopics
	matches := parser.SearchTopics("Ab")
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches for 'Ab', got %d", len(matches))
	}

	// Test SearchDefinitions
	matches = parser.SearchDefinitions("son")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match for 'son', got %d", len(matches))
	}

	// Test ModuleInfo
	info := parser.ModuleInfo()
	if info.Title != "Test Dictionary" {
		t.Errorf("Expected title 'Test Dictionary', got %s", info.Title)
	}
	if info.EntryCount != 3 {
		t.Errorf("Expected 3 entries, got %d", info.EntryCount)
	}

	// Test EntryCount
	count := parser.EntryCount()
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

// TestHandlerDetect tests the Handler.Detect method
func TestHandlerDetect(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &Handler{}

	tests := []struct {
		name     string
		filename string
		setup    func(string) error
		detected bool
		format   string
	}{
		{
			name:     "Bible file",
			filename: "test.bblx",
			setup: func(path string) error {
				db, err := sqlite.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				_, err = db.Exec("CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
				return err
			},
			detected: true,
			format:   "esword",
		},
		{
			name:     "Commentary file",
			filename: "test.cmtx",
			setup: func(path string) error {
				db, err := sqlite.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				_, err = db.Exec("CREATE TABLE Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)")
				return err
			},
			detected: true,
			format:   "esword",
		},
		{
			name:     "Dictionary file",
			filename: "test.dctx",
			setup: func(path string) error {
				db, err := sqlite.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				_, err = db.Exec("CREATE TABLE Dictionary (Topic TEXT, Definition TEXT)")
				return err
			},
			detected: true,
			format:   "esword",
		},
		{
			name:     "Wrong extension",
			filename: "test.txt",
			setup: func(path string) error {
				return os.WriteFile(path, []byte("test"), 0644)
			},
			detected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.filename)
			if err := tt.setup(testFile); err != nil {
				t.Fatal(err)
			}

			result, err := handler.Detect(testFile)
			if err != nil {
				t.Fatal(err)
			}

			if result.Detected != tt.detected {
				t.Errorf("Expected detected=%v, got %v (reason: %s)", tt.detected, result.Detected, result.Reason)
			}

			if tt.detected && result.Format != tt.format {
				t.Errorf("Expected format %s, got %s", tt.format, result.Format)
			}
		})
	}
}

// TestHandlerIngest tests the Handler.Ingest method
func TestHandlerIngest(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &Handler{}

	testFile := filepath.Join(tmpDir, "test.bblx")
	outputDir := filepath.Join(tmpDir, "output")

	// Create a test Bible database
	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	result, err := handler.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if result.ArtifactID != "test" {
		t.Errorf("Expected artifact ID 'test', got %s", result.ArtifactID)
	}

	if result.BlobSHA256 == "" {
		t.Error("Expected BlobSHA256 to be set")
	}

	if result.SizeBytes == 0 {
		t.Error("Expected SizeBytes to be > 0")
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("Expected blob file to exist")
	}
}

// TestHandlerExtractIR tests the Handler.ExtractIR method
func TestHandlerExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &Handler{}

	tests := []struct {
		name       string
		filename   string
		moduleType ir.ModuleType
		setup      func(string) error
	}{
		{
			name:       "Bible",
			filename:   "test.bblx",
			moduleType: ir.ModuleBible,
			setup: func(path string) error {
				db, err := sqlite.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				_, err = db.Exec("CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
				if err != nil {
					return err
				}
				_, err = db.Exec("INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
					1, 1, 1, "In the beginning God created the heaven and the earth.")
				if err != nil {
					return err
				}
				_, err = db.Exec("CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)")
				if err != nil {
					return err
				}
				_, err = db.Exec("INSERT INTO Details (Title, Abbreviation, Information) VALUES (?, ?, ?)",
					"Test Bible", "TST", "Test Info")
				return err
			},
		},
		{
			name:       "Commentary",
			filename:   "test.cmtx",
			moduleType: ir.ModuleCommentary,
			setup: func(path string) error {
				db, err := sqlite.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				_, err = db.Exec("CREATE TABLE Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)")
				if err != nil {
					return err
				}
				_, err = db.Exec("INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)",
					1, 1, 1, 1, 1, "Commentary on Genesis 1:1")
				if err != nil {
					return err
				}
				_, err = db.Exec("CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER, RightToLeft INTEGER)")
				if err != nil {
					return err
				}
				_, err = db.Exec("INSERT INTO Details (Title) VALUES (?)", "Test Commentary")
				return err
			},
		},
		{
			name:       "Dictionary",
			filename:   "test.dctx",
			moduleType: ir.ModuleDictionary,
			setup: func(path string) error {
				db, err := sqlite.Open(path)
				if err != nil {
					return err
				}
				defer db.Close()
				_, err = db.Exec("CREATE TABLE Dictionary (Topic TEXT, Definition TEXT)")
				if err != nil {
					return err
				}
				_, err = db.Exec("INSERT INTO Dictionary (Topic, Definition) VALUES (?, ?)",
					"Abel", "Second son of Adam")
				if err != nil {
					return err
				}
				_, err = db.Exec("CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER)")
				if err != nil {
					return err
				}
				_, err = db.Exec("INSERT INTO Details (Title) VALUES (?)", "Test Dictionary")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.filename)
			outputDir := filepath.Join(tmpDir, "ir_output_"+tt.name)

			if err := tt.setup(testFile); err != nil {
				t.Fatal(err)
			}

			result, err := handler.ExtractIR(testFile, outputDir)
			if err != nil {
				t.Fatalf("ExtractIR failed: %v", err)
			}

			if result.IRPath == "" {
				t.Error("Expected IRPath to be set")
			}

			if result.LossClass != "L1" {
				t.Errorf("Expected LossClass L1, got %s", result.LossClass)
			}

			// Verify IR file was created
			if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
				t.Error("Expected IR file to exist")
			}

			// Verify IR content
			data, err := os.ReadFile(result.IRPath)
			if err != nil {
				t.Fatal(err)
			}

			var corpus ir.Corpus
			if err := json.Unmarshal(data, &corpus); err != nil {
				t.Fatalf("Failed to parse IR: %v", err)
			}

			if corpus.ModuleType != tt.moduleType {
				t.Errorf("Expected module type %s, got %s", tt.moduleType, corpus.ModuleType)
			}

			if corpus.SourceFormat != "e-Sword" {
				t.Errorf("Expected source format 'e-Sword', got %s", corpus.SourceFormat)
			}
		})
	}
}

// TestHandlerEmitNative tests the Handler.EmitNative method
func TestHandlerEmitNative(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &Handler{}

	tests := []struct {
		name       string
		moduleType ir.ModuleType
		extension  string
		corpus     *ir.Corpus
	}{
		{
			name:       "Bible",
			moduleType: ir.ModuleBible,
			extension:  ".bblx",
			corpus: &ir.Corpus{
				ID:           "test-bible",
				Version:      "1.0.0",
				ModuleType:   ir.ModuleBible,
				SourceFormat: "e-Sword",
				Title:        "Test Bible",
				Documents: []*ir.Document{
					{
						ID:    "Gen",
						Title: "Genesis",
						Order: 1,
						Attributes: map[string]string{
							"book_num": "1",
						},
						ContentBlocks: []*ir.ContentBlock{
							{
								ID:       "cb-1",
								Sequence: 1,
								Text:     "In the beginning God created the heaven and the earth.",
								Anchors: []*ir.Anchor{
									{
										ID:       "a-1-0",
										Position: 0,
										Spans: []*ir.Span{
											{
												ID:            "s-Gen.1.1",
												Type:          ir.SpanVerse,
												StartAnchorID: "a-1-0",
												Ref: &ir.Ref{
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
			},
		},
		{
			name:       "Commentary",
			moduleType: ir.ModuleCommentary,
			extension:  ".cmtx",
			corpus: &ir.Corpus{
				ID:           "test-commentary",
				Version:      "1.0.0",
				ModuleType:   ir.ModuleCommentary,
				SourceFormat: "e-Sword",
				Title:        "Test Commentary",
				Documents: []*ir.Document{
					{
						ID:    "commentary",
						Title: "Commentary",
						Order: 1,
						ContentBlocks: []*ir.ContentBlock{
							{
								ID:       "cb-1",
								Sequence: 1,
								Text:     "Commentary on Genesis 1:1",
								Anchors: []*ir.Anchor{
									{
										ID:       "a-1-0",
										Position: 0,
										Spans: []*ir.Span{
											{
												ID:            "s-Gen.1.1",
												Type:          "COMMENT",
												StartAnchorID: "a-1-0",
												Ref: &ir.Ref{
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
			},
		},
		{
			name:       "Dictionary",
			moduleType: ir.ModuleDictionary,
			extension:  ".dctx",
			corpus: &ir.Corpus{
				ID:           "test-dictionary",
				Version:      "1.0.0",
				ModuleType:   ir.ModuleDictionary,
				SourceFormat: "e-Sword",
				Title:        "Test Dictionary",
				Documents: []*ir.Document{
					{
						ID:    "dictionary",
						Title: "Dictionary",
						Order: 1,
						ContentBlocks: []*ir.ContentBlock{
							{
								ID:       "cb-1",
								Sequence: 1,
								Text:     "Second son of Adam",
								Attributes: map[string]interface{}{
									"topic": "Abel",
									"type":  "dictionary",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			irDir := filepath.Join(tmpDir, "ir_"+tt.name)
			outputDir := filepath.Join(tmpDir, "output_"+tt.name)

			if err := os.MkdirAll(irDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write IR file
			irData, err := json.MarshalIndent(tt.corpus, "", "  ")
			if err != nil {
				t.Fatal(err)
			}

			irPath := filepath.Join(irDir, tt.corpus.ID+".ir.json")
			if err := os.WriteFile(irPath, irData, 0644); err != nil {
				t.Fatal(err)
			}

			// Emit native
			result, err := handler.EmitNative(irPath, outputDir)
			if err != nil {
				t.Fatalf("EmitNative failed: %v", err)
			}

			if result.OutputPath == "" {
				t.Error("Expected OutputPath to be set")
			}

			expectedPath := filepath.Join(outputDir, tt.corpus.ID+tt.extension)
			if result.OutputPath != expectedPath {
				t.Errorf("Expected output path %s, got %s", expectedPath, result.OutputPath)
			}

			if result.Format != "e-Sword" {
				t.Errorf("Expected format 'e-Sword', got %s", result.Format)
			}

			// Verify output file exists
			if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
				t.Error("Expected output file to exist")
			}

			// Verify database structure
			db, err := sqlite.OpenReadOnly(result.OutputPath)
			if err != nil {
				t.Fatalf("Failed to open output database: %v", err)
			}
			defer db.Close()

			// Check Details table
			var title string
			row := db.QueryRow("SELECT Title FROM Details LIMIT 1")
			if err := row.Scan(&title); err != nil {
				t.Fatalf("Failed to query Details: %v", err)
			}
			if title != tt.corpus.Title {
				t.Errorf("Expected title %s, got %s", tt.corpus.Title, title)
			}
		})
	}
}

// TestRoundTrip tests extracting IR and emitting native produces equivalent output
func TestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &Handler{}

	// Create original Bible database
	originalFile := filepath.Join(tmpDir, "original.bblx")
	db, err := sqlite.Open(originalFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	testVerses := []struct {
		book    int
		chapter int
		verse   int
		text    string
	}{
		{1, 1, 1, "In the beginning God created the heaven and the earth."},
		{1, 1, 2, "And the earth was without form, and void."},
		{40, 1, 1, "The book of the generation of Jesus Christ."},
	}

	for _, v := range testVerses {
		_, err := db.Exec("INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)",
			v.book, v.chapter, v.verse, v.text)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec("CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO Details (Title, Abbreviation, Information) VALUES (?, ?, ?)",
		"Test Bible", "TST", "Round-trip test")
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	// Extract IR
	irDir := filepath.Join(tmpDir, "ir")
	extractResult, err := handler.ExtractIR(originalFile, irDir)
	if err != nil {
		t.Fatalf("ExtractIR failed: %v", err)
	}

	// Emit native
	outputDir := filepath.Join(tmpDir, "output")
	emitResult, err := handler.EmitNative(extractResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative failed: %v", err)
	}

	// Compare original and round-trip databases
	roundTripParser, err := NewBibleParser(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("Failed to open round-trip database: %v", err)
	}
	defer roundTripParser.Close()

	roundTripVerses, err := roundTripParser.GetAllVerses()
	if err != nil {
		t.Fatalf("Failed to get verses from round-trip: %v", err)
	}

	if len(roundTripVerses) != len(testVerses) {
		t.Errorf("Expected %d verses, got %d", len(testVerses), len(roundTripVerses))
	}

	// Verify verses match
	for i, expected := range testVerses {
		if i >= len(roundTripVerses) {
			break
		}
		verse := roundTripVerses[i]
		if verse.Book != expected.book || verse.Chapter != expected.chapter || verse.Verse != expected.verse {
			t.Errorf("Verse %d: expected %d:%d:%d, got %d:%d:%d",
				i, expected.book, expected.chapter, expected.verse,
				verse.Book, verse.Chapter, verse.Verse)
		}
		if verse.Scripture != expected.text {
			t.Errorf("Verse %d: expected text %q, got %q", i, expected.text, verse.Scripture)
		}
	}

	// Verify metadata
	metadata := roundTripParser.GetMetadata()
	if metadata.Title != "Test Bible" {
		t.Errorf("Expected title 'Test Bible', got %s", metadata.Title)
	}
	if metadata.Abbreviation != "TST" {
		t.Errorf("Expected abbreviation 'TST', got %s", metadata.Abbreviation)
	}
}

// TestCleanESwordText tests the cleanESwordText function
func TestCleanESwordText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No formatting",
			input:    "Plain text",
			expected: "Plain text",
		},
		{
			name:     "With \\par",
			input:    "Line 1\\parLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "With \\line",
			input:    "Line 1\\lineLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "With bold",
			input:    "Normal \\bBold\\b0 Normal",
			expected: "Normal Bold Normal",
		},
		{
			name:     "With italic",
			input:    "Normal \\iItalic\\i0 Normal",
			expected: "Normal Italic Normal",
		},
		{
			name:     "With font spec \\f0",
			input:    "Text\\f0more text",
			expected: "Textmore text",
		},
		{
			name:     "With font spec \\f1",
			input:    "Text\\f1more text",
			expected: "Textmore text",
		},
		{
			name:     "With color spec \\cf0",
			input:    "Text\\cf0colored",
			expected: "Textcolored",
		},
		{
			name:     "With color spec \\cf1",
			input:    "Text\\cf1colored",
			expected: "Textcolored",
		},
		{
			name:     "Multiple font specs",
			input:    "Text\\f0\\f1more",
			expected: "Textmore",
		},
		{
			name:     "Multiple color specs",
			input:    "Text\\cf0\\cf1colored",
			expected: "Textcolored",
		},
		{
			name:     "With underline",
			input:    "Normal \\ulUnderline\\ul0 Normal",
			expected: "Normal Underline Normal",
		},
		{
			name:     "With superscript",
			input:    "Text\\superSuper",
			expected: "TextSuper",
		},
		{
			name:     "With subscript",
			input:    "Text\\subSub",
			expected: "TextSub",
		},
		{
			name:     "With nosupersub",
			input:    "\\nosupersubText",
			expected: "Text",
		},
		{
			name:     "With font size 20 (after font spec removal)",
			input:    "\\fs20Text",
			expected: "s20Text",
		},
		{
			name:     "With multiple RTF codes",
			input:    "\\b\\i\\ulText\\ul0\\i0\\b0",
			expected: "Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanESwordText(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestBookNumToOSIS tests the bookNumToOSIS function
func TestBookNumToOSIS(t *testing.T) {
	tests := []struct {
		bookNum  int
		expected string
	}{
		{1, "Gen"},
		{2, "Exod"},
		{40, "Matt"},
		{66, "Rev"},
		{999, "Book999"}, // Invalid book number
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := bookNumToOSIS(tt.bookNum)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestOSISToBookNum tests the osisToBookNum function
func TestOSISToBookNum(t *testing.T) {
	tests := []struct {
		osisID   string
		expected int
	}{
		{"Gen", 1},
		{"Exod", 2},
		{"Matt", 40},
		{"Rev", 66},
		{"Invalid", 0}, // Invalid OSIS ID
	}

	for _, tt := range tests {
		t.Run(tt.osisID, func(t *testing.T) {
			result := osisToBookNum(tt.osisID)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestHandlerEnumerate(t *testing.T) {
	tmpDir := t.TempDir()
	h := &Handler{}

	t.Run("success", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.bblx")
		content := []byte("test content for size calculation")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatal(err)
		}

		result, err := h.Enumerate(testFile)
		if err != nil {
			t.Fatalf("Enumerate failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
		}

		entry := result.Entries[0]
		if entry.Path != "test.bblx" {
			t.Errorf("Path = %q, want test.bblx", entry.Path)
		}
		if entry.SizeBytes != int64(len(content)) {
			t.Errorf("SizeBytes = %d, want %d", entry.SizeBytes, len(content))
		}
		if entry.IsDir {
			t.Error("IsDir should be false")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := h.Enumerate(filepath.Join(tmpDir, "nonexistent.bblx"))
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
}

func TestHandlerDetectDirectory(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

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
}

func TestHandlerDetectNonexistent(t *testing.T) {
	h := &Handler{}

	result, err := h.Detect("/nonexistent/path.bblx")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result.Detected {
		t.Error("Expected nonexistent file to not be detected")
	}
}

func TestHandlerIngestErrors(t *testing.T) {
	h := &Handler{}

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := h.Ingest("/nonexistent/path.bblx", t.TempDir())
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("non-writable output", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.bblx")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := h.Ingest(testFile, "/nonexistent/path/output")
		if err == nil {
			t.Error("Expected error for non-writable output")
		}
	})
}

func TestHandlerExtractIRErrors(t *testing.T) {
	h := &Handler{}

	t.Run("unsupported extension", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.xyz")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := h.ExtractIR(testFile, tmpDir)
		if err == nil {
			t.Error("Expected error for unsupported extension")
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

func TestCommentaryEntryMethods(t *testing.T) {
	entry := &CommentaryEntry{
		Book:         1,
		ChapterStart: 1,
		ChapterEnd:   2,
		VerseStart:   1,
		VerseEnd:     10,
		Comments:     "Test commentary",
	}

	// Test IsRange
	if !entry.IsRange() {
		t.Error("Expected IsRange to be true for multi-verse entry")
	}

	// Test IsMultiChapter
	if !entry.IsMultiChapter() {
		t.Error("Expected IsMultiChapter to be true for multi-chapter entry")
	}

	// Test single verse entry
	singleEntry := &CommentaryEntry{
		Book:         1,
		ChapterStart: 1,
		ChapterEnd:   1,
		VerseStart:   1,
		VerseEnd:     1,
		Comments:     "Single verse",
	}

	if singleEntry.IsRange() {
		t.Error("Expected IsRange to be false for single verse entry")
	}

	if singleEntry.IsMultiChapter() {
		t.Error("Expected IsMultiChapter to be false for single chapter entry")
	}
}

func TestIsCommentaryFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.cmtx", true},
		{"test.CMTX", true},
		{"test.bblx", false},
		{"test.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsCommentaryFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsCommentaryFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsDictionaryFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"test.dctx", true},
		{"test.DCTX", true},
		{"test.bblx", false},
		{"test.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsDictionaryFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsDictionaryFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestBookName(t *testing.T) {
	tests := []struct {
		bookNum  int
		expected string
	}{
		{1, "Genesis"},
		{2, "Exodus"},
		{40, "Matthew"},
		{66, "Revelation"},
		{99, "Book 99"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := BookName(tt.bookNum)
			if result != tt.expected {
				t.Errorf("BookName(%d) = %q, want %q", tt.bookNum, result, tt.expected)
			}
		})
	}
}

func TestParseOSISRef(t *testing.T) {
	tests := []struct {
		ref         string
		wantBook    int
		wantChapter int
		wantVerse   int
		wantErr     bool
	}{
		{"Gen.1.1", 1, 1, 1, false},
		{"Matt.5.3", 40, 5, 3, false},
		{"Rev.22.21", 66, 22, 21, false},
		{"invalid", 0, 0, 0, true},
		{"", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			book, chapter, verse, err := parseOSISRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseOSISRef(%q) expected error, got nil", tt.ref)
				}
				return
			}
			if err != nil {
				t.Errorf("parseOSISRef(%q) unexpected error: %v", tt.ref, err)
				return
			}
			if book != tt.wantBook {
				t.Errorf("book = %d, want %d", book, tt.wantBook)
			}
			if chapter != tt.wantChapter {
				t.Errorf("chapter = %d, want %d", chapter, tt.wantChapter)
			}
			if verse != tt.wantVerse {
				t.Errorf("verse = %d, want %d", verse, tt.wantVerse)
			}
		})
	}
}

func TestCleanCommentaryText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Plain text", "Plain text"},
		{"Text with \\par newline", "Text with newline"},
		{"", ""},
		{"{\\rtf1 text}", "text"},
		{"multiple   spaces", "multiple spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanCommentaryText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanCommentaryText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanDictionaryText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Plain text", "Plain text"},
		{"Text with \\par newline", "Text with newline"},
		{"", ""},
		{"{\\rtf1 text}", "text"},
		{"multiple   spaces", "multiple spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanDictionaryText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanDictionaryText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCommentaryGetEntryByRef(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cmtx")

	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec("INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)",
		1, 1, 1, 1, 1, "Commentary on Genesis 1:1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER, RightToLeft INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO Details (Title) VALUES (?)", "Test Commentary")
	if err != nil {
		t.Fatal(err)
	}

	db.Close()

	parser, err := NewCommentaryParser(testFile)
	if err != nil {
		t.Fatalf("NewCommentaryParser failed: %v", err)
	}
	defer parser.Close()

	// Test GetEntryByRef
	entry, err := parser.GetEntryByRef("Gen.1.1")
	if err != nil {
		t.Fatalf("GetEntryByRef failed: %v", err)
	}
	if entry == nil {
		t.Fatal("GetEntryByRef returned nil")
	}
	if entry.Comments != "Commentary on Genesis 1:1" {
		t.Errorf("Comments = %q, want 'Commentary on Genesis 1:1'", entry.Comments)
	}

	// Test GetEntryByRef with invalid ref
	_, err = parser.GetEntryByRef("invalid")
	if err == nil {
		t.Error("Expected error for invalid ref")
	}
}

func TestManifest(t *testing.T) {
	m := Manifest()
	if m.PluginID != "format.esword" {
		t.Errorf("PluginID = %q, want format.esword", m.PluginID)
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

func TestBibleParserClose(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bblx")

	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Details (Description TEXT, Abbreviation TEXT, Comments TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER, OT INTEGER, NT INTEGER, Apocrypha INTEGER, Strong INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO Details (Description, Abbreviation, Comments, Version, RightToLeft, OT, NT, Apocrypha, Strong) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Test Bible", "TST", "Test comments", "1.0", 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	parser, err := NewBibleParser(testFile)
	if err != nil {
		t.Fatal(err)
	}

	// Close twice - second should not error
	err = parser.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}
	err = parser.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

func TestBibleParserGetVerse(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bblx")

	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Details (Description TEXT, Abbreviation TEXT, Comments TEXT, Version TEXT, Font TEXT, RightToLeft INTEGER, OT INTEGER, NT INTEGER, Apocrypha INTEGER, Strong INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO Details (Description, Abbreviation, Comments, Version, RightToLeft, OT, NT, Apocrypha, Strong) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Test Bible", "TST", "Test comments", "1.0", 0, 1, 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Bible (Book INTEGER, Chapter INTEGER, Verse INTEGER, Scripture TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES (?, ?, ?, ?)`, 1, 1, 1, "In the beginning")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	parser, err := NewBibleParser(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()

	// Test existing verse
	verse, err := parser.GetVerse(1, 1, 1)
	if err != nil {
		t.Errorf("GetVerse failed: %v", err)
	}
	if verse.Scripture != "In the beginning" {
		t.Errorf("Verse text = %q, want 'In the beginning'", verse.Scripture)
	}

	// Test non-existing verse
	_, err = parser.GetVerse(99, 1, 1)
	if err == nil {
		t.Error("Expected error for non-existing verse")
	}
}

func TestCommentaryHasOTNT(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.cmtx")

	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER, RightToLeft INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO Details (Title, Abbreviation, Information, Version, RightToLeft) VALUES (?, ?, ?, ?, ?)`, "Test Commentary", "TST", "", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Commentary (Book INTEGER, ChapterBegin INTEGER, ChapterEnd INTEGER, VerseBegin INTEGER, VerseEnd INTEGER, Comments TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	// Add OT book
	_, err = db.Exec(`INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)`, 1, 1, 1, 1, 1, "Genesis commentary")
	if err != nil {
		t.Fatal(err)
	}
	// Add NT book
	_, err = db.Exec(`INSERT INTO Commentary (Book, ChapterBegin, ChapterEnd, VerseBegin, VerseEnd, Comments) VALUES (?, ?, ?, ?, ?, ?)`, 40, 1, 1, 1, 1, "Matthew commentary")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	parser, err := NewCommentaryParser(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()

	if !parser.HasOT() {
		t.Error("HasOT() should return true")
	}
	if !parser.HasNT() {
		t.Error("HasNT() should return true")
	}
}

func TestDictionaryParserGetEntry(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dcti")

	db, err := sqlite.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE Details (Title TEXT, Abbreviation TEXT, Information TEXT, Version INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO Details (Title, Abbreviation, Information, Version) VALUES (?, ?, ?, ?)`, "Test Dictionary", "TST", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Dictionary (Topic TEXT, Definition TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO Dictionary (Topic, Definition) VALUES (?, ?)`, "faith", "Belief and trust in God")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	parser, err := NewDictionaryParser(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer parser.Close()

	entry, err := parser.GetEntry("faith")
	if err != nil {
		t.Errorf("GetEntry failed: %v", err)
	}
	if entry == nil {
		t.Fatal("GetEntry returned nil")
	}
	if entry.Topic != "faith" {
		t.Errorf("Topic = %q, want 'faith'", entry.Topic)
	}

	// Test non-existing entry
	_, err = parser.GetEntry("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existing entry")
	}
}
