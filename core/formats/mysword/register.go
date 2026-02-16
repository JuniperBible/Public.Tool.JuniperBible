//go:build !standalone

package mysword

func init() {
	Config.RegisterEmbedded()
}
