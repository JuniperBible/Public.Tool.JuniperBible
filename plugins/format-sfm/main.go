package main

import (
	"os"

	"github.com/JuniperBible/juniper/core/formats/sfm"
	"github.com/JuniperBible/juniper/plugins/sdk/format"
)

func main() {
	if os.Getenv("JUNIPER_PLUGIN_MODE") == "sdk" {
		runSDK()
		return
	}
	format.Run(sfm.Config)
}
