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

package gitclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

var (
	// Compile-time assertion that Client satisfies the GitClient interface.
	_ GitClient = (*Client)(nil)

	// ErrOpenRepository indicates the local git repository could not be opened
	// from the current working directory.
	ErrOpenRepository = errors.New("failed to open local git repository")

	// ErrRemoteBranchList indicates the remote branch listing (the equivalent
	// of `git ls-remote --heads`) could not be completed.
	ErrRemoteBranchList = errors.New("failed to list remote branches")

	// ErrRemoteFetch indicates fetching the remote branch failed.
	ErrRemoteFetch = errors.New("failed to fetch remote branch")

	// ErrMergeBase indicates the common ancestor of the local and remote
	// branches (the equivalent of `git merge-base`) could not be determined.
	ErrMergeBase = errors.New("failed to determine the merge base of the local and remote branches")

	// ErrNoMergeBase indicates the local and remote branches share no common
	// ancestor, so no merge base exists.
	ErrNoMergeBase = errors.New("the local and remote branches share no common ancestor")

	// ErrResolveRevision indicates a revision (such as the remote-tracking
	// branch) could not be resolved to a commit, mirroring a failed `git
	// rev-parse`.
	ErrResolveRevision = errors.New("failed to resolve the requested git revision")

	// ErrCommitRange indicates the list of local commits not present on the
	// remote branch (the equivalent of `git rev-list
	// <remote>/<branch>..<local>`) could not be computed.
	ErrCommitRange = errors.New("failed to list the local commits not present on the remote branch")

	// ErrRebaseAborted indicates the rebase onto the upstream branch reported
	// conflicts or could not complete cleanly. The rebase was aborted to
	// restore the local branch to its pre-rebase state; the operator must
	// reconcile the branches manually before retrying.
	ErrRebaseAborted = errors.New(
		"rebase onto the upstream branch was aborted; reconcile the branches manually and retry",
	)

	// ErrWorktree indicates the repository working tree could not be resolved,
	// which is required to run git subprocesses from the correct directory.
	ErrWorktree = errors.New("failed to resolve the repository working tree")

	// ErrCommitMessage indicates the commit message for a given commit (the
	// equivalent of `git log --format=%B -n 1 <hash>`) could not be retrieved.
	ErrCommitMessage = errors.New("failed to read the commit message for the requested commit")

	// ErrNameStatus indicates the per-commit name-status diff (the equivalent
	// of `git diff --name-status <hash>^ <hash>`) could not be produced.
	ErrNameStatus = errors.New("failed to list the file changes for the requested commit")

	// ErrFileContents indicates the contents of a file at a given commit (the
	// equivalent of `git show <hash>:<path>`) could not be read.
	ErrFileContents = errors.New("failed to read the file contents at the requested commit")
)

type (
	// Client is the production implementation of GitClient. It wraps a go-git
	// repository and falls back to os/exec only where go-git lacks the required
	// capability. Construct it with New.
	Client struct {
		repo *gogit.Repository
	}
)

// New opens the git repository rooted at or above the given path and returns a
// Client bound to it. The repository is discovered by walking up from path, so
// callers may pass any path inside a working tree (typically "."). It returns a
// wrapped error if the repository cannot be opened.
func New(path string) (*Client, error) {
	repo, err := gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOpenRepository, err)
	}

	return &Client{repo: repo}, nil
}

// RemoteBranchExists reports whether the remote branch exists, mirroring `git
// ls-remote --exit-code --heads <remote> <branch>`. It lists the remote's
// references and reports whether refs/heads/<remoteBranchName> is present. A
// missing branch (an empty match, equivalent to the `--exit-code` non-zero
// result) is reported as (false, nil); only unexpected failures such as a
// transport error return a non-nil error.
func (c *Client) RemoteBranchExists(ctx context.Context, remoteName, remoteBranchName string) (bool, error) {
	remote, err := c.repo.Remote(remoteName)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrRemoteBranchList, err)
	}

	refs, err := remote.ListContext(ctx, &gogit.ListOptions{})
	if err != nil {
		// An empty remote repository has no branches, which is the same
		// not-found answer `git ls-remote --exit-code` reports, so it is not an
		// unexpected failure.
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return false, nil
		}

		return false, fmt.Errorf("%w: %w", ErrRemoteBranchList, err)
	}

	target := plumbing.NewBranchReferenceName(remoteBranchName)
	for _, ref := range refs {
		if ref.Name() == target {
			return true, nil
		}
	}

	return false, nil
}

