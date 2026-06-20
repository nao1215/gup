# VHS tape for doc/img/update.gif
# Records "gup update" upgrading ~10 binaries under $GOBIN.
#
# Regenerate with (GOBIN/PATH are inherited from the launching shell):
#   export GOBIN=/tmp/gupdemo/bin PATH="/tmp/gupdemo/tool:$PATH"
#   mkdir -p "$GOBIN" /tmp/gupdemo/tool
#   go build -o /tmp/gupdemo/tool/gup .
#   go install github.com/nao1215/posixer@v0.1.0
#   go install github.com/nao1215/subaru@v1.0.0
#   go install github.com/nao1215/gal/cmd/gal@v1.1.1
#   go install github.com/nao1215/ubume/cmd/ubume@v1.5.0
#   go install github.com/sivchari/tenv/cmd/tenv@v1.0.0
#   go install github.com/Songmu/ghch/cmd/ghch@v0.10.0
#   go install github.com/nao1215/leadtime@v0.0.3
#   go install github.com/mattn/goveralls@v0.0.11
#   go install github.com/fatih/gomodifytags@v1.16.0
#   vhs doc/img/update.tape

Output doc/img/update.gif

Set Shell bash
Set FontSize 18
Set Width 1200
Set Height 420
Set Padding 20
Set TypingSpeed 70ms

Sleep 800ms
Type "gup update" Sleep 400ms Enter
Sleep 6s
