// Package testing provides a table-driven test framework for format plugins.
package testing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/errors"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// FormatTestCase defines a comprehensive test case for a format plugin.
type FormatTestCase struct {
	// Config is the format plugin configuration to test
	Config *format.Config

	// SampleFile is the path to a test fixture file
	SampleFile string

	// SampleContent is the raw content to write (alternative to SampleFile)
	SampleContent string

	// ExpectedIR contains assertions about the parsed IR corpus
	ExpectedIR *IRExpectations

	// RoundTrip enables L0 lossless round-trip testing
	RoundTrip bool

	// NegativeDetection is a file that should NOT be detected as this format
	NegativeDetection string

	// ExpectedLossClass for extract-ir (e.g., "L0", "L1", "L2", "L3")
	ExpectedLossClass string

	// SkipTests allows skipping specific subtests
	SkipTests []string
}

// IRExpectations defines assertions about the parsed IR corpus.
type IRExpectations struct {
	// ID expected in corpus
	ID string

	// Title expected in corpus
	Title string

	// MinDocuments is the minimum number of documents expected
	MinDocuments int

	// MinContentBlocks is the minimum number of content blocks in first document
	MinContentBlocks int

	// CustomValidation is a custom validation function
	CustomValidation func(t *testing.T, corpus *ipc.Corpus)
}

// RunFormatTests executes a comprehensive test suite for a format plugin.
func RunFormatTests(t *testing.T, tc FormatTestCase) {
	if tc.Config == nil {
		t.Fatal("Config is required")
	}

	tmpDir, err := os.MkdirTemp("", "format-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFilePath := prepareTestFile(t, tc, tmpDir)

	for _, st := range buildSubtests(tc, testFilePath, tmpDir) {
		runSubtest(t, tc.SkipTests, st)
	}
}

type subtest struct {
	name    string
	enabled bool
	fn      func(*testing.T)
}

func prepareTestFile(t *testing.T, tc FormatTestCase, tmpDir string) string {
	t.Helper()
	if tc.SampleFile != "" {
		return tc.SampleFile
	}
	if tc.SampleContent == "" {
		t.Fatal("either SampleFile or SampleContent is required")
	}
	ext := ".txt"
	if len(tc.Config.Extensions) > 0 {
		ext = tc.Config.Extensions[0]
	}
	path := filepath.Join(tmpDir, "sample"+ext)
	if err := os.WriteFile(path, []byte(tc.SampleContent), 0600); err != nil {
		t.Fatalf("failed to write sample content: %v", err)
	}
	return path
}

func buildSubtests(tc FormatTestCase, testFilePath, tmpDir string) []subtest {
	hasEmit := tc.Config.Parse != nil && tc.Config.Emit != nil
	return []subtest{
		{"Detect", true, func(t *testing.T) { testDetect(t, tc, testFilePath) }},
		{"DetectNegative", tc.NegativeDetection != "", func(t *testing.T) { testDetectNegative(t, tc, tmpDir) }},
		{"Ingest", true, func(t *testing.T) { testIngest(t, tc, testFilePath, tmpDir) }},
		{"Enumerate", true, func(t *testing.T) { testEnumerate(t, tc, testFilePath, tmpDir) }},
		{"ExtractIR", tc.Config.Parse != nil, func(t *testing.T) { testExtractIR(t, tc, testFilePath, tmpDir) }},
		{"EmitNative", hasEmit, func(t *testing.T) { testEmitNative(t, tc, testFilePath, tmpDir) }},
		{"RoundTrip", hasEmit && tc.RoundTrip, func(t *testing.T) { testRoundTrip(t, tc, testFilePath, tmpDir) }},
	}
}

func runSubtest(t *testing.T, skipList []string, st subtest) {
	t.Helper()
	if !st.enabled || shouldSkip(skipList, st.name) {
		return
	}
	t.Run(st.name, st.fn)
}

// testDetect tests the detect functionality.
func testDetect(t *testing.T, tc FormatTestCase, path string) {
	t.Helper()

	result, err := callDetect(tc.Config, path)
	if err != nil {
		t.Fatalf("detect failed: %v", err)
	}

	if !result.Detected {
		t.Error("expected detected to be true")
	}

	if result.Format != tc.Config.Name {
		t.Errorf("expected format %s, got %s", tc.Config.Name, result.Format)
	}
}

// testDetectNegative tests that non-matching files are not detected.
func testDetectNegative(t *testing.T, tc FormatTestCase, tmpDir string) {
	t.Helper()

	// Create a file that should NOT be detected
	wrongPath := filepath.Join(tmpDir, "wrong.txt")
	if err := os.WriteFile(wrongPath, []byte(tc.NegativeDetection), 0600); err != nil {
		t.Fatalf("failed to write negative test file: %v", err)
	}

	result, err := callDetect(tc.Config, wrongPath)
	if err != nil {
		t.Fatalf("detect failed: %v", err)
	}

	if result.Detected {
		t.Error("expected detected to be false for non-matching file")
	}
}

// testIngest tests the ingest functionality.
func testIngest(t *testing.T, tc FormatTestCase, path, tmpDir string) {
	t.Helper()

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	result, err := callIngest(tc.Config, path, outputDir)
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	if result.BlobSHA256 == "" {
		t.Error("expected non-empty blob_sha256")
	}

	// Verify blob file exists
	blobPath := filepath.Join(outputDir, result.BlobSHA256[:2], result.BlobSHA256)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		t.Error("blob file was not created")
	}
}

