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
	"encoding/base64"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient/gitclientfake"
	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
)

// fakeContents is the raw byte content the fake returns for every
// FileContentsAtCommit call; its base64 encoding is the expected contents on
// every FileAddition produced from a read.
var fakeContents = []byte("hello")

func TestBuildFileChanges(t *testing.T) {
	const commitHash = "abc123"

	// encoded is base64.StdEncoding.EncodeToString([]byte("hello")), i.e.
	// "aGVsbG8=". It is computed rather than hard-coded so the expectation
	// tracks the fake's configured contents.
	encoded := base64.StdEncoding.EncodeToString(fakeContents)

	testCases := []struct {
		name string
		// lines are the raw name-status lines the fake returns from
		// NameStatus for this case.
		lines []string
		// expected is the FileChanges that BuildFileChanges should derive.
		expected githubgql.FileChanges
		// expectedContentsReads is the ordered list of paths that
		// FileContentsAtCommit should be invoked with; empty when no read is
		// expected (e.g. a pure deletion).
		expectedContentsReads []string
	}{
		{
			name:  "added file produces one addition read from its only name",
			lines: []string{"A\tnew.txt"},
			expected: githubgql.FileChanges{
				Deletions: nil,
				Additions: []githubgql.FileAddition{
					{Path: "new.txt", Contents: encoded},
				},
			},
			expectedContentsReads: []string{"new.txt"},
		},
		{
			name:  "modified file produces one addition read from its only name",
			lines: []string{"M\tchanged.txt"},
			expected: githubgql.FileChanges{
				Deletions: nil,
				Additions: []githubgql.FileAddition{
					{Path: "changed.txt", Contents: encoded},
				},
			},
			expectedContentsReads: []string{"changed.txt"},
		},
		{
			name:  "scored rename deletes the old name and adds the new name",
			lines: []string{"R100\told.txt\tnew.txt"},
			expected: githubgql.FileChanges{
				Deletions: []githubgql.FileDeletion{
					{Path: "old.txt"},
				},
				Additions: []githubgql.FileAddition{
					{Path: "new.txt", Contents: encoded},
				},
			},
			// The rename reads from the new name, never the old name.
			expectedContentsReads: []string{"new.txt"},
		},
		{
			name:  "unscored rename deletes the old name and adds the new name",
			lines: []string{"R\told.txt\tnew.txt"},
			expected: githubgql.FileChanges{
				Deletions: []githubgql.FileDeletion{
					{Path: "old.txt"},
				},
				Additions: []githubgql.FileAddition{
					{Path: "new.txt", Contents: encoded},
				},
			},
			expectedContentsReads: []string{"new.txt"},
		},
		{
			name:  "deleted file produces one deletion and reads no contents",
			lines: []string{"D\tgone.txt"},
			expected: githubgql.FileChanges{
				Deletions: []githubgql.FileDeletion{
					{Path: "gone.txt"},
				},
				Additions: nil,
			},
			expectedContentsReads: nil,
		},
		{
			name: "last-vs-first selection: path is the last name, contents read from the first",
			// A crafted two-field addition line proves the divergence: the
			// FileAddition path is the last filename ("last") while the
			// contents are read from the first filename ("first").
			lines: []string{"A\tfirst\tlast"},
			expected: githubgql.FileChanges{
				Deletions: nil,
				Additions: []githubgql.FileAddition{
					{Path: "last", Contents: encoded},
				},
			},
			expectedContentsReads: []string{"first"},
		},
		{
			name: "mixed changes accumulate into combined additions and deletions",
			lines: []string{
				"A\tadded.txt",
				"M\tmodified.txt",
				"R100\trenamed-old.txt\trenamed-new.txt",
				"D\tdeleted.txt",
			},
			expected: githubgql.FileChanges{
				Deletions: []githubgql.FileDeletion{
					{Path: "renamed-old.txt"},
					{Path: "deleted.txt"},
				},
				Additions: []githubgql.FileAddition{
					{Path: "added.txt", Contents: encoded},
					{Path: "modified.txt", Contents: encoded},
					{Path: "renamed-new.txt", Contents: encoded},
				},
			},
			expectedContentsReads: []string{"added.txt", "modified.txt", "renamed-new.txt"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &gitclientfake.Fake{
				NameStatusResult: gitclientfake.NameStatusResult{
					Lines: testCase.lines,
				},
				FileContentsAtCommitResult: gitclientfake.FileContentsAtCommitResult{
					Contents: fakeContents,
				},
			}

			changes, err := BuildFileChanges(context.Background(), fake, commitHash)
			require.NoError(t, err)

			assert.Equal(t, testCase.expected, changes)

			// NameStatus is always consulted exactly once for the commit.
			require.Len(t, fake.NameStatusCalls, 1)
			assert.Equal(t, commitHash, fake.NameStatusCalls[0].CommitHash)

			// FileContentsAtCommit must be invoked once per expected read, in
			// order, with the commit hash and the path dictated by the
			// last-vs-first and rename-reads-new-name rules.
			require.Len(t, fake.FileContentsAtCommitCalls, len(testCase.expectedContentsReads))

			for i, wantPath := range testCase.expectedContentsReads {
				assert.Equal(t, commitHash, fake.FileContentsAtCommitCalls[i].CommitHash)
				assert.Equal(t, wantPath, fake.FileContentsAtCommitCalls[i].Path)
			}
		})
	}
}
