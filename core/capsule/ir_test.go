package capsule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/ir"
)

func TestIRRecordJSON(t *testing.T) {
	record := &IRRecord{
		ID:               "ir-kjv-1",
		SourceArtifactID: "kjv-module",
		IRBlobSHA256:     "abc123def456",
		IRFormat:         "ir-v1",
		IRVersion:        "1.0.0",
		LossClass:        "L1",
		ExtractorPlugin:  "format.sword",
	}

	// Marshal to JSON
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded IRRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify fields
	if decoded.ID != record.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, record.ID)
	}
	if decoded.SourceArtifactID != record.SourceArtifactID {
		t.Errorf("SourceArtifactID = %q, want %q", decoded.SourceArtifactID, record.SourceArtifactID)
	}
	if decoded.IRBlobSHA256 != record.IRBlobSHA256 {
		t.Errorf("IRBlobSHA256 = %q, want %q", decoded.IRBlobSHA256, record.IRBlobSHA256)
	}
	if decoded.LossClass != record.LossClass {
		t.Errorf("LossClass = %q, want %q", decoded.LossClass, record.LossClass)
	}
}

func TestIRRecordWithLossReport(t *testing.T) {
	record := &IRRecord{
		ID:               "ir-kjv-1",
		SourceArtifactID: "kjv-module",
		IRBlobSHA256:     "abc123def456",
		LossReport: &ir.LossReport{
			SourceFormat: "SWORD",
			TargetFormat: "IR",
			LossClass:    ir.LossL1,
			LostElements: []ir.LostElement{
				{Path: "Gen.1.1/format", ElementType: "formatting", Reason: "Not preserved"},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded IRRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify loss report
	if decoded.LossReport == nil {
		t.Fatal("LossReport is nil")
	}
	if decoded.LossReport.LossClass != ir.LossL1 {
		t.Errorf("LossClass = %q, want %q", decoded.LossReport.LossClass, ir.LossL1)
	}
	if len(decoded.LossReport.LostElements) != 1 {
		t.Errorf("len(LostElements) = %d, want 1", len(decoded.LossReport.LostElements))
	}
}

func TestManifestWithIRExtractions(t *testing.T) {
	m := NewManifest()
	m.IRExtractions = map[string]*IRRecord{
		"ir-kjv-1": {
			ID:               "ir-kjv-1",
			SourceArtifactID: "kjv-module",
			IRBlobSHA256:     "abc123",
		},
	}

	// Marshal to JSON
	data, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Unmarshal back
	decoded, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	// Verify IR extractions
	if decoded.IRExtractions == nil {
		t.Fatal("IRExtractions is nil")
	}
	if len(decoded.IRExtractions) != 1 {
		t.Errorf("len(IRExtractions) = %d, want 1", len(decoded.IRExtractions))
	}
	if decoded.IRExtractions["ir-kjv-1"].SourceArtifactID != "kjv-module" {
		t.Errorf("SourceArtifactID = %q, want %q",
			decoded.IRExtractions["ir-kjv-1"].SourceArtifactID, "kjv-module")
	}
}

func TestCapsuleStoreIR(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "capsule-ir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a capsule
	c, err := Create(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create capsule: %v", err)
	}

	// Create a simple corpus
	corpus := &ir.Corpus{
		ID:         "KJV",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Title:      "King James Version",
		Language:   "en",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:   "cb1",
						Text: "In the beginning God created the heaven and the earth.",
					},
				},
			},
		},
	}

	// Store the IR
	artifact, err := c.StoreIR(corpus, "kjv-module")
	if err != nil {
		t.Fatalf("StoreIR failed: %v", err)
	}

	// Verify artifact
	if artifact == nil {
		t.Fatal("artifact is nil")
	}
	if artifact.Kind != ArtifactKindIR {
		t.Errorf("Kind = %q, want %q", artifact.Kind, ArtifactKindIR)
	}
	if artifact.PrimaryBlobSHA256 == "" {
		t.Error("PrimaryBlobSHA256 is empty")
	}

	// Verify IR record was created
	if c.Manifest.IRExtractions == nil {
		t.Fatal("IRExtractions is nil")
	}
	if len(c.Manifest.IRExtractions) != 1 {
		t.Errorf("len(IRExtractions) = %d, want 1", len(c.Manifest.IRExtractions))
	}
}

