//go:build !standalone

package xml

func init() {
	// TODO: Implement Config.RegisterEmbedded() in plugins/sdk/format
	// This will register the format with the embedded plugin registry
	_ = Config
}
