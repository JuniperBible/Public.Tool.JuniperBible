package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrPackageNotAvailable indicates a module exists in the index but has no downloadable package
var ErrPackageNotAvailable = errors.New("package not available for download")

// ModuleUpdate contains update information for an installed module
type ModuleUpdate struct {
	Module           ModuleInfo // Available module info
	InstalledVersion string     // Currently installed version
	AvailableVersion string     // Version available from source
}

// HasUpdate returns true if an update is available
func (u *ModuleUpdate) HasUpdate() bool {
	return u.InstalledVersion != u.AvailableVersion
}

// ProgressFunc is called during operations to report progress
type InstallerProgressFunc func(step, total int, message string)

// Installer handles module installation and uninstallation
type Installer struct {
	config     *LocalConfig
	client     *Client
	OnProgress InstallerProgressFunc
	Verbose    bool
}

// NewInstaller creates a new module installer
func NewInstaller(config *LocalConfig, client *Client) *Installer {
	return &Installer{
		config: config,
		client: client,
	}
}

// Install downloads and installs a module from a source
func (i *Installer) Install(ctx context.Context, source Source, module ModuleInfo) error {
	if i.OnProgress != nil {
		i.OnProgress(1, 3, fmt.Sprintf("Downloading %s...", module.ID))
	}

	// Try multiple possible package URLs (different sources have different structures)
	packageURLs := source.ModulePackageURLs(module.ID)

	var data []byte
	var lastErr error
	allNotFound := true
	for _, packageURL := range packageURLs {
		var err error
		data, err = i.client.Download(ctx, packageURL)
		if err == nil {
			break
		}
		lastErr = err
		// Track if all errors are "not found" vs other errors
		if !IsNotFoundError(err) {
			allNotFound = false
		}
	}

	if data == nil {
		// If all URLs returned 404/550, the package doesn't exist
		if allNotFound {
			return fmt.Errorf("%w: %s (tried %d URLs)", ErrPackageNotAvailable, module.ID, len(packageURLs))
		}
		return fmt.Errorf("downloading module package: %w", lastErr)
	}

	if i.OnProgress != nil {
		i.OnProgress(2, 3, fmt.Sprintf("Installing %s...", module.ID))
	}

	// Extract module zip to destination
	destDir := i.config.SwordDir
	if err := ExtractZipArchive(data, destDir); err != nil {
		return fmt.Errorf("extracting module package: %w", err)
	}

	if i.OnProgress != nil {
		i.OnProgress(3, 3, fmt.Sprintf("Installed %s successfully", module.ID))
	}

	return nil
}

// InstallResult contains the result of installing a single module
type InstallResult struct {
	Module ModuleInfo
	Source Source
	Status string // "done", "skipped", "unavailable", "failed"
	Error  error
}

// BatchInstallOptions configures batch installation behavior
type BatchInstallOptions struct {
	Workers       int                 // Number of parallel workers (default: 4)
	SkipInstalled bool                // Skip already installed modules
	OnResult      func(InstallResult) // Called after each module completes
}

// InstallBatch installs multiple modules in parallel
func (i *Installer) InstallBatch(ctx context.Context, source Source, modules []ModuleInfo, opts BatchInstallOptions) []InstallResult {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}

	// Get list of installed modules for skip check
	var installedSet map[string]bool
	if opts.SkipInstalled {
		installedSet = make(map[string]bool)
		if installed, err := i.config.ListInstalledModules(); err == nil {
			for _, m := range installed {
				installedSet[strings.ToUpper(m.ID)] = true
			}
		}
	}

	// Create work channel and results
	jobs := make(chan ModuleInfo, len(modules))
	results := make([]InstallResult, 0, len(modules))
	resultsMu := sync.Mutex{}

	// Create a separate client for each worker to avoid connection issues
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < opts.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each worker gets its own client
			workerClient, err := NewClient(ClientOptions{
				Timeout: i.client.opts.Timeout,
			})
			if err != nil {
				return
			}
			workerInstaller := &Installer{
				config: i.config,
				client: workerClient,
			}

			for module := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				result := InstallResult{
					Module: module,
					Source: source,
				}

				// Check if already installed
				if opts.SkipInstalled && installedSet[strings.ToUpper(module.ID)] {
					result.Status = "skipped"
				} else {
					// Attempt installation
					err := workerInstaller.Install(ctx, source, module)
					if err == nil {
						result.Status = "done"
					} else if errors.Is(err, ErrPackageNotAvailable) {
						result.Status = "unavailable"
						result.Error = err
					} else {
						result.Status = "failed"
						result.Error = err
					}
				}

				// Store result
				resultsMu.Lock()
				results = append(results, result)
				resultsMu.Unlock()

				// Call progress callback if set
				if opts.OnResult != nil {
					opts.OnResult(result)
				}
			}
		}()
	}

	// Send jobs
	for _, module := range modules {
		jobs <- module
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()

	return results
}

