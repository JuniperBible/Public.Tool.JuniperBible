package web

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/FocuswithJustin/JuniperBible/internal/archive"
)

// ProfileResult holds timing data for a single operation.
type ProfileResult struct {
	Name     string
	Duration time.Duration
	Count    int
}

// TestProfileStartup profiles the startup sequence with detailed timing.
func TestProfileStartup(t *testing.T) {
	// Use actual capsules directory for realistic profiling
	capsulesDir := os.Getenv("CAPSULES_DIR")
	if capsulesDir == "" {
		// Try common locations
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "Programming", "Workspace", "JuniperBible", "capsules"),
			filepath.Join(home, ".juniper", "capsules"),
			filepath.Join(home, ".juniper"),
			filepath.Join(home, "capsules"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				capsulesDir = c
				break
			}
		}
	}

	if _, err := os.Stat(capsulesDir); os.IsNotExist(err) {
		t.Skipf("Capsules directory not found: %s (set CAPSULES_DIR env var)", capsulesDir)
	}

	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = capsulesDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	// Clear all caches
	clearAllCaches()

	var results []ProfileResult
	var mu sync.Mutex
	addResult := func(name string, d time.Duration, count int) {
		mu.Lock()
		results = append(results, ProfileResult{name, d, count})
		mu.Unlock()
	}

	totalStart := time.Now()

	// Profile 1: List capsules (directory walk)
	t.Log("=== Profiling listCapsules ===")
	start := time.Now()
	capsules := listCapsulesUncached()
	addResult("listCapsulesUncached", time.Since(start), len(capsules))
	t.Logf("  listCapsulesUncached: %v (%d capsules)", time.Since(start), len(capsules))

	// Profile 2: Capsule metadata scanning (HasIR checks)
	t.Log("\n=== Profiling capsule metadata scanning ===")
	start = time.Now()
	var hasIRCount int
	for _, c := range capsules {
		fullPath := filepath.Join(capsulesDir, c.Path)
		flags, _ := archive.ScanCapsuleFlags(fullPath)
		if flags.HasIR {
			hasIRCount++
		}
	}
	addResult("ScanCapsuleFlags (all)", time.Since(start), len(capsules))
	t.Logf("  ScanCapsuleFlags (all): %v (%d capsules, %d have IR)", time.Since(start), len(capsules), hasIRCount)

	// Profile 3: Bible list building
	t.Log("\n=== Profiling listBiblesUncached ===")
	clearAllCaches()
	start = time.Now()
	bibles := listBiblesUncached()
	addResult("listBiblesUncached", time.Since(start), len(bibles))
	t.Logf("  listBiblesUncached: %v (%d bibles)", time.Since(start), len(bibles))

	// Profile 4: Individual corpus loading (the main bottleneck)
	t.Log("\n=== Profiling individual corpus loads ===")
	if len(bibles) > 0 {
		// Measure first 5 bibles individually
		maxToProfile := 5
		if len(bibles) < maxToProfile {
			maxToProfile = len(bibles)
		}

		for i := 0; i < maxToProfile; i++ {
			b := bibles[i]
			clearAllCaches() // Clear to force reload
			start = time.Now()
			_, _, err := getCachedCorpus(b.ID)
			d := time.Since(start)
			if err != nil {
				t.Logf("  %s: ERROR - %v", b.ID, err)
			} else {
				addResult(fmt.Sprintf("getCachedCorpus(%s)", b.ID), d, 1)
				t.Logf("  %s: %v", b.ID, d)
			}
		}
	}

	// Profile 5: Parallel corpus loading (simulating startup)
	t.Log("\n=== Profiling parallel corpus loading ===")
	clearAllCaches()
	start = time.Now()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, bible := range bibles {
		wg.Add(1)
		go func(b BibleInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			getCachedCorpus(b.ID)
		}(bible)
	}
	wg.Wait()
	addResult("Parallel corpus load (all)", time.Since(start), len(bibles))
	t.Logf("  Parallel corpus load: %v (%d bibles)", time.Since(start), len(bibles))

	// Profile 6: SWORD modules listing (if applicable)
	t.Log("\n=== Profiling SWORD modules ===")
	origSwordDir := ServerConfig.SwordDir
	home, _ := os.UserHomeDir()
	ServerConfig.SwordDir = filepath.Join(home, ".sword")
	invalidateManageableBiblesCache()
	start = time.Now()
	swordModules := listSWORDModulesUncached()
	addResult("listSWORDModulesUncached", time.Since(start), len(swordModules))
	t.Logf("  listSWORDModulesUncached: %v (%d modules)", time.Since(start), len(swordModules))
	ServerConfig.SwordDir = origSwordDir

	// Profile 7: Manageable bibles (combined installed + installable)
	t.Log("\n=== Profiling manageable bibles ===")
	invalidateManageableBiblesCache()
	start = time.Now()
	installed, installable := listManageableBiblesUncached()
	addResult("listManageableBiblesUncached", time.Since(start), len(installed)+len(installable))
	t.Logf("  listManageableBiblesUncached: %v (%d installed, %d installable)", time.Since(start), len(installed), len(installable))

	totalDuration := time.Since(totalStart)

	// Summary
	t.Log("\n=== SUMMARY ===")
	sort.Slice(results, func(i, j int) bool {
		return results[i].Duration > results[j].Duration
	})
	for _, r := range results {
		perItem := time.Duration(0)
		if r.Count > 0 {
			perItem = r.Duration / time.Duration(r.Count)
		}
		t.Logf("  %-40s %12v  (%d items, %v/item)", r.Name, r.Duration, r.Count, perItem)
	}
	t.Logf("\n  TOTAL: %v", totalDuration)

	// Performance target check
	if len(bibles) > 0 {
		perBible := totalDuration / time.Duration(len(bibles))
		t.Logf("\n  Per-Bible average: %v (target: <500ms)", perBible)
		if perBible > 500*time.Millisecond {
			t.Logf("  WARNING: Exceeds 0.5s per Bible target!")
		}
	}
}

