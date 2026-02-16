//go:build !standalone

package usfm

func init() {
	Config.RegisterEmbedded()
}
