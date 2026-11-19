# Records the README demo: make demo (needs vhs: brew install vhs)
Output docs/demo.gif
Set Shell zsh
Set FontSize 15
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Env PATH "./:$PATH"

Type "dustit scan" Sleep 300ms Enter
Sleep 12s
Type "clear && dustit doctor" Sleep 300ms Enter
Sleep 6s
Type "clear && dustit clean --only npm-cache,stale-deps --dry-run" Sleep 300ms Enter
Sleep 8s
