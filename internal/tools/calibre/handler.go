// Package calibre provides the embedded handler for Calibre ebook tool.
package calibre

import (
	"fmt"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for calibre.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.calibre",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-calibre",
		Capabilities: plugins.Capabilities{
			Inputs:  []string{},
			Outputs: []string{},
		},
	}
}

// Register registers this plugin with the embedded registry.
func Register() {
	plugins.RegisterEmbeddedPlugin(&plugins.EmbeddedPlugin{
		Manifest: Manifest(),
		Tool:     &Handler{},
	})
}

func init() {
	Register()
}

// Execute implements EmbeddedToolHandler.Execute.
func (h *Handler) Execute(command string, args map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("calibre command '%s' requires external plugin", command)
}
