package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yasyf/daemonkit/artifact"
)

func newGCCmd() *cobra.Command {
	var keep int
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Prune the content cache, keeping the newest N materializations per artifact name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if keep < 0 {
				return fmt.Errorf("--keep must be >= 0, got %d", keep)
			}
			store, err := artifact.DefaultStore()
			if err != nil {
				return err
			}
			entries, err := store.CacheEntries()
			if err != nil {
				return err
			}
			for _, entry := range toPrune(entries, keep) {
				if err := store.RemoveCacheEntry(entry); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", entry.Digest, entry.Name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 2, "number of newest materializations to keep per artifact name")
	return cmd
}

// toPrune groups entries by artifact name, orders each group newest-first by
// FetchedAt (digest breaking ties for determinism), and returns everything past
// the newest keep. Entries with no meta.json share the empty-name group and are
// pruned the same way, so a damaged entry is reclaimed rather than orbited.
func toPrune(entries []artifact.CacheEntry, keep int) []artifact.CacheEntry {
	byName := map[string][]artifact.CacheEntry{}
	for _, entry := range entries {
		byName[entry.Name] = append(byName[entry.Name], entry)
	}
	var prune []artifact.CacheEntry
	for _, group := range byName {
		slices.SortFunc(group, func(a, b artifact.CacheEntry) int {
			if !a.FetchedAt.Equal(b.FetchedAt) {
				return b.FetchedAt.Compare(a.FetchedAt)
			}
			return strings.Compare(b.Digest, a.Digest)
		})
		if len(group) > keep {
			prune = append(prune, group[keep:]...)
		}
	}
	slices.SortFunc(prune, func(a, b artifact.CacheEntry) int { return strings.Compare(a.Digest, b.Digest) })
	return prune
}
