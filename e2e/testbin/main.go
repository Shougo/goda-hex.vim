// Command testbin is the binary of the end-to-end tests: the UI hex and the
// analyzers.
package main

import (
	"github.com/Shougo/goda-hex.vim/ext/analyzer/elf"
	"github.com/Shougo/goda-hex.vim/ext/analyzer/zip"
	hexui "github.com/Shougo/goda-hex.vim/ext/ui/hex"
	"github.com/Shougo/goda-hex.vim/hex"
)

func main() {
	b := hex.New()
	b.AddUI(hexui.New())
	b.AddAnalyzer(elf.New())
	b.AddAnalyzer(zip.New())
	b.PatchGlobal(hex.Options{UI: "hex", Analyzers: []string{"elf", "zip"}})
	b.Main()
}