// testEnumerate tests the enumerate functionality.
func testEnumerate(t *testing.T, tc FormatTestCase, path, tmpDir string) {
	t.Helper()

	result, err := callEnumerate(tc.Config, path)
	if err != nil {
		t.Fatalf("enumerate failed: %v", err)
	}

	// For single-file formats, entries may be empty or contain one item
	// Archive formats should have multiple entries
	if result.Entries == nil {
		result.Entries = []ipc.EnumerateEntry{}
	}

	t.Logf("enumerate returned %d entries", len(result.Entries))
}

// testExtractIR tests the extract-ir functionality.
func testExtractIR(t *testing.T, tc FormatTestCase, path, tmpDir string) {
	t.Helper()

	outputDir := filepath.Join(tmpDir, "ir")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	result, err := callExtractIR(tc.Config, path, outputDir)
	if err != nil {
		t.Fatalf("extract-ir failed: %v", err)
	}

	if result.IRPath == "" {
		t.Error("expected non-empty ir_path")
	}

	if tc.ExpectedLossClass != "" && result.LossClass != tc.ExpectedLossClass {
		t.Errorf("expected loss_class %s, got %s", tc.ExpectedLossClass, result.LossClass)
	}

	// Read and validate IR
	irData, err := os.ReadFile(result.IRPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	// Validate expectations
	if tc.ExpectedIR != nil {
		validateIRExpectations(t, tc.ExpectedIR, &corpus)
	}
}

// testEmitNative tests the emit-native functionality.
func testEmitNative(t *testing.T, tc FormatTestCase, path, tmpDir string) {
	t.Helper()

	// First extract IR
	irDir := filepath.Join(tmpDir, "ir-emit")
	if err := os.MkdirAll(irDir, 0700); err != nil {
		t.Fatalf("failed to create ir dir: %v", err)
	}

	extractResult, err := callExtractIR(tc.Config, path, irDir)
	if err != nil {
		t.Fatalf("extract-ir failed: %v", err)
	}

	// Then emit native
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	result, err := callEmitNative(tc.Config, extractResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("emit-native failed: %v", err)
	}

	if result.OutputPath == "" {
		t.Error("expected non-empty output_path")
	}

	if result.Format != tc.Config.Name {
		t.Errorf("expected format %s, got %s", tc.Config.Name, result.Format)
	}

	// Verify output file exists
	if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
		t.Error("output file was not created")
	}
}

// testRoundTrip tests L0 lossless round-trip conversion.
func testRoundTrip(t *testing.T, tc FormatTestCase, path, tmpDir string) {
	t.Helper()

	// Read original
	originalData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read original: %v", err)
	}

	// Extract IR
	irDir := filepath.Join(tmpDir, "ir-roundtrip")
	if err := os.MkdirAll(irDir, 0700); err != nil {
		t.Fatalf("failed to create ir dir: %v", err)
	}

	extractResult, err := callExtractIR(tc.Config, path, irDir)
	if err != nil {
		t.Fatalf("extract-ir failed: %v", err)
	}

	// Emit native
	outputDir := filepath.Join(tmpDir, "roundtrip-output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	emitResult, err := callEmitNative(tc.Config, extractResult.IRPath, outputDir)
	if err != nil {
		t.Fatalf("emit-native failed: %v", err)
	}

	// Read output
	outputData, err := os.ReadFile(emitResult.OutputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Compare hashes
	originalHash := sha256.Sum256(originalData)
	outputHash := sha256.Sum256(outputData)

	if originalHash != outputHash {
		t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))

		// Show a snippet for debugging (first 200 bytes)
		maxLen := 200
		if len(originalData) < maxLen {
			maxLen = len(originalData)
		}
		t.Logf("Original (first %d bytes): %q", maxLen, string(originalData[:maxLen]))

		if len(outputData) < maxLen {
			maxLen = len(outputData)
		}
		t.Logf("Output (first %d bytes): %q", maxLen, string(outputData[:maxLen]))
	}

	// Verify loss class is L0
	if emitResult.LossClass != "L0" {
		t.Errorf("expected loss_class L0 for round-trip, got %s", emitResult.LossClass)
	}
}

