package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yasyf/binrun/internal/livedir"
	"github.com/yasyf/daemonkit/artifact"
)

// listProcesses enumerates this user's running processes; a var so tests can
// stand a fixed process table in for the host's.
var listProcesses livedir.Lister = livedir.List

func newGCCmd() *cobra.Command {
	var keep int
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Prune the content cache and the tool store, keeping the newest N materializations each",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if keep < 0 {
				return fmt.Errorf("--keep must be >= 0, got %d", keep)
			}
			store, err := artifact.DefaultStore()
			if err != nil {
				return err
			}
			if err := pruneCache(cmd, store, keep); err != nil {
				return err
			}
			return pruneTools(cmd, store, keep)
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 2, "number of newest materializations to keep per artifact name and per tool")
	return cmd
}

func pruneCache(cmd *cobra.Command, store artifact.Store, keep int) error {
	entries, err := store.CacheEntries()
	if err != nil {
		return err
	}
	group := func(e artifact.CacheEntry) string { return e.Name }
	order := func(e artifact.CacheEntry) (time.Time, string) { return e.FetchedAt, e.Digest }
	for _, entry := range toPrune(entries, keep, group, order) {
		if err := store.RemoveCacheEntry(entry); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", entry.Digest, entry.Name); err != nil {
			return err
		}
	}
	return nil
}

func pruneTools(cmd *cobra.Command, store artifact.Store, keep int) error {
	entries, err := store.ToolEntries()
	if err != nil {
		return err
	}
	pruned, pruneErr := pruneToolEnvs(store, entries, keep, "")
	for _, entry := range pruned.Removed {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", entry.Dist, entry.Version); err != nil {
			return err
		}
	}
	for _, entry := range pruned.Skipped {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "in use, kept: %s %s\n", entry.Dist, entry.Version); err != nil {
			return err
		}
	}
	return pruneErr
}

type toolPrune struct {
	Removed []artifact.ToolEntry
	Skipped []artifact.ToolEntry
}

// pruneToolEnvs removes the environments in entries past the newest keep per
// dist, skipping any a live process is running out of — an env deleted under a
// running python worker breaks the imports it has not reached yet — and any
// holding running, the path its caller is about to exec.
func pruneToolEnvs(store artifact.Store, entries []artifact.ToolEntry, keep int, running string) (toolPrune, error) {
	group := func(e artifact.ToolEntry) string { return e.Dist }
	order := func(e artifact.ToolEntry) (time.Time, string) { return e.InstalledAt, e.Version }
	candidates := toPrune(entries, keep, group, order)
	dirs := make([]string, 0, len(candidates))
	for _, entry := range candidates {
		dirs = append(dirs, entry.Dir)
	}
	live, err := livedir.InUse(listProcesses, dirs)
	if err != nil {
		return toolPrune{}, err
	}
	for _, dir := range dirs {
		if strings.HasPrefix(running, dir+string(os.PathSeparator)) {
			live[dir] = true
		}
	}
	// An install that finishes while the process table is being read re-marks
	// its directory, and its workers are about to run out of it.
	settled, err := installTimes(store)
	if err != nil {
		return toolPrune{}, err
	}
	var pruned toolPrune
	for _, entry := range candidates {
		if !settled[entry.Dir].Equal(entry.InstalledAt) {
			continue
		}
		if live[entry.Dir] {
			pruned.Skipped = append(pruned.Skipped, entry)
			continue
		}
		if err := store.RemoveToolEntry(entry); err != nil {
			return pruned, err
		}
		pruned.Removed = append(pruned.Removed, entry)
	}
	return pruned, nil
}

func installTimes(store artifact.Store) (map[string]time.Time, error) {
	entries, err := store.ToolEntries()
	if err != nil {
		return nil, err
	}
	times := make(map[string]time.Time, len(entries))
	for _, entry := range entries {
		times[entry.Dir] = entry.InstalledAt
	}
	return times, nil
}

// toPrune buckets entries by group, orders each bucket newest-first by the
// timestamp order reports (its identity string breaking ties for determinism),
// and returns everything past the newest keep. Entries whose provenance could
// not be read carry a zero timestamp, so they share a bucket and sort oldest and
// are reclaimed rather than orbited forever.
func toPrune[T any](entries []T, keep int, group func(T) string, order func(T) (time.Time, string)) []T {
	byGroup := map[string][]T{}
	for _, entry := range entries {
		byGroup[group(entry)] = append(byGroup[group(entry)], entry)
	}
	var prune []T
	for _, bucket := range byGroup {
		slices.SortFunc(bucket, func(a, b T) int {
			at, aID := order(a)
			bt, bID := order(b)
			if !at.Equal(bt) {
				return bt.Compare(at)
			}
			return strings.Compare(bID, aID)
		})
		if len(bucket) > keep {
			prune = append(prune, bucket[keep:]...)
		}
	}
	slices.SortFunc(prune, func(a, b T) int {
		_, aID := order(a)
		_, bID := order(b)
		return strings.Compare(aID, bID)
	})
	return prune
}
