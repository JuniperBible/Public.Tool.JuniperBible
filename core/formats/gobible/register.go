//go:build !standalone

package gobible

func init() {
	Config.RegisterEmbedded()
}
