//  Package crosswire provides canonical CrossWire SWORD distribution format support.
// This handles SWORD module distributions in .zip format as distributed by CrossWire.
//
// IR Support:
// - extract-ir: Reads CrossWire format to IR (L1)
// - emit-native: Converts IR to CrossWire format (L1)
//
// Note: Full implementation in plugins/format/crosswire/main_sdk.go
package crosswire

import (
	"fmt"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the CrossWire format plugin.
var Config = &format.Config{
	PluginID:   "format.crosswire",
	Name:       "crosswire",
	Extensions: []string{".zip"},
	Detect:     detectCrossWire,
	Parse:      parseCrossWire,
	Emit:       emitCrossWire,
}

func detectCrossWire(path string) (*ipc.DetectResult, error) {
	// Stub: Full implementation in plugins/format/crosswire/main_sdk.go
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "CrossWire detection not yet fully implemented in canonical package",
	}, nil
}

func parseCrossWire(path string) (*ir.Corpus, error) {
	// Stub: Full implementation in plugins/format/crosswire/main_sdk.go
	return nil, fmt.Errorf("CrossWire parsing not yet fully implemented in canonical package")
}

func emitCrossWire(corpus *ir.Corpus, outputDir string) (string, error) {
	// Stub: Full implementation in plugins/format/crosswire/main_sdk.go
	return "", fmt.Errorf("CrossWire emission not yet fully implemented in canonical package")
}
