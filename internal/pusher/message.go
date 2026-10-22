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
	"strings"

	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

// BuildCommitMessage constructs the CommitMessage for a recreated commit from
// the full local commit message body and the source commit hash. The
// messageBody is the already-fetched output of CommitMessageBody (equivalent
// to `git log --format=%B -n 1 <hash>`); surrounding whitespace is trimmed
// here per Requirement 7.1.
//
// The headline is the first line of the trimmed message, the body is the
// remaining lines joined by newline characters, and a two-newline provenance
// line naming the source commit hash is appended to the body so the remote
// history remains traceable back to the local commit.
func BuildCommitMessage(messageBody, commitHash string) githubgql.CommitMessage {
	lines := strings.Split(strings.TrimSpace(messageBody), "\n")

	headline := lines[0]
	body := strings.Join(lines[1:], "\n")

	body += "\n\nThis commit was created from the local commit with hash " + commitHash + "."

	return githubgql.CommitMessage{
		Headline: headline,
		Body:     body,
	}
}
