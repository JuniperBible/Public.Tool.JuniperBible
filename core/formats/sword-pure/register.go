//go:build !standalone

package swordpure

func init() {
	// Embedded registration will be implemented when
	// Config.RegisterEmbedded() is added to SDK
	_ = Config
}
