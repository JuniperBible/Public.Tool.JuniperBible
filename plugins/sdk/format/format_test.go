package format

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/runtime"
)

func TestStandardDetect(t *testing.T) {
	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".test", ".tst"},
	}

	tests := []struct {
		name     string
		path     string
		detected bool
	}{
		{"matches first extension", "/path/to/file.test", true},
		{"matches second extension", "/path/to/file.tst", true},
		{"case insensitive", "/path/to/file.TEST", true},
		{"no match", "/path/to/file.txt", false},
		{"no extension", "/path/to/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := standardDetect(cfg, tt.path)
			if err != nil {
				t.Fatalf("standardDetect() error = %v", err)
			}
			if result.Detected != tt.detected {
				t.Errorf("Detected = %v, want %v", result.Detected, tt.detected)
			}
		})
	}
}

func TestMakeDetectHandler(t *testing.T) {
	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".test"},
	}

	handler := makeDetectHandler(cfg)

	// Test missing path
	_, err := handler(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing path")
	}

	// Test with path
	result, err := handler(map[string]interface{}{"path": "/file.test"})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	detectResult, ok := result.(*ipc.DetectResult)
	if !ok {
		t.Fatalf("result type = %T, want *ipc.DetectResult", result)
	}
	if !detectResult.Detected {
		t.Error("expected detected = true")
	}
}

func TestMakeIngestHandler(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")
	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".txt"},
	}

	handler := makeIngestHandler(cfg)

	// Test missing path
	_, err := handler(map[string]interface{}{"output_dir": outputDir})
	if err == nil {
		t.Error("expected error for missing path")
	}

	// Test missing output_dir
	_, err = handler(map[string]interface{}{"path": testFile})
	if err == nil {
		t.Error("expected error for missing output_dir")
	}

	// Test successful ingest
	result, err := handler(map[string]interface{}{
		"path":       testFile,
		"output_dir": outputDir,
	})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	ingestResult, ok := result.(*ipc.IngestResult)
	if !ok {
		t.Fatalf("result type = %T, want *ipc.IngestResult", result)
	}

	if ingestResult.ArtifactID != "test" {
		t.Errorf("ArtifactID = %q, want %q", ingestResult.ArtifactID, "test")
	}
	if ingestResult.SizeBytes != int64(len(testContent)) {
		t.Errorf("SizeBytes = %d, want %d", ingestResult.SizeBytes, len(testContent))
	}
	if ingestResult.Metadata["format"] != "TEST" {
		t.Errorf("Metadata[format] = %q, want %q", ingestResult.Metadata["format"], "TEST")
	}
}

func TestMakeEnumerateHandler(t *testing.T) {
	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".test"},
	}

	handler := makeEnumerateHandler(cfg)

	// Test missing path
	_, err := handler(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing path")
	}

	// Test with path (single-file format returns empty entries)
	result, err := handler(map[string]interface{}{"path": "/some/file.test"})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	enumResult, ok := result.(*ipc.EnumerateResult)
	if !ok {
		t.Fatalf("result type = %T, want *ipc.EnumerateResult", result)
	}

	if len(enumResult.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0 for single-file format", len(enumResult.Entries))
	}
}

func TestMakeExtractIRHandler(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".txt"},
		Parse: func(path string) (*ir.Corpus, error) {
			corpus := ir.NewCorpus("test", "bible", "en")
			corpus.Title = "Test"
			return corpus, nil
		},
	}

	handler := makeExtractIRHandler(cfg)

	// Test missing Parse function
	cfgNoParser := &Config{Name: "TEST"}
	handlerNoParser := makeExtractIRHandler(cfgNoParser)
	_, err := handlerNoParser(map[string]interface{}{
		"path":       testFile,
		"output_dir": outputDir,
	})
	if err == nil {
		t.Error("expected error for missing Parse function")
	}

	// Test successful extraction
	result, err := handler(map[string]interface{}{
		"path":       testFile,
		"output_dir": outputDir,
	})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	extractResult, ok := result.(*ipc.ExtractIRResult)
	if !ok {
		t.Fatalf("result type = %T, want *ipc.ExtractIRResult", result)
	}

	if extractResult.IRPath == "" {
		t.Error("IRPath is empty")
	}
	if extractResult.LossClass == "" {
		t.Error("LossClass is empty")
	}
}

func TestMakeEmitNativeHandler(t *testing.T) {
	tmpDir := t.TempDir()

	// Create IR file
	corpus := ir.NewCorpus("test", "bible", "en")
	irPath, err := ir.Write(corpus, tmpDir)
	if err != nil {
		t.Fatalf("failed to write IR: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".test"},
		Emit: func(c *ir.Corpus, outDir string) (string, error) {
			outPath := filepath.Join(outDir, c.ID+".test")
			return outPath, os.WriteFile(outPath, []byte("output"), 0600)
		},
	}

	handler := makeEmitNativeHandler(cfg)

	// Test missing Emit function
	cfgNoEmit := &Config{Name: "TEST"}
	handlerNoEmit := makeEmitNativeHandler(cfgNoEmit)
	_, err = handlerNoEmit(map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": outputDir,
	})
	if err == nil {
		t.Error("expected error for missing Emit function")
	}

	// Test successful emission
	result, err := handler(map[string]interface{}{
		"ir_path":    irPath,
		"output_dir": outputDir,
	})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	emitResult, ok := result.(*ipc.EmitNativeResult)
	if !ok {
		t.Fatalf("result type = %T, want *ipc.EmitNativeResult", result)
	}

	if emitResult.OutputPath == "" {
		t.Error("OutputPath is empty")
	}
	if emitResult.Format != "TEST" {
		t.Errorf("Format = %q, want %q", emitResult.Format, "TEST")
	}
}

func TestDetermineLossClass(t *testing.T) {
	tests := []struct {
		name   string
		corpus *ir.Corpus
		want   string
	}{
		{"nil corpus", nil, "L4"},
		{"empty loss class", ir.NewCorpus("test", "bible", "en"), "L1"},
		{"explicit L2", func() *ir.Corpus {
			c := ir.NewCorpus("test", "bible", "en")
			c.LossClass = "L2"
			return c
		}(), "L2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineLossClass(tt.corpus)
			if got != tt.want {
				t.Errorf("determineLossClass() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPluginIntegration(t *testing.T) {
	// Test that a format plugin can handle a complete IPC session
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "sample.test")
	if err := os.WriteFile(testFile, []byte("sample data"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &Config{
		Name:       "TEST",
		Extensions: []string{".test"},
		Parse: func(path string) (*ir.Corpus, error) {
			return ir.NewCorpus("sample", "bible", "en"), nil
		},
		Emit: func(c *ir.Corpus, outDir string) (string, error) {
			outPath := filepath.Join(outDir, c.ID+".test")
			return outPath, os.WriteFile(outPath, []byte("output"), 0600)
		},
	}

	// Create dispatcher with handlers
	d := runtime.NewDispatcher()
	d.Register("detect", makeDetectHandler(cfg))
	d.Register("ingest", makeIngestHandler(cfg))
	d.Register("enumerate", makeEnumerateHandler(cfg))
	d.Register("extract-ir", makeExtractIRHandler(cfg))
	d.Register("emit-native", makeEmitNativeHandler(cfg))

	// Test detect command
	detectReq := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": testFile},
	}
	reqBytes, _ := json.Marshal(detectReq)

	var output bytes.Buffer
	err := runtime.RunWithIO(d, strings.NewReader(string(reqBytes)+"\n"), &output)
	if err != nil {
		t.Fatalf("RunWithIO() error = %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
}
