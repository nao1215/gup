// Command lockholder takes one of gup's state locks and holds it until it is
// killed. It exists so the end-to-end suite can stage contention.
//
// A spec cannot stage it any other way now that the lock is the kernel's.
// Planting a lock FILE proves nothing, because a file is not a lock: gup will
// take it and run, which is the correct behavior and the opposite of what a
// contention scenario needs. Something has to actually hold the lock, and
// holding it means being a process that is still alive.
//
// It uses gup's own lockfile package rather than an flock(1)-style utility for
// two reasons: the exclusion under test is gup's, so the holder must take the
// lock exactly the way gup does; and there is no such utility on Windows, where
// this suite also runs.
//
// Usage:
//
//	lockholder -lock <path> -ready <path> [-command <name>]
//
// It writes its PID to the ready file once it holds the lock, so a spec waits
// for the fact rather than for a duration, and then blocks forever. There is no
// clean shutdown on purpose: every scenario using it ends by killing it, which
// is also the case worth testing - a holder that dies without cleaning up must
// leave nothing behind that blocks the next gup.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nao1215/gup/internal/lockfile"
)

func main() {
	lockPath := flag.String("lock", "", "the lock file to hold")
	readyPath := flag.String("ready", "", "the file to create once the lock is held")
	command := flag.String("command", "update", "the gup subcommand to record as the lock's owner")
	flag.Parse()

	if *lockPath == "" || *readyPath == "" {
		fmt.Fprintln(os.Stderr, "lockholder: both -lock and -ready are required")
		os.Exit(2)
	}

	lock, err := lockfile.Acquire(context.Background(), *lockPath, *command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lockholder: can not take %s: %v\n", *lockPath, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*readyPath, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "lockholder: can not announce readiness: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "lockholder: holding %s\n", lock.Path())

	// Held until the runner kills it. A blocking channel receive would be tidier
	// and is not usable: the Go runtime calls a program with nothing left to do a
	// deadlock, kills it, and hands the lock straight back to the spec that is
	// asserting nobody else can have it.
	for {
		time.Sleep(time.Hour)
	}
}
