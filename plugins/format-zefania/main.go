package main

import (
	"os"

	"github.com/FocuswithJustin/JuniperBible/core/formats/zefania"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func main() {
	if os.Getenv("JUNIPER_PLUGIN_MODE") == "sdk" {
		runSDK()
		return
	}
	format.Run(zefania.Config)
}
