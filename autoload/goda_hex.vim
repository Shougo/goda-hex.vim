" goda-hex.vim - a hexadecimal editor for Vim/Neovim on goda.vim, the
" successor of ddx.vim.
"
" The Go side (the user's own build of a configuration, see the README) has
" the file, the UIs and the analyzers; this file starts it and passes on what
" is asked. The UIs have their own Vim script (autoload/goda_hex/ui/*.vim).

" The version of this Vim script that the binary goes with (ScriptVersion of
" its Go side): goda.vim refuses a binary of another one, and says to build
" it again. Bumped with it when they no longer work together.
let s:version = #{version: 2, module: 'github.com/Shougo/goda-hex.vim'}

" ---------------------------------------------------------------------------
" Setup

let s:goda = goda#configured#new('hex', s:version)

" Build the user's configuration (a Go module in g:goda_hex#config_dir) with
" goda#build(): with the plugins installed (g:goda#workspace). The running
" binary is stopped, and the new one starts with the next request. When the
" build fails, the previous binary keeps running.
function! goda_hex#build() abort
  call s:goda.build()
  call s:goda.register()
endfunction

call s:goda.register()

" ---------------------------------------------------------------------------
" API (that of ddx.vim)

" Open the file of these options (merged over the configuration) in the
" session of their name, and show it.
function! goda_hex#start(options = {}) abort
  call s:request('start', a:options, s:context())
endfunction

" Read the file of the session {name} again (its changes are lost).
function! goda_hex#restart(name) abort
  call s:request('restart', a:name)
endfunction

" Show the session {name} again.
function! goda_hex#redraw(name) abort
  call s:request('redraw', a:name)
endfunction

" Run the UI action {action} of the session {name}.
function! goda_hex#ui_action(name, action, params = {}) abort
  if a:name ==# '' || getcmdwintype() !=# ''
    return
  endif
  call s:request('uiAction', a:name, a:action, a:params)
endfunction

" The structure of the file of the session {name}: [{name, values}], by the
" first of the option "analyzers" that detects it.
function! goda_hex#analyze(name) abort
  return s:get('analyze', [], a:name)
endfunction

" Change {value} (one of goda_hex#analyze(), with its address) to
" {new_value}: a number for an integer, a string for a string.
function! goda_hex#change(name, value, new_value) abort
  call s:request('change', a:name, a:value, a:new_value)
endfunction

" Move the cursor of the UI of the session {name} to {address}.
function! goda_hex#jump(name, address) abort
  call s:request('jump', a:name, a:address)
endfunction

" The differences of the file of the session {name} with its option
" "anotherPath": [{address, type, oldValue, newValue}].
function! goda_hex#get_diff(name) abort
  return s:get('getDiff', [], a:name)
endfunction

" The strings of the file of the session {name}: [{text, address,
" encoding}], of at least {min_length} characters in {encoding} (ascii,
" utf-8, utf-16le, utf-16be).
function! goda_hex#get_strings(name, min_length = 4, encoding = 'utf-8') abort
  return s:get('getStrings', [], a:name, a:min_length, a:encoding)
endfunction

" The names of the sessions.
function! goda_hex#sessions() abort
  return goda#is_running('hex') ? goda#request('hex', 'sessions') : []
endfunction

" The options of the session {name}: those it was started with, or those a
" start of it would have (without options of its own), as
" ddx#custom#get_current().
function! goda_hex#get_current(name = get(b:, 'goda_hex_name', '')) abort
  if a:name ==# ''
    return {}
  endif
  return goda#request('hex', 'options', a:name)
endfunction

function! s:context() abort
  return #{cwd: getcwd(), bufNr: bufnr('%'), winId: win_getid()}
endfunction

function! s:request(method, ...) abort
  try
    call call('goda#request', ['hex', a:method] + a:000)
  catch
    call goda_hex#util#print_error(v:exception)
  endtry
endfunction

" The answer of {method}, or {default} without a session.
function! s:get(method, default, name, ...) abort
  if a:name ==# ''
    return a:default
  endif
  return call('goda#request', ['hex', a:method, a:name] + a:000)
endfunction
