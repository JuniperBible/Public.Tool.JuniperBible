// Package main provides a standalone SWORD repository management CLI.
// This binary wraps the internal/juniper/repoman package for standalone usage.
//
// Usage:
//
//	capsule-repoman list-sources
//	capsule-repoman refresh --source CrossWire
//	capsule-repoman list --source CrossWire
//	capsule-repoman install --source CrossWire --module KJV --dest ~/.sword
//	capsule-repoman installed --sword-path ~/.sword
//	capsule-repoman uninstall --module KJV --sword-path ~/.sword
//	capsule-repoman verify --module KJV --sword-path ~/.sword
//
// Prefer using `capsule repoman` instead of this standalone binary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/FocuswithJustin/JuniperBible/internal/juniper/repoman"

	// Import embedded plugins registry to register all embedded plugins
	_ "github.com/FocuswithJustin/JuniperBible/internal/embedded"
)

var commands = map[string]func([]string){
	"list-sources": func(_ []string) { handleListSources() },
	"refresh":      handleRefresh,
	"list":         handleList,
	"install":      handleInstall,
	"installed":    handleInstalled,
	"uninstall":    handleUninstall,
	"verify":       handleVerify,
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	if handler, ok := commands[os.Args[1]]; ok {
		handler(os.Args[2:])
		return
	}
	fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
	printUsage()
	os.Exit(1)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `capsule-repoman - SWORD repository management tool

Usage:
  capsule-repoman <command> [flags]

Commands:
  list-sources                    List available repository sources
  refresh                         Refresh module index from a source
  list                            List available modules from a source
  install                         Install a module from a source
  installed                       List installed modules
  uninstall                       Uninstall a module
  verify                          Verify module integrity

Flags (vary by command):
  --source <name>                 Source name (CrossWire, eBible)
  --module <id>                   Module ID to install/uninstall/verify
  --dest <path>                   Installation destination path
  --sword-path <path>             SWORD modules directory (default: current directory)
  --json                          Output in JSON format

Examples:
  capsule-repoman list-sources
  capsule-repoman refresh --source CrossWire
  capsule-repoman list --source CrossWire
  capsule-repoman install --source CrossWire --module KJV --dest ~/.sword
  capsule-repoman installed --sword-path ~/.sword
  capsule-repoman uninstall --module KJV --sword-path ~/.sword
  capsule-repoman verify --module KJV --sword-path ~/.sword

Prefer using 'capsule repoman' instead of this standalone binary.
`)
}

func handleListSources() {
	sources := repoman.ListSources()
	for _, s := range sources {
		fmt.Printf("%s: %s%s\n", s.Name, s.URL, s.Directory)
	}
}

func handleRefresh(args []string) {
	fs := flag.NewFlagSet("refresh", flag.ExitOnError)
	source := fs.String("source", "", "Source name (required)")
	fs.Parse(args)

	if *source == "" {
		fmt.Fprintf(os.Stderr, "Error: --source is required\n")
		os.Exit(1)
	}

	if err := repoman.RefreshSource(*source); err != nil {
		log.Fatalf("Failed to refresh source: %v", err)
	}

	fmt.Printf("Successfully refreshed source: %s\n", *source)
}

func handleList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	source := fs.String("source", "", "Source name (required)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	if *source == "" {
		fmt.Fprintf(os.Stderr, "Error: --source is required\n")
		os.Exit(1)
	}

	modules, err := repoman.ListAvailable(*source)
	if err != nil {
		log.Fatalf("Failed to list modules: %v", err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(modules); err != nil {
			log.Fatalf("Failed to encode JSON: %v", err)
		}
	} else {
		fmt.Printf("Available modules from %s:\n\n", *source)
		for _, m := range modules {
			fmt.Printf("%-20s %-50s [%s] %s\n", m.Name, m.Description, m.Type, m.Language)
		}
		fmt.Printf("\nTotal: %d modules\n", len(modules))
	}
}

func handleInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	source := fs.String("source", "", "Source name (required)")
	module := fs.String("module", "", "Module ID (required)")
	dest := fs.String("dest", ".", "Installation destination path")
	fs.Parse(args)

	if *source == "" || *module == "" {
		fmt.Fprintf(os.Stderr, "Error: --source and --module are required\n")
		os.Exit(1)
	}

	fmt.Printf("Installing %s from %s to %s...\n", *module, *source, *dest)

	if err := repoman.Install(*source, *module, *dest); err != nil {
		log.Fatalf("Failed to install module: %v", err)
	}

	fmt.Printf("Successfully installed %s\n", *module)
}

func handleInstalled(args []string) {
	fs := flag.NewFlagSet("installed", flag.ExitOnError)
	swordPath := fs.String("sword-path", ".", "SWORD modules directory")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	modules, err := repoman.ListInstalled(*swordPath)
	if err != nil {
		log.Fatalf("Failed to list installed modules: %v", err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(modules); err != nil {
			log.Fatalf("Failed to encode JSON: %v", err)
		}
	} else {
		if len(modules) == 0 {
			fmt.Println("No modules installed")
			return
		}

		fmt.Printf("Installed modules in %s:\n\n", *swordPath)
		for _, m := range modules {
			fmt.Printf("%-20s %-50s [%s] %s\n", m.Name, m.Description, m.Type, m.Language)
		}
		fmt.Printf("\nTotal: %d modules\n", len(modules))
	}
}

func handleUninstall(args []string) {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	module := fs.String("module", "", "Module ID (required)")
	swordPath := fs.String("sword-path", ".", "SWORD modules directory")
	fs.Parse(args)

	if *module == "" {
		fmt.Fprintf(os.Stderr, "Error: --module is required\n")
		os.Exit(1)
	}

	if err := repoman.Uninstall(*module, *swordPath); err != nil {
		log.Fatalf("Failed to uninstall module: %v", err)
	}

	fmt.Printf("Successfully uninstalled %s\n", *module)
}

func handleVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	module := fs.String("module", "", "Module ID (required)")
	swordPath := fs.String("sword-path", ".", "SWORD modules directory")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Parse(args)

	if *module == "" {
		fmt.Fprintf(os.Stderr, "Error: --module is required\n")
		os.Exit(1)
	}

	result, err := repoman.Verify(*module, *swordPath)
	if err != nil {
		log.Fatalf("Failed to verify module: %v", err)
	}

	if *jsonOutput {
		outputVerifyResultJSON(result)
	} else {
		outputVerifyResultText(result)
	}
}

func outputVerifyResultJSON(result *repoman.VerifyResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		log.Fatalf("Failed to encode JSON: %v", err)
	}
}

func outputVerifyResultText(result *repoman.VerifyResult) {
	fmt.Printf("Module: %s\n", result.Module)
	if result.Valid {
		fmt.Println("Status: VALID")
	} else {
		fmt.Println("Status: INVALID")
	}
	printIssueList("Errors", result.Errors)
	printIssueList("Warnings", result.Warnings)
}

func printIssueList(label string, issues []string) {
	if len(issues) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", label)
	for _, issue := range issues {
		fmt.Printf("  - %s\n", issue)
	}
}
