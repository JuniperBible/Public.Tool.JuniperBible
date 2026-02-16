//go:build !standalone

package rtf

func init() {
	// TODO: Implement Config.RegisterEmbedded() in plugins/sdk/format
	// This will register the format with the embedded plugin registry
	_ = Config
}
