// Package tool provides helpers for building tool plugins.
package tool

import (
	"os/exec"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/errors"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/runtime"
)

// Profile describes a tool execution profile.
type Profile struct {
	ID          string
	Description string
}

// Config defines a tool plugin's capabilities and handlers.
type Config struct {
	// Name is the tool plugin name (e.g., "tool-pandoc")
	Name string

	// Version is the semantic version (e.g., "1.0.0")
	Version string

	// Description explains what the tool does
	Description string

	// Profiles lists available execution profiles
	Profiles []Profile

	// Requires lists external tool dependencies (e.g., ["pandoc"])
	Requires []string

	// Check verifies external tools are available.
	// If nil, assumes tools are available.
	Check func() (bool, error)

	// Run executes a tool profile with the given request.
	Run func(req *ipc.ToolRunRequest) error
}

// Run starts a tool plugin with the given configuration.
func Run(cfg *Config) error {
	return runtime.RunDispatcher(func(d *runtime.Dispatcher) {
		d.Register("info", makeInfoHandler(cfg))
		d.Register("check", makeCheckHandler(cfg))
	})
}

// makeInfoHandler creates an info command handler.
func makeInfoHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		profiles := make([]ipc.ProfileInfo, len(cfg.Profiles))
		for i, p := range cfg.Profiles {
			profiles[i] = ipc.ProfileInfo{
				ID:          p.ID,
				Description: p.Description,
			}
		}

		return &ipc.ToolInfo{
			Name:        cfg.Name,
			Version:     cfg.Version,
			Type:        "tool",
			Description: cfg.Description,
			Profiles:    profiles,
			Requires:    cfg.Requires,
		}, nil
	}
}

// makeCheckHandler creates a check command handler.
func makeCheckHandler(cfg *Config) func(map[string]interface{}) (interface{}, error) {
	return func(args map[string]interface{}) (interface{}, error) {
		if cfg.Check == nil {
			return map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"tools_available": true,
				},
			}, nil
		}

		available, err := cfg.Check()
		if err != nil {
			return nil, errors.Wrap(errors.CodeInternal, "tool check failed", err)
		}

		return map[string]interface{}{
			"success": available,
			"data": map[string]interface{}{
				"tools_available": available,
			},
		}, nil
	}
}

// ExecCheck is a helper that checks if an executable exists in PATH.
func ExecCheck(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ExecCheckAll checks if all executables exist in PATH.
func ExecCheckAll(names ...string) bool {
	for _, name := range names {
		if !ExecCheck(name) {
			return false
		}
	}
	return true
}
