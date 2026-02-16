//go:build !standalone

package sqlite

func init() {
	Config.RegisterEmbedded()
}
