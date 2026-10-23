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

import "errors"

var (
	// ErrInvalidBranchName indicates that a provided branch name includes a ref
	// prefix or the remote name, which must be omitted.
	ErrInvalidBranchName = errors.New("invalid branch name")

	// ErrRemoteBranchNotFound indicates that the remote branch the tool was
	// asked to push to does not exist on the remote.
	ErrRemoteBranchNotFound = errors.New("remote branch does not exist")

	// ErrRebaseAborted indicates that the rebase onto the upstream branch could
	// not complete cleanly and was aborted; the branches must be reconciled
	// manually.
	ErrRebaseAborted = errors.New(
		"rebase onto the remote branch was aborted; reconcile the branches manually",
	)

	// ErrRemoteBranchDiverged indicates that the remote branch head has
	// diverged from the local branch, so the local commits cannot be replayed
	// safely.
	ErrRemoteBranchDiverged = errors.New("remote branch has diverged from the local branch")

	// ErrGithubAPI indicates that GitHub's GraphQL API returned an unexpected
	// or error response while creating a commit.
	ErrGithubAPI = errors.New("GitHub API returned an unexpected response")

	// ErrPartialPushFailure indicates that only some commits were pushed before
	// the remote state changed, leaving the push incomplete.
	ErrPartialPushFailure = errors.New("push completed only partially")
)
