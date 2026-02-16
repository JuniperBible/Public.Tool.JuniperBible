// Package embedded provides the embedded plugin registry.
// Importing this package registers all embedded plugins with the core plugin system.
package embedded

import (
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
