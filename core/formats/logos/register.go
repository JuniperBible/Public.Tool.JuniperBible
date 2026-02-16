//go:build !standalone

package logos

func init() {
	Config.RegisterEmbedded()
}
