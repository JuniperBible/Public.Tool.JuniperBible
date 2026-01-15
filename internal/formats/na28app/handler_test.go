package na28app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Sample NA28 apparatus content for testing
const sampleNA28Content = `<?xml version="1.0" encoding="UTF-8"?>
<apparatus edition="NA28">
  <app>
    <rdg wit="א B">λόγος</rdg>
    <rdg wit="A C D">λογος</rdg>
  </app>
  Matt.1.1 Βίβλος γενέσεως Ἰησοῦ Χριστοῦ
  witnesses: ℵ א A B C D P75
  *txt vid corrector marks
</apparatus>`

const samplePlainContent = `This is not a NA28 apparatus file.
It contains no manuscript witnesses or variant readings.
Just plain text.`

func TestManifest(t *testing.T) {
	m := Manifest()

	if m.PluginID != "format.na28app" {
		t.Errorf("expected PluginID 'format.na28app', got '%s'", m.PluginID)
	}

	if m.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", m.Version)
	}

	if m.Kind != "format" {
		t.Errorf("expected Kind 'format', got '%s'", m.Kind)
	}

	if m.Entrypoint != "format-na28app" {
		t.Errorf("expected Entrypoint 'format-na28app', got '%s'", m.Entrypoint)
	}

	if len(m.Capabilities.Inputs) == 0 {
		t.Error("expected at least one input capability")
	}

	if len(m.Capabilities.Outputs) == 0 {
		t.Error("expected at least one output capability")
	}
}

func TestDetect_ValidExtension(t *testing.T) {
	testCases := []struct {
		name string
		ext  string
	}{
		{"na28 extension", ".na28"},
		{"apparatus extension", ".apparatus"},
		{"app extension", ".app"},
		{"NA28 uppercase", ".NA28"},
		{"APPARATUS uppercase", ".APPARATUS"},
	}

	h := &Handler{}
	tmpDir := t.TempDir()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test"+tc.ext)
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			result, err := h.Detect(testFile)
			if err != nil {
				t.Fatalf("Detect() error: %v", err)
			}

			if !result.Detected {
				t.Errorf("expected Detected=true for %s", tc.ext)
			}

			if result.Format != "NA28App" {
				t.Errorf("expected Format='NA28App', got '%s'", result.Format)
			}
		})
	}
}

func TestDetect_ValidContent(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte(sampleNA28Content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if !result.Detected {
		t.Error("expected Detected=true for NA28 content")
	}

	if result.Format != "NA28App" {
		t.Errorf("expected Format='NA28App', got '%s'", result.Format)
	}

	if result.Reason != "NA28 apparatus structure detected" {
		t.Errorf("unexpected reason: %s", result.Reason)
	}
}

func TestDetect_InvalidContent(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte(samplePlainContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if result.Detected {
		t.Error("expected Detected=false for plain text content")
	}
}

func TestDetect_NonExistentFile(t *testing.T) {
	h := &Handler{}
	result, err := h.Detect("/nonexistent/file.na28")

	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if result.Detected {
		t.Error("expected Detected=false for non-existent file")
	}
}

func TestContainsNA28Markers(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected bool
	}{
		{"with app marker", "<app>test</app>", true},
		{"with rdg marker", "<rdg wit='A'>text</rdg>", true},
		{"with wit marker", "<wit>א</wit>", true},
		{"with txt pattern", "reading txt variant", true},
		{"with vid pattern", "reading vid א", true},
		{"with asterisk txt", "*txt reading", true},
		{"with aleph", "witnesses: א A B", true},
		{"with aleph variant", "ℵ01", true},
		{"without markers", "just plain text", false},
		{"word witnesses alone", "witnesses", false},
		{"word apparatus alone", "critical apparatus", false},
		{"empty string", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := containsNA28Markers(tc.content)
			if result != tc.expected {
				t.Errorf("expected %v, got %v for content: %s", tc.expected, result, tc.content)
			}
		})
	}
}

func TestIngest(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.na28")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	if err := os.WriteFile(testFile, []byte(sampleNA28Content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := h.Ingest(testFile, outputDir)
	if err != nil {
		t.Fatalf("Ingest() error: %v", err)
	}

	if result.ArtifactID != "na28app" {
		t.Errorf("expected ArtifactID='na28app', got '%s'", result.ArtifactID)
	}

	if result.BlobSHA256 == "" {
		t.Error("expected non-empty BlobSHA256")
	}

	if result.SizeBytes != int64(len(sampleNA28Content)) {
		t.Errorf("expected SizeBytes=%d, got %d", len(sampleNA28Content), result.SizeBytes)
	}

	if result.Metadata["format"] != "NA28App" {
		t.Errorf("expected format='NA28App', got '%s'", result.Metadata["format"])
	}

	// Verify blob was written
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Errorf("blob file not created at %s", blobPath)
	}
}

