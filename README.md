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

The configuration is a Go module (see [examples/config](examples/config)) in
`~/.vim/goda-hex` (Neovim: `stdpath('config') .. '/goda-hex'`). Build it:

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
