package testing

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultRunnerOptions(t *testing.T) {
	opts := DefaultRunnerOptions()

	if opts.Verbose {
		t.Error("Expected Verbose to be false by default")
	}
	if opts.CompareOpts.SimilarityThreshold != 0.99 {
		t.Errorf("Expected 0.99 threshold, got %f", opts.CompareOpts.SimilarityThreshold)
	}
	if opts.FailFast {
		t.Error("Expected FailFast to be false by default")
	}
}

func TestNewTestRunner(t *testing.T) {
	opts := DefaultRunnerOptions()
	runner := NewTestRunner(opts)
	defer runner.Close()

	if runner.Report == nil {
		t.Error("Expected Report to be initialized")
	}
	if runner.Options.CompareOpts.SimilarityThreshold != 0.99 {
		t.Error("Expected options to be set")
	}
}

func TestTestRunner_RunParserTest_Pass(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	result := runner.RunParserTest("test_pass", func() (string, error) {
		return "expected output", nil
	}, "expected output")

	if !result.Passed {
		t.Error("Expected test to pass")
	}
	if result.Similarity != 1.0 {
		t.Errorf("Expected similarity 1.0, got %f", result.Similarity)
	}
	if result.Duration == 0 {
		t.Error("Expected duration to be recorded")
	}
}

func TestTestRunner_RunParserTest_Fail(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	result := runner.RunParserTest("test_fail", func() (string, error) {
		return "actual output", nil
	}, "expected output")

	if result.Passed {
		t.Error("Expected test to fail")
	}
	if result.Similarity >= 0.99 {
		t.Errorf("Expected low similarity, got %f", result.Similarity)
	}
}

func TestTestRunner_RunParserTest_Error(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	testErr := errors.New("parse error")
	result := runner.RunParserTest("test_error", func() (string, error) {
		return "", testErr
	}, "expected")

	if result.Passed {
		t.Error("Expected test to fail")
	}
	if result.Error != "parse error" {
		t.Errorf("Expected error message 'parse error', got %s", result.Error)
	}
}

func TestTestRunner_RunParserTest_Filter(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.Filter = "matching"
	runner := NewTestRunner(opts)
	defer runner.Close()

	// Should run - matches filter
	result := runner.RunParserTest("matching_test", func() (string, error) {
		return "output", nil
	}, "output")

	if result.Error == "skipped" {
		t.Error("Expected matching test to run")
	}

	// Should skip - doesn't match filter
	result = runner.RunParserTest("other_test", func() (string, error) {
		return "output", nil
	}, "output")

	if result.Error != "skipped" {
		t.Errorf("Expected non-matching test to be skipped, got %s", result.Error)
	}
}

func TestTestRunner_RunConverterTest(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	converter := func(input string) string {
		return "converted: " + input
	}

	result := runner.RunConverterTest("converter_test", converter, "input", "converted: input")

	if !result.Passed {
		t.Error("Expected converter test to pass")
	}
}

func TestTestRunner_RunGoldenTest(t *testing.T) {
	tmpDir := t.TempDir()
	goldenPath := filepath.Join(tmpDir, "test.golden")

	// Create golden file
	if err := SaveGoldenFile(goldenPath, "golden content"); err != nil {
		t.Fatalf("Failed to create golden file: %v", err)
	}

	opts := DefaultRunnerOptions()
	opts.GoldenDir = tmpDir
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunGoldenTest("test", func() (string, error) {
		return "golden content", nil
	})

	if !result.Passed {
		t.Errorf("Expected golden test to pass, got similarity %f", result.Similarity)
	}
}

