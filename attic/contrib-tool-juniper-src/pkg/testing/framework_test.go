package testing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTestSuite(t *testing.T) {
	suite := NewTestSuite("Parser Tests", "Tests for SWORD parsers")

	if suite.Name != "Parser Tests" {
		t.Errorf("Expected name 'Parser Tests', got %s", suite.Name)
	}
	if suite.Description != "Tests for SWORD parsers" {
		t.Errorf("Expected description 'Tests for SWORD parsers', got %s", suite.Description)
	}
	if len(suite.Results) != 0 {
		t.Errorf("Expected empty results, got %d", len(suite.Results))
	}
	if suite.StartTime.IsZero() {
		t.Errorf("Expected StartTime to be set")
	}
}

func TestTestSuite_AddResult(t *testing.T) {
	suite := NewTestSuite("Test Suite", "Description")

	// Add passing result
	suite.AddResult(TestResult{Name: "Test1", Passed: true})
	if suite.PassCount != 1 {
		t.Errorf("Expected PassCount 1, got %d", suite.PassCount)
	}

	// Add failing result
	suite.AddResult(TestResult{Name: "Test2", Passed: false})
	if suite.FailCount != 1 {
		t.Errorf("Expected FailCount 1, got %d", suite.FailCount)
	}

	// Add skipped result
	suite.AddResult(TestResult{Name: "Test3", Error: "skipped"})
	if suite.SkipCount != 1 {
		t.Errorf("Expected SkipCount 1, got %d", suite.SkipCount)
	}

	if len(suite.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(suite.Results))
	}
}

func TestTestSuite_PassRate(t *testing.T) {
	suite := NewTestSuite("Test Suite", "Description")

	// Empty suite
	if suite.PassRate() != 100.0 {
		t.Errorf("Expected 100%% pass rate for empty suite, got %f", suite.PassRate())
	}

	// Add results: 3 pass, 1 fail
	suite.AddResult(TestResult{Name: "Test1", Passed: true})
	suite.AddResult(TestResult{Name: "Test2", Passed: true})
	suite.AddResult(TestResult{Name: "Test3", Passed: true})
	suite.AddResult(TestResult{Name: "Test4", Passed: false})

	passRate := suite.PassRate()
	if passRate != 75.0 {
		t.Errorf("Expected 75%% pass rate, got %f", passRate)
	}
}

func TestTestSuite_Finish(t *testing.T) {
	suite := NewTestSuite("Test Suite", "Description")

	time.Sleep(10 * time.Millisecond)
	suite.Finish()

	if suite.EndTime.IsZero() {
		t.Errorf("Expected EndTime to be set")
	}
	if !suite.EndTime.After(suite.StartTime) {
		t.Errorf("Expected EndTime > StartTime")
	}
}

func TestNewTestReport(t *testing.T) {
	report := NewTestReport("1.0.0")

	if report.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", report.Version)
	}
	if len(report.Suites) != 0 {
		t.Errorf("Expected empty suites, got %d", len(report.Suites))
	}
	if report.GeneratedAt.IsZero() {
		t.Errorf("Expected GeneratedAt to be set")
	}
}

func TestTestReport_AddSuite(t *testing.T) {
	report := NewTestReport("1.0.0")
	suite := NewTestSuite("Test Suite", "Description")
	suite.AddResult(TestResult{Name: "Test1", Passed: true})
	suite.Finish()

	report.AddSuite(suite)

	if len(report.Suites) != 1 {
		t.Errorf("Expected 1 suite, got %d", len(report.Suites))
	}
}

