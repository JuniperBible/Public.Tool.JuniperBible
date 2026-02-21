// Package docgen provides documentation generation for Juniper Bible.
//
// It generates markdown documentation from:
// - Plugin manifests (plugin.json files)
// - CLI usage information
// - Format support matrix
package docgen

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
)

// PluginManifest represents a plugin.json file.
type PluginManifest struct {
	PluginID    string     `json:"plugin_id"`
	Version     string     `json:"version"`
	Kind        string     `json:"kind"`
	Entrypoint  string     `json:"entrypoint"`
	Description string     `json:"description"`
	Profiles    []Profile  `json:"profiles,omitempty"`
	Requires    []string   `json:"requires,omitempty"`
	Extensions  []string   `json:"extensions,omitempty"`
	LossClass   string     `json:"loss_class,omitempty"`
	IRSupport   *IRSupport `json:"ir_support,omitempty"`
}

// Profile represents a tool plugin profile.
type Profile struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// IRSupport describes IR capabilities.
type IRSupport struct {
	CanExtract bool   `json:"can_extract"`
	CanEmit    bool   `json:"can_emit"`
	LossClass  string `json:"loss_class,omitempty"`
}

// Generator generates documentation from source files.
type Generator struct {
	PluginDir string
	OutputDir string
}

// NewGenerator creates a new documentation generator.
func NewGenerator(pluginDir, outputDir string) *Generator {
	return &Generator{
		PluginDir: pluginDir,
		OutputDir: outputDir,
	}
}

// GenerateAll generates all documentation files.
func (g *Generator) GenerateAll() error {
	if err := os.MkdirAll(g.OutputDir, 0700); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if err := g.GeneratePlugins(); err != nil {
		return fmt.Errorf("failed to generate plugins doc: %w", err)
	}

	if err := g.GenerateFormats(); err != nil {
		return fmt.Errorf("failed to generate formats doc: %w", err)
	}

	if err := g.GenerateCLI(); err != nil {
		return fmt.Errorf("failed to generate CLI doc: %w", err)
	}

	return nil
}

// LoadPlugins loads all plugin manifests from the plugin directory.
func (g *Generator) LoadPlugins() ([]PluginManifest, error) {
	if err := validatePluginDir(g.PluginDir); err != nil {
		return nil, err
	}

	var plugins []PluginManifest
	plugins = append(plugins, loadPluginsFromSubdir(g.PluginDir, "format")...)
	plugins = append(plugins, loadPluginsWithPrefix(g.PluginDir, "format-")...)
	plugins = append(plugins, loadPluginsFromSubdir(g.PluginDir, "tool")...)
	plugins = append(plugins, loadPluginsWithPrefix(g.PluginDir, "tool-")...)

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].PluginID < plugins[j].PluginID
	})

	return plugins, nil
}

// validatePluginDir returns an error when the directory is set but inaccessible.
func validatePluginDir(dir string) error {
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin directory does not exist: %s", dir)
		}
		return fmt.Errorf("cannot access plugin directory: %w", err)
	}
	return nil
}

// loadPluginsFromSubdir reads every sub-directory of baseDir/subdir and loads
// a plugin.json from each one. Errors are silently skipped (directory absent
// or manifest missing/malformed).
func loadPluginsFromSubdir(baseDir, subdir string) []PluginManifest {
	dir := filepath.Join(baseDir, subdir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var plugins []PluginManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if manifest, err := loadManifest(filepath.Join(dir, entry.Name(), "plugin.json")); err == nil {
			plugins = append(plugins, manifest)
		}
	}
	return plugins
}

// loadPluginsWithPrefix reads every sub-directory of baseDir whose name starts
// with prefix and loads a plugin.json from each one.
func loadPluginsWithPrefix(baseDir, prefix string) []PluginManifest {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	var plugins []PluginManifest
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		if manifest, err := loadManifest(filepath.Join(baseDir, entry.Name(), "plugin.json")); err == nil {
			plugins = append(plugins, manifest)
		}
	}
	return plugins
}

func loadManifest(path string) (PluginManifest, error) {
	data, err := safefile.ReadFile(path)
	if err != nil {
		return PluginManifest{}, err
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PluginManifest{}, err
	}

	return manifest, nil
}

