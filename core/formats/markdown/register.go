//go:build !standalone

package markdown

func init() {
	Config.RegisterEmbedded()
}
