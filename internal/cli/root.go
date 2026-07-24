// Package cli routes an invocation to either the transparent-exec path or the
// verb tree, and maps runner-domain errors to a terse message.
//
// binrun has two shapes. "binrun FILE args…" (or a direct shebang invocation,
// where the kernel passes the descriptor as the first argument) resolves the
// descriptor and execs the pinned artifact, forwarding args untouched — cobra
// never sees them, so an artifact's own flags are not interpreted. "binrun --
// VERB …" runs a management verb (fetch, resolve, parse, latest, gc, cache-dir)
// under the cobra tree. The "--" is the separator that selects the verb tree;
// without it the first argument is always a descriptor path.
package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yasyf/binrun/internal/version"
)

type mode int

const (
	modeExec mode = iota
	modeVerbs
)

type route struct {
	mode       mode
	descriptor string
	args       []string
}

// classify decides whether args select the transparent-exec path or the verb
// tree. A leading "--" selects the verbs; any other leading non-flag token is a
// descriptor path; a leading flag (or no args) falls through to the verb root so
// "--version"/"--help" work.
func classify(args []string) route {
	switch {
	case len(args) > 0 && args[0] == "--":
		return route{mode: modeVerbs, args: args[1:]}
	case len(args) > 0 && !strings.HasPrefix(args[0], "-"):
		return route{mode: modeExec, descriptor: args[0], args: args[1:]}
	default:
		return route{mode: modeVerbs, args: args}
	}
}

// Run routes args and executes. On the exec path it does not return on success:
// the resolved artifact replaces this process.
func Run(ctx context.Context, args []string) error {
	switch r := classify(args); r.mode {
	case modeExec:
		return execDescriptor(ctx, r.descriptor, r.args)
	default:
		return runVerbs(ctx, r.args)
	}
}

func runVerbs(ctx context.Context, args []string) error {
	root := newVerbRoot()
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newVerbRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "binrun",
		Short:         "Fetch, verify, and exec the exact artifact a descriptor pins — release binaries, Python tools, signed apps.",
		Long:          "binrun FILE [args…] resolves a descriptor and execs the pinned artifact.\nManagement verbs run behind a \"--\" separator: binrun -- resolve FILE.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(
		newFetchCmd(),
		newResolveCmd(),
		newParseCmd(),
		newLatestCmd(),
		newGCCmd(),
		newCacheDirCmd(),
	)
	return root
}