func TestTestReport_CalculateSummary(t *testing.T) {
	report := NewTestReport("1.0.0")

	// Add first suite: 2 pass, 1 fail
	suite1 := NewTestSuite("Suite 1", "First suite")
	suite1.AddResult(TestResult{Name: "T1", Passed: true, Similarity: 1.0})
	suite1.AddResult(TestResult{Name: "T2", Passed: true, Similarity: 0.95})
	suite1.AddResult(TestResult{Name: "T3", Passed: false, Similarity: 0.8})
	suite1.Finish()
	report.AddSuite(suite1)

	// Add second suite: 1 pass, 1 skip
	suite2 := NewTestSuite("Suite 2", "Second suite")
	suite2.AddResult(TestResult{Name: "T4", Passed: true, Similarity: 1.0})
	suite2.AddResult(TestResult{Name: "T5", Error: "skipped"})
	suite2.Finish()
	report.AddSuite(suite2)

	report.CalculateSummary()

	summary := report.Summary
	if summary.TotalTests != 5 {
		t.Errorf("Expected 5 total tests, got %d", summary.TotalTests)
	}
	if summary.PassedTests != 3 {
		t.Errorf("Expected 3 passed tests, got %d", summary.PassedTests)
	}
	if summary.FailedTests != 1 {
		t.Errorf("Expected 1 failed test, got %d", summary.FailedTests)
	}
	if summary.SkippedTests != 1 {
		t.Errorf("Expected 1 skipped test, got %d", summary.SkippedTests)
	}
	// Pass rate: 3 passed / (5 total - 1 skipped) = 75%
	if summary.PassRate != 75.0 {
		t.Errorf("Expected 75%% pass rate, got %f", summary.PassRate)
	}
	// Average similarity: (1.0 + 0.95 + 0.8 + 1.0) / 4 = 0.9375
	if summary.AverageSimilarity < 0.93 || summary.AverageSimilarity > 0.94 {
		t.Errorf("Expected average similarity ~0.9375, got %f", summary.AverageSimilarity)
	}
}

func TestTestReport_HasFailures(t *testing.T) {
	report := NewTestReport("1.0.0")
	suite := NewTestSuite("Suite", "Description")

	// No failures
	suite.AddResult(TestResult{Name: "T1", Passed: true})
	suite.Finish()
	report.AddSuite(suite)
	report.CalculateSummary()

	if report.HasFailures() {
		t.Errorf("Expected no failures")
	}

	// Add failure
	suite2 := NewTestSuite("Suite 2", "Description")
	suite2.AddResult(TestResult{Name: "T2", Passed: false})
	suite2.Finish()
	report.AddSuite(suite2)
	report.CalculateSummary()

	if !report.HasFailures() {
		t.Errorf("Expected failures")
	}
}

func TestTestReport_SaveJSON(t *testing.T) {
	report := NewTestReport("1.0.0")
	suite := NewTestSuite("Test Suite", "Description")
	suite.AddResult(TestResult{
		Name:       "Test1",
		Passed:     true,
		Expected:   "expected",
		Actual:     "expected",
		Similarity: 1.0,
		Duration:   100 * time.Millisecond,
	})
	suite.Finish()
	report.AddSuite(suite)
	report.CalculateSummary()

	// Save to temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "report.json")

	err := report.SaveJSON(path)
	if err != nil {
		t.Fatalf("Failed to save report: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	var loaded TestReport
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to parse report: %v", err)
	}

	if loaded.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", loaded.Version)
	}
	if len(loaded.Suites) != 1 {
		t.Errorf("Expected 1 suite, got %d", len(loaded.Suites))
	}
}

func TestGoldenFile_LoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.golden")
	content := "This is the golden content.\nWith multiple lines."

	// Save golden file
	err := SaveGoldenFile(path, content)
	if err != nil {
		t.Fatalf("Failed to save golden file: %v", err)
	}

	// Load golden file
	golden, err := LoadGoldenFile(path)
	if err != nil {
		t.Fatalf("Failed to load golden file: %v", err)
	}

	if golden.Path != path {
		t.Errorf("Expected path %s, got %s", path, golden.Path)
	}
	if golden.Content != content {
		t.Errorf("Expected content %q, got %q", content, golden.Content)
	}
}

func TestLoadGoldenFile_NotFound(t *testing.T) {
	_, err := LoadGoldenFile("/nonexistent/path/file.golden")
	if err == nil {
		t.Errorf("Expected error for nonexistent file")
	}
}

func TestSaveGoldenFile_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "test.golden")
	content := "Content"

	err := SaveGoldenFile(path, content)
	if err != nil {
		t.Fatalf("Failed to save golden file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to be created at %s", path)
	}
}

