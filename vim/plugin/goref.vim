if !exists("g:goref_command")
    let g:goref_command = "goref"
endif

function! GorefUnderCursor()
    let pos = getpos(".")[1:2]
    if &encoding == 'utf-8'
        let offs = line2byte(pos[0]) + pos[1] - 2
    else
        let c = pos[1]
        let buf = line('.') == 1 ? "" : (join(getline(1, pos[0] - 1), "\n") . "\n")
        let buf .= c == 1 ? "" : getline(pos[0])[:c-2]
        let offs = len(iconv(buf, &encoding, "utf-8"))
    endif
    silent call Goref("-o=" . offs)
endfunction

function! Goref(arg)
    let bufname = bufname('%')
    let references=system(g:goref_command . " -v -R -f=" . bufname . " " . shellescape(a:arg) . " " . getcwd())

    let old_efm = &efm
    let &efm="%I%f:%l:%c,%C,%Z%m"

    if references =~ 'goref: '
        let references=substitute(references, '\n$', '', '')
        echom references
    else
        cexpr references
    end

    let &efm=old_efm
endfunction

autocmd FileType go nnoremap <buffer> gr :call GorefUnderCursor()<cr>
command! -range -nargs=1 Goref :call Goref(<q-args>)
