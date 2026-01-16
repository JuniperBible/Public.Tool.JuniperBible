// Package embedded provides the embedded plugin registry.
// Importing this package registers all embedded plugins with the core plugin system.
package embedded

import (
	// Import all embedded format plugins to trigger their init() registration
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/accordance"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/bibletime"
	// Import all embedded tool plugins to trigger their init() registration
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/calibre"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/gobiblecreator"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/hugo"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/libsword"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/libxml2"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/pandoc"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/repoman"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/sqlite"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/unrtf"
	_ "github.com/FocuswithJustin/JuniperBible/internal/tools/usfm2osis"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/dbl"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/dir"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/ecm"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/epub"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/esword"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/file"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/flex"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/gobible"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/html"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/json"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/logos"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/markdown"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/morphgnt"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/mybible"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/mysword"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/na28app"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/odf"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/olive"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/onlinebible"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/oshb"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/osis"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/pdb"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/rtf"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/sblgnt"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/sfm"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/sword"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/swordpure"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/tar"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/tei"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/theword"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/txt"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/usfm"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/usx"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/xml"
	_ "github.com/FocuswithJustin/JuniperBible/internal/formats/zefania"

	"github.com/FocuswithJustin/JuniperBible/core/plugins"
)

// initialized indicates whether the embedded registry has been initialized.
// This is set to true when the package is imported.
var initialized bool

func init() {
	initialized = true
}

// IsInitialized returns true if the embedded registry has been initialized.
// This function exists primarily to provide a testable entry point for coverage.
func IsInitialized() bool {
	return initialized
}

// PluginCount returns the total number of registered embedded plugins.
func PluginCount() int {
	return len(plugins.ListEmbeddedPlugins())
}