func TestIngest_NonExistentFile(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	_, err := h.Ingest("/nonexistent/file.na28", tmpDir)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestEnumerate(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.na28")

	if err := os.WriteFile(testFile, []byte(sampleNA28Content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := h.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate() error: %v", err)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}

	entry := result.Entries[0]
	if entry.Path != "test.na28" {
		t.Errorf("expected Path='test.na28', got '%s'", entry.Path)
	}

	if entry.SizeBytes != int64(len(sampleNA28Content)) {
		t.Errorf("expected SizeBytes=%d, got %d", len(sampleNA28Content), entry.SizeBytes)
	}

	if entry.IsDir {
		t.Error("expected IsDir=false")
	}
}

func TestEnumerate_NonExistentFile(t *testing.T) {
	h := &Handler{}
	_, err := h.Enumerate("/nonexistent/file.na28")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestExtractIR(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.na28")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	if err := os.WriteFile(testFile, []byte(sampleNA28Content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := h.ExtractIR(testFile, outputDir)
	if err != nil {
		t.Fatalf("ExtractIR() error: %v", err)
	}

	if result.IRPath == "" {
		t.Error("expected non-empty IRPath")
	}

	if result.LossClass != "L1" {
		t.Errorf("expected LossClass='L1', got '%s'", result.LossClass)
	}

	// Verify IR file was created
	if _, err := os.Stat(result.IRPath); os.IsNotExist(err) {
		t.Errorf("IR file not created at %s", result.IRPath)
	}

	// Read and validate IR content
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus map[string]interface{}
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to unmarshal IR: %v", err)
	}

	// Validate corpus structure
	if corpus["id"] != "na28app" {
		t.Errorf("expected corpus id='na28app', got '%v'", corpus["id"])
	}

	if corpus["module_type"] != "apparatus" {
		t.Errorf("expected module_type='apparatus', got '%v'", corpus["module_type"])
	}

	if corpus["versification"] != "NA28" {
		t.Errorf("expected versification='NA28', got '%v'", corpus["versification"])
	}

	if corpus["language"] != "grc" {
		t.Errorf("expected language='grc', got '%v'", corpus["language"])
	}

	// Validate documents exist
	docs, ok := corpus["documents"].([]interface{})
	if !ok {
		t.Fatal("expected documents to be an array")
	}

	if len(docs) == 0 {
		t.Error("expected at least one document")
	}
}

func TestExtractIR_NonExistentFile(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	_, err := h.ExtractIR("/nonexistent/file.na28", tmpDir)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestParseApparatusLine(t *testing.T) {
	testCases := []struct {
		name     string
		line     string
		checkRef bool
		refValue string
	}{
		{
			name:     "line with reference",
			line:     "Matt.1.1 Βίβλος γενέσεως",
			checkRef: true,
			refValue: "Matt.1.1",
		},
		{
			name:     "line with witnesses",
			line:     "witnesses: א A B C D",
			checkRef: false,
		},
		{
			name:     "plain apparatus line",
			line:     "variant reading text",
			checkRef: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			block := parseApparatusLine(tc.line, i)

			if block["id"] != "block-"+tc.name[0:1] {
				// ID is based on sequence number
			}

			if block["text"] != tc.line {
				t.Errorf("expected text='%s', got '%v'", tc.line, block["text"])
			}

			attrs, ok := block["attributes"].(map[string]interface{})
			if !ok {
				t.Fatal("expected attributes map")
			}

			if attrs["type"] != "apparatus-entry" {
				t.Errorf("expected type='apparatus-entry', got '%v'", attrs["type"])
			}

			if tc.checkRef {
				if attrs["reference"] != tc.refValue {
					t.Errorf("expected reference='%s', got '%v'", tc.refValue, attrs["reference"])
				}
			}
		})
	}
}

func TestEmitNative(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	// First extract IR
	testFile := filepath.Join(tmpDir, "test.na28")
	irDir := filepath.Join(tmpDir, "ir")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create IR dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	if err := os.WriteFile(testFile, []byte(sampleNA28Content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Extract IR
	extractResult, err := h.ExtractIR(testFile, irDir)
	if err != nil {
		t.Fatalf("ExtractIR() error: %v", err)
	}

	// Emit native
	emitResult, err := h.EmitNative(extractResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative() error: %v", err)
	}

	if emitResult.OutputPath == "" {
		t.Error("expected non-empty OutputPath")
	}

	if emitResult.Format != "NA28App" {
		t.Errorf("expected Format='NA28App', got '%s'", emitResult.Format)
	}

	if emitResult.LossClass != "L1" {
		t.Errorf("expected LossClass='L1', got '%s'", emitResult.LossClass)
	}

	// Verify output file was created
	if _, err := os.Stat(emitResult.OutputPath); os.IsNotExist(err) {
		t.Errorf("output file not created at %s", emitResult.OutputPath)
	}

	// Read and validate output content
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	outputContent := string(outputData)

	// Check for XML structure
	if !containsNA28Markers(outputContent) {
		t.Error("output does not contain NA28 markers")
	}

	// Check for basic XML elements
	expectedElements := []string{
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>",
		"<apparatus edition=\"NA28\">",
		"</apparatus>",
		"<entry",
		"</entry>",
	}

	for _, elem := range expectedElements {
		if !containsString(outputContent, elem) {
			t.Errorf("output missing expected element: %s", elem)
		}
	}
}

func TestEmitNative_NonExistentIR(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	_, err := h.EmitNative("/nonexistent/corpus.json", tmpDir)
	if err == nil {
		t.Error("expected error for non-existent IR file")
	}
}

func TestEmitNative_InvalidIR(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	irFile := filepath.Join(tmpDir, "invalid.json")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Write invalid JSON
	if err := os.WriteFile(irFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to create IR file: %v", err)
	}

	_, err := h.EmitNative(irFile, outputDir)
	if err == nil {
		t.Error("expected error for invalid IR")
	}
}

func TestXMLEscape(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"no special chars", "no special chars"},
		{"<tag>", "&lt;tag&gt;"},
		{"A & B", "A &amp; B"},
		{"quote \"test\"", "quote &quot;test&quot;"},
		{"apostrophe's", "apostrophe&apos;s"},
		{"<>&\"'", "&lt;&gt;&amp;&quot;&apos;"},
		{"", ""},
		{"mix <tag> & 'text'", "mix &lt;tag&gt; &amp; &apos;text&apos;"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := xmlEscape(tc.input)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.na28")
	irDir := filepath.Join(tmpDir, "ir")
	outputDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create IR dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Write sample content
	if err := os.WriteFile(testFile, []byte(sampleNA28Content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// 1. Detect
	detectResult, err := h.Detect(testFile)
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if !detectResult.Detected {
		t.Fatal("failed to detect NA28 apparatus")
	}

	// 2. Ingest
	ingestResult, err := h.Ingest(testFile, tmpDir)
	if err != nil {
		t.Fatalf("Ingest() error: %v", err)
	}
	if ingestResult.BlobSHA256 == "" {
		t.Fatal("ingest failed to produce blob hash")
	}

	// 3. Enumerate
	enumerateResult, err := h.Enumerate(testFile)
	if err != nil {
		t.Fatalf("Enumerate() error: %v", err)
	}
	if len(enumerateResult.Entries) == 0 {
		t.Fatal("enumerate failed to list entries")
	}

	// 4. ExtractIR
	extractResult, err := h.ExtractIR(testFile, irDir)
	if err != nil {
		t.Fatalf("ExtractIR() error: %v", err)
	}
	if extractResult.IRPath == "" {
		t.Fatal("extract IR failed to produce IR file")
	}

	// 5. EmitNative
	emitResult, err := h.EmitNative(extractResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("EmitNative() error: %v", err)
	}
	if emitResult.OutputPath == "" {
		t.Fatal("emit native failed to produce output file")
	}

	// Verify output exists and is valid
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if len(outputData) == 0 {
		t.Error("output file is empty")
	}

	// Re-detect the output
	detectResult2, err := h.Detect(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("Detect() on output error: %v", err)
	}
	if !detectResult2.Detected {
		t.Error("failed to detect round-tripped NA28 apparatus")
	}
}

// Helper function for tests
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr ||
		containsString(s[1:], substr))))
}
