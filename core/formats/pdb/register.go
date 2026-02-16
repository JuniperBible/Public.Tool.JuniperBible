//go:build !standalone

package pdb

func init() {
	Config.RegisterEmbedded()
}
