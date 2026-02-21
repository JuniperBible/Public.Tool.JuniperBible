// Package na28app provides canonical NA28 App format support.
//
// IR Support:
// - extract-ir: Reads NA28 App to IR (L2)
// - emit-native: Converts IR to NA28 App format (L2)
//
// Note: Full implementation in plugins/format/na28app/main_sdk.go
package na28app

import (
	"fmt"

	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/ipc"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/ir"
)

// Config defines the NA28 App format plugin.
var Config = &format.Config{
	PluginID:   "format.na28app",
	Name:       "NA28App",
	Extensions: []string{".na28"},
	Detect:     detectNA28App,
	Parse:      parseNA28App,
	Emit:       emitNA28App,
}

func detectNA28App(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "NA28App detection not yet fully implemented in canonical package",
	}, nil
}

func parseNA28App(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("NA28App parsing not yet fully implemented in canonical package")
}

func emitNA28App(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("NA28App emission not yet fully implemented in canonical package")
}
