// Package pdb provides canonical Palm Database format support.
//
// IR Support:
// - extract-ir: Reads PDB format to IR (L2)
// - emit-native: Converts IR to PDB format (L2)
//
// Note: Full implementation in plugins/format-pdb/main_sdk.go
package pdb

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the PDB format plugin.
var Config = &format.Config{
	Name:       "PDB",
	Extensions: []string{".pdb"},
	Detect:     detectPDB,
	Parse:      parsePDB,
	Emit:       emitPDB,
}

func detectPDB(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "PDB detection not yet fully implemented in canonical package",
	}, nil
}

func parsePDB(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("PDB parsing not yet fully implemented in canonical package")
}

func emitPDB(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("PDB emission not yet fully implemented in canonical package")
}
