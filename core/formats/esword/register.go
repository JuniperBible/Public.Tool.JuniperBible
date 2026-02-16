//go:build !standalone

package esword

func init() {
	Config.RegisterEmbedded()
}
