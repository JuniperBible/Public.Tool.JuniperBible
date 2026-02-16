//go:build standalone

package main

import (
	"github.com/FocuswithJustin/JuniperBible/core/formats/epub"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
)

func main() {
	format.Run(epub.Config)
}
