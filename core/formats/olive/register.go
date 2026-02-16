//go:build !standalone

package olive

func init() {
	Config.RegisterEmbedded()
}