// Uninstall removes an installed module
func (i *Installer) Uninstall(moduleID string) error {
	module, found := i.config.GetInstalledModule(moduleID)
	if !found {
		return fmt.Errorf("module %s is not installed", moduleID)
	}

	// Remove data directory
	dataPath := i.config.GetModuleDataPath(module.DataPath)
	if err := os.RemoveAll(dataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing data directory: %w", err)
	}

	// Remove conf file
	if module.ConfPath != "" {
		if err := os.Remove(module.ConfPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing conf file: %w", err)
		}
	} else {
		// Try to find and remove conf file by name
		confPath := filepath.Join(i.config.ModsDir(), strings.ToLower(moduleID)+".conf")
		os.Remove(confPath)
	}

	return nil
}

// RefreshSource downloads and parses the module index from a source
func (i *Installer) RefreshSource(ctx context.Context, source Source) ([]ModuleInfo, error) {
	indexURL := source.ModsIndexURL()

	data, err := i.client.Download(ctx, indexURL)
	if err != nil {
		return nil, fmt.Errorf("downloading module index: %w", err)
	}

	modules, err := ParseModsArchive(data)
	if err != nil {
		return nil, fmt.Errorf("parsing module index: %w", err)
	}

	return modules, nil
}

// ListAvailable returns available modules from a source
func (i *Installer) ListAvailable(ctx context.Context, source Source) ([]ModuleInfo, error) {
	return i.RefreshSource(ctx, source)
}

// CheckUpdates compares installed modules with available versions
func (i *Installer) CheckUpdates(ctx context.Context, source Source) ([]ModuleUpdate, error) {
	available, err := i.RefreshSource(ctx, source)
	if err != nil {
		return nil, err
	}

	installed, err := i.config.ListInstalledModules()
	if err != nil {
		return nil, err
	}

	// Build map of installed modules
	installedMap := make(map[string]ModuleInfo)
	for _, m := range installed {
		installedMap[m.ID] = m
	}

	var updates []ModuleUpdate
	for _, avail := range available {
		if inst, ok := installedMap[avail.ID]; ok {
			if inst.Version != avail.Version {
				updates = append(updates, ModuleUpdate{
					Module:           avail,
					InstalledVersion: inst.Version,
					AvailableVersion: avail.Version,
				})
			}
		}
	}

	return updates, nil
}

// InstallConf installs just a conf file (without module data)
func (i *Installer) InstallConf(moduleID string, content []byte) error {
	confPath := filepath.Join(i.config.ModsDir(), strings.ToLower(moduleID)+".conf")
	return os.WriteFile(confPath, content, 0600)
}

// RemoveConf removes a conf file
func (i *Installer) RemoveConf(moduleID string) error {
	confPath := filepath.Join(i.config.ModsDir(), strings.ToLower(moduleID)+".conf")
	return os.Remove(confPath)
}

