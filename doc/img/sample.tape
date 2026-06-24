# VHS tape for doc/img/sample.gif
# Records gup managing a $GOBIN tool set: listing, pinning one tool to a
# version, checking, and updating (the pinned tool stays put while the rest
# update).
#
# Regenerate with an isolated GOBIN/HOME so the recording never touches your
# real environment:
#   demo=/tmp/gupdemo
#   rm -rf "$demo"; mkdir -p "$demo/tool" "$demo/bin" "$demo/home/.config"
#   go build -o "$demo/tool/gup" .
#   export GOBIN="$demo/bin" HOME="$demo/home" XDG_CONFIG_HOME="$demo/home/.config"
#   export PATH="$demo/tool:$PATH"
#   go install github.com/nao1215/posixer@v0.1.0
#   go install github.com/nao1215/subaru@v1.0.0
#   go install github.com/nao1215/gal/cmd/gal@v1.1.1
#   vhs doc/img/sample.tape
#
# GOBIN/HOME/PATH are inherited from the shell that launches vhs, so the
# recording only shows the gup commands themselves.

Output doc/img/sample.gif

Set Shell bash
Set FontSize 20
Set Width 1100
Set Height 600
Set Padding 20
Set TypingSpeed 70ms

Sleep 800ms

Type "gup list" Sleep 500ms Enter
Sleep 2s

Type "gup pin posixer v0.1.0" Sleep 500ms Enter
Sleep 1500ms

Type "gup check" Sleep 500ms Enter
Sleep 3s

Type "gup update" Sleep 500ms Enter
Sleep 4s
