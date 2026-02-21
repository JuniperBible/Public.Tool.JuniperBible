// Package embedded provides the embedded plugin registry.
// Importing this package registers all embedded plugins with the core plugin system.
package embedded

import (
	// Import all embedded format plugins to trigger their init() registration
	_ "github.com/JuniperBible/juniper/core/formats/accordance"
	_ "github.com/JuniperBible/juniper/core/formats/bibletime"
	_ "github.com/JuniperBible/juniper/core/formats/crosswire"
	_ "github.com/JuniperBible/juniper/core/formats/dbl"
	_ "github.com/JuniperBible/juniper/core/formats/dir"
	_ "github.com/JuniperBible/juniper/core/formats/ecm"
	_ "github.com/JuniperBible/juniper/core/formats/epub"
	_ "github.com/JuniperBible/juniper/core/formats/esword"
	_ "github.com/JuniperBible/juniper/core/formats/file"
	_ "github.com/JuniperBible/juniper/core/formats/flex"
	_ "github.com/JuniperBible/juniper/core/formats/gobible"
	_ "github.com/JuniperBible/juniper/core/formats/html"
	_ "github.com/JuniperBible/juniper/core/formats/json"
	_ "github.com/JuniperBible/juniper/core/formats/logos"
	_ "github.com/JuniperBible/juniper/core/formats/markdown"
	_ "github.com/JuniperBible/juniper/core/formats/morphgnt"
	_ "github.com/JuniperBible/juniper/core/formats/mybible"
	_ "github.com/JuniperBible/juniper/core/formats/mysword"
	_ "github.com/JuniperBible/juniper/core/formats/na28app"
	_ "github.com/JuniperBible/juniper/core/formats/odf"
	_ "github.com/JuniperBible/juniper/core/formats/olive"
	_ "github.com/JuniperBible/juniper/core/formats/onlinebible"
	_ "github.com/JuniperBible/juniper/core/formats/oshb"
	_ "github.com/JuniperBible/juniper/core/formats/osis"
	_ "github.com/JuniperBible/juniper/core/formats/pdb"
	_ "github.com/JuniperBible/juniper/core/formats/rtf"
	_ "github.com/JuniperBible/juniper/core/formats/sblgnt"
	_ "github.com/JuniperBible/juniper/core/formats/sfm"
	_ "github.com/JuniperBible/juniper/core/formats/sqlite"
	_ "github.com/JuniperBible/juniper/core/formats/sword"
	swordpure "github.com/JuniperBible/juniper/core/formats/sword-pure"
	_ "github.com/JuniperBible/juniper/core/formats/tar"
	_ "github.com/JuniperBible/juniper/core/formats/tei"
	_ "github.com/JuniperBible/juniper/core/formats/theword"
	_ "github.com/JuniperBible/juniper/core/formats/tischendorf"
	_ "github.com/JuniperBible/juniper/core/formats/txt"
	_ "github.com/JuniperBible/juniper/core/formats/usfm"
	_ "github.com/JuniperBible/juniper/core/formats/usx"
	_ "github.com/JuniperBible/juniper/core/formats/xml"
	_ "github.com/JuniperBible/juniper/core/formats/zefania"
	_ "github.com/JuniperBible/juniper/core/formats/zip"

	// Import all embedded tool plugins to trigger their init() registration
	_ "github.com/JuniperBible/juniper/internal/tools/calibre"
	_ "github.com/JuniperBible/juniper/internal/tools/gobiblecreator"
	_ "github.com/JuniperBible/juniper/internal/tools/hugo"
	_ "github.com/JuniperBible/juniper/internal/tools/libsword"
	_ "github.com/JuniperBible/juniper/internal/tools/libxml2"
	_ "github.com/JuniperBible/juniper/internal/tools/pandoc"
	_ "github.com/JuniperBible/juniper/internal/tools/repoman"
	_ "github.com/JuniperBible/juniper/internal/tools/sqlite"
	_ "github.com/JuniperBible/juniper/internal/tools/unrtf"
	_ "github.com/JuniperBible/juniper/internal/tools/usfm2osis"

	"github.com/JuniperBible/juniper/core/plugins"
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
