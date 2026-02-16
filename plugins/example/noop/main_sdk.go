//go:build sdk

// Plugin example-noop is a placeholder plugin for the dist/plugins/ directory.
// It serves as a template for external/premium plugins and indicates where
// additional plugins can be installed.
//
// This plugin does nothing - it's a noop (no operation) placeholder.
// SDK version - uses the format SDK pattern for consistency.
package main

import (
	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

func main() {
	format.Run(&format.Config{
		Name:       "example-noop",
		Extensions: []string{},
		Detect:     detect,
		Parse:      parse,
		Emit:       emit,
		Enumerate:  enumerate,
	})
}

// detect always returns false - this is a placeholder plugin.
func detect(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Format:   "noop",
		Reason:   "noop plugin - placeholder for external/premium plugins",
	}, nil
}

// parse returns an empty corpus - this is a placeholder plugin.
func parse(path string) (*ir.Corpus, error) {
	return ir.NewCorpus("noop", "placeholder", "0.0.0"), nil
}

// emit does nothing - this is a placeholder plugin.
func emit(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", nil
}

// enumerate returns empty - this is a placeholder plugin.
func enumerate(path string) (*ipc.EnumerateResult, error) {
	return &ipc.EnumerateResult{
		Entries: []ipc.EnumerateEntry{},
	}, nil
}