func TestUpdateGoldenFile_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.golden")
	content := "Updated content"

	// Without env var, should not create file
	os.Unsetenv("UPDATE_GOLDEN")
	err := UpdateGoldenFile(path, content)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("File should not be created without UPDATE_GOLDEN")
	}

	// With env var, should create file
	os.Setenv("UPDATE_GOLDEN", "1")
	defer os.Unsetenv("UPDATE_GOLDEN")

	err = UpdateGoldenFile(path, content)
	if err != nil {
		t.Fatalf("Failed to update golden file: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

func TestTestResult_Fields(t *testing.T) {
	result := TestResult{
		Name:       "Test Name",
		Passed:     true,
		Expected:   "expected value",
		Actual:     "actual value",
		Diff:       "diff output",
		Error:      "",
		Duration:   150 * time.Millisecond,
		Similarity: 0.95,
	}

	if result.Name != "Test Name" {
		t.Errorf("Unexpected Name: %s", result.Name)
	}
	if !result.Passed {
		t.Errorf("Expected Passed to be true")
	}
	if result.Similarity != 0.95 {
		t.Errorf("Expected Similarity 0.95, got %f", result.Similarity)
	}
	if result.Duration != 150*time.Millisecond {
		t.Errorf("Expected Duration 150ms, got %v", result.Duration)
	}
}

func TestTestResult_JSONSerialization(t *testing.T) {
	result := TestResult{
		Name:       "Test",
		Passed:     true,
		Duration:   100 * time.Millisecond,
		Similarity: 1.0,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var loaded TestResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if loaded.Name != result.Name {
		t.Errorf("Name mismatch: %s vs %s", loaded.Name, result.Name)
	}
	if loaded.Passed != result.Passed {
		t.Errorf("Passed mismatch: %v vs %v", loaded.Passed, result.Passed)
	}
}

func TestTestReport_PrintSummary(t *testing.T) {
	report := NewTestReport("1.0.0")

	// Add a passing suite
	suite1 := NewTestSuite("Passing Suite", "All tests pass")
	suite1.AddResult(TestResult{Name: "T1", Passed: true, Similarity: 1.0})
	suite1.Finish()
	report.AddSuite(suite1)

	// Add a failing suite
	suite2 := NewTestSuite("Failing Suite", "Some tests fail")
	suite2.AddResult(TestResult{Name: "T2", Passed: false, Similarity: 0.5})
	suite2.AddResult(TestResult{Name: "T3", Error: "skipped"})
	suite2.Finish()
	report.AddSuite(suite2)

	report.CalculateSummary()

	// PrintSummary should not panic
	report.PrintSummary()
}

func TestTestReport_CalculateSummary_NoTests(t *testing.T) {
	report := NewTestReport("1.0.0")
	report.CalculateSummary()

	if report.Summary.TotalTests != 0 {
		t.Errorf("Expected 0 total tests, got %d", report.Summary.TotalTests)
	}
	if report.Summary.PassRate != 0 {
		t.Errorf("Expected 0 pass rate for no tests, got %f", report.Summary.PassRate)
	}
}

func TestTestReport_CalculateSummary_NoSimilarity(t *testing.T) {
	report := NewTestReport("1.0.0")

	suite := NewTestSuite("Suite", "Desc")
	suite.AddResult(TestResult{Name: "T1", Passed: true, Similarity: 0})
	suite.Finish()
	report.AddSuite(suite)

	report.CalculateSummary()

	if report.Summary.AverageSimilarity != 0 {
		t.Errorf("Expected 0 average similarity when all are 0, got %f", report.Summary.AverageSimilarity)
	}
}

func TestGoldenFile_Metadata(t *testing.T) {
	golden := &GoldenFile{
		Path:     "/path/to/file.golden",
		Content:  "content",
		Metadata: make(map[string]string),
	}

	golden.Metadata["key"] = "value"

	if golden.Metadata["key"] != "value" {
		t.Errorf("Expected metadata key=value, got %s", golden.Metadata["key"])
	}
}

func TestTestSummary_Fields(t *testing.T) {
	summary := TestSummary{
		TotalTests:        10,
		PassedTests:       8,
		FailedTests:       1,
		SkippedTests:      1,
		PassRate:          88.89,
		TotalDuration:     5 * time.Second,
		AverageSimilarity: 0.95,
	}

	if summary.TotalTests != 10 {
		t.Errorf("Expected 10 total tests, got %d", summary.TotalTests)
	}
	if summary.PassedTests != 8 {
		t.Errorf("Expected 8 passed tests, got %d", summary.PassedTests)
	}
}