// TestProfileArchiveReading profiles archive reading in detail.
func TestProfileArchiveReading(t *testing.T) {
	capsulesDir := os.Getenv("CAPSULES_DIR")
	if capsulesDir == "" {
		home, _ := os.UserHomeDir()
		capsulesDir = filepath.Join(home, ".juniper", "capsules")
	}

	if _, err := os.Stat(capsulesDir); os.IsNotExist(err) {
		t.Skipf("Capsules directory not found: %s", capsulesDir)
	}

	entries, err := os.ReadDir(capsulesDir)
	if err != nil {
		t.Fatalf("Failed to read capsules dir: %v", err)
	}

	// Find capsule files
	var capsuleFiles []string
	for _, e := range entries {
		if !e.IsDir() && archive.IsSupportedFormat(e.Name()) {
			capsuleFiles = append(capsuleFiles, filepath.Join(capsulesDir, e.Name()))
		}
	}

	if len(capsuleFiles) == 0 {
		t.Skip("No capsule files found")
	}

	t.Logf("Found %d capsule files", len(capsuleFiles))

	// Profile reading first 10 capsules
	maxToProfile := 10
	if len(capsuleFiles) < maxToProfile {
		maxToProfile = len(capsuleFiles)
	}

	// Clear TOC cache
	archive.ClearTOCCache()

	t.Log("\n=== Profiling archive reads (cold cache) ===")
	var coldTimes []time.Duration
	for i := 0; i < maxToProfile; i++ {
		path := capsuleFiles[i]
		name := filepath.Base(path)

		// Time ScanCapsuleFlags
		start := time.Now()
		flags, _ := archive.ScanCapsuleFlags(path)
		scanTime := time.Since(start)

		// Time ReadIR
		start = time.Now()
		_, readErr := archive.ReadIR(path)
		readTime := time.Since(start)

		totalTime := scanTime + readTime
		coldTimes = append(coldTimes, totalTime)
		errStr := ""
		if readErr != nil {
			errStr = " ERROR"
		}
		t.Logf("  %s: scan=%v, readIR=%v, total=%v (HasIR=%v)%s",
			name, scanTime, readTime, totalTime, flags.HasIR, errStr)
	}

	// Now profile with warm TOC cache
	t.Log("\n=== Profiling archive reads (warm TOC cache) ===")
	var warmTimes []time.Duration
	for i := 0; i < maxToProfile; i++ {
		path := capsuleFiles[i]
		name := filepath.Base(path)

		start := time.Now()
		flags, _ := archive.ScanCapsuleFlags(path)
		scanTime := time.Since(start)

		// Note: ReadIR doesn't use TOC cache, it still reads the file
		start = time.Now()
		_, _ = archive.ReadIR(path)
		readTime := time.Since(start)

		totalTime := scanTime + readTime
		warmTimes = append(warmTimes, totalTime)
		t.Logf("  %s: scan=%v, readIR=%v, total=%v (HasIR=%v)",
			name, scanTime, readTime, totalTime, flags.HasIR)
	}

	// Calculate averages
	var coldTotal, warmTotal time.Duration
	for i := range coldTimes {
		coldTotal += coldTimes[i]
		warmTotal += warmTimes[i]
	}
	t.Logf("\n=== AVERAGES ===")
	t.Logf("  Cold cache: %v/capsule", coldTotal/time.Duration(len(coldTimes)))
	t.Logf("  Warm cache: %v/capsule", warmTotal/time.Duration(len(warmTimes)))
	t.Logf("  Speedup: %.2fx", float64(coldTotal)/float64(warmTotal))
}

