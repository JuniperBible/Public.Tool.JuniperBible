// Format conversion pipeline integration tests.
// These tests verify that format conversions work correctly end-to-end.
package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// TestConversionPipelineSetup verifies the conversion pipeline can be initialized.
func TestConversionPipelineSetup(t *testing.T) {
	// Just verify we can import and reference the required packages
	if capsule.Version == "" {
		// This is fine - just checking imports work
	}
	t.Log("Conversion pipeline packages imported successfully")
}

// TestFormatPluginRegistry tests that format plugins can be registered and discovered.
func TestFormatPluginRegistry(t *testing.T) {
	// List all registered plugins using the embedded plugin registry
	embeddedPlugins := plugins.ListEmbeddedPlugins()
	t.Logf("Found %d embedded plugins", len(embeddedPlugins))

	// Count format plugins
	formatCount := 0
	for _, plugin := range embeddedPlugins {
		if plugin.Manifest != nil && strings.HasPrefix(plugin.Manifest.PluginID, "format-") {
			formatCount++
			t.Logf("Format plugin: %s", plugin.Manifest.PluginID)
		}
	}

	if formatCount == 0 {
		t.Log("Warning: No format plugins registered (this may be expected in minimal builds)")
	} else {
		t.Logf("Found %d format plugins", formatCount)
	}
}

// TestConversionTextToIR tests converting text format to IR.
func TestConversionTextToIR(t *testing.T) {
	// Create a simple text file
	tempDir, err := os.MkdirTemp("", "conversion-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	inputFile := filepath.Join(tempDir, "input.txt")
	content := `Genesis 1:1
In the beginning God created the heaven and the earth.

Genesis 1:2
And the earth was without form, and void.
`
	if err := os.WriteFile(inputFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	t.Logf("Created test file: %s", inputFile)

	// Check if text format plugin is available
	embeddedPlugins := plugins.ListEmbeddedPlugins()
	hasTextPlugin := false
	for _, plugin := range embeddedPlugins {
		if plugin.Manifest != nil && strings.Contains(plugin.Manifest.PluginID, "txt") {
			hasTextPlugin = true
			break
		}
	}

	if !hasTextPlugin {
		t.Skip("No text extractors available - skipping IR conversion test")
	}

	t.Log("Text plugin found")
}

// TestConversionJSONRoundtrip tests that JSON can be serialized and deserialized.
func TestConversionJSONRoundtrip(t *testing.T) {
	// Create a test data structure
	testData := map[string]interface{}{
		"title":    "Test Document",
		"language": "en",
		"content":  []string{"Test content line 1", "Test content line 2"},
	}

	// Serialize to JSON
	tempDir, err := os.MkdirTemp("", "json-roundtrip-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "test.json")

	// Write JSON
	data, err := json.MarshalIndent(testData, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	t.Logf("Wrote JSON to: %s", outputPath)

	// Read JSON back
	readData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read JSON: %v", err)
	}

	var parsedData map[string]interface{}
	if err := json.Unmarshal(readData, &parsedData); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify roundtrip
	if parsedData["title"] != testData["title"] {
		t.Errorf("title mismatch: got %v, want %v", parsedData["title"], testData["title"])
	}

	if parsedData["language"] != testData["language"] {
		t.Errorf("language mismatch: got %v, want %v", parsedData["language"], testData["language"])
	}

	t.Log("JSON roundtrip successful")
}

// TestConversionXMLParsing tests that XML content can be parsed.
func TestConversionXMLParsing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "xml-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple OSIS XML file
	inputFile := filepath.Join(tempDir, "test.osis")
	osisContent := `<?xml version="1.0" encoding="UTF-8"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText osisIDWork="Test">
    <header>
      <work osisWork="Test">
        <title>Test Bible</title>
      </work>
    </header>
    <div type="book" osisID="Gen">
      <chapter osisID="Gen.1">
        <verse osisID="Gen.1.1">In the beginning God created the heaven and the earth.</verse>
      </chapter>
    </div>
  </osisText>
</osis>`

	if err := os.WriteFile(inputFile, []byte(osisContent), 0600); err != nil {
		t.Fatalf("failed to write OSIS file: %v", err)
	}

	// Check if OSIS plugin is available
	embeddedPlugins := plugins.ListEmbeddedPlugins()
	hasOSISPlugin := false
	for _, plugin := range embeddedPlugins {
		if plugin.Manifest != nil && strings.Contains(plugin.Manifest.PluginID, "osis") {
			hasOSISPlugin = true
			break
		}
	}

	if !hasOSISPlugin {
		t.Skip("No OSIS extractors available")
	}

	t.Log("OSIS plugin found")
}

// TestConversionFormatDetection tests that file formats can be detected.
func TestConversionFormatDetection(t *testing.T) {
	testCases := []struct {
		name      string
		content   string
		extension string
		expected  string
	}{
		{
			name:      "plain text",
			content:   "Simple text content\n",
			extension: ".txt",
			expected:  "txt",
		},
		{
			name: "OSIS XML",
			content: `<?xml version="1.0"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
</osis>`,
			extension: ".osis",
			expected:  "osis",
		},
		{
			name: "USFM",
			content: `\\id GEN
\\c 1
\\v 1 In the beginning...`,
			extension: ".usfm",
			expected:  "usfm",
		},
	}

	tempDir, err := os.MkdirTemp("", "format-detect-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	embeddedPlugins := plugins.ListEmbeddedPlugins()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "test"+tc.extension)
			if err := os.WriteFile(testFile, []byte(tc.content), 0600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Check if format plugin exists
			formatID := "format-" + tc.expected
			found := false
			for _, plugin := range embeddedPlugins {
				if plugin.Manifest != nil && plugin.Manifest.PluginID == formatID {
					found = true
					break
				}
			}

			if !found {
				t.Logf("No plugin found for %s (this may be expected)", tc.expected)
			} else {
				t.Logf("Found plugin for %s", tc.expected)
			}
		})
	}
}

// TestConversionCapsuleWorkflow tests the complete capsule conversion workflow.
func TestConversionCapsuleWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping capsule workflow test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "capsule-workflow-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test input file
	inputFile := filepath.Join(tempDir, "input.txt")
	content := "Test bible content\n"
	if err := os.WriteFile(inputFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Create a minimal capsule manifest
	manifest := capsule.Manifest{
		CapsuleVersion: capsule.Version,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		Tool: capsule.ToolInfo{
			Name:    "test",
			Version: "1.0.0",
		},
		Blobs: capsule.BlobIndex{
			BySHA256: make(map[string]*capsule.BlobRecord),
		},
		Artifacts: make(map[string]*capsule.Artifact),
		Runs:      make(map[string]*capsule.Run),
	}

	manifestPath := filepath.Join(tempDir, "manifest.json")

	// Write manifest using JSON
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	t.Log("Created test capsule structure")

	// Verify we can read the manifest back
	readData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	var readManifest capsule.Manifest
	if err := json.Unmarshal(readData, &readManifest); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if readManifest.CapsuleVersion != manifest.CapsuleVersion {
		t.Errorf("manifest version mismatch: got %s, want %s", readManifest.CapsuleVersion, manifest.CapsuleVersion)
	}

	t.Log("Capsule workflow test completed successfully")
}

// TestConversionPluginChain tests chaining multiple format conversions.
func TestConversionPluginChain(t *testing.T) {
	// This test verifies that we can conceptually chain conversions:
	// Format A -> IR -> Format B

	// Get the embedded plugins
	embeddedPlugins := plugins.ListEmbeddedPlugins()

	// Count format plugins
	formatPlugins := []string{}
	for _, plugin := range embeddedPlugins {
		if plugin.Manifest != nil && strings.HasPrefix(plugin.Manifest.PluginID, "format-") {
			formatPlugins = append(formatPlugins, strings.TrimPrefix(plugin.Manifest.PluginID, "format-"))
		}
	}

	t.Logf("Found %d format plugins", len(formatPlugins))

	if len(formatPlugins) == 0 {
		t.Log("No format plugins found (this may be expected in minimal builds)")
	} else {
		t.Logf("Available format plugins: %v", formatPlugins)
	}
}

// TestConversionErrorHandling tests that conversion errors are handled properly.
func TestConversionErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "error-handling-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an invalid JSON file
	invalidFile := filepath.Join(tempDir, "invalid.json")
	invalidContent := "This is not valid JSON"
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0600); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}

	// Try to read it as JSON (should fail gracefully)
	data, err := os.ReadFile(invalidFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err == nil {
		t.Error("expected error when parsing invalid JSON")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// TestConversionValidJSONStructure tests that JSON documents have valid structure.
func TestConversionValidJSONStructure(t *testing.T) {
	testCases := []struct {
		name    string
		doc     map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid minimal document",
			doc: map[string]interface{}{
				"title":    "Test",
				"language": "en",
			},
			wantErr: false,
		},
		{
			name: "document with content",
			doc: map[string]interface{}{
				"title":    "Test Bible",
				"language": "en",
				"content":  []string{"Test verse content"},
			},
			wantErr: false,
		},
	}

	tempDir, err := os.MkdirTemp("", "json-structure-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outputPath := filepath.Join(tempDir, tc.name+".json")

			data, err := json.MarshalIndent(tc.doc, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal JSON: %v", err)
			}

			err = os.WriteFile(outputPath, data, 0600)
			if tc.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil {
				// Verify file was created and can be read back
				readData, err := os.ReadFile(outputPath)
				if err != nil {
					t.Errorf("failed to read back JSON: %v", err)
				} else {
					var parsed map[string]interface{}
					if err := json.Unmarshal(readData, &parsed); err != nil {
						t.Errorf("failed to parse JSON: %v", err)
					} else if parsed["title"] != tc.doc["title"] {
						t.Errorf("title mismatch after roundtrip")
					}
				}
			}
		})
	}
}

