//go:build !standalone

package mybible

func init() {
	Config.RegisterEmbedded()
}
