// Package ecm provides canonical Editio Critica Maior format support.
//
// IR Support:
// - extract-ir: Reads ECM to IR (L1)
// - emit-native: Converts IR to ECM format (L1)
//
// Note: Full implementation in plugins/format/ecm/main_sdk.go
package ecm

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the ECM format plugin.
var Config = &format.Config{
	PluginID:   "format.ecm",
	Name:       "ECM",
	Extensions: []string{".ecm", ".txt"},
	Detect:     detectECM,
	Parse:      parseECM,
	Emit:       emitECM,
}

func detectECM(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "ECM detection not yet fully implemented in canonical package",
	}, nil
}

func parseECM(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("ECM parsing not yet fully implemented in canonical package")
}

func emitECM(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("ECM emission not yet fully implemented in canonical package")
}
