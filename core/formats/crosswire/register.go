//go:build !standalone

package crosswire

func init() {
	Config.RegisterEmbedded()
}
