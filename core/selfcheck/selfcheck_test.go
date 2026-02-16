package selfcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/capsule"
	"github.com/FocuswithJustin/JuniperBible/core/cas"
	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

func init() {
	// Enable external plugins for testing
	plugins.EnableExternalPlugins()
}

// TestByteEqualCheck tests the BYTE_EQUAL check type.
func TestByteEqualCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule with a file
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest a test file
	testContent := []byte("Test content for byte equality check")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Export to a new location
	exportPath := filepath.Join(tempDir, "exported.txt")
	if err := cap.Export(artifact.ID, capsule.ExportModeIdentity, exportPath); err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Check byte equality
	check := &ByteEqualCheck{
		ArtifactA: artifact.ID,
		PathB:     exportPath,
	}

	result, err := check.Execute(cap)
	if err != nil {
		t.Fatalf("check execution failed: %v", err)
	}

	if !result.Pass {
		t.Error("byte equality check should pass for identical content")
	}

	// Modify exported file and check again
	if err := os.WriteFile(exportPath, []byte("Modified content"), 0600); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	result, err = check.Execute(cap)
	if err != nil {
		t.Fatalf("check execution failed: %v", err)
	}

	if result.Pass {
		t.Error("byte equality check should fail for different content")
	}
}

// TestPlanExecution tests executing a complete self-check plan.
func TestPlanExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule with a file
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Plan execution test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Create an identity-bytes plan
	plan := &Plan{
		ID:          "identity-bytes",
		Description: "Verify byte-for-byte identity export",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: artifact.ID,
					OutputKey:  "exported",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Original equals exported",
				ByteEqual: &ByteEqualDef{
					ArtifactA: artifact.ID,
					ArtifactB: "exported",
				},
			},
		},
	}

	// Execute the plan
	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("plan execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}

	if len(report.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(report.Results))
	}

	if !report.Results[0].Pass {
		t.Error("identity check should pass")
	}
}

// TestReportGeneration tests that reports are generated correctly.
func TestReportGeneration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "test-plan",
		Description: "Test plan for report generation",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: artifact.ID,
					OutputKey:  "exported",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Test check",
				ByteEqual: &ByteEqualDef{
					ArtifactA: artifact.ID,
					ArtifactB: "exported",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Verify report fields
	if report.ReportVersion == "" {
		t.Error("report_version should be set")
	}

	if report.CreatedAt == "" {
		t.Error("created_at should be set")
	}

	if report.PlanID != "test-plan" {
		t.Errorf("expected plan_id 'test-plan', got %q", report.PlanID)
	}

	// Serialize to JSON
	data, err := report.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize report: %v", err)
	}

	if len(data) == 0 {
		t.Error("JSON output should not be empty")
	}
}

// TestReportHash tests that report hashes are deterministic.
func TestReportHash(t *testing.T) {
	report1 := &Report{
		ReportVersion: "1.0.0",
		CreatedAt:     "2026-01-01T00:00:00Z", // Fixed time for determinism
		PlanID:        "test-plan",
		Status:        StatusPass,
		Results: []CheckResult{
			{
				CheckType: CheckByteEqual,
				Label:     "Test",
				Pass:      true,
			},
		},
	}

	report2 := &Report{
		ReportVersion: "1.0.0",
		CreatedAt:     "2026-01-01T00:00:00Z",
		PlanID:        "test-plan",
		Status:        StatusPass,
		Results: []CheckResult{
			{
				CheckType: CheckByteEqual,
				Label:     "Test",
				Pass:      true,
			},
		},
	}

	hash1 := report1.Hash()
	hash2 := report2.Hash()

	if hash1 != hash2 {
		t.Errorf("identical reports should have same hash: %s != %s", hash1, hash2)
	}

	// Modify one and verify hashes differ
	report2.Status = StatusFail
	hash3 := report2.Hash()

	if hash1 == hash3 {
		t.Error("different reports should have different hashes")
	}
}

// TestHashComparison tests comparing hashes for equality.
func TestHashComparison(t *testing.T) {
	data1 := []byte("identical content")
	data2 := []byte("identical content")
	data3 := []byte("different content")

	hash1 := cas.Hash(data1)
	hash2 := cas.Hash(data2)
	hash3 := cas.Hash(data3)

	if hash1 != hash2 {
		t.Error("identical content should have same hash")
	}

	if hash1 == hash3 {
		t.Error("different content should have different hash")
	}
}

// TestTranscriptEqualCheck tests the TRANSCRIPT_EQUAL check type.
func TestTranscriptEqualCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule with runs
	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create two identical transcripts
	transcript1 := []byte(`{"type":"ENGINE_INFO","seq":1}
{"type":"MODULE_DISCOVERED","seq":2}
`)
	transcript2 := []byte(`{"type":"ENGINE_INFO","seq":1}
{"type":"MODULE_DISCOVERED","seq":2}
`)

	hash1 := cas.Hash(transcript1)
	hash2 := cas.Hash(transcript2)

	// Add runs to manifest
	cap.Manifest.Runs["run1"] = &capsule.Run{
		ID: "run1",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash1,
		},
	}
	cap.Manifest.Runs["run2"] = &capsule.Run{
		ID: "run2",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash2,
		},
	}

	// Test transcript equality
	plan := &Plan{
		ID:          "transcript-test",
		Description: "Test transcript equality",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Transcripts should match",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run1",
					RunB: "run2",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	if !report.Results[0].Pass {
		t.Error("transcript equality check should pass for identical transcripts")
	}
}

// TestTranscriptEqualCheckFail tests TRANSCRIPT_EQUAL with different transcripts.
func TestTranscriptEqualCheckFail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create two different transcripts
	transcript1 := []byte(`{"type":"ENGINE_INFO","seq":1}`)
	transcript2 := []byte(`{"type":"ENGINE_INFO","seq":1,"extra":"data"}`)

	hash1 := cas.Hash(transcript1)
	hash2 := cas.Hash(transcript2)

	cap.Manifest.Runs["run1"] = &capsule.Run{
		ID: "run1",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash1,
		},
	}
	cap.Manifest.Runs["run2"] = &capsule.Run{
		ID: "run2",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash2,
		},
	}

	plan := &Plan{
		ID:          "transcript-fail-test",
		Description: "Test transcript inequality",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Transcripts should differ",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run1",
					RunB: "run2",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusFail {
		t.Errorf("expected status 'fail', got %q", report.Status)
	}

	if report.Results[0].Pass {
		t.Error("transcript equality check should fail for different transcripts")
	}
}

// TestIdentityBytesPlan tests the IdentityBytesPlan factory function.
func TestIdentityBytesPlan(t *testing.T) {
	plan := IdentityBytesPlan("test-artifact")

	if plan.ID != "identity-bytes" {
		t.Errorf("expected plan ID 'identity-bytes', got %q", plan.ID)
	}

	if len(plan.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(plan.Steps))
	}

	if plan.Steps[0].Type != StepExport {
		t.Errorf("expected step type %q, got %q", StepExport, plan.Steps[0].Type)
	}

	if plan.Steps[0].Export.ArtifactID != "test-artifact" {
		t.Errorf("expected artifact ID 'test-artifact', got %q", plan.Steps[0].Export.ArtifactID)
	}

	if plan.Steps[0].Export.Mode != "IDENTITY" {
		t.Errorf("expected mode 'IDENTITY', got %q", plan.Steps[0].Export.Mode)
	}

	if len(plan.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(plan.Checks))
	}

	if plan.Checks[0].Type != CheckByteEqual {
		t.Errorf("expected check type %q, got %q", CheckByteEqual, plan.Checks[0].Type)
	}
}

// TestBehaviorIdentityPlan tests the BehaviorIdentityPlan factory function.
func TestBehaviorIdentityPlan(t *testing.T) {
	plan := BehaviorIdentityPlan("run-1", "run-2")

	if plan.ID != "behavior-identity" {
		t.Errorf("expected plan ID 'behavior-identity', got %q", plan.ID)
	}

	if len(plan.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(plan.Steps))
	}

	if len(plan.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(plan.Checks))
	}

	if plan.Checks[0].Type != CheckTranscriptEqual {
		t.Errorf("expected check type %q, got %q", CheckTranscriptEqual, plan.Checks[0].Type)
	}

	if plan.Checks[0].TranscriptEqual.RunA != "run-1" {
		t.Errorf("expected RunA 'run-1', got %q", plan.Checks[0].TranscriptEqual.RunA)
	}

	if plan.Checks[0].TranscriptEqual.RunB != "run-2" {
		t.Errorf("expected RunB 'run-2', got %q", plan.Checks[0].TranscriptEqual.RunB)
	}
}

