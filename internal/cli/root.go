// Package cli routes an invocation to either the transparent-exec path or the
// verb tree, and maps runner-domain errors to a terse message.
//
// binrun has two shapes. "binrun FILE [args…]" (or a direct shebang invocation,
// where the kernel passes the descriptor as the first argument) resolves the
// descriptor and execs the pinned artifact, forwarding args untouched — cobra
// never sees them, so an artifact's own flags are not interpreted. "binrun --
// VERB …" runs a management verb (fetch, resolve, parse, latest, gc, cache-dir)
// under the cobra tree.
//
// Only four leading-dash strings mean "the runner": a first argument of exactly
// --version, -v, --help, or -h prints binrun's own version or help. A first
// argument of exactly "--" selects the verb tree. Every other first argument is
// a descriptor path — even one starting with "-", which is stat-checked so a
// flag-shaped typo fails with guidance rather than misparsing (dotslash's rule:
// descriptors are invoked by real paths, and ./--version disambiguates the rare
// descriptor named like a flag). No argument at all is a usage error, so a
// wrapper that resolves an empty descriptor fails loud instead of silently
// succeeding.
package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/yasyf/binrun/internal/version"
)

var errNoDescriptor = errors.New("no descriptor given; usage: binrun FILE [args…] (verbs: binrun -- VERB)")

// metaFlags are the only leading-dash first arguments that address the runner
// itself; each maps to its canonical cobra flag.
var metaFlags = map[string]string{
	"--version": "--version",
	"-v":        "--version",
	"--help":    "--help",
	"-h":        "--help",
}

type mode int

const (
	modeExec mode = iota
	modeVerbs
	modeUsage
)

type route struct {
	mode       mode
	descriptor string
	args       []string
}

// classify decides how to dispatch args: a first argument that is exactly a
// runner meta-flag (--version/-v/--help/-h) or exactly "--" reaches the cobra
// tree; any other first argument is a descriptor path (even one starting with
// "-"); and no arguments at all is a usage error.
func classify(args []string) route {
	switch {
	case len(args) == 0:
		return route{mode: modeUsage}
	case args[0] == "--":
		return route{mode: modeVerbs, args: args[1:]}
	default:
		if canonical, ok := metaFlags[args[0]]; ok {
			return route{mode: modeVerbs, args: []string{canonical}}
		}
		return route{mode: modeExec, descriptor: args[0], args: args[1:]}
	}
}

// Run routes args and executes. On the exec path it does not return on success:
// the resolved artifact replaces this process.
func Run(ctx context.Context, args []string) error {
	switch r := classify(args); r.mode {
	case modeExec:
		return execDescriptor(ctx, r.descriptor, r.args)
	case modeVerbs:
		return runVerbs(ctx, r.args)
	default:
		return errNoDescriptor
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
