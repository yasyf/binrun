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

	if err := cli.NewRootCmd().ExecuteContext(ctx); err != nil {
		// Minimal error handling: report on stderr and exit non-zero. As the CLI
		// grows, map typed errors to exit codes here (see STYLEGUIDE.md § Error Handling).
		fmt.Fprintln(os.Stderr, "binrun:", err)
		os.Exit(1)
	}
}
