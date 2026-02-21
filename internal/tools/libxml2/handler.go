// Package libxml2 provides the embedded handler for libxml2 tool.
package libxml2

import (
	"fmt"

	"github.com/JuniperBible/juniper/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for libxml2.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.libxml2",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-libxml2",
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
	return nil, fmt.Errorf("libxml2 command '%s' requires external plugin", command)
}
