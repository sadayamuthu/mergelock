Output demo.gif
Set Shell bash
Set FontSize 15
Set Width 1500
Set Height 440
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms

Hide
Type "bash /work/setup.sh && cd /tmp/shop && export PS1='$ ' && clear" Enter
Sleep 1s
Show

Type "# Two coding agents, two branches, working in parallel" Enter
Sleep 800ms
Type "git log --oneline main..agent/auth-hardening; git log --oneline main..agent/checkout-api" Enter
Sleep 2s
Type "# git: no conflict, merges cleanly" Enter
Sleep 600ms
Type "git merge-tree --write-tree agent/auth-hardening agent/checkout-api >/dev/null && echo 'merge OK'" Enter
Sleep 2s
Type "# ...but checkout.ts calls login() the old way. mergelock sees it:" Enter
Sleep 800ms
Type "mergelock check agent/auth-hardening agent/checkout-api --format text" Enter
Sleep 7s
