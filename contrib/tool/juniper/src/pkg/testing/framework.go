// Package testing provides a comprehensive test framework for validating
// SWORD/e-Sword parser output against reference implementations.
//
// The framework supports:
//   - Golden file testing for regression detection
//   - CGo libsword comparison testing (when available)
//   - Fuzzy text comparison for acceptable variations
//   - Detailed diff reporting with context
//   - Performance benchmarking
//   - Coverage tracking per book/chapter
package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TestResult represents the outcome of a single test.
type TestResult struct {
	Name       string        `json:"name"`
	Passed     bool          `json:"passed"`
	Expected   string        `json:"expected,omitempty"`
	Actual     string        `json:"actual,omitempty"`
	Diff       string        `json:"diff,omitempty"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Similarity float64       `json:"similarity"` // 0.0-1.0
}

// TestSuite represents a collection of related tests.
type TestSuite struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Results     []TestResult `json:"results"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     time.Time    `json:"end_time"`
	PassCount   int          `json:"pass_count"`
	FailCount   int          `json:"fail_count"`
	SkipCount   int          `json:"skip_count"`
}

// TestReport aggregates results from multiple test suites.
type TestReport struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Version     string      `json:"version"`
	Suites      []TestSuite `json:"suites"`
	Summary     TestSummary `json:"summary"`
}

// TestSummary provides aggregate statistics.
type TestSummary struct {
	TotalTests        int           `json:"total_tests"`
	PassedTests       int           `json:"passed_tests"`
	FailedTests       int           `json:"failed_tests"`
	SkippedTests      int           `json:"skipped_tests"`
	PassRate          float64       `json:"pass_rate"`
	TotalDuration     time.Duration `json:"total_duration"`
	AverageSimilarity float64       `json:"average_similarity"`
}

// NewTestSuite creates a new test suite.
func NewTestSuite(name, description string) *TestSuite {
	return &TestSuite{
		Name:        name,
		Description: description,
		Results:     make([]TestResult, 0),
		StartTime:   time.Now(),
	}
}

// AddResult adds a test result to the suite.
func (s *TestSuite) AddResult(result TestResult) {
	s.Results = append(s.Results, result)
	if result.Passed {
		s.PassCount++
	} else if result.Error == "skipped" {
		s.SkipCount++
	} else {
		s.FailCount++
	}
}

// Finish marks the suite as complete.
func (s *TestSuite) Finish() {
	s.EndTime = time.Now()
}

// PassRate returns the percentage of passing tests.
func (s *TestSuite) PassRate() float64 {
	total := s.PassCount + s.FailCount
	if total == 0 {
		return 100.0
	}
	return float64(s.PassCount) / float64(total) * 100.0
}

// NewTestReport creates a new test report.
func NewTestReport(version string) *TestReport {
	return &TestReport{
		GeneratedAt: time.Now(),
		Version:     version,
		Suites:      make([]TestSuite, 0),
	}
}

// AddSuite adds a test suite to the report.
func (r *TestReport) AddSuite(suite *TestSuite) {
	r.Suites = append(r.Suites, *suite)
}

// CalculateSummary computes aggregate statistics.
func (r *TestReport) CalculateSummary() {
	var totalSimilarity float64
	var similarityCount int

	for _, suite := range r.Suites {
		r.Summary.TotalTests += len(suite.Results)
		r.Summary.PassedTests += suite.PassCount
		r.Summary.FailedTests += suite.FailCount
		r.Summary.SkippedTests += suite.SkipCount
		r.Summary.TotalDuration += suite.EndTime.Sub(suite.StartTime)

		for _, result := range suite.Results {
			if result.Similarity > 0 {
				totalSimilarity += result.Similarity
				similarityCount++
			}
		}
	}

	if r.Summary.TotalTests > 0 {
		r.Summary.PassRate = float64(r.Summary.PassedTests) / float64(r.Summary.TotalTests-r.Summary.SkippedTests) * 100.0
	}

	if similarityCount > 0 {
		r.Summary.AverageSimilarity = totalSimilarity / float64(similarityCount)
	}
}

// SaveJSON saves the report as JSON.
func (r *TestReport) SaveJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// PrintSummary prints a human-readable summary.
func (r *TestReport) PrintSummary() {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("TEST REPORT SUMMARY\n")
	fmt.Printf(strings.Repeat("=", 60) + "\n")
	fmt.Printf("Generated: %s\n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("Version:   %s\n\n", r.Version)

	for _, suite := range r.Suites {
		status := "✓"
		if suite.FailCount > 0 {
			status = "✗"
		}
		fmt.Printf("%s %s: %d passed, %d failed, %d skipped (%.1f%%)\n",
			status, suite.Name, suite.PassCount, suite.FailCount, suite.SkipCount, suite.PassRate())
	}

	fmt.Printf("\n" + strings.Repeat("-", 60) + "\n")
	fmt.Printf("TOTAL: %d tests, %d passed, %d failed, %d skipped\n",
		r.Summary.TotalTests, r.Summary.PassedTests, r.Summary.FailedTests, r.Summary.SkippedTests)
	fmt.Printf("Pass Rate: %.2f%%\n", r.Summary.PassRate)
	fmt.Printf("Avg Similarity: %.2f%%\n", r.Summary.AverageSimilarity*100)
	fmt.Printf("Duration: %v\n", r.Summary.TotalDuration)
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")
}

// HasFailures returns true if any tests failed.
func (r *TestReport) HasFailures() bool {
	return r.Summary.FailedTests > 0
}

// GoldenFile represents a golden file for comparison testing.
type GoldenFile struct {
	Path     string
	Content  string
	Metadata map[string]string
}

// LoadGoldenFile loads a golden file from disk.
func LoadGoldenFile(path string) (*GoldenFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &GoldenFile{
		Path:     path,
		Content:  string(content),
		Metadata: make(map[string]string),
	}, nil
}

// SaveGoldenFile saves content as a golden file.
func SaveGoldenFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

// UpdateGoldenFile updates a golden file if UPDATE_GOLDEN env is set.
func UpdateGoldenFile(path, content string) error {
	if os.Getenv("UPDATE_GOLDEN") != "" {
		return SaveGoldenFile(path, content)
	}
	return nil
}
