// Command binrun: Fetch, verify, and exec the exact artifact a descriptor pins — release binaries, Python tools, signed apps.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yasyf/binrun/internal/cli"
	applog "github.com/yasyf/binrun/internal/log"
)

func main() {
	applog.Setup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// On the transparent-exec path a successful Resolve is followed by an exec
	// that replaces this process, so Run only returns on a runner-domain error.
	// Every such error maps to exit 1: exit 2 is reserved for real hook verdicts,
	// and the only other codes come from the exec'd artifact itself.
	if err := cli.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "binrun: "+cli.Message(err))
		os.Exit(1)
	}
}
