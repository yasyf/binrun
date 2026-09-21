package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"syscall"

	"github.com/yasyf/binrun/internal/cwdguard"
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
			// A flag-shaped path that simply does not exist is a routing mistake;
			// any other stat failure (EACCES, ENOTDIR, …) keeps its true cause.
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%q is not a descriptor file; run a binrun verb as 'binrun -- VERB'", path)
			}
			return err
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
	resolved, err := resolveWithRetention(ctx, store, desc)
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

// toolRetention is how many of a python tool's environments survive the install
// of a new one. The installer owns retention because an install is the only
// moment the tool store grows.
const toolRetention = 3

// resolveWithRetention materializes desc and, when that installed a python-tool
// environment the store did not already hold, prunes the dist's older ones.
func resolveWithRetention(ctx context.Context, store artifact.Store, desc *artifact.Descriptor) (string, error) {
	reclaim := snapshotToolStore(store, desc)
	resolved, err := store.Resolve(ctx, desc, resolveOptions()...)
	if err != nil {
		return "", err
	}
	reclaim.run(store, resolved)
	return resolved, nil
}

// toolReclaim is the tool store as it stood before a resolve, so the reclaim
// after it can tell a fresh install from a warm hit. Its zero value reclaims
// nothing.
type toolReclaim struct {
	dist   string
	before map[string]bool
}

func snapshotToolStore(store artifact.Store, desc *artifact.Descriptor) toolReclaim {
	if desc.Kind != artifact.PythonTool {
		return toolReclaim{}
	}
	entries, err := store.ToolEntries()
	if err != nil {
		slog.Warn("binrun: tool store unreadable, skipping reclaim", "error", err)
		return toolReclaim{}
	}
	before := make(map[string]bool, len(entries))
	for _, entry := range entries {
		before[entry.Dir] = true
	}
	return toolReclaim{dist: desc.Tool.Dist, before: before}
}

// run prunes the dist's environments past toolRetention, but only once a
// resolve has materialized one the snapshot did not hold. The caller already
// holds resolved by then, so a failure here is logged and the caller proceeds.
func (r toolReclaim) run(store artifact.Store, resolved string) {
	if r.dist == "" {
		return
	}
	entries, err := store.ToolEntries()
	if err != nil {
		slog.Warn("binrun: tool store unreadable, skipping reclaim", "dist", r.dist, "error", err)
		return
	}
	siblings := make([]artifact.ToolEntry, 0, len(entries))
	fresh := false
	for _, entry := range entries {
		if entry.Dist != r.dist {
			continue
		}
		fresh = fresh || !r.before[entry.Dir]
		siblings = append(siblings, entry)
	}
	if !fresh {
		return
	}
	pruned, err := pruneToolEnvs(store, siblings, toolRetention, resolved)
	if len(pruned.Removed) > 0 || len(pruned.Skipped) > 0 {
		slog.Info("binrun: reclaimed tool environments", "dist", r.dist,
			"removed", versionsOf(pruned.Removed), "kept_in_use", versionsOf(pruned.Skipped))
	}
	if err != nil {
		slog.Warn("binrun: reclaim tool environments", "dist", r.dist, "error", err)
	}
}

func versionsOf(entries []artifact.ToolEntry) []string {
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		versions = append(versions, entry.Version)
	}
	return versions
}

func execAt(path string, args []string) error {
	if err := cwdguard.Return(); err != nil {
		return err
	}
	return execProcess(path, append([]string{path}, args...), os.Environ())
}
