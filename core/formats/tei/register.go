//go:build !standalone

package tei

func init() {
	Config.RegisterEmbedded()
}