// FetchRemoteBranch runs the equivalent of `git fetch <remote> <branch>`,
// pulling the latest remote branch state into the local repository without
// integrating it. A go-git "already up-to-date" result is treated as success.
func (c *Client) FetchRemoteBranch(
	ctx context.Context,
	remoteName, remoteBranchName string,
) error {
	refSpec := config.RefSpec(fmt.Sprintf(
		"+refs/heads/%s:refs/remotes/%s/%s",
		remoteBranchName,
		remoteName,
		remoteBranchName,
	))

	err := c.repo.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{refSpec},
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("%w: %w", ErrRemoteFetch, err)
	}

	return nil
}

// RebaseOnto replays the local branch onto the latest upstream by running `git
// rebase <remote>/<branch> <local>`. go-git provides no rebase operation, so
// this shells out through os/exec, building the argument vector from the
// discrete validated branch selectors rather than a shell string (gosec G204).
//
// If the rebase reports conflicts or cannot complete cleanly, it runs `git
// rebase --abort` to restore the pre-rebase state and returns an error wrapping
// ErrRebaseAborted instructing the operator to reconcile the branches manually.
// Conflicts are never auto-resolved.
func (c *Client) RebaseOnto(ctx context.Context, opts *RebaseOptions) error {
	wt, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWorktree, err)
	}

	worktree := wt.Filesystem.Root()
	upstream := opts.RemoteName + "/" + opts.RemoteBranchName

	rebaseCmd := exec.CommandContext( // lint:allow_possible_insecure
		ctx,
		"git",
		"rebase",
		upstream,
		opts.LocalBranchName,
	)

	rebaseCmd.Dir = worktree

	output, rebaseErr := rebaseCmd.CombinedOutput()
	if rebaseErr == nil {
		return nil
	}

	// The rebase failed. Abort it to restore the local branch to its pre-rebase
	// state before surfacing the error. Conflicts are never auto-resolved.
	abortDetail := ""

	abortCmd := exec.CommandContext(ctx, "git", "rebase", "--abort") // lint:allow_possible_insecure

	abortCmd.Dir = worktree

	abortOutput, abortErr := abortCmd.CombinedOutput()
	if abortErr != nil {
		abortDetail = "; the subsequent abort also failed: " + strings.TrimSpace(string(abortOutput))
	}

	return fmt.Errorf("%w: %s%s", ErrRebaseAborted, strings.TrimSpace(string(output)), abortDetail)
}

// MergeBase returns the OID of the best common ancestor of the local branch and
// the remote-tracking branch, mirroring `git merge-base <local>
// <remote>/<branch>`. It resolves both selectors to commits and asks go-git for
// their merge base, returning the first (best) result's hash. An absent common
// ancestor returns an error wrapping ErrNoMergeBase.
func (c *Client) MergeBase(_ context.Context, opts *MergeBaseOptions) (string, error) {
	localCommit, err := c.resolveCommit(opts.LocalBranchName)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrMergeBase, err)
	}

	remoteRevision := opts.RemoteName + "/" + opts.RemoteBranchName

	remoteCommit, err := c.resolveCommit(remoteRevision)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrMergeBase, err)
	}

	bases, err := localCommit.MergeBase(remoteCommit)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrMergeBase, err)
	}

	if len(bases) == 0 {
		return "", ErrNoMergeBase
	}

	return bases[0].Hash.String(), nil
}

// RemoteHeadOID returns the OID at the tip of the remote-tracking branch,
// mirroring `git rev-parse <remote>/<branch>`. go-git's ResolveRevision
// understands the `<remote>/<branch>` shorthand and resolves it against
// refs/remotes/<remote>/<branch>.
func (c *Client) RemoteHeadOID(_ context.Context, remoteName, remoteBranchName string) (string, error) {
	revision := plumbing.Revision(remoteName + "/" + remoteBranchName)

	hash, err := c.repo.ResolveRevision(revision)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrResolveRevision, err)
	}

	return hash.String(), nil
}

