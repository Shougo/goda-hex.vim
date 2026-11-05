" The end-to-end scenario: the UI in a real Vim or Neovim. Failures go to
" $GODA_HEX_RESULT; it says "ok" at the end.

set nocompatible
set encoding=utf-8
set nomore
syntax on
let s:errors = []
let s:result = $GODA_HEX_RESULT
let s:files = $GODA_HEX_FILES

execute 'set runtimepath^=' . fnameescape($GODA_VIM_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_HEX_DIR)
let g:goda_hex#binary = $GODA_HEX_BINARY

function! s:check(name, got, want) abort
  if a:got !=# a:want
    call add(s:errors, printf('%s: got %s, want %s', a:name, string(a:got), string(a:want)))
  endif
endfunction

function! s:file(name) abort
  return s:files . '/' . a:name
endfunction

" The highlights of the line {lnum}: [[col, length, group]].
function! s:highlights(lnum) abort
  if has('nvim')
    let ns = nvim_get_namespaces()['goda-hex-ui-hex']
    return map(nvim_buf_get_extmarks(0, ns, [a:lnum - 1, 0], [a:lnum - 1, -1], #{details: v:true}),
          \ {_, m -> [m[2] + 1, m[3].end_col - m[2], m[3].hl_group]})
  endif
  return map(prop_list(a:lnum), {_, p -> [p.col, p.length,
        \ prop_type_get(p.type).highlight]})
endfunction

" Type {input} in the input() of the action {action}.
function! s:input(action, input, params = {}) abort
  call feedkeys(printf(":call goda_hex#ui#hex#do_action(%s, %s)\<CR>%s\<CR>",
        \ string(a:action), string(a:params), a:input), 'xt')
endfunction

" The byte {address} under the cursor.
function! s:goto(address) abort
  call goda_hex#ui#hex#_cursor(win_getid(), a:address, 0)
endfunction

function! s:hex() abort
  let path = s:file('a.bin')
  call goda_hex#start(#{path: path})
  call s:check('filetype', &filetype, 'goda-hex-hex')
  call s:check('name', b:goda_hex_name, 'default')
  call s:check('sessions', goda_hex#sessions(), ['default'])
  call s:check('lines', getline(1, '$'), [
        \ '00000000: 48 65 6c 6c 6f 2c 20 77 6f 72 6c 64 21 0a 00 01 |   Hello, world!...',
        \ '00000010: 02 e3 81 82 ff                                  |   .あ.',
        \ ])
  call s:check('modified', &modified, 0)

  " The address under the cursor: on a byte, on its text.
  call cursor(1, 14)
  call s:check('address hex', goda_hex#ui#hex#_get_current_address(), ['hex', 1])
  call cursor(1, 63 + 4)
  call s:check('address ascii', goda_hex#ui#hex#_get_current_address(), ['ascii', 4])

  " A selection, yanked.
  call s:goto(1)
  call goda_hex#ui#hex#do_action('selectAddress')
  call s:check('selected', s:highlights(1), [[14, 2, 'Visual']])
  call s:goto(3)
  call goda_hex#ui#hex#do_action('yank')
  call s:check('yank', getreg('"'), '65 6c 6c')
  call s:check('unselected', s:highlights(1), [])

  " change, undo, redo: the changed byte highlighted.
  call s:goto(3)
  call s:input('change', '41')
  call s:check('change', getline(1)[:29], '00000000: 48 65 6c 41 6f 2c 20')
  call s:check('change modified', &modified, 1)
  call s:check('change highlight', s:highlights(1), [[20, 2, 'ErrorMsg']])
  call s:check('change cursor', goda_hex#ui#hex#_get_current_address(), ['hex', 3])
  call goda_hex#ui#hex#do_action('undo')
  call s:check('undo', [getline(1)[:29], &modified], ['00000000: 48 65 6c 6c 6f 2c 20', 0])
  call goda_hex#ui#hex#do_action('redo')
  call s:check('redo', getline(1)[:29], '00000000: 48 65 6c 41 6f 2c 20')
  call goda_hex#ui#hex#do_action('undo')

  " A number of 2 bytes, big endian.
  call s:goto(0)
  call s:input('change', '0x4142', #{type: 'number', size: 2, isLittle: v:false})
  call s:check('change number', getline(1)[:20], '00000000: 41 42 6c 6c')
  call goda_hex#ui#hex#do_action('undo')

  " insert, remove, paste.
  call s:goto(0)
  call s:input('insert', 'ab', #{type: 'string'})
  call s:check('insert', getline(1)[:23], '00000000: 61 62 48 65 6c')
  call goda_hex#ui#hex#do_action('remove')
  call s:check('remove', getline(1)[:20], '00000000: 62 48 65 6c')
  call s:goto(1)
  call goda_hex#ui#hex#do_action('paste')
  call s:check('paste', getline(1)[:23], '00000000: 62 61 48 65 6c')
  call goda_hex#ui#hex#do_action('undo')
  call goda_hex#ui#hex#do_action('undo')
  call goda_hex#ui#hex#do_action('undo')
  call s:check('undo all', [getline(1)[:20], &modified], ['00000000: 48 65 6c 6c', 0])

  " search (with "??"), nextSearch, backward; jump.
  call s:goto(0)
  call s:input('search', '6c ??')
  call s:check('search', goda_hex#ui#hex#_get_current_address(), ['hex', 2])
  call goda_hex#ui#hex#do_action('nextSearch')
  call s:check('nextSearch', goda_hex#ui#hex#_get_current_address(), ['hex', 3])
  call goda_hex#ui#hex#do_action('nextSearch', #{direction: 'backward'})
  call s:check('nextSearch backward', goda_hex#ui#hex#_get_current_address(), ['hex', 2])
  " (input() starts with "0x".)
  call s:input('jump', '12')
  call s:check('jump', [line('.'), goda_hex#ui#hex#_get_current_address()], [2, ['hex', 0x12]])
  call goda_hex#jump('default', 5)
  call s:check('goda_hex#jump', goda_hex#ui#hex#_get_current_address(), ['hex', 5])

  " substitute from the cursor; checksum and inspect of a selection.
  call s:goto(0)
  call feedkeys(":call goda_hex#ui#hex#do_action('substitute')\<CR>6c\<CR>4c\<CR>", 'xt')
  call s:check('substitute', getline(1)[16:41], '4c 4c 6f 2c 20 77 6f 72 4c')
  call goda_hex#ui#hex#do_action('undo')
  call s:goto(0)
  call goda_hex#ui#hex#do_action('selectAddress')
  call s:goto(8)
  messages clear
  call goda_hex#ui#hex#do_action('checksum', #{method: 'crc-32'})
  call s:check('checksum', execute('messages') =~# 'Checksum crc-32: 5CD0D7DC', v:true)
  call s:goto(0)
  call goda_hex#ui#hex#do_action('inspect', #{type: 'number', size: 2})
  call s:check('inspect', execute('messages') =~# 'Number: 25928 (0x6548)', v:true)
  call goda_hex#ui#hex#do_action('selectAddress')

  " save, to another file, and :write.
  call s:goto(13)
  call s:input('change', '21')
  call goda_hex#ui#hex#do_action('save', #{path: s:file('c.bin')})
  call s:check('save', readfile(s:file('c.bin'), 'b')[0][:13], 'Hello, world!!')
  call s:check('save modified', &modified, 0)
  call s:input('change', '0a')
  silent write
  call s:check(':write', readfile(path, 'b')[0], 'Hello, world!')
  call s:check(':write modified', &modified, 0)

  " The strings, by the API of the sources.
  call s:check('strings', goda_hex#get_strings('default', 4, 'utf-8'),
        \ [#{text: "Hello, world!\n", address: 0, encoding: 'utf-8'}])

  call goda_hex#ui#hex#do_action('quit')
  call s:check('quit', bufwinid(bufnr('^goda-hex-default$')), -1)
endfunction

" The diff with anotherPath: the bytes changed highlighted, nextDiff.
function! s:diff() abort
  call goda_hex#start(#{name: 'diff', path: s:file('a.bin'), anotherPath: s:file('b.bin')})
  call s:check('diff', goda_hex#get_diff('diff'), [
        \ #{address: 7, type: 'changed', oldValue: [0x77], newValue: [0x57]},
        \ #{address: 20, type: 'deleted', oldValue: [0xff], newValue: []},
        \ ])
  call s:check('diff highlight', s:highlights(1), [[32, 2, 'DiffChange']])
  call s:goto(0)
  call goda_hex#ui#hex#do_action('nextDiff')
  call s:check('nextDiff', goda_hex#ui#hex#_get_current_address(), ['hex', 7])
  call goda_hex#ui#hex#do_action('quit')
endfunction

" The analyzer: that of a zip archive; goda_hex#change() of a value.
function! s:analyze() abort
  call goda_hex#start(#{name: 'zip', path: s:file('a.zip')})
  let res = goda_hex#analyze('zip')
  call s:check('analyze', map(copy(res), 'v:val.name')[:1], ['ZIP_HEADER', 'ZIP_HEADER(PK78)'])
  let name = filter(copy(res[0].values), 'v:val.name ==# "filename"')
  call s:check('analyze filename', [name[0].value, name[0].address], ['a.txt', 30])
  call goda_hex#change('zip', name[0], 'b.txt')
  call s:check('change value', goda_hex#analyze('zip')[0].values[-3].value, 'b.txt')
  let ver = res[0].values[4]
  call goda_hex#change('zip', ver, 21)
  call s:check('change integer', goda_hex#analyze('zip')[0].values[4].value, 21)
  call goda_hex#ui#hex#do_action('quit')
endfunction

try
  call s:hex()
  call s:diff()
  call s:analyze()
catch
  call add(s:errors, v:throwpoint . ': ' . v:exception)
endtry
call writefile(s:errors + (empty(s:errors) ? ['ok'] : []) + goda#log(), s:result)
call goda#stop()
qall!
