//go:build !standalone

package crosswire

func init() {
	// Embedded registration will be implemented when
	// Config.RegisterEmbedded() is added to SDK
	_ = Config
}
