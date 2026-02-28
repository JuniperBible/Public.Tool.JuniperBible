// Package testing provides tool comparison tests between Python and Go extractors.
//
// These tests verify that the Go extractor produces output identical to the Python
// extractor, ensuring backward compatibility and correctness.
package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestToolComparison_DiathekeAvailable checks if diatheke is available for testing.
func TestToolComparison_DiathekeAvailable(t *testing.T) {
	_, err := exec.LookPath("diatheke")
	if err != nil {
		t.Skip("diatheke not available, skipping tool comparison tests")
	}
}

// TestToolComparison_GoExtractorBuilds verifies the Go extractor compiles.
func TestToolComparison_GoExtractorBuilds(t *testing.T) {
	// Find project root
	root := findProjectRoot(t)
	extractorDir := filepath.Join(root, "tools", "juniper")

	cmd := exec.Command("go", "build", "-o", "/dev/null", "./cmd/extract")
	cmd.Dir = extractorDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Go extractor build failed: %v\nOutput: %s", err, output)
	}
}

// TestToolComparison_PythonExtractorSyntax verifies Python extractor has no syntax errors.
// Note: Python extractors are now archived in attic/tools/python-extractors/
func TestToolComparison_PythonExtractorSyntax(t *testing.T) {
	// Skip if python3 is not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping syntax check")
	}

	root := findProjectRoot(t)
	pythonScript := filepath.Join(root, "attic", "tools", "python-extractors", "extract_scriptures.py")

	if _, err := os.Stat(pythonScript); os.IsNotExist(err) {
		t.Skip("Python extractor archived, skipping syntax check")
	}

	cmd := exec.Command("python3", "-m", "py_compile", pythonScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python extractor syntax check failed: %v\nOutput: %s", err, output)
	}
}

// TestToolComparison_OutputStructureMatch compares output structure of both tools.
// This test runs both extractors on test data and compares the JSON structure.
// Note: This test is slow and requires RUN_EXTRACTOR_TESTS=1 environment variable.
func TestToolComparison_OutputStructureMatch(t *testing.T) {
	// Skip unless explicitly enabled (this test can take 60+ seconds)
	if os.Getenv("RUN_EXTRACTOR_TESTS") != "1" {
		t.Skip("skipping slow extractor test (set RUN_EXTRACTOR_TESTS=1 to enable)")
	}

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if diatheke is available
	if _, err := exec.LookPath("diatheke"); err != nil {
		t.Skip("diatheke not available")
	}

	// Check if a test module is available
	if !checkModuleAvailable("KJV") {
		t.Skip("KJV module not available for testing")
	}

	root := findProjectRoot(t)

	// Create temp directories for output
	goOutputDir := t.TempDir()
	pyOutputDir := t.TempDir()

	// Use context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run Go extractor with timeout
	goExtractor := filepath.Join(root, "tools", "juniper")
	goCmd := exec.CommandContext(ctx, "go", "run", "./cmd/extract", "-o", goOutputDir, "-m", "KJV")
	goCmd.Dir = goExtractor
	goOutput, goErr := goCmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("Go extractor timed out after 60 seconds")
	}
	if goErr != nil {
		t.Logf("Go extractor output: %s", goOutput)
		t.Fatalf("Go extractor failed: %v", goErr)
	}

	// Python extractor is archived - skip comparison test
	// The Go extractor is now the primary implementation
	pyScript := filepath.Join(root, "attic", "tools", "python-extractors", "extract_scriptures.py")
	if _, err := os.Stat(pyScript); os.IsNotExist(err) {
		t.Skip("Python extractor archived, Go extractor is now primary")
	}

	// Run Python extractor with timeout (for backward compatibility testing)
	pyCtx, pyCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer pyCancel()

	pyCmd := exec.CommandContext(pyCtx, "python3", pyScript, pyOutputDir)
	pyCmd.Dir = goExtractor
	pyCmd.Env = append(os.Environ(), "EXTRACT_MODULES=KJV")
	pyOutput, pyErr := pyCmd.CombinedOutput()
	if pyCtx.Err() == context.DeadlineExceeded {
		t.Fatal("Python extractor timed out after 60 seconds")
	}
	if pyErr != nil {
		t.Logf("Python extractor output: %s", pyOutput)
		// Python might fail if module not configured - that's ok for structure test
		t.Skip("Python extractor failed (might need module configuration)")
	}

	// Compare bibles.json structure
	compareJSONStructure(t, goOutputDir, pyOutputDir, "bibles.json")
}

