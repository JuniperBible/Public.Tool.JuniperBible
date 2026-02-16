//go:build !standalone

package flex

func init() {
	Config.RegisterEmbedded()
}
