//go:build !sdk

// Plugin example-noop is a placeholder plugin for the dist/plugins/ directory.
// It serves as a template for external/premium plugins and indicates where
// additional plugins can be installed.
//
// This plugin does nothing - it's a noop (no operation) placeholder.
package main

import (
	"encoding/json"
	"os"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
)

func main() {
	var req ipc.Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		respond(ipc.Response{Status: "error", Error: "failed to decode request"})
		return
	}

	switch req.Command {
	case "detect":
		respond(ipc.Response{
			Status: "ok",
			Result: map[string]interface{}{
				"detected": false,
				"reason":   "noop plugin - placeholder for external/premium plugins",
			},
		})
	case "info":
		respond(ipc.Response{
			Status: "ok",
			Result: map[string]interface{}{
				"description": "Placeholder plugin for the plugins directory. Add external or premium plugins here.",
				"note":        "Core functionality is embedded in the main binaries.",
			},
		})
	default:
		respond(ipc.Response{
			Status: "error",
			Error:  "noop plugin: command not supported - this is a placeholder",
		})
	}
}

func respond(resp ipc.Response) {
	json.NewEncoder(os.Stdout).Encode(resp)
}
