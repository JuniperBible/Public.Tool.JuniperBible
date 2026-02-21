// Package libsword provides the embedded handler for libsword tool.
package libsword

import (
	"fmt"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for libsword.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.libsword",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-libsword",
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
	return nil, fmt.Errorf("libsword command '%s' requires external plugin", command)
}