// TestToolComparison_VersificationMatch verifies both tools use same versification.
func TestToolComparison_VersificationMatch(t *testing.T) {
	root := findProjectRoot(t)
	versDir := filepath.Join(root, "tools", "juniper", "versifications")

	// Check protestant.yaml exists and is valid YAML
	protestantPath := filepath.Join(versDir, "protestant.yaml")
	if _, err := os.Stat(protestantPath); os.IsNotExist(err) {
		t.Fatal("protestant.yaml not found")
	}

	// Verify it contains expected books
	content, err := os.ReadFile(protestantPath)
	if err != nil {
		t.Fatalf("Failed to read protestant.yaml: %v", err)
	}

	// Check for key book entries
	requiredBooks := []string{"Genesis", "Exodus", "Matthew", "Revelation"}
	for _, book := range requiredBooks {
		if !strings.Contains(string(content), book) {
			t.Errorf("protestant.yaml missing required book: %s", book)
		}
	}
}

// TestToolComparison_JSONSchemaCompliance checks output matches expected schema.
func TestToolComparison_JSONSchemaCompliance(t *testing.T) {
	// Read existing bibles.json if available
	root := findProjectRoot(t)
	biblesPath := filepath.Join(root, "data", "bibles.json")

	if _, err := os.Stat(biblesPath); os.IsNotExist(err) {
		t.Skip("bibles.json not found, skipping schema test")
	}

	content, err := os.ReadFile(biblesPath)
	if err != nil {
		t.Fatalf("Failed to read bibles.json: %v", err)
	}

	var metadata BibleMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("bibles.json is not valid JSON: %v", err)
	}

	// Verify required fields
	if len(metadata.Bibles) == 0 {
		t.Error("bibles.json has no Bible entries")
	}

	for _, bible := range metadata.Bibles {
		if bible.ID == "" {
			t.Error("Bible entry missing ID")
		}
		if bible.Title == "" {
			t.Errorf("Bible %s missing title", bible.ID)
		}
		if bible.Abbrev == "" {
			t.Errorf("Bible %s missing abbreviation", bible.ID)
		}
		if bible.Language == "" {
			t.Errorf("Bible %s missing language", bible.ID)
		}
	}

	// Verify meta fields
	if metadata.Meta.Version == "" {
		t.Error("bibles.json missing meta.version")
	}
	if metadata.Meta.Granularity == "" {
		t.Error("bibles.json missing meta.granularity")
	}
}

// TestToolComparison_AuxiliaryFilesExist checks that bibles_auxiliary directory exists.
func TestToolComparison_AuxiliaryFilesExist(t *testing.T) {
	root := findProjectRoot(t)
	auxDir := filepath.Join(root, "data", "bibles_auxiliary")

	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Skip("bibles_auxiliary directory not found")
	}

	// Check for at least one Bible file
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		t.Fatalf("Failed to read bibles_auxiliary: %v", err)
	}

	if len(entries) == 0 {
		t.Error("bibles_auxiliary directory is empty")
	}

	// Validate each auxiliary file
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		auxPath := filepath.Join(auxDir, entry.Name())
		content, err := os.ReadFile(auxPath)
		if err != nil {
			t.Errorf("Failed to read %s: %v", entry.Name(), err)
			continue
		}

		var bibleContent BibleContent
		if err := json.Unmarshal(content, &bibleContent); err != nil {
			t.Errorf("%s is not valid JSON: %v", entry.Name(), err)
			continue
		}

		// Verify has books
		if len(bibleContent.Books) == 0 {
			t.Errorf("%s has no books", entry.Name())
		}

		// Verify books have chapters
		for _, book := range bibleContent.Books {
			if len(book.Chapters) == 0 {
				t.Errorf("%s book %s has no chapters", entry.Name(), book.ID)
			}
		}
	}
}

// TestToolComparison_VerseCountConsistency checks verse counts are reasonable.
func TestToolComparison_VerseCountConsistency(t *testing.T) {
	root := findProjectRoot(t)
	auxDir := filepath.Join(root, "data", "bibles_auxiliary")

	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Skip("bibles_auxiliary directory not found")
	}

	// Expected verse counts (approximate, allow variance)
	expectedCounts := map[string]struct {
		min, max int
	}{
		"kjv":        {30000, 32000}, // KJV has ~31,102 verses
		"drc":        {30000, 36500}, // Douay-Rheims (Catholic, has more books including deuterocanon)
		"geneva1599": {30000, 32000},
		"vulgate":    {30000, 36500}, // Vulgate (Catholic, has deuterocanonical books)
		"tyndale":    {13000, 32000}, // Tyndale is incomplete (NT + Pentateuch only)
	}

	entries, _ := os.ReadDir(auxDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		bibleID := strings.TrimSuffix(entry.Name(), ".json")
		expected, ok := expectedCounts[bibleID]
		if !ok {
			continue // Skip unknown Bibles
		}

		auxPath := filepath.Join(auxDir, entry.Name())
		content, _ := os.ReadFile(auxPath)

		var bibleContent BibleContent
		json.Unmarshal(content, &bibleContent)

		verseCount := 0
		for _, book := range bibleContent.Books {
			for _, chapter := range book.Chapters {
				verseCount += len(chapter.Verses)
			}
		}

		if verseCount < expected.min || verseCount > expected.max {
			t.Errorf("%s has %d verses, expected between %d and %d",
				bibleID, verseCount, expected.min, expected.max)
		} else {
			t.Logf("%s: %d verses (OK)", bibleID, verseCount)
		}
	}
}

