// Copyright 2026, Northwood Labs, LLC <license@northwood-labs.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient"
	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
	"go.nwlabs.dev/push-signed-commits/internal/pusher"
)

var (
	// fGithubToken holds the --github-token flag value: the GitHub personal
	// access token used to authenticate the GraphQL request.
	fGithubToken string

	// fRepoWithOwner holds the --repository-name-with-owner flag value: the
	// GitHub repository in owner/name form used in the GraphQL mutation.
	fRepoWithOwner string

	// fLocalBranch holds the --local-branch-name flag value: the local branch
	// whose commits are recreated on the remote branch.
	fLocalBranch string

	// fRemoteName holds the --remote-name flag value: the configured git remote.
	fRemoteName string

	// fRemoteBranch holds the --remote-branch-name flag value: the remote branch
	// that the local commits are recreated on.
	fRemoteBranch string

	// fLogLevel holds the --log-level flag value: the logging verbosity,
	// defaulting to WARN.
	fLogLevel string

	// rootCmd is the single-purpose root command. Its RunE wires the pipeline
	// that validates input, verifies remote state, and pushes the commits.
	rootCmd = &cobra.Command{
		Use:   "push-signed-commits",
		Short: "Recreate local commits on a remote branch as verified commits",
		Long: "push-signed-commits recreates local commits on a remote branch as " +
			"verified commits through GitHub's GraphQL createCommitOnBranch mutation.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ResolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve configuration: %w", err)
			}

			// Configure logging from the resolved level before any further
			// pipeline work, so every later stage honors the chosen verbosity.
			// The default is WARN, which suppresses informational and debug
			// output.
			ConfigureLogging(cfg.LogLevel)

			ctx := cmd.Context()
			slog.DebugContext(ctx, "Resolved configuration",
				LogKeyRemoteName, cfg.RemoteName,
				LogKeyBranchName, cfg.RemoteBranchName,
			)

			return runPipeline(ctx, cfg)
		},
	}
)

func init() { // lint:allow_init
	flags := rootCmd.Flags()

	flags.StringVar(&fGithubToken, FlagGithubToken, "", "GitHub personal access token used for GraphQL authentication")
	flags.StringVar(
		&fRepoWithOwner,
		FlagRepoWithOwner,
		"",
		"GitHub repository in owner/name form used in the GraphQL mutation",
	)
	flags.StringVar(&fLocalBranch, FlagLocalBranch, "", "local branch whose commits are recreated on the remote branch")
	flags.StringVar(&fRemoteName, FlagRemoteName, "", "configured git remote to operate against")
	flags.StringVar(&fRemoteBranch, FlagRemoteBranch, "", "remote branch that the local commits are recreated on")
	flags.StringVar(
		&fLogLevel,
		FlagLogLevel,
		DefaultLogLevel,
		"logging verbosity (for example DEBUG, INFO, WARN, ERROR)",
	)

	requiredFlags := []string{
		FlagGithubToken,
		FlagRepoWithOwner,
		FlagLocalBranch,
		FlagRemoteName,
		FlagRemoteBranch,
	}

	for _, name := range requiredFlags {
		err := rootCmd.MarkFlagRequired(name)
		if err != nil {
			panic(fmt.Sprintf("failed to mark flag %q required: %v", name, err))
		}
	}
}

// Execute builds the root context, runs the root command through fang, and
// returns any error encountered so that the entrypoint can exit with a
// non-zero status. [os.Exit] stays in main; this function only reports failure.
func Execute() error {
	ctx := context.Background()

	err := fang.Execute(ctx, rootCmd)
	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	return nil
}

