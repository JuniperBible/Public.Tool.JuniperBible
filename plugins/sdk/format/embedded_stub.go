//go:build standalone

package format

// registerEmbedded is a no-op when building standalone plugins.
// Standalone plugins don't register with the embedded plugin system.
func registerEmbedded(cfg *Config) {
	// No-op: standalone plugins don't register as embedded
}
