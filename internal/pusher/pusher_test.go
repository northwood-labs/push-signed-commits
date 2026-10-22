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
	"errors"
	"fmt"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient/gitclientfake"
	"go.nwlabs.dev/push-signed-commits/internal/githubgql"
	"go.nwlabs.dev/push-signed-commits/internal/githubgql/githubgqlfake"
)

const (
	// testMergeBaseOID is the merge-base OID used as the expected head for the
	// first commit in every push test.
	testMergeBaseOID = "base0"

	// testCommitBody is the local commit message body returned by the git fake
	// in the push tests.
	testCommitBody = "Add a file"
)

// errBoom is a stand-in transport-layer failure used to wrap the githubgql
// sentinels in the error-handling tests.
var errBoom = errors.New("boom") // lint:allow_errorf

// newTestOptions builds a PushOptions with a known remote-addressing Config,
// the shared merge-base OID, and the supplied local hashes. It centralizes the
// fixture so the individual tests stay focused on the behavior under test.
func newTestOptions(localHashes []string) *PushOptions {
	return &PushOptions{
		Config: &Config{
			RepositoryNameWithOwner: "owner/repo",
			RemoteName:              "origin",
			RemoteBranchName:        "main",
		},
		MergeBaseOID: testMergeBaseOID,
		LocalHashes:  localHashes,
	}
}

// newSuccessGitFake builds a gitclient fake whose single-file, small-contents
// results make BuildFileChanges produce exactly one small addition (one
// chunk). The RemoteHeadOID result is set to reverifyOID so a re-verification
// before a later commit matches when reverifyOID equals the previously created
// OID.
func newSuccessGitFake(reverifyOID string) *gitclientfake.Fake {
	return &gitclientfake.Fake{
		CommitMessageBodyResult: gitclientfake.CommitMessageBodyResult{
			Body: testCommitBody,
		},
		NameStatusResult: gitclientfake.NameStatusResult{
			Lines: []string{"A\tfile.txt"},
		},
		FileContentsAtCommitResult: gitclientfake.FileContentsAtCommitResult{
			Contents: []byte("hello"),
		},
		RemoteHeadOIDResult: gitclientfake.RemoteHeadOIDResult{
			OID: reverifyOID,
		},
	}
}

// TestPushFirstCommitUsesMergeBase confirms the first commit created uses the
// merge-base OID as its expected head.
//
// Validates: Requirements 12.1.
func TestPushFirstCommitUsesMergeBase(t *testing.T) {
	git := newSuccessGitFake("")
	gql := &githubgqlfake.Fake{
		Results: []githubgqlfake.Result{{OID: "oid1"}},
	}
	opts := newTestOptions([]string{"localA"})

	err := Push(context.Background(), git, gql, opts)
	require.NoError(t, err)

	require.Len(t, gql.Calls, 1)
	assert.Equal(t, opts.MergeBaseOID, gql.Calls[0].ExpectedHeadOid)
	assert.Equal(t, testMergeBaseOID, gql.Calls[0].ExpectedHeadOid)
}

// TestPushSubsequentCommitUsesPreviousOID confirms the second commit uses the
// first commit's OID as its expected head, and that the remote head is
// re-verified against that OID before the second commit is created.
//
// Validates: Requirements 12.2, 12.3.
func TestPushSubsequentCommitUsesPreviousOID(t *testing.T) {
	// The re-verify before commit two reads RemoteHeadOID, which must equal the
	// OID the first commit produced ("oid1") for the push to proceed.
	git := newSuccessGitFake("oid1")
	gql := &githubgqlfake.Fake{
		Results: []githubgqlfake.Result{
			{OID: "oid1"},
			{OID: "oid2"},
		},
	}
	opts := newTestOptions([]string{"localA", "localB"})

	err := Push(context.Background(), git, gql, opts)
	require.NoError(t, err)

	require.Len(t, gql.Calls, 2)
	assert.Equal(t, opts.MergeBaseOID, gql.Calls[0].ExpectedHeadOid)
	assert.Equal(t, "oid1", gql.Calls[1].ExpectedHeadOid)

	// The remote head was re-fetched and re-read exactly once, before the
	// second commit.
	require.Len(t, git.FetchRemoteBranchCalls, 1)
	require.Len(t, git.RemoteHeadOIDCalls, 1)
	assert.Equal(t, "origin", git.FetchRemoteBranchCalls[0].RemoteName)
	assert.Equal(t, "main", git.FetchRemoteBranchCalls[0].RemoteBranchName)
}

// TestPushReverifyMismatchYieldsPartialFailure confirms that when the re-fetched
// remote head no longer matches the last commit this tool pushed, Push returns
// an error wrapping ErrPartialPushFailure that reports the already-pushed OIDs
// and the local commits not yet pushed.
//
// Validates: Requirements 12.4, 13.2.
func TestPushReverifyMismatchYieldsPartialFailure(t *testing.T) {
	// RemoteHeadOID returns a value that does not match the first commit's OID
	// ("oid1"), so the re-verification before commit two fails.
	git := newSuccessGitFake("somethingElse")
	gql := &githubgqlfake.Fake{
		Results: []githubgqlfake.Result{
			{OID: "oid1"},
			{OID: "oid2"},
		},
	}
	opts := newTestOptions([]string{"localA", "localB"})

	err := Push(context.Background(), git, gql, opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPartialPushFailure))

	// Only the first commit was created; the second was never attempted.
	require.Len(t, gql.Calls, 1)

	// The error reports the pushed OID and the local hash that was not pushed.
	assert.Contains(t, err.Error(), "oid1")
	assert.Contains(t, err.Error(), "localB")
}

