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

import (
	"context"
	"errors"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/go-openapi/testify/assert"
	"github.com/go-openapi/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// errTransport is a stand-in for a transport/HTTP-level failure that does not
// carry a GraphQL error list, used to exercise the ErrGithubAPI mapping.
var errTransport = errors.New("connection refused") // lint:allow_errorf

type (
	// stubGraphQLClient is a minimal [graphql.Client] that scripts a single
	// MakeRequest outcome. When makeErr is non-nil it is returned unchanged so
	// the response-interpretation logic sees the raw error; otherwise the OID
	// is written into the generated response type so CreateCommitOnBranch
	// exercises its success and empty-OID paths.
	stubGraphQLClient struct {
		makeErr error
		oid     string
	}
)

// MakeRequest either returns the scripted error or populates the generated
// response's commit OID. It satisfies the [graphql.Client] interface so a
// *Client can be driven end-to-end without a live endpoint.
func (s *stubGraphQLClient) MakeRequest(
	_ context.Context,
	_ *graphql.Request,
	resp *graphql.Response,
) error {
	if s.makeErr != nil {
		return s.makeErr
	}

	data, ok := resp.Data.(*createCommitOnBranchResponse)
	if !ok {
		return errTransport
	}

	data.CreateCommitOnBranch.Commit.Oid = s.oid

	return nil
}

// staleList builds a GraphQL error list whose single entry marks a stale head
// via the Extensions "type" value.
func staleList() gqlerror.List {
	return gqlerror.List{
		&gqlerror.Error{
			Message:    "head is out of date",
			Extensions: map[string]any{"type": staleDataType},
		},
	}
}

// staleMessageList builds a GraphQL error list whose stale marker is detectable
// only through the message substring fallback (no Extensions type).
func staleMessageList() gqlerror.List {
	return gqlerror.List{
		&gqlerror.Error{
			Message: "the branch moved: STALE_DATA detected",
		},
	}
}

// genericList builds a GraphQL error list that carries no stale marker, which
// must map to ErrGithubAPI.
func genericList() gqlerror.List {
	return gqlerror.List{
		&gqlerror.Error{
			Message:    "something else went wrong",
			Extensions: map[string]any{"type": "OTHER"},
		},
	}
}

func TestInterpretError(t *testing.T) {
	testCases := []struct {
		err          error
		wantSentinel error
		name         string
	}{
		{
			name:         "transport error maps to ErrGithubAPI",
			err:          errTransport,
			wantSentinel: ErrGithubAPI,
		},
		{
			name:         "stale-data list maps to ErrPartialPushFailure",
			err:          staleList(),
			wantSentinel: ErrPartialPushFailure,
		},
		{
			name:         "stale-data via message fallback maps to ErrPartialPushFailure",
			err:          staleMessageList(),
			wantSentinel: ErrPartialPushFailure,
		},
		{
			name:         "non-stale list maps to ErrGithubAPI",
			err:          genericList(),
			wantSentinel: ErrGithubAPI,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, sentinel := interpretError(context.Background(), testCase.err, "owner/repo/main")

			require.Error(t, sentinel)
			assert.True(
				t,
				errors.Is(sentinel, testCase.wantSentinel),
				"expected sentinel to match the mapped package error",
			)
		})
	}
}

func TestHasStaleData(t *testing.T) {
	testCases := []struct {
		name      string
		gqlErrors gqlerror.List
		want      bool
	}{
		{
			name:      "extensions type marks stale data",
			gqlErrors: staleList(),
			want:      true,
		},
		{
			name:      "message substring marks stale data",
			gqlErrors: staleMessageList(),
			want:      true,
		},
		{
			name:      "generic list is not stale",
			gqlErrors: genericList(),
			want:      false,
		},
		{
			name:      "nil entries are skipped",
			gqlErrors: gqlerror.List{nil},
			want:      false,
		},
		{
			name:      "empty list is not stale",
			gqlErrors: gqlerror.List{},
			want:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, hasStaleData(testCase.gqlErrors))
		})
	}
}

func TestClientCreateCommitOnBranch(t *testing.T) {
	testCases := []struct {
		wantSentinel error
		stub         *stubGraphQLClient
		name         string
		wantOID      string
		wantErr      bool
	}{
		{
			name:         "transport error maps to ErrGithubAPI",
			stub:         &stubGraphQLClient{makeErr: errTransport},
			wantErr:      true,
			wantSentinel: ErrGithubAPI,
		},
		{
			name:         "stale-data list maps to ErrPartialPushFailure",
			stub:         &stubGraphQLClient{makeErr: staleList()},
			wantErr:      true,
			wantSentinel: ErrPartialPushFailure,
		},
		{
			name:         "non-stale list maps to ErrGithubAPI",
			stub:         &stubGraphQLClient{makeErr: genericList()},
			wantErr:      true,
			wantSentinel: ErrGithubAPI,
		},
		{
			name:         "empty OID maps to ErrGithubAPI",
			stub:         &stubGraphQLClient{oid: ""},
			wantErr:      true,
			wantSentinel: ErrGithubAPI,
		},
		{
			name:    "success returns the OID",
			stub:    &stubGraphQLClient{oid: "deadbeefcafe"},
			wantOID: "deadbeefcafe",
			wantErr: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := &Client{gql: testCase.stub}

			oid, err := client.CreateCommitOnBranch(context.Background(), &CreateCommitInput{
				RepositoryNameWithOwner: "owner/repo",
				RemoteBranchName:        "main",
				ExpectedHeadOid:         "abc123",
			})

			if !testCase.wantErr {
				require.NoError(t, err)
				assert.Equal(t, testCase.wantOID, oid)

				return
			}

			require.Error(t, err)
			assert.Empty(t, oid)
			assert.True(
				t,
				errors.Is(err, testCase.wantSentinel),
				"expected error to wrap the mapped package sentinel",
			)
		})
	}
}