// TestProfileJSONParsing profiles JSON parsing of IR files.
func TestProfileJSONParsing(t *testing.T) {
	capsulesDir := os.Getenv("CAPSULES_DIR")
	if capsulesDir == "" {
		home, _ := os.UserHomeDir()
		capsulesDir = filepath.Join(home, ".juniper", "capsules")
	}

	if _, err := os.Stat(capsulesDir); os.IsNotExist(err) {
		t.Skipf("Capsules directory not found: %s", capsulesDir)
	}

	origCapsulesDir := ServerConfig.CapsulesDir
	ServerConfig.CapsulesDir = capsulesDir
	defer func() {
		ServerConfig.CapsulesDir = origCapsulesDir
	}()

	clearAllCaches()
	bibles := listBiblesUncached()

	if len(bibles) == 0 {
		t.Skip("No bibles found")
	}

	t.Logf("Found %d bibles", len(bibles))

	// Profile first 5
	maxToProfile := 5
	if len(bibles) < maxToProfile {
		maxToProfile = len(bibles)
	}

	for i := 0; i < maxToProfile; i++ {
		b := bibles[i]

		// Find capsule path
		capsules := listCapsules()
		var capsulePath string
		for _, c := range capsules {
			id := archive.ExtractCapsuleID(c.Name)
			if id == b.ID {
				capsulePath = filepath.Join(capsulesDir, c.Path)
				break
			}
		}
		if capsulePath == "" {
			continue
		}

		// Time archive read
		start := time.Now()
		irContent, err := archive.ReadIR(capsulePath)
		readTime := time.Since(start)
		if err != nil {
			t.Logf("  %s: read error: %v", b.ID, err)
			continue
		}

		// Time parsing
		start = time.Now()
		corpus := parseIRToCorpus(irContent)
		parseTime := time.Since(start)

		docCount := 0
		verseCount := 0
		if corpus != nil {
			docCount = len(corpus.Documents)
			for _, doc := range corpus.Documents {
				verseCount += len(doc.ContentBlocks)
			}
		}

		t.Logf("  %s: read=%v, parse=%v, total=%v (%d docs, %d verses)",
			b.ID, readTime, parseTime, readTime+parseTime, docCount, verseCount)
	}
}
