// Command docgen generates documentation for Juniper Bible.
//
// Usage:
//
//	docgen plugins --output docs/    Generate PLUGINS.md
//	docgen formats --output docs/    Generate FORMATS.md
//	docgen cli --output docs/        Generate CLI_REFERENCE.md
//	docgen all --output docs/        Generate all documentation
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuniperBible/juniper/core/docgen"
)

// generator interface for testing.
type generator interface {
	GeneratePlugins() error
	GenerateFormats() error
	GenerateCLI() error
	GenerateAll() error
}

// newGenerator creates a new generator (allows injection in tests).
var newGenerator = func(pluginDir, outputDir string) generator {
	return docgen.NewGenerator(pluginDir, outputDir)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// config holds the parsed command-line configuration.
type config struct {
	cmd       string
	outputDir string
	pluginDir string
}

// parseArgs parses command-line arguments and returns the configuration.
func parseArgs(args []string) (config, error) {
	if len(args) < 1 {
		return config{}, fmt.Errorf("no command specified")
	}

	cfg := config{
		cmd:       args[0],
		outputDir: "docs",
		pluginDir: "plugins",
	}

	// Parse flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--output", "-o":
			if i+1 < len(args) {
				cfg.outputDir = args[i+1]
				i++
			}
		case "--plugins", "-p":
			if i+1 < len(args) {
				cfg.pluginDir = args[i+1]
				i++
			}
		}
	}

	// Resolve to absolute paths
	cfg.outputDir, _ = filepath.Abs(cfg.outputDir)
	cfg.pluginDir, _ = filepath.Abs(cfg.pluginDir)

	return cfg, nil
}

// isHelpCommand returns true if the command is a help command.
func isHelpCommand(cmd string) bool {
	return cmd == "help" || cmd == "-h" || cmd == "--help"
}

// executeCommand executes the specified command and returns an error if any.
func executeCommand(cfg config, gen generator, stdout io.Writer) error {
	switch cfg.cmd {
	case "plugins":
		fmt.Fprintf(stdout, "Generating PLUGINS.md...\n")
		return gen.GeneratePlugins()
	case "formats":
		fmt.Fprintf(stdout, "Generating FORMATS.md...\n")
		return gen.GenerateFormats()
	case "cli":
		fmt.Fprintf(stdout, "Generating CLI_REFERENCE.md...\n")
		return gen.GenerateCLI()
	case "all":
		fmt.Fprintf(stdout, "Generating all documentation...\n")
		fmt.Fprintf(stdout, "  Plugin dir: %s\n", cfg.pluginDir)
		fmt.Fprintf(stdout, "  Output dir: %s\n", cfg.outputDir)
		fmt.Fprintln(stdout)
		return gen.GenerateAll()
	default:
		return fmt.Errorf("unknown command: %s", cfg.cmd)
	}
}

// run executes the docgen logic and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		printUsageTo(stderr)
		return 1
	}

	if isHelpCommand(cfg.cmd) {
		printUsageTo(stdout)
		return 0
	}

	gen := newGenerator(cfg.pluginDir, cfg.outputDir)

	if err := executeCommand(cfg, gen, stdout); err != nil {
		// Handle unknown command with special formatting
		if strings.HasPrefix(err.Error(), "unknown command:") {
			fmt.Fprintf(stderr, "Unknown command: %s\n", strings.TrimPrefix(err.Error(), "unknown command: "))
			printUsageTo(stderr)
		} else {
			fmt.Fprintf(stderr, "Error: %v\n", err)
		}
		return 1
	}

	fmt.Fprintln(stdout, "Documentation generated successfully!")
	return 0
}

func printUsage() {
	printUsageTo(os.Stdout)
}

func printUsageTo(w io.Writer) {
	fmt.Fprint(w, `docgen - Juniper Bible Documentation Generator

Usage: docgen <command> [options]

Commands:
  plugins     Generate PLUGINS.md (plugin catalog)
  formats     Generate FORMATS.md (format support matrix)
  cli         Generate CLI_REFERENCE.md (CLI reference)
  all         Generate all documentation files

Options:
  --output, -o <dir>     Output directory (default: docs)
  --plugins, -p <dir>    Plugins directory (default: plugins)

Examples:
  docgen all --output docs/
  docgen plugins --output docs/ --plugins plugins/
  docgen formats -o docs/
`)
}
