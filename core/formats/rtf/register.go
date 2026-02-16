//go:build !standalone

package rtf

func init() {
	Config.RegisterEmbedded()
}
