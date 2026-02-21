// Package gobiblecreator provides the embedded handler for GoBible Creator tool.
package gobiblecreator

import (
	"fmt"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for gobible-creator.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.gobible-creator",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-gobible-creator",
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
	return nil, fmt.Errorf("gobible-creator command '%s' requires external plugin", command)
}