func TestTestRunner_RunGoldenTest_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	opts := DefaultRunnerOptions()
	opts.GoldenDir = tmpDir
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunGoldenTest("nonexistent", func() (string, error) {
		return "content", nil
	})

	if result.Passed {
		t.Error("Expected test to fail for missing golden file")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestTestRunner_RunGoldenTest_Update(t *testing.T) {
	tmpDir := t.TempDir()

	opts := DefaultRunnerOptions()
	opts.GoldenDir = tmpDir
	opts.UpdateGolden = true
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunGoldenTest("new_golden", func() (string, error) {
		return "new content", nil
	})

	if !result.Passed {
		t.Error("Expected update to pass")
	}

	// Verify file was created
	goldenPath := filepath.Join(tmpDir, "new_golden.golden")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Golden file not created: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("Expected 'new content', got %s", string(data))
	}
}

func TestTestRunner_RunSuite(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	suite := NewTestSuite("Test Suite", "Description")
	suite.AddResult(TestResult{Name: "T1", Passed: true, Similarity: 1.0})
	suite.AddResult(TestResult{Name: "T2", Passed: false, Similarity: 0.5})

	runner.RunSuite(suite)

	if len(runner.Report.Suites) != 1 {
		t.Errorf("Expected 1 suite in report, got %d", len(runner.Report.Suites))
	}
}

func TestTestRunner_Finalize(t *testing.T) {
	tmpDir := t.TempDir()
	reportPath := filepath.Join(tmpDir, "report.json")

	opts := DefaultRunnerOptions()
	opts.ReportPath = reportPath
	runner := NewTestRunner(opts)
	defer runner.Close()

	suite := NewTestSuite("Suite", "Desc")
	suite.AddResult(TestResult{Name: "T1", Passed: true})
	suite.Finish()
	runner.Report.AddSuite(suite)

	err := runner.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// Verify report was saved
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Error("Expected report file to be created")
	}
}

func TestTestRunner_HasFailures(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	// No failures
	suite := NewTestSuite("Suite", "Desc")
	suite.AddResult(TestResult{Name: "T1", Passed: true})
	suite.Finish()
	runner.Report.AddSuite(suite)
	runner.Report.CalculateSummary()

	if runner.HasFailures() {
		t.Error("Expected no failures")
	}

	// With failure
	suite2 := NewTestSuite("Suite2", "Desc")
	suite2.AddResult(TestResult{Name: "T2", Passed: false})
	suite2.Finish()
	runner.Report.AddSuite(suite2)
	runner.Report.CalculateSummary()

	if !runner.HasFailures() {
		t.Error("Expected failures")
	}
}

func TestTestRunner_GetSummary(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	suite := NewTestSuite("Suite", "Desc")
	suite.AddResult(TestResult{Name: "T1", Passed: true})
	suite.AddResult(TestResult{Name: "T2", Passed: false})
	suite.AddResult(TestResult{Name: "T3", Error: "skipped"})
	suite.Finish()
	runner.Report.AddSuite(suite)
	runner.Report.CalculateSummary()

	summary := runner.GetSummary()

	// Should contain key statistics
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
	t.Logf("Summary: %s", summary)
}

func TestGetPlatformInfo(t *testing.T) {
	info := GetPlatformInfo()

	if info["os"] == "" {
		t.Error("Expected OS to be set")
	}
	if info["arch"] == "" {
		t.Error("Expected arch to be set")
	}
	if info["go_version"] == "" {
		t.Error("Expected go_version to be set")
	}
	if info["cgo_enabled"] == "" {
		t.Error("Expected cgo_enabled to be set")
	}

	t.Logf("Platform info: %+v", info)
}

func TestRunBenchmark(t *testing.T) {
	counter := 0
	result := RunBenchmark("test_benchmark", 100, 1024, func() {
		counter++
		time.Sleep(time.Microsecond)
	})

	if result.Name != "test_benchmark" {
		t.Errorf("Expected name 'test_benchmark', got %s", result.Name)
	}
	if result.Iterations != 100 {
		t.Errorf("Expected 100 iterations, got %d", result.Iterations)
	}
	if counter < 100 {
		t.Errorf("Expected at least 100 runs, got %d", counter)
	}
	if result.TotalTime == 0 {
		t.Error("Expected non-zero TotalTime")
	}
	if result.AvgTime == 0 {
		t.Error("Expected non-zero AvgTime")
	}
	if result.MinTime > result.MaxTime {
		t.Error("Expected MinTime <= MaxTime")
	}
	if result.BytesPerSec == 0 {
		t.Error("Expected non-zero BytesPerSec")
	}
}

