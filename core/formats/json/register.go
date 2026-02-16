//go:build !standalone

package json

func init() {
	Config.RegisterEmbedded()
}
