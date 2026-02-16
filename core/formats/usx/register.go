//go:build !standalone

package usx

func init() {
	Config.RegisterEmbedded()
}
