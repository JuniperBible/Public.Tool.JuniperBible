//go:build !standalone

package na28app

func init() {
	Config.RegisterEmbedded()
}
