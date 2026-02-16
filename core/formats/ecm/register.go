//go:build !standalone

package ecm

func init() {
	Config.RegisterEmbedded()
}
