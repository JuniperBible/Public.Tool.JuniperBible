//go:build !standalone

package html

func init() {
	Config.RegisterEmbedded()
}
