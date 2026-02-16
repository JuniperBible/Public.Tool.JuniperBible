//go:build !standalone

package osis

func init() {
	Config.RegisterEmbedded()
}