// CommitsNotOnRemote returns the hashes of commits reachable from the local
// branch but not from the remote-tracking branch, oldest-to-newest, mirroring
// `git rev-list <remote>/<branch>..<local>` with its default newest-first order
// reversed. It walks the log from the local head, excluding every commit
// reachable from the remote head.
func (c *Client) CommitsNotOnRemote(_ context.Context, opts *CommitRangeOptions) ([]string, error) {
	localCommit, err := c.resolveCommit(opts.LocalBranchName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommitRange, err)
	}

	remoteRevision := opts.RemoteName + "/" + opts.RemoteBranchName

	remoteCommit, err := c.resolveCommit(remoteRevision)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommitRange, err)
	}

	excluded, err := reachableSet(remoteCommit)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommitRange, err)
	}

	newestFirst := []string(nil)

	walker := object.NewCommitPreorderIter(localCommit, nil, nil)
	err = walker.ForEach(func(commit *object.Commit) error {
		if _, found := excluded[commit.Hash]; found {
			// This commit (and its ancestors) are already on the remote, so
			// stop descending this line of history.
			return storer.ErrStop
		}

		newestFirst = append(newestFirst, commit.Hash.String())

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommitRange, err)
	}

	// `git rev-list` emits newest-first; the tool wants oldest-to-newest.
	slices.Reverse(newestFirst)

	return newestFirst, nil
}

// CommitMessageBody returns the full, raw commit message for the given commit,
// mirroring `git log --format=%B -n 1 <hash>`. go-git's CommitObject.Message is
// the complete message (headline and body), equivalent to the `%B` format
// placeholder. The message is returned untrimmed; the commit-message builder is
// responsible for trimming surrounding whitespace.
func (c *Client) CommitMessageBody(_ context.Context, commitHash string) (string, error) {
	commit, err := c.repo.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCommitMessage, err)
	}

	return commit.Message, nil
}

// NameStatus returns the raw lines of `git diff --name-status <hash>^ <hash>`.
// go-git cannot reproduce git's rename scoring (for example `R100 old new`), so
// this shells out through os/exec, building the argument vector from the
// discrete commit hash rather than a shell string (gosec G204). The raw output
// is split into lines by splitNameStatusLines, which returns the lines
// unaltered for downstream parsing, matching Python's splitlines(): a trailing
// empty element from the final newline is dropped. A non-zero exit returns an
// error wrapping ErrNameStatus.
func (c *Client) NameStatus(ctx context.Context, commitHash string) ([]string, error) {
	wt, err := c.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorktree, err)
	}

	diffCmd := exec.CommandContext( // lint:allow_possible_insecure
		ctx,
		"git",
		"diff",
		"--name-status",
		commitHash+"^",
		commitHash,
	)

	diffCmd.Dir = wt.Filesystem.Root()

	output, diffErr := diffCmd.Output()
	if diffErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrNameStatus, diffErr)
	}

	return splitNameStatusLines(string(output)), nil
}

// splitNameStatusLines splits raw `git diff --name-status` output into its
// constituent lines without altering them, so downstream parsing can split
// each line on tab characters. It matches Python's splitlines(): the lines are
// returned unaltered and a trailing empty element produced by the final
// newline is dropped. Empty output yields an empty slice.
func splitNameStatusLines(output string) []string {
	lines := strings.Split(output, "\n")

	// Drop a trailing empty element produced by the final newline, matching
	// Python's splitlines() which does not emit a trailing empty line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// FileContentsAtCommit returns the raw bytes of the file at path as of the
// given commit, mirroring `git show <hash>:<path>` in binary mode. It resolves
// the commit, reads the blob from the commit tree, and returns the bytes with
// no text-mode decoding; the caller base64-encodes them.
func (c *Client) FileContentsAtCommit(_ context.Context, commitHash, path string) ([]byte, error) {
	commit, err := c.repo.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFileContents, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFileContents, err)
	}

	file, err := tree.File(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFileContents, err)
	}

	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFileContents, err)
	}

	contents, err := io.ReadAll(reader)
	if err != nil {
		closeErr := reader.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrFileContents, errors.Join(err, closeErr))
		}

		return nil, fmt.Errorf("%w: %w", ErrFileContents, err)
	}

	closeErr := reader.Close()
	if closeErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrFileContents, closeErr)
	}

	return contents, nil
}

// resolveCommit resolves a revision string (a branch name or a
// `<remote>/<branch>` shorthand) to its commit object.
func (c *Client) resolveCommit(revision string) (*object.Commit, error) {
	hash, err := c.repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResolveRevision, err)
	}

	commit, err := c.repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResolveRevision, err)
	}

	return commit, nil
}

// reachableSet returns the set of hashes reachable from the given commit,
// walking its full ancestry. It is used to exclude commits that already exist
// on the remote branch.
func reachableSet(start *object.Commit) (map[plumbing.Hash]struct{}, error) {
	reachable := make(map[plumbing.Hash]struct{})

	walker := object.NewCommitPreorderIter(start, nil, nil)

	err := walker.ForEach(func(commit *object.Commit) error {
		reachable[commit.Hash] = struct{}{}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResolveRevision, err)
	}

	return reachable, nil
}
