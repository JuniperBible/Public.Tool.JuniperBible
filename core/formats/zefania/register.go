//go:build !standalone

package zefania

func init() {
	Config.RegisterEmbedded()
}