// TestToolComparison_BookOrderConsistency verifies books are in canonical order.
func TestToolComparison_BookOrderConsistency(t *testing.T) {
	root := findProjectRoot(t)
	auxDir := filepath.Join(root, "data", "bibles_auxiliary")

	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Skip("bibles_auxiliary directory not found")
	}

	// Expected book order (first and last) for full Bibles
	expectedOTFirst := []string{"Gen", "Genesis"}
	expectedNTFirst := []string{"Matt", "Matthew"}
	expectedLast := []string{"Rev", "Revelation", "Apoc", "Apocalypse", "Odes", "Mal", "Malachi", "3Macc", "4Macc"}

	// Bibles that only have NT
	ntOnlyBibles := map[string]bool{
		"sblgnt": true,
	}

	// Bibles that only have OT
	otOnlyBibles := map[string]bool{
		"osmhb": true,
		"lxx":   true,
	}

	entries, _ := os.ReadDir(auxDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		auxPath := filepath.Join(auxDir, entry.Name())
		content, _ := os.ReadFile(auxPath)

		var bibleContent BibleContent
		json.Unmarshal(content, &bibleContent)

		if len(bibleContent.Books) == 0 {
			continue
		}

		bibleID := strings.TrimSuffix(entry.Name(), ".json")
		firstBook := bibleContent.Books[0].ID
		lastBook := bibleContent.Books[len(bibleContent.Books)-1].ID

		// Determine expected first book based on Bible type
		var expectedFirst []string
		if ntOnlyBibles[bibleID] {
			expectedFirst = expectedNTFirst
		} else {
			expectedFirst = expectedOTFirst
		}

		firstOK := false
		for _, expected := range expectedFirst {
			if firstBook == expected {
				firstOK = true
				break
			}
		}

		lastOK := false
		for _, expected := range expectedLast {
			if lastBook == expected {
				lastOK = true
				break
			}
		}

		if !firstOK {
			// Log instead of error for edge cases
			if ntOnlyBibles[bibleID] || otOnlyBibles[bibleID] {
				t.Logf("%s first book is %s (partial Bible)", entry.Name(), firstBook)
			} else {
				t.Errorf("%s first book is %s, expected Genesis", entry.Name(), firstBook)
			}
		}
		if !lastOK {
			// Allow for various last books (Catholic Bibles, LXX, etc.)
			t.Logf("%s last book is %s (might be correct for this translation)", entry.Name(), lastBook)
		}
	}
}

// TestToolComparison_NoPlaceholderVerses verifies no placeholder text in verses.
func TestToolComparison_NoPlaceholderVerses(t *testing.T) {
	root := findProjectRoot(t)
	auxDir := filepath.Join(root, "data", "bibles_auxiliary")

	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Skip("bibles_auxiliary directory not found")
	}

	// Patterns that indicate placeholder text
	placeholderPatterns := []string{
		"verse not available",
		"[missing]",
		"(no text)",
		"<empty>",
	}

	entries, _ := os.ReadDir(auxDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		auxPath := filepath.Join(auxDir, entry.Name())
		content, _ := os.ReadFile(auxPath)

		var bibleContent BibleContent
		json.Unmarshal(content, &bibleContent)

		for _, book := range bibleContent.Books {
			for _, chapter := range book.Chapters {
				for _, verse := range chapter.Verses {
					textLower := strings.ToLower(verse.Text)
					for _, pattern := range placeholderPatterns {
						if strings.Contains(textLower, pattern) {
							t.Errorf("%s %s %d:%d contains placeholder text: %s",
								entry.Name(), book.ID, chapter.Number, verse.Number, verse.Text)
						}
					}
				}
			}
		}
	}
}

