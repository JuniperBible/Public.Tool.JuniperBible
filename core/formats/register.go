//go:build !standalone

package accordance

func init() {
	Config.RegisterEmbedded()
}
