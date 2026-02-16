//go:build !standalone

package accordance

func init() {
	// Embedded registration will be implemented when
	// Config.RegisterEmbedded() is added to SDK
	_ = Config
}
