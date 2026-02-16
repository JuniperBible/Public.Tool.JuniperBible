//go:build !standalone

package bibletime

func init() {
	Config.RegisterEmbedded()
}
