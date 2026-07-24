package cli

import (
	"context"
	"os"
	"syscall"

	"github.com/yasyf/daemonkit/artifact"
)

// execDescriptor resolves the descriptor at path and replaces this process with
// the pinned artifact, forwarding args as-is. syscall.Exec returns only on
// failure; on success the artifact's exit code becomes binrun's.
func execDescriptor(ctx context.Context, path string, args []string) error {
	desc, err := artifact.ParseFile(path)
	if err != nil {
		return err
	}
	store, err := artifact.DefaultStore()
	if err != nil {
		return err
	}
	resolved, err := store.Resolve(ctx, desc, resolveOptions()...)
	if err != nil {
		return err
	}
	// Exec'ing a resolved, digest-verified artifact is binrun's whole purpose.
	return syscall.Exec(resolved, append([]string{resolved}, args...), os.Environ()) //nolint:gosec // G204: the resolved path is the artifact binrun exists to run
}
