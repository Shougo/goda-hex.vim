" Helpers of goda-hex.vim and of its UIs.

function! goda_hex#util#print_error(msg, name = 'goda-hex') abort
  echohl ErrorMsg
  for line in split(type(a:msg) == v:t_string ? a:msg : string(a:msg), "\n")
    if line !=# ''
      echomsg line =~# '^\[\|^goda' ? line : printf('[%s] %s', a:name, line)
    endif
  endfor
  echohl None
endfunction

function! goda_hex#util#print(msg, name = 'goda-hex') abort
  for line in split(type(a:msg) == v:t_string ? a:msg : string(a:msg), "\n")
    if line !=# ''
      echomsg printf('[%s] %s', a:name, line)
    endif
  endfor
endfunction

" input() that gives '' when it is interrupted.
function! goda_hex#util#input(prompt, text = '') abort
  redraw
  try
    return input(a:prompt, a:text)
  catch /^Vim:Interrupt/
  endtry
  return ''
endfunction
