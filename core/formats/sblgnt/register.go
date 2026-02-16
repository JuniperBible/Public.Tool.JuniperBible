//go:build !standalone

package sblgnt

func init() {
	Config.RegisterEmbedded()
}
