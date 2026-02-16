//go:build standalone

package main

import (
	"github.com/FocuswithJustin/JuniperBible/core/formats/odf"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func main() {
	format.Run(odf.Config)
}
