// Package morphgnt provides canonical MorphGNT Greek NT format support.
// MorphGNT is a morphologically parsed Greek New Testament in TSV format.
//
// IR Support:
// - extract-ir: Reads MorphGNT to IR (L1)
// - emit-native: Converts IR to MorphGNT format (L1)
//
// Note: Full implementation in plugins/format-morphgnt/main_sdk.go
package morphgnt

import (
	"fmt"

	"github.com/JuniperBible/juniper/plugins/ipc"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
	"github.com/JuniperBible/juniper/plugins/sdk/ir"
)

// Config defines the MorphGNT format plugin.
var Config = &format.Config{
	PluginID:   "format.morphgnt",
	Name:       "MorphGNT",
	Extensions: []string{".txt", ".tsv"},
	Detect:     detectMorphGNT,
	Parse:      parseMorphGNT,
	Emit:       emitMorphGNT,
}

func detectMorphGNT(path string) (*ipc.DetectResult, error) {
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "MorphGNT detection not yet fully implemented in canonical package",
	}, nil
}

func parseMorphGNT(path string) (*ir.Corpus, error) {
	return nil, fmt.Errorf("MorphGNT parsing not yet fully implemented in canonical package")
}

func emitMorphGNT(corpus *ir.Corpus, outputDir string) (string, error) {
	return "", fmt.Errorf("MorphGNT emission not yet fully implemented in canonical package")
}
