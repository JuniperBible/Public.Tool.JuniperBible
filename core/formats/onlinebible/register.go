//go:build !standalone

package onlinebible

func init() {
	Config.RegisterEmbedded()
}