func TestBenchmarkResult_String(t *testing.T) {
	result := BenchmarkResult{
		Name:        "test",
		Iterations:  1000,
		TotalTime:   time.Second,
		AvgTime:     time.Millisecond,
		MinTime:     500 * time.Microsecond,
		MaxTime:     2 * time.Millisecond,
		BytesPerSec: 10 * 1024 * 1024, // 10 MB/s
	}

	str := result.String()

	if str == "" {
		t.Error("Expected non-empty string")
	}
	// Should contain MB/s for high throughput
	if !containsStr(str, "MB/s") {
		t.Errorf("Expected MB/s in output: %s", str)
	}
	t.Logf("Benchmark result: %s", str)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTestRunner_IsCGoAvailable(t *testing.T) {
	runner := NewTestRunner(DefaultRunnerOptions())
	defer runner.Close()

	// Result should match cgo.IsCGoAvailable()
	// (may be false even if cgo available if no SWORD_PATH)
	available := runner.IsCGoAvailable()
	t.Logf("CGo comparison available: %v", available)
}

func TestTestRunner_RunSuite_Verbose(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.Verbose = true
	runner := NewTestRunner(opts)
	defer runner.Close()

	suite := NewTestSuite("Verbose Suite", "Testing verbose output")
	suite.AddResult(TestResult{Name: "PassingTest", Passed: true, Similarity: 1.0})
	suite.AddResult(TestResult{Name: "FailingTest", Passed: false, Similarity: 0.5, Diff: "short diff"})
	suite.AddResult(TestResult{Name: "SkippedTest", Error: "skipped"})
	suite.AddResult(TestResult{Name: "ErrorTest", Error: "some error occurred"})

	runner.RunSuite(suite)

	if len(runner.Report.Suites) != 1 {
		t.Errorf("Expected 1 suite in report, got %d", len(runner.Report.Suites))
	}
}

func TestTestRunner_RunSuite_FailFast(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.FailFast = true
	opts.Verbose = true
	runner := NewTestRunner(opts)
	defer runner.Close()

	suite := NewTestSuite("FailFast Suite", "Testing fail fast")
	suite.AddResult(TestResult{Name: "First", Passed: true})
	suite.AddResult(TestResult{Name: "Second", Passed: false})
	suite.AddResult(TestResult{Name: "Third", Passed: true})

	runner.RunSuite(suite)

	// Suite should be added regardless
	if len(runner.Report.Suites) != 1 {
		t.Errorf("Expected 1 suite in report, got %d", len(runner.Report.Suites))
	}
}

func TestTestRunner_Finalize_Verbose(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.Verbose = true
	runner := NewTestRunner(opts)
	defer runner.Close()

	suite := NewTestSuite("Suite", "Desc")
	suite.AddResult(TestResult{Name: "T1", Passed: true})
	suite.Finish()
	runner.Report.AddSuite(suite)

	err := runner.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}
}

func TestBenchmarkResult_String_LowThroughput(t *testing.T) {
	result := BenchmarkResult{
		Name:        "low_throughput_test",
		Iterations:  100,
		TotalTime:   time.Second,
		AvgTime:     10 * time.Millisecond,
		MinTime:     5 * time.Millisecond,
		MaxTime:     20 * time.Millisecond,
		BytesPerSec: 500, // 500 B/s
	}

	str := result.String()
	if str == "" {
		t.Error("Expected non-empty string")
	}
	// Should contain B/s for low throughput
	if !containsStr(str, "B/s") {
		t.Errorf("Expected B/s in output: %s", str)
	}
}