// GeneratePlugins generates PLUGINS.md.
func (g *Generator) GeneratePlugins() error {
	plugins, err := g.LoadPlugins()
	if err != nil {
		return err
	}

	path := filepath.Join(g.OutputDir, "PLUGINS.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return g.writePluginsDoc(f, plugins)
}

func (g *Generator) writePluginsDoc(w io.Writer, plugins []PluginManifest) error {
	fmt.Fprintln(w, "# Plugin Catalog")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "This document is auto-generated by `capsule docgen plugins`.")
	fmt.Fprintln(w)

	formatPlugins, toolPlugins := separatePluginsByKind(plugins)

	g.writeFormatPluginsTable(w, formatPlugins)
	g.writeFormatPluginDetails(w, formatPlugins)

	if len(toolPlugins) > 0 {
		g.writeToolPluginsTable(w, toolPlugins)
		g.writeToolPluginDetails(w, toolPlugins)
	}

	return nil
}

func separatePluginsByKind(plugins []PluginManifest) (format, tool []PluginManifest) {
	for _, p := range plugins {
		switch p.Kind {
		case "format":
			format = append(format, p)
		case "tool":
			tool = append(tool, p)
		}
	}
	return
}

func getLossClass(p PluginManifest) string {
	lossClass := p.LossClass
	if lossClass == "" && p.IRSupport != nil {
		lossClass = p.IRSupport.LossClass
	}
	return lossClass
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (g *Generator) writeFormatPluginsTable(w io.Writer, plugins []PluginManifest) {
	fmt.Fprintf(w, "## Format Plugins (%d)\n\n", len(plugins))
	fmt.Fprintln(w, "| Plugin ID | Version | Loss Class | Extensions | Description |")
	fmt.Fprintln(w, "|-----------|---------|------------|------------|-------------|")

	for _, p := range plugins {
		lossClass := valueOrDash(getLossClass(p))
		extensions := valueOrDash(strings.Join(p.Extensions, ", "))
		desc := valueOrDash(p.Description)
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n",
			p.PluginID, p.Version, lossClass, extensions, desc)
	}
	fmt.Fprintln(w)
}

func (g *Generator) writeFormatPluginDetails(w io.Writer, plugins []PluginManifest) {
	fmt.Fprintln(w, "### Format Plugin Details")
	fmt.Fprintln(w)

	for _, p := range plugins {
		fmt.Fprintf(w, "#### %s\n\n", p.PluginID)
		fmt.Fprintf(w, "- **Version**: %s\n", p.Version)

		if p.Description != "" {
			fmt.Fprintf(w, "- **Description**: %s\n", p.Description)
		}
		if len(p.Extensions) > 0 {
			fmt.Fprintf(w, "- **Extensions**: %s\n", strings.Join(p.Extensions, ", "))
		}

		lossClass := getLossClass(p)
		if lossClass != "" {
			fmt.Fprintf(w, "- **Loss Class**: %s\n", lossClass)
		}

		writeIRCapabilities(w, p.IRSupport)
		fmt.Fprintln(w)
	}
}

func writeIRCapabilities(w io.Writer, ir *IRSupport) {
	if ir == nil {
		return
	}

	var capabilities []string
	if ir.CanExtract {
		capabilities = append(capabilities, "extract-ir")
	}
	if ir.CanEmit {
		capabilities = append(capabilities, "emit-native")
	}

	if len(capabilities) > 0 {
		fmt.Fprintf(w, "- **IR Support**: %s\n", strings.Join(capabilities, ", "))
	}
}

func (g *Generator) writeToolPluginsTable(w io.Writer, plugins []PluginManifest) {
	fmt.Fprintf(w, "## Tool Plugins (%d)\n\n", len(plugins))
	fmt.Fprintln(w, "| Plugin ID | Version | Profiles | Requires |")
	fmt.Fprintln(w, "|-----------|---------|----------|----------|")

	for _, p := range plugins {
		profiles := extractProfileIDs(p.Profiles)
		requires := valueOrDash(strings.Join(p.Requires, ", "))
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n",
			p.PluginID, p.Version, profiles, requires)
	}
	fmt.Fprintln(w)
}

func extractProfileIDs(profiles []Profile) string {
	var ids []string
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	return strings.Join(ids, ", ")
}

func (g *Generator) writeToolPluginDetails(w io.Writer, plugins []PluginManifest) {
	fmt.Fprintln(w, "### Tool Plugin Details")
	fmt.Fprintln(w)

	for _, p := range plugins {
		fmt.Fprintf(w, "#### %s\n\n", p.PluginID)
		fmt.Fprintf(w, "- **Version**: %s\n", p.Version)

		if p.Description != "" {
			fmt.Fprintf(w, "- **Description**: %s\n", p.Description)
		}
		if len(p.Requires) > 0 {
			fmt.Fprintf(w, "- **Requires**: %s\n", strings.Join(p.Requires, ", "))
		}

		writeProfileDetails(w, p.Profiles)
		fmt.Fprintln(w)
	}
}

