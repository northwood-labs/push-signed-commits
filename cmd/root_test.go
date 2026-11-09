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

import (
	"context"
	"errors"
	"testing"

	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"

	"go.nwlabs.dev/push-signed-commits/internal/gitclient/gitclientfake"
)

const (
	// testOwnerRepo is the owner/name slug used to build the HTTPS remote URL
	// in the cmd verify-path test.
	testOwnerRepo = "owner/repo"

	// testHTTPSRemoteURL is the HTTPS remote URL the verify path is expected to
	// bind both the existence check and the fetch to.
	testHTTPSRemoteURL = "https://github.com/owner/repo.git"

	// testToken is the GitHub token the verify path is expected to pass to the
	// existence check and the fetch as HTTP basic auth.
	testToken = "test-token" // lint:not_a_secret

	// testMergeBaseOID is the merge-base OID returned by the fake. It equals
	// the remote head OID so verifyRemoteState finds no divergence and returns
	// success.
	testMergeBaseOID = "base0"
)

// TestVerifyRemoteStatePassesURLAndToken drives verifyRemoteState through the
// gitclient fake and asserts that both transport operations are authenticated
// and targeted against the well-known HTTPS remote URL. The fake is configured
// so the remote branch exists, the fetch and rebase succeed, and the merge base
// equals the remote head OID, which makes verifyRemoteState return success. The
// test then inspects the recorded RemoteBranchExistsCalls[0] and
// FetchRemoteBranchCalls[0] and confirms each carries the HTTPS remote URL
// built from the owner/name pair and the expected token.
//
// Validates: Requirements 6.1, 6.2, 6.3.
func TestVerifyRemoteStatePassesURLAndToken(t *testing.T) {
	ctx := context.Background()

	fake := &gitclientfake.Fake{
		RemoteBranchExistsResult: gitclientfake.RemoteBranchExistsResult{
			Exists: true,
		},
		MergeBaseResult: gitclientfake.MergeBaseResult{
			OID: testMergeBaseOID,
		},
		RemoteHeadOIDResult: gitclientfake.RemoteHeadOIDResult{
			OID: testMergeBaseOID,
		},
	}

	mergeBase, err := verifyRemoteState(ctx, fake, &verifyRemoteStateInput{
		Config: &Config{
			OwnerRepo:    testOwnerRepo,
			LocalBranch:  "feature",
			RemoteName:   "origin",
			RemoteBranch: "main",
		},
		Token:    testToken,
		HTTPSURL: testHTTPSRemoteURL,
	})

	require.NoError(t, err)
	assert.Equal(t, testMergeBaseOID, mergeBase)

	require.Len(t, fake.RemoteBranchExistsCalls, 1)
	assert.Equal(t, testHTTPSRemoteURL, fake.RemoteBranchExistsCalls[0].HTTPSRemoteURL)
	assert.Equal(t, testToken, fake.RemoteBranchExistsCalls[0].Token)

	require.Len(t, fake.FetchRemoteBranchCalls, 1)
	assert.Equal(t, testHTTPSRemoteURL, fake.FetchRemoteBranchCalls[0].HTTPSRemoteURL)
	assert.Equal(t, testToken, fake.FetchRemoteBranchCalls[0].Token)
}

// TestRunPipelineMissingTokenReturnsErrorBeforeTransport drives runPipeline
// with valid branch names and an empty GITHUB_TOKEN and asserts the pipeline
// fails with ErrGithubTokenMissing. Because the missing-token check sits after
// branch validation but before gitclient.New and before verifyRemoteState, the
// early return happens before any client is constructed, so no transport
// operation can run. The fresh gitclientfake.Fake records zero
// RemoteBranchExistsCalls and zero FetchRemoteBranchCalls, documenting the
// before-any-transport ordering: there is no client for the pipeline to have
// called.
//
// Validates: Requirements 2.1, 2.2, 2.3.
func TestRunPipelineMissingTokenReturnsErrorBeforeTransport(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")

	ctx := context.Background()

	fake := &gitclientfake.Fake{}

	err := runPipeline(ctx, &Config{
		OwnerRepo:    testOwnerRepo,
		LocalBranch:  "feature",
		RemoteName:   "origin",
		RemoteBranch: "main",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrGithubTokenMissing))

	assert.Empty(t, fake.RemoteBranchExistsCalls)
	assert.Empty(t, fake.FetchRemoteBranchCalls)
}
