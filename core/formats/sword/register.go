//go:build !standalone

package sword

func init() {
	Config.RegisterEmbedded()
}
