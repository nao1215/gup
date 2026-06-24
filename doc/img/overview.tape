# VHS tape for doc/img/overview.png
# A single still that showcases gup's main workflow end to end: list the global
# $GOBIN tool set, pin one tool to a version, check the set, and update it (the
# pinned tool stays put while the rest update in parallel). Used for external
# showcases such as Terminal Trove; it is not referenced from the README.
#
# Regenerate with the same isolated GOBIN/HOME setup as doc/img/sample.tape:
#   vhs doc/img/overview.tape

# vhs requires a video Output; the still we want is captured with Screenshot.
Output doc/img/overview-capture.gif

Set Shell bash
Set FontSize 20
Set Width 1100
Set Height 600
Set Padding 20
Set TypingSpeed 70ms

Sleep 800ms

Type "gup list" Sleep 500ms Enter
Sleep 1500ms

Type "gup pin posixer v0.1.0" Sleep 500ms Enter
Sleep 1200ms

Type "gup check" Sleep 500ms Enter
Sleep 3s

Type "gup update" Sleep 500ms Enter
Sleep 4s

Screenshot doc/img/overview.png
