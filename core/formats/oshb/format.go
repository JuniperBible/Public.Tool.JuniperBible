// Package oshb provides canonical Open Scriptures Hebrew Bible format support.
//
// IR Support:
// - extract-ir: Reads OSHB to IR (L1)
// - emit-native: Converts IR to OSHB format (L1)
//
// Note: Full implementation in plugins/format-oshb/main_sdk.go
package oshb

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the OSHB format plugin.
var Config = &format.Config{
	Name:       "OSHB",
	Extensions: []string{".txt", ".tsv"},
	Detect:     detectOSHB,
	Parse:      parseOSHB,
	Emit:       emitOSHB,
}

func detectOSHB(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "OSHB detection not yet fully implemented in canonical package",
	}, nil
}

func parseOSHB(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("OSHB parsing not yet fully implemented in canonical package")
}

func emitOSHB(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("OSHB emission not yet fully implemented in canonical package")
}