// validateIRExpectations checks that the corpus meets expectations.
func validateIRExpectations(t *testing.T, exp *IRExpectations, corpus *ipc.Corpus) {
	t.Helper()

	if exp.ID != "" && corpus.ID != exp.ID {
		t.Errorf("expected ID %s, got %s", exp.ID, corpus.ID)
	}

	if exp.Title != "" && corpus.Title != exp.Title {
		t.Errorf("expected title %s, got %s", exp.Title, corpus.Title)
	}

	if exp.MinDocuments > 0 && len(corpus.Documents) < exp.MinDocuments {
		t.Errorf("expected at least %d documents, got %d", exp.MinDocuments, len(corpus.Documents))
	}

	if exp.MinContentBlocks > 0 && len(corpus.Documents) > 0 {
		blocks := len(corpus.Documents[0].ContentBlocks)
		if blocks < exp.MinContentBlocks {
			t.Errorf("expected at least %d content blocks, got %d", exp.MinContentBlocks, blocks)
		}
	}

	if exp.CustomValidation != nil {
		exp.CustomValidation(t, corpus)
	}
}

// Helper functions that call format.Config methods

func callDetect(cfg *format.Config, path string) (*ipc.DetectResult, error) {
	args := map[string]interface{}{"path": path}
	handler := makeTestDetectHandler(cfg)
	result, err := handler(args)
	if err != nil {
		return nil, err
	}
	return result.(*ipc.DetectResult), nil
}

func callIngest(cfg *format.Config, path, outputDir string) (*ipc.IngestResult, error) {
	args := map[string]interface{}{
		"path":       path,
		"output_dir": outputDir,
	}
	handler := makeTestIngestHandler(cfg)
	result, err := handler(args)
	if err != nil {
		return nil, err
	}
	return result.(*ipc.IngestResult), nil
}

func callEnumerate(cfg *format.Config, path string) (*ipc.EnumerateResult, error) {
	args := map[string]interface{}{"path": path}
	handler := makeTestEnumerateHandler(cfg)
	result, err := handler(args)
	if err != nil {
		return nil, err
	}
	return result.(*ipc.EnumerateResult), nil
}

func callExtractIR(cfg *format.Config, path, outputDir string) (*ipc.ExtractIRResult, error) {
	args := map[string]interface{}{
		"path":       path,
		"output_dir": outputDir,
	}
	handler := makeTestExtractIRHandler(cfg)
	result, err := handler(args)
	if err != nil {
		return nil, err
	}
	return result.(*ipc.ExtractIRResult), nil
}

func callEmitNative(cfg *format.Config, irPath, outputDir string) (*ipc.EmitNativeResult, error) {
	args := map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": outputDir,
	}
	handler := makeTestEmitNativeHandler(cfg)
	result, err := handler(args)
	if err != nil {
		return nil, err
	}
	return result.(*ipc.EmitNativeResult), nil
}

// makeTestDetectHandler creates a detect handler for testing (mimics plugins/sdk/format behavior).
func makeTestDetectHandler(cfg *format.Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}

		if cfg.Detect != nil {
			return cfg.Detect(path)
		}

		// Standard detection
		ext := filepath.Ext(path)
		for _, e := range cfg.Extensions {
			if e == ext {
				return &ipc.DetectResult{
					Detected: true,
					Format:   cfg.Name,
					Reason:   cfg.Name + " format detected via extension",
				}, nil
			}
		}

		return &ipc.DetectResult{
			Detected: false,
			Reason:   "Extension does not match " + cfg.Name + " format",
		}, nil
	}
}