func writeProfileDetails(w io.Writer, profiles []Profile) {
	if len(profiles) == 0 {
		return
	}

	fmt.Fprintln(w, "- **Profiles**:")
	for _, profile := range profiles {
		fmt.Fprintf(w, "  - `%s`: %s\n", profile.ID, profile.Description)
	}
}

// GenerateFormats generates FORMATS.md.
func (g *Generator) GenerateFormats() error {
	plugins, err := g.LoadPlugins()
	if err != nil {
		return err
	}

	path := filepath.Join(g.OutputDir, "FORMATS.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return g.writeFormatsDoc(f, plugins)
}

func (g *Generator) writeFormatsDoc(w io.Writer, plugins []PluginManifest) error {
	fmt.Fprintln(w, "# Format Support Matrix")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "This document is auto-generated by `capsule docgen formats`.")
	fmt.Fprintln(w)

	lossDescs := formatLossDescriptions()
	lossOrder := []string{"L0", "L1", "L2", "L3", "L4", "Unknown"}

	formatPlugins := filterFormatPlugins(plugins)
	byLoss := groupByLossClass(formatPlugins)

	writeFormatsOverviewTable(w, byLoss, lossOrder, lossDescs, len(formatPlugins))

	for _, lc := range lossOrder {
		writeFormatLossSection(w, lc, lossDescs[lc], byLoss[lc])
	}

	writeFormatConversionNote(w)
	return nil
}

// formatLossDescriptions returns the canonical map of loss-class codes to
// human-readable descriptions used throughout the formats document.
func formatLossDescriptions() map[string]string {
	return map[string]string{
		"L0":      "Lossless - Byte-identical round-trip",
		"L1":      "Semantically Lossless - Content preserved, formatting may differ",
		"L2":      "Minor Loss - Some metadata or structure lost",
		"L3":      "Significant Loss - Text preserved, most markup lost",
		"L4":      "Text Only - Minimal preservation",
		"Unknown": "Loss class not specified",
	}
}

// filterFormatPlugins returns only plugins whose Kind is "format".
func filterFormatPlugins(plugins []PluginManifest) []PluginManifest {
	var out []PluginManifest
	for _, p := range plugins {
		if p.Kind == "format" {
			out = append(out, p)
		}
	}
	return out
}

// resolvePluginLossClass returns the effective loss class for p, falling back
// to IRSupport.LossClass and then to "Unknown".
func resolvePluginLossClass(p PluginManifest) string {
	if p.LossClass != "" {
		return p.LossClass
	}
	if p.IRSupport != nil && p.IRSupport.LossClass != "" {
		return p.IRSupport.LossClass
	}
	return "Unknown"
}

// groupByLossClass groups format plugins by their effective loss class.
func groupByLossClass(plugins []PluginManifest) map[string][]PluginManifest {
	byLoss := make(map[string][]PluginManifest)
	for _, p := range plugins {
		lc := resolvePluginLossClass(p)
		byLoss[lc] = append(byLoss[lc], p)
	}
	return byLoss
}

// writeFormatsOverviewTable writes the summary table listing each non-empty
// loss class alongside its description and plugin count.
func writeFormatsOverviewTable(w io.Writer, byLoss map[string][]PluginManifest, lossOrder []string, lossDescs map[string]string, total int) {
	fmt.Fprintln(w, "## Overview")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Total format plugins: **%d**\n\n", total)
	fmt.Fprintln(w, "| Loss Class | Description | Count |")
	fmt.Fprintln(w, "|------------|-------------|-------|")
	for _, lc := range lossOrder {
		if group := byLoss[lc]; len(group) > 0 {
			fmt.Fprintf(w, "| %s | %s | %d |\n", lc, lossDescs[lc], len(group))
		}
	}
	fmt.Fprintln(w)
}

// irColumns returns the IR Extract and IR Emit column values for a plugin row.
func irColumns(ir *IRSupport) (canExtract, canEmit string) {
	canExtract, canEmit = "-", "-"
	if ir == nil {
		return
	}
	if ir.CanExtract {
		canExtract = "✓"
	}
	if ir.CanEmit {
		canEmit = "✓"
	}
	return
}

