package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/pkg/profile"
)

var (
	actions = map[string]func(){}
	prof    = kingpin.Flag("profile", "Profile performance.").Bool()
)

func register(name string, f func()) func() {
	actions[name] = f
	return f
}

// fail reports an error the way Nix does, on stderr and with a failing exit
// status. Parse and evaluation errors render as a message, a source excerpt
// and a backtrace.
func fail(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func main() {
	kingpin.HelpFlag.Short('h')
	action := kingpin.Parse()
	if *prof {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}
	actions[action]()
}
