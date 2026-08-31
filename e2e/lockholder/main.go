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
//	lockholder -lock <path> [-lock <path>...] -ready <path> [-command <name>] [-hold all|last]
//
// It writes its PID to the ready file once it holds the lock, so a spec waits
// for the fact rather than for a duration, and then blocks forever. There is no
// clean shutdown on purpose: every scenario using it ends by killing it, which
// is also the case worth testing - a holder that dies without cleaning up must
// leave nothing behind that blocks the next gup.
//
// -hold last is what lets a spec put a real gup in the middle of a set. gup
// takes a set of locks in the order the FILESYSTEM decides (see
// lockfile.AcquireAll), which no spec can predict, so "hold the one gup will
// take second" cannot be written as a path. Given the same paths gup will be
// given, this asks gup's own package which of them comes last and holds only
// that: the gup then acquires everything before it and blocks, holding real
// locks that a third process can be refused by.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nao1215/gup/internal/lockfile"
)

// lockPaths collects a repeatable -lock flag.
type lockPaths []string

func (p *lockPaths) String() string     { return strings.Join(*p, ", ") }
func (p *lockPaths) Set(v string) error { *p = append(*p, v); return nil }

const (
	// holdAll takes every -lock given, which is what a single -lock means too.
	holdAll = "all"
	// holdLast takes only the one gup would take last out of the set.
	holdLast = "last"
)

func main() {
	var paths lockPaths
	flag.Var(&paths, "lock", "a lock file to hold; repeat for a set")
	readyPath := flag.String("ready", "", "the file to create once the lock is held")
	command := flag.String("command", "update", "the gup subcommand to record as the lock's owner")
	hold := flag.String("hold", holdAll, "which of the -lock paths to hold: all, or last (the one gup takes last)")
	flag.Parse()

	if len(paths) == 0 || *readyPath == "" {
		fmt.Fprintln(os.Stderr, "lockholder: both -lock and -ready are required")
		os.Exit(2)
	}

	wanted, err := selectPaths(paths, *hold)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lockholder: %v\n", err)
		os.Exit(1)
	}

	lock, err := lockfile.AcquireAll(context.Background(), *command, wanted...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lockholder: can not take %v: %v\n", wanted, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*readyPath, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "lockholder: can not announce readiness: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "lockholder: holding %v\n", lock.Paths())

	// Held until the runner kills it. A blocking channel receive would be tidier
	// and is not usable: the Go runtime calls a program with nothing left to do a
	// deadlock, kills it, and hands the lock straight back to the spec that is
	// asserting nobody else can have it.
	for {
		time.Sleep(time.Hour)
	}
}

// selectPaths reduces the -lock set to what -hold asked for.
//
// The "last" case asks gup's own package for the acquisition order rather than
// sorting anything here, because agreeing with gup is the entire point: an order
// this program worked out for itself would stage a scenario that tests this
// program.
func selectPaths(paths []string, hold string) ([]string, error) {
	switch hold {
	case holdAll:
		return paths, nil
	case holdLast:
		order, err := lockfile.AcquisitionOrder(paths...)
		if err != nil {
			return nil, fmt.Errorf("can not work out the order gup takes %v in: %w", paths, err)
		}
		if len(order) == 0 {
			return nil, fmt.Errorf("no lock to hold out of %v", paths)
		}
		return order[len(order)-1:], nil
	default:
		return nil, fmt.Errorf("unknown -hold %q: want %q or %q", hold, holdAll, holdLast)
	}
}
