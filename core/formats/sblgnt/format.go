// Package sblgnt provides canonical SBL Greek New Testament format support.
//
// IR Support:
// - extract-ir: Reads SBLGNT to IR (L1)
// - emit-native: Converts IR to SBLGNT format (L1)
//
// Note: Full implementation in plugins/format-sblgnt/main_sdk.go
package sblgnt

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the SBLGNT format plugin.
var Config = &format.Config{
	Name:       "SBLGNT",
	Extensions: []string{".txt", ".tsv"},
	Detect:     detectSBLGNT,
	Parse:      parseSBLGNT,
	Emit:       emitSBLGNT,
}

func detectSBLGNT(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "SBLGNT detection not yet fully implemented in canonical package",
	}, nil
}

func parseSBLGNT(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("SBLGNT parsing not yet fully implemented in canonical package")
}

func emitSBLGNT(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("SBLGNT emission not yet fully implemented in canonical package")
}
