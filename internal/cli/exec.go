package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"

	"github.com/yasyf/daemonkit/artifact"
)

// execProcess replaces the current process image with the artifact at path,
// forwarding argv and env. It returns only on failure; it is a var so tests can
// exercise the retry path without actually replacing the test process.
var execProcess = func(path string, argv, env []string) error {
	// Exec'ing a resolved, digest-verified artifact is binrun's whole purpose.
	return syscall.Exec(path, argv, env) //nolint:gosec // G204: the resolved path is the artifact binrun exists to run
}

// execDescriptor resolves the descriptor at path and replaces this process with
// the pinned artifact, forwarding args as-is. On success the artifact's exit
// code becomes binrun's.
func execDescriptor(ctx context.Context, path string, args []string) error {
	if strings.HasPrefix(path, "-") {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%q is not a descriptor file; run a binrun verb as 'binrun -- VERB'", path)
		}
	}
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
	// execProcess returns only on failure. A concurrent `gc` can prune the
	// resolved entry between Resolve and the exec — Resolve drops its
	// per-artifact lock on return — so on ENOENT re-resolve once, refetching the
	// pruned entry, and retry.
	if err := execAt(resolved, args); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	resolved, err = store.Resolve(ctx, desc, resolveOptions()...)
	if err != nil {
		return err
	}
	return execAt(resolved, args)
}

func execAt(path string, args []string) error {
	return execProcess(path, append([]string{path}, args...), os.Environ())
}
