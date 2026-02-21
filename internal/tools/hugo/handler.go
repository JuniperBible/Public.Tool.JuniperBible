// Package hugo provides the embedded handler for Hugo JSON output generator tool.
package hugo

import (
	"fmt"

	"github.com/JuniperBible/juniper/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for hugo.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.hugo",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-hugo",
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
	return nil, fmt.Errorf("hugo command '%s' requires external plugin", command)
}
