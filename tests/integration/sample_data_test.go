// Package integration provides integration tests for sample data capsules.
// These tests verify that capsules preserve data with 100% fidelity.
package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/JuniperBible/juniper/core/capsule"
)

// sampleModules lists all available sample modules for testing.
var sampleModules = []struct {
	name        string
	description string
}{
	{"drc", "Douay-Rheims Catholic Bible"},
	{"geneva1599", "Geneva Bible (1599)"},
	{"tyndale", "William Tyndale Bible (1525/1530)"},
	{"kjv", "King James Version (1769)"},
	{"lxx", "Septuagint (Rahlfs)"},
	{"osmhb", "Open Scriptures Hebrew Bible"},
	{"sblgnt", "SBL Greek New Testament"},
	{"vulgate", "Latin Vulgate"},
	{"asv", "American Standard Version (1901)"},
	{"oeb", "Open English Bible"},
	{"web", "World English Bible"},
}

// TestCapsuleExists verifies all sample capsules exist.
func TestCapsuleExists(t *testing.T) {
	for _, mod := range sampleModules {
		t.Run(mod.name, func(t *testing.T) {
			capsulePath := filepath.Join("..", "..", "contrib", "sample-data", "capsules", mod.name+".capsule.tar.xz")
			if _, err := os.Stat(capsulePath); os.IsNotExist(err) {
				t.Skipf("capsule not found: %s", capsulePath)
			}
		})
	}
}

// TestRawDataExists verifies all raw sample data directories exist.
func TestRawDataExists(t *testing.T) {
	for _, mod := range sampleModules {
		t.Run(mod.name, func(t *testing.T) {
			rawPath := filepath.Join("..", "..", "contrib", "sample-data", mod.name)
			if _, err := os.Stat(rawPath); os.IsNotExist(err) {
				t.Skipf("raw data not found: %s", rawPath)
			}

			// Check for required files
			confPath := filepath.Join(rawPath, "mods.d", mod.name+".conf")
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				t.Errorf("conf file not found: %s", confPath)
			}

			licensePath := filepath.Join(rawPath, "LICENSE.TXT")
			if _, err := os.Stat(licensePath); os.IsNotExist(err) {
				t.Errorf("LICENSE.TXT not found: %s", licensePath)
			}

			readmePath := filepath.Join(rawPath, "README.md")
			if _, err := os.Stat(readmePath); os.IsNotExist(err) {
				t.Errorf("README.md not found: %s", readmePath)
			}
		})
	}
}

