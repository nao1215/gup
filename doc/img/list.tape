# VHS tape for doc/img/list.gif
# Records "gup list" showing installed binaries with import path and version.
# Use the same $GOBIN setup as doc/img/update.tape, then:
#   vhs doc/img/list.tape

Output doc/img/list.gif

Set Shell bash
Set FontSize 18
Set Width 1200
Set Height 420
Set Padding 20
Set TypingSpeed 70ms

Sleep 800ms
Type "gup list" Sleep 400ms Enter
Sleep 3s