// writeFormatLossSection writes a per-loss-class section with a plugin table.
// It is a no-op when plugins is empty.
func writeFormatLossSection(w io.Writer, lc, desc string, plugins []PluginManifest) {
	if len(plugins) == 0 {
		return
	}

	fmt.Fprintf(w, "## %s - %s (%d formats)\n\n", lc, desc, len(plugins))
	fmt.Fprintln(w, "| Format | Plugin | Extensions | IR Extract | IR Emit | Notes |")
	fmt.Fprintln(w, "|--------|--------|------------|------------|---------|-------|")

	for _, p := range plugins {
		format := strings.TrimPrefix(p.PluginID, "format-")
		extensions := valueOrDash(strings.Join(p.Extensions, ", "))
		canExtract, canEmit := irColumns(p.IRSupport)
		notes := valueOrDash(p.Description)
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s |\n",
			format, p.PluginID, extensions, canExtract, canEmit, notes)
	}
	fmt.Fprintln(w)
}

// writeFormatConversionNote writes the static conversion quality section.
func writeFormatConversionNote(w io.Writer) {
	fmt.Fprintln(w, "## Format Conversion")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "All bidirectional formats can convert to any other format through the IR:")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w, "Source → extract-ir → IR → emit-native → Target")
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "**Conversion Quality:**")
	fmt.Fprintln(w, "- Same Loss Class: Full fidelity (L0→L0, L1→L1)")
	fmt.Fprintln(w, "- Higher to Lower: Minimal loss (L0→L3)")
	fmt.Fprintln(w, "- Lower to Higher: Cannot recover lost data (L3→L0)")
}

