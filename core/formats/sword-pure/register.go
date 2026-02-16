//go:build !standalone

package swordpure

func init() {
	Config.RegisterEmbedded()
}