// runPipeline runs the tool's full pipeline after configuration and logging
// are resolved: it validates the local and remote branch names, verifies the
// remote branch state (existence, fetch, rebase, merge base, and divergence),
// collects the local commits that are not yet on the remote branch, and
// recreates each one on the remote branch as a verified commit through the
// GraphQL API. The per-commit message build, file-change derivation, and
// chunking are performed inside [pusher.Push]. Any failure is returned as a
// wrapped error so the caller exits non-zero; [os.Exit] stays in main.
func runPipeline(ctx context.Context, cfg *Config) error {
	err := ValidateBranchName(&BranchNameInput{
		BranchName: cfg.LocalBranchName,
		BranchType: "local",
		RemoteName: cfg.RemoteName,
	})
	if err != nil {
		return fmt.Errorf("failed to validate the local branch name: %w", err)
	}

	err = ValidateBranchName(&BranchNameInput{
		BranchName: cfg.RemoteBranchName,
		BranchType: "remote",
		RemoteName: cfg.RemoteName,
	})
	if err != nil {
		return fmt.Errorf("failed to validate the remote branch name: %w", err)
	}

	git, err := gitclient.New(".")
	if err != nil {
		return fmt.Errorf("failed to open the local git repository: %w", err)
	}

	gql := githubgql.NewClient(cfg.GithubToken)

	mergeBase, err := verifyRemoteState(ctx, git, cfg)
	if err != nil {
		return fmt.Errorf("failed to verify the remote branch state: %w", err)
	}

	hashes, err := pusher.CollectCommitsNotOnRemote(ctx, git, &gitclient.CommitRangeOptions{
		LocalBranchName:  cfg.LocalBranchName,
		RemoteName:       cfg.RemoteName,
		RemoteBranchName: cfg.RemoteBranchName,
	})
	if err != nil {
		return fmt.Errorf("failed to collect the local commits not on the remote branch: %w", err)
	}

	err = pusher.Push(ctx, git, gql, &pusher.PushOptions{
		Config: &pusher.Config{
			RepositoryNameWithOwner: cfg.RepositoryNameWithOwner,
			RemoteName:              cfg.RemoteName,
			RemoteBranchName:        cfg.RemoteBranchName,
		},
		MergeBaseOID: mergeBase,
		LocalHashes:  hashes,
	})
	if err != nil {
		return fmt.Errorf("failed to recreate the local commits on the remote branch: %w", err)
	}

	return nil
}

// verifyRemoteState performs the remote-verification sequence required before
// any commit is recreated: it confirms the remote branch exists, fetches it,
// rebases the local branch onto the upstream, computes the merge base, reads
// the remote head OID, and checks for divergence. It returns the merge-base
// OID, which task 12.1 uses as the expected head for the first created commit.
//
// Boundary errors from the gitclient package are mapped to the cmd sentinels
// the requirements name: a missing branch to [ErrRemoteBranchNotFound], an
// aborted rebase to [ErrRebaseAborted], and a diverged head to
// [ErrRemoteBranchDiverged].
func verifyRemoteState(ctx context.Context, git gitclient.GitClient, cfg *Config) (string, error) {
	exists, err := git.RemoteBranchExists(ctx, cfg.RemoteName, cfg.RemoteBranchName)
	if err != nil {
		return "", fmt.Errorf("failed to check whether the remote branch exists: %w", err)
	}

	if !exists {
		return "", fmt.Errorf(
			"%w: %s/%s",
			ErrRemoteBranchNotFound,
			cfg.RemoteName,
			cfg.RemoteBranchName,
		)
	}

	err = git.FetchRemoteBranch(ctx, cfg.RemoteName, cfg.RemoteBranchName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch the remote branch: %w", err)
	}

	err = git.RebaseOnto(ctx, &gitclient.RebaseOptions{
		RemoteName:       cfg.RemoteName,
		RemoteBranchName: cfg.RemoteBranchName,
		LocalBranchName:  cfg.LocalBranchName,
	})
	if err != nil {
		if errors.Is(err, gitclient.ErrRebaseAborted) {
			return "", fmt.Errorf("%w: %w", ErrRebaseAborted, err)
		}

		return "", fmt.Errorf("failed to rebase the local branch onto the upstream: %w", err)
	}

	mergeBase, err := git.MergeBase(ctx, &gitclient.MergeBaseOptions{
		LocalBranchName:  cfg.LocalBranchName,
		RemoteName:       cfg.RemoteName,
		RemoteBranchName: cfg.RemoteBranchName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to determine the merge base of the local and remote branches: %w", err)
	}

	head, err := git.RemoteHeadOID(ctx, cfg.RemoteName, cfg.RemoteBranchName)
	if err != nil {
		return "", fmt.Errorf("failed to read the remote branch head: %w", err)
	}

	if head != mergeBase {
		return "", fmt.Errorf(
			"%w: the remote head %s does not match the merge base %s",
			ErrRemoteBranchDiverged,
			head,
			mergeBase,
		)
	}

	return mergeBase, nil
}
