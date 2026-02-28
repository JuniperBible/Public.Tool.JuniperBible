// Package main provides the CLI entry point for Juniper, a Bible module toolkit.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/juniper/pkg/config"
	"github.com/FocuswithJustin/juniper/pkg/migrate"
	"github.com/FocuswithJustin/juniper/pkg/output"
	"github.com/FocuswithJustin/juniper/pkg/sword"
	"github.com/alecthomas/kong"
)

var cfg *config.Config

// CLI defines the command-line interface using Kong
var CLI struct {
	Config  string `name:"config" short:"c" help:"Config file (default: config.yaml)" type:"path"`
	Verbose bool   `name:"verbose" short:"v" help:"Verbose output"`

	// Subcommands
	Diatheke DiathekeCmd `cmd:"" help:"Diatheke-compatible verse lookup (legacy mode)"`
	Convert  ConvertCmd  `cmd:"" help:"Convert SWORD modules to Hugo JSON format"`
	Migrate  MigrateCmd  `cmd:"" help:"Copy SWORD modules from system directory"`
	Backup   BackupCmd   `cmd:"" help:"Backup SWORD modules and converted data"`
	Test     TestCmd     `cmd:"" help:"Test parser accuracy against CGo reference"`
	Watch    WatchCmd    `cmd:"" help:"Watch for changes and auto-convert"`
	Version  VersionCmd  `cmd:"" help:"Print version information"`
	Repo     RepoCmd     `cmd:"" help:"Manage SWORD module repositories"`
}

// DiathekeCmd provides diatheke-compatible verse lookup
type DiathekeCmd struct {
	Module    string   `name:"module" short:"b" required:"" help:"Module name (e.g., KJV)"`
	Format    string   `name:"format" short:"f" help:"Output format (plain, HTML, RTF, etc.)"`
	Locale    string   `name:"locale" short:"l" help:"Locale for output"`
	Option    string   `name:"option" short:"o" help:"Module option"`
	Variant   string   `name:"variant" help:"Text variant"`
	Reference []string `arg:"" required:"" help:"Bible reference to look up"`
}

func (d *DiathekeCmd) Run() error {
	reference := strings.Join(d.Reference, " ")

	// Build diatheke command
	diathekeArgs := []string{"-b", d.Module}

	if d.Format != "" {
		diathekeArgs = append(diathekeArgs, "-f", d.Format)
	}
	if d.Locale != "" {
		diathekeArgs = append(diathekeArgs, "-l", d.Locale)
	}
	if d.Option != "" {
		diathekeArgs = append(diathekeArgs, "-o", d.Option)
	}
	if d.Variant != "" {
		diathekeArgs = append(diathekeArgs, "-v", d.Variant)
	}

	diathekeArgs = append(diathekeArgs, "-k", reference)

	// Execute diatheke
	diathekePath, err := exec.LookPath("diatheke")
	if err != nil {
		return fmt.Errorf("diatheke not found in PATH: install SWORD tools")
	}

	diathekeCmd := exec.Command(diathekePath, diathekeArgs...) // #nosec G204 -- diathekePath is from trusted config
	diathekeCmd.Stdout = os.Stdout
	diathekeCmd.Stderr = os.Stderr

	return diathekeCmd.Run()
}

// ConvertCmd converts SWORD modules to Hugo JSON format
type ConvertCmd struct {
	Input       string   `name:"input" short:"i" help:"Input directory (default: sword_data/incoming)"`
	Output      string   `name:"output" short:"o" help:"Output directory (default: data/)"`
	Granularity string   `name:"granularity" short:"g" default:"chapter" help:"Page granularity: book, chapter, verse"`
	Modules     []string `name:"modules" short:"m" help:"Specific modules to convert"`
}