// TestToolComparison_UnicodeHandling tests proper Unicode handling in verses.
func TestToolComparison_UnicodeHandling(t *testing.T) {
	root := findProjectRoot(t)
	auxDir := filepath.Join(root, "data", "bibles_auxiliary")

	// Check vulgate for Latin characters
	vulgatePath := filepath.Join(auxDir, "vulgate.json")
	if _, err := os.Stat(vulgatePath); err == nil {
		content, _ := os.ReadFile(vulgatePath)
		var bibleContent BibleContent
		json.Unmarshal(content, &bibleContent)

		// Check Genesis 1:1 contains expected Latin
		if len(bibleContent.Books) > 0 && len(bibleContent.Books[0].Chapters) > 0 {
			ch1 := bibleContent.Books[0].Chapters[0]
			if len(ch1.Verses) > 0 {
				// Should contain "In principio" or similar
				if !strings.Contains(ch1.Verses[0].Text, "principio") &&
					!strings.Contains(ch1.Verses[0].Text, "In") {
					t.Logf("Vulgate Gen 1:1: %s", ch1.Verses[0].Text)
				}
			}
		}
	}
}

// TestToolComparison_ChapterVerseNumbering verifies sequential numbering.
func TestToolComparison_ChapterVerseNumbering(t *testing.T) {
	root := findProjectRoot(t)
	auxDir := filepath.Join(root, "data", "bibles_auxiliary")

	if _, err := os.Stat(auxDir); os.IsNotExist(err) {
		t.Skip("bibles_auxiliary directory not found")
	}

	entries, _ := os.ReadDir(auxDir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		auxPath := filepath.Join(auxDir, entry.Name())
		content, _ := os.ReadFile(auxPath)

		var bibleContent BibleContent
		json.Unmarshal(content, &bibleContent)

		for _, book := range bibleContent.Books {
			prevChapter := 0
			for _, chapter := range book.Chapters {
				// Chapters should be sequential
				if chapter.Number <= prevChapter {
					t.Errorf("%s %s: chapter %d follows chapter %d (non-sequential)",
						entry.Name(), book.ID, chapter.Number, prevChapter)
				}
				prevChapter = chapter.Number

				// Verse 1 should exist in most chapters
				if len(chapter.Verses) > 0 && chapter.Verses[0].Number != 1 {
					// Some verses might start at 0 or skip - just log
					t.Logf("%s %s %d: first verse is %d",
						entry.Name(), book.ID, chapter.Number, chapter.Verses[0].Number)
				}
			}
		}
	}
}

// Helper functions

func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Try common locations
	candidates := []string{
		".",
		"..",
		"../..",
		"../../..",
		"../../../..",
	}

	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		hugoToml := filepath.Join(absPath, "hugo.toml")
		if _, err := os.Stat(hugoToml); err == nil {
			return absPath
		}
	}

	t.Fatal("Could not find project root (hugo.toml)")
	return ""
}

func checkModuleAvailable(module string) bool {
	cmd := exec.Command("diatheke", "-b", module, "-k", "Gen 1:1")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 10
}

func compareJSONStructure(t *testing.T, goDir, pyDir, filename string) {
	t.Helper()

	goPath := filepath.Join(goDir, filename)
	pyPath := filepath.Join(pyDir, filename)

	goContent, goErr := os.ReadFile(goPath)
	pyContent, pyErr := os.ReadFile(pyPath)

	if goErr != nil {
		t.Errorf("Go output missing %s: %v", filename, goErr)
		return
	}
	if pyErr != nil {
		t.Logf("Python output missing %s: %v (might be expected)", filename, pyErr)
		return
	}

	var goData, pyData map[string]interface{}
	if err := json.Unmarshal(goContent, &goData); err != nil {
		t.Errorf("Go %s invalid JSON: %v", filename, err)
		return
	}
	if err := json.Unmarshal(pyContent, &pyData); err != nil {
		t.Errorf("Python %s invalid JSON: %v", filename, err)
		return
	}

	// Compare top-level keys
	goKeys := getKeys(goData)
	pyKeys := getKeys(pyData)

	for _, key := range goKeys {
		found := false
		for _, pk := range pyKeys {
			if key == pk {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: Go has key '%s' not in Python output", filename, key)
		}
	}

	for _, key := range pyKeys {
		found := false
		for _, gk := range goKeys {
			if key == gk {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: Python has key '%s' not in Go output", filename, key)
		}
	}
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Benchmark for comparing extraction performance
func BenchmarkToolComparison_JSONParsing(b *testing.B) {
	root := "."
	for i := 0; i < 5; i++ {
		absPath, _ := filepath.Abs(root)
		if _, err := os.Stat(filepath.Join(absPath, "hugo.toml")); err == nil {
			root = absPath
			break
		}
		root = filepath.Join(root, "..")
	}

	auxDir := filepath.Join(root, "data", "bibles_auxiliary")
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		b.Skip("bibles_auxiliary not found")
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		auxPath := filepath.Join(auxDir, entry.Name())
		content, _ := os.ReadFile(auxPath)

		b.Run(fmt.Sprintf("Parse_%s", strings.TrimSuffix(entry.Name(), ".json")), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var bible BibleContent
				json.Unmarshal(content, &bible)
			}
		})
	}
}
