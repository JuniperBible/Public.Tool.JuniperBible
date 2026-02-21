// Package tischendorf provides canonical Tischendorf Greek NT format support.
//
// IR Support:
// - extract-ir: Reads Tischendorf to IR (L1)
// - emit-native: Converts IR to Tischendorf format (L1)
//
// Note: Full implementation in plugins/format/tischendorf/main_sdk.go
package tischendorf

import (
	"fmt"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// Config defines the Tischendorf format plugin.
var Config = &format.Config{
	PluginID:   "format.tischendorf",
	Name:       "Tischendorf",
	Extensions: []string{".txt"},
	Detect:     detectTischendorf,
	Parse:      parseTischendorf,
	Emit:       emitTischendorf,
}

func detectTischendorf(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "Tischendorf detection not yet fully implemented in canonical package",
	}, nil
}

func parseTischendorf(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("Tischendorf parsing not yet fully implemented in canonical package")
}

func emitTischendorf(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("Tischendorf emission not yet fully implemented in canonical package")
}