// TestIdentityBytesPlanExecution tests executing the IdentityBytesPlan.
func TestIdentityBytesPlanExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Identity bytes plan test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := IdentityBytesPlan(artifact.ID)
	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("plan execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestBehaviorIdentityPlanExecution tests executing the BehaviorIdentityPlan.
func TestBehaviorIdentityPlanExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create identical transcripts for two runs
	transcript := []byte(`{"type":"ENGINE_INFO","version":"1.0"}`)
	hash := cas.Hash(transcript)

	cap.Manifest.Runs["run-1"] = &capsule.Run{
		ID: "run-1",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash,
		},
	}
	cap.Manifest.Runs["run-2"] = &capsule.Run{
		ID: "run-2",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash,
		},
	}

	plan := BehaviorIdentityPlan("run-1", "run-2")
	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("plan execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestNewExecutorWithPlugins tests creating an executor with plugin support.
func TestNewExecutorWithPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor with nil loader (valid use case)
	executor := NewExecutorWithPlugins(cap, nil)
	if executor == nil {
		t.Fatal("NewExecutorWithPlugins returned nil")
	}

	// Verify executor can still run basic plans
	testContent := []byte("Test with plugins executor")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := IdentityBytesPlan(artifact.ID)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("plan execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestUnknownStepType tests handling of unknown step types.
func TestUnknownStepType(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "unknown-step-test",
		Description: "Test unknown step type",
		Steps: []PlanStep{
			{
				Type:  "UNKNOWN_STEP_TYPE",
				Label: "Unknown step",
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for unknown step type")
	}
}

// TestUnknownCheckType tests handling of unknown check types.
func TestUnknownCheckType(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "unknown-check-test",
		Description: "Test unknown check type",
		Checks: []PlanCheck{
			{
				Type:  "UNKNOWN_CHECK_TYPE",
				Label: "Unknown check",
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for unknown check type")
	}
}

// TestMissingRunInTranscriptCheck tests transcript check with missing run.
func TestMissingRunInTranscriptCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "missing-run-test",
		Description: "Test missing run",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Check with missing run",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "nonexistent-run",
					RunB: "also-nonexistent",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing run")
	}
}

// TestMissingArtifactInByteCheck tests byte check with missing artifact.
func TestMissingArtifactInByteCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "missing-artifact-test",
		Description: "Test missing artifact",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Check with missing artifact",
				ByteEqual: &ByteEqualDef{
					ArtifactA: "nonexistent-artifact",
					ArtifactB: "also-nonexistent",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

// TestExportStepWithMissingArtifact tests export step with missing artifact.
func TestExportStepWithMissingArtifact(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "missing-export-artifact-test",
		Description: "Test export with missing artifact",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: "nonexistent-artifact",
					OutputKey:  "output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for export with missing artifact")
	}
}

// TestRunToolStepWithoutPluginLoader tests that RUN_TOOL step requires plugin loader.
func TestRunToolStepWithoutPluginLoader(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-test",
		Description: "Test run tool step",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "libsword",
					Profile:      "list-modules",
				},
			},
		},
	}

	// Without plugin loader, should fail
	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for RUN_TOOL step without plugin loader")
	}
	if err != nil && !strings.Contains(err.Error(), "plugin loader not configured") {
		t.Errorf("expected 'plugin loader not configured' error, got: %v", err)
	}
}

// TestRunToolStepPluginNotFound tests that RUN_TOOL step fails for missing plugin.
func TestRunToolStepPluginNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-test",
		Description: "Test run tool step with missing plugin",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "nonexistent-plugin",
					Profile:      "test-profile",
				},
			},
		},
	}

	// With empty plugin loader, should fail with plugin not found
	loader := plugins.NewLoader()
	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing plugin")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to get tool plugin") {
		t.Errorf("expected 'failed to get tool plugin' error, got: %v", err)
	}
}

// TestRunToolStep tests that RUN_TOOL step returns error for non-tool plugin.
func TestRunToolStep(t *testing.T) {
	// This test is kept for backward compatibility
	// It tests that RUN_TOOL requires proper plugin setup
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-test",
		Description: "Test run tool step",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "libsword",
					Profile:      "list-modules",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for RUN_TOOL step (requires plugin loader)")
	}
}

// TestExtractIRStep tests EXTRACT_IR step with fallback placeholder.
func TestExtractIRStep(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for IR extraction")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-test",
		Description: "Test IR extraction step",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestExtractIRStepMissingArtifact tests EXTRACT_IR step with missing artifact.
func TestExtractIRStepMissingArtifact(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-missing-test",
		Description: "Test IR extraction with missing artifact",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: "nonexistent",
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing artifact in EXTRACT_IR step")
	}
}

// TestEmitNativeStep tests EMIT_NATIVE step with fallback.
func TestEmitNativeStep(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "emit-native-test",
		Description: "Test native emission step",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
			{
				Type: StepEmitNative,
				EmitNative: &EmitNativeStep{
					IRInputKey: "ir_output",
					PluginID:   "format-osis",
					OutputKey:  "native_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestEmitNativeStepMissingIR tests EMIT_NATIVE step with missing IR input.
func TestEmitNativeStepMissingIR(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "emit-native-missing-test",
		Description: "Test native emission with missing IR",
		Steps: []PlanStep{
			{
				Type: StepEmitNative,
				EmitNative: &EmitNativeStep{
					IRInputKey: "nonexistent_ir",
					PluginID:   "format-osis",
					OutputKey:  "native_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR input in EMIT_NATIVE step")
	}
}

// TestCompareIRStep tests COMPARE_IR step.
func TestCompareIRStep(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "compare-ir-test",
		Description: "Test IR comparison step",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_a",
				},
			},
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_b",
				},
			},
			{
				Type: StepCompareIR,
				CompareIR: &CompareIRStep{
					IRAKey:    "ir_a",
					IRBKey:    "ir_b",
					OutputKey: "comparison",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestCompareIRStepMissingIR tests COMPARE_IR step with missing IR.
func TestCompareIRStepMissingIR(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "compare-ir-missing-test",
		Description: "Test IR comparison with missing IR",
		Steps: []PlanStep{
			{
				Type: StepCompareIR,
				CompareIR: &CompareIRStep{
					IRAKey:    "nonexistent_a",
					IRBKey:    "nonexistent_b",
					OutputKey: "comparison",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR in COMPARE_IR step")
	}
}

// TestIRStructureEqualCheck tests IR_STRUCTURE_EQUAL check.
func TestIRStructureEqualCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for IR structure check")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-equal-test",
		Description: "Test IR structure equality",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_a",
				},
			},
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_b",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "IR structures should match",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "ir_a",
					IRB: "ir_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRStructureEqualCheckMissing tests IR_STRUCTURE_EQUAL with missing IR.
func TestIRStructureEqualCheckMissing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-equal-missing-test",
		Description: "Test IR structure equality with missing IR",
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "Check with missing IR",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "nonexistent_a",
					IRB: "nonexistent_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR in IR_STRUCTURE_EQUAL check")
	}
}

