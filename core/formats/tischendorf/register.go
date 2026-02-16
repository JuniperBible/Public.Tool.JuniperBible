//go:build !standalone

package tischendorf

func init() {
	Config.RegisterEmbedded()
}
