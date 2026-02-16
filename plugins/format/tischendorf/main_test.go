//go:build !sdk

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
		reason   string
	}{
		{
			name: "valid tischendorf with Greek and apparatus",
			content: `Matthew 1
1:1 Βίβλος [γενέσεως] Ἰησοῦ Χριστοῦ
1:2 Ἀβραὰμ ἐγέννησεν τὸν Ἰσαάκ`,
			expected: true,
			reason:   "detected Greek text with critical apparatus and verse references",
		},
		{
			name: "plain text without Greek",
			content: `Matthew 1:1
In the beginning`,
			expected: false,
			reason:   "not a Tischendorf format file",
		},
		{
			name:     "Greek without apparatus",
			content:  `Βίβλος γενέσεως Ἰησοῦ Χριστοῦ`,
			expected: false,
			reason:   "not a Tischendorf format file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.txt")
			if err := os.WriteFile(testFile, []byte(tt.content), 0600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			args := map[string]interface{}{
				"path": testFile,
			}

			var buf bytes.Buffer
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run detect in separate goroutine to capture output
			done := make(chan bool)
			go func() {
				handleDetect(args)
				w.Close()
				done <- true
			}()

			<-done
			buf.ReadFrom(r)
			os.Stdout = oldStdout

			var resp ipc.Response
			if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Status != "ok" {
				t.Fatalf("expected ok status, got: %s", resp.Status)
			}

			resultData, err := json.Marshal(resp.Result)
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}

			var result ipc.DetectResult
			if err := json.Unmarshal(resultData, &result); err != nil {
				t.Fatalf("failed to unmarshal detect result: %v", err)
			}

			if result.Detected != tt.expected {
				t.Errorf("expected detected=%v, got=%v (reason: %s)", tt.expected, result.Detected, result.Reason)
			}

			if !strings.Contains(result.Reason, tt.reason) && result.Reason != tt.reason {
				t.Errorf("expected reason containing %q, got %q", tt.reason, result.Reason)
			}
		})
	}
}

func TestIngest(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	content := `Matthew 1
1:1 Βίβλος γενέσεως Ἰησοῦ Χριστοῦ`

	testFile := filepath.Join(tmpDir, "tischendorf.txt")
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	args := map[string]interface{}{
		"path":       testFile,
		"output_dir": outputDir,
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan bool)
	go func() {
		handleIngest(args)
		w.Close()
		done <- true
	}()

	<-done
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (error: %s)", resp.Status, resp.Error)
	}

	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var result ipc.IngestResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("failed to unmarshal ingest result: %v", err)
	}

	if result.ArtifactID != "tischendorf" {
		t.Errorf("expected artifact_id=tischendorf, got=%s", result.ArtifactID)
	}

	if result.BlobSHA256 == "" {
		t.Error("expected non-empty blob hash")
	}

	if result.SizeBytes != int64(len(content)) {
		t.Errorf("expected size=%d, got=%d", len(content), result.SizeBytes)
	}

	if result.Metadata["format"] != "tischendorf" {
		t.Errorf("expected format=tischendorf, got=%s", result.Metadata["format"])
	}
}

func TestExtractIR(t *testing.T) {
	tmpDir := t.TempDir()
	content := `Matthew
1:1 Βίβλος γενέσεως Ἰησοῦ Χριστοῦ
1:2 Ἀβραὰμ ἐγέννησεν τὸν Ἰσαάκ`

	testFile := filepath.Join(tmpDir, "tischendorf.txt")
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "output.json")

	args := map[string]interface{}{
		"path":        testFile,
		"output_path": outputPath,
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan bool)
	go func() {
		handleExtractIR(args)
		w.Close()
		done <- true
	}()

	<-done
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (error: %s)", resp.Status, resp.Error)
	}

	// Check IR file was created
	irData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read IR file: %v", err)
	}

	var corpus ipc.Corpus
	if err := json.Unmarshal(irData, &corpus); err != nil {
		t.Fatalf("failed to unmarshal IR: %v", err)
	}

	if corpus.ID != "tischendorf-nt" {
		t.Errorf("expected ID=tischendorf-nt, got=%s", corpus.ID)
	}

	if corpus.Language != "grc" {
		t.Errorf("expected language=grc, got=%s", corpus.Language)
	}

	if len(corpus.Documents) == 0 {
		t.Error("expected at least one document")
	}
}

