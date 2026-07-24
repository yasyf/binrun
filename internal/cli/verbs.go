package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yasyf/daemonkit/artifact"
	"github.com/yasyf/daemonkit/ghrelease"
)

func resolveOptions() []artifact.Option {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return []artifact.Option{artifact.WithGitHubToken(token)}
	}
	return nil
}

func newFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch FILE",
		Short: "Materialize a descriptor's artifact without executing it (SessionStart pre-warm)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc, err := artifact.ParseFile(args[0])
			if err != nil {
				return err
			}
			store, err := artifact.DefaultStore()
			if err != nil {
				return err
			}
			return store.Fetch(cmd.Context(), desc, resolveOptions()...)
		},
	}
}

func newResolveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve FILE",
		Short: "Print the resolved local path to a descriptor's artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc, err := artifact.ParseFile(args[0])
			if err != nil {
				return err
			}
			store, err := artifact.DefaultStore()
			if err != nil {
				return err
			}
			path, err := store.Resolve(cmd.Context(), desc, resolveOptions()...)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
}

func newParseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse FILE",
		Short: "Print the normalized descriptor JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc, err := artifact.ParseFile(args[0])
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(desc, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		},
	}
}

func newLatestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "latest FILE",
		Short: "Print the latest release tag for the descriptor's repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc, err := artifact.ParseFile(args[0])
			if err != nil {
				return err
			}
			client := ghrelease.Client{BaseURL: os.Getenv("GITHUB_API_URL"), Token: os.Getenv("GITHUB_TOKEN")}
			tag, err := latestTag(cmd.Context(), client, desc)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), tag)
			return err
		},
	}
}

func newCacheDirCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cache-dir",
		Short: "Print the content cache directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := artifact.DefaultStore()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), store.CacheDir())
			return err
		},
	}
}

// latestTag reads the descriptor's github-release repository and returns its
// latest published tag. Self-update composes on top of this; resolution never
// does.
func latestTag(ctx context.Context, client ghrelease.Client, desc *artifact.Descriptor) (string, error) {
	repo, err := descriptorRepo(desc)
	if err != nil {
		return "", err
	}
	release, err := client.Latest(ctx, repo)
	if err != nil {
		return "", err
	}
	return release.Tag, nil
}

func descriptorRepo(desc *artifact.Descriptor) (string, error) {
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		return "", err
	}
	entry, ok := desc.Platforms[platform]
	if !ok || len(entry.Providers) == 0 {
		return "", fmt.Errorf("descriptor %q has no github-release provider for %s", desc.Name, platform)
	}
	return entry.Providers[0].Repo, nil
}