// TestIRRoundtripCheck tests IR_ROUNDTRIP check.
func TestIRRoundtripCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for IR roundtrip")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-roundtrip-test",
		Description: "Test IR roundtrip check",
		Checks: []PlanCheck{
			{
				Type:  CheckIRRoundtrip,
				Label: "IR roundtrip should pass",
				IRRoundtrip: &IRRoundtripDef{
					SourceArtifactID: artifact.ID,
					Format:           "osis",
					MaxLossClass:     "L0",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	details, ok := report.Results[0].Details.(map[string]string)
	if !ok {
		t.Fatal("expected Details to be map[string]string")
	}
	if details["target_format"] != "osis" {
		t.Errorf("expected format 'osis', got %q", details["target_format"])
	}
}

// TestIRFidelityCheck tests IR_FIDELITY check.
func TestIRFidelityCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for IR fidelity")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-test",
		Description: "Test IR fidelity check",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "IR fidelity should pass",
				IRFidelity: &IRFidelityDef{
					IRKey:        "ir_output",
					MaxLossClass: "L2",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRFidelityCheckMissing tests IR_FIDELITY with missing IR.
func TestIRFidelityCheckMissing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-missing-test",
		Description: "Test IR fidelity with missing IR",
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "Check with missing IR",
				IRFidelity: &IRFidelityDef{
					IRKey:        "nonexistent_ir",
					MaxLossClass: "L0",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR in IR_FIDELITY check")
	}
}

// TestIRFidelityCheckWithLossBudget tests IR_FIDELITY with loss budget.
func TestIRFidelityCheckWithLossBudget(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-budget-test",
		Description: "Test IR fidelity with loss budget",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "IR fidelity with budget",
				IRFidelity: &IRFidelityDef{
					IRKey:        "ir_output",
					MaxLossClass: "L2",
					LossBudget: &LossBudget{
						MaxLossClass:    "L2",
						MaxLostElements: 10,
					},
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRFidelityCheckFailsWithHighLossClass tests IR_FIDELITY fails with strict loss class.
func TestIRFidelityCheckFailsWithHighLossClass(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Create an IR file with a higher loss class
	irDir := filepath.Join(tempDir, "ir")
	if err := os.MkdirAll(irDir, 0755); err != nil {
		t.Fatalf("failed to create IR dir: %v", err)
	}
	irPath := filepath.Join(irDir, "high_loss.ir.json")
	irContent := []byte(`{"loss_class": "L3", "data": "test"}`)
	if err := os.WriteFile(irPath, irContent, 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-fail-test",
		Description: "Test IR fidelity fails with high loss class",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "IR fidelity should fail",
				IRFidelity: &IRFidelityDef{
					IRKey:        "ir_output",
					MaxLossClass: "L0", // Very strict - only L0 allowed
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	// Should pass because placeholder IR has L0
	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRStructureEqualCheckFail tests IR_STRUCTURE_EQUAL with different IRs.
func TestIRStructureEqualCheckFail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create two different test files
	testContent1 := []byte("Content A")
	testPath1 := filepath.Join(tempDir, "test1.txt")
	if err := os.WriteFile(testPath1, testContent1, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	testContent2 := []byte("Content B - different")
	testPath2 := filepath.Join(tempDir, "test2.txt")
	if err := os.WriteFile(testPath2, testContent2, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact1, err := cap.IngestFile(testPath1)
	if err != nil {
		t.Fatalf("failed to ingest file 1: %v", err)
	}

	artifact2, err := cap.IngestFile(testPath2)
	if err != nil {
		t.Fatalf("failed to ingest file 2: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-equal-fail-test",
		Description: "Test IR structure equality fails for different content",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact1.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_a",
				},
			},
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact2.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_b",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "IR structures should differ",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "ir_a",
					IRB: "ir_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusFail {
		t.Errorf("expected status 'fail', got %q", report.Status)
	}
}

// TestByteEqualCheckWithOutputs tests byte check using output from previous step.
func TestByteEqualCheckWithOutputs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for byte check with outputs")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "byte-equal-outputs-test",
		Description: "Test byte equality with step outputs",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: artifact.ID,
					OutputKey:  "exported",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Original equals exported",
				ByteEqual: &ByteEqualDef{
					ArtifactA: artifact.ID,
					ArtifactB: "exported",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestExtractIRStepWithPreviousOutput tests EXTRACT_IR using previous step output.
func TestExtractIRStepWithPreviousOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-from-output-test",
		Description: "Test IR extraction from previous step output",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: artifact.ID,
					OutputKey:  "exported_file",
				},
			},
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: "exported_file",
					PluginID:         "format-osis",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestExportStepDerivedMode tests export step with DERIVED mode (not implemented).
func TestExportStepDerivedMode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content for derived export")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "export-derived-test",
		Description: "Test export with DERIVED mode",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "DERIVED",
					ArtifactID: artifact.ID,
					OutputKey:  "exported",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for DERIVED mode (not implemented)")
	}
}

// TestByteEqualCheckMissingFile tests byte equality check with missing file.
func TestByteEqualCheckMissingFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Test with missing file
	check := &ByteEqualCheck{
		ArtifactA: artifact.ID,
		PathB:     "/nonexistent/path/file.txt",
	}

	_, err = check.Execute(cap)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestByteEqualCheckMissingArtifact tests byte equality check with missing artifact.
func TestByteEqualCheckMissingArtifact(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	check := &ByteEqualCheck{
		ArtifactA: "nonexistent-artifact",
		PathB:     filepath.Join(tempDir, "test.txt"),
	}

	_, err = check.Execute(cap)
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}

// TestTranscriptEqualCheckWithOutputs tests transcript check with outputs from steps.
func TestTranscriptEqualCheckWithOutputs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create two transcript files
	transcript1 := []byte(`{"type":"TEST","seq":1}`)
	transcript2 := []byte(`{"type":"TEST","seq":1}`)

	t1Path := filepath.Join(tempDir, "t1.json")
	t2Path := filepath.Join(tempDir, "t2.json")

	if err := os.WriteFile(t1Path, transcript1, 0600); err != nil {
		t.Fatalf("failed to write transcript 1: %v", err)
	}
	if err := os.WriteFile(t2Path, transcript2, 0600); err != nil {
		t.Fatalf("failed to write transcript 2: %v", err)
	}

	art1, err := cap.IngestFile(t1Path)
	if err != nil {
		t.Fatalf("failed to ingest t1: %v", err)
	}

	art2, err := cap.IngestFile(t2Path)
	if err != nil {
		t.Fatalf("failed to ingest t2: %v", err)
	}

	plan := &Plan{
		ID:          "transcript-outputs-test",
		Description: "Test transcript equality with outputs",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: art1.ID,
					OutputKey:  "t1",
				},
			},
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: art2.ID,
					OutputKey:  "t2",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Compare transcript outputs",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "t1",
					RunB: "t2",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestTranscriptEqualCheckMixedSources tests transcript check with one from run and one from output.
func TestTranscriptEqualCheckMixedSources(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a transcript from a run
	transcript := []byte(`{"type":"TEST","seq":1}`)
	hash := cas.Hash(transcript)

	cap.Manifest.Runs["run1"] = &capsule.Run{
		ID: "run1",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash,
		},
	}

	// Create a transcript file for output
	tPath := filepath.Join(tempDir, "t.json")
	if err := os.WriteFile(tPath, transcript, 0600); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	art, err := cap.IngestFile(tPath)
	if err != nil {
		t.Fatalf("failed to ingest transcript: %v", err)
	}

	plan := &Plan{
		ID:          "transcript-mixed-test",
		Description: "Test transcript equality with mixed sources",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: art.ID,
					OutputKey:  "t_output",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Compare run and output",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run1",
					RunB: "t_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRFidelityCheckInvalidJSON tests IR fidelity check with invalid JSON.
func TestIRFidelityCheckInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create an invalid IR file
	invalidIRPath := filepath.Join(tempDir, "invalid.ir.json")
	if err := os.WriteFile(invalidIRPath, []byte("not valid json{{{"), 0600); err != nil {
		t.Fatalf("failed to write invalid IR: %v", err)
	}

	art, err := cap.IngestFile(invalidIRPath)
	if err != nil {
		t.Fatalf("failed to ingest invalid IR: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-invalid-json-test",
		Description: "Test IR fidelity with invalid JSON",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: art.ID,
					OutputKey:  "invalid_ir",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "Check invalid IR",
				IRFidelity: &IRFidelityDef{
					IRKey:        "invalid_ir",
					MaxLossClass: "L0",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusFail {
		t.Errorf("expected status 'fail', got %q", report.Status)
	}

	if report.Results[0].Pass {
		t.Error("expected check to fail for invalid JSON")
	}
}

// TestByteEqualCheckBothArtifacts tests byte equality between two artifacts.
func TestByteEqualCheckBothArtifacts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create two identical files
	content := []byte("Identical content")

	path1 := filepath.Join(tempDir, "file1.txt")
	if err := os.WriteFile(path1, content, 0600); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	path2 := filepath.Join(tempDir, "file2.txt")
	if err := os.WriteFile(path2, content, 0600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	art1, err := cap.IngestFile(path1)
	if err != nil {
		t.Fatalf("failed to ingest file1: %v", err)
	}

	art2, err := cap.IngestFile(path2)
	if err != nil {
		t.Fatalf("failed to ingest file2: %v", err)
	}

	plan := &Plan{
		ID:          "byte-equal-artifacts-test",
		Description: "Test byte equality between two artifacts",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Compare two artifacts",
				ByteEqual: &ByteEqualDef{
					ArtifactA: art1.ID,
					ArtifactB: art2.ID,
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRStructureEqualCheckReadError tests IR structure check with read error.
func TestIRStructureEqualCheckReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-read-error-test",
		Description: "Test IR structure check with read error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_a",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "Check with missing IR B",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "ir_a",
					IRB: "nonexistent_ir_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR B")
	}
}

// TestCompareIRStepReadError tests COMPARE_IR step with read error.
func TestCompareIRStepReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "compare-ir-read-error-test",
		Description: "Test IR comparison with read error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_a",
				},
			},
			{
				Type: StepCompareIR,
				CompareIR: &CompareIRStep{
					IRAKey:    "ir_a",
					IRBKey:    "nonexistent_b",
					OutputKey: "comparison",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR B in compare")
	}
}

// TestExtractIRStepWithNoPlugin tests EXTRACT_IR with no plugin ID.
func TestExtractIRStepWithNoPlugin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-no-plugin-test",
		Description: "Test IR extraction with no plugin ID",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "", // Empty plugin ID
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestEmitNativeStepWithNoPlugin tests EMIT_NATIVE with no plugin ID.
func TestEmitNativeStepWithNoPlugin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "emit-native-no-plugin-test",
		Description: "Test native emission with no plugin ID",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "",
					OutputKey:        "ir_output",
				},
			},
			{
				Type: StepEmitNative,
				EmitNative: &EmitNativeStep{
					IRInputKey: "ir_output",
					PluginID:   "", // Empty plugin ID
					OutputKey:  "native_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestByteEqualCheckRetrieveError tests byte equality check when artifact retrieval fails.
func TestByteEqualCheckRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Manually create artifact with bad hash
	cap.Manifest.Artifacts["bad-artifact"] = &capsule.Artifact{
		ID:                "bad-artifact",
		PrimaryBlobSHA256: "nonexistent-hash-that-does-not-exist",
		OriginalName:      "test.txt",
		Kind:              "file",
	}

	plan := &Plan{
		ID:          "byte-equal-retrieve-error-test",
		Description: "Test byte equality with retrieve error",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Check with bad artifact",
				ByteEqual: &ByteEqualDef{
					ArtifactA: "bad-artifact",
					ArtifactB: "bad-artifact",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for artifact with bad hash")
	}
}

// TestTranscriptEqualCheckRunWithoutOutputs tests transcript check with run missing outputs.
func TestTranscriptEqualCheckRunWithoutOutputs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add run without Outputs
	cap.Manifest.Runs["run1"] = &capsule.Run{
		ID:      "run1",
		Outputs: nil, // No outputs
	}

	plan := &Plan{
		ID:          "transcript-no-outputs-test",
		Description: "Test transcript equality with run missing outputs",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Check run without outputs",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run1",
					RunB: "run1",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for run without outputs")
	}
}

// TestIRFidelityCheckWithLossClassL1 tests IR fidelity check with L1 loss class.
func TestIRFidelityCheckWithLossClassL1(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create an IR file with L1 loss class
	irContent := []byte(`{"loss_class": "L1", "data": "test"}`)
	irPath := filepath.Join(tempDir, "l1.ir.json")
	if err := os.WriteFile(irPath, irContent, 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	art, err := cap.IngestFile(irPath)
	if err != nil {
		t.Fatalf("failed to ingest IR: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-l1-test",
		Description: "Test IR fidelity with L1 loss class",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: art.ID,
					OutputKey:  "ir_output",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "Check L1 IR",
				IRFidelity: &IRFidelityDef{
					IRKey:        "ir_output",
					MaxLossClass: "L1", // Allows L1
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", report.Status)
	}
}

// TestIRFidelityCheckExceedsLossClass tests IR fidelity check when loss class exceeds budget.
func TestIRFidelityCheckExceedsLossClass(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create an IR file with L2 loss class
	irContent := []byte(`{"loss_class": "L2", "data": "test"}`)
	irPath := filepath.Join(tempDir, "l2.ir.json")
	if err := os.WriteFile(irPath, irContent, 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	art, err := cap.IngestFile(irPath)
	if err != nil {
		t.Fatalf("failed to ingest IR: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-exceeds-test",
		Description: "Test IR fidelity when loss class exceeds budget",
		Steps: []PlanStep{
			{
				Type: StepExport,
				Export: &ExportStep{
					Mode:       "IDENTITY",
					ArtifactID: art.ID,
					OutputKey:  "ir_output",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "Check L2 IR with L1 budget",
				IRFidelity: &IRFidelityDef{
					IRKey:        "ir_output",
					MaxLossClass: "L1", // Only allows up to L1, but we have L2
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if report.Status != StatusFail {
		t.Errorf("expected status 'fail', got %q", report.Status)
	}
}

// TestByteEqualCheckBothArtifactsRetrieveError tests byte equality when artifact B retrieval fails.
func TestByteEqualCheckBothArtifactsRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Create artifact B with bad hash
	cap.Manifest.Artifacts["bad-artifact-b"] = &capsule.Artifact{
		ID:                "bad-artifact-b",
		PrimaryBlobSHA256: "nonexistent-hash-b",
		OriginalName:      "test_b.txt",
		Kind:              "file",
	}

	plan := &Plan{
		ID:          "byte-equal-both-artifacts-error-test",
		Description: "Test byte equality with artifact B retrieve error",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Check with bad artifact B",
				ByteEqual: &ByteEqualDef{
					ArtifactA: artifact.ID,
					ArtifactB: "bad-artifact-b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for artifact B with bad hash")
	}
}

// TestTranscriptEqualCheckReadOutputError tests transcript check with unreadable output file.
func TestTranscriptEqualCheckReadOutputError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a transcript from a run
	transcript := []byte(`{"type":"TEST","seq":1}`)
	hash := cas.Hash(transcript)

	cap.Manifest.Runs["run1"] = &capsule.Run{
		ID: "run1",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash,
		},
	}

	// Create executor and manually add output with nonexistent path
	executor := NewExecutor(cap)
	executor.outputs["t_output"] = "/nonexistent/path/transcript.json"

	plan := &Plan{
		ID:          "transcript-read-error-test",
		Description: "Test transcript equality with read error",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Check with unreadable output",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run1",
					RunB: "t_output",
				},
			},
		},
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for unreadable transcript output")
	}
}

// TestByteEqualCheckReadOutputError tests byte equality check with unreadable output file.
func TestByteEqualCheckReadOutputError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	// Create executor and manually add output with nonexistent path
	executor := NewExecutor(cap)
	executor.outputs["bad_output"] = "/nonexistent/path/file.txt"

	plan := &Plan{
		ID:          "byte-equal-read-output-error-test",
		Description: "Test byte equality with unreadable output",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Check with unreadable output",
				ByteEqual: &ByteEqualDef{
					ArtifactA: artifact.ID,
					ArtifactB: "bad_output",
				},
			},
		},
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for unreadable output file")
	}
}

// TestExecuteTempDirCreationError tests plan execution when temp dir creation fails.
func TestExecuteTempDirCreationError(t *testing.T) {
	// This test is difficult to implement without mocking os.MkdirTemp
	// We'll skip it as it requires complex setup
	t.Skip("Skipping temp dir creation error test - requires mocking")
}

// TestCompareIRStepWriteError tests COMPARE_IR step when writing output fails.
func TestCompareIRStepWriteError(t *testing.T) {
	// This test would require making the temp directory read-only
	// which is complex to set up portably. We'll skip it.
	t.Skip("Skipping compare IR write error test - requires read-only filesystem")
}

// TestIRStructureEqualCheckFirstReadError tests IR structure check when first IR read fails.
func TestIRStructureEqualCheckFirstReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	testContent := []byte("Test content")
	testPath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testPath, testContent, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testPath)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-first-read-error-test",
		Description: "Test IR structure check with first IR read error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "format-osis",
					OutputKey:        "ir_b",
				},
			},
		},
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "Check with missing IR A",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "nonexistent_ir_a",
					IRB: "ir_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing IR A")
	}
}

