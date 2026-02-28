package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/FocuswithJustin/juniper/pkg/repository"
)

// RepoCmd manages SWORD module repositories
type RepoCmd struct {
	SwordPath string `name:"sword-path" help:"SWORD directory path (default: ~/.sword)"`

	// Subcommands
	ListSources ListSourcesCmd `cmd:"" name:"list-sources" help:"List available remote sources"`
	Refresh     RefreshCmd     `cmd:"" help:"Refresh module index from a source"`
	List        ListCmd        `cmd:"" help:"List available modules from a source"`
	Install     InstallCmd     `cmd:"" help:"Install a module from a source"`
	InstallAll  InstallAllCmd  `cmd:"" name:"install-all" help:"Install all modules from a source in parallel"`
	InstallMega InstallMegaCmd `cmd:"" name:"install-mega" help:"Install all modules from ALL sources in parallel"`
	Installed   InstalledCmd   `cmd:"" help:"List installed modules"`
	Uninstall   UninstallCmd   `cmd:"" help:"Uninstall a module"`
	Verify      VerifyCmd      `cmd:"" help:"Verify installed module integrity"`
}

// ListSourcesCmd lists available remote sources
type ListSourcesCmd struct{}

func (l *ListSourcesCmd) Run(repo *RepoCmd) error {
	sources := repository.DefaultSources()

	// Find max name length for alignment
	maxLen := 0
	for _, s := range sources {
		if len(s.Name) > maxLen {
			maxLen = len(s.Name)
		}
	}

	fmt.Printf("%-*s  %-4s  %-25s  %s\n", maxLen, "SOURCE", "TYPE", "HOST", "DIRECTORY")
	fmt.Printf("%s\n", strings.Repeat("-", maxLen+4+25+40))

	for _, s := range sources {
		fmt.Printf("%-*s  %-4s  %-25s  %s\n", maxLen, s.Name, s.Type, s.Host, s.Directory)
	}

	return nil
}

// RefreshCmd refreshes module index from a source
type RefreshCmd struct {
	Source string `arg:"" required:"" help:"Source name to refresh"`
}

