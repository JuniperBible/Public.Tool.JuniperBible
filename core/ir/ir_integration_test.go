package ir_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuniperBible/juniper/core/ir"
)

const testdataDir = "../../testdata"

// TestIRFixturesExist verifies all test fixtures are in place.
func TestIRFixturesExist(t *testing.T) {
	fixtures := []string{
		"fixtures/inputs/osis/sample.osis",
		"fixtures/inputs/usfm/sample.usfm",
		"fixtures/inputs/usx/sample.usx",
		"fixtures/inputs/zefania/sample.xml",
		"fixtures/inputs/theword/sample.ont",
		"fixtures/ir/osis/sample.ir.json",
		"fixtures/ir/usfm/sample.ir.json",
		"fixtures/ir/usx/sample.ir.json",
		"fixtures/ir/zefania/sample.ir.json",
		"fixtures/ir/theword/sample.ir.json",
	}

	for _, f := range fixtures {
		path := filepath.Join(testdataDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("fixture missing: %s", f)
		}
	}
}

// TestIRGoldenHashesExist verifies golden hashes are in place.
func TestIRGoldenHashesExist(t *testing.T) {
	goldens := []string{
		"goldens/ir/osis-sample-ir.sha256",
		"goldens/ir/usfm-sample-ir.sha256",
		"goldens/ir/usx-sample-ir.sha256",
		"goldens/ir/zefania-sample-ir.sha256",
		"goldens/ir/theword-sample-ir.sha256",
	}

	for _, g := range goldens {
		path := filepath.Join(testdataDir, g)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("golden hash missing: %s", g)
		}
	}
}

// TestIRFixtureParses verifies IR fixtures parse correctly.
func TestIRFixtureParses(t *testing.T) {
	irFixtures := []string{
		"fixtures/ir/osis/sample.ir.json",
		"fixtures/ir/usfm/sample.ir.json",
		"fixtures/ir/usx/sample.ir.json",
		"fixtures/ir/zefania/sample.ir.json",
		"fixtures/ir/theword/sample.ir.json",
	}

	for _, f := range irFixtures {
		t.Run(f, func(t *testing.T) {
			path := filepath.Join(testdataDir, f)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var corpus ir.Corpus
			if err := json.Unmarshal(data, &corpus); err != nil {
				t.Fatalf("failed to parse IR fixture: %v", err)
			}

			// Verify basic fields
			if corpus.ID == "" {
				t.Error("corpus.ID is empty")
			}
			if corpus.Version == "" {
				t.Error("corpus.Version is empty")
			}
			if corpus.ModuleType == "" {
				t.Error("corpus.ModuleType is empty")
			}
			if len(corpus.Documents) == 0 {
				t.Error("corpus has no documents")
			}
		})
	}
}

// TestIRFixtureMatchesGolden verifies IR fixture hashes match goldens.
func TestIRFixtureMatchesGolden(t *testing.T) {
	tests := []struct {
		fixture string
		golden  string
	}{
		{"fixtures/ir/osis/sample.ir.json", "goldens/ir/osis-sample-ir.sha256"},
		{"fixtures/ir/usfm/sample.ir.json", "goldens/ir/usfm-sample-ir.sha256"},
		{"fixtures/ir/usx/sample.ir.json", "goldens/ir/usx-sample-ir.sha256"},
		{"fixtures/ir/zefania/sample.ir.json", "goldens/ir/zefania-sample-ir.sha256"},
		{"fixtures/ir/theword/sample.ir.json", "goldens/ir/theword-sample-ir.sha256"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			// Read fixture
			fixturePath := filepath.Join(testdataDir, tt.fixture)
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			// Compute hash
			h := sha256.Sum256(data)
			actualHash := hex.EncodeToString(h[:])

			// Read golden
			goldenPath := filepath.Join(testdataDir, tt.golden)
			goldenData, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden: %v", err)
			}
			expectedHash := strings.TrimSpace(string(goldenData))

			// Compare
			if actualHash != expectedHash {
				t.Errorf("hash mismatch:\n  got:  %s\n  want: %s", actualHash, expectedHash)
			}
		})
	}
}

// TestIRCorpusValidation verifies IR corpus validation.
func TestIRCorpusValidation(t *testing.T) {
	irFixtures := []string{
		"fixtures/ir/osis/sample.ir.json",
		"fixtures/ir/usfm/sample.ir.json",
		"fixtures/ir/usx/sample.ir.json",
		"fixtures/ir/zefania/sample.ir.json",
		"fixtures/ir/theword/sample.ir.json",
	}

	for _, f := range irFixtures {
		t.Run(f, func(t *testing.T) {
			path := filepath.Join(testdataDir, f)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var corpus ir.Corpus
			if err := json.Unmarshal(data, &corpus); err != nil {
				t.Fatalf("failed to parse IR fixture: %v", err)
			}

			// Validate the corpus
			if err := ir.ValidateCorpus(&corpus); err != nil {
				t.Errorf("corpus validation failed: %v", err)
			}
		})
	}
}