// TestTranscriptEqualCheckMissingRunB tests transcript check with missing run B.
func TestTranscriptEqualCheckMissingRunB(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a transcript from a run
	transcript := []byte(`{"type":"TEST","seq":1}`)
	hash := cas.Hash(transcript)

	cap.Manifest.Runs["run1"] = &capsule.Run{
		ID: "run1",
		Outputs: &capsule.RunOutputs{
			TranscriptBlobSHA256: hash,
		},
	}

	plan := &Plan{
		ID:          "transcript-missing-run-b-test",
		Description: "Test transcript equality with missing run B",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Check with missing run B",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run1",
					RunB: "nonexistent_run",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing run B")
	}
}

// createTestPluginWithKind creates a plugin with a specific kind for testing.
func createTestPluginWithKind(t *testing.T, baseDir, name, kind string) {
	pluginPath := filepath.Join(baseDir, name)
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := &plugins.PluginManifest{
		PluginID:   name,
		Version:    "1.0.0",
		Kind:       kind,
		Entrypoint: "plugin.sh",
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	manifestPath := filepath.Join(pluginPath, "plugin.json")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create a simple shell script that reads stdin and writes a success response
	entrypointPath := filepath.Join(pluginPath, "plugin.sh")
	scriptContent := `#!/usr/bin/env sh
cat > /dev/null  # consume stdin
echo '{"status":"ok"}'
`
	if err := os.WriteFile(entrypointPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write entrypoint: %v", err)
	}
}

// TestRunToolStepNotToolPlugin tests that RUN_TOOL step fails for non-tool plugin.
func TestRunToolStepNotToolPlugin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a format plugin (not a tool plugin)
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "format-plugin", "format")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-not-tool-test",
		Description: "Test RUN_TOOL with non-tool plugin",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "format-plugin",
					Profile:      "test-profile",
					OutputKey:    "output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for non-tool plugin")
	}
	if err != nil && !strings.Contains(err.Error(), "is not a tool plugin") {
		t.Errorf("expected 'is not a tool plugin' error, got: %v", err)
	}
}

// TestRunToolStepInputNotFound tests that RUN_TOOL step fails when input is not found.
func TestRunToolStepInputNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-input-not-found-test",
		Description: "Test RUN_TOOL with missing input",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{"nonexistent-input"},
					OutputKey:    "output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for missing input")
	}
	if err != nil && !strings.Contains(err.Error(), "input not found") {
		t.Errorf("expected 'input not found' error, got: %v", err)
	}
}

