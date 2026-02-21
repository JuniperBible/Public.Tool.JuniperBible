package main

import (
	"os"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/formats/logos"
	"github.com/JuniperBible/Public.Tool.JuniperBible/plugins/sdk/format"
)

func main() {
	if os.Getenv("JUNIPER_PLUGIN_MODE") == "sdk" {
		runSDK()
		return
	}
	format.Run(logos.Config)
}
