// Package cli builds the cobra command tree.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/yasyf/binrun/internal/version"
)

// NewRootCmd builds the root command and registers its subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "binrun",
		Short:         "Fetch, verify, and exec the exact artifact a descriptor pins — release binaries, Python tools, signed apps.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newHelloCmd())
	return root
}