// ValidateModule checks if a module is properly installed
func (i *Installer) ValidateModule(moduleID string) (bool, error) {
	module, found := i.config.GetInstalledModule(moduleID)
	if !found {
		return false, fmt.Errorf("module %s is not installed", moduleID)
	}

	// Check if data path exists
	dataPath := i.config.GetModuleDataPath(module.DataPath)
	info, err := os.Stat(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if !info.IsDir() {
		return false, nil
	}

	// Check for expected data files based on driver type
	switch {
	case strings.HasPrefix(strings.ToLower(module.Driver), "ztext"):
		// Check for OT or NT files
		hasOT := fileExists(filepath.Join(dataPath, "ot.bzs"))
		hasNT := fileExists(filepath.Join(dataPath, "nt.bzs"))
		return hasOT || hasNT, nil
	case strings.HasPrefix(strings.ToLower(module.Driver), "zld"):
		// Check for dictionary files
		hasIdx := fileExists(filepath.Join(dataPath, "dict.idx")) ||
			fileExists(filepath.Join(dataPath, "dict.zdx"))
		return hasIdx, nil
	default:
		// For other types, just check directory isn't empty
		entries, err := os.ReadDir(dataPath)
		if err != nil {
			return false, err
		}
		return len(entries) > 0, nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ModuleVerification contains detailed verification results for a module
type ModuleVerification struct {
	ModuleID     string
	Installed    bool   // Conf file exists
	DataExists   bool   // Data directory exists with files
	SizeMatch    bool   // Actual size matches InstallSize
	ExpectedSize int64  // InstallSize from conf
	ActualSize   int64  // Actual size on disk
	Error        string // Any error encountered
}

// IsValid returns true if the module passes all verification checks
func (v *ModuleVerification) IsValid() bool {
	return v.Installed && v.DataExists && (v.ExpectedSize == 0 || v.SizeMatch)
}

// VerifyModule performs comprehensive verification of an installed module
func (i *Installer) VerifyModule(moduleID string) ModuleVerification {
	result := ModuleVerification{ModuleID: moduleID}

	if i.Verbose {
		fmt.Printf("[verbose] VerifyModule: looking up %s in config\n", moduleID)
	}
	module, found := i.config.GetInstalledModule(moduleID)
	if !found {
		if i.Verbose {
			fmt.Printf("[verbose] VerifyModule: %s not found in config\n", moduleID)
		}
		result.Error = "module not installed"
		return result
	}
	result.Installed = true
	result.ExpectedSize = module.InstallSize
	if i.Verbose {
		fmt.Printf("[verbose] VerifyModule: %s found, DataPath=%s, ExpectedSize=%d\n", moduleID, module.DataPath, module.InstallSize)
	}

	// Check data directory exists and has files
	dataPath := i.config.GetModuleDataPath(module.DataPath)
	if i.Verbose {
		fmt.Printf("[verbose] VerifyModule: checking data path %s\n", dataPath)
	}
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		if i.Verbose {
			fmt.Printf("[verbose] VerifyModule: ReadDir error: %v\n", err)
		}
		result.Error = fmt.Sprintf("cannot read data directory: %v", err)
		return result
	}
	result.DataExists = len(entries) > 0
	if i.Verbose {
		fmt.Printf("[verbose] VerifyModule: found %d entries in data dir\n", len(entries))
	}

	// Calculate actual size
	if i.Verbose {
		fmt.Printf("[verbose] VerifyModule: calculating actual size...\n")
	}
	actualSize, err := i.config.GetModuleActualSize(module.DataPath)
	if err != nil {
		if i.Verbose {
			fmt.Printf("[verbose] VerifyModule: size calculation error: %v\n", err)
		}
		result.Error = fmt.Sprintf("cannot calculate size: %v", err)
		return result
	}
	result.ActualSize = actualSize
	if i.Verbose {
		fmt.Printf("[verbose] VerifyModule: actual size = %d bytes\n", actualSize)
	}

	// Check size match (if expected size is specified)
	if module.InstallSize > 0 {
		result.SizeMatch = actualSize == module.InstallSize
		if i.Verbose {
			fmt.Printf("[verbose] VerifyModule: size match = %v (expected %d, actual %d)\n", result.SizeMatch, module.InstallSize, actualSize)
		}
	} else {
		result.SizeMatch = true // No expected size to compare
		if i.Verbose {
			fmt.Printf("[verbose] VerifyModule: no expected size, skipping size check\n")
		}
	}

	return result
}

// VerifyAllModules verifies all installed modules
func (i *Installer) VerifyAllModules() ([]ModuleVerification, error) {
	modules, err := i.config.ListInstalledModules()
	if err != nil {
		return nil, err
	}

	var results []ModuleVerification
	for _, m := range modules {
		results = append(results, i.VerifyModule(m.ID))
	}

	return results, nil
}

func generateConfContent(module ModuleInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]\n", module.ID))
	sb.WriteString(fmt.Sprintf("DataPath=%s\n", module.DataPath))
	if module.Driver != "" {
		sb.WriteString(fmt.Sprintf("ModDrv=%s\n", module.Driver))
	}
	if module.Language != "" {
		sb.WriteString(fmt.Sprintf("Lang=%s\n", module.Language))
	}
	if module.Description != "" {
		sb.WriteString(fmt.Sprintf("Description=%s\n", module.Description))
	}
	if module.Version != "" {
		sb.WriteString(fmt.Sprintf("Version=%s\n", module.Version))
	}
	if module.SourceType != "" {
		sb.WriteString(fmt.Sprintf("SourceType=%s\n", module.SourceType))
	}
	if module.Encoding != "" {
		sb.WriteString(fmt.Sprintf("Encoding=%s\n", module.Encoding))
	}
	for _, feature := range module.Features {
		sb.WriteString(fmt.Sprintf("Feature=%s\n", feature))
	}
	return sb.String()
}
