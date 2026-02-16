package main

import (
	"os"

	"github.com/FocuswithJustin/JuniperBible/core/formats/osis"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func main() {
	if os.Getenv("JUNIPER_PLUGIN_MODE") == "sdk" {
		runSDK()
		return
	}
	format.Run(osis.Config)
}
