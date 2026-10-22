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

package pusher

import (
	"context"
	"fmt"
	"log/slog"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient"
)

const (
	// logKeyCommitCount is the number of local commits found that are not yet
	// on the remote branch. It mirrors the cmd package's log key but is defined
	// locally because the cmd package is not importable here.
	logKeyCommitCount = "commit_count"

	// logKeyCommitHashes is the oldest-to-newest list of local commit hashes to
	// be recreated on the remote branch. It mirrors the cmd package's log key
	// but is defined locally because the cmd package is not importable here.
	logKeyCommitHashes = "commit_hashes"
)

// CollectCommitsNotOnRemote returns the oldest-to-newest list of local commit
// hashes that are not yet present on the remote branch by delegating to the
// GitClient's CommitsNotOnRemote, which runs the equivalent of
// `git rev-list <remote>/<branch>..<local>` and reverses the result.
//
// The count of commits found is logged at info level and the full list of
// hashes is logged at debug level, mirroring the Python tool's
// get_local_commits_not_on_remote behavior.
func CollectCommitsNotOnRemote(
	ctx context.Context,
	git gitclient.GitClient,
	opts *gitclient.CommitRangeOptions,
) ([]string, error) {
	hashes, err := git.CommitsNotOnRemote(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to collect the local commits not on the remote branch: %w",
			err,
		)
	}

	slog.InfoContext(
		ctx,
		"Found local commits that are not on the remote branch",
		logKeyCommitCount, len(hashes),
	)

	slog.DebugContext(
		ctx,
		"Listing the local commit hashes to be created on the remote branch",
		logKeyCommitHashes, hashes,
	)

	return hashes, nil
}