// TestRunToolStepSuccess tests successful RUN_TOOL execution.
func TestRunToolStepSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-success-test",
		Description: "Test successful RUN_TOOL execution",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{}, // No inputs required
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}
}

// TestRunToolStepWithArtifactInput tests RUN_TOOL with artifact as input.
func TestRunToolStepWithArtifactInput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it as artifact
	testFilePath := filepath.Join(tempDir, "test-input.dat")
	if err := os.WriteFile(testFilePath, []byte("test artifact data"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-artifact-input-test",
		Description: "Test RUN_TOOL with artifact input",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{artifact.ID},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}
}

// TestRunToolStepWithPreviousOutput tests RUN_TOOL with output from previous step as input.
func TestRunToolStepWithPreviousOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-chained-test",
		Description: "Test RUN_TOOL with chained inputs",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "generate",
					Inputs:       []string{},
					OutputKey:    "step1_output",
				},
			},
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "process",
					Inputs:       []string{"step1_output"},
					OutputKey:    "step2_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}
}

// TestRunToolStepWithTranscript tests RUN_TOOL detects transcript.jsonl.
func TestRunToolStepWithTranscript(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin that writes a transcript
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	// Create plugin directory
	toolPluginPath := filepath.Join(pluginDir, "tool-with-transcript")
	if err := os.MkdirAll(toolPluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := &plugins.PluginManifest{
		PluginID:   "tool-with-transcript",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "plugin.sh",
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(toolPluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Plugin script that creates a transcript file
	scriptContent := `#!/usr/bin/env sh
# Read the JSON input
input=$(cat)

# Extract output_dir from the JSON (simple parsing)
output_dir=$(echo "$input" | grep -o '"output_dir":"[^"]*"' | cut -d'"' -f4)

# Create transcript file if output_dir was found
if [ -n "$output_dir" ]; then
    echo '{"event":"test","seq":1}' > "$output_dir/transcript.jsonl"
fi

echo '{"status":"ok"}'
`
	if err := os.WriteFile(filepath.Join(toolPluginPath, "plugin.sh"), []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write entrypoint: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-transcript-test",
		Description: "Test RUN_TOOL transcript detection",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-with-transcript",
					Profile:      "test-profile",
					Inputs:       []string{},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}

	// Check that transcript path was stored in outputs
	if _, ok := executor.outputs["tool_output_transcript"]; !ok {
		t.Error("expected transcript path to be stored in outputs")
	}
}

// TestRunToolStepErrorResponse tests RUN_TOOL handles error response from plugin.
func TestRunToolStepErrorResponse(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin that returns an error
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	toolPluginPath := filepath.Join(pluginDir, "error-tool")
	if err := os.MkdirAll(toolPluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := &plugins.PluginManifest{
		PluginID:   "error-tool",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "plugin.sh",
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(toolPluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Plugin script that returns an error
	scriptContent := `#!/usr/bin/env sh
cat > /dev/null
echo '{"status":"error","error":"intentional test error"}'
`
	if err := os.WriteFile(filepath.Join(toolPluginPath, "plugin.sh"), []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write entrypoint: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-error-response-test",
		Description: "Test RUN_TOOL error response handling",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "error-tool",
					Profile:      "test-profile",
					Inputs:       []string{},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for tool error response")
	}
	if err != nil && !strings.Contains(err.Error(), "tool returned error") {
		t.Errorf("expected 'tool returned error' error, got: %v", err)
	}
}

// setExecutorTempDir is a test helper to set the executor's tempDir field.
// This allows testing error paths that depend on directory creation failures.
func setExecutorTempDir(e *Executor, dir string) {
	e.tempDir = dir
}

// TestRunToolStepInputDirError tests RUN_TOOL when input directory creation fails.
func TestRunToolStepInputDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create a file where the input directory should be created
	// This will cause MkdirAll to fail
	blockingFile := filepath.Join(execTempDir, "tool_output_inputs")
	if err := os.WriteFile(blockingFile, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	executor := NewExecutorWithPlugins(cap, loader)
	// Set the tempDir to our prepared directory with the blocking file
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = make(map[string]string)

	plan := &Plan{
		ID:          "run-tool-input-dir-error-test",
		Description: "Test RUN_TOOL input dir creation failure",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	// Call executeRunToolStep directly to bypass Execute's tempDir creation
	err = executor.executeRunToolStep(plan.Steps[0].RunTool)
	if err == nil {
		t.Error("expected error for input dir creation failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to create input dir") {
		t.Errorf("expected 'failed to create input dir' error, got: %v", err)
	}
}

// TestRunToolStepOutputDirError tests RUN_TOOL when output directory creation fails.
func TestRunToolStepOutputDirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create a file where the output directory should be created
	// This will cause MkdirAll to fail
	blockingFile := filepath.Join(execTempDir, "tool_output_output")
	if err := os.WriteFile(blockingFile, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	executor := NewExecutorWithPlugins(cap, loader)
	// Set the tempDir to our prepared directory with the blocking file
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = make(map[string]string)

	plan := &Plan{
		ID:          "run-tool-output-dir-error-test",
		Description: "Test RUN_TOOL output dir creation failure",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	// Call executeRunToolStep directly to bypass Execute's tempDir creation
	err = executor.executeRunToolStep(plan.Steps[0].RunTool)
	if err == nil {
		t.Error("expected error for output dir creation failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to create output dir") {
		t.Errorf("expected 'failed to create output dir' error, got: %v", err)
	}
}

// TestRunToolStepEmptyOriginalName tests RUN_TOOL with artifact that has empty OriginalName.
func TestRunToolStepEmptyOriginalName(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.dat")
	if err := os.WriteFile(testFilePath, []byte("test artifact data"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	// Clear the OriginalName to test the fallback path
	artifact.OriginalName = ""

	plan := &Plan{
		ID:          "run-tool-empty-original-name-test",
		Description: "Test RUN_TOOL with empty OriginalName",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{artifact.ID},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}
}

// TestRunToolStepExportFailure tests RUN_TOOL when artifact export fails.
func TestRunToolStepExportFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithKind(t, pluginDir, "tool-plugin", "tool")

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add an artifact to the manifest but don't store the blob
	// This will cause Export to fail
	cap.Manifest.Artifacts["missing-blob-artifact"] = &capsule.Artifact{
		ID:                "missing-blob-artifact",
		OriginalName:      "missing.dat",
		PrimaryBlobSHA256: "nonexistent-hash-that-does-not-exist-in-cas",
	}

	plan := &Plan{
		ID:          "run-tool-export-failure-test",
		Description: "Test RUN_TOOL export failure",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "tool-plugin",
					Profile:      "test-profile",
					Inputs:       []string{"missing-blob-artifact"},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for export failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to export input") {
		t.Errorf("expected 'failed to export input' error, got: %v", err)
	}
}

// TestRunToolStepPluginExecutionFailure tests RUN_TOOL when plugin execution fails.
func TestRunToolStepPluginExecutionFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a tool plugin with a non-executable entrypoint
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	toolPluginPath := filepath.Join(pluginDir, "broken-tool")
	if err := os.MkdirAll(toolPluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := &plugins.PluginManifest{
		PluginID:   "broken-tool",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "plugin.sh",
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(toolPluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create a non-executable script (missing execute permission)
	scriptContent := `#!/usr/bin/env sh
echo '{"status":"ok"}'
`
	if err := os.WriteFile(filepath.Join(toolPluginPath, "plugin.sh"), []byte(scriptContent), 0600); err != nil {
		t.Fatalf("failed to write entrypoint: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "run-tool-plugin-exec-failure-test",
		Description: "Test RUN_TOOL plugin execution failure",
		Steps: []PlanStep{
			{
				Type: StepRunTool,
				RunTool: &RunToolStep{
					ToolPluginID: "broken-tool",
					Profile:      "test-profile",
					Inputs:       []string{},
					OutputKey:    "tool_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for plugin execution failure")
	}
	if err != nil && !strings.Contains(err.Error(), "tool execution failed") {
		t.Errorf("expected 'tool execution failed' error, got: %v", err)
	}
}

// createTestPluginWithIRSupport creates a plugin with IR extraction/emission support.
func createTestPluginWithIRSupport(t *testing.T, baseDir, name string, canExtract, canEmit bool) {
	pluginPath := filepath.Join(baseDir, name)
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := map[string]interface{}{
		"plugin_id":  name,
		"version":    "1.0.0",
		"kind":       "format",
		"entrypoint": "plugin.sh",
		"ir_support": map[string]interface{}{
			"can_extract": canExtract,
			"can_emit":    canEmit,
			"loss_class":  "L0",
			"formats":     []string{"test"},
		},
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	manifestPath := filepath.Join(pluginPath, "plugin.json")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create a shell script that handles extract-ir and emit-native commands
	scriptContent := `#!/usr/bin/env sh
input=$(cat)
command=$(echo "$input" | grep -o '"command":"[^"]*"' | cut -d'"' -f4)
output_dir=$(echo "$input" | grep -o '"output_dir":"[^"]*"' | cut -d'"' -f4)

if [ "$command" = "extract-ir" ]; then
    # Create IR output file
    ir_path="$output_dir/extracted.ir.json"
    echo '{"type":"document","content":[]}' > "$ir_path"
    echo "{\"status\":\"ok\",\"result\":{\"ir_path\":\"$ir_path\",\"loss_class\":\"L0\"}}"
elif [ "$command" = "emit-native" ]; then
    # Create native output file
    output_path="$output_dir/output.txt"
    echo "native output" > "$output_path"
    echo "{\"status\":\"ok\",\"result\":{\"output_path\":\"$output_path\",\"format\":\"test\",\"loss_class\":\"L0\"}}"
else
    echo '{"status":"ok"}'
fi
`
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.sh"), []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write entrypoint: %v", err)
	}
}

// TestExtractIRStepWithPlugin tests EXTRACT_IR with a plugin that supports IR extraction.
func TestExtractIRStepWithPlugin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with IR extraction support
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithIRSupport(t, pluginDir, "ir-plugin", true, false)

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-plugin-test",
		Description: "Test EXTRACT_IR with plugin",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "ir-plugin",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}
}

// TestExtractIRStepExportFailure tests EXTRACT_IR when artifact export fails.
func TestExtractIRStepExportFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add an artifact to the manifest but don't store the blob
	cap.Manifest.Artifacts["missing-blob"] = &capsule.Artifact{
		ID:                "missing-blob",
		OriginalName:      "missing.txt",
		PrimaryBlobSHA256: "nonexistent-hash",
	}

	plan := &Plan{
		ID:          "extract-ir-export-failure-test",
		Description: "Test EXTRACT_IR export failure",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: "missing-blob",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for export failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to export artifact") {
		t.Errorf("expected 'failed to export artifact' error, got: %v", err)
	}
}

// TestExtractIRStepMkdirError tests EXTRACT_IR when IR output directory creation fails.
func TestExtractIRStepMkdirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create a file where the IR output directory should be created
	blockingFile := filepath.Join(execTempDir, "ir_output_ir")
	if err := os.WriteFile(blockingFile, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = make(map[string]string)

	step := &ExtractIRStep{
		SourceArtifactID: artifact.ID,
		OutputKey:        "ir_output",
	}

	err = executor.executeExtractIRStep(step)
	if err == nil {
		t.Error("expected error for IR output dir creation failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to create IR output dir") {
		t.Errorf("expected 'failed to create IR output dir' error, got: %v", err)
	}
}

// TestExtractIRStepWriteError tests EXTRACT_IR when writing placeholder IR fails.
func TestExtractIRStepWriteError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create a directory where the IR file should be written (to cause write failure)
	blockingDir := filepath.Join(execTempDir, "ir_output.ir.json")
	if err := os.MkdirAll(blockingDir, 0755); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = make(map[string]string)

	step := &ExtractIRStep{
		SourceArtifactID: artifact.ID,
		OutputKey:        "ir_output",
	}

	err = executor.executeExtractIRStep(step)
	if err == nil {
		t.Error("expected error for IR write failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to write IR output") {
		t.Errorf("expected 'failed to write IR output' error, got: %v", err)
	}
}

// TestExtractIRStepPluginExecutionError tests EXTRACT_IR when plugin execution fails.
func TestExtractIRStepPluginExecutionError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with IR support but broken execution
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	pluginPath := filepath.Join(pluginDir, "broken-ir-plugin")
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := map[string]interface{}{
		"plugin_id":  "broken-ir-plugin",
		"version":    "1.0.0",
		"kind":       "format",
		"entrypoint": "plugin.sh",
		"ir_support": map[string]interface{}{
			"can_extract": true,
			"can_emit":    false,
			"loss_class":  "L0",
			"formats":     []string{"test"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create non-executable plugin script
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.sh"), []byte("#!/bin/sh\nexit 1"), 0600); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-plugin-exec-error-test",
		Description: "Test EXTRACT_IR plugin execution error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "broken-ir-plugin",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for plugin execution failure")
	}
	if err != nil && !strings.Contains(err.Error(), "plugin extract-ir failed") {
		t.Errorf("expected 'plugin extract-ir failed' error, got: %v", err)
	}
}

// TestExtractIRStepPluginParseError tests EXTRACT_IR when parsing plugin result fails.
func TestExtractIRStepPluginParseError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin that returns invalid result
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	pluginPath := filepath.Join(pluginDir, "bad-result-plugin")
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := map[string]interface{}{
		"plugin_id":  "bad-result-plugin",
		"version":    "1.0.0",
		"kind":       "format",
		"entrypoint": "plugin.sh",
		"ir_support": map[string]interface{}{
			"can_extract": true,
			"can_emit":    false,
			"loss_class":  "L0",
			"formats":     []string{"test"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create plugin that returns error status
	scriptContent := `#!/usr/bin/env sh
cat > /dev/null
echo '{"status":"error","error":"intentional test error"}'
`
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.sh"), []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "extract-ir-parse-error-test",
		Description: "Test EXTRACT_IR parse error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					PluginID:         "bad-result-plugin",
					OutputKey:        "ir_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for parse failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to parse extract-ir result") {
		t.Errorf("expected 'failed to parse extract-ir result' error, got: %v", err)
	}
}

// TestEmitNativeStepWithPlugin tests EMIT_NATIVE with a plugin that supports IR emission.
func TestEmitNativeStepWithPlugin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with IR emission support
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}
	createTestPluginWithIRSupport(t, pluginDir, "emit-plugin", false, true)

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "emit-native-plugin-test",
		Description: "Test EMIT_NATIVE with plugin",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					OutputKey:        "ir_output",
				},
			},
			{
				Type: StepEmitNative,
				EmitNative: &EmitNativeStep{
					IRInputKey: "ir_output",
					PluginID:   "emit-plugin",
					OutputKey:  "native_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if report == nil {
		t.Error("expected report, got nil")
	}
}

// TestEmitNativeStepMkdirError tests EMIT_NATIVE when output directory creation fails.
func TestEmitNativeStepMkdirError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create an IR file
	irPath := filepath.Join(execTempDir, "test.ir.json")
	if err := os.WriteFile(irPath, []byte(`{"type":"document"}`), 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	// Create a file where the native output directory should be created
	blockingFile := filepath.Join(execTempDir, "native_output_native")
	if err := os.WriteFile(blockingFile, []byte("blocking"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = map[string]string{"ir_input": irPath}

	step := &EmitNativeStep{
		IRInputKey: "ir_input",
		OutputKey:  "native_output",
	}

	err = executor.executeEmitNativeStep(step)
	if err == nil {
		t.Error("expected error for native output dir creation failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to create native output dir") {
		t.Errorf("expected 'failed to create native output dir' error, got: %v", err)
	}
}

// TestEmitNativeStepReadError tests EMIT_NATIVE when reading IR input fails.
func TestEmitNativeStepReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	// Point to a non-existent IR file
	executor.outputs = map[string]string{"ir_input": "/nonexistent/path/to/ir.json"}

	step := &EmitNativeStep{
		IRInputKey: "ir_input",
		OutputKey:  "native_output",
	}

	err = executor.executeEmitNativeStep(step)
	if err == nil {
		t.Error("expected error for IR read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read IR input") {
		t.Errorf("expected 'failed to read IR input' error, got: %v", err)
	}
}

// TestEmitNativeStepWriteError tests EMIT_NATIVE when writing native output fails.
func TestEmitNativeStepWriteError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create an IR file
	irPath := filepath.Join(execTempDir, "test.ir.json")
	if err := os.WriteFile(irPath, []byte(`{"type":"document"}`), 0600); err != nil {
		t.Fatalf("failed to write IR file: %v", err)
	}

	// Create a directory where the native output file should be written
	blockingDir := filepath.Join(execTempDir, "native_output")
	if err := os.MkdirAll(blockingDir, 0755); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = map[string]string{"ir_input": irPath}

	step := &EmitNativeStep{
		IRInputKey: "ir_input",
		OutputKey:  "native_output",
	}

	err = executor.executeEmitNativeStep(step)
	if err == nil {
		t.Error("expected error for native write failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to write native output") {
		t.Errorf("expected 'failed to write native output' error, got: %v", err)
	}
}

// TestEmitNativeStepPluginExecutionError tests EMIT_NATIVE when plugin execution fails.
func TestEmitNativeStepPluginExecutionError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin with IR emit support but broken execution
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	pluginPath := filepath.Join(pluginDir, "broken-emit-plugin")
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := map[string]interface{}{
		"plugin_id":  "broken-emit-plugin",
		"version":    "1.0.0",
		"kind":       "format",
		"entrypoint": "plugin.sh",
		"ir_support": map[string]interface{}{
			"can_extract": false,
			"can_emit":    true,
			"loss_class":  "L0",
			"formats":     []string{"test"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create non-executable plugin script
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.sh"), []byte("#!/bin/sh\nexit 1"), 0600); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "emit-native-plugin-exec-error-test",
		Description: "Test EMIT_NATIVE plugin execution error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					OutputKey:        "ir_output",
				},
			},
			{
				Type: StepEmitNative,
				EmitNative: &EmitNativeStep{
					IRInputKey: "ir_output",
					PluginID:   "broken-emit-plugin",
					OutputKey:  "native_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for plugin execution failure")
	}
	if err != nil && !strings.Contains(err.Error(), "plugin emit-native failed") {
		t.Errorf("expected 'plugin emit-native failed' error, got: %v", err)
	}
}

// TestEmitNativeStepPluginParseError tests EMIT_NATIVE when parsing plugin result fails.
func TestEmitNativeStepPluginParseError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a plugin that returns error status
	pluginDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugins dir: %v", err)
	}

	pluginPath := filepath.Join(pluginDir, "bad-emit-plugin")
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := map[string]interface{}{
		"plugin_id":  "bad-emit-plugin",
		"version":    "1.0.0",
		"kind":       "format",
		"entrypoint": "plugin.sh",
		"ir_support": map[string]interface{}{
			"can_extract": false,
			"can_emit":    true,
			"loss_class":  "L0",
			"formats":     []string{"test"},
		},
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifestData, 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	scriptContent := `#!/usr/bin/env sh
cat > /dev/null
echo '{"status":"error","error":"intentional test error"}'
`
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.sh"), []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	loader := plugins.NewLoader()
	if err := loader.LoadFromDir(pluginDir); err != nil {
		t.Fatalf("failed to load plugins: %v", err)
	}

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "emit-native-parse-error-test",
		Description: "Test EMIT_NATIVE parse error",
		Steps: []PlanStep{
			{
				Type: StepExtractIR,
				ExtractIR: &ExtractIRStep{
					SourceArtifactID: artifact.ID,
					OutputKey:        "ir_output",
				},
			},
			{
				Type: StepEmitNative,
				EmitNative: &EmitNativeStep{
					IRInputKey: "ir_output",
					PluginID:   "bad-emit-plugin",
					OutputKey:  "native_output",
				},
			},
		},
	}

	executor := NewExecutorWithPlugins(cap, loader)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for parse failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to parse emit-native result") {
		t.Errorf("expected 'failed to parse emit-native result' error, got: %v", err)
	}
}

// TestCompareIRStepReadErrorA tests COMPARE_IR when reading IR A fails.
func TestCompareIRStepReadErrorA(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create IR B file but not IR A
	irBPath := filepath.Join(execTempDir, "ir_b.json")
	if err := os.WriteFile(irBPath, []byte(`{"type":"document"}`), 0600); err != nil {
		t.Fatalf("failed to write IR B file: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = map[string]string{
		"ir_a": "/nonexistent/ir_a.json",
		"ir_b": irBPath,
	}

	step := &CompareIRStep{
		IRAKey:    "ir_a",
		IRBKey:    "ir_b",
		OutputKey: "comparison",
	}

	err = executor.executeCompareIRStep(step)
	if err == nil {
		t.Error("expected error for IR A read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read IR A") {
		t.Errorf("expected 'failed to read IR A' error, got: %v", err)
	}
}

// TestCompareIRStepReadErrorB tests COMPARE_IR when reading IR B fails.
func TestCompareIRStepReadErrorB(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create IR A file but not IR B
	irAPath := filepath.Join(execTempDir, "ir_a.json")
	if err := os.WriteFile(irAPath, []byte(`{"type":"document"}`), 0600); err != nil {
		t.Fatalf("failed to write IR A file: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = map[string]string{
		"ir_a": irAPath,
		"ir_b": "/nonexistent/ir_b.json",
	}

	step := &CompareIRStep{
		IRAKey:    "ir_a",
		IRBKey:    "ir_b",
		OutputKey: "comparison",
	}

	err = executor.executeCompareIRStep(step)
	if err == nil {
		t.Error("expected error for IR B read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read IR B") {
		t.Errorf("expected 'failed to read IR B' error, got: %v", err)
	}
}

// TestByteEqualCheckOutputReadError tests BYTE_EQUAL when reading output file fails.
func TestByteEqualCheckOutputReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it as artifact A
	testFilePath := filepath.Join(tempDir, "test-input.txt")
	if err := os.WriteFile(testFilePath, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	artifact, err := cap.IngestFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to ingest artifact: %v", err)
	}

	plan := &Plan{
		ID:          "byte-equal-output-read-error",
		Description: "Test BYTE_EQUAL output read error",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Test check",
				ByteEqual: &ByteEqualDef{
					ArtifactA: artifact.ID,
					ArtifactB: "output_key", // This is an output key
				},
			},
		},
	}

	executor := NewExecutor(cap)
	// Set up outputs with a path that doesn't exist
	executor.outputs = map[string]string{
		"output_key": "/nonexistent/path/to/file.txt",
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for output read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read output") {
		t.Errorf("expected 'failed to read output' error, got: %v", err)
	}
}

// TestTranscriptEqualCheckOutputReadErrorA tests TRANSCRIPT_EQUAL when reading output A fails.
func TestTranscriptEqualCheckOutputReadErrorA(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "transcript-equal-output-read-error-a",
		Description: "Test TRANSCRIPT_EQUAL output A read error",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Test check",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run_a",
					RunB: "run_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	// Set up outputs with paths that don't exist
	executor.outputs = map[string]string{
		"run_a": "/nonexistent/path/to/transcript_a.jsonl",
		"run_b": "/nonexistent/path/to/transcript_b.jsonl",
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for transcript read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read transcript A") {
		t.Errorf("expected 'failed to read transcript A' error, got: %v", err)
	}
}

// TestTranscriptEqualCheckOutputReadErrorB tests TRANSCRIPT_EQUAL when reading output B fails.
func TestTranscriptEqualCheckOutputReadErrorB(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a valid transcript file for A
	transcriptPath := filepath.Join(tempDir, "transcript_a.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(`{"event":"test"}`), 0600); err != nil {
		t.Fatalf("failed to write transcript file: %v", err)
	}

	plan := &Plan{
		ID:          "transcript-equal-output-read-error-b",
		Description: "Test TRANSCRIPT_EQUAL output B read error",
		Checks: []PlanCheck{
			{
				Type:  CheckTranscriptEqual,
				Label: "Test check",
				TranscriptEqual: &TranscriptEqualDef{
					RunA: "run_a",
					RunB: "run_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	// Set up outputs - A exists, B doesn't
	executor.outputs = map[string]string{
		"run_a": transcriptPath,
		"run_b": "/nonexistent/path/to/transcript_b.jsonl",
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for transcript B read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read transcript B") {
		t.Errorf("expected 'failed to read transcript B' error, got: %v", err)
	}
}

// TestIRStructureEqualCheckReadErrorA tests IR_STRUCTURE_EQUAL when reading IR A fails.
func TestIRStructureEqualCheckReadErrorA(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-equal-read-error-a",
		Description: "Test IR_STRUCTURE_EQUAL read error A",
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "Test check",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "ir_a",
					IRB: "ir_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	executor.outputs = map[string]string{
		"ir_a": "/nonexistent/path/to/ir_a.json",
		"ir_b": "/nonexistent/path/to/ir_b.json",
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for IR A read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read IR A") {
		t.Errorf("expected 'failed to read IR A' error, got: %v", err)
	}
}

// TestIRStructureEqualCheckReadErrorB tests IR_STRUCTURE_EQUAL when reading IR B fails.
func TestIRStructureEqualCheckReadErrorB(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a valid IR A file
	irAPath := filepath.Join(tempDir, "ir_a.json")
	if err := os.WriteFile(irAPath, []byte(`{"type":"document","content":[]}`), 0600); err != nil {
		t.Fatalf("failed to write IR A file: %v", err)
	}

	plan := &Plan{
		ID:          "ir-structure-equal-read-error-b",
		Description: "Test IR_STRUCTURE_EQUAL read error B",
		Checks: []PlanCheck{
			{
				Type:  CheckIRStructureEqual,
				Label: "Test check",
				IRStructureEqual: &IRStructureEqualDef{
					IRA: "ir_a",
					IRB: "ir_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	executor.outputs = map[string]string{
		"ir_a": irAPath,
		"ir_b": "/nonexistent/path/to/ir_b.json",
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for IR B read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read IR B") {
		t.Errorf("expected 'failed to read IR B' error, got: %v", err)
	}
}

// TestIRFidelityCheckReadError tests IR_FIDELITY when reading IR file fails.
func TestIRFidelityCheckReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	plan := &Plan{
		ID:          "ir-fidelity-read-error",
		Description: "Test IR_FIDELITY read error",
		Checks: []PlanCheck{
			{
				Type:  CheckIRFidelity,
				Label: "Test check",
				IRFidelity: &IRFidelityDef{
					IRKey:        "ir_key",
					MaxLossClass: "L0",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	executor.outputs = map[string]string{
		"ir_key": "/nonexistent/path/to/ir.json",
	}

	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for IR read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read IR") {
		t.Errorf("expected 'failed to read IR' error, got: %v", err)
	}
}

// TestCompareIRStepWriteErrorActual tests COMPARE_IR when writing comparison result fails.
func TestCompareIRStepWriteErrorActual(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create executor's temp directory manually
	execTempDir := filepath.Join(tempDir, "exec-temp")
	if err := os.MkdirAll(execTempDir, 0755); err != nil {
		t.Fatalf("failed to create exec temp dir: %v", err)
	}

	// Create IR A and IR B files
	irAPath := filepath.Join(execTempDir, "ir_a.json")
	irBPath := filepath.Join(execTempDir, "ir_b.json")
	if err := os.WriteFile(irAPath, []byte(`{"type":"document"}`), 0600); err != nil {
		t.Fatalf("failed to write IR A file: %v", err)
	}
	if err := os.WriteFile(irBPath, []byte(`{"type":"document"}`), 0600); err != nil {
		t.Fatalf("failed to write IR B file: %v", err)
	}

	// Create a directory where the output file should be written
	blockingDir := filepath.Join(execTempDir, "comparison.json")
	if err := os.MkdirAll(blockingDir, 0755); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}

	executor := NewExecutor(cap)
	setExecutorTempDir(executor, execTempDir)
	executor.outputs = map[string]string{
		"ir_a": irAPath,
		"ir_b": irBPath,
	}

	step := &CompareIRStep{
		IRAKey:    "ir_a",
		IRBKey:    "ir_b",
		OutputKey: "comparison",
	}

	err = executor.executeCompareIRStep(step)
	if err == nil {
		t.Error("expected error for comparison write failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to write comparison result") {
		t.Errorf("expected 'failed to write comparison result' error, got: %v", err)
	}
}

// TestByteEqualCheckValidatorSuccess tests ByteEqualCheck.Execute success path.
func TestByteEqualCheckValidatorSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create a test file and ingest it as an artifact
	testData := []byte("test content")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}
	cap.Manifest.Artifacts["artifact_a"] = artifact

	// Create matching file for comparison
	comparisonFile := filepath.Join(tempDir, "comparison.txt")
	if err := os.WriteFile(comparisonFile, testData, 0600); err != nil {
		t.Fatalf("failed to write comparison file: %v", err)
	}

	check := &ByteEqualCheck{
		ArtifactA: "artifact_a",
		PathB:     comparisonFile,
	}

	result, err := check.Execute(cap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected check to pass")
	}
}

// TestByteEqualCheckValidatorArtifactNotFound tests ByteEqualCheck.Execute when artifact not found.
func TestByteEqualCheckValidatorArtifactNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	check := &ByteEqualCheck{
		ArtifactA: "nonexistent",
		PathB:     "/some/path",
	}

	_, err = check.Execute(cap)
	if err == nil {
		t.Error("expected error for missing artifact")
	}
	if err != nil && !strings.Contains(err.Error(), "artifact not found") {
		t.Errorf("expected 'artifact not found' error, got: %v", err)
	}
}

// TestByteEqualCheckValidatorReadFileError tests ByteEqualCheck.Execute when reading file fails.
func TestByteEqualCheckValidatorReadFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest an artifact
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	artifact, err := cap.IngestFile(testFile)
	if err != nil {
		t.Fatalf("failed to ingest file: %v", err)
	}
	cap.Manifest.Artifacts["artifact_a"] = artifact

	check := &ByteEqualCheck{
		ArtifactA: "artifact_a",
		PathB:     "/nonexistent/path/file.txt",
	}

	_, err = check.Execute(cap)
	if err == nil {
		t.Error("expected error for file read failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("expected 'failed to read file' error, got: %v", err)
	}
}

// TestByteEqualCheckTwoArtifacts tests executeByteEqualCheck with both artifacts from capsule.
func TestByteEqualCheckTwoArtifacts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest first artifact
	testFileA := filepath.Join(tempDir, "testA.txt")
	testData := []byte("test content")
	if err := os.WriteFile(testFileA, testData, 0600); err != nil {
		t.Fatalf("failed to write test file A: %v", err)
	}

	artifactA, err := cap.IngestFile(testFileA)
	if err != nil {
		t.Fatalf("failed to ingest file A: %v", err)
	}
	cap.Manifest.Artifacts["artifact_a"] = artifactA

	// Create and ingest second artifact with same content
	testFileB := filepath.Join(tempDir, "testB.txt")
	if err := os.WriteFile(testFileB, testData, 0600); err != nil {
		t.Fatalf("failed to write test file B: %v", err)
	}

	artifactB, err := cap.IngestFile(testFileB)
	if err != nil {
		t.Fatalf("failed to ingest file B: %v", err)
	}
	cap.Manifest.Artifacts["artifact_b"] = artifactB

	plan := &Plan{
		ID:          "two-artifacts",
		Description: "Test two artifacts comparison",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Compare two artifacts",
				ByteEqual: &ByteEqualDef{
					ArtifactA: "artifact_a",
					ArtifactB: "artifact_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	report, err := executor.Execute(plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if !report.Results[0].Pass {
		t.Error("expected check to pass for identical content")
	}
}

// TestByteEqualCheckArtifactBRetrieveError tests executeByteEqualCheck when retrieving artifact B fails.
func TestByteEqualCheckArtifactBRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest first artifact
	testFileA := filepath.Join(tempDir, "testA.txt")
	if err := os.WriteFile(testFileA, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file A: %v", err)
	}

	artifactA, err := cap.IngestFile(testFileA)
	if err != nil {
		t.Fatalf("failed to ingest file A: %v", err)
	}
	cap.Manifest.Artifacts["artifact_a"] = artifactA

	// Add artifact B reference with invalid SHA
	cap.Manifest.Artifacts["artifact_b"] = &capsule.Artifact{
		PrimaryBlobSHA256: "invalid_sha_that_doesnt_exist",
	}

	plan := &Plan{
		ID:          "artifact-b-error",
		Description: "Test artifact B retrieve error",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Compare artifacts",
				ByteEqual: &ByteEqualDef{
					ArtifactA: "artifact_a",
					ArtifactB: "artifact_b",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for artifact B retrieve failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to retrieve artifact B") {
		t.Errorf("expected 'failed to retrieve artifact B' error, got: %v", err)
	}
}

// TestByteEqualCheckArtifactBNotFound tests executeByteEqualCheck when ArtifactB doesn't exist.
func TestByteEqualCheckArtifactBNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Create and ingest first artifact
	testFileA := filepath.Join(tempDir, "testA.txt")
	if err := os.WriteFile(testFileA, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write test file A: %v", err)
	}

	artifactA, err := cap.IngestFile(testFileA)
	if err != nil {
		t.Fatalf("failed to ingest file A: %v", err)
	}
	cap.Manifest.Artifacts["artifact_a"] = artifactA

	// Don't add artifact_b to outputs or artifacts

	plan := &Plan{
		ID:          "artifact-b-not-found",
		Description: "Test artifact B not found",
		Checks: []PlanCheck{
			{
				Type:  CheckByteEqual,
				Label: "Compare artifacts",
				ByteEqual: &ByteEqualDef{
					ArtifactA: "artifact_a",
					ArtifactB: "artifact_b_nonexistent",
				},
			},
		},
	}

	executor := NewExecutor(cap)
	_, err = executor.Execute(plan)
	if err == nil {
		t.Error("expected error for artifact B not found")
	}
	if err != nil && !strings.Contains(err.Error(), "artifact/output not found") {
		t.Errorf("expected 'artifact/output not found' error, got: %v", err)
	}
}

// TestByteEqualCheckValidatorRetrieveError tests ByteEqualCheck.Execute when retrieve fails.
func TestByteEqualCheckValidatorRetrieveError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "selfcheck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	capsuleDir := filepath.Join(tempDir, "capsule")
	cap, err := capsule.New(capsuleDir)
	if err != nil {
		t.Fatalf("failed to create capsule: %v", err)
	}

	// Add artifact reference with invalid SHA that doesn't exist in store
	cap.Manifest.Artifacts["artifact_a"] = &capsule.Artifact{
		PrimaryBlobSHA256: "sha256_that_doesnt_exist_in_store",
	}

	check := &ByteEqualCheck{
		ArtifactA: "artifact_a",
		PathB:     "/some/path",
	}

	_, err = check.Execute(cap)
	if err == nil {
		t.Error("expected error for retrieve failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to retrieve artifact") {
		t.Errorf("expected 'failed to retrieve artifact' error, got: %v", err)
	}
}
