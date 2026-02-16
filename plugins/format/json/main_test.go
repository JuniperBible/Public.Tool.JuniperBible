package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

// TestJSONDetect tests the detect command.
func TestJSONDetect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonContent := `{
  "meta": {
    "id": "test",
    "title": "Test Bible",
    "version": "1.0.0"
  },
  "books": [
    {
      "id": "Gen",
      "name": "Genesis",
      "order": 1,
      "chapters": [
        {
          "number": 1,
          "verses": [
            {"book": "Gen", "chapter": 1, "verse": 1, "text": "In the beginning.", "id": "Gen.1.1"}
          ]
        }
      ]
    }
  ]
}
`

	jsonPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": jsonPath},
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
	if result["format"] != "JSON" {
		t.Errorf("expected format JSON, got %v", result["format"])
	}
}

// TestJSONDetectNonJSON tests detect command on non-JSON file.
func TestJSONDetectNonJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
		t.Error("expected detected to be false for non-JSON file")
	}
}

// TestJSONExtractIR tests the extract-ir command.
func TestJSONExtractIR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonContent := `{
  "meta": {
    "id": "test",
    "title": "Test Bible",
    "version": "1.0.0"
  },
  "books": [
    {
      "id": "Gen",
      "name": "Genesis",
      "order": 1,
      "chapters": [
        {
          "number": 1,
          "verses": [
            {"book": "Gen", "chapter": 1, "verse": 1, "text": "In the beginning.", "id": "Gen.1.1"},
            {"book": "Gen", "chapter": 1, "verse": 2, "text": "And the earth was void.", "id": "Gen.1.2"}
          ]
        }
      ]
    }
  ]
}
`

	jsonPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       jsonPath,
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

	if result["loss_class"] != "L0" {
		t.Errorf("expected loss_class L0, got %v", result["loss_class"])
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

	if corpus.ID != "test" {
		t.Errorf("expected ID test, got %s", corpus.ID)
	}
	if corpus.Title != "Test Bible" {
		t.Errorf("expected title Test Bible, got %s", corpus.Title)
	}
	if len(corpus.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(corpus.Documents))
	}
	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks, got %d", len(corpus.Documents[0].ContentBlocks))
	}
}

// TestJSONEmitNative tests the emit-native command.
func TestJSONEmitNative(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	corpus := ipc.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: "BIBLE",
		Title:      "Test Bible",
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

	if result["format"] != "JSON" {
		t.Errorf("expected format JSON, got %v", result["format"])
	}

	jsonPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read JSON file: %v", err)
	}

	if !bytes.Contains(jsonData, []byte(`"meta"`)) {
		t.Error("output does not contain meta field")
	}
	if !bytes.Contains(jsonData, []byte("In the beginning.")) {
		t.Error("output does not contain verse text")
	}
}

// TestJSONRoundTrip tests L0 lossless round-trip.
func TestJSONRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalContent := `{
  "meta": {
    "id": "test",
    "title": "Test Bible",
    "version": "1.0.0"
  },
  "books": [
    {
      "id": "Gen",
      "name": "Genesis",
      "order": 1,
      "chapters": [
        {
          "number": 1,
          "verses": [
            {"book": "Gen", "chapter": 1, "verse": 1, "text": "In the beginning God created.", "id": "Gen.1.1"},
            {"book": "Gen", "chapter": 1, "verse": 2, "text": "And the earth was void.", "id": "Gen.1.2"}
          ]
        }
      ]
    }
  ]
}
`

	jsonPath := filepath.Join(tmpDir, "original.json")
	if err := os.WriteFile(jsonPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	irDir := filepath.Join(tmpDir, "ir")
	outDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(irDir, 0755)
	os.MkdirAll(outDir, 0755)

	// Extract IR
	extractReq := ipc.Request{
		Command: "extract-ir",
		Args: map[string]interface{}{
			"path":       jsonPath,
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

	// Compare original and output
	originalData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read original: %v", err)
	}

	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	originalHash := sha256.Sum256(originalData)
	outputHash := sha256.Sum256(outputData)

	if originalHash != outputHash {
		t.Errorf("L0 round-trip failed: hashes differ\noriginal: %s\noutput:   %s",
			hex.EncodeToString(originalHash[:]),
			hex.EncodeToString(outputHash[:]))
	}
}

// TestJSONIngest tests the ingest command.
func TestJSONIngest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonContent := `{"meta": {"id": "test"}, "books": []}`

	jsonPath := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "blobs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "ingest",
		Args: map[string]interface{}{
			"path":       jsonPath,
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

// TestJSONEmitParallel tests the emit-parallel command.
func TestJSONEmitParallel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	parallel := ipc.ParallelCorpus{
		ID:               "kjv-niv-parallel",
		Version:          "1.0.0",
		DefaultAlignment: "verse",
		BaseCorpus: &ipc.CorpusRef{
			ID:       "KJV",
			Language: "en",
			Title:    "King James Version",
		},
		Corpora: []*ipc.CorpusRef{
			{ID: "KJV", Language: "en", Title: "King James Version"},
			{ID: "NIV", Language: "en", Title: "New International Version"},
		},
		Alignments: []*ipc.Alignment{
			{
				ID:    "verse-alignment",
				Level: "verse",
				Units: []*ipc.AlignedUnit{
					{
						ID:    "au1",
						Level: "verse",
						Ref:   &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 1, OSISID: "Gen.1.1"},
						Texts: map[string]string{
							"KJV": "In the beginning God created the heaven and the earth.",
							"NIV": "In the beginning God created the heavens and the earth.",
						},
					},
					{
						ID:    "au2",
						Level: "verse",
						Ref:   &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 2, OSISID: "Gen.1.2"},
						Texts: map[string]string{
							"KJV": "And the earth was without form, and void.",
							"NIV": "Now the earth was formless and empty.",
						},
					},
				},
			},
		},
	}

	irData, err := json.MarshalIndent(&parallel, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal parallel corpus: %v", err)
	}

	irPath := filepath.Join(tmpDir, "parallel.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-parallel",
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

	if result["format"] != "JSON-Parallel" {
		t.Errorf("expected format JSON-Parallel, got %v", result["format"])
	}

	outputPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	jsonData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify content
	if !bytes.Contains(jsonData, []byte(`"translations"`)) {
		t.Error("output does not contain translations field")
	}
	if !bytes.Contains(jsonData, []byte(`"KJV"`)) {
		t.Error("output does not contain KJV translation")
	}
	if !bytes.Contains(jsonData, []byte(`"NIV"`)) {
		t.Error("output does not contain NIV translation")
	}
	if !bytes.Contains(jsonData, []byte(`"Gen.1.1"`)) {
		t.Error("output does not contain verse reference")
	}
}

