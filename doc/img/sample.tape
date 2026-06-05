# VHS tape for doc/img/sample.gif
# Records "gup update" upgrading binaries under $GOBIN.
#
# Regenerate with:
#   go build -o /tmp/gupdemo/tool/gup .
#   export GOBIN=/tmp/gupdemo/bin PATH="/tmp/gupdemo/tool:$PATH"
#   mkdir -p "$GOBIN"
#   go install github.com/nao1215/posixer@v0.1.0
#   go install github.com/nao1215/subaru@v1.0.0
#   go install github.com/nao1215/gal/cmd/gal@v1.1.1
#   vhs doc/img/sample.tape
#
# GOBIN/PATH are inherited from the shell that launches vhs, so the recording
# only shows the gup commands themselves.

Output doc/img/sample.gif

Set Shell bash
Set FontSize 20
Set Width 1100
Set Height 560
Set Padding 20
Set TypingSpeed 70ms

Sleep 800ms

Type "gup list" Sleep 500ms Enter
Sleep 2s

Type "gup update" Sleep 500ms Enter
Sleep 3s

Type "gup list" Sleep 500ms Enter
Sleep 3s
