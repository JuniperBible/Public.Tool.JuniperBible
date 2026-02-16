//go:build !standalone

package sword

func init() {
	// Embedded registration will be implemented when
	// Config.RegisterEmbedded() is added to SDK
	_ = Config
}