// TestJSONEmitInterlinear tests the emit-interlinear command.
func TestJSONEmitInterlinear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lines := []ipc.InterlinearLine{
		{
			Ref: &ipc.Ref{Book: "Gen", Chapter: 1, Verse: 1, OSISID: "Gen.1.1"},
			Layers: map[string]*ipc.InterlinearLayer{
				"hebrew": {
					CorpusID: "OSHB",
					Tokens:   []string{"בְּרֵאשִׁית", "בָּרָא", "אֱלֹהִים"},
					Label:    "Hebrew",
				},
				"english": {
					CorpusID: "KJV",
					Tokens:   []string{"In the beginning", "created", "God"},
					Label:    "English",
				},
			},
		},
	}

	irData, err := json.MarshalIndent(&lines, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal interlinear lines: %v", err)
	}

	irPath := filepath.Join(tmpDir, "interlinear.json")
	if err := os.WriteFile(irPath, irData, 0644); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	req := ipc.Request{
		Command: "emit-interlinear",
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

	if result["format"] != "JSON-Interlinear" {
		t.Errorf("expected format JSON-Interlinear, got %v", result["format"])
	}

	outputPath, ok := result["output_path"].(string)
	if !ok {
		t.Fatal("output_path is not a string")
	}

	jsonData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// Verify content
	if !bytes.Contains(jsonData, []byte(`"layers"`)) {
		t.Error("output does not contain layers field")
	}
	if !bytes.Contains(jsonData, []byte(`"hebrew"`)) {
		t.Error("output does not contain hebrew layer")
	}
	if !bytes.Contains(jsonData, []byte(`"english"`)) {
		t.Error("output does not contain english layer")
	}
	if !bytes.Contains(jsonData, []byte(`"Gen.1.1"`)) {
		t.Error("output does not contain verse reference")
	}
}

// executePlugin runs the plugin with a request and returns the response.
func executePlugin(t *testing.T, req *ipc.Request) *ipc.Response {
	t.Helper()

	pluginPath := "./format-json"

	// Always rebuild to ensure tests use current code
	buildCmd := exec.Command("go", "build", "-o", pluginPath, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, output)
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

// TestJSONDetectMalformedJSON tests detection of malformed JSON.
func TestJSONDetectMalformedJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	malformedContent := `{"module_type": "BIBLE", "documents": [unclosed`
	malformedPath := filepath.Join(tmpDir, "malformed.json")
	if err := os.WriteFile(malformedPath, []byte(malformedContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": malformedPath},
	}
	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	result := resp.Result.(map[string]interface{})
	if result["detected"] == true {
		t.Error("expected detected to be false for malformed JSON")
	}
}

// TestJSONDetectEmptyJSON tests detection of empty JSON.
func TestJSONDetectEmptyJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "json-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	emptyPath := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(emptyPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	req := ipc.Request{
		Command: "detect",
		Args:    map[string]interface{}{"path": emptyPath},
	}
	resp := executePlugin(t, &req)
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	result := resp.Result.(map[string]interface{})
	// Empty JSON object should not be detected as Bible format
	if result["detected"] == true {
		t.Error("expected detected to be false for empty JSON object")
	}
}
