# goda-hex.vim

A hexadecimal editor in Vim and Neovim on
[goda.vim](https://github.com/Shougo/goda.vim): the successor of
[ddx.vim](https://github.com/Shougo/ddx.vim), with the configuration in Go
instead of TypeScript, and no Deno.

- The UI `hex`: the bytes of a file, sixteen a line, with their address and
  their text (ddx-ui-hex). Change, insert, remove, search (with `??` for any
  byte), substitute, undo and redo, checksums, the differences with another
  file; `:write` saves it.
- The analyzers `elf` and `zip`: the headers of an ELF binary and of a zip
  archive (ddx-analyzer-elf, ddx-analyzer-zip).
- The sources `hex_analyze`, `hex_diff`, `hex_string` and the kind `hex` of
  [goda-fi.vim](https://github.com/Shougo/goda-fi.vim) (ddx.vim's ddu
  sources and kind), in `ext/fi/hexfi`.

## Requirements

- Vim 9.1 or Neovim 0.11
- Go 1.26
- [goda.vim](https://github.com/Shougo/goda.vim)

## Usage

The configuration is a Go module in `~/.vim/goda-hex` (Neovim:
`stdpath('config') .. '/goda-hex'`); its `main.go`:

```go
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
	b.PatchGlobal(hex.Options{
		UI:        "hex",
		Analyzers: []string{"elf", "zip"},
	})
	b.Main()
}
```

```sh
go mod init goda-hex-config
go get github.com/Shougo/goda-hex.vim@latest
go mod tidy
```

Build it:

```vim
call goda_hex#build()
```

Then:

```vim
call goda_hex#start(#{path: 'foo.zip'})

autocmd FileType goda-hex-hex call s:goda_hex_settings()
function s:goda_hex_settings() abort
  nnoremap <buffer> q <Cmd>call goda_hex#ui#hex#do_action('quit')<CR>
  nnoremap <buffer> r <Cmd>call goda_hex#ui#hex#do_action('change')<CR>
  nnoremap <buffer> u <Cmd>call goda_hex#ui#hex#do_action('undo')<CR>
  nnoremap <buffer> / <Cmd>call goda_hex#ui#hex#do_action('search')<CR>
endfunction
```

See `:help goda-hex` for everything else, and `:help goda-hex-ddx` for the
differences from ddx.vim.

## With goda-pm.vim

With [goda-pm.vim](https://github.com/Shougo/goda-pm.vim), the build uses
the plugins installed (`g:goda#workspace` of goda.vim): after an update,
build again, with no `go get -u`. To build after every update:

```toml
[[plugins]]
repo = 'Shougo/goda.vim'

[[plugins]]
repo = 'Shougo/goda-hex.vim'
depends = 'goda.vim'
lazy = true
hook_post_update = 'call goda_hex#build()'
```