// TestIRDocumentStructure verifies document structure.
func TestIRDocumentStructure(t *testing.T) {
	path := filepath.Join(testdataDir, "fixtures/ir/osis/sample.ir.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var corpus ir.Corpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("failed to parse IR fixture: %v", err)
	}

	// Verify we have expected documents
	if len(corpus.Documents) < 1 {
		t.Fatal("expected at least 1 document")
	}

	doc := corpus.Documents[0]
	if doc.ID == "" {
		t.Error("document ID is empty")
	}
	if doc.Title == "" {
		t.Error("document Title is empty")
	}
	if len(doc.ContentBlocks) == 0 {
		t.Error("document has no content blocks")
	}

	// Verify content blocks have text
	for i, cb := range doc.ContentBlocks {
		if cb.Text == "" {
			t.Errorf("content block %d has no text", i)
		}
	}
}

// TestIRContentBlockSequencing verifies content blocks are sequenced correctly.
func TestIRContentBlockSequencing(t *testing.T) {
	irFixtures := []string{
		"fixtures/ir/osis/sample.ir.json",
		"fixtures/ir/usfm/sample.ir.json",
	}

	for _, f := range irFixtures {
		t.Run(f, func(t *testing.T) {
			path := filepath.Join(testdataDir, f)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var corpus ir.Corpus
			if err := json.Unmarshal(data, &corpus); err != nil {
				t.Fatalf("failed to parse IR fixture: %v", err)
			}

			for _, doc := range corpus.Documents {
				for i, cb := range doc.ContentBlocks {
					if cb.Sequence != i {
						t.Errorf("doc %s: content block %d has sequence %d, want %d",
							doc.ID, i, cb.Sequence, i)
					}
				}
			}
		})
	}
}

// TestIRTestPlansExist verifies test plans are in place.
func TestIRTestPlansExist(t *testing.T) {
	plans := []string{
		"plans/ir-roundtrip-osis.json",
		"plans/ir-roundtrip-usfm.json",
		"plans/ir-extraction-sword.json",
		"plans/ir-roundtrip-sword.json",
	}

	for _, p := range plans {
		path := filepath.Join(testdataDir, p)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("test plan missing: %s", p)
		}
	}
}

// TestIRTestPlansParse verifies test plans parse correctly.
func TestIRTestPlansParse(t *testing.T) {
	plans := []string{
		"plans/ir-roundtrip-osis.json",
		"plans/ir-roundtrip-usfm.json",
		"plans/ir-extraction-sword.json",
		"plans/ir-roundtrip-sword.json",
	}

	for _, p := range plans {
		t.Run(p, func(t *testing.T) {
			path := filepath.Join(testdataDir, p)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read plan: %v", err)
			}

			// Parse as generic map to verify JSON structure
			var plan map[string]interface{}
			if err := json.Unmarshal(data, &plan); err != nil {
				t.Fatalf("failed to parse plan: %v", err)
			}

			// Verify required fields
			if plan["id"] == nil {
				t.Error("plan missing 'id' field")
			}
			if plan["description"] == nil {
				t.Error("plan missing 'description' field")
			}
			if plan["steps"] == nil && plan["checks"] == nil {
				t.Error("plan missing both 'steps' and 'checks' fields")
			}
		})
	}
}

// TestIRLossClassValues verifies loss class values in fixtures.
func TestIRLossClassValues(t *testing.T) {
	irFixtures := []string{
		"fixtures/ir/osis/sample.ir.json",
		"fixtures/ir/usfm/sample.ir.json",
		"fixtures/ir/usx/sample.ir.json",
		"fixtures/ir/zefania/sample.ir.json",
		"fixtures/ir/theword/sample.ir.json",
	}

	for _, f := range irFixtures {
		t.Run(f, func(t *testing.T) {
			path := filepath.Join(testdataDir, f)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var corpus ir.Corpus
			if err := json.Unmarshal(data, &corpus); err != nil {
				t.Fatalf("failed to parse IR fixture: %v", err)
			}

			// Verify loss class is valid
			validClasses := map[ir.LossClass]bool{
				ir.LossL0: true,
				ir.LossL1: true,
				ir.LossL2: true,
				ir.LossL3: true,
				ir.LossL4: true,
			}

			if corpus.LossClass != "" && !validClasses[corpus.LossClass] {
				t.Errorf("invalid loss class: %s", corpus.LossClass)
			}
		})
	}
}
