//go:build !standalone

package xml

func init() {
	Config.RegisterEmbedded()
}
