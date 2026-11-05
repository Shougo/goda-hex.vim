" The Vim script of the UI "hex" (ext/ui/hex): its buffer, its window, and
" the address under the cursor.

let s:ns = has('nvim') ? nvim_create_namespace('goda-hex-ui-hex') : 0

" Run the action {name} of the session of this buffer.
function! goda_hex#ui#hex#do_action(name, params = {}) abort
  if !exists('b:goda_hex_name') || &filetype !=# 'goda-hex-hex'
    return
  endif
  call goda_hex#ui_action(b:goda_hex_name, a:name, a:params)
endfunction

" [type, address] of the text {cur_text} (of the line {string} to the
" cursor): type "address" (on the address), "hex" (on a byte) or "ascii"
" (on its text).
function! goda_hex#ui#hex#parse_address(string, cur_text, encoding) abort
  let address = str2nr(matchstr(a:string, '^\x\+'), 16)
  let type = 'address'
  if a:cur_text =~# '^\s*\x\+\s*:[[:xdigit:][:space:]]\+\S$'
    let offset = len(split(matchstr(a:cur_text,
          \ '^\s*\x\+\s*:\zs[[:xdigit:][:space:]]\+$'))) - 1
    if 0 <= offset && offset < 16
      return ['hex', address + offset]
    endif
  elseif a:cur_text =~# '\x\+\s\+|.*$'
    let chars = matchstr(a:cur_text, '\x\+\s\+|\zs.*\ze.$')
    let offset = (a:encoding ==# 'latin1' ? len(chars) : strwidth(chars)) - 3
    if 0 <= offset && offset < 16
      return ['ascii', address + offset]
    endif
  endif
  return [type, address]
endfunction

function! goda_hex#ui#hex#get_cur_text(string, col) abort
  return matchstr(a:string, '^.*\%' . a:col . 'c.')
endfunction

" [type, address] under the cursor (['', 0] out of a buffer of the UI).
function! goda_hex#ui#hex#_get_current_address() abort
  if !exists('b:goda_hex_encoding')
    return ['', 0]
  endif
  let line = getline('.')
  if line !~# '^\x\+:'
    return ['', 0]
  endif
  return goda_hex#ui#hex#parse_address(line,
        \ goda_hex#ui#hex#get_cur_text(line, col('.')), b:goda_hex_encoding)
endfunction

" The text of the byte under the cursor: highlighted.
function! goda_hex#ui#hex#_highlight_cursor() abort
  if !exists('b:goda_hex_encoding')
    return
  endif
  let [type, address] = goda_hex#ui#hex#_get_current_address()
  silent! call matchdelete(b:goda_hex_cursor_match)
  if type ==# 'hex'
    " The address, ": ", 16 bytes of 3 columns, " |   ".
    let col = b:goda_hex_addr_width + 2 + 47 + 5 + 1 + address % 16
    let b:goda_hex_cursor_match = matchadd(
          \ get(b:goda_hex_highlights, 'cursorAscii', 'Search'),
          \ printf('\%%.l\%%%dv', col), 100)
  endif
endfunction

" The status line: the line and the address.
function! goda_hex#ui#hex#_status() abort
  let st = get(w:, 'goda_hex_ui_hex_status', {})
  let last = get(st, 'size', 0) > 1 ? st.offset + st.size - 1 : 0
  return printf('%' . (len(line('$')) + 1) . 'd/%d 0x%08x/0x%08x',
        \ line('.'), line('$'), goda_hex#ui#hex#_get_current_address()[1], last)
endfunction

function! s:buffer(name) abort
  let bufname = 'goda-hex-' . a:name
  let bufnr = bufnr('^' . bufname . '$')
  if bufnr > 0 && bufloaded(bufnr)
    return [bufnr, 0]
  endif
  let bufnr = bufadd(bufname)
  call bufload(bufnr)
  call setbufvar(bufnr, '&bufhidden', 'hide')
  call setbufvar(bufnr, '&buftype', 'acwrite')
  call setbufvar(bufnr, '&swapfile', 0)
  call setbufvar(bufnr, '&buflisted', 0)
  let group = 'goda-hex-ui-hex-' . bufnr
  execute 'augroup' group
    autocmd!
    execute 'autocmd CursorMoved <buffer=' . bufnr . '> call goda_hex#ui#hex#_highlight_cursor()'
    " :write saves the file.
    execute 'autocmd BufWriteCmd <buffer=' . bufnr . '> call goda_hex#ui#hex#do_action("save")'
  augroup END
  return [bufnr, 1]
endfunction

function! s:open_window(bufnr, p) abort
  if a:p.split ==# 'horizontal'
    execute printf('silent keepalt %s sbuffer +resize\ %d %d',
          \ a:p.splitDirection, a:p.winHeight, a:bufnr)
  elseif a:p.split ==# 'vertical'
    execute printf('silent keepalt vertical %s sbuffer +vertical\ resize\ %d %d',
          \ a:p.splitDirection, a:p.winWidth > 0 ? a:p.winWidth : &columns / 2, a:bufnr)
  elseif a:p.split ==# 'floating' && has('nvim')
    call nvim_open_win(a:bufnr, v:true, #{
          \ relative: 'editor',
          \ row: a:p.winRow > 0 ? a:p.winRow : &lines / 2 - 10,
          \ col: a:p.winCol > 0 ? a:p.winCol : &columns / 4,
          \ width: a:p.winWidth > 0 ? a:p.winWidth : &columns / 2,
          \ height: a:p.winHeight,
          \ border: a:p.floatingBorder,
          \ })
    let &l:winhighlight = 'Normal:' . get(a:p.highlights, 'floating', 'NormalFloat')
  elseif a:p.split ==# 'no' || a:p.split ==# 'floating'
    execute 'silent keepalt buffer' a:bufnr
  else
    throw 'goda-hex: invalid split param: ' . a:p.split
  endif
  return bufwinid(a:bufnr)
endfunction

function! s:init_window(winid, bufnr, r) abort
  for [name, value] in items(#{list: 0, colorcolumn: '', cursorline: 1,
        \ foldcolumn: '0', foldenable: 0, number: 0, relativenumber: 0,
        \ signcolumn: 'no', spell: 0, wrap: 0})
    call setwinvar(a:winid, '&' . name, value)
  endfor
  if a:r.params.split ==# 'horizontal'
    call setwinvar(a:winid, '&winfixheight', 1)
  elseif a:r.params.split ==# 'vertical'
    call setwinvar(a:winid, '&winfixwidth', 1)
  endif
  call setwinvar(a:winid, 'goda_hex_ui_hex_status', a:r.status)
  let header = '[goda-hex-' . a:r.name . '] '
  let status = '%{goda_hex#ui#hex#_status()}'
  if &laststatus == 0 || a:r.params.overwriteTitle
    if !exists('g:goda_hex#ui#hex#_save_title')
      let g:goda_hex#ui#hex#_save_title = &titlestring
    endif
    call setbufvar(a:bufnr, 'goda_hex_ui_hex_title', header . status)
    let &titlestring = header . status
    execute 'augroup goda-hex-ui-hex-' . a:bufnr
      execute 'autocmd WinEnter,BufEnter <buffer=' . a:bufnr . '>'
            \ 'let &titlestring = get(b:, "goda_hex_ui_hex_title", "")'
      execute 'autocmd WinClosed,BufLeave <buffer=' . a:bufnr . '>'
            \ 'let &titlestring = g:goda_hex#ui#hex#_save_title'
    augroup END
  elseif a:r.params.split ==# 'floating' && has('nvim')
    call setwinvar(a:winid, '&statusline', '')
  elseif a:r.params.overwriteStatusline
    call setwinvar(a:winid, '&statusline',
          \ escape(header, '%') . ' %#LineNR#' . status . '%*')
  endif