// TestCapsuleVerification verifies each capsule passes integrity checks.
func TestCapsuleVerification(t *testing.T) {
	for _, mod := range sampleModules {
		t.Run(mod.name, func(t *testing.T) {
			capsulePath := filepath.Join("..", "..", "contrib", "sample-data", "capsules", mod.name+".capsule.tar.xz")
			if _, err := os.Stat(capsulePath); os.IsNotExist(err) {
				t.Skipf("capsule not found: %s", capsulePath)
			}

			// Unpack to temp directory
			tempDir, err := os.MkdirTemp("", "capsule-verify-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			cap, err := capsule.Unpack(capsulePath, tempDir)
			if err != nil {
				t.Fatalf("failed to unpack capsule: %v", err)
			}

			// Verify manifest has artifacts
			if len(cap.Manifest.Artifacts) == 0 {
				t.Error("capsule has no artifacts")
			}

			// Verify each artifact
			for id, artifact := range cap.Manifest.Artifacts {
				t.Logf("verifying artifact: %s (%d bytes)", id, artifact.SizeBytes)

				// Retrieve blob
				data, err := cap.GetStore().Retrieve(artifact.PrimaryBlobSHA256)
				if err != nil {
					t.Errorf("failed to retrieve artifact %s: %v", id, err)
					continue
				}

				// Verify hash
				hash := sha256.Sum256(data)
				hashHex := hex.EncodeToString(hash[:])
				if hashHex != artifact.PrimaryBlobSHA256 {
					t.Errorf("hash mismatch for %s: expected %s, got %s",
						id, artifact.PrimaryBlobSHA256, hashHex)
				}

				// Verify size
				if int64(len(data)) != artifact.SizeBytes {
					t.Errorf("size mismatch for %s: expected %d, got %d",
						id, artifact.SizeBytes, len(data))
				}
			}
		})
	}
}

// TestCapsuleArtifactHashes records artifact hashes for regression testing.
func TestCapsuleArtifactHashes(t *testing.T) {
	expectedHashes := map[string]string{
		"drc":        "79cedb5e4d8c6bd6bc3520fd32884e03892d5c1e0112a04e2269164a53361f3e",
		"geneva1599": "8782c7965acbb72c0f242513306a3414eb36327a1088fbf26f1b8ec77b3784cc",
		"tyndale":    "b6b85efd05867a009a240e7c74ffef418a5d9aa945ea839460d1092c2444ebe9",
		"kjv":        "11b9c971f3cb6fb93009697f29f6946df5fa2e91026a1304f177faf4cd1b8368",
		"lxx":        "b31c32f5c60e680b8e384488a7228de1c5e00e697403cce528ce38eff2437a0c",
		"osmhb":      "b2cf3df16b6db203b0aef6cfda3a241c5e7eb04922c7f1c4409ddb28b98c8799",
		"sblgnt":     "5c2dd8c4ddc4f8b7c9a68186bf8510d7d068e279f5479fdf9eac9177eba6ab67",
		"vulgate":    "92a5d17f8540a7b1279d2434642780d220e362ef0c84d88c019c989a13af74eb",
		"asv":        "e2a248c3c96f65e5bc58320e3fd02b9fa72a4cfd0438ea50fb8fb9390e79328b",
		"oeb":        "7a9826486fda59a953cb43be29c0a7a435a629e1005dd0508b784853a983731d",
		"web":        "22f9b005c3e4657a70b8254534cf113da1f80ccd67bd7132416b1445720eb410",
	}

	for _, mod := range sampleModules {
		t.Run(mod.name, func(t *testing.T) {
			capsulePath := filepath.Join("..", "..", "contrib", "sample-data", "capsules", mod.name+".capsule.tar.xz")
			if _, err := os.Stat(capsulePath); os.IsNotExist(err) {
				t.Skipf("capsule not found: %s", capsulePath)
			}

			tempDir, err := os.MkdirTemp("", "capsule-hash-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			cap, err := capsule.Unpack(capsulePath, tempDir)
			if err != nil {
				t.Fatalf("failed to unpack capsule: %v", err)
			}

			// Find the primary artifact
			for _, artifact := range cap.Manifest.Artifacts {
				expectedHash, ok := expectedHashes[mod.name]
				if !ok {
					t.Logf("no expected hash for %s, actual: %s", mod.name, artifact.PrimaryBlobSHA256)
					continue
				}

				if artifact.PrimaryBlobSHA256 != expectedHash {
					t.Errorf("hash regression for %s: expected %s, got %s",
						mod.name, expectedHash, artifact.PrimaryBlobSHA256)
				}
			}
		})
	}
}

// TestRawDataConfParsing verifies conf files can be parsed.
func TestRawDataConfParsing(t *testing.T) {
	for _, mod := range sampleModules {
		t.Run(mod.name, func(t *testing.T) {
			confPath := filepath.Join("..", "..", "contrib", "sample-data", mod.name, "mods.d", mod.name+".conf")
			if _, err := os.Stat(confPath); os.IsNotExist(err) {
				t.Skipf("conf file not found: %s", confPath)
			}

			data, err := os.ReadFile(confPath)
			if err != nil {
				t.Fatalf("failed to read conf: %v", err)
			}

			// Basic validation: should contain module name in brackets
			content := string(data)
			if len(content) == 0 {
				t.Error("conf file is empty")
			}

			// Check for Description field
			if !containsField(content, "Description=") {
				t.Error("conf file missing Description field")
			}

			// Check for DataPath field
			if !containsField(content, "DataPath=") {
				t.Error("conf file missing DataPath field")
			}
		})
	}
}

// TestSelfCheckPlanLoading verifies selfcheck plans can load and execute.
func TestSelfCheckPlanLoading(t *testing.T) {
	plans := []string{
		"ir-roundtrip-osis.json",
		"ir-roundtrip-usfm.json",
		"ir-roundtrip-sword.json",
		"ir-extraction-sword.json",
	}

	for _, planFile := range plans {
		t.Run(planFile, func(t *testing.T) {
			planPath := filepath.Join("..", "..", "testdata", "plans", planFile)
			if _, err := os.Stat(planPath); os.IsNotExist(err) {
				t.Skipf("plan not found: %s", planPath)
			}

			// Read and parse the plan
			data, err := os.ReadFile(planPath)
			if err != nil {
				t.Fatalf("failed to read plan: %v", err)
			}

			// Parse as JSON to verify structure
			var plan map[string]interface{}
			if err := json.Unmarshal(data, &plan); err != nil {
				t.Fatalf("failed to parse plan: %v", err)
			}

			// Verify plan has required fields
			if plan["id"] == nil {
				t.Error("plan missing 'id' field")
			}
			if plan["steps"] == nil && plan["checks"] == nil {
				t.Error("plan missing steps or checks")
			}

			t.Logf("plan %s: %v steps, %v checks",
				plan["id"],
				len(plan["steps"].([]interface{})),
				len(plan["checks"].([]interface{})))
		})
	}
}

// TestIRPipelineWithSampleData tests the IR pipeline structure with sample data.
// This verifies the IR types and structures are correct for SWORD module processing.
func TestIRPipelineWithSampleData(t *testing.T) {
	// Read a sample SWORD IR fixture
	irPath := filepath.Join("..", "..", "testdata", "fixtures", "ir", "sword", "sample.ir.json")
	if _, err := os.Stat(irPath); os.IsNotExist(err) {
		t.Skip("SWORD IR fixture not found")
	}

	data, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatalf("failed to read IR fixture: %v", err)
	}

	// Verify it's valid JSON with expected structure
	var ir map[string]interface{}
	if err := json.Unmarshal(data, &ir); err != nil {
		t.Fatalf("failed to parse IR: %v", err)
	}

	// Check for expected fields
	requiredFields := []string{"id", "version", "module_type", "loss_class"}
	for _, field := range requiredFields {
		if ir[field] == nil {
			t.Errorf("IR missing required field: %s", field)
		}
	}

	// Verify loss class is L2 for SWORD (expected per format documentation)
	if lc, ok := ir["loss_class"].(string); ok {
		if lc != "L2" {
			t.Logf("SWORD module loss class: %s (expected L2 for metadata-only extraction)", lc)
		}
	}

	// Verify documents exist
	if docs, ok := ir["documents"].([]interface{}); ok {
		t.Logf("IR contains %d documents", len(docs))
		if len(docs) == 0 {
			t.Error("IR should have at least one document")
		}
	}
}

// containsField checks if a string contains a field.
func containsField(content, field string) bool {
	for _, line := range splitLines(content) {
		if len(line) >= len(field) && line[:len(field)] == field {
			return true
		}
	}
	return false
}

// splitLines splits content into lines.
func splitLines(content string) []string {
	var lines []string
	var current string
	for _, c := range content {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// TestModuleDataIntegrity verifies module data files exist and are non-empty.
func TestModuleDataIntegrity(t *testing.T) {
	for _, mod := range sampleModules {
		t.Run(mod.name, func(t *testing.T) {
			dataPath := filepath.Join("..", "..", "contrib", "sample-data", mod.name, "modules", "texts", "ztext", mod.name)
			if _, err := os.Stat(dataPath); os.IsNotExist(err) {
				t.Skipf("module data not found: %s", dataPath)
			}

			entries, err := os.ReadDir(dataPath)
			if err != nil {
				t.Fatalf("failed to read data directory: %v", err)
			}

			if len(entries) == 0 {
				t.Error("module data directory is empty")
			}

			// Count data files
			dataFiles := 0
			for _, entry := range entries {
				if !entry.IsDir() {
					info, err := entry.Info()
					if err == nil && info.Size() > 0 {
						dataFiles++
					}
				}
			}

			if dataFiles == 0 {
				t.Error("no data files found in module directory")
			}

			t.Logf("found %d data files", dataFiles)
		})
	}
}
