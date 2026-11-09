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
	"net/url"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"
)

const (
	// testRemoteName is the arbitrary remote name used across the
	// preserved-semantics tests; the stored remote config is never consulted,
	// so the value only needs to be stable.
	testRemoteName = "origin"

	// testToken is a dummy token. The file transport ignores HTTP basic auth,
	// so any non-empty value is harmless and never leaves the machine.
	testToken = "x-token" // lint:not_a_secret

	// testBranchName is the branch that test fixtures actually create.
	testBranchName = "main"
)

// fileURL converts a local directory path into a file:// URL that go-git's
// file transport accepts, so the preserved-semantics tests drive a real remote
// without any network I/O.
func fileURL(t *testing.T, dir string) string {
	t.Helper()

	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}).String()
}

// newLocalClient returns a Client whose storer belongs to a throwaway,
// non-bare repository in a temp directory. RemoteBranchExists and
// FetchRemoteBranch only use c.repo.Storer together with the HTTPS URL they are
// handed, so this local repository is a sufficient, network-free stand-in for a
// real clone.
func newLocalClient(t *testing.T) *Client {
	t.Helper()

	repo, err := gogit.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	return &Client{repo: repo}
}

// newEmptyBareRepo initializes an empty (no commits) bare repository in a temp
// directory and returns its path. Listing such a repository reports
// transport.ErrEmptyRemoteRepository.
func newEmptyBareRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	_, err := gogit.PlainInit(dir, true)
	require.NoError(t, err)

	return dir
}

// newBareRepoWithBranch initializes a bare repository in a temp directory,
// seeds it with a single commit on the given branch, and returns its path. The
// commit guarantees the repository is non-empty so a List no longer reports
// ErrEmptyRemoteRepository and instead returns a ref list to match against.
func newBareRepoWithBranch(t *testing.T, branch string) string {
	t.Helper()

	bareDir := newEmptyBareRepo(t)

	// A bare repository cannot hold a worktree, so commit into a normal
	// repository first and then push the branch into the bare one over the
	// file transport.
	workRepo, err := gogit.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	wt, err := workRepo.Worktree()
	require.NoError(t, err)

	signature := &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Unix(0, 0).UTC(),
	}

	commitHash, err := wt.Commit("seed commit", &gogit.CommitOptions{
		Author:            signature,
		Committer:         signature,
		AllowEmptyCommits: true,
	})
	require.NoError(t, err)

	// Point the fresh commit at the requested branch name regardless of the
	// default branch PlainInit chose, so the fixture always offers exactly the
	// branch the test asked for.
	branchRef := plumbing.NewBranchReferenceName(branch)
	require.NoError(t, workRepo.Storer.SetReference(plumbing.NewHashReference(branchRef, commitHash)))

	remote, err := workRepo.CreateRemote(&config.RemoteConfig{
		Name: testRemoteName,
		URLs: []string{fileURL(t, bareDir)},
	})
	require.NoError(t, err)

	refSpec := config.RefSpec(branchRef.String() + ":" + branchRef.String())
	require.NoError(t, remote.Push(&gogit.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	}))

	return bareDir
}

// TestRemoteBranchExistsNonMatchingRefList asserts that when the remote
// repository has refs but none of them match the queried branch,
// RemoteBranchExists reports the missing branch as (false, nil) — the
// not-found answer `git ls-remote --exit-code` gives — rather than an error.
//
// Validates: Requirements 4.3.
func TestRemoteBranchExistsNonMatchingRefList(t *testing.T) {
	bareDir := newBareRepoWithBranch(t, testBranchName)
	client := newLocalClient(t)

	exists, err := client.RemoteBranchExists(context.Background(), &RemoteBranchExistsOptions{
		RemoteName:       testRemoteName,
		RemoteBranchName: "does-not-exist",
		HTTPSRemoteURL:   fileURL(t, bareDir),
		Token:            testToken,
	})

	require.NoError(t, err)
	assert.False(t, exists)
}

