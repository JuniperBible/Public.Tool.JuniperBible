// Package repoman provides the embedded handler for SWORD repository management tool.
package repoman

import (
	"fmt"

	"github.com/JuniperBible/juniper/core/plugins"
)

// Handler implements the EmbeddedToolHandler interface for repoman.
type Handler struct{}

// Manifest returns the plugin manifest for registration.
func Manifest() *plugins.PluginManifest {
	return &plugins.PluginManifest{
		PluginID:   "tool.repoman",
		Version:    "1.0.0",
		Kind:       "tool",
		Entrypoint: "tool-repoman",
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
	switch command {
	case "list-sources":
		return map[string]interface{}{
			"sources": []map[string]string{
				{"name": "CrossWire", "url": "https://www.crosswire.org/ftpmirror/pub/sword/packages/rawzip/"},
				{"name": "eBible", "url": "https://ebible.org/sword/"},
			},
		}, nil
	default:
		return nil, fmt.Errorf("repoman command '%s' requires external plugin", command)
	}
}