func (c *ConvertCmd) Run() error {
	input := c.Input
	if input == "" {
		input = filepath.Join(cfg.OutputDir, "incoming")
	}

	outputDir := c.Output
	if outputDir == "" {
		outputDir = cfg.OutputDir
	}

	granularity := c.Granularity
	if granularity == "" {
		granularity = string(cfg.Granularity)
	}

	fmt.Printf("Converting SWORD modules from %s to %s (granularity: %s)\n", input, outputDir, granularity)

	// Load modules from input directory
	modules, err := sword.LoadAllModules(input)
	if err != nil {
		return fmt.Errorf("failed to load modules: %w", err)
	}

	// Filter by specified modules if provided
	if len(c.Modules) > 0 {
		filterSet := make(map[string]bool)
		for _, m := range c.Modules {
			filterSet[strings.ToLower(m)] = true
		}

		var filtered []*sword.Module
		for _, m := range modules {
			if filterSet[strings.ToLower(m.ID)] {
				filtered = append(filtered, m)
			}
		}
		modules = filtered
	}

	if len(modules) == 0 {
		fmt.Println("No modules found to convert.")
		return nil
	}

	fmt.Printf("Found %d modules to convert\n", len(modules))

	// Generate JSON output
	generator := output.NewGenerator(outputDir, granularity)

	// Load SPDX licenses for validation
	spdxPath := filepath.Join(filepath.Dir(outputDir), "spdx_licenses.json")
	if err := generator.LoadSPDXLicenses(spdxPath); err != nil {
		spdxPath = filepath.Join(outputDir, "spdx_licenses.json")
		if err2 := generator.LoadSPDXLicenses(spdxPath); err2 != nil {
			fmt.Printf("Warning: could not load SPDX licenses for validation: %v\n", err)
		}
	}

	if err := generator.GenerateFromModules(modules, input); err != nil {
		return fmt.Errorf("failed to generate output: %w", err)
	}

	fmt.Printf("\nConversion complete:\n")
	fmt.Printf("  Output: %s/bibles.json\n", outputDir)
	fmt.Printf("  Output: %s/bibles_auxiliary/\n", outputDir)

	return nil
}

// MigrateCmd copies SWORD modules from system directory
type MigrateCmd struct {
	Source  string   `name:"source" help:"Source SWORD directory (default: ~/.sword)"`
	Dest    string   `name:"dest" help:"Destination directory (default: sword_data/incoming)"`
	Modules []string `name:"modules" help:"Specific modules to migrate"`
}

func (m *MigrateCmd) Run() error {
	source := m.Source
	if source == "" {
		source = cfg.SwordDir
	}

	dest := m.Dest
	if dest == "" {
		dest = cfg.OutputDir
	}

	fmt.Printf("Migrating SWORD modules from %s to %s\n", source, dest)

	migrator := migrate.NewMigrator(source, dest)
	migrator.Verbose = CLI.Verbose

	result, err := migrator.Migrate(m.Modules)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Printf("\nMigration complete:\n")
	fmt.Printf("  Modules found:   %d\n", result.ModulesFound)
	fmt.Printf("  Modules copied:  %d\n", result.ModulesCopied)
	fmt.Printf("  Modules skipped: %d\n", result.ModulesSkipped)

	if len(result.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	return nil
}

// TestCmd tests parser accuracy against CGo reference
type TestCmd struct {
	Verses     string `name:"verses" help:"Verses to test (comma-separated)"`
	CompareCGo bool   `name:"compare-cgo" help:"Compare against CGo libsword"`
}

func (t *TestCmd) Run() error {
	if t.CompareCGo {
		fmt.Println("CGo comparison not yet implemented.")
		return nil
	}

	fmt.Println("Parser testing not yet implemented.")
	return nil
}

// WatchCmd watches for changes and auto-converts
type WatchCmd struct{}

func (w *WatchCmd) Run() error {
	fmt.Println("Watch mode not yet implemented.")
	return nil
}

// VersionCmd prints version information
type VersionCmd struct{}

func (v *VersionCmd) Run() error {
	fmt.Println("Juniper v1.0.0")
	fmt.Println("Bible module toolkit for SWORD/e-Sword formats")
	fmt.Println("https://github.com/FocuswithJustin/juniper")
	return nil
}

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("juniper"),
		kong.Description("Bible module toolkit with diatheke-compatible verse lookup"),
		kong.UsageOnError(),
	)

	// Load config
	var err error
	if CLI.Config != "" {
		cfg, err = config.Load(CLI.Config)
	} else {
		cfg, err = config.Load("config.yaml")
	}
	if err != nil {
		// Config is optional for diatheke mode
		cfg = config.DefaultConfig()
	}

	err = ctx.Run()
	ctx.FatalIfErrorf(err)
}