// TestRemoteBranchExistsEmptyRemote asserts that an empty remote repository
// (no commits, no refs) is reported as (false, nil). go-git surfaces this as
// transport.ErrEmptyRemoteRepository, which RemoteBranchExists must translate
// into the same not-found answer rather than an unexpected failure.
//
// Validates: Requirements 4.3.
func TestRemoteBranchExistsEmptyRemote(t *testing.T) {
	bareDir := newEmptyBareRepo(t)
	client := newLocalClient(t)

	exists, err := client.RemoteBranchExists(context.Background(), &RemoteBranchExistsOptions{
		RemoteName:       testRemoteName,
		RemoteBranchName: testBranchName,
		HTTPSRemoteURL:   fileURL(t, bareDir),
		Token:            testToken,
	})

	require.NoError(t, err)
	assert.False(t, exists)
}

// TestFetchRemoteBranchAlreadyUpToDate asserts that a fetch which brings in no
// new objects is treated as success. The first fetch populates the
// remote-tracking ref; the second fetch has nothing to do, so go-git returns
// gogit.NoErrAlreadyUpToDate, which FetchRemoteBranch must swallow and report
// as a nil error.
//
// Validates: Requirements 4.3.
func TestFetchRemoteBranchAlreadyUpToDate(t *testing.T) {
	bareDir := newBareRepoWithBranch(t, testBranchName)
	client := newLocalClient(t)

	opts := &FetchRemoteBranchOptions{
		RemoteName:       testRemoteName,
		RemoteBranchName: testBranchName,
		HTTPSRemoteURL:   fileURL(t, bareDir),
		Token:            testToken,
	}

	// The first fetch pulls the branch down; it must succeed so the second
	// fetch can exercise the already-up-to-date path.
	err := client.FetchRemoteBranch(context.Background(), opts)
	require.NoError(t, err)

	// The second fetch finds nothing new. go-git returns NoErrAlreadyUpToDate,
	// which must be treated as success.
	err = client.FetchRemoteBranch(context.Background(), opts)
	assert.NoError(t, err)
}

// TestSplitNameStatusLines verifies the line-splitting transformation that
// NameStatus applies to raw `git diff --name-status` output. The raw lines must
// be returned unaltered (tabs and rename scores preserved) so downstream
// parsing can split each line on tab characters, and the trailing empty element
// produced by the final newline must be dropped to match Python's splitlines().
func TestSplitNameStatusLines(t *testing.T) {
	testCases := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name:     "single addition with trailing newline",
			output:   "A\tnewfile.txt\n",
			expected: []string{"A\tnewfile.txt"},
		},
		{
			name:     "single modification with trailing newline",
			output:   "M\tmod.txt\n",
			expected: []string{"M\tmod.txt"},
		},
		{
			name:     "rename with similarity score preserves both filenames",
			output:   "R100\told.txt\tnew.txt\n",
			expected: []string{"R100\told.txt\tnew.txt"},
		},
		{
			name:     "deletion with trailing newline",
			output:   "D\tgone.txt\n",
			expected: []string{"D\tgone.txt"},
		},
		{
			name:   "multiple mixed lines with trailing newline drops no content",
			output: "A\tnewfile.txt\nM\tmod.txt\nR100\told.txt\tnew.txt\nD\tgone.txt\n",
			expected: []string{
				"A\tnewfile.txt",
				"M\tmod.txt",
				"R100\told.txt\tnew.txt",
				"D\tgone.txt",
			},
		},
		{
			name:   "no trailing newline keeps every line",
			output: "A\tfoo\nM\tbar",
			expected: []string{
				"A\tfoo",
				"M\tbar",
			},
		},
		{
			name:     "empty output yields an empty slice",
			output:   "",
			expected: []string(nil),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lines := splitNameStatusLines(testCase.output)

			// The empty-output case must drop the trailing empty element down
			// to a zero-length slice so a repo with no changes yields no
			// spurious entries for downstream parsing.
			require.Len(t, lines, len(testCase.expected))

			// The lines must be returned exactly as produced by git, in order,
			// with tabs and rename scores intact so task 7's parser can split
			// on tab. Compare element by element so a zero-length result
			// matches whether the slice is nil or empty.
			for i, want := range testCase.expected {
				assert.Equal(t, want, lines[i])
			}
		})
	}
}