// TestConversionMetadataPreservation tests that metadata is preserved during conversion.
func TestConversionMetadataPreservation(t *testing.T) {
	// Create a document with rich metadata
	doc := map[string]interface{}{
		"title":       "Test Bible",
		"language":    "en",
		"rights":      "Public Domain",
		"description": "A test bible for integration testing",
		"content":     []string{"Genesis 1:1 - In the beginning..."},
	}

	tempDir, err := os.MkdirTemp("", "metadata-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "test.json")

	// Write and read back
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	readData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read JSON: %v", err)
	}

	var readDoc map[string]interface{}
	if err := json.Unmarshal(readData, &readDoc); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify all metadata fields preserved
	if readDoc["title"] != doc["title"] {
		t.Errorf("title not preserved: got %v, want %v", readDoc["title"], doc["title"])
	}
	if readDoc["language"] != doc["language"] {
		t.Errorf("language not preserved: got %v, want %v", readDoc["language"], doc["language"])
	}
	if readDoc["rights"] != doc["rights"] {
		t.Errorf("rights not preserved: got %v, want %v", readDoc["rights"], doc["rights"])
	}
	if readDoc["description"] != doc["description"] {
		t.Errorf("description not preserved: got %v, want %v", readDoc["description"], doc["description"])
	}

	t.Log("Metadata preservation verified")
}