func TestEmitNative(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test IR
	corpus := &ipc.Corpus{
		ID:          "test-tischendorf",
		Version:     "8.0",
		ModuleType:  "bible",
		Language:    "grc",
		Title:       "Test Tischendorf",
		Description: "Test edition",
		Documents: []*ipc.Document{
			{
				ID:    "Matt",
				Title: "Matthew",
				Order: 0,
				ContentBlocks: []*ipc.ContentBlock{
					{
						ID:       "block_0",
						Sequence: 0,
						Text:     "Βίβλος γενέσεως Ἰησοῦ Χριστοῦ",
						Attributes: map[string]interface{}{
							"verse_ref": "1:1",
						},
					},
					{
						ID:       "block_1",
						Sequence: 1,
						Text:     "Ἀβραὰμ ἐγέννησεν τὸν Ἰσαάκ",
						Attributes: map[string]interface{}{
							"verse_ref": "1:2",
						},
					},
				},
			},
		},
	}

	irPath := filepath.Join(tmpDir, "test.json")
	irData, err := json.MarshalIndent(corpus, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal IR: %v", err)
	}

	if err := os.WriteFile(irPath, irData, 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "output.txt")

	args := map[string]interface{}{
		"ir_path":     irPath,
		"output_path": outputPath,
	}

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan bool)
	go func() {
		handleEmitNative(args)
		w.Close()
		done <- true
	}()

	<-done
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	var resp ipc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got: %s (error: %s)", resp.Status, resp.Error)
	}

	// Check output file was created
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Matthew") {
		t.Error("expected output to contain 'Matthew'")
	}

	if !strings.Contains(outputStr, "Βίβλος γενέσεως") {
		t.Error("expected output to contain Greek text")
	}

	if !strings.Contains(outputStr, "1:1") {
		t.Error("expected output to contain verse reference")
	}
}

func TestParseTischendorfToIR(t *testing.T) {
	content := `Matthew
1:1 Βίβλος γενέσεως Ἰησοῦ Χριστοῦ
1:2 Ἀβραὰμ ἐγέννησεν τὸν Ἰσαάκ

Mark
1:1 Ἀρχὴ τοῦ εὐαγγελίου Ἰησοῦ Χριστοῦ`

	corpus := parseTischendorfToIR([]byte(content))

	if corpus.ID != "tischendorf-nt" {
		t.Errorf("expected ID=tischendorf-nt, got=%s", corpus.ID)
	}

	if corpus.Language != "grc" {
		t.Errorf("expected language=grc, got=%s", corpus.Language)
	}

	if len(corpus.Documents) != 2 {
		t.Errorf("expected 2 documents, got=%d", len(corpus.Documents))
	}

	if corpus.Documents[0].ID != "Matthew" {
		t.Errorf("expected first document ID=Matthew, got=%s", corpus.Documents[0].ID)
	}

	if len(corpus.Documents[0].ContentBlocks) != 2 {
		t.Errorf("expected 2 content blocks in first document, got=%d", len(corpus.Documents[0].ContentBlocks))
	}

	firstBlock := corpus.Documents[0].ContentBlocks[0]
	if !strings.Contains(firstBlock.Text, "Βίβλος") {
		t.Errorf("expected first block to contain Greek text, got: %s", firstBlock.Text)
	}
}

func TestExtractReference(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple reference",
			line:     "1:1 Βίβλος γενέσεως",
			expected: "1:1",
		},
		{
			name:     "reference with higher numbers",
			line:     "12:34 Some text here",
			expected: "12:34",
		},
		{
			name:     "no reference",
			line:     "Just some text",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReference(tt.line)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "text with reference",
			line:     "1:1 Βίβλος γενέσεως",
			expected: "Βίβλος γενέσεως",
		},
		{
			name:     "text with apparatus",
			line:     "1:1 Βίβλος [γενέσεως] Ἰησοῦ",
			expected: "Βίβλος  Ἰησοῦ",
		},
		{
			name:     "plain text",
			line:     "Ἀρχὴ τοῦ εὐαγγελίου",
			expected: "Ἀρχὴ τοῦ εὐαγγελίου",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractText(tt.line)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