endfunction

" Draw {r} (ext/ui/hex): the lines (or those changed), the highlights; the
" window opened when it is not shown. Returns {bufnr, full}: full when all
" the lines are needed (a new buffer, but only those changed were sent).
function! goda_hex#ui#hex#_redraw(name, r) abort
  let [bufnr, new] = s:buffer(a:name)
  let winid = bufwinid(bufnr)
  let opened = winid < 0
  if opened
    let winid = s:open_window(bufnr, a:r.params)
  endif
  if new || opened
    call s:init_window(winid, bufnr, a:r)
    call setbufvar(bufnr, '&filetype', 'goda-hex-hex')
  endif
  call setwinvar(winid, 'goda_hex_ui_hex_status', a:r.status)
  call setbufvar(bufnr, 'goda_hex_name', a:name)
  call setbufvar(bufnr, 'goda_hex_encoding', a:r.params.encoding)
  call setbufvar(bufnr, 'goda_hex_highlights', a:r.params.highlights)
  call setbufvar(bufnr, 'goda_hex_addr_width', a:r.addrWidth)
  call setbufvar(bufnr, 'goda_hex_ui_hex_prev', [a:r.prevBufnr, a:r.prevWinid])

  " The address under the cursor, to be there again.
  let address = -1
  if !new && !opened
    let address = win_execute(winid, 'echon goda_hex#ui#hex#_get_current_address()[1]')->str2nr()
  endif

  if a:r.lines is v:null && (new || getbufvar(bufnr, 'goda_hex_ui_hex_drawn', 0) == 0)
    return #{bufnr: bufnr, full: v:true}
  endif
  call setbufvar(bufnr, '&modifiable', 1)
  if a:r.lines is v:null
    for [lnum, line] in a:r.changes
      call setbufline(bufnr, lnum, line)
    endfor
  elseif has('nvim')
    call nvim_buf_set_lines(bufnr, 0, -1, v:false, a:r.lines)
  else
    let n = getbufinfo(bufnr)[0].linecount
    call setbufline(bufnr, 1, a:r.lines)
    if n > len(a:r.lines)
      call deletebufline(bufnr, len(a:r.lines) + 1, n)
    endif
  endif
  call setbufvar(bufnr, '&modifiable', 0)
  call setbufvar(bufnr, 'goda_hex_ui_hex_drawn', 1)
  call s:highlight(bufnr, a:r.highlights)
  call setbufvar(bufnr, '&modified', a:r.modified)
  if address >= 0
    call goda_hex#ui#hex#_cursor(winid, address, a:r.offset)
  endif
  return #{bufnr: bufnr, full: v:false}