func TestBenchmarkResult_String_MediumThroughput(t *testing.T) {
	result := BenchmarkResult{
		Name:        "medium_throughput_test",
		Iterations:  100,
		TotalTime:   time.Second,
		AvgTime:     10 * time.Millisecond,
		MinTime:     5 * time.Millisecond,
		MaxTime:     20 * time.Millisecond,
		BytesPerSec: 50 * 1024, // 50 KB/s
	}

	str := result.String()
	if str == "" {
		t.Error("Expected non-empty string")
	}
	// Should contain KB/s for medium throughput
	if !containsStr(str, "KB/s") {
		t.Errorf("Expected KB/s in output: %s", str)
	}
}

func TestBenchmarkResult_String_ZeroThroughput(t *testing.T) {
	result := BenchmarkResult{
		Name:        "no_throughput_test",
		Iterations:  100,
		TotalTime:   time.Second,
		AvgTime:     10 * time.Millisecond,
		MinTime:     5 * time.Millisecond,
		MaxTime:     20 * time.Millisecond,
		BytesPerSec: 0, // No throughput
	}

	str := result.String()
	if str == "" {
		t.Error("Expected non-empty string")
	}
	// Should NOT contain any throughput suffix
	if containsStr(str, "B/s") || containsStr(str, "KB/s") || containsStr(str, "MB/s") {
		t.Errorf("Expected no throughput in output: %s", str)
	}
}

func TestRunBenchmark_ZeroDataSize(t *testing.T) {
	result := RunBenchmark("zero_size_test", 10, 0, func() {
		// Do nothing
	})

	if result.BytesPerSec != 0 {
		t.Errorf("Expected BytesPerSec to be 0 with zero data size, got %f", result.BytesPerSec)
	}
}

func TestTestRunner_RunGoldenTest_Error(t *testing.T) {
	tmpDir := t.TempDir()

	opts := DefaultRunnerOptions()
	opts.GoldenDir = tmpDir
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunGoldenTest("error_test", func() (string, error) {
		return "", errors.New("production error")
	})

	if result.Passed {
		t.Error("Expected test to fail when producer errors")
	}
	if result.Error != "production error" {
		t.Errorf("Expected error 'production error', got %s", result.Error)
	}
}

func TestTestRunner_RunGoldenTest_Filter(t *testing.T) {
	tmpDir := t.TempDir()

	opts := DefaultRunnerOptions()
	opts.GoldenDir = tmpDir
	opts.Filter = "specific"
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunGoldenTest("other_test", func() (string, error) {
		return "content", nil
	})

	if result.Error != "skipped" {
		t.Errorf("Expected test to be skipped due to filter, got error: %s", result.Error)
	}
}

func TestTestRunner_RunGoldenTest_UpdateError(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.GoldenDir = "/dev/null/cannot/create" // Invalid path
	opts.UpdateGolden = true
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunGoldenTest("fail_update", func() (string, error) {
		return "content", nil
	})

	if result.Passed {
		t.Error("Expected test to fail when golden update fails")
	}
	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestTestRunner_RunCGoComparisonTest_Skipped(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.SkipCGo = true
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunCGoComparisonTest("cgo_test", "KJV", "Gen.1.1", func() (string, error) {
		return "content", nil
	})

	if result.Error != "skipped" {
		t.Errorf("Expected CGo test to be skipped, got: %s", result.Error)
	}
}

func TestTestRunner_RunCGoComparisonTest_Filter(t *testing.T) {
	opts := DefaultRunnerOptions()
	opts.Filter = "specific"
	runner := NewTestRunner(opts)
	defer runner.Close()

	result := runner.RunCGoComparisonTest("other_test", "KJV", "Gen.1.1", func() (string, error) {
		return "content", nil
	})

	if result.Error != "skipped" {
		t.Errorf("Expected test to be skipped due to filter, got: %s", result.Error)
	}
}
