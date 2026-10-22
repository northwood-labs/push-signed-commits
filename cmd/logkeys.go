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

// Structured-logging key constants. All keys are snake_case so that log output
// is consistent across the tool and satisfies the sloglint key-naming rule. The
// forbidden slog keys (time, level, msg, source) are never used here.
const (
	// LogKeyCommitCount is the number of local commits not yet on the remote
	// branch.
	LogKeyCommitCount = "commit_count"

	// LogKeyCommitHashes is the list of local commit hashes to be recreated on
	// the remote branch.
	LogKeyCommitHashes = "commit_hashes"

	// LogKeyCommitHash is a single local commit hash currently being processed.
	LogKeyCommitHash = "commit_hash"

	// LogKeyCommitOID is the OID of a commit created on the remote branch.
	LogKeyCommitOID = "commit_oid"

	// LogKeyBranchName is the name of the remote branch being operated on.
	LogKeyBranchName = "branch_name"

	// LogKeyRemoteName is the name of the configured git remote.
	LogKeyRemoteName = "remote_name"

	// LogKeyNameStatusOutput is the raw output of git diff --name-status for a
	// single commit.
	LogKeyNameStatusOutput = "name_status_output"

	// LogKeyChangeStatus is the detected status code for a single file change.
	LogKeyChangeStatus = "change_status"

	// LogKeyFileNames is the set of filenames detected for a single file change.
	LogKeyFileNames = "file_names"

	// LogKeyPayloadMegabytes is the approximate payload size in megabytes when a
	// commit must be split.
	LogKeyPayloadMegabytes = "payload_megabytes"

	// LogKeyChunkCount is the number of chunks produced for a split commit.
	LogKeyChunkCount = "chunk_count"

	// LogKeyCreatedCount is the number of commits created on the remote branch.
	LogKeyCreatedCount = "created_count"

	// LogKeyErrorContent is the content of a GraphQL error response from GitHub.
	LogKeyErrorContent = "error_content"
)
