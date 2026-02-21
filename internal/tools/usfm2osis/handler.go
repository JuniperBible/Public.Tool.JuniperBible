// Package usfm2osis provides the embedded handler for USFM to OSIS converter tool.
package usfm2osis

import (
	"fmt"

	"github.com/JuniperBible/juniper/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for usfm2osis.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.usfm2osis",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-usfm2osis",
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
	return nil, fmt.Errorf("usfm2osis command '%s' requires external plugin", command)
}