// makeTestIngestHandler creates an ingest handler for testing.
func makeTestIngestHandler(cfg *format.Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}

		outputDir, ok := args["output_dir"].(string)
		if !ok || outputDir == "" {
			return nil, errors.MissingArg("output_dir")
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		// Apply transform if available
		var metadata map[string]string
		if cfg.IngestTransform != nil {
			data, metadata, err = cfg.IngestTransform(path)
			if err != nil {
				return nil, err
			}
		}

		// Calculate hash
		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])

		// Store blob
		blobDir := filepath.Join(outputDir, hashStr[:2])
		if err := os.MkdirAll(blobDir, 0700); err != nil {
			return nil, err
		}

		blobPath := filepath.Join(blobDir, hashStr)
		if err := os.WriteFile(blobPath, data, 0600); err != nil {
			return nil, err
		}

		result := &ipc.IngestResult{
			BlobSHA256: hashStr,
			SizeBytes:  int64(len(data)),
			Metadata:   metadata,
		}

		return result, nil
	}
}

// makeTestEnumerateHandler creates an enumerate handler for testing.
func makeTestEnumerateHandler(cfg *format.Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}

		if cfg.Enumerate != nil {
			return cfg.Enumerate(path)
		}

		// Default: single-file format
		return &ipc.EnumerateResult{
			Entries: []ipc.EnumerateEntry{},
		}, nil
	}
}

// makeTestExtractIRHandler creates an extract-ir handler for testing.
func makeTestExtractIRHandler(cfg *format.Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return nil, errors.MissingArg("path")
		}

		outputDir, ok := args["output_dir"].(string)
		if !ok || outputDir == "" {
			return nil, errors.MissingArg("output_dir")
		}

		if cfg.Parse == nil {
			return nil, errors.New(errors.CodeInternal, "extract-ir not supported")
		}

		// Parse to IR
		corpus, err := cfg.Parse(path)
		if err != nil {
			return nil, err
		}

		// Convert ir.Corpus to ipc.Corpus
		ipcCorpus := convertToIPCCorpus(corpus)

		// Write IR to file
		irPath := filepath.Join(outputDir, filepath.Base(path)+".ir.json")
		irData, err := json.MarshalIndent(ipcCorpus, "", "  ")
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(irPath, irData, 0600); err != nil {
			return nil, err
		}

		return &ipc.ExtractIRResult{
			IRPath:    irPath,
			LossClass: ipcCorpus.LossClass,
		}, nil
	}
}

// makeTestEmitNativeHandler creates an emit-native handler for testing.
func makeTestEmitNativeHandler(cfg *format.Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		irPath, ok := args["ir_path"].(string)
		if !ok || irPath == "" {
			return nil, errors.MissingArg("ir_path")
		}

		outputDir, ok := args["output_dir"].(string)
		if !ok || outputDir == "" {
			return nil, errors.MissingArg("output_dir")
		}

		if cfg.Emit == nil {
			return nil, errors.New(errors.CodeInternal, "emit-native not supported")
		}

		// Read IR
		irData, err := os.ReadFile(irPath)
		if err != nil {
			return nil, err
		}

		var ipcCorpus ipc.Corpus
		if err := json.Unmarshal(irData, &ipcCorpus); err != nil {
			return nil, err
		}

		// Convert ipc.Corpus to ir.Corpus (they're the same type via type alias)
		corpus := (*ir.Corpus)(&ipcCorpus)

		// Emit native
		outputPath, err := cfg.Emit(corpus, outputDir)
		if err != nil {
			return nil, err
		}

		return &ipc.EmitNativeResult{
			OutputPath: outputPath,
			Format:     cfg.Name,
			LossClass:  ipcCorpus.LossClass,
		}, nil
	}
}

// convertToIPCCorpus converts ir.Corpus to ipc.Corpus.
func convertToIPCCorpus(corpus interface{}) *ipc.Corpus {
	// This is a placeholder - in reality, ir.Corpus and ipc.Corpus
	// have the same structure, so we can marshal/unmarshal
	data, _ := json.Marshal(corpus)
	var result ipc.Corpus
	json.Unmarshal(data, &result)
	return &result
}

// convertFromIPCCorpus converts ipc.Corpus to ir.Corpus.
func convertFromIPCCorpus(corpus *ipc.Corpus) interface{} {
	// This is a placeholder - in reality, ir.Corpus and ipc.Corpus
	// have the same structure, so we can marshal/unmarshal
	data, _ := json.Marshal(corpus)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

// shouldSkip checks if a test should be skipped.
func shouldSkip(skipList []string, testName string) bool {
	for _, skip := range skipList {
		if skip == testName {
			return true
		}
	}
	return false
}

// CreateTestFile is a helper to create a test file with given content.
func CreateTestFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

// CompareFiles compares two files byte-by-byte.
func CompareFiles(t *testing.T, path1, path2 string) bool {
	t.Helper()

	data1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path1, err)
	}

	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path2, err)
	}

	return bytes.Equal(data1, data2)
}
