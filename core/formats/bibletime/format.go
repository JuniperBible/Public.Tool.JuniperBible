// Package bibletime provides canonical BibleTime Bible study format support.
// BibleTime uses SWORD modules with additional bookmarks/notes metadata.
//
// IR Support:
// - extract-ir: Reads BibleTime format (SWORD + bookmarks) to IR (L1)
// - emit-native: Converts IR to BibleTime format (L1)
//
// Note: Full implementation in plugins/format/bibletime/main_sdk.go
package bibletime

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the BibleTime format plugin.
var Config = &format.Config{
	PluginID:  "format.bibletime",
	Name:      "BIBLETIME",
	Detect:    detectBibleTime,
	Parse:     parseBibleTime,
	Emit:      emitBibleTime,
	Enumerate: enumerateBibleTime,
}

func detectBibleTime(path string) (*ipc.DetectResult, error) {
	// Stub: Full implementation in plugins/format/bibletime/main_sdk.go
	return &ipc.DetectResult{
		Detected: false,
		Reason:   "BibleTime detection not yet fully implemented in canonical package",
	}, nil
}

func parseBibleTime(path string) (*ir.Corpus, error) {
	// Stub: Full implementation in plugins/format/bibletime/main_sdk.go
	return nil, fmt.Errorf("BibleTime parsing not yet fully implemented in canonical package")
}

func emitBibleTime(corpus *ir.Corpus, outputDir string) (string, error) {
	// Stub: Full implementation in plugins/format/bibletime/main_sdk.go
	return "", fmt.Errorf("BibleTime emission not yet fully implemented in canonical package")
}

func enumerateBibleTime(path string) (*ipc.EnumerateResult, error) {
	// Stub: Full implementation in plugins/format/bibletime/main_sdk.go
	return &ipc.EnumerateResult{Entries: []ipc.EnumerateEntry{}}, nil
}
