// Package flex provides canonical FLEx (FieldWorks Language Explorer) format support.
//
// IR Support:
// - extract-ir: Reads FLEx to IR (L2)
// - emit-native: Converts IR to FLEx format (L2)
//
// Note: Full implementation in plugins/format-flex/main_sdk.go
package flex

import (
	"fmt"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the FLEx format plugin.
var Config = &format.Config{
	PluginID:   "format.flex",
	Name:       "FLEx",
	Extensions: []string{".fwdata", ".flextext"},
	Detect:     detectFLEx,
	Parse:      parseFLEx,
	Emit:       emitFLEx,
}

func detectFLEx(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "FLEx detection not yet fully implemented in canonical package",
	}, nil
}

func parseFLEx(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("FLEx parsing not yet fully implemented in canonical package")
}

func emitFLEx(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("FLEx emission not yet fully implemented in canonical package")
}
