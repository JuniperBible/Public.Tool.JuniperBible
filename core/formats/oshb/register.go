//go:build !standalone

package oshb

func init() {
	Config.RegisterEmbedded()
}
