package main

import (
	"os"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/markdown"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
)

func main() {
	if os.Getenv("JUNIPER_PLUGIN_MODE") == "sdk" {
		runSDK()
		return
	}
	format.Run(markdown.Config)
}
