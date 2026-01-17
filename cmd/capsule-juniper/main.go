// Package main provides a standalone CLI for Bible/SWORD module tools.
// This binary wraps the internal/juniper package for standalone usage.
//
// Usage:
//
//	capsule-juniper list [--path ~/.sword]
//	capsule-juniper ingest --all [--path ~/.sword] [--output ./capsules]
//	capsule-juniper cas-to-sword capsule.tar.gz [--output ~/.sword] [--name MODULE]
//	capsule-juniper repoman <command> [options]
//
// Prefer using `capsule juniper` instead of this standalone binary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/FocuswithJustin/JuniperBible/internal/juniper"
	"github.com/FocuswithJustin/JuniperBible/internal/juniper/repoman"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/FocuswithJustin/JuniperBible/internal/embedded"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		runList(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
	case "cas-to-sword":
		runCASToSword(os.Args[2:])
	case "hugo":
		runHugo(os.Args[2:])
	case "repoman":
		runRepoman(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	path := fs.String("path", "", "Path to SWORD installation (default: ~/.sword)")
	fs.Parse(args)

	if err := juniper.List(juniper.ListConfig{Path: *path}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	path := fs.String("path", "", "Path to SWORD installation (default: ~/.sword)")
	output := fs.String("output", "capsules", "Output directory for capsules")
	all := fs.Bool("all", false, "Ingest all Bible modules")
	fs.Parse(args)

	cfg := juniper.IngestConfig{
		Path:    *path,
		Output:  *output,
		All:     *all,
		Modules: fs.Args(),
	}
	if err := juniper.Ingest(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	path := fs.String("path", "", "Path to SWORD installation (default: ~/.sword)")
	output := fs.String("output", "capsules", "Output directory for capsules")
	all := fs.Bool("all", false, "Install all Bible modules")
	fs.Parse(args)

	cfg := juniper.InstallConfig{
		Path:    *path,
		Output:  *output,
		All:     *all,
		Modules: fs.Args(),
	}
	if err := juniper.Install(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCASToSword(args []string) {
	fs := flag.NewFlagSet("cas-to-sword", flag.ExitOnError)
	output := fs.String("output", "", "Output directory for SWORD module (default: ~/.sword)")
	name := fs.String("name", "", "Module name (default: derived from capsule)")
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: capsule path required\n")
		fs.Usage()
		os.Exit(1)
	}

	cfg := juniper.CASToSwordConfig{
		Capsule: fs.Args()[0],
		Output:  *output,
		Name:    *name,
	}
	if err := juniper.CASToSword(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runHugo(args []string) {
	fs := flag.NewFlagSet("hugo", flag.ExitOnError)
	path := fs.String("path", "", "Path to SWORD installation (default: ~/.sword)")
	output := fs.String("output", "data", "Output directory for Hugo data files")
	all := fs.Bool("all", false, "Export all Bible modules")
	workers := fs.Int("workers", 0, "Number of parallel workers (default: number of CPUs)")
	fs.Parse(args)

	cfg := juniper.HugoConfig{
		Path:    *path,
		Output:  *output,
		All:     *all,
		Modules: fs.Args(),
		Workers: *workers,
	}
	if err := juniper.Hugo(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runRepoman(args []string) {
	if len(args) < 1 {
		printRepomanUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "list-sources":
		runRepomanListSources()
	case "refresh":
		runRepomanRefresh(args[1:])
	case "list":
		runRepomanList(args[1:])
	case "install":
		runRepomanInstall(args[1:])
	case "installed":
		runRepomanInstalled(args[1:])
	case "uninstall":
		runRepomanUninstall(args[1:])
	case "verify":
		runRepomanVerify(args[1:])
	case "help", "-h", "--help":
		printRepomanUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown repoman command: %s\n", args[0])
		printRepomanUsage()
		os.Exit(1)
	}
}

func runRepomanListSources() {
	sources := repoman.ListSources()
	fmt.Println("Available SWORD repository sources:")
	fmt.Printf("%-15s %s\n", "NAME", "URL")
	fmt.Printf("%-15s %s\n", "----", "---")
	for _, s := range sources {
		fmt.Printf("%-15s %s%s\n", s.Name, s.URL, s.Directory)
	}
	fmt.Printf("\nTotal: %d sources\n", len(sources))
}

func runRepomanRefresh(args []string) {
	fs := flag.NewFlagSet("refresh", flag.ExitOnError)
	source := fs.String("source", "CrossWire", "Source name to refresh")
	fs.Parse(args)

	fmt.Printf("Refreshing %s...\n", *source)
	if err := repoman.RefreshSource(*source); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done!")
}

func runRepomanList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	source := fs.String("source", "CrossWire", "Source name to list modules from")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	modules, err := repoman.ListAvailable(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(modules)
		return
	}

	fmt.Printf("Available modules from %s:\n\n", *source)
	fmt.Printf("%-15s %-8s %-8s %-40s\n", "MODULE", "TYPE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-8s %-40s\n", "------", "----", "----", "-----------")
	for _, m := range modules {
		desc := m.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Printf("%-15s %-8s %-8s %-40s\n", m.Name, m.Type, m.Language, desc)
	}
	fmt.Printf("\nTotal: %d modules\n", len(modules))
}

func runRepomanInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	source := fs.String("source", "CrossWire", "Source name to install from")
	dest := fs.String("dest", "", "Destination path (default: ~/.sword)")
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: module name required\n")
		os.Exit(1)
	}

	module := fs.Args()[0]
	destPath := *dest
	if destPath == "" {
		home, _ := os.UserHomeDir()
		destPath = home + "/.sword"
	}

	fmt.Printf("Installing %s from %s to %s...\n", module, *source, destPath)
	if err := repoman.Install(*source, module, destPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done!")
}

func runRepomanInstalled(args []string) {
	fs := flag.NewFlagSet("installed", flag.ExitOnError)
	path := fs.String("path", "", "SWORD installation path (default: ~/.sword)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	swordPath := *path
	if swordPath == "" {
		home, _ := os.UserHomeDir()
		swordPath = home + "/.sword"
	}

	modules, err := repoman.ListInstalled(swordPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(modules)
		return
	}

	fmt.Printf("Installed modules in %s:\n\n", swordPath)
	fmt.Printf("%-15s %-8s %-8s %-40s\n", "MODULE", "TYPE", "LANG", "DESCRIPTION")
	fmt.Printf("%-15s %-8s %-8s %-40s\n", "------", "----", "----", "-----------")
	for _, m := range modules {
		desc := m.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Printf("%-15s %-8s %-8s %-40s\n", m.Name, m.Type, m.Language, desc)
	}
	fmt.Printf("\nTotal: %d modules\n", len(modules))
}

func runRepomanUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	path := fs.String("path", "", "SWORD installation path (default: ~/.sword)")
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: module name required\n")
		os.Exit(1)
	}

	module := fs.Args()[0]
	swordPath := *path
	if swordPath == "" {
		home, _ := os.UserHomeDir()
		swordPath = home + "/.sword"
	}

	fmt.Printf("Uninstalling %s from %s...\n", module, swordPath)
	if err := repoman.Uninstall(module, swordPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done!")
}

func runRepomanVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	path := fs.String("path", "", "SWORD installation path (default: ~/.sword)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "Error: module name required\n")
		os.Exit(1)
	}

	module := fs.Args()[0]
	swordPath := *path
	if swordPath == "" {
		home, _ := os.UserHomeDir()
		swordPath = home + "/.sword"
	}

	result, err := repoman.Verify(module, swordPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		return
	}

	fmt.Printf("Verification result for %s:\n", module)
	if result.Valid {
		fmt.Println("  Status: VALID")
	} else {
		fmt.Println("  Status: INVALID")
	}
	if len(result.Errors) > 0 {
		fmt.Println("  Errors:")
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Println("  Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("    - %s\n", w)
		}
	}
}

func printUsage() {
	fmt.Println(`capsule-juniper - Bible/SWORD module tools

Usage:
  capsule-juniper <command> [options]

Commands:
  list          List Bible modules in SWORD installation
  ingest        Ingest SWORD modules into capsules (raw, no IR)
  install       Install SWORD modules as capsules with IR (recommended)
  cas-to-sword  Convert CAS capsule to SWORD module
  hugo          Export SWORD modules to Hugo JSON data files
  repoman       SWORD repository management

Options for 'list':
  --path        Path to SWORD installation (default: ~/.sword)

Options for 'ingest':
  --path        Path to SWORD installation (default: ~/.sword)
  --output      Output directory for capsules (default: capsules)
  --all         Ingest all Bible modules

Options for 'install':
  --path        Path to SWORD installation (default: ~/.sword)
  --output      Output directory for capsules (default: capsules)
  --all         Install all Bible modules

Options for 'cas-to-sword':
  --output      Output directory for SWORD module (default: ~/.sword)
  --name        Module name (default: derived from capsule filename)

Options for 'hugo':
  --path        Path to SWORD installation (default: ~/.sword)
  --output      Output directory for Hugo data files (default: data)
  --all         Export all Bible modules
  --workers     Number of parallel workers (default: number of CPUs)
  [modules...]  Specific module names to export

Examples:
  capsule-juniper install KJV ASV       Install specific modules with IR
  capsule-juniper install --all         Install all Bible modules with IR
  capsule-juniper hugo KJV ASV DRC      Export specific modules
  capsule-juniper hugo --all            Export all Bible modules

Run 'capsule-juniper repoman help' for repoman subcommands.

Prefer using 'capsule juniper' instead of this standalone binary.`)
}

func printRepomanUsage() {
	fmt.Println(`capsule-juniper repoman - SWORD repository management

Usage:
  capsule-juniper repoman <command> [options]

Commands:
  list-sources  List configured repository sources
  refresh       Refresh module index from a source
  list          List available modules from a source
  install       Install a module from a source
  installed     List installed modules
  uninstall     Uninstall a module
  verify        Verify module integrity

Options for 'refresh':
  --source      Source name (default: CrossWire)

Options for 'list':
  --source      Source name (default: CrossWire)
  --json        Output as JSON

Options for 'install':
  --source      Source name (default: CrossWire)
  --dest        Destination path (default: ~/.sword)

Options for 'installed':
  --path        SWORD installation path (default: ~/.sword)
  --json        Output as JSON

Options for 'uninstall':
  --path        SWORD installation path (default: ~/.sword)

Options for 'verify':
  --path        SWORD installation path (default: ~/.sword)
  --json        Output as JSON`)
}
