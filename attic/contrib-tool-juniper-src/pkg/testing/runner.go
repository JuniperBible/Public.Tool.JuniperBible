// Package testing provides a comprehensive test framework for validating
// SWORD/e-Sword parser output against reference implementations.
package testing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/FocuswithJustin/juniper/pkg/cgo"
)

// TestRunner coordinates test execution across different test types.
type TestRunner struct {
	Report       *TestReport
	Options      RunnerOptions
	swordMgr     *cgo.SwordManager
	cgoAvailable bool
}

// RunnerOptions configures test execution.
type RunnerOptions struct {
	// Verbose enables detailed output
	Verbose bool

	// UpdateGolden updates golden files instead of comparing
	UpdateGolden bool

	// SkipCGo skips CGo comparison tests
	SkipCGo bool

	// SwordPath is the path to SWORD module directory
	SwordPath string

	// GoldenDir is the directory containing golden files
	GoldenDir string

	// ReportPath is where to save the JSON report
	ReportPath string

	// CompareOptions for text comparison
	CompareOpts CompareOptions

	// FailFast stops on first failure
	FailFast bool

	// Filter runs only tests matching this pattern
	Filter string
}

// DefaultRunnerOptions returns sensible defaults.
func DefaultRunnerOptions() RunnerOptions {
	return RunnerOptions{
		Verbose:      false,
		UpdateGolden: os.Getenv("UPDATE_GOLDEN") != "",
		SkipCGo:      os.Getenv("SKIP_CGO") != "",
		SwordPath:    os.Getenv("SWORD_PATH"),
		GoldenDir:    "testdata/golden",
		ReportPath:   "",
		CompareOpts:  DefaultCompareOptions(),
		FailFast:     false,
		Filter:       "",
	}
}

// NewTestRunner creates a new test runner.
func NewTestRunner(opts RunnerOptions) *TestRunner {
	runner := &TestRunner{
		Report:       NewTestReport("1.0.0"),
		Options:      opts,
		cgoAvailable: cgo.IsCGoAvailable(),
	}

	// Try to initialize CGo if available and not skipped
	if runner.cgoAvailable && !opts.SkipCGo && opts.SwordPath != "" {
		mgr, err := cgo.NewSwordManager(opts.SwordPath)
		if err == nil {
			runner.swordMgr = mgr
		}
	}

	return runner
}

// Close releases resources.
func (r *TestRunner) Close() {
	if r.swordMgr != nil {
		r.swordMgr.Close()
	}
}

// IsCGoAvailable returns whether CGo comparison is available.
func (r *TestRunner) IsCGoAvailable() bool {
	return r.cgoAvailable && r.swordMgr != nil
}

// RunParserTest runs a single parser test.
func (r *TestRunner) RunParserTest(name string, parseFunc func() (string, error), expected string) TestResult {
	start := time.Now()

	result := TestResult{
		Name:     name,
		Expected: expected,
	}

	// Apply filter if set
	if r.Options.Filter != "" && !strings.Contains(name, r.Options.Filter) {
		result.Error = "skipped"
		return result
	}

	// Run the parser
	actual, err := parseFunc()
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	result.Actual = actual

	// Compare results
	compareResult := Compare(expected, actual, r.Options.CompareOpts)
	result.Similarity = compareResult.Similarity
	result.Passed = compareResult.Match

	if !result.Passed {
		result.Diff = compareResult.Diff
	}

	result.Duration = time.Since(start)
	return result
}

// RunConverterTest runs a markup converter test.
func (r *TestRunner) RunConverterTest(name string, converter func(string) string, input, expected string) TestResult {
	return r.RunParserTest(name, func() (string, error) {
		return converter(input), nil
	}, expected)
}

