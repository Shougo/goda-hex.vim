" goda-hex.vim - a hexadecimal editor for Vim/Neovim on goda.vim, the
" successor of ddx.vim.
"
" The Go side (the user's own build of a configuration, see the README) has
" the file, the UIs and the analyzers; this file starts it and passes on what
" is asked. The UIs have their own Vim script (autoload/goda_hex/ui/*.vim).

" The version of this Vim script that the binary goes with (ScriptVersion of
" its Go side): goda.vim refuses a binary of another one, and says to build
" it again. Bumped with it when they no longer work together.
let s:version = #{version: 1, module: 'github.com/Shougo/goda-hex.vim'}

" Patches of the options made at run time, as ddx#custom#patch_*(). They
" live here, not in the binary: they survive a restart of it, and they can
" be made before it is started. Every request carries them.
let s:global_patch = {}
let s:local_patch = {}

" ---------------------------------------------------------------------------
" Setup

" The binary is machine specific, so it is not built into the configuration
" directory (which belongs in your dotfiles) but into a cache directory.
function! s:cache_dir() abort
  if has('nvim')
    let base = stdpath('cache')
  elseif !empty($XDG_CACHE_HOME)
    let base = $XDG_CACHE_HOME . '/vim'
  elseif has('win32') && !empty($LOCALAPPDATA)
    let base = $LOCALAPPDATA . '/vim'
  else
    let base = expand('~/.cache/vim')
  endif
  return base . '/goda-hex'
endfunction

function! s:default_bin() abort
  if exists('g:goda_hex#binary')
    return expand(g:goda_hex#binary)
  endif
  return s:cache_dir() . '/goda-hex' . (has('win32') ? '.exe' : '')
endfunction

" The options of goda#register(): the version of the Vim script, and how to
" build the binary when there is none or it is of another version (goda.vim
" builds it then). Not for the binary of g:goda_hex#binary: the user builds it.
function! s:register_opts() abort
  if exists('g:goda_hex#binary')
    return s:version
  endif
  return extend(#{build: function('s:build_binary'), bin: s:default_bin()}, s:version)
endfunction

" The build of the binary for goda.vim, as it starts it (nothing else: it
" is not running).
function! s:build_binary() abort
  call goda#build(#{name: 'hex', dir: s:config_dir(), bin: s:default_bin()})
endfunction

function! s:config_dir() abort
  if exists('g:goda_hex#config_dir')
    return expand(g:goda_hex#config_dir)
  endif
  return (has('nvim') ? stdpath('config') : expand('~/.vim')) . '/goda-hex'
endfunction

" Build the user's configuration (a Go module in g:goda_hex#config_dir) with
" goda#build(): with the plugins installed (g:goda#workspace). The running
" binary is stopped, and the new one starts with the next request. When the
" build fails, the previous binary keeps running.
function! goda_hex#build() abort
  call goda#build(#{name: 'hex', dir: s:config_dir(), bin: s:default_bin()})
  call goda#register('hex', function('s:start'), s:register_opts())
endfunction

" Start the binary: goda.vim calls it when a request needs the plugin "hex"
" and no binary serves it.
function! s:start() abort
  let bin = s:default_bin()
  if !executable(bin)
    throw 'goda-hex: ' . bin . ' does not exist yet; build it with: call goda_hex#build()'
  endif
  let plugins = goda#start(bin)
  if index(plugins, 'hex') < 0
    if !empty(plugins)
      call goda#stop(plugins[0])
    endif
    throw 'goda-hex: ' . bin . ' does not contain the "hex" plugin'
  endif
  " A binary built from an invalid configuration starts anyway and says why.
  let status = goda#request('hex', 'status')
  if type(status) == v:t_string && status !=# ''
    call goda#stop('hex')
    throw 'goda-hex: fix the configuration in ' . s:config_dir()
          \ . ' and run goda_hex#build(): ' . status
  endif
endfunction

call goda#register('hex', function('s:start'), s:register_opts())

" ---------------------------------------------------------------------------
" API (that of ddx.vim)

" Open the file of these options (merged over the configuration and the
" patches) in the session of their name, and show it.
function! goda_hex#start(options = {}) abort
  call s:request('start', a:options, s:context(), s:patches())
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

" ---------------------------------------------------------------------------
" Patches: goda_hex#custom#*() keep them here.

function! goda_hex#_patch_global(patch, set = v:false) abort
  let s:global_patch = a:set ? deepcopy(a:patch) : s:merge(s:global_patch, a:patch)
endfunction

function! goda_hex#_patch_local(name, patch, set = v:false) abort
  let s:local_patch[a:name] = a:set ? deepcopy(a:patch)
        \ : s:merge(get(s:local_patch, a:name, {}), a:patch)
endfunction

function! goda_hex#_get_global() abort
  return deepcopy(s:global_patch)
endfunction

function! goda_hex#_get_local() abort
  return deepcopy(s:local_patch)
endfunction

function! goda_hex#_patches() abort
  return s:patches()
endfunction

function! s:patches() abort
  return #{global: s:global_patch, local: s:local_patch}
endfunction

" {dst} with {src} over it; the params of a UI are merged param by param.
function! s:merge(dst, src) abort
  let out = deepcopy(a:dst)
  for [key, value] in items(a:src)
    if (key ==# 'uiParams' || key ==# 'analyzerParams') && type(value) == v:t_dict
      let params = get(out, key, {})
      for [ui, p] in items(value)
        let params[ui] = extend(get(params, ui, {}), p)
      endfor
      let out[key] = params
    else
      let out[key] = deepcopy(value)
    endif
  endfor
  return out
endfunction
