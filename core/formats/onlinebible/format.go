// Package onlinebible provides canonical Online Bible format support.
//
// IR Support:
// - extract-ir: Reads Online Bible to IR (L2)
// - emit-native: Converts IR to Online Bible format (L2)
//
// Note: Full implementation in plugins/format-onlinebible/main_sdk.go
package onlinebible

import (
	"fmt"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the Online Bible format plugin.
var Config = &format.Config{
	PluginID:   "format.onlinebible",
	Name:       "OnlineBible",
	Extensions: []string{".ont"},
	Detect:     detectOnlineBible,
	Parse:      parseOnlineBible,
	Emit:       emitOnlineBible,
}

func detectOnlineBible(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "OnlineBible detection not yet fully implemented in canonical package",
	}, nil
}

func parseOnlineBible(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("OnlineBible parsing not yet fully implemented in canonical package")
}

func emitOnlineBible(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("OnlineBible emission not yet fully implemented in canonical package")
}
