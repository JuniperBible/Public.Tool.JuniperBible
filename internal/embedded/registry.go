// Package embedded provides the embedded plugin registry.
// Importing this package registers all embedded plugins with the core plugin system.
package embedded

import (
	// Import all embedded format plugins to trigger their init() registration
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/accordance"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/bibletime"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/crosswire"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/dbl"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/dir"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/ecm"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/epub"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/esword"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/file"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/flex"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/gobible"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/html"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/json"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/logos"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/markdown"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/morphgnt"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/mybible"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/mysword"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/na28app"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/odf"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/olive"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/onlinebible"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/oshb"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/osis"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/pdb"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/rtf"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/sblgnt"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/sfm"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/sqlite"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/sword"
	swordpure "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/sword-pure"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/tar"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/tei"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/theword"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/tischendorf"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/txt"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/usfm"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/usx"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/xml"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/zefania"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/zip"

	// Import all embedded tool plugins to trigger their init() registration
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/calibre"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/gobiblecreator"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/hugo"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/libsword"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/libxml2"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/pandoc"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/repoman"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/sqlite"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/unrtf"
	_ "github.com/JuniperBible/Public.Tool.JuniperBible/internal/tools/usfm2osis"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// Ensure swordpure is used (package has hyphen in path)
var _ = swordpure.Config

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
