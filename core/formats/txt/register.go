//go:build !standalone

package txt

func init() {
	Config.RegisterEmbedded()
}