// TestChunkMessageMultiChunkAnnotatesParts confirms a multi-chunk commit has
// each chunk's headline annotated with its one-based part index and total
// count while the body is left unchanged.
//
// Validates: Requirements 12.5.
func TestChunkMessageMultiChunkAnnotatesParts(t *testing.T) {
	message := githubgql.CommitMessage{
		Headline: "Add a file",
		Body:     "Detailed body text.",
	}

	testCases := []struct {
		name             string
		expectedHeadline string
		chunkIndex       int
		chunkCount       int
	}{
		{
			name:             "first of three parts",
			chunkIndex:       0,
			chunkCount:       3,
			expectedHeadline: "Add a file (part 1/3)",
		},
		{
			name:             "second of three parts",
			chunkIndex:       1,
			chunkCount:       3,
			expectedHeadline: "Add a file (part 2/3)",
		},
		{
			name:             "third of three parts",
			chunkIndex:       2,
			chunkCount:       3,
			expectedHeadline: "Add a file (part 3/3)",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := chunkMessage(message, testCase.chunkIndex, testCase.chunkCount)

			assert.Equal(t, testCase.expectedHeadline, result.Headline)
			assert.Equal(t, message.Body, result.Body)
		})
	}
}

// TestChunkMessageSingleChunkPassthrough confirms a single-chunk commit returns
// its message unmodified, both when the count is one and when the count is zero
// or negative (defensive lower bound).
//
// Validates: Requirements 12.6.
func TestChunkMessageSingleChunkPassthrough(t *testing.T) {
	message := githubgql.CommitMessage{
		Headline: "Add a file",
		Body:     "Detailed body text.",
	}

	testCases := []struct {
		name       string
		chunkIndex int
		chunkCount int
	}{
		{name: "single chunk", chunkIndex: 0, chunkCount: 1},
		{name: "zero chunk count", chunkIndex: 0, chunkCount: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := chunkMessage(message, testCase.chunkIndex, testCase.chunkCount)

			assert.Equal(t, message, result)
		})
	}
}

// TestPushSingleCommitUsesUnmodifiedMessage confirms an end-to-end single-chunk
// push sends the unmodified BuildCommitMessage output as the commit message,
// with no part annotation on the headline.
//
// Validates: Requirements 12.6.
func TestPushSingleCommitUsesUnmodifiedMessage(t *testing.T) {
	git := newSuccessGitFake("")
	gql := &githubgqlfake.Fake{
		Results: []githubgqlfake.Result{{OID: "oid1"}},
	}
	opts := newTestOptions([]string{"localA"})

	err := Push(context.Background(), git, gql, opts)
	require.NoError(t, err)

	require.Len(t, gql.Calls, 1)

	expected := BuildCommitMessage(testCommitBody, "localA")
	assert.Equal(t, expected, gql.Calls[0].Message)
	assert.NotContains(t, gql.Calls[0].Message.Headline, "part")
}

// TestPushFirstCommitFailurePropagates confirms that a GitHub API failure on
// the very first commit propagates an error that still matches
// ErrGithubAPI via errors.Is, and that no commit was recorded as created.
//
// Validates: Requirements 13.1.
func TestPushFirstCommitFailurePropagates(t *testing.T) {
	git := newSuccessGitFake("")
	gql := &githubgqlfake.Fake{
		Results: []githubgqlfake.Result{
			{Err: fmt.Errorf("%w: %w", githubgql.ErrGithubAPI, errBoom)},
		},
	}
	opts := newTestOptions([]string{"localA"})

	err := Push(context.Background(), git, gql, opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, githubgql.ErrGithubAPI))

	// The first (and only) commit was attempted once.
	require.Len(t, gql.Calls, 1)

	// No re-verification occurred because no commit was ever recorded as
	// pushed.
	assert.Empty(t, git.FetchRemoteBranchCalls)
}

// TestPushLaterCommitFailurePropagates confirms that when the first commit
// succeeds and re-verification passes, a GitHub API failure on the second
// commit propagates an error matching ErrGithubAPI, with exactly one commit
// created and one re-fetch recorded before the failing attempt.
//
// Validates: Requirements 13.2.
func TestPushLaterCommitFailurePropagates(t *testing.T) {
	// The first commit produces "oid1" and the re-verify before commit two
	// matches it, so the second commit is attempted and fails.
	git := newSuccessGitFake("oid1")
	gql := &githubgqlfake.Fake{
		Results: []githubgqlfake.Result{
			{OID: "oid1"},
			{Err: fmt.Errorf("%w: %w", githubgql.ErrGithubAPI, errBoom)},
		},
	}
	opts := newTestOptions([]string{"localA", "localB"})

	err := Push(context.Background(), git, gql, opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, githubgql.ErrGithubAPI))

	// One commit succeeded before the failing second attempt, so two calls
	// were made in total.
	require.Len(t, gql.Calls, 2)

	// The remote head was re-fetched once, before the second commit.
	require.Len(t, git.FetchRemoteBranchCalls, 1)
}
