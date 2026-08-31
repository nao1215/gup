// Command hardlinker gives an existing file a second name. It exists so the
// end-to-end suite can stage the one attack on gup's lock that a fixture cannot
// describe.
//
// atago's fixtures write files and symlinks, which covers the lock path replaced
// by a symbolic link. A hard link is a different thing and needs a different
// tool: the two names are equally real, so there is nothing for a fixture to
// declare - the file simply has another name, and the lock path that carries it
// looks exactly like a lock file until its link count is asked for.
//
// It is a Go program rather than `ln` or `mklink /H` because this suite runs on
// Windows too, and os.Link is the one spelling that works on all three operating
// systems gup ships for. Nothing here is gup's: the point is to build the
// hostile state from OUTSIDE gup, the way a user or an attacker would, and then
// see what gup does when it meets it.
//
// Usage:
//
//	hardlinker -target <existing file> -link <new name for it>
//
// It exits 0 when the link exists, and 1 with the operating system's own reason
// when it does not - a filesystem that has no hard links included, which is a
// spec failure on purpose: a scenario that silently linked nothing would assert
// gup's refusal against an ordinary lock file and pass without testing anything.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	target := flag.String("target", "", "the existing file to give another name")
	link := flag.String("link", "", "the new name to give it")
	flag.Parse()

	if *target == "" || *link == "" {
		fmt.Fprintln(os.Stderr, "hardlinker: both -target and -link are required")
		os.Exit(2)
	}
	if err := os.Link(*target, *link); err != nil {
		fmt.Fprintf(os.Stderr, "hardlinker: can not link %s to %s: %v\n", *link, *target, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "hardlinker: %s is now a second name for %s\n", *link, *target)
}
