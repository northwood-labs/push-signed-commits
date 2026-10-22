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

import "context"

type (
	// GitClient performs the local git operations the tool needs. Methods
	// return wrapped errors; none call [os.Exit].
	GitClient interface {
		// RemoteBranchExists reports whether the remote branch exists,
		// mirroring `git ls-remote --exit-code --heads <remote> <branch>`. A
		// missing branch is reported as (false, nil); only unexpected failures
		// return a non-nil error.
		RemoteBranchExists(ctx context.Context, remoteName, remoteBranchName string) (bool, error)

		// FetchRemoteBranch runs the equivalent of `git fetch <remote>
		// <branch>`.
		FetchRemoteBranch(ctx context.Context, remoteName, remoteBranchName string) error

		// RebaseOnto attempts `git rebase <remote>/<branch> <local>` and, on
		// failure, runs `git rebase --abort` before returning an error.
		RebaseOnto(ctx context.Context, opts *RebaseOptions) error

		// MergeBase returns the OID from `git merge-base <local>
		// <remote>/<branch>`.
		MergeBase(ctx context.Context, opts *MergeBaseOptions) (string, error)

		// RemoteHeadOID returns the OID from `git rev-parse <remote>/<branch>`.
		RemoteHeadOID(ctx context.Context, remoteName, remoteBranchName string) (string, error)

		// CommitsNotOnRemote returns oldest-to-newest local commit hashes from
		// `git rev-list <remote>/<branch>..<local>`.
		CommitsNotOnRemote(ctx context.Context, opts *CommitRangeOptions) ([]string, error)

		// CommitMessageBody returns the full message from `git log --format=%B
		// -n 1 <hash>`.
		CommitMessageBody(ctx context.Context, commitHash string) (string, error)

		// NameStatus returns the raw lines of `git diff --name-status <hash>^
		// <hash>`.
		NameStatus(ctx context.Context, commitHash string) ([]string, error)

		// FileContentsAtCommit returns the raw bytes of a file at a commit,
		// mirroring `git show <hash>:<path>` in binary mode.
		FileContentsAtCommit(ctx context.Context, commitHash, path string) ([]byte, error)
	}

	// RebaseOptions carries the branch selectors for RebaseOnto. It is passed
	// by pointer.
	RebaseOptions struct {
		RemoteName       string
		RemoteBranchName string
		LocalBranchName  string
	}

	// MergeBaseOptions carries the branch selectors for MergeBase. It is passed
	// by pointer.
	MergeBaseOptions struct {
		LocalBranchName  string
		RemoteName       string
		RemoteBranchName string
	}

	// CommitRangeOptions carries the branch selectors for CommitsNotOnRemote.
	// It is passed by pointer.
	CommitRangeOptions struct {
		LocalBranchName  string
		RemoteName       string
		RemoteBranchName string
	}
)