// RunGoldenTest compares output against a golden file.
func (r *TestRunner) RunGoldenTest(name string, produceFunc func() (string, error)) TestResult {
	start := time.Now()

	result := TestResult{
		Name: name,
	}

	// Apply filter
	if r.Options.Filter != "" && !strings.Contains(name, r.Options.Filter) {
		result.Error = "skipped"
		return result
	}

	// Produce the output
	actual, err := produceFunc()
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	result.Actual = actual

	// Determine golden file path
	goldenPath := filepath.Join(r.Options.GoldenDir, name+".golden")

	// Update golden file if requested
	if r.Options.UpdateGolden {
		if err := SaveGoldenFile(goldenPath, actual); err != nil {
			result.Error = fmt.Sprintf("failed to update golden file: %v", err)
			result.Duration = time.Since(start)
			return result
		}
		result.Passed = true
		result.Similarity = 1.0
		result.Duration = time.Since(start)
		return result
	}

	// Load golden file
	golden, err := LoadGoldenFile(goldenPath)
	if err != nil {
		result.Error = fmt.Sprintf("golden file not found: %v (run with UPDATE_GOLDEN=1 to create)", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Expected = golden.Content

	// Compare
	compareResult := Compare(golden.Content, actual, r.Options.CompareOpts)
	result.Similarity = compareResult.Similarity
	result.Passed = compareResult.Match

	if !result.Passed {
		result.Diff = compareResult.Diff
	}

	result.Duration = time.Since(start)
	return result
}

// RunCGoComparisonTest compares pure Go output against libsword.
func (r *TestRunner) RunCGoComparisonTest(name, moduleName, reference string, pureGoFunc func() (string, error)) TestResult {
	start := time.Now()

	result := TestResult{
		Name: name,
	}

	// Apply filter
	if r.Options.Filter != "" && !strings.Contains(name, r.Options.Filter) {
		result.Error = "skipped"
		return result
	}

	// Skip if CGo not available
	if !r.IsCGoAvailable() {
		result.Error = "skipped"
		if r.Options.Verbose {
			fmt.Printf("  [SKIP] %s: CGo not available\n", name)
		}
		return result
	}

	// Get reference output from libsword
	mod, err := r.swordMgr.GetModule(moduleName)
	if err != nil {
		result.Error = fmt.Sprintf("module not found: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	expected, err := mod.GetVerse(reference)
	if err != nil {
		result.Error = fmt.Sprintf("libsword error: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Expected = expected

	// Get pure Go output
	actual, err := pureGoFunc()
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	result.Actual = actual

	// Compare
	compareResult := Compare(expected, actual, r.Options.CompareOpts)
	result.Similarity = compareResult.Similarity
	result.Passed = compareResult.Match

	if !result.Passed {
		result.Diff = compareResult.Diff
	}

	result.Duration = time.Since(start)
	return result
}

// RunSuite runs a complete test suite.
func (r *TestRunner) RunSuite(suite *TestSuite) {
	if r.Options.Verbose {
		fmt.Printf("\n=== %s ===\n", suite.Name)
		fmt.Printf("%s\n\n", suite.Description)
	}

	for i := range suite.Results {
		result := &suite.Results[i]
		if r.Options.Verbose {
			if result.Passed {
				fmt.Printf("  [PASS] %s (%.2f%% similar, %v)\n",
					result.Name, result.Similarity*100, result.Duration)
			} else if result.Error == "skipped" {
				fmt.Printf("  [SKIP] %s\n", result.Name)
			} else if result.Error != "" {
				fmt.Printf("  [FAIL] %s: %s\n", result.Name, result.Error)
			} else {
				fmt.Printf("  [FAIL] %s (%.2f%% similar)\n",
					result.Name, result.Similarity*100)
				if result.Diff != "" && len(result.Diff) < 500 {
					fmt.Printf("%s\n", result.Diff)
				}
			}
		}

		if !result.Passed && result.Error != "skipped" && r.Options.FailFast {
			break
		}
	}

	suite.Finish()
	r.Report.AddSuite(suite)
}

// Finalize completes the report and optionally saves it.
func (r *TestRunner) Finalize() error {
	r.Report.CalculateSummary()

	if r.Options.Verbose {
		r.Report.PrintSummary()
	}

	if r.Options.ReportPath != "" {
		if err := r.Report.SaveJSON(r.Options.ReportPath); err != nil {
			return fmt.Errorf("failed to save report: %w", err)
		}
	}

	return nil
}

// HasFailures returns true if any tests failed.
func (r *TestRunner) HasFailures() bool {
	return r.Report.HasFailures()
}

// GetSummary returns a formatted summary string.
func (r *TestRunner) GetSummary() string {
	s := r.Report.Summary
	return fmt.Sprintf("%d passed, %d failed, %d skipped (%.1f%% pass rate)",
		s.PassedTests, s.FailedTests, s.SkippedTests, s.PassRate)
}

// GetPlatformInfo returns information about the test environment.
func GetPlatformInfo() map[string]string {
	return map[string]string{
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"go_version":  runtime.Version(),
		"cgo_enabled": fmt.Sprintf("%v", cgo.IsCGoAvailable()),
		"num_cpu":     fmt.Sprintf("%d", runtime.NumCPU()),
	}
}

// BenchmarkResult stores benchmark metrics.
type BenchmarkResult struct {
	Name        string
	Iterations  int
	TotalTime   time.Duration
	AvgTime     time.Duration
	MinTime     time.Duration
	MaxTime     time.Duration
	BytesPerSec float64
}

// RunBenchmark runs a performance benchmark.
func RunBenchmark(name string, iterations int, dataSize int, fn func()) BenchmarkResult {
	result := BenchmarkResult{
		Name:       name,
		Iterations: iterations,
		MinTime:    time.Hour, // Start high
	}

	// Warm up
	for i := 0; i < 3; i++ {
		fn()
	}

	// Actual benchmark
	start := time.Now()
	for i := 0; i < iterations; i++ {
		iterStart := time.Now()
		fn()
		iterTime := time.Since(iterStart)

		if iterTime < result.MinTime {
			result.MinTime = iterTime
		}
		if iterTime > result.MaxTime {
			result.MaxTime = iterTime
		}
	}
	result.TotalTime = time.Since(start)
	result.AvgTime = result.TotalTime / time.Duration(iterations)

	if dataSize > 0 && result.AvgTime > 0 {
		result.BytesPerSec = float64(dataSize) / result.AvgTime.Seconds()
	}

	return result
}

// FormatBenchmark returns a formatted benchmark result.
func (b BenchmarkResult) String() string {
	throughput := ""
	if b.BytesPerSec > 0 {
		if b.BytesPerSec > 1024*1024 {
			throughput = fmt.Sprintf(", %.2f MB/s", b.BytesPerSec/1024/1024)
		} else if b.BytesPerSec > 1024 {
			throughput = fmt.Sprintf(", %.2f KB/s", b.BytesPerSec/1024)
		} else {
			throughput = fmt.Sprintf(", %.2f B/s", b.BytesPerSec)
		}
	}
	return fmt.Sprintf("%s: %d iterations, avg=%v, min=%v, max=%v%s",
		b.Name, b.Iterations, b.AvgTime, b.MinTime, b.MaxTime, throughput)
}