func TestCapsuleLoadIR(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "capsule-ir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a capsule
	c, err := Create(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create capsule: %v", err)
	}

	// Create a simple corpus
	corpus := &ir.Corpus{
		ID:         "KJV",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Title:      "King James Version",
		Language:   "en",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
			},
		},
	}

	// Store the IR
	artifact, err := c.StoreIR(corpus, "kjv-module")
	if err != nil {
		t.Fatalf("StoreIR failed: %v", err)
	}

	// Load the IR
	loaded, err := c.LoadIR(artifact.ID)
	if err != nil {
		t.Fatalf("LoadIR failed: %v", err)
	}

	// Verify corpus
	if loaded == nil {
		t.Fatal("loaded corpus is nil")
	}
	if loaded.ID != corpus.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, corpus.ID)
	}
	if loaded.Title != corpus.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, corpus.Title)
	}
	if len(loaded.Documents) != 1 {
		t.Errorf("len(Documents) = %d, want 1", len(loaded.Documents))
	}
	if loaded.Documents[0].ID != "Gen" {
		t.Errorf("Documents[0].ID = %q, want %q", loaded.Documents[0].ID, "Gen")
	}
}

func TestCapsuleIRRoundTrip(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "capsule-ir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a capsule
	c, err := Create(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create capsule: %v", err)
	}

	// Create a complex corpus
	corpus := &ir.Corpus{
		ID:            "KJV",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "King James Version",
		LossClass:     ir.LossL0,
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "cb1",
						Sequence: 0,
						Text:     "In the beginning God created the heaven and the earth.",
					},
					{
						ID:       "cb2",
						Sequence: 1,
						Text:     "And the earth was without form, and void.",
					},
				},
			},
			{
				ID:    "Exod",
				Title: "Exodus",
				Order: 2,
			},
		},
	}

	// Compute hashes
	ir.ComputeAllHashes(corpus)

	// Store the IR
	artifact, err := c.StoreIR(corpus, "kjv-module")
	if err != nil {
		t.Fatalf("StoreIR failed: %v", err)
	}

	// Load the IR
	loaded, err := c.LoadIR(artifact.ID)
	if err != nil {
		t.Fatalf("LoadIR failed: %v", err)
	}

	// Verify complete round-trip
	if loaded.ID != corpus.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, corpus.ID)
	}
	if loaded.ModuleType != corpus.ModuleType {
		t.Errorf("ModuleType = %q, want %q", loaded.ModuleType, corpus.ModuleType)
	}
	if loaded.Versification != corpus.Versification {
		t.Errorf("Versification = %q, want %q", loaded.Versification, corpus.Versification)
	}
	if loaded.LossClass != corpus.LossClass {
		t.Errorf("LossClass = %q, want %q", loaded.LossClass, corpus.LossClass)
	}
	if len(loaded.Documents) != 2 {
		t.Fatalf("len(Documents) = %d, want 2", len(loaded.Documents))
	}
	if len(loaded.Documents[0].ContentBlocks) != 2 {
		t.Errorf("len(Documents[0].ContentBlocks) = %d, want 2",
			len(loaded.Documents[0].ContentBlocks))
	}

	// Verify hashes are preserved
	if loaded.Documents[0].ContentBlocks[0].Hash !=
		corpus.Documents[0].ContentBlocks[0].Hash {
		t.Error("ContentBlock hash not preserved")
	}
}

func TestCapsuleIRDeterminism(t *testing.T) {
	// Create same corpus twice and verify deterministic hashing
	corpus := &ir.Corpus{
		ID:         "test",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
		Title:      "Test",
	}

	// Create two capsules with same corpus
	var hashes []string
	for i := 0; i < 2; i++ {
		tmpDir, err := os.MkdirTemp("", "capsule-ir-test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		c, err := Create(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create capsule: %v", err)
		}

		artifact, err := c.StoreIR(corpus, "source")
		if err != nil {
			t.Fatalf("StoreIR failed: %v", err)
		}

		hashes = append(hashes, artifact.PrimaryBlobSHA256)
	}

	// Hashes should be identical
	if hashes[0] != hashes[1] {
		t.Errorf("non-deterministic hashing: %q vs %q", hashes[0], hashes[1])
	}
}

func TestCapsuleLoadIRNotFound(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "capsule-ir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a capsule
	c, err := Create(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create capsule: %v", err)
	}

	// Try to load non-existent IR
	_, err = c.LoadIR("nonexistent")
	if err == nil {
		t.Error("LoadIR should return error for non-existent artifact")
	}
}

func TestCapsulePackWithIR(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "capsule-ir-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a capsule
	c, err := Create(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create capsule: %v", err)
	}

	// Store IR
	corpus := &ir.Corpus{
		ID:         "KJV",
		Version:    "1.0.0",
		ModuleType: ir.ModuleBible,
	}
	_, err = c.StoreIR(corpus, "source")
	if err != nil {
		t.Fatalf("StoreIR failed: %v", err)
	}

	// Pack the capsule
	outPath := filepath.Join(tmpDir, "test.capsule.tar.xz")
	if err := c.Pack(outPath); err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("Packed capsule file not found")
	}
}
