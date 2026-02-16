//go:build !standalone

package sfm

func init() {
	Config.RegisterEmbedded()
}
