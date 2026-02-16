//go:build !standalone

package theword

func init() {
	Config.RegisterEmbedded()
}