func (r *RefreshCmd) Run(repo *RepoCmd) error {
	source, found := repository.GetSource(r.Source)
	if !found {
		return fmt.Errorf("source not found: %s", r.Source)
	}

	fmt.Printf("Refreshing %s... ", r.Source)

	client, err := repository.NewClient(repository.ClientOptions{
		Timeout: 60 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	localCfg := repository.NewLocalConfig(getSwordPath(repo.SwordPath))
	if err := localCfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	installer := repository.NewInstaller(localCfg, client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	modules, err := installer.RefreshSource(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to refresh source: %w", err)
	}

	fmt.Printf("done (%d modules)\n", len(modules))

	return nil
}

// ListCmd lists available modules from a source
type ListCmd struct {
	Source string `arg:"" required:"" help:"Source name to list modules from"`
}

func (l *ListCmd) Run(repo *RepoCmd) error {
	source, found := repository.GetSource(l.Source)
	if !found {
		return fmt.Errorf("source not found: %s", l.Source)
	}

	client, err := repository.NewClient(repository.ClientOptions{
		Timeout: 60 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	localCfg := repository.NewLocalConfig(getSwordPath(repo.SwordPath))
	installer := repository.NewInstaller(localCfg, client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	modules, err := installer.ListAvailable(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	if len(modules) == 0 {
		fmt.Printf("No modules available from %s (try: repo refresh %s)\n", l.Source, l.Source)
		return nil
	}

	// Find max lengths for alignment
	maxID, maxVer, maxLang := 0, 0, 0
	for _, m := range modules {
		if len(m.ID) > maxID {
			maxID = len(m.ID)
		}
		if len(m.Version) > maxVer {
			maxVer = len(m.Version)
		}
		if len(m.Language) > maxLang {
			maxLang = len(m.Language)
		}
	}
	if maxVer < 7 {
		maxVer = 7
	}
	if maxLang < 4 {
		maxLang = 4
	}

	fmt.Printf("Available from %s (%d modules):\n\n", l.Source, len(modules))
	fmt.Printf("%-*s  %-*s  %-*s  %s\n", maxID, "MODULE", maxVer, "VERSION", maxLang, "LANG", "DESCRIPTION")
	fmt.Printf("%s\n", strings.Repeat("-", maxID+maxVer+maxLang+50))

	for _, m := range modules {
		version := m.Version
		if version == "" {
			version = "-"
		}
		lang := m.Language
		if lang == "" {
			lang = "-"
		}
		desc := m.Description
		if desc == "" {
			desc = m.ID
		}
		fmt.Printf("%-*s  %-*s  %-*s  %s\n", maxID, m.ID, maxVer, version, maxLang, lang, desc)
	}

	return nil
}

// InstallCmd installs a module from a source
type InstallCmd struct {
	Source string `arg:"" required:"" help:"Source name"`
	Module string `arg:"" required:"" help:"Module name to install"`
}

func (i *InstallCmd) Run(repo *RepoCmd) error {
	source, found := repository.GetSource(i.Source)
	if !found {
		return fmt.Errorf("source not found: %s", i.Source)
	}

	client, err := repository.NewClient(repository.ClientOptions{
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	localCfg := repository.NewLocalConfig(getSwordPath(repo.SwordPath))
	if err := localCfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	installer := repository.NewInstaller(localCfg, client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// First get the list of available modules to find the one we want
	fmt.Printf("Installing %s from %s... ", i.Module, i.Source)

	modules, err := installer.ListAvailable(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	var targetModule *repository.ModuleInfo
	for idx := range modules {
		if strings.EqualFold(modules[idx].ID, i.Module) {
			targetModule = &modules[idx]
			break
		}
	}

	if targetModule == nil {
		fmt.Println("not found")
		return fmt.Errorf("module not found: %s", i.Module)
	}

	if err := installer.Install(ctx, source, *targetModule); err != nil {
		if errors.Is(err, repository.ErrPackageNotAvailable) {
			fmt.Println("unavailable (no package on server)")
			return nil // Not a fatal error - module just doesn't have a package
		}
		fmt.Println("failed")
		return fmt.Errorf("failed to install module: %w", err)
	}

	fmt.Println("done")

	return nil
}

// InstallAllCmd installs all modules from a source in parallel
type InstallAllCmd struct {
	Source  string `arg:"" required:"" help:"Source name"`
	Workers int    `name:"workers" short:"w" default:"4" help:"Number of parallel download workers"`
}

func (i *InstallAllCmd) Run(repo *RepoCmd) error {
	source, found := repository.GetSource(i.Source)
	if !found {
		return fmt.Errorf("source not found: %s", i.Source)
	}

	client, err := repository.NewClient(repository.ClientOptions{
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	localCfg := repository.NewLocalConfig(getSwordPath(repo.SwordPath))
	if err := localCfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	installer := repository.NewInstaller(localCfg, client)

	// Use a longer timeout for batch operations
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	// Get available modules
	fmt.Printf("Fetching module list from %s...\n", i.Source)
	modules, err := installer.ListAvailable(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	if len(modules) == 0 {
		fmt.Println("No modules available")
		return nil
	}

	fmt.Printf("Installing %d modules using %d parallel workers...\n\n", len(modules), i.Workers)

	// Track counts
	doneCount := 0
	skippedCount := 0
	unavailableCount := 0
	failedCount := 0
	completed := 0

	// Install with progress callback
	results := installer.InstallBatch(ctx, source, modules, repository.BatchInstallOptions{
		Workers:       i.Workers,
		SkipInstalled: true,
		OnResult: func(result repository.InstallResult) {
			completed++
			switch result.Status {
			case "done":
				doneCount++
				fmt.Printf("[%d/%d] %s... done\n", completed, len(modules), result.Module.ID)
			case "skipped":
				skippedCount++
				fmt.Printf("[%d/%d] %s... skipped (already installed)\n", completed, len(modules), result.Module.ID)
			case "unavailable":
				unavailableCount++
				fmt.Printf("[%d/%d] %s... unavailable\n", completed, len(modules), result.Module.ID)
			case "failed":
				failedCount++
				fmt.Printf("[%d/%d] %s... failed: %v\n", completed, len(modules), result.Module.ID, result.Error)
			}
		},
	})

	_ = results // Results already processed via callback

	fmt.Printf("\n%s: %d installed, %d skipped, %d unavailable, %d failed\n",
		i.Source, doneCount, skippedCount, unavailableCount, failedCount)

	return nil
}

// InstallMegaCmd installs all modules from ALL sources in parallel
type InstallMegaCmd struct {
	Workers int `name:"workers" short:"w" default:"4" help:"Number of parallel download workers"`
}

func (i *InstallMegaCmd) Run(repo *RepoCmd) error {
	client, err := repository.NewClient(repository.ClientOptions{
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	localCfg := repository.NewLocalConfig(getSwordPath(repo.SwordPath))
	if err := localCfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	installer := repository.NewInstaller(localCfg, client)

	// Use a longer timeout for mega operations
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	sources := repository.DefaultSources()

	fmt.Printf("=== MEGA DOWNLOAD: All modules from %d sources ===\n", len(sources))
	fmt.Printf("Using %d parallel workers\n\n", i.Workers)

	// Track totals
	totalDone := 0
	totalSkipped := 0
	totalUnavailable := 0
	totalFailed := 0

	// Track already installed/downloaded to avoid duplicates across sources
	installedSet := make(map[string]bool)
	if installed, err := localCfg.ListInstalledModules(); err == nil {
		for _, m := range installed {
			installedSet[strings.ToUpper(m.ID)] = true
		}
	}

	for _, source := range sources {
		fmt.Printf("=== %s ===\n", source.Name)

		// Get available modules
		modules, err := installer.ListAvailable(ctx, source)
		if err != nil {
			fmt.Printf("Failed to fetch module list: %v\n\n", err)
			continue
		}

		if len(modules) == 0 {
			fmt.Println("No modules available")
			continue
		}

		// Filter out already installed/downloaded
		var toInstall []repository.ModuleInfo
		preSkipped := 0
		for _, m := range modules {
			if installedSet[strings.ToUpper(m.ID)] {
				preSkipped++
			} else {
				toInstall = append(toInstall, m)
			}
		}

		if len(toInstall) == 0 {
			fmt.Printf("All %d modules already installed\n\n", len(modules))
			totalSkipped += preSkipped
			continue
		}

		fmt.Printf("Installing %d modules (%d already installed)...\n", len(toInstall), preSkipped)

		// Track counts for this source
		doneCount := 0
		skippedCount := preSkipped
		unavailableCount := 0
		failedCount := 0
		completed := 0

		// Install with progress callback
		installer.InstallBatch(ctx, source, toInstall, repository.BatchInstallOptions{
			Workers:       i.Workers,
			SkipInstalled: false, // We already filtered
			OnResult: func(result repository.InstallResult) {
				completed++
				switch result.Status {
				case "done":
					doneCount++
					installedSet[strings.ToUpper(result.Module.ID)] = true
					fmt.Printf("[%d/%d] %s... done\n", completed, len(toInstall), result.Module.ID)
				case "skipped":
					skippedCount++
					fmt.Printf("[%d/%d] %s... skipped\n", completed, len(toInstall), result.Module.ID)
				case "unavailable":
					unavailableCount++
					fmt.Printf("[%d/%d] %s... unavailable\n", completed, len(toInstall), result.Module.ID)
				case "failed":
					failedCount++
					fmt.Printf("[%d/%d] %s... failed\n", completed, len(toInstall), result.Module.ID)
				}
			},
		})

		fmt.Printf("%s: %d installed, %d skipped, %d unavailable, %d failed\n\n",
			source.Name, doneCount, skippedCount, unavailableCount, failedCount)

		totalDone += doneCount
		totalSkipped += skippedCount
		totalUnavailable += unavailableCount
		totalFailed += failedCount
	}

	fmt.Println("=== SUMMARY ===")
	fmt.Printf("Total: %d installed, %d skipped, %d unavailable, %d failed\n",
		totalDone, totalSkipped, totalUnavailable, totalFailed)

	return nil
}

// InstalledCmd lists installed modules
type InstalledCmd struct{}

func (i *InstalledCmd) Run(repo *RepoCmd) error {
	localCfg, err := repository.LoadLocalConfig(getSwordPath(repo.SwordPath))
	if err != nil {
		// If directory doesn't exist, just show empty list
		if os.IsNotExist(err) {
			fmt.Println("No modules installed")
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	modules, err := localCfg.ListInstalledModules()
	if err != nil {
		return fmt.Errorf("failed to list modules: %w", err)
	}

	if len(modules) == 0 {
		fmt.Println("No modules installed")
		return nil
	}

	fmt.Printf("Installed modules (%d):\n\n", len(modules))

	// Print header
	fmt.Printf("%-20s  %-10s  %-6s  %-25s  %s\n",
		"MODULE", "VERSION", "LANG", "LICENSE", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 100))

	for _, m := range modules {
		version := m.Version
		if version == "" {
			version = "-"
		}
		lang := m.Language
		if lang == "" {
			lang = "-"
		}
		license := m.LicenseSPDX()
		if license == "" {
			license = "-"
		}
		desc := m.Description
		if desc == "" {
			desc = m.ID
		}

		fmt.Printf("%-20s  %-10s  %-6s  %-25s  %s\n",
			m.ID, version, lang, license, desc)
	}

	return nil
}

// UninstallCmd uninstalls a module
type UninstallCmd struct {
	Module string `arg:"" required:"" help:"Module name to uninstall"`
}

func (u *UninstallCmd) Run(repo *RepoCmd) error {
	localCfg, err := repository.LoadLocalConfig(getSwordPath(repo.SwordPath))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, _ := repository.NewClient(repository.ClientOptions{})
	installer := repository.NewInstaller(localCfg, client)

	fmt.Printf("Uninstalling %s... ", u.Module)

	if err := installer.Uninstall(u.Module); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("failed to uninstall module: %w", err)
	}

	fmt.Println("done")

	return nil
}

// VerifyCmd verifies installed module integrity
type VerifyCmd struct {
	Module string `arg:"" optional:"" help:"Module name to verify (all if not specified)"`
}

func (v *VerifyCmd) Run(repo *RepoCmd) error {
	if CLI.Verbose {
		fmt.Println("[verbose] Loading local SWORD config...")
	}
	localCfg, err := repository.LoadLocalConfig(getSwordPath(repo.SwordPath))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No modules installed")
			return nil
		}
		return fmt.Errorf("failed to load config: %w", err)
	}
	if CLI.Verbose {
		fmt.Printf("[verbose] SWORD path: %s\n", getSwordPath(repo.SwordPath))
	}

	client, _ := repository.NewClient(repository.ClientOptions{})
	installer := repository.NewInstaller(localCfg, client)
	installer.Verbose = CLI.Verbose

	var results []repository.ModuleVerification

	if v.Module != "" {
		// Verify single module
		if CLI.Verbose {
			fmt.Printf("[verbose] Verifying single module: %s\n", v.Module)
		}
		results = append(results, installer.VerifyModule(v.Module))
	} else {
		// Verify all modules
		if CLI.Verbose {
			fmt.Println("[verbose] Fetching list of installed modules...")
		}
		installed, listErr := localCfg.ListInstalledModules()
		if listErr != nil {
			return fmt.Errorf("failed to list modules: %w", listErr)
		}

		total := len(installed)
		fmt.Printf("Verifying %d modules...\n", total)

		for i, mod := range installed {
			if CLI.Verbose {
				fmt.Printf("[verbose] Verifying module %d/%d: %s\n", i+1, total, mod.ID)
			} else {
				// Show progress indicator
				fmt.Printf("\r[%d/%d] Verifying %s...", i+1, total, mod.ID)
			}
			results = append(results, installer.VerifyModule(mod.ID))
		}
		if !CLI.Verbose {
			fmt.Println() // Clear progress line
		}
	}

	if len(results) == 0 {
		fmt.Println("No modules installed")
		return nil
	}

	// Find max lengths for alignment
	maxID := 0
	for _, r := range results {
		if len(r.ModuleID) > maxID {
			maxID = len(r.ModuleID)
		}
	}

	fmt.Printf("%-*s  %-6s  %-4s  %-5s  %-12s  %-12s  %s\n", maxID, "MODULE", "STATUS", "CONF", "DATA", "EXPECTED", "ACTUAL", "NOTES")
	fmt.Printf("%s\n", strings.Repeat("-", maxID+6+4+5+12+12+30))

	validCount := 0
	invalidCount := 0

	for _, r := range results {
		status := "OK"
		notes := ""

		if !r.Installed {
			status = "FAIL"
			notes = "not installed"
		} else if !r.DataExists {
			status = "FAIL"
			notes = "missing data"
		} else if r.ExpectedSize > 0 && !r.SizeMatch {
			status = "WARN"
			notes = "size mismatch"
		}

		if r.Error != "" {
			status = "ERR"
			notes = r.Error
		}

		if r.IsValid() {
			validCount++
		} else {
			invalidCount++
		}

		conf := "-"
		if r.Installed {
			conf = "✓"
		}
		data := "-"
		if r.DataExists {
			data = "✓"
		}

		expected := "-"
		if r.ExpectedSize > 0 {
			expected = formatBytes(r.ExpectedSize)
		}
		actual := "-"
		if r.ActualSize > 0 {
			actual = formatBytes(r.ActualSize)
		}

		fmt.Printf("%-*s  %-6s  %-4s  %-5s  %-12s  %-12s  %s\n",
			maxID, r.ModuleID, status, conf, data, expected, actual, notes)
	}

	fmt.Println()
	fmt.Printf("Summary: %d valid, %d issues\n", validCount, invalidCount)

	if invalidCount > 0 {
		return fmt.Errorf("%d modules have issues", invalidCount)
	}

	return nil
}

func getSwordPath(override string) string {
	if override != "" {
		return override
	}
	return repository.DefaultSwordDir()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