endfunction

function! s:highlight(bufnr, hls) abort
  if has('nvim')
    call nvim_buf_clear_namespace(a:bufnr, s:ns, 0, -1)
    for [lnum, col, len, group] in a:hls
      call nvim_buf_set_extmark(a:bufnr, s:ns, lnum - 1, col,
            \ #{end_col: col + len, hl_group: group})
    endfor
    return
  endif
  call prop_clear(1, max([1, getbufinfo(a:bufnr)[0].linecount]), #{bufnr: a:bufnr})
  for [lnum, col, len, group] in a:hls
    let type = 'goda_hex_' . group
    if empty(prop_type_get(type))
      call prop_type_add(type, #{highlight: group, priority: 10, override: v:true})
    endif
    call prop_add(lnum, col + 1, #{length: len, type: type, bufnr: a:bufnr})
  endfor
endfunction

" The cursor of the window {winid} on the byte {address} (of the buffer
" from {offset}).
function! goda_hex#ui#hex#_cursor(winid, address, offset) abort
  if a:address < a:offset
    return
  endif
  let row = (a:address - a:offset) / 16 + 1
  let width = getbufvar(winbufnr(a:winid), 'goda_hex_addr_width', 8)
  let col = width + 3 + (a:address - (a:offset + (row - 1) * 16)) * 3
  call win_execute(a:winid, printf('call cursor(%d, %d)', row, col))
  call win_execute(a:winid, 'call goda_hex#ui#hex#_highlight_cursor()')
endfunction

" Go to the window of the session {name}, on the byte {address}.
function! goda_hex#ui#hex#_jump(name, address, offset) abort
  let winid = bufwinid(bufnr('^goda-hex-' . a:name . '$'))
  if winid < 0
    return
  endif
  call win_gotoid(winid)
  call goda_hex#ui#hex#_cursor(winid, a:address, a:offset)
endfunction

" Close the window of the session {name}: the buffer it was started from
" again without another window.
function! goda_hex#ui#hex#_quit(name, split, prev_bufnr, prev_winid) abort
  let bufnr = bufnr('^goda-hex-' . a:name . '$')
  for winid in win_findbuf(bufnr)
    call win_gotoid(winid)
    if a:split ==# 'no' || winnr('$') == 1
      if a:prev_bufnr > 0 && a:prev_bufnr != bufnr && bufexists(a:prev_bufnr)
        execute 'buffer!' a:prev_bufnr
      else
        enew!
      endif
    else
      silent! close!
      call win_gotoid(a:prev_winid)
    endif
  endfor
endfunction

" The text yanked: in the unnamed register and v:register.
function! goda_hex#ui#hex#_yank(text) abort
  call setreg('"', a:text, 'v')
  if v:register !=# '"'
    call setreg(v:register, a:text, 'v')
  endif
endfunction
