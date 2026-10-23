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

package githubgql

// Structured-logging key constants for the githubgql package. All keys are
// snake_case so that log output is consistent across the tool and satisfies the
// sloglint key-naming rule. The forbidden slog keys (time, level, msg, source)
// are never used here.
const (
	// LogKeyCommitOID is the OID of a commit created on the remote branch.
	LogKeyCommitOID = "commit_oid"

	// LogKeyBranchName is the name of the remote branch the commit was created
	// on.
	LogKeyBranchName = "branch_name"

	// LogKeyErrorContent is the content of a GraphQL error response from
	// GitHub.
	LogKeyErrorContent = "error_content"
)