// GenerateCLI generates CLI_REFERENCE.md.
func (g *Generator) GenerateCLI() error {
	path := filepath.Join(g.OutputDir, "CLI_REFERENCE.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return g.writeCLIDoc(f)
}

func (g *Generator) writeCLIDoc(w io.Writer) error {
	fmt.Fprintln(w, "# CLI Reference")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "This document is auto-generated by `capsule docgen cli`.")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Synopsis")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w, "capsule <command> [arguments]")
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w)

	// Core commands
	fmt.Fprintln(w, "## Core Commands")
	fmt.Fprintln(w)

	commands := []struct {
		Name    string
		Usage   string
		Desc    string
		Example string
	}{
		{"ingest", "capsule ingest <path> --out <capsule.tar.xz>", "Ingest a file into a new capsule", "capsule ingest myfile.zip --out myfile.capsule.tar.xz"},
		{"export", "capsule export <capsule> --artifact <id> --out <path>", "Export an artifact from a capsule", "capsule export my.capsule.tar.xz --artifact main --out restored.zip"},
		{"verify", "capsule verify <capsule>", "Verify capsule integrity (all hashes match)", "capsule verify my.capsule.tar.xz"},
		{"selfcheck", "capsule selfcheck <capsule> [--plan <plan-id>] [--json]", "Run self-check verification plan", "capsule selfcheck my.capsule.tar.xz --plan identity-bytes"},
	}

	for _, cmd := range commands {
		fmt.Fprintf(w, "### %s\n\n", cmd.Name)
		fmt.Fprintf(w, "%s\n\n", cmd.Desc)
		fmt.Fprintln(w, "**Usage:**")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, cmd.Usage)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "**Example:**")
		fmt.Fprintln(w, "```bash")
		fmt.Fprintln(w, cmd.Example)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	// Plugin commands
	fmt.Fprintln(w, "## Plugin Commands")
	fmt.Fprintln(w)

	pluginCmds := []struct {
		Name    string
		Usage   string
		Desc    string
		Example string
	}{
		{"plugins", "capsule plugins [--dir <path>]", "List available plugins", "capsule plugins"},
		{"detect", "capsule detect <path> [--plugin-dir <path>]", "Detect file format using plugins", "capsule detect myfile.xml"},
		{"enumerate", "capsule enumerate <path> [--plugin-dir <path>]", "Enumerate contents of archive", "capsule enumerate myfile.zip"},
	}

	for _, cmd := range pluginCmds {
		fmt.Fprintf(w, "### %s\n\n", cmd.Name)
		fmt.Fprintf(w, "%s\n\n", cmd.Desc)
		fmt.Fprintln(w, "**Usage:**")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, cmd.Usage)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "**Example:**")
		fmt.Fprintln(w, "```bash")
		fmt.Fprintln(w, cmd.Example)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	// IR commands
	fmt.Fprintln(w, "## IR Commands")
	fmt.Fprintln(w)

	irCmds := []struct {
		Name    string
		Usage   string
		Desc    string
		Example string
	}{
		{"extract-ir", "capsule extract-ir <path> --format <format> --out <ir.json>", "Extract Intermediate Representation from a file", "capsule extract-ir bible.usfm --format usfm --out bible.ir.json"},
		{"emit-native", "capsule emit-native <ir.json> --format <format> --out <path>", "Emit native format from IR", "capsule emit-native bible.ir.json --format osis --out bible.osis"},
		{"convert", "capsule convert <path> --to <format> --out <path>", "Convert file to different format via IR", "capsule convert bible.usfm --to osis --out bible.osis"},
		{"ir-info", "capsule ir-info <ir.json> [--json]", "Display IR structure summary", "capsule ir-info bible.ir.json"},
	}

	for _, cmd := range irCmds {
		fmt.Fprintf(w, "### %s\n\n", cmd.Name)
		fmt.Fprintf(w, "%s\n\n", cmd.Desc)
		fmt.Fprintln(w, "**Usage:**")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, cmd.Usage)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "**Example:**")
		fmt.Fprintln(w, "```bash")
		fmt.Fprintln(w, cmd.Example)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	// Tool commands
	fmt.Fprintln(w, "## Tool Commands")
	fmt.Fprintln(w)

	toolCmds := []struct {
		Name    string
		Usage   string
		Desc    string
		Example string
	}{
		{"run", "capsule run <tool> <profile> [--input <path>] [--out <dir>]", "Run a tool plugin with Nix executor", "capsule run libsword list-modules --input ./kjv"},
		{"tool-run", "capsule tool-run <capsule> <artifact> <tool> <profile>", "Run tool on artifact and store transcript", "capsule tool-run kjv.capsule.tar.xz main libsword render-all"},
		{"tool-list", "capsule tool-list [<contrib-dir>]", "List available tools in contrib/tool", "capsule tool-list"},
		{"tool-archive", "capsule tool-archive <tool-id> --version <ver> --bin <name>=<path> --out <capsule>", "Create tool archive capsule from binaries", "capsule tool-archive diatheke --version 1.8.1 --bin diatheke=/usr/bin/diatheke --out diatheke.capsule.tar.xz"},
	}

	for _, cmd := range toolCmds {
		fmt.Fprintf(w, "### %s\n\n", cmd.Name)
		fmt.Fprintf(w, "%s\n\n", cmd.Desc)
		fmt.Fprintln(w, "**Usage:**")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, cmd.Usage)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "**Example:**")
		fmt.Fprintln(w, "```bash")
		fmt.Fprintln(w, cmd.Example)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	// Behavioral testing commands
	fmt.Fprintln(w, "## Behavioral Testing Commands")
	fmt.Fprintln(w)

	behaviorCmds := []struct {
		Name    string
		Usage   string
		Desc    string
		Example string
	}{
		{"runs", "capsule runs <capsule>", "List all runs in a capsule", "capsule runs kjv.capsule.tar.xz"},
		{"compare", "capsule compare <capsule> <run1> <run2>", "Compare transcripts between two runs", "capsule compare kjv.capsule.tar.xz run-1 run-2"},
		{"golden", "capsule golden <capsule> <run> [--save <file>] [--check <file>]", "Save or check golden transcript hash", "capsule golden kjv.capsule.tar.xz run-1 --save golden.sha256"},
		{"test", "capsule test <fixtures-dir> [--golden <dir>]", "Run tests against golden hashes", "capsule test testdata/fixtures"},
	}

	for _, cmd := range behaviorCmds {
		fmt.Fprintf(w, "### %s\n\n", cmd.Name)
		fmt.Fprintf(w, "%s\n\n", cmd.Desc)
		fmt.Fprintln(w, "**Usage:**")
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w, cmd.Usage)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "**Example:**")
		fmt.Fprintln(w, "```bash")
		fmt.Fprintln(w, cmd.Example)
		fmt.Fprintln(w, "```")
		fmt.Fprintln(w)
	}

	// Documentation commands
	fmt.Fprintln(w, "## Documentation Commands")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "### docgen")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Generate documentation from plugins and CLI.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "**Usage:**")
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w, "capsule docgen <subcommand> [--output <dir>]")
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "**Subcommands:**")
	fmt.Fprintln(w, "- `plugins` - Generate PLUGINS.md")
	fmt.Fprintln(w, "- `formats` - Generate FORMATS.md")
	fmt.Fprintln(w, "- `cli` - Generate CLI_REFERENCE.md")
	fmt.Fprintln(w, "- `all` - Generate all documentation")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "**Example:**")
	fmt.Fprintln(w, "```bash")
	fmt.Fprintln(w, "capsule docgen all --output docs/")
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w)

	// Other commands
	fmt.Fprintln(w, "## Other Commands")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Print version information.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "```bash")
	fmt.Fprintln(w, "capsule version")
	fmt.Fprintln(w, "```")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Show help message.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "```bash")
	fmt.Fprintln(w, "capsule help")
	fmt.Fprintln(w, "```")

	return nil
}
