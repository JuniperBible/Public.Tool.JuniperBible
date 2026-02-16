//go:build !standalone

package morphgnt

func init() {
	Config.RegisterEmbedded()
}
